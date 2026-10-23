package content

import (
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

// InventoryMouse computes the complete inventory and equipment projection before
// the player owner commits anything. Return is valid even for downed players.
func (c *Catalog) InventoryMouse(p model.Player, r protocol.Request) (model.Player, error) {
	n := model.ClonePlayer(p)
	var err error
	if r.Kind == "inventory_return" {
		n.Inventory, err = inventory.ReturnCursor(p.Inventory, c.Item)
	} else {
		if r.Revision != p.Inventory.Revision {
			return model.Player{}, protocol.Reject("error.inventory_changed")
		}
		if p.Life != "alive" {
			return model.Player{}, protocol.Reject("error.incapacitated")
		}
		slot := inventory.Slot{Area: r.Area, Index: r.Slot}
		if r.Kind == "inventory_quick" {
			n.Inventory, err = inventory.QuickTransfer(p.Inventory, slot, c.Item)
		} else {
			n.Inventory, err = inventory.Click(p.Inventory, p.ID, slot, r.Right, c.Item)
		}
	}
	if err != nil {
		return model.Player{}, err
	}
	return c.RefreshEquipment(n)
}
