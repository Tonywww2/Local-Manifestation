package stats

import (
	"hsmultiplayer/pkg/model"
	"math"
	"reflect"
	"testing"
)

func contribution(id, stat, operation, group string, value float64) model.StatContribution {
	return model.StatContribution{StatModifier: model.StatModifier{ID: id, Stat: stat, Operation: operation, Group: group, Value: value}, Source: model.SourceRef{Kind: "item", Actor: "p", Instance: id, Definition: "test:item"}}
}
func TestOrderedGroupsExplainEverySourceAndReevaluateRemoval(t *testing.T) {
	base := model.StatValues{MaxHP: 100, Speed: 200, Attributes: model.Attributes{Strength: 100}}
	cs := []model.StatContribution{contribution("c", "strength", "percent", "accessory", .3), contribution("d", "strength", "final_add", "", 5), contribution("a", "strength", "add", "", 10), contribution("b1", "strength", "percent", "attribute", .2), contribution("b2", "strength", "percent", "attribute", .3)}
	r, err := Evaluate(Input{Base: base, Contributions: cs})
	if err != nil || r.Values.Attributes.Strength != 219.5 {
		t.Fatal(r.Values, err)
	}
	reversed := append([]model.StatContribution(nil), cs...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	other, err := Evaluate(Input{Base: base, Contributions: reversed})
	if err != nil || !reflect.DeepEqual(r, other) {
		t.Fatal("order affected evaluation")
	}
	for _, s := range r.Steps {
		if s.Group == "attribute" && (len(s.Contributions) != 2 || s.Operand != 1.5) {
			t.Fatal("collapsed source identities", s)
		}
	}
	r.Steps[2].Contributions[0].Value = 99
	if cs[0].Value != .3 {
		t.Fatal("input mutated")
	}
	removed, err := Evaluate(Input{Base: base, Contributions: cs[1:]})
	if err != nil || removed.Values.Attributes.Strength != 170 {
		t.Fatal("removal was not recomputed", removed.Values, err)
	}
}
func TestInstanceIdentityAndAtomicValidation(t *testing.T) {
	c := contribution("same", "speed", "add", "", 10)
	second := c
	second.Source.Instance = "other"
	cs := []model.StatContribution{c, second}
	if err := Validate(cs); err != nil {
		t.Fatal(err)
	}
	updated := c
	updated.Value = 20
	next, err := Upsert(cs, updated)
	if err != nil || len(next) != 2 || next[0].Value != 20 || cs[0].Value != 10 {
		t.Fatal(next, err)
	}
	for _, bad := range []model.StatContribution{contribution("x", "unknown", "add", "", 1), contribution("x", "speed", "percent", "", .2), contribution("x", "speed", "add", "group", 1), contribution("x", "speed", "add", "", math.NaN())} {
		if next, err := Upsert(cs, bad); err == nil || next != nil || cs[0].Value != 10 {
			t.Fatal("bad transaction committed", next, err)
		}
	}
	if err := Validate([]model.StatContribution{c, c}); err == nil {
		t.Fatal("duplicate identity")
	}
}
func TestClampAndHealthRatio(t *testing.T) {
	r, err := Evaluate(Input{Base: model.StatValues{MaxHP: 100, Speed: 200}, Contributions: []model.StatContribution{contribution("hp", "max_hp", "add", "", -1000), contribution("speed", "speed", "add", "", -300)}})
	if err != nil || r.Values.MaxHP != 1 || r.Values.Speed != 0 {
		t.Fatal(r, err)
	}
	for _, v := range [][4]float64{{50, 100, 200, 100}, {50, 100, 60, 30}, {0, 100, 200, 0}, {33.33, 100, 200, 66.66}} {
		hp, err := ResizeHealth(v[0], v[1], v[2])
		if err != nil || hp != v[3] {
			t.Fatal(v, hp, err)
		}
	}
	if _, err := ResizeHealth(101, 100, 200); err == nil {
		t.Fatal("invalid old health")
	}
}
func BenchmarkEvaluateContributions(b *testing.B) {
	q := Input{Base: model.StatValues{MaxHP: 120, Speed: 200, Attributes: model.Attributes{Strength: 8, Agility: 12}}}
	for i := 0; i < 64; i++ {
		c := contribution(string(rune('a'+i)), "strength", "percent", "gear", .01)
		q.Contributions = append(q.Contributions, c)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Evaluate(q); err != nil {
			b.Fatal(err)
		}
	}
}
