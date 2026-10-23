package content

import (
	"fmt"

	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
)

func StatInput(p model.Player) stats.Input {
	contributions := append([]model.StatContribution(nil), p.StatContributions...)
	contributions = append(contributions, status.Contributions(p.Statuses)...)
	return stats.Input{Base: p.BaseStats, BaseSource: model.SourceRef{Kind: "chassis", Actor: p.ID, Definition: p.Chassis}, Contributions: contributions}
}

// RefreshPlayer computes a detached replacement. The owner commits it only on success.
func RefreshPlayer(p model.Player) (model.Player, error) {
	r, err := stats.Evaluate(StatInput(p))
	if err != nil {
		return model.Player{}, err
	}
	next := model.ClonePlayer(p)
	next.Attributes, next.Armor, next.Speed = r.Values.Attributes, r.Values.Armor, r.Values.Speed
	// Preserve the unpenalized projection for the final permanent penalty stage.
	next.BaseHP = r.Values.MaxHP
	next.MaxHP = MaxHP(next)
	if p.MaxHP > 0 {
		next.HP, err = stats.ResizeHealth(p.HP, p.MaxHP, next.MaxHP)
		if err != nil {
			return model.Player{}, err
		}
	} else {
		next.HP = next.MaxHP
	}
	return next, nil
}

func ApplyFloorJoinGrowth(p model.Player, floor int) (model.Player, error) {
	if floor < 1 {
		return model.Player{}, fmt.Errorf("invalid floor")
	}
	c := model.StatContribution{StatModifier: model.StatModifier{ID: "engine:floor_join", Stat: "max_hp", Operation: "add", Value: float64(floor-1) * 10}, Source: model.SourceRef{Kind: "growth", Actor: p.ID, Definition: "engine:floor_join"}}
	next := model.ClonePlayer(p)
	var err error
	next.StatContributions, err = stats.Upsert(next.StatContributions, c)
	if err != nil {
		return model.Player{}, err
	}
	return RefreshPlayer(next)
}

// Attack captures source values once. Defense is filled in at the actual hit.
func Attack(c *Catalog, p model.Player) (damage.Request, error) {
	w := c.Weapon(p.Weapon)
	if w.ID == "" {
		return damage.Request{}, fmt.Errorf("unknown weapon")
	}
	for i := range w.Modifiers {
		w.Modifiers[i].Origin = model.SourceRef{Kind: "weapon", Actor: p.ID, Definition: w.ID, Instance: p.Inventory.Equipment[0]}
	}
	for i := range w.AdditionalDamage {
		for j := range w.AdditionalDamage[i].Modifiers {
			w.AdditionalDamage[i].Modifiers[j].Origin = model.SourceRef{Kind: "weapon", Actor: p.ID, Definition: w.ID, Instance: p.Inventory.Equipment[0]}
		}
	}
	for _, it := range p.Inventory.Items {
		if inventory.Equipped(p.Inventory, it.ID) {
			d, _ := c.Item(it.Definition)
			if a, ok := c.Affix(it.Affix); ok {
				d.DamageModifiers = append(d.DamageModifiers, a.DamageModifiers...)
			}
			for _, m := range d.DamageModifiers {
				m.Origin = model.SourceRef{Kind: "equipment", Actor: p.ID, Definition: d.ID, Instance: it.ID}
				w.Modifiers = append(w.Modifiers, m)
				for i := range w.AdditionalDamage {
					w.AdditionalDamage[i].Modifiers = append(w.AdditionalDamage[i].Modifiers, m)
				}
			}
		}
	}
	input := StatInput(p)
	q := damage.Request{Source: model.SourceRef{Kind: "weapon", Actor: p.ID, Definition: w.ID, Instance: p.Inventory.Equipment[0]}, Weapon: w, Stats: &input, FinalMultiplier: life.PenaltyFactor(p.Downs), FinalSource: life.PenaltySource(p)}
	if p.WeaponBonus != 0 {
		q.FlatBonuses = []damage.Bonus{{ID: "engine:weapon_upgrade", Value: p.WeaponBonus, Origin: model.SourceRef{Kind: "weapon_upgrade", Actor: p.ID, Definition: w.ID, Instance: p.Inventory.Equipment[0]}}}
	}
	q.Effects = c.CombatContributions(p)
	q.CritMultiplier = 1.5
	for _, e := range q.Effects {
		if e.Kind == "critical_damage" {
			q.CritMultiplier *= 1 + e.Amount
		}
	}
	return q, nil
}
