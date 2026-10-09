package content

import (
	"fmt"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"slices"
)

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
	it := model.ItemInstance{ID: fmt.Sprintf("%s:%d", p.ID, n.Inventory.NextID), Definition: definition, Count: 1}
	if len(d.Affixes) > 0 {
		it.Affix = d.Affixes[min(len(d.Affixes)-1, int(roll*float64(len(d.Affixes))))]
	}
	n.Loot = append(n.Loot, model.Loot{Item: it, Node: node, Position: position})
	return n, nil
}
func (c *Catalog) Pickup(p model.Player, id string) (model.Player, error) {
	n := model.ClonePlayer(p)
	index := slices.IndexFunc(n.Loot, func(x model.Loot) bool { return x.Item.ID == id })
	if index < 0 {
		return model.Player{}, fmt.Errorf("loot unavailable")
	}
	it := n.Loot[index]
	if it.Node != p.Node || it.Position.Sub(p.Position).Length() > 90 {
		return model.Player{}, fmt.Errorf("move closer to this drop")
	}
	var err error
	n.Inventory, _, err = inventory.Insert(n.Inventory, it.Item, c.Item)
	if err != nil {
		return model.Player{}, err
	}
	n.Loot = slices.Delete(n.Loot, index, index+1)
	return n, nil
}
