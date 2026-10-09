package scene

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"math"
)

// Both battle actors and coordinator-owned spaces use the same fixed-step actions.
func (w *world) stepPlayer(p *fighter, dt float64, open []int) error {
	if p.player.Life != "alive" {
		p.attack, p.dodgeTime, p.invulnerable = nil, 0, 0
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
		p.input.Dodge = false
	}
	if p.dodgeTime > 0 {
		p.dodgeTime -= dt
		move = p.dodgeDir.Scale(3)
	}
	if status.Controlled(p.player.Statuses, "root") || status.Controlled(p.player.Statuses, "stun") {
		move = model.Vec{}
	}
	candidate := w.clamp(p.player.Position.Add(move.Scale(p.player.Speed*dt)), 12)
	if len(open) > 0 {
		candidate = geometry.NewArena(w.room.Width, w.room.Height).ClampWalk(p.player.Position.Add(move.Scale(p.player.Speed*dt)), 12, open, p.player.Speed*dt*move.Length())
	}
	if w.clearObstacles(p.player.Position, candidate, 12) && (p.dodgeTime > 0 || w.canMove(p.player.Position, candidate)) {
		p.player.Position = candidate
	}
	// Exact multiples of a fixed step can leave a tiny positive float remainder.
	if p.input.Attack && !status.Controlled(p.player.Statuses, "stun") && p.player.Weapon != "" && p.cooldown <= 1e-9 && p.attack == nil {
		if err := w.beginAttack(p); err != nil {
			return err
		}
	}
	w.releaseAttack(p, dt)
	return nil
}
