package life_test

import (
	. "spork/life"
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

	bdd.It("Register duplicate", func() {
		Register("pkg1", nil, nil)
		tassert.Panics(t(), func() {
			Register("pkg1", nil, nil)
		}, "[life] package 'pkg1' already registered")
	})

	bdd.It("OnStart One", func() {
		Register("pkg1", func() {
			appendLog("pkg1")
			assert.Equal(t(), Starting, State())
		}, nil)
		Start()
		assert.Equal(t(), Running, State())
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
			Start()
			tassert.Panics(t(), func() {
				Shutdown()
			}, "[life] Can not register package \"pkg1\" in \"shutdown\" phase")
		})

	})

	bdd.It("OnShutdown one", func() {
		Register("pkg1", nil, func() {
			appendLog("pkg1")
			assert.Equal(t(), Shutingdown, State())
		})
		Start()
		Shutdown()
		assertLog("pkg1\n")
		assert.Equal(t(), Shutingdown, State())
	})

	bdd.It("OnShutdown two", func() {
		Register("pkg1", nil, newLogFunc("pkg1"))
		Register("pkg11", nil, nil)
		Register("pkg2", nil, newLogFunc("pkg2"))
		Start()
		Shutdown()
		assertLog("pkg2\npkg1\n")
	})

	bdd.Context("Sort by dependency", func() {

		bdd.It("Two pkgs", func() {
			Register("pkg2", newLogFunc("pkg2"), newLogFunc("pkg2"), "pkg1")
			Register("pkg1", newLogFunc("pkg1"), newLogFunc("pkg1"))
			Start()
			assertLog("pkg1\npkg2\n")
			Shutdown()
			assertLog("pkg2\npkg1\n")
		})

		bdd.It("Case 2", func() {
			Register("a", newLogFunc("a"), nil, "b")
			Register("b", newLogFunc("b"), nil)
			Register("c", newLogFunc("c"), nil, "b")
			Start()
			assertLog("b\na\nc\n")
		})

		bdd.It("Loop dependency", func() {
			Register("pkg1", nil, nil, "pkg2", "pkg3")
			Register("pkg2", nil, nil, "pkg1")
			Register("pkg3", nil, nil)
			tassert.Panics(t(), Start, "[life] Loop dependency detected\n\tpkg1 -> pkg2, pkg3\n\tpkg2 -> pkg1")
		})

	})

})
