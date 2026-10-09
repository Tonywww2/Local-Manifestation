package scene

import (
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestCombatUsesHexBoundsForBodiesAndProjectiles(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	w := templateWorld(t, c, "base:skirmish")
	a := geometry.NewArena(w.room.Width, w.room.Height)
	if a.ContainsDisk(model.Vec{X: 1, Y: 1}, 0) || !a.ContainsDisk(w.clamp(model.Vec{}, 12), 12) {
		t.Fatal("rectangle corner accepted")
	}
	w.spawn()
	w.state = "fighting"
	for _, e := range w.enemies {
		if !a.ContainsDisk(e.Position, e.Radius) {
			t.Fatal("enemy spawned outside hex", e)
		}
	}
	for _, p := range w.players {
		p.player.Online = true
		p.updated = w.tick
		p.input = model.Input{Move: model.Vec{X: -1, Y: -1}}
	}
	w.shots = []shot{{view: model.Projectile{Position: a.Center, Velocity: model.Vec{X: 10000, Y: -10000}, TTL: 2}}}
	w.step(.1)
	if len(w.shots) != 0 {
		t.Fatal("projectile survived beyond hex border")
	}
	for _, p := range w.players {
		if !a.ContainsDisk(p.player.Position, 12) {
			t.Fatal("player outside hex")
		}
	}
}
