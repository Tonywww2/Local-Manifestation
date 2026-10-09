package client

import (
	"fmt"
	"strconv"
	"strings"

	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

// A detached display projection makes rapid mouse operations responsive. Every
// intent still carries its expected server revision; rejected projections reset.
func (g *Game) displayInventory() model.Inventory {
	if g.u.mouseInventory != nil {
		return *g.u.mouseInventory
	}
	if p := g.self(); p != nil {
		return p.Inventory
	}
	return model.Inventory{}
}
func accessoryDisplaySlots(v model.Inventory) int {
	if v.AccessorySlots < 0 {
		return len(v.Accessories) + 1
	}
	return min(v.AccessorySlots, len(v.Accessories)+1)
}
func (g *Game) inventoryClick(c uiControl, right bool) {
	p := g.self()
	if p == nil || g.link == nil {
		return
	}
	index, err := strconv.Atoi(c.Value)
	if err != nil {
		return
	}
	v := g.displayInventory()
	area := strings.TrimPrefix(c.Action, "inventory-")
	n, err := inventory.Click(v, p.ID, inventory.Slot{Area: area, Index: index}, right, g.catalog.Item)
	if err != nil {
		g.notifyMessage(protocol.ErrorMessage(err))
		return
	}
	if n.Revision == v.Revision {
		return
	}
	if !g.sendItem(protocol.Request{Kind: "inventory_click", Area: area, Slot: index, Right: right, Revision: v.Revision}) {
		g.notify("error.inventory_queue")
		return
	}
	if v.Cursor != "" {
		g.u.selectedItem = v.Cursor
	} else {
		g.u.selectedItem = n.Cursor
	}
	g.u.mouseInventory = &n
}
func (g *Game) returnInventoryCursor() {
	if p := g.self(); p != nil && g.link != nil {
		g.u.returnCursorPending = !g.sendItem(protocol.Request{Kind: "inventory_return"})
		if g.u.returnCursorPending {
			return
		}
		v := g.displayInventory()
		if n, err := inventory.ReturnCursor(v, g.catalog.Item); err == nil && v.Cursor != "" {
			g.u.mouseInventory = &n
		}
	}
}
func (g *Game) statModifierText(m model.StatModifier) string {
	name := g.localized("attribute."+m.Stat, m.Stat)
	switch m.Operation {
	case "multiply":
		return fmt.Sprintf("%s × %.2f", name, m.Value)
	case "percent":
		return fmt.Sprintf("%s %+.2f%%", name, m.Value*100)
	default:
		return fmt.Sprintf("%s %+.2f", name, m.Value)
	}
}
