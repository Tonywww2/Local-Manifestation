package damage

import (
	"hsmultiplayer/pkg/model"
	"math"
	"testing"
)

func TestNamedBucketsAndDefenseOrder(t *testing.T) {
	q := Request{Weapon: model.Weapon{ID: "test", Type: "physical", Base: 100, Flat: 10, Modifiers: []model.Modifier{{Group: "attribute", Percent: .2}, {Group: "attribute", Percent: .3}, {Group: "accessory", Percent: .3}}, FinalFlat: 5}, Critical: true, CritMultiplier: 1.5, FinalMultiplier: 1, Defense: Defense{Armor: 20, Reductions: []float64{.25, .2}}}
	r, err := Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	// ((110 * 1.3 * 1.5 + 5) * 1.5 - 20) * .75 * .8
	if r.Total != 185.55 {
		t.Fatalf("got %v", r.Total)
	}
	if len(r.Steps) < 10 {
		t.Fatal("missing explanation")
	}
}
func TestMinimumImmunityAndIndependentSegments(t *testing.T) {
	q := Request{Weapon: model.Weapon{Base: 10}, FinalMultiplier: 1, Defense: Defense{Armor: 99}}
	r, _ := Evaluate(q)
	if r.Total != 1 {
		t.Fatal(r)
	}
	q.Defense.Immune = true
	r, _ = Evaluate(q)
	if r.Total != 0 {
		t.Fatal(r)
	}
	q.Defense = Defense{Armor: 6}
	r, _ = Evaluate(q)
	if 2*r.Total != 8 {
		t.Fatal("each segment must pay armor")
	}
	q.Defense.IgnoreArmor = true
	r, _ = Evaluate(q)
	if r.Total != 10 {
		t.Fatal(r)
	}
}
func TestRejectNonFinite(t *testing.T) {
	if _, err := Evaluate(Request{Weapon: model.Weapon{Base: math.NaN()}}); err == nil {
		t.Fatal("accepted NaN")
	}
}

func TestAttributeContributionsExplainExactWeights(t *testing.T) {
	r, err := Evaluate(Request{Weapon: model.Weapon{Base: 8, Scaling: model.Attributes{Strength: .25, Agility: 1}}, Attributes: model.Attributes{Strength: 8, Agility: 12}, FinalMultiplier: 1})
	if err != nil || r.Total != 22 {
		t.Fatal(r, err)
	}
	if r.Steps[1].Stat != "strength" || r.Steps[1].Score != 8 || r.Steps[1].Weight != .25 || r.Steps[1].Operand != 2 {
		t.Fatal("missing exact attribute calculation", r.Steps)
	}
}

func TestGroupMembersRetainIndividualSourceIdentity(t *testing.T) {
	a := model.SourceRef{Kind: "item", Actor: "p", Instance: "ring-a", Definition: "test:ring"}
	b := a
	b.Instance = "ring-b"
	r, err := Evaluate(Request{Weapon: model.Weapon{Base: 100, Modifiers: []model.Modifier{{Group: "gear", Source: "ring", Percent: .2, Origin: a}, {Group: "gear", Source: "ring", Percent: .3, Origin: b}}}, FlatBonuses: []Bonus{{ID: "upgrade", Value: 10, Origin: model.SourceRef{Kind: "weapon_upgrade", Actor: "p", Definition: "test:weapon"}}}, FinalMultiplier: 1})
	if err != nil || r.Total != 165 {
		t.Fatal(r, err)
	}
	for _, s := range r.Steps {
		if s.Group == "gear" && (len(s.Members) != 2 || s.Members[0].Origin == s.Members[1].Origin) {
			t.Fatal("lost item identity", s)
		}
	}
	c := CloneResult(r)
	for i := range c.Steps {
		if len(c.Steps[i].Members) > 0 {
			c.Steps[i].Members[0].Percent = 99
		}
	}
	for _, s := range r.Steps {
		if len(s.Members) > 0 && s.Members[0].Percent != .2 {
			t.Fatal("result shares nested members")
		}
	}
}
