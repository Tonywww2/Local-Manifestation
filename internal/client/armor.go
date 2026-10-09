package client

import (
	"fmt"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/inventory"
)

func (g *Game) buildArmor(f *uiFrame, layout ui.Layout) {
	r := g.windowPanel(f, layout, "armor", g.tr("ui.armor"), ui.Size{W: 360, H: 260})
	p := g.self()
	if p == nil {
		return
	}
	g.addLabel(f, "armor", ui.Rect{X: r.X + 14, Y: r.Y + 38, W: r.W - 28, H: 30}, g.tr("ui.armor.total", "value", fmt.Sprintf("%.2f", p.Armor)))
	for slot := 1; slot <= 4; slot++ {
		it, _ := inventory.Find(p.Inventory, p.Inventory.Equipment[slot])
		c := g.itemControl("armor", fmt.Sprintf("armor-slot:%d", slot), it, ui.Rect{X: r.X + 14 + float64(slot-1)*80, Y: r.Y + 85, W: 70, H: 62}, "equip-slot")
		c.Value = fmt.Sprint(slot)
		if it.ID == "" {
			c.Title = g.tr("ui.slot." + inventory.Slots[slot])
		}
		f.Controls = append(f.Controls, c)
	}
	addControl(f, "armor", "armor:bag", "backpack", "", "window", "backpack", g.tr("ui.backpack"), ui.Rect{X: r.X + 14, Y: r.Y + r.H - 48, W: 38, H: 32})
}
