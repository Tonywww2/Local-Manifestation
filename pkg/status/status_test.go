package status

import (
	"hsmultiplayer/pkg/model"
	"reflect"
	"testing"
)

func definition() model.StatusDef {
	return model.StatusDef{ID: "base:power", Name: "Power", Level: 2, MaxLayers: 2, DurationMillis: 2000, Modifiers: []model.StatModifier{{ID: "base:strength", Stat: "strength", Operation: "add", Value: 3}}}
}
func source(actor, instance string) model.SourceRef {
	return model.SourceRef{Kind: "skill", Actor: actor, Instance: instance, Definition: "base:skill"}
}
func apply(t *testing.T, s model.StatusSet, d model.StatusDef, ref model.SourceRef, now int64) model.StatusSet {
	t.Helper()
	next, err := Apply(s, d, ref, now)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestIndependentSourcesLayerExpiryRefreshAndDetachedSnapshots(t *testing.T) {
	d := definition()
	s := apply(t, model.StatusSet{}, d, source("a", "one"), 0)
	s = apply(t, s, d, source("a", "one"), UnitsPerSecond/2)
	first, highest := s.Instances[0].Layers[0], s.Instances[0].Layers[1]
	s = apply(t, s, d, source("a", "one"), UnitsPerSecond)
	if !reflect.DeepEqual(s.Instances[0].Layers[0], first) || s.Instances[0].Layers[1].ID != highest.ID || s.Instances[0].Layers[1].Expires != 3*UnitsPerSecond {
		t.Fatal("did not refresh only highest layer")
	}
	s = apply(t, s, d, source("b", "one"), UnitsPerSecond)
	s = apply(t, s, d, source("a", "two"), UnitsPerSecond)
	if len(s.Instances) != 3 {
		t.Fatal("merged separate sources")
	}
	d.Modifiers[0].Value = 99
	if s.Instances[0].Layers[0].Modifiers[0].Value != 3 {
		t.Fatal("definition mutated snapshot")
	}
	old := model.CloneStatuses(s)
	next, changed, err := Advance(s, 2*UnitsPerSecond)
	if err != nil || !changed || len(next.Instances[0].Layers) != 1 || next.Instances[0].Layers[0].ID != highest.ID || !reflect.DeepEqual(s, old) {
		t.Fatal("expiry mutated input or lost independent timer")
	}
	copy := model.CloneStatuses(next)
	copy.Instances[0].Layers[0].Modifiers[0].Value = 88
	if next.Instances[0].Layers[0].Modifiers[0].Value != 3 {
		t.Fatal("shallow status copy")
	}
}

func TestUniqueRejectionConflictReplacementAndBoundLifetimes(t *testing.T) {
	d := definition()
	d.Unique = true
	s := apply(t, model.StatusSet{}, d, source("a", "one"), 0)
	old := model.CloneStatuses(s)
	if _, err := Apply(s, d, source("b", "two"), 1); err == nil || !reflect.DeepEqual(s, old) {
		t.Fatal("unique application was not rejected atomically")
	}
	other := definition()
	other.ID = "base:other"
	other.Conflicts = []string{d.ID}
	other.ClearOnLeave = true
	s = apply(t, s, other, source("a", "other"), 2)
	if len(s.Instances) != 1 || s.Instances[0].Definition != other.ID {
		t.Fatal("conflict did not replace old state")
	}
	s, changed := RemoveLeaving(s)
	if !changed || len(s.Instances) != 0 {
		t.Fatal("explicit leave lifetime")
	}
	d.Unique = false
	d.BoundToSource = true
	s = apply(t, s, d, source("a", "bound"), 3)
	unbound := definition()
	unbound.ID = "base:unbound"
	s = apply(t, s, unbound, source("a", "bound"), 3)
	s, changed = RemoveBoundSource(s, source("a", "bound"))
	if !changed || len(s.Instances) != 1 || s.Instances[0].Definition != unbound.ID {
		t.Fatal("unbound applied status did not survive source removal")
	}
	if _, _, err := Advance(s, 2); err == nil {
		t.Fatal("backwards time")
	}
}
