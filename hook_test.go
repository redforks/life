// package document
package life

import (
	bdd "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
)

var _ = bdd.Describe("hook", func() {
	var (
		oldHooks [][]*hook
	)

	bdd.BeforeEach(func() {
		slog = ""
		oldHooks, hooks = hooks, make([][]*hook, 3)

		Register("foo", newLogFunc("onStart"), newLogFunc("onShutdown"))
	})

	bdd.AfterEach(func() {
		Reset()
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

	bdd.It("Sort by order", func() {
		RegisterHook("foo", 10, BeforeStarting, newLogFunc("foo"))
		RegisterHook("bar", 9, BeforeStarting, newLogFunc("bar"))
		RegisterHook("foobar", 11, BeforeStarting, newLogFunc("foobar"))
		Start()
		assertLog("bar\nfoo\nfoobar\nonStart\n")
	})

})
