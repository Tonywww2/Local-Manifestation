package content

import (
	"fmt"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"math"
)

// CombatContributions captures equipment and active status sources by value.
func (c *Catalog) CombatContributions(p model.Player) []model.CombatContribution {
	var out []model.CombatContribution
	if c == nil {
		return out
	}
	for _, it := range p.Inventory.Items {
		if !inventory.Equipped(p.Inventory, it.ID) {
			continue
		}
		d, _ := c.Item(it.Definition)
		for _, e := range d.Combat {
			out = append(out, model.CombatContribution{CombatEffect: e, Source: model.SourceRef{Kind: "equipment", Actor: p.ID, Definition: d.ID, Instance: it.ID}})
		}
	}
	for _, s := range p.Statuses.Instances {
		d, _ := c.Status(s.Definition)
		for range s.Layers {
			for _, e := range d.Combat {
				out = append(out, model.CombatContribution{CombatEffect: e, Source: s.Source})
			}
		}
	}
	return out
}

// HealingAmount deliberately excludes lifesteal, rescue and resurrection.
func (c *Catalog) HealingAmount(p model.Player, base float64, unamplified bool) float64 {
	if unamplified {
		return base
	}
	return damage.Round(base * c.HealingFactor(p))
}
func (c *Catalog) HealingFactor(p model.Player) float64 {
	bonus := 0.0
	for _, e := range c.CombatContributions(p) {
		if e.Kind == "healing" {
			bonus += e.Amount
		}
	}
	return 1 + bonus
}

func TriggerDodge(c *Catalog, p model.Player) (model.Player, error) {
	n := model.ClonePlayer(p)
	for _, e := range c.CombatContributions(p) {
		if e.Kind == "dodge_status" {
			var err error
			n, err = ApplyStatus(c, n, e.Status, e.Source)
			if err != nil {
				return model.Player{}, err
			}
		}
	}
	return n, nil
}
func EnterBattle(c *Catalog, p model.Player) (model.Player, error) {
	n := model.ClonePlayer(p)
	if p.Life != "alive" {
		return n, nil
	}
	for _, e := range c.CombatContributions(p) {
		if e.Kind == "battle_shield" {
			var err error
			n, err = ApplyStatus(c, n, e.Status, e.Source)
			if err != nil {
				return model.Player{}, err
			}
		}
	}
	return n, nil
}

// PrepareHit reads the untouched target exactly once. Appended damage stays in
// this hit and uses the same critical roll, while armor settles per segment.
func PrepareHit(q damage.Request, target model.Enemy, weaponAttack bool) (damage.Request, error) {
	appendDamage, conditions := false, false
	for _, e := range q.Effects {
		appendDamage = appendDamage || e.Kind == "append_damage" && weaponAttack
		conditions = conditions || e.Kind == "hit_multiplier"
	}
	if !appendDamage && !conditions {
		return q, nil
	}
	q = damage.CloneRequest(q)
	var basis model.DamageBasis
	if appendDamage {
		var err error
		basis, err = PreMultiplierBasis(q)
		if err != nil {
			return damage.Request{}, err
		}
	}
	burning := false
	for _, s := range target.Statuses.Instances {
		if s.Definition == "base:burn" {
			burning = true
		}
	}
	// Append before conditional multipliers so every segment receives them once.
	for i, e := range q.Effects {
		if e.Kind != "append_damage" || !weaponAttack {
			continue
		}
		var err error
		q, err = damage.Contribute(q, fmt.Sprintf("%s:feedback:%d", e.Source.Definition, i), "append_hit_damage", e.Type, max(0, basis.Total())*e.Amount, e.Source)
		if err != nil {
			return damage.Request{}, err
		}
		basis.Factor = e.Amount
		q.Weapon.AdditionalDamage[len(q.Weapon.AdditionalDamage)-1].Bonuses[0].Basis = basis
	}
	for _, e := range q.Effects {
		if e.Kind != "hit_multiplier" {
			continue
		}
		active := e.Condition == "" || e.Condition == "weapon_attack" && weaponAttack || e.Condition == "full_health" && target.HP >= target.MaxHP || e.Condition == "burning" && burning
		if active {
			var err error
			q, err = damage.WithMultiplier(q, e.Group, e.Type, 1+e.Amount, e.Source)
			if err != nil {
				return damage.Request{}, err
			}
		}
	}
	return q, nil
}

// PreMultiplierBase is the same primary attack basis used by Feedback Clip.
// Secondary native damage segments are separate; this effect is appended once.
func PreMultiplierBase(q damage.Request) (float64, error) {
	b, err := PreMultiplierBasis(q)
	return b.Total(), err
}
func PreMultiplierBasis(q damage.Request) (model.DamageBasis, error) {
	attrs := q.Attributes
	if q.Stats != nil {
		r, err := stats.Evaluate(*q.Stats)
		if err != nil {
			return model.DamageBasis{}, err
		}
		attrs = r.Values.Attributes
	}
	basis := model.DamageBasis{Base: q.Weapon.Base, Flat: q.Weapon.Flat, Attributes: q.Weapon.Scaling.Dot(attrs)}
	for _, b := range q.FlatBonuses {
		if b.ID == "engine:weapon_upgrade" {
			basis.Upgrade += b.Value
		} else {
			basis.Flat += b.Value
		}
	}
	return basis, nil
}
func EvaluatePreview(q damage.Request) (damage.Result, error) {
	q, err := PrepareHit(q, model.Enemy{MaxHP: 1}, true)
	if err != nil {
		return damage.Result{}, err
	}
	return damage.Evaluate(q)
}

func (c *Catalog) validateCombat(effects []model.CombatEffect, statusOwner bool) error {
	if len(effects) > 16 {
		return fmt.Errorf("combat effect budget exceeded")
	}
	for _, e := range effects {
		if math.IsNaN(e.Amount) || math.IsInf(e.Amount, 0) || e.Amount < 0 || e.Amount > 10 || math.IsNaN(e.Threshold) || math.IsInf(e.Threshold, 0) || e.Threshold < 0 || e.Threshold > 1 {
			return fmt.Errorf("invalid combat effect value")
		}
		switch e.Kind {
		case "critical_damage", "healing":
		case "hit_multiplier":
			if e.Group == "" || e.Type != "" && !damage.ValidType(e.Type) || e.Condition != "" && e.Condition != "weapon_attack" && e.Condition != "full_health" && e.Condition != "burning" {
				return fmt.Errorf("invalid hit condition")
			}
		case "append_damage":
			if !damage.ValidType(e.Type) {
				return fmt.Errorf("invalid appended damage type")
			}
		case "low_health_reduction":
			if e.Amount > 1 {
				return fmt.Errorf("invalid reduction")
			}
		case "dodge_status", "battle_shield":
			if statusOwner {
				return fmt.Errorf("recursive status trigger")
			}
			if _, ok := c.statuses[e.Status]; !ok {
				return fmt.Errorf("unknown combat status")
			}
			if e.Kind == "battle_shield" {
				d := c.statuses[e.Status]
				if d.Shield <= 0 && d.ShieldMaxHP <= 0 {
					return fmt.Errorf("battle shield requires shield status")
				}
			}
		default:
			return fmt.Errorf("unknown combat effect %s", e.Kind)
		}
	}
	return nil
}

// SelectLoot is deterministic given an explicit uniform sample in [0,1).
func SelectLoot(items []model.WeightedDrop, sample float64) string {
	total := 0.0
	for _, e := range items {
		total += e.Weight
	}
	value := max(0, min(math.Nextafter(1, 0), sample)) * total
	for _, e := range items {
		value -= e.Weight
		if value < 0 {
			return e.Item
		}
	}
	return items[len(items)-1].Item
}
func (c *Catalog) validateDraws(draws []model.LootDraw) error {
	if len(draws) > 16 {
		return fmt.Errorf("loot draw budget")
	}
	for _, d := range draws {
		if d.Rolls < 1 || d.Rolls > 16 || math.IsNaN(d.Chance) || math.IsInf(d.Chance, 0) || d.Chance < 0 || d.Chance > 1 || len(d.Items) == 0 || len(d.Items) > 512 {
			return fmt.Errorf("invalid loot draw")
		}
		for _, e := range d.Items {
			if _, ok := c.items[e.Item]; !ok || math.IsNaN(e.Weight) || math.IsInf(e.Weight, 0) || e.Weight <= 0 || e.Weight > 1e6 {
				return fmt.Errorf("invalid weighted loot")
			}
		}
	}
	return nil
}
