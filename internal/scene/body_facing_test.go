package scene

import (
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestWholeBodyCollisionAndMovementFacing(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	w := templateWorld(t, c, "base:skirmish")
	p := w.players["a"]
	p.player.Online, p.player.Scene = true, w.id
	p.input = model.Input{Move: model.Vec{Y: -1}, Aim: p.player.Position.Add(model.Vec{X: 200})}
	step := func(dt float64) {
		t.Helper()
		if err := w.stepPlayer(p, dt, nil); err != nil {
			t.Fatal(err)
		}
	}
	step(1.0 / 60)
	if p.visual.Facing.Y > -.99 {
		t.Fatal("walking follows idle cursor instead of movement", p.visual)
	}
	p.input.Attack = true
	p.input.Aim = p.player.Position.Add(model.Vec{X: 200})
	step(1.0 / 60)
	if p.visual.Facing.X < .99 || !animation.Active(p.visual.Attack) {
		t.Fatal("attack does not face accepted aim", p.visual)
	}
	p.input.Attack = false
	p.input.Move = model.Vec{X: -1}
	p.input.Aim = p.player.Position.Add(model.Vec{Y: 200})
	step(1.0 / 60)
	if p.visual.Facing.X < .99 {
		t.Fatal("moving or cursor turned an active attack", p.visual)
	}
	step(p.visual.Attack.Duration)
	if p.visual.Facing.X > -.99 {
		t.Fatal("movement facing did not resume after recovery", p.visual)
	}
	p.input.Move = model.Vec{}
	step(1.0 / 60)
	if p.visual.Facing.X > -.99 {
		t.Fatal("idle cursor changed the last facing", p.visual)
	}

	def := c.Enemy("base:stalker")
	center := model.Vec{X: w.room.Width/2 + 150, Y: w.room.Height / 2}
	w.enemies = []model.Enemy{{ID: 1, Definition: def.ID, Position: center, Radius: def.Radius, HP: def.HP, MaxHP: def.HP}}
	// These trajectories miss the previous foot circle but intersect the body.
	near := center.Add(model.Vec{Y: -27})
	if w.canMove(center.Add(model.Vec{Y: -65}), near) {
		t.Fatal("walking entered the visible body")
	}
	if !w.canMove(near, near.Add(model.Vec{Y: -5})) {
		t.Fatal("existing overlap trapped the player")
	}
	w.shots = []shot{{view: model.Projectile{ID: 1, Owner: "a", Position: center.Add(model.Vec{X: -60, Y: -22}), Velocity: model.Vec{X: 1200}, TTL: 1}, weapon: model.Weapon{ID: "body-check", Type: "physical", Base: 20}, multiplier: 1, remaining: 1}}
	if err := w.updateShots(.1); err != nil {
		t.Fatal(err)
	}
	if w.enemies[0].HP >= def.HP {
		t.Fatal("projectile crossing the upper body missed")
	}
}
