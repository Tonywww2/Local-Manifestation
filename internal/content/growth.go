package content

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/progression"
	"hsmultiplayer/pkg/stats"
)

// LevelContributions is a value projection: base growth and chassis preference
// remain separate sources in the same reusable attribute/damage explanations.
func LevelContributions(p model.Player, ch model.Chassis) []model.StatContribution {
	var out []model.StatContribution
	if p.Level <= 1 {
		return out
	}
	for _, key := range model.AttributeKeys() {
		for _, kind := range []string{"engine:level", "engine:chassis_growth"} {
			value := 1.0
			if kind == "engine:chassis_growth" {
				value, _ = ch.GrowthPreference.Value(key)
			}
			if value == 0 {
				continue
			}
			out = append(out, model.StatContribution{StatModifier: model.StatModifier{ID: kind + ":" + key, Stat: key, Operation: "add", Value: value * float64(p.Level-1)}, Source: model.SourceRef{Kind: "growth", Actor: p.ID, Definition: kind}})
		}
	}
	return out
}

func (c *Catalog) GrantExperience(p model.Player, amount int) (model.Player, error) {
	if amount < 0 || amount > 1e9 || p.XP < 0 || p.XP > 1e9-amount || p.Level < 1 {
		return model.Player{}, fmt.Errorf("invalid experience")
	}
	_, ok := c.Chassis(p.Chassis)
	if !ok {
		return model.Player{}, fmt.Errorf("unknown chassis")
	}
	next := model.ClonePlayer(p)
	next.XP += amount
	for next.XP >= progression.NextLevelXP(next.Level) {
		next.XP -= progression.NextLevelXP(next.Level)
		next.Level++
	}
	if next.Level == p.Level {
		return next, nil
	}
	return c.RefreshLevelGrowth(next)
}

// Late arrivals start at the floor's level and receive the same growth sources.
func (c *Catalog) RefreshLevelGrowth(p model.Player) (model.Player, error) {
	ch, ok := c.Chassis(p.Chassis)
	if !ok || p.Level < 1 {
		return model.Player{}, fmt.Errorf("invalid chassis level")
	}
	next := model.ClonePlayer(p)
	var err error
	for _, contribution := range LevelContributions(next, ch) {
		next.StatContributions, err = stats.Upsert(next.StatContributions, contribution)
		if err != nil {
			return model.Player{}, err
		}
	}
	return RefreshPlayer(next)
}
