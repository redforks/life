package life

import (
	"spork/testing/reset"
	"spork/testing/tassert"

	bdd "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
)

var _ = bdd.Describe("life", func() {
	var (
		log = ""

		appendLog = func(msg string) {
			log += msg + "\n"
		}

		assertLog = func(expected string) {
			assert.Equal(t(), expected, log)
			log = ""
		}

		newLogFunc = func(msg string) LifeCallback {
			return func() {
				appendLog(msg)
			}
		}
	)

	bdd.BeforeEach(func() {
		reset.Enable()
	})

	bdd.AfterEach(func() {
		reset.Disable()
	})

	bdd.It("OnStart One", func() {
		OnStart(func() {
			appendLog("pkg1")
			assert.Equal(t(), starting, state)
		})
		Start()
		assert.Equal(t(), running, currentState())
		assertLog("pkg1\n")
	})

	bdd.It("OnStart two", func() {
		OnStart(newLogFunc("pkg1"))
		OnStart(newLogFunc("pkg2"))
		Start()
		assertLog("pkg1\npkg2\n")
	})

	bdd.Context("Call OnStart() in wrong phase", func() {

		bdd.It("Running", func() {
			Start()
			tassert.Panics(t(), func() {
				OnStart(newLogFunc("pkg1"))
			}, "[life] Can not register OnStart function in \"running\" phase")
		})

		bdd.It("Starting", func() {
			OnStart(func() {
				OnStart(func() {})
			})
			tassert.Panics(t(), func() {
				Start()
			}, "[life] Can not register OnStart function in \"starting\" phase")
		})

		bdd.It("Shutdown", func() {
			OnShutdown(func() {
				OnStart(func() {})
			})
			tassert.Panics(t(), func() {
				state = running
				Shutdown()
			}, "[life] Can not register OnStart function in \"shutdown\" phase")
		})

	})

	bdd.It("OnShutdown one", func() {
		OnShutdown(func() {
			appendLog("pkg1")
			assert.Equal(t(), shutdown, state)
		})
		state = running
		Shutdown()
		assertLog("pkg1\n")
		assert.Equal(t(), shutdown, state)
	})

	bdd.It("OnShutdown two", func() {
		OnShutdown(newLogFunc("pkg1"))
		OnShutdown(newLogFunc("pkg2"))
		state = running
		Shutdown()
		assertLog("pkg2\npkg1\n")
	})

	bdd.Context("OnShutdown if not in running state", func() {

		bdd.BeforeEach(func() {
			OnShutdown(newLogFunc("pkg1"))
		})

		bdd.AfterEach(func() {
			assert.Equal(t(), shutdown, state)
			assertLog("")
		})

		bdd.It("In init state", func() {
			Shutdown()
		})

	})

})
