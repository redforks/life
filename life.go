//go:generate stringer -type stateT

// life package manages life cycle of application. An application has follow life phase:
//
//  1. Config/init. If a package need initialization, provides Init() function.
//  App main() function call these Init() functions in proper order.
//  TODO: support united config framework, get config settings from config
//  files and command arguments.
//  2. Starting. App call life.Start() function indicate going to starting
//  phase. Each package register a function by life.OnStart(), they will called
//  in register order.
//  3. After life.Start() complete, going to  running phase.
//  4. Stopping. Calling life.Shutdown() function going to shutdown phase. Each
//  package can register a function by life.OnShutdown(), they will called in
//  reversed order.
//
//  Use life package, app do not need to remember start every package in
//  correct order. Keep calling of Start() inside the package itself, clean and elegant.
//  Shutdown phase enforces all package and go routines exit properly, without
//  unpredictable state and corrupting data.
//
//  TODO: OnStart() and OnShutdown() called in golang package dependency order, not
//  support if the package actual dependency not match.
package life

import (
	"log"
	"os"
	"os/signal"
	"spork/testing/reset"
	"sync/atomic"
	"syscall"
	"time"
)

type LifeCallback func()

type stateT int32

const (
	// life phase constants
	Initing stateT = iota
	Starting
	Running
	shutdown

	// tag for log
	tag = "life"
)

var (
	state stateT
	pkgs  = []*pkg{}
)

type pkg struct {
	name                string
	onStart, onShutdown LifeCallback
}

func State() stateT {
	return stateT(atomic.LoadInt32((*int32)(&state)))
}

func Register(name string, onStart, onShutdown LifeCallback) {
	if State() != Initing {
		log.Panicf("[%s] Can not register package \"%s\" in \"%s\" phase", tag, name, state)
	}
	pkgs = append(pkgs, &pkg{name, onStart, onShutdown})
}

// Put phase to starting, Run all registered OnStart() functions, if all
// succeed, move to running phase.
// If any OnStart function panic, Start() won't recover, it is normal to panic
// and exit the app during starting.
func Start() {
	if !atomic.CompareAndSwapInt32((*int32)(&state), int32(Initing), int32(Starting)) {
		log.Panicf("[%s] Can not register OnStart function in \"%s\" phase", tag, state)
	}

	for _, pkg := range pkgs {
		log.Printf("[%s] Start package %s", tag, pkg.name)
		if pkg.onStart != nil {
			pkg.onStart()
		}
	}

	if !atomic.CompareAndSwapInt32((*int32)(&state), int32(Starting), int32(Running)) {
		log.Panicf("[%s] Corrputed state, expected %s, but %s", tag, Starting, State())
	}
	log.Printf("[%s] all packages started, ready to serve", tag)
}

// Put phase to shutdown, Run all registered OnShutdown() function in reserved order.
func Shutdown() {
	if !atomic.CompareAndSwapInt32((*int32)(&state), int32(Running), int32(shutdown)) {
		// app can shutdown at any phase, but if not in correct phase, doing nothing
		atomic.StoreInt32((*int32)(&state), int32(shutdown))
		return
	}

	for i := len(pkgs) - 1; i >= 0; i-- {
		log.Printf("[%s] Shutdown package %s", tag, pkgs[i].name)
		if pkgs[i].onShutdown != nil {
			pkgs[i].onShutdown()
		}
	}

	log.Printf("[%s] all packages shutdown, ready to exit", tag)
}

func init() {
	reset.Register(nil, func() {
		Shutdown()
		state = Initing
		pkgs = pkgs[:0]
	})

	go monitorSignal()
}

func monitorSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	log.Printf("[%s] Receive %v signal, start shutdown", tag, <-c)

	go func() {
		log.Fatalf("[%s] Receive %v again, exit immediately", tag, <-c)
	}()

	done := make(chan struct{})
	go func() {
		Shutdown()
		done <- struct{}{}
	}()

	select {
	case <-done:
		break
	case <-time.After(60 * time.Second):
		log.Printf("[%s] Shutdown timeout", tag)
	}
	os.Exit(1)
}
