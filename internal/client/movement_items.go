package client

import (
	"fmt"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
)

func (g *Game) selectedMovementItem() (model.ItemInstance, model.ItemDef, bool) {
	it, ok := inventory.Find(g.displayInventory(), g.u.movementItem)
	d, known := g.catalog.Item(it.Definition)
	if !ok || !known || d.Kind != "movement" || d.Movement == nil || !inventory.InBag(g.displayInventory(), it.ID) {
		g.u.movementItem = ""
		return model.ItemInstance{}, model.ItemDef{}, false
	}
	return it, d, true
}

func (g *Game) buildMovementSelector(f *uiFrame, bounds ui.Rect) {
	r := ui.Rect{X: bounds.X + bounds.W - 38, Y: bounds.Y + 6, W: 30, H: 30}
	title := g.tr("ui.movement.none")
	icon := "node-passage"
	var summary []string
	if it, d, ok := g.selectedMovementItem(); ok {
		title = g.contentName(d.ID, d.Name)
		icon = "movement"
		summary = []string{g.localized("content."+d.ID+".description", d.Description), g.tr("ui.item.durability", "remaining", it.Durability, "maximum", d.Durability)}
	}
	f.Controls = append(f.Controls, uiControl{Owner: "map", ID: "movement:selector", Bounds: r, Icon: icon, Title: g.tr("ui.movement.selected", "name", title), Summary: summary, Action: "movement-menu"})
}

func (g *Game) buildMovementMenu(f *uiFrame) {
	if !g.u.movementMenu {
		return
	}
	var bounds, selector ui.Rect
	for _, p := range f.Panels {
		if p.ID == "map" {
			bounds = mapContentBounds(p.Bounds)
		}
	}
	for _, c := range f.Controls {
		if c.Owner == "map" && c.ID == "movement:selector" {
			selector = c.Bounds
			break
		}
	}
	if bounds.W == 0 || selector.W == 0 {
		g.u.movementMenu = false
		return
	}
	bag := inventory.Bag(g.displayInventory())
	items := make([]model.ItemInstance, 0)
	for _, it := range bag {
		d, _ := g.catalog.Item(it.Definition)
		if d.Kind == "movement" {
			items = append(items, it)
		}
	}
	menuY := selector.Y + selector.H + 6
	// Keep the dropdown inside the resizable map, including its page controls.
	availableHeight := bounds.Y + bounds.H - menuY
	pageSize := max(1, min(7, int((availableHeight-38-34)/38)))
	pages := max(1, (len(items)+pageSize-1)/pageSize)
	page := min(max(0, g.u.scroll["movement-items"]), pages-1)
	g.u.scroll["movement-items"] = page
	height := 38 + float64(min(len(items), pageSize))*38
	if pages > 1 {
		height += 34
	}
	r := ui.Resolve(ui.Placement{Anchor: "top_left", X: selector.X + selector.W - 270, Y: menuY}, ui.Size{W: 270, H: height}, g.uiBounds())
	f.Panels = append(f.Panels, ui.Panel{ID: "movement-menu", Bounds: r})
	f.Controls = append(f.Controls, uiControl{Owner: "movement-menu", ID: "movement:none", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: 30}, Caption: g.tr("ui.movement.none"), Title: g.tr("ui.movement.none"), Action: "movement-select"})
	start := page * pageSize
	for i, it := range items[start:min(len(items), start+pageSize)] {
		c := g.itemControl("movement-menu", "movement:item:"+it.ID, it, ui.Rect{X: r.X + 4, Y: r.Y + 38 + float64(i)*38, W: r.W - 8, H: 34}, "movement-select")
		c.Caption = fmt.Sprintf("%s · %d", c.Title, it.Durability)
		f.Controls = append(f.Controls, c)
	}
	if pages > 1 {
		for i, direction := range []string{"-1", "1"} {
			f.Controls = append(f.Controls, uiControl{Owner: "movement-menu", ID: "movement:page:" + direction, Bounds: ui.Rect{X: r.X + 4 + float64(i)*132, Y: r.Y + r.H - 32, W: 128, H: 28}, Icon: []string{"scroll-left", "scroll-right"}[i], Caption: fmt.Sprintf("%d / %d", page+1, pages), Action: "inventory-page", Value: "movement-items:" + direction, Count: pages})
		}
	}
}
