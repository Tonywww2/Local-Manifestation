package damage

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"math"
)

func ValidType(t string) bool {
	return t == "physical" || t == "fire" || t == "cold" || t == "electromagnetic" || t == "chemical" || t == "magic"
}

// WithMultiplier extends named groups. Factor 1.2 contributes +20%; equal
// names add their percentages. Empty type applies to every damage segment.
func WithMultiplier(q Request, group, t string, factor float64, origin model.SourceRef) (Request, error) {
	if group == "" || t != "" && !ValidType(t) || math.IsNaN(factor) || math.IsInf(factor, 0) || factor < 0 || factor > 1e6 {
		return Request{}, fmt.Errorf("invalid hit multiplier")
	}
	if err := stats.ValidateSource(origin); err != nil {
		return Request{}, err
	}
	q.Weapon = model.CloneWeapon(q.Weapon)
	m := model.Modifier{Group: group, Type: t, Percent: factor - 1, Source: origin.Definition, Origin: origin}
	q.Weapon.Modifiers = append(q.Weapon.Modifiers, m)
	for i := range q.Weapon.AdditionalDamage {
		q.Weapon.AdditionalDamage[i].Modifiers = append(q.Weapon.AdditionalDamage[i].Modifiers, m)
	}
	return q, nil
}

// Contribute merges a typed hit bonus or appends an independent segment. Both
// carry the actual contributing source through the ordinary evaluator.
func Contribute(q Request, id, kind, t string, amount float64, origin model.SourceRef) (Request, error) {
	if id == "" || kind != "add_hit_damage" && kind != "append_hit_damage" || !ValidType(t) || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > 1e6 {
		return Request{}, fmt.Errorf("unknown damage type")
	}
	if err := stats.ValidateSource(origin); err != nil {
		return Request{}, err
	}
	q.Weapon = model.CloneWeapon(q.Weapon)
	q.FlatBonuses = append([]Bonus(nil), q.FlatBonuses...)
	if kind == "add_hit_damage" && q.Weapon.Type == t {
		q.FlatBonuses = append(q.FlatBonuses, Bonus{ID: id, Value: amount, Origin: origin})
		return q, nil
	}
	if kind == "add_hit_damage" {
		for i := range q.Weapon.AdditionalDamage {
			if q.Weapon.AdditionalDamage[i].Type == t {
				q.Weapon.AdditionalDamage[i].Bonuses = append(q.Weapon.AdditionalDamage[i].Bonuses, model.DamageBonus{ID: id, Amount: amount, Source: origin})
				return q, nil
			}
		}
	}
	q.Weapon.AdditionalDamage = append(q.Weapon.AdditionalDamage, model.DamagePart{ID: id, Type: t, Modifiers: append([]model.Modifier(nil), q.Weapon.Modifiers...), Bonuses: []model.DamageBonus{{ID: id, Amount: amount, Source: origin}}})
	return q, nil
}
