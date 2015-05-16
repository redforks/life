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
		Register("pkg1", func() {
			appendLog("pkg1")
			assert.Equal(t(), starting, state)
		}, nil)
		Start()
		assert.Equal(t(), running, currentState())
		assertLog("pkg1\n")
	})

	bdd.It("OnStart two", func() {
		Register("pkg1", newLogFunc("pkg1"), nil)
		Register("pkg2", newLogFunc("pkg2"), nil)
		Register("pkg3", nil, nil)
		Start()
		assertLog("pkg1\npkg2\n")
	})

	bdd.Context("Register() in wrong phase", func() {

		bdd.It("Running", func() {
			Start()
			tassert.Panics(t(), func() {
				Register("pkg1", nil, nil)
			}, "[life] Can not register package \"pkg1\" in \"running\" phase")
		})

		bdd.It("Starting", func() {
			Register("pkg2", func() {
				Register("pkg1", func() {}, nil)
			}, nil)
			tassert.Panics(t(), func() {
				Start()
			}, "[life] Can not register package \"pkg1\" in \"starting\" phase")
		})

		bdd.It("Shutdown", func() {
			Register("pkg2", nil, func() {
				Register("pkg1", nil, nil)
			})
			tassert.Panics(t(), func() {
				state = running
				Shutdown()
			}, "[life] Can not register package \"pkg1\" in \"shutdown\" phase")
		})

	})

	bdd.It("OnShutdown one", func() {
		Register("pkg1", nil, func() {
			appendLog("pkg1")
			assert.Equal(t(), shutdown, state)
		})
		state = running
		Shutdown()
		assertLog("pkg1\n")
		assert.Equal(t(), shutdown, state)
	})

	bdd.It("OnShutdown two", func() {
		Register("pkg1", nil, newLogFunc("pkg1"))
		Register("pkg11", nil, nil)
		Register("pkg2", nil, newLogFunc("pkg2"))
		state = running
		Shutdown()
		assertLog("pkg2\npkg1\n")
	})

})
