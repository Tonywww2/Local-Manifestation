// Package sdk defines extension contracts without importing engine internals.
package sdk

import "hsmultiplayer/pkg/model"

const Version = 6

type Context struct {
	Self     model.Vec `json:"self"`
	Target   model.Vec `json:"target"`
	Distance float64   `json:"distance"`
	Time     float64   `json:"time"`
	Delta    float64   `json:"delta"`
}
type Action struct {
	Move   model.Vec `json:"move"`
	Attack bool      `json:"attack"`
}
type Behavior interface{ Decide(Context) (Action, error) }
type Registrar interface {
	RegisterBehavior(string, Behavior) error
	RegisterNode(string, map[string]Parameter, NodeHandler) error
	RegisterEvent(string, string, string, EventHandler) (func(), error)
}

// Bus is owned by one simulation goroutine. Listeners receive values, not world pointers.
type Bus struct {
	next     uint64
	handlers map[string][]listener
}
type listener struct {
	id   uint64
	call func(model.Event)
}

func (b *Bus) Has(kind string) bool { return len(b.handlers[kind]) > 0 }

func (b *Bus) On(kind string, call func(model.Event)) func() {
	if b.handlers == nil {
		b.handlers = map[string][]listener{}
	}
	b.next++
	id := b.next
	b.handlers[kind] = append(b.handlers[kind], listener{id, call})
	return func() {
		list := b.handlers[kind]
		for i, v := range list {
			if v.id == id {
				b.handlers[kind] = append(list[:i], list[i+1:]...)
				break
			}
		}
	}
}
func (b *Bus) Emit(e model.Event) {
	for _, v := range append([]listener(nil), b.handlers[e.Kind]...) {
		v.call(e)
	}
}
