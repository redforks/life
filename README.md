[![Build Status](https://travis-ci.org/redforks/life.svg?branch=master)](https://travis-ci.org/redforks/life)
[![codebeat badge](https://codebeat.co/badges/c4ea93a5-0266-4614-b68f-06b04999be1a)](https://codebeat.co/projects/github-com-redforks-life)
[![Go Report Card](https://goreportcard.com/badge/github.com/redforks/life)](https://goreportcard.com/report/github.com/redforks/life)
[![Go doc](https://godoc.org/github.com/redforks/life?status.svg)](https://godoc.org/github.com/redforks/life)
[![Go Cover](http://gocover.io/_badge/github.com/redforks/life)](http://gocover.io/github.com/redforks/life)
[![Go Walker](http://gowalker.org/api/v1/badge)](https://gowalker.org/github.com/redforks/life)

# Life

Life is an application runtime life management framework for Go.

## Install

    go install github.com/redforks/life

## Define packages

    // in foo.go
    func init() {
      life.Register("foo", startFoo, shutdownFoo, "bar") 
    }

    // in foobar.go
    func init() {
      life.Register("foobar", startFooBar, nil, "foo", "bar")
    }

    // in bar.go
    func init() {
      life.Register("bar", nil, shutdownBar)
    }

A package has a name, optional `onStart` and `onShutdown` callbacks, zero or more depended packages.
As above example, package `foo` depends on package `bar', `foobar` depends both `foo` and `bar`,
while package `bar` depends no one.

Packages can registered at any order, controlling `init()` execute order is nearlly
impossible afterall. `Life` sort packages in dependence order (topological sorting): `bar`,
`foo`, `foobar`, because `bar` depends on nothing, `foobar` depends on both.

It is common that initialize code depends on services provided by other packages, 
by using `life`, we can ensure they are run in correct order.

Packages have optional onStart callbacks, they will execute in depends order
during `life.Start()`. OnShutdown callbacks execute in reverse order during
`life.Shutdown()`.

## Hooks

If some code must execute at centern point, use hooks. Register hooks this way:

    func init() {
        life.RegisterHook("foo", 10, life.BeforeRunning, fnFoo)
    }

Hook names are used only in log.

Hooks are execute by order argument (2nd argument), lesser value execute first. 

There are following hook types:

 * BeforeStarting, execute before all `onStart` callbacks.
 * BeforeRunning, execute before entering `Running` state, i.e. execute after
   all `onStart` callbacks.
 * BeforeShutingdown, execute before all `onShutdown` callbacks.
 * OnAbort, execute if life exit Unexpectedly.

## States

Life manages application in states, here is the state diagram:

![State diagram](https://github.com/redforks/life/raw/gh-pages/Life%20Cycle.png)

In this diagram, round rectangles are states, rectangles are functions trigger the state changes,
diamonds are callbacks and hooks registered to life package.

On application start, life state default to `Initing', during this state, all
packages init there codes by using `init()` functions.

A typical `main()` function using `life` looks like:

    func main() {
      life.Start()

      // run your service, such as:
      log.Print(http.ListenAndServe(":8080", nil))
      
      // Start shutdown process
      life.Shutdown()
    }

`life.Start()` will:

 1. Execute `BeforeStarting` hooks
 1. Set state to `Starting`
 1. Execute `OnStart` callbacks in dependency order
 1. Execute `BeforeStarting` hooks
 1. Set state to `Running`

If panic cached in one of `OnStart` callbacks, `life` calls all started
packages' `OnShutdown` callbacks to shutdown properly before application exit.

`life.Shutdown()` will:

 1. Execute `BeforeShutingdown` hooks
 1. Set state to `Shutingdown`
 1. Execute `OnShutdown` callbacks in reversed dependency order

## Abort

Application may encounter fatal error must abort its execution, but some
critical works must be done before abort. These work can use `OnAbort` hook.

`OnAbort` hooks called if a panic cached in one of `OnStart` and `OnShutdown`
callbacks, but not called if `life.Shutdown()` succeed. `OnAbort` used in
abnormal exit situation.

Call `life.Abort()` to trigger abort sequence on other abort situations, such as
in a signal handler.

`life.Abort()` set application exit to 12, call `life.Exit(n)` if want other
exit code.
