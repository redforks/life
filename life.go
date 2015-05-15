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
	initing stateT = iota
	starting
	running
	shutdown

	// tag for log
	tag = "life"
)

var (
	onStarts, onShutdowns = []LifeCallback{}, []LifeCallback{}
	state                 stateT
)

func currentState() stateT {
	return stateT(atomic.LoadInt32((*int32)(&state)))
}

// Register a function called during Staring phase.
func OnStart(fn LifeCallback) {
	if currentState() != initing {
		log.Panicf("[%s] Can not register OnStart function in \"%s\" phase", tag, state)
	}
	onStarts = append(onStarts, fn)
}

// Register a function called during shutdown phase.
func OnShutdown(fn LifeCallback) {
	if currentState() != initing {
		log.Panicf("[%s] Can not register OnShutdown function in \"%s\" phase", tag, state)
	}
	onShutdowns = append(onShutdowns, fn)
}

// Put phase to starting, Run all registered OnStart() functions, if all
// succeed, move to running phase.
// If any OnStart function panic, Start() won't recover, it is normal to panic
// and exit the app during starting.
func Start() {
	if !atomic.CompareAndSwapInt32((*int32)(&state), int32(initing), int32(starting)) {
		log.Panicf("[%s] Can not register OnStart function in \"%s\" phase", tag, state)
	}
	for _, f := range onStarts {
		f()
	}
	if !atomic.CompareAndSwapInt32((*int32)(&state), int32(starting), int32(running)) {
		log.Panicf("[%s] Corrputed state, expected %s, but %s", tag, starting, currentState())
	}
}

// Put phase to shutdown, Run all registered OnShutdown() function in reserved order.
func Shutdown() {
	if !atomic.CompareAndSwapInt32((*int32)(&state), int32(running), int32(shutdown)) {
		// app can shutdown at any phase, but if not in correct phase, doing nothing
		atomic.StoreInt32((*int32)(&state), int32(shutdown))
		return
	}

	for i := len(onShutdowns) - 1; i >= 0; i-- {
		onShutdowns[i]()
	}
}

func init() {
	reset.Register(nil, func() {
		state = initing
		onStarts = onStarts[:0]
		onShutdowns = onShutdowns[:0]
	})

	go monitorSignal()
}

func monitorSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	log.Printf("[%s] Receive %v signal, start shutdown", tag, <-c)

	done := make(chan struct{})
	go func() {
		Shutdown()
		<-done
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		log.Printf("[%s] Shutdown timeout", tag)
	}
	os.Exit(1)
}
