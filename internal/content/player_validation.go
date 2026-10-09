package content

import (
	"fmt"
	"math"

	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"reflect"
)

// ValidatePlayerStats resolves the sources that this engine version can persist.
// Future items/statuses must extend this boundary when their owners are implemented.
func ValidatePlayerStats(c *Catalog, p model.Player, floor int) error {
	if p.Energy < 0 || p.Energy > p.MaxEnergy || p.MaxEnergy != c.definitions.Resource.Maximum || p.EnergyRegen != c.definitions.Resource.Regeneration || math.IsNaN(p.Energy) || math.IsInf(p.Energy, 0) || p.ResourceTime != p.Statuses.Time {
		return fmt.Errorf("invalid resource snapshot")
	}
	ch, _ := c.Chassis(p.Chassis)
	if p.Skills != ch.Skills {
		return fmt.Errorf("invalid skill loadout")
	}
	for _, time := range p.SkillReady {
		if time < 0 || time > p.Statuses.Time+86400000*60 {
			return fmt.Errorf("invalid cooldown")
		}
	}
	if err := ValidatePlayerStatuses(c, p); err != nil {
		return err
	}
	if err := inventory.Validate(p.Inventory, c.Item); err != nil {
		return err
	}
	seenLoot := map[string]bool{}
	for _, it := range p.Inventory.Items {
		seenLoot[it.ID] = true
	}
	if len(p.Loot) > 4096 {
		return fmt.Errorf("loot budget")
	}
	for _, drop := range p.Loot {
		d, ok := c.Item(drop.Item.Definition)
		if inventory.Count(drop.Item) < 1 || inventory.Count(drop.Item) > inventory.StackLimit(d) {
			return fmt.Errorf("invalid loot stack")
		}
		if !ok || seenLoot[drop.Item.ID] || drop.Item.ID == "" || !drop.Position.Finite() || drop.Node == "" || drop.Item.Upgrade < 0 || drop.Item.Upgrade > 100000 || math.IsNaN(drop.Item.WeaponBonus) || math.IsInf(drop.Item.WeaponBonus, 0) || drop.Item.WeaponBonus < 0 || drop.Item.WeaponBonus > 1e9 {
			return fmt.Errorf("invalid loot")
		}
		if drop.Item.Affix != "" {
			found := false
			for _, id := range d.Affixes {
				if id == drop.Item.Affix {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("invalid loot affix")
			}
		}
		seenLoot[drop.Item.ID] = true
	}
	equipment, err := c.RefreshEquipment(p)
	if err != nil {
		return err
	}
	filter := func(v model.Player) []model.StatContribution {
		var out []model.StatContribution
		for _, x := range v.StatContributions {
			if x.Source.Kind == "equipment" {
				out = append(out, x)
			}
		}
		return out
	}
	if equipment.Weapon != p.Weapon || equipment.Upgrade != p.Upgrade || equipment.WeaponBonus != p.WeaponBonus || !reflect.DeepEqual(filter(equipment), filter(p)) {
		return fmt.Errorf("equipment projection mismatch")
	}
	chassis, ok := c.Chassis(p.Chassis)
	if !ok || p.BaseStats != (model.StatValues{AttackSpeed: 1, Attributes: chassis.Attributes, MaxHP: chassis.HP, Armor: chassis.Armor, Speed: chassis.Speed}) {
		return fmt.Errorf("invalid base stats")
	}
	expected := map[string]model.StatModifier{}
	for _, m := range chassis.Modifiers {
		expected[m.ID] = m
	}
	seen := map[string]bool{}
	allocated := 0.0
	for _, s := range p.StatContributions {
		if s.Source.Actor != p.ID || s.Source.Kind != "equipment" && s.Source.Instance != "" {
			return fmt.Errorf("invalid contribution owner")
		}
		switch s.Source.Kind {
		case "meta":
			d, ok := c.Perk(s.Source.Definition)
			rank := p.MetaRanks[d.ID]
			if !ok || rank < 1 || rank > d.MaxRank {
				return fmt.Errorf("invalid meta source")
			}
			valid := false
			for _, v := range d.Modifiers {
				v.Value *= float64(rank)
				if v == s.StatModifier {
					valid = true
				}
			}
			key := s.Source.Definition + ":" + s.ID
			if !valid || seen[key] {
				return fmt.Errorf("invalid meta contribution")
			}
			seen[key] = true
			continue
		case "equipment":
			continue // Exact definitions/instances were checked by RefreshEquipment above.
		case "chassis":
			m, ok := expected[s.ID]
			if !ok || s.Source.Definition != p.Chassis || s.StatModifier != m || seen[s.ID] {
				return fmt.Errorf("invalid chassis contribution")
			}
			seen[s.ID] = true
		case "growth":
			switch s.Source.Definition {
			case "engine:level":
				if _, ok := p.Attributes.Value(s.Stat); !ok || s.ID != "engine:allocation:"+s.Stat || s.Operation != "add" || s.Group != "" || s.Value < 0 || s.Value != math.Trunc(s.Value) {
					return fmt.Errorf("invalid allocation contribution")
				}
				allocated += s.Value
			case "engine:floor_join":
				if s.ID != "engine:floor_join" || s.Stat != "max_hp" || s.Operation != "add" || s.Group != "" || s.Value < 0 || s.Value > float64(floor-1)*10 || s.Value/10 != math.Trunc(s.Value/10) {
					return fmt.Errorf("invalid floor join contribution")
				}
			default:
				return fmt.Errorf("unknown growth source")
			}
		default:
			return fmt.Errorf("unknown stat source")
		}
	}
	chassisSeen := 0
	for id := range expected {
		if seen[id] {
			chassisSeen++
		}
	}
	if chassisSeen != len(expected) || p.Points < 0 || allocated+float64(p.Points) > float64(p.Level-1) {
		return fmt.Errorf("missing source or invalid allocation budget")
	}
	derived, err := RefreshPlayer(p)
	if err != nil {
		return err
	}
	if derived.Attributes != p.Attributes || derived.BaseHP != p.BaseHP || derived.MaxHP != p.MaxHP || derived.Armor != p.Armor || derived.Speed != p.Speed {
		return fmt.Errorf("derived stats mismatch")
	}
	return nil
}
