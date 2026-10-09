package content

import (
	"fmt"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"slices"
)

func (c *Catalog) Item(id string) (model.ItemDef, bool) {
	d, ok := c.items[id]
	return model.CloneItemDef(d), ok
}
func (c *Catalog) ItemList() []model.ItemDef { return clone(c.definitions.Items) }
func (c *Catalog) InitializeInventory(p model.Player) (model.Player, error) {
	if p.Inventory.Capacity == 0 {
		p.Inventory.Capacity = 27
		p.Inventory.AccessorySlots = 12
	}
	// Old development packs without explicit item definitions get weapon items.
	for _, d := range c.ItemList() {
		if d.Kind == "weapon" && d.Weapon == p.Weapon {
			var it model.ItemInstance
			var err error
			p.Inventory, it, err = inventory.Grant(p.Inventory, p.ID, d.ID, c.Item)
			if err != nil {
				return model.Player{}, err
			}
			p.Inventory, err = inventory.Equip(p.Inventory, it.ID, 0, c.Item)
			if err != nil {
				return model.Player{}, err
			}
			break
		}
	}
	for _, id := range c.definitions.StartingItems {
		var err error
		p.Inventory, _, err = inventory.Grant(p.Inventory, p.ID, id, c.Item)
		if err != nil {
			return model.Player{}, err
		}
	}
	return c.RefreshEquipment(p)
}
func (c *Catalog) RefreshEquipment(p model.Player) (model.Player, error) {
	if err := inventory.Validate(p.Inventory, c.Item); err != nil {
		return model.Player{}, err
	}
	return c.projectEquipment(p)
}

// PreviewEquipment uses the same source-aware projection as committed equipment.
// It never grants an item, changes bag limits or mutates the supplied player.
func (c *Catalog) PreviewEquipment(p model.Player, item model.ItemInstance, slot int) (model.Player, error) {
	v, err := inventory.PreviewEquip(p.Inventory, item, slot, c.Item)
	if err != nil {
		return model.Player{}, err
	}
	p.Inventory = v
	return c.projectEquipment(p)
}

func (c *Catalog) projectEquipment(p model.Player) (model.Player, error) {
	n := model.ClonePlayer(p)
	n.StatContributions = slices.DeleteFunc(n.StatContributions, func(x model.StatContribution) bool { return x.Source.Kind == "equipment" })
	for _, it := range n.Inventory.Items {
		if inventory.Equipped(n.Inventory, it.ID) {
			d, _ := c.Item(it.Definition)
			for _, m := range d.Modifiers {
				n.StatContributions = append(n.StatContributions, model.StatContribution{StatModifier: m, Source: model.SourceRef{Kind: "equipment", Actor: p.ID, Definition: d.ID, Instance: it.ID}})
			}
			if a, ok := c.Affix(it.Affix); ok {
				for _, m := range a.Modifiers {
					n.StatContributions = append(n.StatContributions, model.StatContribution{StatModifier: m, Source: model.SourceRef{Kind: "equipment", Actor: p.ID, Definition: a.ID, Instance: it.ID}})
				}
			}
		}
	}
	if it, ok := inventory.Find(n.Inventory, n.Inventory.Equipment[0]); ok {
		d, _ := c.Item(it.Definition)
		n.Weapon, n.Upgrade, n.WeaponBonus = d.Weapon, it.Upgrade, it.WeaponBonus
	} else {
		n.Weapon = ""
		n.Upgrade = 0
		n.WeaponBonus = 0
	}
	// Only explicitly bound timed statuses disappear on unequip.
	for _, old := range p.StatContributions {
		if old.Source.Kind == "equipment" && !inventory.Equipped(n.Inventory, old.Source.Instance) {
			n.Statuses, _ = status.RemoveBoundSource(n.Statuses, old.Source)
		}
	}
	for _, v := range n.Statuses.Instances {
		if v.BoundToSource && (v.Source.Kind == "weapon" || v.Source.Kind == "item" || v.Source.Kind == "equipment") && v.Source.Actor == n.ID && v.Source.Instance != "" && !inventory.Equipped(n.Inventory, v.Source.Instance) {
			n.Statuses, _ = status.RemoveBoundSource(n.Statuses, v.Source)
		}
	}
	return RefreshPlayer(n)
}
func (c *Catalog) Equip(p model.Player, id string, slot int) (model.Player, error) {
	n := model.ClonePlayer(p)
	var err error
	n.Inventory, err = inventory.Equip(p.Inventory, id, slot, c.Item)
	if err != nil {
		return model.Player{}, err
	}
	return c.RefreshEquipment(n)
}
func (c *Catalog) GrantItem(p model.Player, id string) (model.Player, error) {
	n := model.ClonePlayer(p)
	var err error
	n.Inventory, _, err = inventory.Grant(p.Inventory, p.ID, id, c.Item)
	if err != nil {
		return model.Player{}, err
	}
	return n, nil
}
func (c *Catalog) Sell(p model.Player, id string) (model.Player, error) {
	if inventory.Equipped(p.Inventory, id) {
		return model.Player{}, fmt.Errorf("unequip before selling")
	}
	n := model.ClonePlayer(p)
	var it model.ItemInstance
	var err error
	n.Inventory, it, err = inventory.Remove(p.Inventory, id, c.Item)
	if err != nil {
		return model.Player{}, err
	}
	d, _ := c.Item(it.Definition)
	_ = d
	n.Coins += c.ItemValue(it)
	if n.Coins > 1e9 {
		return model.Player{}, fmt.Errorf("coin limit")
	}
	return n, nil
}
func (c *Catalog) StoreWeaponUpgrade(p model.Player) (model.Player, error) {
	n := model.ClonePlayer(p)
	for i := range n.Inventory.Items {
		it := &n.Inventory.Items[i]
		if it.ID == n.Inventory.Equipment[0] {
			it.Upgrade = n.Upgrade
			it.WeaponBonus = n.WeaponBonus
			n.Inventory.Revision++
			return n, nil
		}
	}
	return model.Player{}, fmt.Errorf("no equipped weapon")
}
