package content

import (
	"errors"
	"fmt"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"slices"
)

const LootPickupRange = 90.0

// ValidateLoot shares inventory's component constraints for personal/shared drops.
func ValidateLoot(c *Catalog, drop model.Loot, now int64) error {
	if drop.Node == "" || !drop.Position.Finite() {
		return fmt.Errorf("invalid loot location")
	}
	d, ok := c.Item(drop.Item.Definition)
	if !ok || drop.Item.SkillReady > now+86400000*60 || drop.Item.SkillReady != 0 && c.weapons[d.Weapon].Skill == "" {
		return fmt.Errorf("invalid dropped weapon skill cooldown")
	}
	return inventory.Validate(model.Inventory{Capacity: 1, Items: []model.ItemInstance{drop.Item}, BagSlots: []string{drop.Item.ID}}, c.Item)
}

func (c *Catalog) Affix(id string) (model.AffixDef, bool) {
	d, ok := c.affixes[id]
	d.Modifiers = slices.Clone(d.Modifiers)
	d.DamageModifiers = slices.Clone(d.DamageModifiers)
	return d, ok
}
func (c *Catalog) ItemValue(it model.ItemInstance) int {
	d, _ := c.Item(it.Definition)
	a, _ := c.Affix(it.Affix)
	return (d.Value + a.Value + it.Upgrade*20) * inventory.Count(it)
}
func (c *Catalog) Drop(p model.Player, definition, node string, position model.Vec, roll float64) (model.Player, error) {
	if len(p.Loot) >= 4096 {
		return model.Player{}, fmt.Errorf("loot budget exceeded")
	}
	d, ok := c.Item(definition)
	if !ok {
		return model.Player{}, fmt.Errorf("unknown loot item")
	}
	n := model.ClonePlayer(p)
	n.Inventory.NextID++
	it := inventory.NewInstance(fmt.Sprintf("%s:%d", p.ID, n.Inventory.NextID), d)
	if len(d.Affixes) > 0 {
		it.Affix = d.Affixes[min(len(d.Affixes)-1, int(roll*float64(len(d.Affixes))))]
	}
	n.Loot = append(n.Loot, model.Loot{Item: it, Node: node, Position: position})
	return n, nil
}
func (c *Catalog) Pickup(p model.Player, id string) (model.Player, error) {
	if p.Life != "alive" {
		return model.Player{}, protocol.Reject("error.incapacitated")
	}
	n := model.ClonePlayer(p)
	index := slices.IndexFunc(n.Loot, func(x model.Loot) bool { return x.Item.ID == id })
	if index < 0 {
		return model.Player{}, protocol.Reject("error.loot_unavailable")
	}
	it := n.Loot[index]
	n, err := c.PickupDrop(n, it)
	if err != nil {
		return model.Player{}, err
	}
	n.Loot = slices.Delete(n.Loot, index, index+1)
	return n, nil
}

// PickupDrop computes receiving an existing, owner-provided drop. The caller
// removes that drop only after the complete player transaction has committed.
func (c *Catalog) PickupDrop(p model.Player, drop model.Loot) (model.Player, error) {
	if p.Life != "alive" {
		return model.Player{}, protocol.Reject("error.incapacitated")
	}
	if drop.Node != p.Node || drop.Position.Sub(p.Position).Length() > LootPickupRange {
		return model.Player{}, protocol.Reject("error.loot_distance")
	}
	n := model.ClonePlayer(p)
	var err error
	n.Inventory, _, err = inventory.Insert(n.Inventory, drop.Item, c.Item)
	if err != nil {
		if errors.Is(err, inventory.ErrFull) {
			return model.Player{}, protocol.Reject("error.loot_backpack_full")
		}
		return model.Player{}, err
	}
	return n, nil
}

// DropItem moves the entire existing stack, preserving its identity/components.
// It cannot leave half an equipment change committed when validation fails.
func (c *Catalog) DropItem(p model.Player, id string, revision uint64) (model.Player, model.Loot, error) {
	if p.Life != "alive" {
		return model.Player{}, model.Loot{}, protocol.Reject("error.incapacitated")
	}
	if revision != p.Inventory.Revision {
		return model.Player{}, model.Loot{}, protocol.Reject("error.inventory_changed")
	}
	if p.Node == "" || !p.Position.Finite() || p.Inventory.Cursor != "" {
		return model.Player{}, model.Loot{}, protocol.Reject("error.drop_unavailable")
	}
	n := model.ClonePlayer(p)
	var it model.ItemInstance
	var err error
	n.Inventory, it, err = inventory.Remove(p.Inventory, id, c.Item)
	if err != nil {
		return model.Player{}, model.Loot{}, err
	}
	n, err = c.RefreshEquipment(n)
	if err != nil {
		return model.Player{}, model.Loot{}, err
	}
	return n, model.Loot{Item: it, Node: p.Node, Position: p.Position}, nil
}
