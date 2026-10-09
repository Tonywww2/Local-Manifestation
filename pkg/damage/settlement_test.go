package damage

import (
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"testing"
)

func TestTypedSegmentsShareCritAndSettleArmorShieldOverkill(t *testing.T) {
	s := model.StatusSet{}
	var err error
	for i, capacity := range []float64{7, 8} {
		s, err = status.ApplySnapshot(s, model.StatusDef{ID: "check:shield", Name: "Shield", Shield: capacity, MaxLayers: 1, DurationMillis: 10000}, model.SourceRef{Kind: "item", Actor: "p", Definition: "check:item", Instance: string(rune('a' + i))}, 0, model.Attributes{})
		if err != nil {
			t.Fatal(err)
		}
	}
	r, err := Evaluate(Request{Weapon: model.Weapon{ID: "check:weapon", Type: "physical", Base: 20, AdditionalDamage: []model.DamagePart{{ID: "flame", Type: "fire", Base: 10}}}, Critical: true, CritMultiplier: 1.5, FinalMultiplier: 1, Defense: Defense{Armor: 5}})
	if err != nil {
		t.Fatal(err)
	}
	settled, hp, left, err := Settle(r, 12, s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 35 || hp != 0 || settled.Absorbed != 15 || settled.HealthLoss != 12 || settled.Overkill != 8 || len(settled.Segments) != 2 || settled.Segments[0].Shields[0].Amount != 8 || settled.Segments[1].HealthLoss != 2 {
		t.Fatalf("unexpected typed settlement: %+v", settled)
	}
	if status.ShieldTotal(s) != 15 || status.ShieldTotal(left) != 0 {
		t.Fatal("settlement mutated the input or did not consume shields")
	}
}
