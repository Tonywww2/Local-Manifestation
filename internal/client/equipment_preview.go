package client

import (
	"fmt"
	"slices"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
)

func equipmentSlot(v model.Inventory, d model.ItemDef) int {
	switch d.Kind {
	case "weapon":
		return 0
	case "armor":
		return slices.Index(inventory.Slots[:], d.Slot)
	case "consumable":
		for i := 5; i < 9; i++ {
			if v.Equipment[i] == "" {
				return i
			}
		}
		return 5
	case "accessory":
		for i, id := range v.Accessories {
			if id == "" {
				return i + 9
			}
		}
		return 9 + len(v.Accessories)
	}
	return -1
}

func (g *Game) equipmentComparison(c *uiControl, item model.ItemInstance, d model.ItemDef, p model.Player) {
	if d.Kind == "consumable" && len(d.Modifiers) == 0 && len(d.DamageModifiers) == 0 {
		return
	}
	slot := equipmentSlot(p.Inventory, d)
	if slot < 0 {
		return
	}
	if inventory.Equipped(p.Inventory, item.ID) {
		for i, id := range p.Inventory.Equipment {
			if id == item.ID {
				slot = i
			}
		}
		for i, id := range p.Inventory.Accessories {
			if id == item.ID {
				slot = i + 9
			}
		}
	}
	next, err := g.catalog.PreviewEquipment(p, item, slot)
	if err != nil {
		c.Details = append(c.Details, g.tr("ui.compare.unavailable"))
		return
	}
	before, err := stats.Evaluate(content.StatInput(p))
	if err != nil {
		return
	}
	after, err := stats.Evaluate(content.StatInput(next))
	if err != nil {
		return
	}
	for _, v := range []struct {
		key  string
		a, b float64
	}{
		{"strength", before.Values.Attributes.Strength, after.Values.Attributes.Strength},
		{"agility", before.Values.Attributes.Agility, after.Values.Attributes.Agility},
		{"constitution", before.Values.Attributes.Constitution, after.Values.Attributes.Constitution},
		{"intelligence", before.Values.Attributes.Intelligence, after.Values.Attributes.Intelligence},
		{"sense", before.Values.Attributes.Sense, after.Values.Attributes.Sense},
		{"mana", before.Values.Attributes.Mana, after.Values.Attributes.Mana},
		{"max_hp", before.Values.MaxHP, after.Values.MaxHP}, {"armor", before.Values.Armor, after.Values.Armor},
		{"speed", before.Values.Speed, after.Values.Speed}, {"attack_speed", before.Values.AttackSpeed, after.Values.AttackSpeed},
	} {
		if v.a != v.b {
			c.Details = append(c.Details, g.tr("ui.compare.stat", "stat", g.localized("attribute."+v.key, v.key), "before", fmt.Sprintf("%.2f", v.a), "after", fmt.Sprintf("%.2f", v.b), "delta", fmt.Sprintf("%+.2f", v.b-v.a)))
		}
	}
}
