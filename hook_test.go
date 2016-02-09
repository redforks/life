// package document
package life

import (
	"os"
	"strconv"
	"time"

	bdd "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
)

var _ = bdd.Describe("hook", func() {
	var (
		oldHooks [][]*hook
	)

	bdd.BeforeSuite(func() {
		testMode = true
	})

	bdd.AfterSuite(func() {
		testMode = false
	})

	bdd.BeforeEach(func() {
		slog = ""
		oldHooks, hooks = hooks, make([][]*hook, 4)

		Register("foo", newLogFunc("onStart"), newLogFunc("onShutdown"))
		exit = func(n int) {
			appendLog("Exit " + strconv.Itoa(n))
		}
	})

	bdd.AfterEach(func() {
		Reset()
		exit = os.Exit
		hooks, oldHooks = oldHooks, nil
	})

	bdd.It("Do not allow add hook other than Initing phase", func() {
		Start()
		assert.Panics(t(), func() {
			RegisterHook("foo", 0, BeforeRunning, newLogFunc("foo"))
		})
	})

	bdd.It("BeforeStarting", func() {
		RegisterHook("foo", 0, BeforeStarting, newLogFunc("foo"))
		Start()
		Shutdown()
		assertLog("foo\nonStart\nonShutdown\n")
	})

	bdd.It("BeforeRunning", func() {
		RegisterHook("foo", 0, BeforeRunning, newLogFunc("foo"))
		Start()
		Shutdown()
		assertLog("onStart\nfoo\nonShutdown\n")
	})

	bdd.It("BeforeShutingdown", func() {
		RegisterHook("foo", 0, BeforeShutingdown, newLogFunc("foo"))
		RegisterHook("bar", 0, BeforeRunning, newLogFunc("bar"))

		Start()
		Shutdown()
		assertLog("onStart\nbar\nfoo\nonShutdown\n")
	})

	bdd.It("Abort because start failed", func() {
		Register("panic", func() {
			panic("foo")
		}, nil)

		RegisterHook("foo", 0, Abort, newLogFunc("foo"))
		RegisterHook("bar", 1, Abort, newLogFunc("bar"))

		assert.Panics(t(), Start)
		assertLog("onStart\nfoo\nbar\nExit 10\n")
	})

	bdd.It("Abort because shutdow failed", func() {
		Register("panic", nil, func() {
			panic("foo")
		})

		RegisterHook("foo", 0, Abort, newLogFunc("foo"))
		RegisterHook("bar", 1, Abort, newLogFunc("bar"))

		Start()
		assert.Panics(t(), Shutdown)
		assertLog("onStart\nfoo\nbar\nExit 11\n")
	})

	bdd.It("Hooks timeout", func() {
		hold := make(chan interface{})
		wait := make(chan interface{})

		Register("panic", func() {
			panic("foo")
		}, nil)

		RegisterHook("bar", 1, Abort, func() {
			<-hold
		})

		go func() {
			assert.Panics(t(), Start)
			close(wait)
		}()

		select {
		case <-wait:
		case <-time.After(10 * time.Millisecond):
			assert.Fail(t(), "abort hooks timeout")
		}
		close(hold)
	})

	bdd.It("Sort by order", func() {
		RegisterHook("foo", 10, BeforeStarting, newLogFunc("foo"))
		RegisterHook("bar", 9, BeforeStarting, newLogFunc("bar"))
		RegisterHook("foobar", 11, BeforeStarting, newLogFunc("foobar"))
		Start()
		assertLog("bar\nfoo\nfoobar\nonStart\n")
	})

})
