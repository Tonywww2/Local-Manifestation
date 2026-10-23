package client

import (
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
)

func (g *Game) lootLists() [2][]model.Loot {
	var lists [2][]model.Loot
	if p := g.self(); p != nil {
		lists[0] = p.Loot
	}
	if g.state != nil {
		lists[1] = g.state.GroundLoot
	}
	return lists
}

// Ground controls use the same camera and world positions as the rendered drops.
// They are below HUD/windows and never draw an extra button over the item sprite.
func (g *Game) addLootControls(f *uiFrame, c Camera) {
	p := g.self()
	if p == nil {
		return
	}
	for _, list := range g.lootLists() {
		for _, drop := range list {
			if drop.Node != p.Node {
				continue
			}
			v := c.Project(drop.Position)
			size := max(24, 28*c.Scale)
			r := ui.Rect{X: v.X - size/2, Y: v.Y - size*.75, W: size, H: size}
			if r.X+r.W < 0 || r.Y+r.H < 0 || r.X > float64(g.width) || r.Y > float64(g.height) {
				continue
			}
			owner := "world-loot:" + drop.Item.ID
			control := uiControl{ID: owner, Owner: owner, Item: drop.Item, Action: "item-pickup", Value: drop.Item.ID, Bounds: r}
			// Ground sprites are already drawn by the world renderer. Only the
			// hovered item needs localized equipment comparisons and damage previews.
			if r.Contains(g.u.cursor) || g.cfg.SmokeFrames > 0 && g.cfg.SmokeHover == owner {
				control = g.itemControl(owner, owner, drop.Item, r, "item-pickup")
				hint := g.tr("error.loot_distance")
				if drop.Position.Sub(p.Position).Length() <= content.LootPickupRange {
					hint = g.tr("ui.loot.pickup_hint", "key", g.bindingLabel(g.bound("interact")))
				}
				control.Summary = append(append([]string(nil), control.Summary...), hint)
			}
			if p.Life == "alive" {
				f.Panels = append(f.Panels, ui.Panel{ID: owner, Bounds: r})
				f.Controls = append(f.Controls, control)
			} else {
				control.Action = ""
				f.Hints = append(f.Hints, control)
			}
		}
	}
}

func (g *Game) nearLoot() (model.Loot, bool) {
	p := g.self()
	if p == nil || p.Life != "alive" {
		return model.Loot{}, false
	}
	var nearest model.Loot
	distance, found := content.LootPickupRange, false
	for _, list := range g.lootLists() {
		for _, drop := range list {
			if drop.Node != p.Node {
				continue
			}
			if d := drop.Position.Sub(p.Position).Length(); d <= distance {
				nearest, distance, found = drop, d, true
			}
		}
	}
	return nearest, found
}
