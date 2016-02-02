//go:generate stringer -type=hookType

package life

import (
	"log"
	"sort"
)

// HookFunc called when a hook event occurred. See hookType constants.
type HookFunc func()

type hookType int

const (
	// Hooks called before entering staring phase
	BeforeStarting hookType = iota

	// Hooks called before entering running phase
	BeforeRunning

	// Hooks called before entering shutingdown phase
	BeforeShutingdown
)

type hook struct {
	name  string
	order int
	fn    HookFunc
}

var (
	hooks [][]*hook = make([][]*hook, 3)
)

// RegisterHook register a function that executed when typ hook event occurred. Name is
// used in log only. If multiple function hook to one hookType, they executed
// by order, smaller execute first, If two hooks have the same order, they will
// execute in any order.
//
// Hooks not reset by spork/testing/reset package.
func RegisterHook(name string, order int, typ hookType, fn HookFunc) {
	if State() != Initing {
		log.Panicf("[%s] Can not register hook \"%s\" in \"%s\" phase", tag, name, state)
	}

	hooks[typ] = append(hooks[typ], &hook{
		name:  name,
		order: order,
		fn:    fn,
	})
}

func callHooks(typ hookType) {
	items := hooks[typ]
	sort.Sort(sortHook(items))
	for _, hook := range items {
		log.Printf("[%s] Execute %s hook: %s", tag, typ, hook.name)
		hook.fn()
		log.Printf("[%s] Done %s", tag, hook.name)
	}
}

type sortHook []*hook

func (s sortHook) Len() int {
	return len(s)
}

func (s sortHook) Less(i, j int) bool {
	return s[i].order < s[j].order
}

func (s sortHook) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
