package client

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/inventory"
)

func (g *Game) drawInterface(dst *ebiten.Image) {
	for _, p := range g.u.frame.Panels {
		if strings.HasPrefix(p.ID, "world-device:") {
			continue
		}
		r := p.Bounds
		g.drawPanelSkin(dst, p)
		if p.Floating {
			fitText(dst, g.u.frame.Titles[p.ID], r.X+18, r.Y+4, r.W-54, 14, false)
		}
		if g.u.editor != nil && p.ID != "editor" {
			vector.StrokeRect(dst, float32(r.X), float32(r.Y), float32(r.W), float32(r.H), 1, accent, false)
		}
		if p.ID == "minimap" {
			g.drawMapIn(dst, ui.Rect{X: r.X + 6, Y: r.Y + 6, W: r.W - 12, H: r.H - 40}, true)
		}
		if p.ID == "map" {
			g.drawMapIn(dst, ui.Rect{X: r.X + 12, Y: r.Y + 40, W: r.W - 24, H: r.H - 120}, false)
		}
		if p.ID == "hall-heading" {
			rect(dst, r.X, r.Y+3, 3, 54, accent)
		}
		if p.ID == "hall-wealth" {
			g.drawIconSized(dst, "meta-points", r.X, r.Y+5, 25)
		}
		for _, meter := range g.u.frame.Meters {
			if meter.Owner == p.ID {
				drawMeterSkin(dst, meter)
			}
		}
		for _, label := range g.u.frame.Labels {
			if label.Owner == p.ID {
				col := label.Color
				if col.A == 0 {
					col = uiInk
				}
				x := label.Bounds.X
				if label.Centered {
					x += max(0, (label.Bounds.W-measuredText(label.Text, label.Size))/2)
				}
				drawColoredText(clippedUI(dst, label.Clip), label.Text, x, label.Bounds.Y, label.Size, col)
			}
		}
		for _, dot := range g.u.frame.Dots {
			if dot.Owner == p.ID {
				if dot.Available {
					vector.DrawFilledCircle(dst, float32(dot.Center.X), float32(dot.Center.Y), 5, accent, true)
				} else {
					vector.StrokeCircle(dst, float32(dot.Center.X), float32(dot.Center.Y), 5, 1.5, muted, true)
				}
			}
		}
		for _, c := range g.u.frame.Controls {
			if c.Owner == p.ID {
				g.drawControl(clippedUI(dst, c.Clip), c)
			}
		}
	}
	g.drawTooltip(dst)
	if it, ok := inventory.Find(g.displayInventory(), g.displayInventory().Cursor); ok {
		c := g.itemControl("", "cursor-item", it, ui.Rect{X: g.u.cursor.X + 12, Y: g.u.cursor.Y + 14, W: 48, H: 42}, "")
		g.drawControl(dst, c)
	}
}
func (g *Game) drawControl(dst *ebiten.Image, c uiControl) {
	r := c.Bounds
	if c.InventoryStatus != nil {
		g.drawInventoryStatus(dst, c)
		return
	}
	if c.ID == "character:portrait" {
		g.drawCharacterPortrait(dst, c)
		return
	}
	if strings.HasPrefix(c.ID, "attribute-info:") {
		vector.StrokeRect(dst, float32(r.X), float32(r.Y), float32(r.W), float32(r.H), 1, uiEdge, false)
		return
	}
	if c.ID == "minimap" || c.ID == "self" || c.ID == "wealth" || c.ID == "journey" || strings.HasPrefix(c.ID, "map-node:") {
		return
	}
	g.drawControlSkin(dst, c)
	if c.Action == "chassis-preset" {
		g.drawPresetCard(dst, c)
		return
	}
	if c.Caption != "" {
		if c.ID == "backpack:attributes" {
			g.drawIconSized(dst, "person", r.X+8, r.Y+7, 20)
			fitText(dst, c.Caption, r.X+36, r.Y+7, r.W-62, 13, false)
			if c.Badge != "" {
				fitText(dst, c.Badge, r.X+r.W-24, r.Y+7, 20, 12, true)
			}
			return
		}
		if c.Action == "chassis" {
			g.drawIconSized(dst, c.Icon, r.X+7, r.Y+7, 18)
			fitText(dst, strings.Trim(c.Caption, "[] "), r.X+34, r.Y+6, r.W-40, 12, false)
			return
		}
		if strings.HasPrefix(c.ID, "main:") {
			size := min(24, r.H*.42)
			x := r.X + (r.W-measuredText(c.Caption, size))/2
			drawColoredText(dst, c.Caption, x, r.Y+(r.H-size)/2-2, size, menuInk)
			return
		}
		fitText(dst, c.Caption, r.X+8, r.Y+(r.H-20)/2, r.W-16, 15, true)
		return
	}
	if strings.HasPrefix(c.ID, "peer:") {
		id := strings.TrimPrefix(c.ID, "peer:")
		for _, p := range g.state.Players {
			if p.ID == id {
				if g.sameRoom(p) {
					g.drawBox(dst, r, color.RGBA{34, 52, 55, 244}, accent)
				}
				name := p.Name
				if !p.Online {
					name += " · " + g.tr("online.no")
				}
				g.drawIconSized(dst, "person", r.X+9, r.Y+12, 18)
				fitText(dst, name, r.X+36, r.Y+7, r.W-115, 13, false)
				location := g.tr("ui.peer.other")
				if g.sameRoom(p) {
					location = g.tr("ui.peer.same")
				}
				if p.Life != "alive" {
					location = g.tr("life." + p.Life)
				}
				fitText(dst, location, r.X+r.W-81, r.Y+9, 72, 10, true)
				drawMeterSkin(dst, uiMeter{Bounds: ui.Rect{X: r.X + 37, Y: r.Y + 30, W: r.W - 132, H: 5}, Fraction: p.HP / mathMaxHP(p.MaxHP), Kind: "peer"})
				return
			}
		}
	}
	if c.Key != "" && r.H < 40 && r.W >= 64 {
		g.drawIconSized(dst, c.Icon, r.X+8, r.Y+7, 22)
		g.drawKeycap(dst, c.Key, ui.Rect{X: r.X + 36, Y: r.Y + 7, W: r.W - 42, H: 22})
		return
	}
	size := min(32, min(r.W-14, r.H-18))
	y := r.Y + 5
	if c.Key == "" && c.Badge == "" {
		size = min(30, min(r.W-8, r.H-8))
		y = r.Y + (r.H-size)/2
	}
	g.drawIconSized(dst, c.Icon, r.X+(r.W-size)/2, y, size)
	if c.Count > 1 && c.Action != "inventory-page" {
		fitText(dst, fmt.Sprint(c.Count), r.X+r.W-25, r.Y+3, 22, 11, true)
	}
	if c.Key != "" {
		g.drawKeycap(dst, c.Key, ui.Rect{X: r.X + 3, Y: r.Y + r.H - 18, W: r.W - 6, H: 16})
		if c.Badge != "" {
			rect(dst, r.X+3, r.Y+size-4, r.W-6, 15, color.RGBA{25, 33, 36, 240})
			fitText(dst, c.Badge, r.X+3, r.Y+size-6, r.W-6, 11, true)
		}
	} else if c.Badge != "" {
		fitText(dst, c.Badge, r.X+3, r.Y+r.H-16, r.W-6, 11, true)
	}
}

func (g *Game) drawKeycap(dst *ebiten.Image, key string, r ui.Rect) {
	w := min(r.W, max(22, measuredText(key, 11)+10))
	r.X += (r.W - w) / 2
	r.W = w
	g.drawBox(dst, r, color.RGBA{48, 59, 62, 255}, uiEdge)
	fitText(dst, key, r.X+2, r.Y-1, r.W-4, 11, true)
}

// Glyph textures are cached separately from content assets; a resource reload
// clears the cache and material packs can still replace icon:<name>.
func (g *Game) drawIcon(dst *ebiten.Image, id string, x, y float64) {
	g.drawIconSized(dst, id, x, y, 18)
}
func (g *Game) drawMapIn(dst *ebiten.Image, r ui.Rect, mini bool) {
	if g.state == nil {
		return
	}
	positions, radius := mapPositionsWithTerrain(g.state.Nodes, g.state.Terrain, r)
	for _, t := range g.state.Terrain {
		v := positions[terrainID(t)]
		hexagon(dst, v, radius, color.RGBA{29, 34, 42, 255})
		if !mini {
			fitText(dst, g.localized(t.NameKey, t.Name), v.X-radius, v.Y-8, radius*2, 10, true)
		}
	}
	for _, n := range g.state.Nodes {
		for _, edge := range n.Edges {
			if n.ID < edge {
				line(dst, positions[n.ID], positions[edge], muted, 1)
			}
		}
	}
	for _, n := range g.state.Nodes {
		v := positions[n.ID]
		c := color.RGBA{50, 70, 90, 255}
		if n.Completed {
			c = color.RGBA{40, 110, 90, 255}
		}
		if n.Kind == "passage" {
			c = color.RGBA{60, 72, 65, 255}
		}
		if n.ID == g.u.selectedNode {
			c = accent
		}
		hexagon(dst, v, radius, c)
		if !mini {
			fitText(dst, g.nodeName(n.ID), v.X-radius, v.Y-8, radius*2, 12, true)
		}
		count := 0
		for _, p := range g.state.Players {
			if p.Node != n.ID || !p.Active {
				continue
			}
			col := color.RGBA{118, 166, 247, 255}
			if p.ID == g.id {
				col = color.RGBA{255, 255, 255, 255}
			}
			if p.Life != "alive" {
				col = muted
			}
			rect(dst, v.X-radius*.7+float64(count%4)*4, v.Y+radius*.4+float64(count/4)*4, 3, 3, col)
			count++
		}
	}
}
