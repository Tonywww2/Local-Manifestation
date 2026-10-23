// Package animation samples value timelines without clocks or rendering dependencies.
package animation

import "hsmultiplayer/pkg/model"

func AdvanceAction(a model.ActionAnimation, dt float64) model.ActionAnimation {
	if a.Duration > 0 {
		a.Elapsed = max(0, a.Elapsed+dt)
	}
	return a
}

func Advance(p model.PlayerActionState, dt float64) model.PlayerActionState {
	p.Attack = AdvanceAction(p.Attack, dt)
	p.Skill = AdvanceAction(p.Skill, dt)
	p.Dodge = AdvanceAction(p.Dodge, dt)
	p.Hit = AdvanceAction(p.Hit, dt)
	p.Life = AdvanceAction(p.Life, dt)
	p.Walk += p.Move.Length() * max(0, dt) / 72
	return p
}

func Active(a model.ActionAnimation) bool {
	return a.Duration > 0 && a.Elapsed >= 0 && a.Elapsed < a.Duration
}

func Cancel(p model.PlayerActionState) model.PlayerActionState {
	p.Attack.Duration, p.Skill.Duration, p.Dodge.Duration = 0, 0, 0
	p.Hit.Duration = 0
	p.Move = model.Vec{}
	return p
}

// Elapsed remains uncapped after completion so a delayed display sample cannot
// replay an expired action. Before a new action's birth, retain the prior one.
func sampleAction(a, b model.ActionAnimation, offset, span float64) model.ActionAnimation {
	if b.Sequence == a.Sequence {
		if span <= 0 {
			return b
		}
		b.Elapsed = a.Elapsed + (b.Elapsed-a.Elapsed)*max(0, min(1, offset/span))
		return b
	}
	if b.Duration > 0 && b.Elapsed >= span-offset-1e-9 {
		return AdvanceAction(b, offset-span)
	}
	return AdvanceAction(a, max(0, offset))
}

func Sample(a, b model.PlayerActionState, offset, span float64) model.PlayerActionState {
	out := b
	alpha := 1.0
	if span > 0 {
		alpha = max(0, min(1, offset/span))
	}
	if a.Facing.Length() > 0 && b.Facing.Length() > 0 {
		facing := a.Facing.Scale(1 - alpha).Add(b.Facing.Scale(alpha))
		if facing.Length() > 1e-6 {
			out.Facing = facing.Unit()
		}
	}
	out.Attack = sampleAction(a.Attack, b.Attack, offset, span)
	out.Skill = sampleAction(a.Skill, b.Skill, offset, span)
	out.Dodge = sampleAction(a.Dodge, b.Dodge, offset, span)
	out.Hit = sampleAction(a.Hit, b.Hit, offset, span)
	out.Life = sampleAction(a.Life, b.Life, offset, span)
	out.Move = a.Move.Scale(1 - alpha).Add(b.Move.Scale(alpha))
	out.Walk = a.Walk + (b.Walk-a.Walk)*alpha
	return out
}

func AdvanceEnemy(p model.EnemyActionState, dt float64) model.EnemyActionState {
	p.Attack = AdvanceAction(p.Attack, dt)
	p.Hit = AdvanceAction(p.Hit, dt)
	p.Death = AdvanceAction(p.Death, dt)
	p.Walk += p.Move.Length() * max(0, dt) / 72
	return p
}

func SampleEnemy(a, b model.EnemyActionState, offset, span float64) model.EnemyActionState {
	alpha := 1.0
	if span > 0 {
		alpha = max(0, min(1, offset/span))
	}
	out := b
	out.Position = a.Position.Add(b.Position.Sub(a.Position).Scale(alpha))
	out.Facing = a.Facing.Scale(1 - alpha).Add(b.Facing.Scale(alpha)).Unit()
	out.Move = a.Move.Scale(1 - alpha).Add(b.Move.Scale(alpha))
	out.Walk = a.Walk + (b.Walk-a.Walk)*alpha
	out.Attack = sampleAction(a.Attack, b.Attack, offset, span)
	out.Hit = sampleAction(a.Hit, b.Hit, offset, span)
	out.Death = sampleAction(a.Death, b.Death, offset, span)
	return out
}
