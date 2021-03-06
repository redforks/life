package life_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "github.com/redforks/life"

	"github.com/redforks/testing/matcher"
	"github.com/redforks/testing/reset"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redforks/errors"
	"github.com/redforks/hal"
)

var _ = Describe("life", func() {

	BeforeEach(func() {
		reset.Enable()
		slog = ""

		hal.Exit = func(n int) {
			appendLog("Exit " + strconv.Itoa(n))
		}
	})

	AfterEach(func() {
		reset.Disable()
	})

	It("Register duplicate", func() {
		Register("pkg1", nil, nil)
		Ω(func() {
			Register("pkg1", nil, nil)
		}).Should(matcher.Panics("[life] package 'pkg1' already registered"))
	})

	It("OnStart One", func() {
		Register("pkg1", func() {
			appendLog("pkg1")
			Ω(State()).Should(Equal(Starting))
		}, nil)
		Start()
		Ω(State()).Should(Equal(Running))
		assertLog("pkg1\n")
	})

	It("OnStart two", func() {
		Register("pkg1", newLogFunc("pkg1"), nil)
		Register("pkg2", newLogFunc("pkg2"), nil)
		Register("pkg3", nil, nil)
		Start()
		assertLog("pkg1\npkg2\n")
	})

	Context("Register() in wrong state", func() {

		It("Running", func() {
			Start()
			Ω(func() {
				Register("pkg1", nil, nil)
			}).Should(matcher.Panics("[life] Can not register package \"pkg1\" in \"Running\" state"))
		})

		It("Starting", func() {
			Register("pkg2", func() {
				Register("pkg1", func() {}, nil)
			}, nil)
			Ω(func() {
				Start()
			}).Should(matcher.Panics("[life] Can not register package \"pkg1\" in \"Starting\" state"))
		})

		It("Shutdown", func() {
			Register("pkg2", nil, func() {
				Register("pkg1", nil, nil)
			})
			Start()
			Ω(func() {
				Shutdown()
			}).Should(matcher.Panics("[life] Can not register package \"pkg1\" in \"Shutingdown\" state"))
		})

	})

	It("OnShutdown one", func() {
		Register("pkg1", nil, func() {
			appendLog("pkg1")
			Ω(State()).Should(Equal(Shutingdown))
		})
		Start()
		Shutdown()
		assertLog("pkg1\n")
		Ω(State()).Should(Equal(Halt))
	})

	It("OnShutdown two", func() {
		Register("pkg1", nil, newLogFunc("pkg1"))
		Register("pkg11", nil, nil)
		Register("pkg2", nil, newLogFunc("pkg2"))
		Start()
		Shutdown()
		assertLog("pkg2\npkg1\n")
	})

	It("Stop started packages on some package panic", func() {
		Register("pkg1", newLogFunc("start1"), newLogFunc("stop1"))
		Register("pkg2", func() {
			panic("pkg2")
		}, newLogFunc("stop2"))
		Register("pkg3", newLogFunc("start3"), newLogFunc("stop3"))
		Ω(Start).Should(Panic())
		assertLog("start1\nstop1\nExit 10\n")
	})

	Context("WaitToEnd", func() {
		var (
			wait  chan struct{}
			start time.Time
		)

		BeforeEach(func() {
			wait = make(chan struct{})
		})

		var startWait = func() {
			start = time.Now()
			go func() {
				WaitToEnd()
				close(wait)
			}()
		}

		var assertShutdown = func(delayMin, delayMax time.Duration) {
			Eventually(wait).Should(BeClosed())
			Ω(time.Since(start)).Should(BeNumerically(">", delayMin))
		}

		It("block until shutdown", func() {
			Register("pkg", nil, func() {
				time.Sleep(5 * time.Millisecond)
			})
			Start()

			startWait()
			Shutdown()
			assertShutdown(4*time.Millisecond, 15*time.Millisecond)
		})

		It("During shutdown", func() {
			Register("pkg", nil, func() {
				time.Sleep(6 * time.Millisecond)
			})

			Start()
			go Shutdown()
			startWait()
			assertShutdown(3*time.Millisecond, 15*time.Millisecond)
		})

		It("after shutdown", func() {
			Start()
			Shutdown()
			startWait()
			assertShutdown(0, 5*time.Millisecond)
		})

		It("Shutdown wait for ongoing shutdown request", func() {
			Register("pkg", nil, func() {
				time.Sleep(5 * time.Millisecond)
			})

			Start()
			go Shutdown()
			time.Sleep(time.Millisecond)

			go func() {
				Shutdown()
				close(wait)
			}()
			assertShutdown(3*time.Millisecond, 15*time.Millisecond)
		})

		Context("errors.Handle", func() {

			BeforeEach(func() {
				errors.SetHandler(func(_ context.Context, err interface{}) {
					appendLog(fmt.Sprintf("%s", err))
				})
			})

			AfterEach(func() {
				errors.SetHandler(nil)
			})

			It("error in start", func() {
				Register("pkg", func() {
					panic("error")
				}, newLogFunc("should not called"))

				Ω(Start).Should(Panic(), "error")
				assertLog("error\nExit 10\n")
			})

			It("error in shutdown", func() {
				Register("pkg", nil, func() {
					panic("error")
				})

				Start()
				Ω(Shutdown).Should(Panic(), "error")
				assertLog("error\nExit 11\n")
			})

		})

	})

	Context("Abort hooks", func() {

		It("Abort", func() {
			RegisterHook("pkg1", 0, OnAbort, newLogFunc("foo"))
			Abort()
			assertLog("foo\nExit 12\n")
		})

		It("Exit", func() {
			RegisterHook("pkg1", 0, OnAbort, newLogFunc("foo"))
			Exit(100)
			assertLog("foo\nExit 100\n")
		})

		It("Do not call aborts if already shutdown", func() {
			RegisterHook("pkg1", 0, OnAbort, newLogFunc("foo"))
			Shutdown()
			Exit(100)
			assertLog("Exit 100\n")
		})

		It("Call Abort on Abort", func() {
			// Abort() calls Exit() internally, this test to ensure	even shutdown
			// complete, call Abort() still triggers onAbort hooks
			RegisterHook("pkg1", 0, OnAbort, newLogFunc("foo"))
			Shutdown()
			Abort()
			assertLog("foo\nExit 12\n")
		})

	})

	Context("Sort by dependency", func() {

		It("Two pkgs", func() {
			Register("pkg2", newLogFunc("pkg2"), newLogFunc("pkg2"), "pkg1")
			Register("pkg1", newLogFunc("pkg1"), newLogFunc("pkg1"))
			Start()
			assertLog("pkg1\npkg2\n")
			Shutdown()
			assertLog("pkg2\npkg1\n")
		})

		It("Case 2", func() {
			Register("a", newLogFunc("a"), nil, "b")
			Register("b", newLogFunc("b"), nil)
			Register("c", newLogFunc("c"), nil, "b")
			Start()
			assertLog("b\na\nc\n")
		})

		It("Loop dependency", func() {
			Register("pkg1", nil, nil, "pkg2", "pkg3")
			Register("pkg2", nil, nil, "pkg1")
			Register("pkg3", nil, nil)
			Ω(Start).Should(matcher.Panics("[life] Loop dependency detected\n\tpkg1 -> pkg2, pkg3\n\tpkg2 -> pkg1"))
		})

		It("Depends on not exist package", func() {
			Register("pkg2", nil, nil, "pkg1")
			Ω(Start).ShouldNot(Panic(), "It is not error when depended package not registered, a warning will add to the log")
		})

	})

	Context("EnsureState", func() {
		It("Succeed", func() {
			Ω(func() {
				EnsureState(Initing, "msg")
			}).ShouldNot(Panic())
		})

		It("Failed with message", func() {
			Ω(func() {
				EnsureState(Starting, "msg")
			}).Should(matcher.Panics("msg"))
		})

		It("Failed with formatted message", func() {
			Ω(func() {
				EnsureStatef(Starting, "msg %s %d", "foo", 1)
			}).Should(matcher.Panics("msg foo 1"))
		})
	})

})
