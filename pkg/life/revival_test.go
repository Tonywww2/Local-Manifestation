package life

import (
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestResurrectionStartsAtOneAndDiesOnThirdSubsequentKnockdown(t *testing.T) {
	p := model.Player{ID: "p", Life: "dead", Knockdowns: 4, Downs: 3, AP: 2, HP: 0, MaxHP: 25}
	n, err := Revive(p)
	if err != nil {
		t.Fatal(err)
	}
	if n.Knockdowns-n.KnockdownBase != 1 || n.Knockdowns != 4 || n.AP != 2 || p.Life != "dead" {
		t.Fatal("resurrection lost counter, history or AP")
	}
	for i := 1; i <= 3; i++ {
		n.HP = 0
		n, err = Incapacitate(n)
		if err != nil {
			t.Fatal(err)
		}
		if i < 3 {
			if n.Life != "down" {
				t.Fatal("died too early")
			}
			n, err = Raise(n)
			if err != nil {
				t.Fatal(err)
			}
		} else if n.Life != "dead" || n.Knockdowns != 7 {
			t.Fatal("did not die at the confirmed boundary")
		}
	}
}
