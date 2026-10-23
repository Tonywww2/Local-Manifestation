package damage

import (
	"reflect"
	"testing"

	"hsmultiplayer/pkg/model"
)

func TestOutgoingPreviewExcludesTargetSettlement(t *testing.T) {
	q := Request{
		Weapon: model.Weapon{ID: "preview:weapon", Type: "physical", Base: 100, Flat: 10, FinalFlat: 5,
			Scaling:          model.Attributes{Strength: 1, Agility: .5},
			Modifiers:        []model.Modifier{{Group: "gear", Percent: .2}, {Group: "gear", Percent: .3}},
			AdditionalDamage: []model.DamagePart{{ID: "fire", Type: "fire", Base: .1}},
		},
		Attributes:      model.Attributes{Strength: 10, Agility: 4},
		Critical:        true,
		CritMultiplier:  1.5,
		FinalMultiplier: .75,
		Defense:         Defense{Armor: 20, Resistance: .2, Reductions: []float64{.25}},
		TypedDefense:    map[string]Defense{"fire": {Immune: true}},
	}
	before := CloneRequest(q)
	preview, err := EvaluateOutgoing(q)
	if err != nil || preview.Total != 211.61 || len(preview.Segments) != 2 {
		t.Fatalf("player calculation: %+v, %v", preview, err)
	}
	if preview.Segments[1].Damage.Total != .11 {
		t.Fatal("outgoing segment acquired a minimum-damage floor")
	}
	for _, segment := range preview.Segments {
		for _, step := range segment.Damage.Steps {
			if step.Operation == "subtract" || step.Operation == "minimum" || step.Operation == "override" || step.Source == "resistance" {
				t.Fatal("target step in outgoing formula", step)
			}
		}
		if _, err := Formula(segment.Damage); err != nil {
			t.Fatal("outgoing steps cannot build a formula", err)
		}
	}
	if step := preview.Segments[0].Damage.Steps[1]; step.Stat != "strength" || step.Score != 10 || step.Weight != 1 {
		t.Fatal("lost attribute explanation", step)
	}
	actual, err := Evaluate(q)
	if err != nil || actual.Total != 114.9 || actual.Segments[1].Damage.Total != 0 {
		t.Fatalf("target settlement changed: %+v, %v", actual, err)
	}
	if !reflect.DeepEqual(q, before) {
		t.Fatal("preview mutated its input")
	}
}
