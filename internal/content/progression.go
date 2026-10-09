package content

import (
	"fmt"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"slices"
	"sort"
)

func (c *Catalog) Perk(id string) (model.PerkDef, bool) { v, ok := c.perks[id]; return clone(v), ok }
func (c *Catalog) Perks() []model.PerkDef               { return clone(c.definitions.Perks) }
func (c *Catalog) ClearPoints() int                     { return c.definitions.ClearPoints }
func (c *Catalog) ApplyMeta(p model.Player, ranks map[string]int, rules model.MetaRules) (model.Player, error) {
	if err := c.ValidateRanks(ranks); err != nil {
		return model.Player{}, err
	}
	if chassis, ok := c.Chassis(p.Chassis); ok && rules.Unlocks && chassis.RequiredPerk != "" && ranks[chassis.RequiredPerk] < 1 {
		return model.Player{}, fmt.Errorf("chassis requires an unlock")
	}
	n := model.ClonePlayer(p)
	n.MetaRanks = map[string]int{}
	for _, d := range c.definitions.Perks {
		rank := ranks[d.ID]
		if rank < 0 || rank > d.MaxRank {
			return model.Player{}, fmt.Errorf("invalid perk rank")
		}
		if rank == 0 {
			continue
		}
		for _, req := range d.Requires {
			if ranks[req] < 1 {
				return model.Player{}, fmt.Errorf("missing perk prerequisite")
			}
		}
		n.MetaRanks[d.ID] = rank
		if rules.Stats {
			for _, m := range d.Modifiers {
				m.Value *= float64(rank)
				n.StatContributions = append(n.StatContributions, model.StatContribution{StatModifier: m, Source: model.SourceRef{Kind: "meta", Actor: p.ID, Definition: d.ID}})
			}
		}
		if rules.Coins {
			n.Coins += rank * d.Coins
		}
		if rules.Equipment {
			for i := 0; i < rank; i++ {
				for _, id := range d.StartingItems {
					var err error
					n, err = c.GrantItem(n, id)
					if err != nil {
						return model.Player{}, err
					}
				}
			}
		}
	}
	for id := range ranks {
		if _, ok := c.perks[id]; !ok {
			return model.Player{}, fmt.Errorf("unknown perk")
		}
	}
	if err := inventory.Validate(n.Inventory, c.Item); err != nil {
		return model.Player{}, err
	}
	return RefreshPlayer(n)
}
func (c *Catalog) ValidateRanks(ranks map[string]int) error {
	for id, rank := range ranks {
		d, ok := c.perks[id]
		if !ok || rank < 0 || rank > d.MaxRank {
			return fmt.Errorf("invalid perk rank")
		}
		if rank > 0 {
			for _, required := range d.Requires {
				if ranks[required] < 1 {
					return fmt.Errorf("missing perk prerequisite")
				}
			}
		}
	}
	return nil
}
func (c *Catalog) TeamMeta(players []model.Player, rules model.MetaRules) (int, int, []string) {
	personal, shared := 0, 0
	var special []string
	if !rules.AP && !rules.Nodes {
		return 0, 0, nil
	}
	nodes := map[string]bool{}
	for _, p := range players {
		x, y := 0, 0
		for _, d := range c.definitions.Perks {
			rank := p.MetaRanks[d.ID]
			if rules.AP {
				x += d.PersonalAP * rank
				y += d.SharedAP * rank
			}
			if rules.Nodes && rank > 0 {
				for _, id := range d.SpecialNodes {
					nodes[id] = true
				}
			}
		}
		personal = max(personal, x)
		shared = max(shared, y)
	}
	for id := range nodes {
		special = append(special, id)
	}
	sort.Strings(special)
	return personal, shared, special
}
func (c *Catalog) validatePerks(claim func(string) bool) error {
	c.perks = map[string]model.PerkDef{}
	for _, d := range c.definitions.CompletionRewards {
		if !claim(d.ID) || d.Name == "" || (d.Kind != "achievement" && d.Kind != "challenge") || d.Points < 0 || d.Points > 10000 || (d.Condition != "clear" && d.Condition != "no_knockdowns" && d.Condition != "enhanced_clear") {
			return fmt.Errorf("invalid completion reward")
		}
	}
	if c.definitions.ClearPoints < 0 || c.definitions.ClearPoints > 1e6 {
		return fmt.Errorf("invalid clear reward")
	}
	for _, d := range c.definitions.Perks {
		if !claim(d.ID) || d.Name == "" || d.Cost < 1 || d.Cost > 1e6 || d.MaxRank < 1 || d.MaxRank > 1000 || d.Coins < 0 || d.Coins > 1e6 || d.PersonalAP < 0 || d.PersonalAP > 1000 || d.SharedAP < 0 || d.SharedAP > 1000 {
			return fmt.Errorf("invalid perk")
		}
		for _, m := range d.Modifiers {
			if err := stats.ValidateModifier(m); err != nil {
				return err
			}
		}
		for _, it := range d.StartingItems {
			if _, ok := c.items[it]; !ok {
				return fmt.Errorf("unknown perk item")
			}
		}
		c.perks[d.ID] = d
	}
	visiting := map[string]bool{}
	done := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if done[id] {
			return nil
		}
		d, ok := c.perks[id]
		if !ok || visiting[id] {
			return fmt.Errorf("invalid perk dependency")
		}
		visiting[id] = true
		for _, req := range d.Requires {
			if err := visit(req); err != nil {
				return err
			}
		}
		visiting[id] = false
		done[id] = true
		return nil
	}
	for id := range c.perks {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
func (c *Catalog) CompletionRewards(p model.Player, history model.Progression) (int, []string) {
	points := c.ClearPoints()
	var ids []string
	for _, d := range c.definitions.CompletionRewards {
		if slices.Contains(history.Achievements, d.ID) {
			continue
		}
		eligible := d.Condition == "clear" || d.Condition == "no_knockdowns" && p.Knockdowns == 0 || d.Condition == "enhanced_clear" && p.EnhancedWins > 0
		if eligible {
			points += d.Points
			ids = append(ids, d.ID)
		}
	}
	return points, ids
}
