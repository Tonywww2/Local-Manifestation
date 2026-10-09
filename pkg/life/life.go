// Package life separates irreversible knockdown history from removable penalties.
// It does not own HP formulas, adventure clocks, scenes or rescue costs.
package life

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"math"
)

const FatalKnockdown = 4

func PenaltyFactor(layers int) float64 { return max(0, 1-.25*float64(layers)) }

func Incapacitate(p model.Player) (model.Player, error) {
	if p.Life != "alive" || p.HP != 0 || p.Knockdowns < 0 || p.Knockdowns >= 1000000000 {
		return model.Player{}, fmt.Errorf("invalid knockdown transition")
	}
	next := model.ClonePlayer(p)
	next.HP = 0
	next.Knockdowns++
	if next.Knockdowns-next.KnockdownBase >= FatalKnockdown {
		next.Life = "dead"
	} else {
		next.Life = "down"
	}
	return next, nil
}
func Raise(p model.Player) (model.Player, error) {
	if p.Life != "down" || p.Knockdowns-p.KnockdownBase < 1 || p.Knockdowns-p.KnockdownBase >= FatalKnockdown {
		return model.Player{}, fmt.Errorf("invalid rescue transition")
	}
	next := model.ClonePlayer(p)
	next.Life = "alive"
	next.Downs = min(3, next.Downs+1)
	return next, nil
}
func Validate(p model.Player) error {
	if p.KnockdownBase < 0 || p.KnockdownBase > p.Knockdowns {
		return fmt.Errorf("invalid knockdown baseline")
	}
	if math.IsNaN(p.HP) || math.IsInf(p.HP, 0) || p.HP < 0 || p.HP > p.MaxHP {
		return fmt.Errorf("invalid current health")
	}
	if p.Knockdowns < 0 || p.Knockdowns > 1000000000 || p.Downs < 0 || p.Downs > 3 || p.Downs > p.Knockdowns {
		return fmt.Errorf("invalid knockdown history or penalty")
	}
	switch p.Life {
	case "alive":
		if p.Knockdowns-p.KnockdownBase >= FatalKnockdown {
			return fmt.Errorf("invalid living knockdown count")
		}
	case "down":
		if p.Knockdowns-p.KnockdownBase < 1 || p.Knockdowns-p.KnockdownBase >= FatalKnockdown || p.HP != 0 {
			return fmt.Errorf("invalid downed player")
		}
	case "dead":
		if p.Knockdowns-p.KnockdownBase < FatalKnockdown || p.HP != 0 {
			return fmt.Errorf("invalid dead player")
		}
	default:
		return fmt.Errorf("invalid life state")
	}
	return nil
}

func Revive(p model.Player) (model.Player, error) {
	if p.Life != "dead" || p.Knockdowns-p.KnockdownBase < FatalKnockdown {
		return model.Player{}, fmt.Errorf("invalid resurrection")
	}
	n := model.ClonePlayer(p)
	n.Life = "alive"
	n.KnockdownBase = n.Knockdowns - 1
	return n, nil
}

// Permanent penalty projections share the source contract used by statuses and
// equipment. History is deliberately not part of the removable contribution.
func PenaltySource(p model.Player) model.SourceRef {
	return model.SourceRef{Kind: "penalty", Actor: p.ID, Definition: "engine:down_penalty"}
}
