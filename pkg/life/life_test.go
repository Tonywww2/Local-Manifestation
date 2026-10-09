package life

import (
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestRemovingPenaltyCannotResetFourthKnockdown(t *testing.T) {
	p := model.Player{ID: "p", Life: "alive", MaxHP: 100, HP: 0}
	for i := 1; i <= 4; i++ {
		var err error
		p, err = Incapacitate(p)
		if err != nil {
			t.Fatal(err)
		}
		if p.Knockdowns != i {
			t.Fatal(p)
		}
		if i == 4 {
			if p.Life != "dead" || p.Downs != 0 {
				t.Fatal(p)
			}
			break
		}
		p, err = Raise(p)
		if err != nil {
			t.Fatal(err)
		}
		if p.Downs != 1 {
			t.Fatal(p)
		}
		p.Downs = 0 // Recovery facilities remove penalties, never history.
		p.HP = 0
	}
	if _, err := Raise(p); err == nil {
		t.Fatal("rescue revived death")
	}
}
func TestPenaltyIsLinearAndTransitionsDetached(t *testing.T) {
	for i, want := range []float64{1, .75, .5, .25} {
		if PenaltyFactor(i) != want {
			t.Fatal(i)
		}
	}
	p := model.Player{Life: "down", Knockdowns: 2, Downs: 1, HP: 0, MaxHP: 75, Statuses: model.StatusSet{Instances: []model.StatusInstance{{Definition: "test"}}}}
	next, err := Raise(p)
	if err != nil {
		t.Fatal(err)
	}
	next.Statuses.Instances[0].Definition = "changed"
	if p.Statuses.Instances[0].Definition != "test" || p.Life != "down" || next.Knockdowns != 2 || next.Downs != 2 {
		t.Fatal("mutation or lost history")
	}
}
