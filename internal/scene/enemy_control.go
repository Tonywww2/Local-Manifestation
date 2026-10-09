package scene

import "hsmultiplayer/pkg/model"

// EnemyAttackState is owned by the room simulation. AdvanceEnemyAttack is a
// pure transition; the owner performs movement and projectile creation.
type enemyAttackState struct {
	Index     int
	Phase     string
	Remaining float64
	Direction model.Vec
}
type enemyAttackStep struct {
	State    enemyAttackState
	Fire     bool
	Move     model.Vec
	Override bool
}

func advanceEnemyAttack(s enemyAttackState, patterns []model.EnemyAttackDef, toward model.Vec, distance, dt, tempo float64) enemyAttackStep {
	out := enemyAttackStep{State: s}
	if len(patterns) == 0 {
		return out
	}
	p := patterns[s.Index%len(patterns)]
	if s.Phase == "" {
		s.Phase = "approach"
		s.Remaining = p.Cooldown
	}
	s.Remaining -= dt * tempo
	switch s.Phase {
	case "approach":
		if s.Remaining <= 0 && distance <= p.Range {
			s.Direction = toward.Unit()
			s.Phase = "windup"
			s.Remaining = p.Windup
			out.Override = true
		}
	case "windup":
		out.Override = true
		if s.Remaining <= 0 {
			if p.Kind == "shoot" {
				out.Fire = true
				s.Phase = "recovery"
				s.Remaining = p.Recovery
			} else {
				s.Phase = "rush"
				s.Remaining = p.Active
				out.Move = s.Direction.Scale(p.Speed)
			}
		}
	case "rush":
		out.Override = true
		out.Move = s.Direction.Scale(p.Speed)
		if s.Remaining <= 0 {
			s.Phase = "recovery"
			s.Remaining = p.Recovery
			out.Move = model.Vec{}
		}
	case "recovery":
		out.Override = true
		if s.Remaining <= 0 {
			s.Index = (s.Index + 1) % len(patterns)
			s.Phase = "approach"
			s.Remaining = patterns[s.Index].Cooldown
		}
	}
	out.State = s
	return out
}
func hasSlow(d model.StatusDef) bool {
	for _, m := range d.Modifiers {
		if m.Stat == "speed" && m.Value < 0 {
			return true
		}
	}
	return false
}
