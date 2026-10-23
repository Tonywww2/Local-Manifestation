package scene

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"math"
)

// Both battle actors and coordinator-owned spaces use the same fixed-step actions.
func (w *world) stepPlayer(p *fighter, dt float64, open []int) error {
	p.syncVisualLife()
	if p.player.Life != "alive" {
		p.attack, p.dodgeTime, p.invulnerable = nil, 0, 0
		p.visual = animation.Cancel(p.visual)
		return nil
	}
	p.cooldown = math.Max(0, p.cooldown-dt)
	p.invulnerable = math.Max(0, p.invulnerable-dt)
	if w.tick-p.updated > uint64(w.rate()/2) || !p.player.Online {
		p.input = model.Input{}
	}
	if p.player.Dodges < 2 {
		p.recharge += dt
		if p.recharge >= 2-1e-9 {
			p.player.Dodges++
			p.recharge = max(0, p.recharge-2)
		}
	}
	move := p.input.Move.Unit()
	if status.Controlled(p.player.Statuses, "stun") || status.Controlled(p.player.Statuses, "root") {
		move = model.Vec{}
	}
	if p.input.Dodge && p.player.Dodges > 0 && p.dodgeTime <= 0 {
		if status.Controlled(p.player.Statuses, "root") || status.Controlled(p.player.Statuses, "stun") {
			p.input.Dodge = false
		}
	}
	if p.input.Dodge && p.player.Dodges > 0 && p.dodgeTime <= 0 {
		next, err := content.TriggerDodge(w.catalog, p.player)
		if err != nil {
			return err
		}
		p.player = next
		p.player.Dodges--
		p.dodgeTime = .18
		p.invulnerable = .18
		p.dodgeDir = move
		if move.Length() == 0 {
			p.dodgeDir = p.input.Aim.Sub(p.player.Position).Unit()
		}
		p.visual.Dodge = model.ActionAnimation{Sequence: p.visual.Dodge.Sequence + 1, Direction: p.dodgeDir, Duration: p.dodgeTime}
		p.input.Dodge = false
	}
	if p.dodgeTime > 0 {
		p.dodgeTime -= dt
		move = p.dodgeDir.Scale(3)
	}
	if status.Controlled(p.player.Statuses, "root") || status.Controlled(p.player.Statuses, "stun") {
		move = model.Vec{}
	}
	before := p.player.Position
	candidate := w.clamp(p.player.Position.Add(move.Scale(p.player.Speed*dt)), 12)
	if len(open) > 0 {
		candidate = w.arenaGeometry().ClampWalk(p.player.Position.Add(move.Scale(p.player.Speed*dt)), 12, open, p.player.Speed*dt*move.Length())
	}
	if w.clearMove(p.player.Position, candidate, 12) && (p.dodgeTime > 0 || w.canMove(p.player.Position, candidate)) {
		p.player.Position = candidate
	}
	p.visual.Move = model.Vec{}
	if dt > 0 {
		p.visual.Move = p.player.Position.Sub(before).Scale(1 / dt)
	}
	// Exact multiples of a fixed step can leave a tiny positive float remainder.
	if p.input.Attack && !status.Controlled(p.player.Statuses, "stun") && p.player.Weapon != "" && p.cooldown <= 1e-9 && p.attack == nil {
		if err := w.beginAttack(p); err != nil {
			return err
		}
	}
	w.releaseAttack(p, dt)
	p.visual = animation.Advance(p.visual, dt)
	aim := model.Vec{}
	if p.input.Aim != (model.Vec{}) && p.input.Aim.Finite() {
		aim = p.input.Aim.Sub(p.player.Position)
	}
	p.visual.Facing = animation.PlayerFacing(p.visual, p.visual.Move, aim, p.input.Attack && p.player.Weapon != "" && !status.Controlled(p.player.Statuses, "stun"))
	return nil
}
