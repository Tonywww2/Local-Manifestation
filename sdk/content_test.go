package sdk

import (
	"hsmultiplayer/pkg/model"
	"math"
	"testing"
)

func TestParametersRejectUnknownMissingAndNonFinite(t *testing.T) {
	schema := map[string]Parameter{"cost": {Type: "integer", Minimum: 1, Maximum: 100}}
	for _, params := range []map[string]any{nil, {"cost": 1, "unknown": 2}, {"cost": 1.5}, {"cost": math.Inf(1)}, {"cost": "2"}, {"cost": nil}} {
		if ValidateParameters(schema, params) == nil {
			t.Fatal("accepted", params)
		}
	}
	if err := ValidateParameters(schema, map[string]any{"cost": 2}); err != nil {
		t.Fatal(err)
	}
}
func TestBusUsesRegistrationOrderAndDispatchSnapshot(t *testing.T) {
	var bus Bus
	var received []string
	var remove func()
	bus.On("fact", func(model.Event) { received = append(received, "a"); remove() })
	remove = bus.On("fact", func(model.Event) { received = append(received, "b") })
	bus.Emit(model.Event{Kind: "fact"})
	bus.Emit(model.Event{Kind: "fact"})
	if len(received) != 3 || received[0] != "a" || received[1] != "b" || received[2] != "a" {
		t.Fatal(received)
	}
}
