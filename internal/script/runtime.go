// Package script adapts JS to the public SDK. Each Runtime has one owner.
package script

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/dop251/goja"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/sdk"
)

// Source keeps the adapter independent of the content loader.
type Source interface {
	Programs() []*goja.Program
	BehaviorIDs() []string
	NodeDefinitions() []model.NodeDef
	Namespace() string
}
type nodeRegistration struct {
	schema  map[string]sdk.Parameter
	handler sdk.NodeHandler
}
type Runtime struct {
	namespaces   []string
	vm           *goja.Runtime
	behaviors    map[string]sdk.Behavior
	nodes        map[string]nodeRegistration
	ids          map[string]bool
	namespace    string
	frozen       bool
	closed       bool
	logicTime    float64
	bus          sdk.Bus
	eventContext *sdk.EventContext
	commands     []sdk.Command
	eventError   error
}
type behavior struct {
	owner *Runtime
	id    string
	fn    goja.Callable
}
type nodeHandler struct {
	owner *Runtime
	id    string
	fn    goja.Callable
}
type eventHandler struct {
	owner *Runtime
	id    string
	fn    goja.Callable
}

func New(c Source, seed int64) (*Runtime, error) {
	var register func(sdk.Registrar) error
	if source, ok := c.(interface {
		NativeRegistrar() func(sdk.Registrar) error
	}); ok {
		register = source.NativeRegistrar()
	}
	return NewRegistered(c, seed, register)
}

// NewRegistered lets statically linked Go content use the same registration stage.
func NewRegistered(c Source, seed int64, register func(sdk.Registrar) error) (*Runtime, error) {
	r := &Runtime{vm: goja.New(), behaviors: map[string]sdk.Behavior{}, nodes: map[string]nodeRegistration{}, ids: map[string]bool{}, namespace: c.Namespace()}
	r.namespaces = []string{c.Namespace()}
	if source, ok := c.(interface{ Namespaces() []string }); ok {
		r.namespaces = source.Namespaces()
	}
	r.nodes["engine:noop"] = nodeRegistration{handler: noop{}}
	r.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	rng := rand.New(rand.NewSource(seed))
	r.vm.SetRandSource(rng.Float64)
	r.vm.SetTimeSource(func() time.Time { return time.Unix(0, int64(r.logicTime*1e9)).UTC() })
	o := r.vm.NewObject()
	_ = o.Set("version", sdk.Version)
	function := func(id string, v goja.Value) goja.Callable {
		fn, ok := goja.AssertFunction(v)
		if !ok {
			panic(r.vm.NewTypeError("%s must be a function", id))
		}
		return fn
	}
	_ = o.Set("registerBehavior", func(id string, v goja.Value) {
		if err := r.RegisterBehavior(id, &behavior{r, id, function(id, v)}); err != nil {
			panic(r.vm.NewGoError(err))
		}
	})
	_ = o.Set("registerNode", func(id string, schema map[string]sdk.Parameter, v goja.Value) {
		if err := r.RegisterNode(id, schema, &nodeHandler{r, id, function(id, v)}); err != nil {
			panic(r.vm.NewGoError(err))
		}
	})
	_ = o.Set("on", func(id, scope, kind string, v goja.Value) func() {
		fn := function(id, v)
		unsubscribe, err := r.RegisterEvent(id, scope, kind, &eventHandler{r, id, fn})
		if err != nil {
			panic(r.vm.NewGoError(err))
		}
		return unsubscribe
	})
	_ = r.vm.Set("sdk", o)
	if register != nil {
		if err := register(r); err != nil {
			r.Close()
			return nil, err
		}
	}
	for _, p := range c.Programs() {
		if err := r.budget(func() error { _, err := r.vm.RunProgram(p); return err }); err != nil {
			r.Close()
			return nil, fmt.Errorf("script registration: %w", err)
		}
	}
	r.frozen = true
	for _, id := range c.BehaviorIDs() {
		if _, ok := r.behaviors[id]; !ok {
			r.Close()
			return nil, fmt.Errorf("unregistered behavior %s", id)
		}
	}
	for _, n := range c.NodeDefinitions() {
		entry, ok := r.nodes[n.Handler]
		if !ok {
			r.Close()
			return nil, fmt.Errorf("node %s: unregistered handler %s", n.ID, n.Handler)
		}
		if err := sdk.ValidateParameters(entry.schema, n.Parameters); err != nil {
			r.Close()
			return nil, fmt.Errorf("node %s: %w", n.ID, err)
		}
	}
	return r, nil
}

type noop struct{}

func (noop) Execute(sdk.NodeContext) ([]sdk.Command, error) { return nil, nil }
func (r *Runtime) claim(id string) error {
	if r.frozen || r.closed {
		return fmt.Errorf("registration closed")
	}
	allowed := false
	for _, namespace := range r.namespaces {
		if strings.HasPrefix(id, namespace+":") && len(id) > len(namespace)+1 {
			allowed = true
		}
	}
	if !allowed {
		return fmt.Errorf("invalid registration ID %s", id)
	}
	if r.ids[id] {
		return fmt.Errorf("duplicate registration %s", id)
	}
	r.ids[id] = true
	return nil
}
func (r *Runtime) RegisterBehavior(id string, b sdk.Behavior) error {
	if b == nil {
		return fmt.Errorf("invalid behavior")
	}
	if err := r.claim(id); err != nil {
		return err
	}
	r.behaviors[id] = b
	return nil
}
func (r *Runtime) RegisterNode(id string, schema map[string]sdk.Parameter, h sdk.NodeHandler) error {
	if h == nil {
		return fmt.Errorf("invalid node handler")
	}
	if err := sdk.ValidateSchema(schema); err != nil {
		return err
	}
	if err := r.claim(id); err != nil {
		return err
	}
	copySchema := map[string]sdk.Parameter{}
	for k, v := range schema {
		copySchema[k] = v
	}
	r.nodes[id] = nodeRegistration{copySchema, h}
	return nil
}
func (r *Runtime) RegisterEvent(id, scope, kind string, h sdk.EventHandler) (func(), error) {
	if h == nil {
		return nil, fmt.Errorf("invalid event handler")
	}
	allowed := scope == "client" && kind == "snapshot" || scope == "session" && kind == "node_entered" || scope == "scene" && (kind == "hit" || kind == "after_damage" || kind == "wave_completed")
	if !allowed {
		return nil, fmt.Errorf("unsupported event %s:%s", scope, kind)
	}
	if err := r.claim(id); err != nil {
		return nil, err
	}
	return r.bus.On(scope+":"+kind, func(model.Event) {
		if r.eventError != nil {
			return
		}
		commands, err := h.Handle(detach(*r.eventContext))
		if err != nil {
			r.eventError = fmt.Errorf("%s: %w", id, err)
			return
		}
		if len(r.commands)+len(commands) > 128 {
			r.eventError = fmt.Errorf("%s: event command budget exceeded", id)
			return
		}
		r.commands = append(r.commands, sdk.CloneCommands(commands)...)
	}), nil
}
func (r *Runtime) Decide(id string, ctx sdk.Context) (sdk.Action, error) {
	if r.closed {
		return sdk.Action{}, fmt.Errorf("runtime closed")
	}
	b, ok := r.behaviors[id]
	if !ok {
		return sdk.Action{}, fmt.Errorf("unknown behavior %s", id)
	}
	r.logicTime = ctx.Time
	a, err := b.Decide(ctx)
	if err == nil && !a.Move.Finite() {
		err = fmt.Errorf("%s: non-finite behavior output", id)
	}
	return a, err
}
func (b *behavior) Decide(ctx sdk.Context) (sdk.Action, error) {
	var a sdk.Action
	err := b.owner.invoke(b.id, b.fn, ctx, &a)
	if err == nil && !a.Move.Finite() {
		err = fmt.Errorf("%s: non-finite script output", b.id)
	}
	return a, err
}
func (h *nodeHandler) Execute(ctx sdk.NodeContext) ([]sdk.Command, error) {
	var commands []sdk.Command
	err := h.owner.invoke(h.id, h.fn, ctx, &commands)
	return commands, err
}
func (h *eventHandler) Handle(ctx sdk.EventContext) ([]sdk.Command, error) {
	var commands []sdk.Command
	err := h.owner.invoke(h.id, h.fn, ctx, &commands)
	return commands, err
}

// Contexts are detached for every callback, including Go handlers and observers.
func detach[T any](value T) T {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var copy T
	if err = json.Unmarshal(data, &copy); err != nil {
		panic(err)
	}
	return copy
}
func (r *Runtime) Node(id string, ctx sdk.NodeContext) ([]sdk.Command, error) {
	if r.closed {
		return nil, fmt.Errorf("runtime closed")
	}
	entry, ok := r.nodes[id]
	if !ok {
		return nil, fmt.Errorf("unknown node handler %s", id)
	}
	if err := sdk.ValidateParameters(entry.schema, ctx.Parameters); err != nil {
		return nil, err
	}
	commands, err := entry.handler.Execute(detach(ctx))
	return sdk.CloneCommands(commands), err
}
func (r *Runtime) Dispatch(ctx sdk.EventContext) ([]sdk.Command, error) {
	if r.closed || r.eventContext != nil {
		return nil, fmt.Errorf("runtime closed or recursive dispatch")
	}
	r.logicTime = ctx.Time
	r.eventContext = &ctx
	r.commands = nil
	r.eventError = nil
	e := ctx.Event
	e.Kind = ctx.Scope + ":" + e.Kind
	r.bus.Emit(e)
	commands, err := r.commands, r.eventError
	r.eventContext = nil
	r.commands = nil
	return commands, err
}
func (r *Runtime) Close() {
	r.closed = true
	r.bus = sdk.Bus{}
	r.nodes = nil
	r.behaviors = nil
	r.commands = nil
	r.eventContext = nil
	r.eventError = nil
	r.ids = nil
	r.vm = nil
}

func (r *Runtime) HasEvent(scope, kind string) bool { return !r.closed && r.bus.Has(scope+":"+kind) }
func (r *Runtime) invoke(id string, fn goja.Callable, ctx, out any) error {
	err := r.budget(func() error {
		v, err := fn(goja.Undefined(), r.vm.ToValue(ctx))
		if err != nil {
			return err
		}
		if goja.IsUndefined(v) || goja.IsNull(v) {
			return nil
		}
		return r.vm.ExportTo(v, out)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", id, err)
	}
	return nil
}

// Interrupt is the only cross-goroutine VM operation. A late timer must complete
// before ClearInterrupt so it cannot poison the next callback.
func (r *Runtime) budget(call func() error) error {
	done := make(chan struct{})
	timer := time.AfterFunc(25*time.Millisecond, func() { r.vm.Interrupt("script budget exceeded"); close(done) })
	err := call()
	if !timer.Stop() {
		<-done
	}
	r.vm.ClearInterrupt()
	return err
}
