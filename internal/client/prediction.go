package client

import (
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"math"
	"time"
)

// MovementPrediction is display state only. It cannot open doors, spend AP,
// alter health or replace server snapshots. Other entities keep interpolation.
type MovementPrediction struct {
	DodgeTime  float64
	DodgeDir   model.Vec
	Node       string
	Epoch      uint64
	Position   model.Vec
	Correction model.Vec
	At         time.Time
	Valid      bool
}

// Called once per input edge. It predicts motion only; the server owns charges.
func (p MovementPrediction) BeginDodge(in model.Input, player model.Player) MovementPrediction {
	if !p.Valid || !in.Dodge || player.Dodges < 1 || player.Life != "alive" || p.DodgeTime > 0 || status.Controlled(player.Statuses, "root") || status.Controlled(player.Statuses, "stun") {
		return p
	}
	p.DodgeTime = .18
	p.DodgeDir = in.Move.Unit()
	if p.DodgeDir.Length() == 0 {
		p.DodgeDir = in.Aim.Sub(p.Position).Unit()
	}
	return p
}

func (p MovementPrediction) Rebase(from, to model.Node, a, b geometry.Arena) MovementPrediction {
	if !p.Valid || p.Node != from.ID {
		return p
	}
	p.Node = to.ID
	p.Position = geometry.TransferPoint(from, to, a, b, p.Position)
	p.Correction = p.Correction.Scale(b.Radius / a.Radius)
	return p
}

func (p MovementPrediction) Reconcile(player model.Player, a geometry.Arena, at time.Time, open ...int) MovementPrediction {
	if !p.Valid || p.Node != player.Node || p.Epoch != player.PositionEpoch || p.Position.Sub(player.Position).Length() > 96 || player.Life != "alive" {
		return MovementPrediction{Node: player.Node, Epoch: player.PositionEpoch, Position: a.ClampWalk(player.Position, 12, open, 36), At: at, Valid: true}
	}
	p.Correction = player.Position.Sub(p.Position)
	return p
}
func (p MovementPrediction) Advance(in model.Input, speed float64, a geometry.Arena, now time.Time, movable bool, open ...int) MovementPrediction {
	if !p.Valid {
		return p
	}
	dt := min(.05, max(0, now.Sub(p.At).Seconds()))
	p.At = now
	if !movable {
		in = model.Input{}
		p.DodgeTime = 0
	}
	// Time-based presentation makes distance independent of rendered frame count.
	move := in.Move.Unit().Scale(dt)
	if p.DodgeTime > 0 {
		dodgeStep := min(dt, p.DodgeTime)
		move = p.DodgeDir.Scale(3 * dodgeStep).Add(in.Move.Unit().Scale(dt - dodgeStep))
		p.DodgeTime = max(0, p.DodgeTime-dt)
	}
	p.Position = a.ClampWalk(p.Position.Add(move.Scale(speed)), 12, open, 36)
	correction := p.Correction.Scale(1 - math.Exp(-dt/0.08))
	p.Position = a.ClampWalk(p.Position.Add(correction), 12, open, 36)
	p.Correction = p.Correction.Sub(correction)
	return p
}
