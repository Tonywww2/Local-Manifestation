package content

import (
	"fmt"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"math"
)

func (c *Catalog) validateItems(claim func(string) bool) error {
	c.affixes = map[string]model.AffixDef{}
	for _, a := range c.definitions.Affixes {
		if !claim(a.ID) || a.Name == "" || a.Value < 0 || a.Value > 1e6 {
			return fmt.Errorf("invalid affix")
		}
		for _, m := range a.Modifiers {
			if err := stats.ValidateModifier(m); err != nil {
				return err
			}
		}
		for _, m := range a.DamageModifiers {
			if m.Group == "" || m.Source == "" || !damage.ValidType(m.Type) && m.Type != "" || math.IsNaN(m.Percent) || math.IsInf(m.Percent, 0) || m.Percent < -1 || m.Percent > 1e6 {
				return fmt.Errorf("invalid affix multiplier")
			}
		}
		c.affixes[a.ID] = a
	}
	c.items = map[string]model.ItemDef{}
	if len(c.definitions.Items) == 0 {
		for _, w := range c.definitions.Weapons {
			d := model.ItemDef{ID: w.ID + "-item", Name: w.Name, Description: "Development weapon", Kind: "weapon", Weapon: w.ID, Value: 20}
			c.definitions.Items = append(c.definitions.Items, d)
		}
	}
	for _, d := range c.definitions.Items {
		if d.Tier < 0 || d.Tier > 3 || d.Tier > 0 && d.Kind != "accessory" && d.Kind != "movement" && d.Kind != "passive" && d.Kind != "consumable" {
			return fmt.Errorf("invalid item tier")
		}
		if err := c.validateSpecialConsumable(d); err != nil {
			return fmt.Errorf("%s: %w", d.ID, err)
		}
		if err := c.validateCombat(d.Combat, false); err != nil {
			return fmt.Errorf("%s: %w", d.ID, err)
		}
		if d.StackLimit < 0 || d.StackLimit > 30 || d.Kind != "consumable" && d.StackLimit > 1 {
			return fmt.Errorf("invalid stack limit for %s", d.ID)
		}
		for _, id := range d.Affixes {
			if _, ok := c.affixes[id]; !ok {
				return fmt.Errorf("unknown affix")
			}
		}
		for k, v := range d.Resistance {
			if !damage.ValidType(k) || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
				return fmt.Errorf("invalid item resistance")
			}
		}
		for _, v := range d.Reductions {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
				return fmt.Errorf("invalid item reduction")
			}
		}
		if !claim(d.ID) || d.Name == "" || d.Value < 0 || d.Value > 1e6 {
			return fmt.Errorf("invalid item %s", d.ID)
		}
		switch d.Kind {
		case "weapon":
			if c.weapons[d.Weapon].ID == "" {
				return fmt.Errorf("unknown weapon")
			}
		case "armor":
			if d.Slot != "head" && d.Slot != "body" && d.Slot != "legs" && d.Slot != "feet" {
				return fmt.Errorf("invalid armor slot")
			}
		case "accessory", "consumable", "movement", "passive":
		default:
			return fmt.Errorf("invalid item kind")
		}
		if d.Kind != "weapon" && d.Weapon != "" || d.Kind != "armor" && d.Slot != "" || d.Kind != "consumable" && len(d.Use) != 0 {
			return fmt.Errorf("unexpected item fields")
		}
		for _, m := range d.Modifiers {
			if err := stats.ValidateModifier(m); err != nil {
				return err
			}
		}
		for _, m := range d.DamageModifiers {
			if m.Type != "" && !damage.ValidType(m.Type) {
				return fmt.Errorf("invalid item damage type")
			}
			if m.Group == "" || m.Source == "" || math.IsNaN(m.Percent) || math.IsInf(m.Percent, 0) || m.Percent < -1 || m.Percent > 1e6 {
				return fmt.Errorf("invalid item damage modifier")
			}
		}
		for _, e := range d.Use {
			if err := validateActionScaling(e); err != nil {
				return err
			}
			if math.IsNaN(e.Amount) || math.IsInf(e.Amount, 0) || e.Amount < 0 || e.Amount > 1e6 {
				return fmt.Errorf("invalid item effect")
			}
			switch e.Kind {
			case "apply_status":
				if _, ok := c.statuses[e.Status]; !ok {
					return fmt.Errorf("unknown item status")
				}
			case "heal", "remove_penalty", "clear_curse", "support":
			default:
				return fmt.Errorf("unknown item effect")
			}
		}
		c.items[d.ID] = d
	}
	for _, id := range c.definitions.StartingItems {
		if c.items[id].ID == "" {
			return fmt.Errorf("unknown starting item")
		}
	}
	return nil
}
