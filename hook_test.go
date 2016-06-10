package life_test

import (
	"github.com/redforks/testing/reset"
	"os"
	. "spork/life"
	"strconv"

	bdd "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redforks/hal"
)

var _ = bdd.Describe("hook", func() {

	bdd.BeforeEach(func() {
		reset.Enable()
		slog = ""

		Register("foo", newLogFunc("onStart"), newLogFunc("onShutdown"))
		hal.Exit = func(n int) {
			appendLog("Exit " + strconv.Itoa(n))
		}
	})

	bdd.AfterEach(func() {
		reset.Disable()
		hal.Exit = os.Exit
	})

	bdd.It("Do not allow add hook other than Initing phase", func() {
		Start()
		Ω(func() {
			RegisterHook("foo", 0, BeforeRunning, newLogFunc("foo"))
		}).Should(Panic())

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

		RegisterHook("foo", 0, OnAbort, newLogFunc("foo"))
		RegisterHook("bar", 1, OnAbort, newLogFunc("bar"))

		Ω(Start).Should(Panic())
		assertLog("onStart\nonShutdown\nfoo\nbar\nExit 10\n")
	})

	bdd.It("Abort because shutdow failed", func() {
		Register("panic", nil, func() {
			panic("foo")
		})

		RegisterHook("foo", 0, OnAbort, newLogFunc("foo"))
		RegisterHook("bar", 1, OnAbort, newLogFunc("bar"))

		Start()
		Ω(Shutdown).Should(Panic())
		assertLog("onStart\nfoo\nbar\nExit 11\n")
	})

	bdd.It("Hooks timeout", func() {
		hold := make(chan interface{})
		wait := make(chan interface{})

		Register("panic", func() {
			panic("foo")
		}, nil)

		RegisterHook("bar", 1, OnAbort, func() {
			<-hold
		})

		go func() {
			Ω(Start).Should(Panic())
			close(wait)
		}()

		Eventually(wait, 1.5).Should(BeClosed(), "abort hooks timeout")
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
