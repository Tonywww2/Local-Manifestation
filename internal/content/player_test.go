package content_test

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/sdk"
	"reflect"
	"testing"
)

func TestGrowthDamageSnapshotAndHealthRatio(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		d.Chassis[0].Modifiers = []model.StatModifier{{ID: "base:strong", Stat: "strength", Operation: "percent", Group: "chassis", Value: .2}}
	}, "")
	p, err := content.Player(c, "p", "p", c.ChassisList()[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	p.HP = p.MaxHP / 2
	before := model.ClonePlayer(p)
	q, err := content.Attack(c, p)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := damage.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	next, err := c.GrantExperience(p, 20)
	if err != nil || next.Level != 2 || next.Attributes.Strength != (p.BaseStats.Attributes.Strength+1)*1.2 || !reflect.DeepEqual(before, p) {
		t.Fatal(next, err)
	}
	after, _ := damage.Evaluate(q)
	if !reflect.DeepEqual(initial, after) {
		t.Fatal("old attack snapshot changed")
	}
	grown, err := content.ApplyFloorJoinGrowth(next, 3)
	if err != nil || grown.HP != grown.MaxHP/2 {
		t.Fatal("growth healed or changed health ratio", grown, err)
	}
	for _, s := range initial.Steps {
		if s.Stat == "strength" && len(s.AttributeSteps) < 2 {
			t.Fatal("missing attribute sources")
		}
	}
	if bad, err := c.GrantExperience(p, -1); err == nil || bad.ID != "" || !reflect.DeepEqual(before, p) {
		t.Fatal("failed experience grant committed")
	}
	next.StatContributions[0].Value = 99
	if p.StatContributions[0].Value != .2 {
		t.Fatal("player mutation escaped")
	}
}

func TestRecoveryPreservesRatioBeforeExplicitHealing(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	p, err := content.Player(c, "p", "p", "base:scout")
	if err != nil {
		t.Fatal(err)
	}
	p.Downs = 1
	p.MaxHP = content.MaxHP(p)
	p.HP = p.MaxHP / 2
	next, err := content.ApplyCommands(c, map[string]model.Player{"p": p}, []sdk.Command{{Kind: "remove_penalty", Target: "p", Amount: 1}}, content.NodeCommands)
	if err != nil || next["p"].HP != 60 || next["p"].MaxHP != 120 || p.HP != 45 {
		t.Fatal(next, err)
	}
	next, err = content.ApplyCommands(c, map[string]model.Player{"p": p}, []sdk.Command{{Kind: "remove_penalty", Target: "p", Amount: 1}, {Kind: "heal_full", Target: "p"}}, content.NodeCommands)
	if err != nil || next["p"].HP != 120 {
		t.Fatal("explicit healing missing", next, err)
	}
}
