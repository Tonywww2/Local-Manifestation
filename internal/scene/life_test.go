package scene

import (
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestFourthContactKnockdownIgnoresRemovedPenaltyAndCountsOnce(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	w := templateWorld(t, c, "base:skirmish")
	w.state = "fighting"
	w.wave = 1
	w.waves = 1
	p := w.players["a"]
	p.player.Knockdowns = 3
	p.player.Downs = 0
	p.player.HP = 1
	p.player.Position = geometry.NewArena(w.room.Width, w.room.Height).Center
	w.enemies = []model.Enemy{{ID: 1, Definition: "base:stalker", HP: 1000, Radius: 13, Position: p.player.Position}}
	w.step(1.0 / 30)
	if p.player.Life != "dead" || p.player.Knockdowns != 4 || p.player.Downs != 0 {
		t.Fatal(p.player)
	}
	w.step(1.0 / 30)
	if p.player.Knockdowns != 4 {
		t.Fatal("duplicate history for incapacitated actor")
	}
}
