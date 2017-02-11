//go:generate stringer -type stateT

// Package life manages life cycle of application. An application has follow life state:
//
//  1. Config/init. If a package need initialization, provides Init() function.
//  App main() function call these Init() functions in proper order.
//  TODO: support united config framework, get config settings from config
//  files and command arguments.
//  2. Starting. App call life.Start() function indicate going to starting
//  state. Each package register a function by life.OnStart(), they will called
//  in register order.
//  3. After life.Start() complete, going to  running state.
//  4. Stopping. Calling life.Shutdown() function going to shutdown state. Each
//  package can register a function by life.OnShutdown(), they will called in
//  reversed order.
//
//  Use life package, app do not need to remember start every package in
//  correct order. Keep calling of Start() inside the package itself, clean and elegant.
//  Shutdown state enforces all package and go routines exit properly, without
//  unpredictable state and corrupting data.
package life

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redforks/testing/reset"

	"github.com/redforks/errors"
	"github.com/redforks/hal"
	"github.com/stevenle/topsort"
)

// Callback is callback function called by life package.
type Callback func()

// StateT indicate current application life state.
type StateT int32

const (
	// Initing is the default state of life, in this state, all packages doing
	// init stuff using init() func.
	Initing StateT = iota

	// Starting state runs all package's start functions, they are running in
	// dependent order
	Starting

	// Running is the normal running state, after all packages started.
	Running

	// Shutingdown is the state to do the packages shutdown work.
	Shutingdown

	// Halt is a temporary state after all package shutdown and application not exit,
	// it is mainly used inside life package, normally outside package won't got a change
	// to saw Halt state.
	Halt

	// tag for log
	tag = "life"
)

var (
	l     = sync.Mutex{}
	state StateT

	// copy of state, returned by State() to allow calling State() function without
	// dead-lock. golang do not allow nested Mutex, use `l' variable inside State()
	// will cause nested lock if State() was called inside a callback, such as
	// onStart.
	lastState int32
	pkgs      = []*pkg{}

	// shutdown chann to notify WaitToEnd. Channel closed on shutdown complete.
	shutdown = make(chan struct{})
)

type pkg struct {
	name                string
	onStart, onShutdown Callback
	depends             []string
}

// State return current life state.
func State() StateT {
	return StateT(atomic.LoadInt32(&lastState))
}

// EnsureState ensure current state is expected, panic with specific message if
// failed.
func EnsureState(exp StateT, msg string) {
	if State() != exp {
		log.Panic(msg)
	}
}

// EnsureStatef ensure current state is expected, panic with formatted message
// if failed.
func EnsureStatef(exp StateT, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	EnsureState(exp, msg)
}

func setState(st StateT) {
	// Must called inside `l.Lock()'
	state = st
	atomic.StoreInt32(&lastState, int32(st))
}

// Register a package, optionally includes depended packages. If not provides
// depended package, it will run as registered order. Depends need not to be
// exist, it will check and sort in Start().
func Register(name string, onStart, onShutdown Callback, depends ...string) {
	st := State()
	if st != Initing {
		log.Panicf("[%s] Can not register package \"%s\" in \"%v\" state", tag, name, st)
	}

	for _, p := range pkgs {
		if p.name == name {
			log.Panicf("[%s] package '%s' already registered", tag, name)
		}
	}
	pkgs = append(pkgs, &pkg{name, onStart, onShutdown, depends})
}

func doShutdownPackages(pkgs []*pkg) {
	for i := len(pkgs) - 1; i >= 0; i-- {
		log.Printf("[%s] Shutdown package %s", tag, pkgs[i].name)
		if pkgs[i].onShutdown != nil {
			pkgs[i].onShutdown()
		}
	}
}

// Start put state to starting, Run all registered OnStart() functions, if all
// succeed, move to running state.
// If any OnStart function panic, shutdown all started packages.
func Start() {
	startedPkgs := 0
	l.Lock()
	defer func() {
		l.Unlock()
		if err := recover(); err != nil {
			// stop started packages
			l.Lock()
			defer l.Unlock()

			if startedPkgs > 0 {
				log.Printf("[%s] Error in starting package %s, shutdown all started packages", tag, pkgs[startedPkgs-1].name)
				doShutdownPackages(pkgs[:startedPkgs])
			}

			errors.Handle(nil, err)
			callHooks(OnAbort)
			hal.Exit(10)
			panic(err)
		}
	}()

	if state != Initing {
		log.Panicf("[%s] Can not start in \"%v\" state", tag, state)
	}

	callHooks(BeforeStarting)
	setState(Starting)

	pkgs = sortByDependency(pkgs)
	for i, pkg := range pkgs {
		log.Printf("[%s] Starting package %s", tag, pkg.name)
		if pkg.onStart != nil {
			pkg.onStart()
		}
		startedPkgs = i + 1
	}

	callHooks(BeforeRunning)
	log.Printf("[%s] all packages started, ready to serve", tag)
	setState(Running)

	if !reset.TestMode() {
		go monitorSignal()
	}
}

// Shutdown put state to shutdown, Run all registered OnShutdown() function in
// reserved order.
func Shutdown() {
	l.Lock()
	defer func() {
		// always set exit state to halt
		setState(Halt)
		l.Unlock()

		if err := recover(); err != nil {
			errors.Handle(nil, err)
			callHooks(OnAbort)
			hal.Exit(11)
			panic(err)
		}
	}()

	switch state {
	case Running:
	case Shutingdown:
		log.Fatalf("[%s] corrupt internal state: %v", tag, state)
	default:
		// app can shutdown at any state
		return
	}

	setState(Shutingdown)

	callHooks(BeforeShutingdown)
	doShutdownPackages(pkgs)

	log.Printf("[%s] all packages shutdown, ready to exit", tag)
	close(shutdown)
}

// Abort calling Abort hooks, and then exit. It is useful when fatal error
// occurred outside life package, ensure abort hooks done its job
// (such as: spork/errrpt, async log).
func Abort() {
	Exit(12)
}

// Exit the problem with n as exit code after executing all OnAbort
// hooks. Like Abort() but can set exit code.
func Exit(n int) {
	if State() != Halt || n == 12 {
		callHooks(OnAbort)
	}
	hal.Exit(n)
}

// WaitToEnd block calling goroutine until safely Shutdown. Can only be called
// in running and afterwards state.
func WaitToEnd() {
	l.Lock()

	switch state {
	case Halt:
	case Running, Starting, Initing:
		l.Unlock()
		<-shutdown
		return
	default:
		// Shutingdown can not visible, it is only in Shutdown function
		log.Fatalf("[%s] Unknown state: %v", tag, state)
	}

	l.Unlock()
}

func sortByDependency(pkgs []*pkg) []*pkg {
	graph := topsort.NewGraph()
	pkgMap := make(map[string]*pkg, len(pkgs))

	for _, p := range pkgs {
		pkgMap[p.name] = p
		graph.AddNode(p.name)
	}

	for _, p := range pkgs {
		for _, name := range p.depends {
			if _, exist := pkgMap[name]; !exist {
				log.Printf("[%s] Warning: \"%s\" depends on not exist package \"%s\"", tag, p.name, name)
				continue
			}
			if err := graph.AddEdge(p.name, name); err != nil {
				log.Panicf("[%s] Dependency failed: %s", tag, err)
			}
		}
	}

	sortedPkgNames := doSort(graph)
	if len(sortedPkgNames) != len(pkgs) {
		msg := ""
		for _, p := range pkgs {
			if len(p.depends) != 0 {
				msg += fmt.Sprintf("\n\t%s -> %s", p.name, strings.Join(p.depends, ", "))
			}
		}
		log.Panicf("[%s] Loop dependency detected%s", tag, msg)
	}

	result := make([]*pkg, 0, len(pkgs))
	for _, pkgName := range sortedPkgNames {
		result = append(result, pkgMap[pkgName])
	}

	return result
}

func doSort(g *topsort.Graph) []string {
	result := make([]string, 0, len(pkgs))
	added := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		if noIncoming(pkgs, p) {
			depends, err := g.TopSort(p.name)
			if err != nil {
				log.Panicf("[%s] %v", tag, err)
			}

			for _, p := range depends {
				if !added[p] {
					result = append(result, p)
					added[p] = true
				}
			}
		}
	}

	return result
}

func noIncoming(pkgs []*pkg, p *pkg) bool {
	for _, v := range pkgs {
		for _, pkgName := range v.depends {
			if p.name == pkgName {
				return false
			}
		}
	}
	return true
}

func init() {
	reset.Register(Shutdown, func() {
		setState(Initing)
		pkgs = pkgs[:0]
		hooks = make([][]*hook, 4)
		shutdown = make(chan struct{})
	})
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
