// Package progression plans learning and once-per-adventure settlement as values.
package progression

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"slices"
)

type Resolve func(string) (model.PerkDef, bool)

func Learn(p model.Progression, id string, resolve Resolve) (model.Progression, error) {
	d, ok := resolve(id)
	if !ok || p.Ranks[id] >= d.MaxRank {
		return model.Progression{}, fmt.Errorf("perk unavailable")
	}
	if p.Points < d.Cost {
		return model.Progression{}, fmt.Errorf("insufficient progression points")
	}
	for _, req := range d.Requires {
		if p.Ranks[req] == 0 {
			return model.Progression{}, fmt.Errorf("prerequisite not learned")
		}
	}
	n := model.CloneProgression(p)
	n.Points -= d.Cost
	n.Ranks[id]++
	return n, nil
}
func Validate(p model.Progression, resolve Resolve) error {
	if p.Points < 0 || p.Earned < 0 || p.Earned > 1e9 || p.Points > p.Earned || len(p.Warehouse) > 100000 || len(p.Settlements) > 100000 {
		return fmt.Errorf("invalid progression bounds")
	}
	spent := 0
	for id, rank := range p.Ranks {
		d, ok := resolve(id)
		if !ok || rank < 0 || rank > d.MaxRank {
			return fmt.Errorf("invalid perk rank")
		}
		spent += rank * d.Cost
		for _, req := range d.Requires {
			if rank > 0 && p.Ranks[req] < 1 {
				return fmt.Errorf("missing prerequisite")
			}
		}
	}
	if spent+p.Points > p.Earned {
		return fmt.Errorf("invalid progression budget")
	}
	return nil
}
func Settle(p model.Progression, runID string, player model.Player, won bool, points int) (model.Progression, bool, error) {
	if runID == "" || len(runID) > 128 || points < 0 || points > 1e6 {
		return model.Progression{}, false, fmt.Errorf("invalid settlement")
	}
	if slices.Contains(p.Settlements, runID) {
		return p, false, nil
	}
	n := model.CloneProgression(p)
	n.Settlements = append(n.Settlements, runID)
	if won {
		n.Points += points
		n.Earned += points
		for _, it := range player.Inventory.Items {
			it.SkillReady = 0
			n.Warehouse = append(n.Warehouse, model.StoredItem{Item: it, RunID: runID})
		}
		if !slices.Contains(n.Achievements, "engine:first-clear") {
			n.Achievements = append(n.Achievements, "engine:first-clear")
		}
	}
	return n, true, nil
}
