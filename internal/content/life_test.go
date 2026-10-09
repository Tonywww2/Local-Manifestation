package content_test

import (
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestRescuePenaltyRecoveryAndLayerPreparationPreserveHistory(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	p, err := content.Player(c, "p", "p", "base:scout")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		p.HP = 0
		p, err = life.Incapacitate(p)
		if err != nil {
			t.Fatal(err)
		}
		p, err = content.RescuePlayer(p)
		if err != nil {
			t.Fatal(err)
		}
		if p.Knockdowns != i || p.Downs != i || p.HP != 120*life.PenaltyFactor(i) || p.MaxHP != p.HP {
			t.Fatal(p)
		}
	}
	p.HP = p.MaxHP / 2
	next, err := content.ReducePenalty(p, 3)
	if err != nil {
		t.Fatal(err)
	}
	if next.Downs != 0 || next.Knockdowns != 3 || next.HP != 60 || p.Downs != 3 {
		t.Fatal(next)
	}
	next, err = content.PrepareLayerPlayer(next)
	if err != nil || next.HP != 120 || next.Knockdowns != 3 {
		t.Fatal(next, err)
	}
	next.HP = 0
	next, err = life.Incapacitate(next)
	if err != nil || next.Life != "dead" {
		t.Fatal(next, err)
	}
	if _, err = content.RescuePlayer(next); err == nil {
		t.Fatal("ordinary rescue revived dead player")
	}
	if err = life.Validate(next); err != nil {
		t.Fatal(err)
	}
	q, err := content.Attack(c, model.Player{ID: p.ID, Chassis: p.Chassis, BaseStats: p.BaseStats, Weapon: p.Weapon, Downs: 2})
	if err != nil || q.FinalMultiplier != .5 || q.FinalSource.Definition != "engine:down_penalty" {
		t.Fatal(q, err)
	}
}
