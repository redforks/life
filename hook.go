//go:generate stringer -type=hookType

package life

import (
	"log"
	"sort"
	"time"

	"github.com/redforks/testing/reset"
)

// HookFunc called when a hook event occurred. See hookType constants.
type HookFunc func()

type hookType int

const (
	// BeforeStarting hooks called before entering staring state
	BeforeStarting hookType = iota

	// BeforeRunning hooks called before entering running state
	BeforeRunning

	// BeforeShutingdown hooks called before entering shutingdown state
	BeforeShutingdown

	// OnAbort hooks called on abnormal error before exit. Abort hooks run in any state,
	// even before your package initialized, check your hooks to work on any states,
	// do not assume opened file, socket, channel, etc.
	OnAbort
)

type hook struct {
	name  string
	order int
	fn    HookFunc
}

var (
	hooks = make([][]*hook, 4)
)

// RegisterHook register a function that executed when typ hook event occurred. Name is
// used in log only. If multiple function hook to one hookType, they executed
// by order, smaller execute first, If two hooks have the same order, they will
// execute in any order.
func RegisterHook(name string, order int, typ hookType, fn HookFunc) {
	if State() != Initing {
		log.Panicf("[%s] Can not register hook \"%s\" in \"%v\" state", tag, name, state)
	}

	hooks[typ] = append(hooks[typ], &hook{
		name:  name,
		order: order,
		fn:    fn,
	})
}

func callHooks(typ hookType) {
	wait := make(chan interface{})
	go func() {
		items := hooks[typ]
		sort.Sort(sortHook(items))
		for _, hook := range items {
			log.Printf("[%s] Execute %v hook: %s", tag, typ, hook.name)
			hook.fn()
			log.Printf("[%s] Done %s", tag, hook.name)
		}
		close(wait)
	}()

	timeout := 30 * time.Second
	if reset.TestMode() {
		timeout = time.Second
	}
	select {
	case <-wait:
	case <-time.After(timeout):
		log.Printf("[%s] %v hook timeout", tag, typ)
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
