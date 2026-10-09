package content

import (
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
)

var DamageTypes = []string{"physical", "fire", "cold", "electromagnetic", "chemical", "magic"}

func (c *Catalog) PlayerDefense(p model.Player) map[string]damage.Defense {
	return c.PlayerDefenseAtHealth(p, p.HP, p.MaxHP)
}
func (c *Catalog) PlayerDefenseAtHealth(p model.Player, hp, maxHP float64) map[string]damage.Defense {
	out := map[string]damage.Defense{}
	for _, kind := range DamageTypes {
		d := damage.Defense{Armor: p.Armor, Immune: status.Immune(p.Statuses, kind)}
		for _, it := range p.Inventory.Items {
			if inventory.Equipped(p.Inventory, it.ID) {
				def, _ := c.Item(it.Definition)
				d.Resistance += def.Resistance[kind]
				d.Reductions = append(d.Reductions, def.Reductions...)
				for _, e := range def.Combat {
					if e.Kind == "low_health_reduction" && hp <= maxHP*e.Threshold {
						d.Reductions = append(d.Reductions, e.Amount)
					}
				}
			}
		}
		d.Resistance = min(1, d.Resistance)
		out[kind] = d
	}
	return out
}
func (c *Catalog) EnemyDefense(e model.Enemy, floor int) map[string]damage.Defense {
	def := c.Enemy(e.Definition)
	armor := def.Armor + def.ArmorPerFloor*float64(max(0, floor-1))
	if evaluated, err := stats.Evaluate(stats.Input{Base: model.StatValues{MaxHP: e.MaxHP, Speed: def.Speed, Armor: armor, AttackSpeed: 1}, Contributions: status.Contributions(e.Statuses)}); err == nil {
		armor = evaluated.Values.Armor
	}
	out := map[string]damage.Defense{}
	for _, kind := range DamageTypes {
		out[kind] = damage.Defense{Armor: armor, Resistance: min(1, def.Resistance[kind]+def.ResistancePerFloor[kind]*float64(max(0, floor-1))), Immune: status.Immune(e.Statuses, kind)}
	}
	return out
}
