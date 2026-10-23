package client

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/inventory"
)

func (g *Game) drawInterface(dst *ebiten.Image) {
	if g.u.editor != nil {
		g.drawHUDEditorGrid(dst)
	}
	for _, p := range g.u.frame.Panels {
		if strings.HasPrefix(p.ID, "world-device:") || strings.HasPrefix(p.ID, "world-loot:") {
			continue
		}
		r := p.Bounds
		if p.ID == "editor" {
			g.drawHUDEditorHandles(dst)
		}
		g.drawPanelSkin(dst, p)
		if p.Floating {
			g.fitText(dst, g.u.frame.Titles[p.ID], r.X+18, r.Y+4, r.W-54, 14, false)
		}
		if p.ID == "minimap" {
			if g.u.editor != nil && g.state == nil {
				g.drawHUDPreviewMap(dst, minimapContentBounds(r))
			} else {
				g.drawMapIn(dst, minimapContentBounds(r), true)
			}
		}
		if p.ID == "performance-hud" && g.u.editor != nil {
			g.drawPerformanceText(dst, r, "FPS 60  ·  "+g.tr("ui.performance.latency", "value", "32 ms"))
		}
		if p.ID == "map" {
			g.drawMapIn(dst, mapContentBounds(r), false)
			g.drawMapOverlay(dst, mapContentBounds(r))
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
				g.drawColoredText(clippedUI(dst, label.Clip), label.Text, x, label.Bounds.Y, label.Size, col)
			}
		}
		for _, formula := range g.u.frame.Formulas {
			if formula.Owner == p.ID {
				g.drawFormula(dst, formula)
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
	if g.u.editor == nil && g.state != nil && g.u.arrivalNode != "" && time.Now().Before(g.u.arrivalUntil) {
		r := ui.Rect{X: float64(g.width)/2 - 240, Y: float64(g.height) * .22, W: 480, H: 46}
		g.drawBox(dst, r, color.RGBA{24, 33, 36, 235}, uiEdge)
		g.fitText(dst, g.nodeName(g.u.arrivalNode), r.X+12, r.Y+8, r.W-24, 22, true)
	}
	g.drawTooltip(dst)
	if it, ok := inventory.Find(g.displayInventory(), g.displayInventory().Cursor); ok && g.u.editor == nil {
		c := g.itemControl("", "cursor-item", it, ui.Rect{X: g.u.cursor.X + 12, Y: g.u.cursor.Y + 14, W: 48, H: 42}, "")
		g.drawControl(dst, c)
	}
}
func (g *Game) drawControl(dst *ebiten.Image, c uiControl) {
	r := c.Bounds
	if c.Action == "window-resize" {
		g.drawIconSized(dst, c.Icon, r.X, r.Y, min(r.W, r.H))
		return
	}
	if isFormulaToken(c.ID) {
		if c.ID == g.u.hover {
			rect(dst, r.X, r.Y+r.H-4, r.W, 1, accent)
		}
		return
	}
	if c.InventoryStatus != nil {
		g.drawInventoryStatus(dst, c)
		return
	}
	if c.ID == "character:portrait" {
		g.drawCharacterPortrait(dst, c)
		return
	}
	if strings.HasPrefix(c.ID, "attribute-info:") {
		rect(dst, r.X, r.Y+5, 2, r.H-10, semanticColor(strings.TrimPrefix(c.ID, "attribute-info:"), uiInk))
		if c.ID == g.u.hover {
			rect(dst, r.X+6, r.Y+r.H-2, r.W-6, 1, uiEdge)
		}
		return
	}
	if c.ID == "minimap" || c.ID == "self" || c.ID == "wealth" || c.ID == "journey" || strings.HasPrefix(c.ID, "map-node:") {
		return
	}
	g.drawControlSkin(dst, c)
	if c.Peer != nil {
		g.drawHUDPeer(dst, r, *c.Peer)
		return
	}
	if c.Action == "chassis-preset" {
		g.drawPresetCard(dst, c)
		return
	}
	if c.Caption != "" {
		if c.ID == "backpack:attributes" {
			g.drawIconSized(dst, "person", r.X+8, r.Y+7, 20)
			g.fitText(dst, c.Caption, r.X+36, r.Y+7, r.W-62, 13, false)
			if c.Badge != "" {
				g.fitText(dst, c.Badge, r.X+r.W-24, r.Y+7, 20, 12, true)
			}
			return
		}
		if c.Action == "chassis" {
			g.drawIconSized(dst, c.Icon, r.X+7, r.Y+7, 18)
			g.fitText(dst, strings.Trim(c.Caption, "[] "), r.X+34, r.Y+6, r.W-40, 12, false)
			return
		}
		if strings.HasPrefix(c.ID, "main:") {
			size := min(24, r.H*.42)
			x := r.X + (r.W-measuredText(c.Caption, size))/2
			g.drawColoredText(dst, c.Caption, x, r.Y+(r.H-size)/2-2, size, menuInk)
			return
		}
		g.fitText(dst, c.Caption, r.X+8, r.Y+(r.H-20)/2, r.W-16, 15, true)
		return
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
	if c.Item.Definition != "" && g.resources.images["icon:"+c.Item.Definition] != nil {
		size = min(r.W-10, r.H-10)
		if c.Key != "" {
			size = min(size, r.H-24)
			y = r.Y + 4
		} else {
			y = r.Y + (r.H-size)/2
		}
	}
	if c.Slot == "" || c.Count > 0 {
		g.drawIconSized(dst, c.Icon, r.X+(r.W-size)/2, y, size)
	}
	if c.Count > 1 && c.Action != "inventory-page" {
		g.fitText(dst, fmt.Sprint(c.Count), r.X+r.W-25, r.Y+3, 22, 11, true)
	}
	if c.Key != "" {
		g.drawKeycap(dst, c.Key, ui.Rect{X: r.X + 3, Y: r.Y + r.H - 18, W: r.W - 6, H: 16})
		if c.Badge != "" {
			rect(dst, r.X+3, r.Y+size-4, r.W-6, 15, color.RGBA{25, 33, 36, 240})
			g.fitText(dst, c.Badge, r.X+3, r.Y+size-6, r.W-6, 11, true)
		}
	} else if c.Badge != "" {
		g.fitText(dst, c.Badge, r.X+3, r.Y+r.H-16, r.W-6, 11, true)
	}
}

func (g *Game) drawKeycap(dst *ebiten.Image, key string, r ui.Rect) {
	w := min(r.W, max(22, measuredText(key, 11)+10))
	r.X += (r.W - w) / 2
	r.W = w
	g.drawBox(dst, r, color.RGBA{48, 59, 62, 255}, uiEdge)
	g.fitText(dst, key, r.X+2, r.Y-1, r.W-4, 11, true)
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
	dst = clippedUI(dst, r)
	target := "map"
	if mini {
		target = "minimap"
	}
	positions, radius := g.mapLayout(r, target)
	visible := g.visibleMapNodes()
	for _, t := range g.state.Terrain {
		v := positions[terrainID(t)]
		hexagon(dst, v, radius, color.RGBA{29, 34, 42, 255}, g.cameraAngle())
		if !mini {
			g.fitText(dst, g.localized(t.NameKey, t.Name), v.X-radius, v.Y-8, radius*2, 10, true)
		}
	}
	for _, n := range g.state.Nodes {
		for _, edge := range n.Edges {
			if to, ok := positions[edge]; ok && n.ID < edge {
				drawMapConnection(dst, positions[n.ID], to, radius, g.cameraAngle(), color.RGBA{58, 66, 75, 255})
			}
		}
	}
	selectedTarget := g.u.frame.MapTargets[g.u.selectedNode]
	if !mini && g.u.movementItem == "" && (selectedTarget.Direct || selectedTarget.Vote) {
		previous := g.currentNode().ID
		for _, id := range g.personalRoute(g.u.selectedNode) {
			drawMapConnection(dst, positions[previous], positions[id], radius, g.cameraAngle(), accent)
			previous = id
		}
	}
	for _, n := range g.state.Nodes {
		v := positions[n.ID]
		c, icon := mapNodePresentation(n, visible[n.ID])
		target := g.u.frame.MapTargets[n.ID]
		inset := 0.0
		if target.Direct || target.Vote {
			border := hudGold
			if target.Direct {
				border = accent
			}
			hexagon(dst, v, radius, border, g.cameraAngle())
			inset = min(radius*.12, 2.5)
			if mini {
				inset = min(radius*.16, 1.5)
			}
		}
		if n.ID == g.u.selectedNode {
			hexagon(dst, v, max(1, radius-inset), uiInk, g.cameraAngle())
			inset += min(radius*.08, 1.5)
			hexagon(dst, v, max(1, radius-inset), c, g.cameraAngle())
		} else {
			hexagon(dst, v, max(1, radius-inset), c, g.cameraAngle())
		}
		size := radius * 1.15
		g.drawIconSized(dst, icon, v.X-size/2, v.Y-size/2, size)
		if visible[n.ID] && g.mapCompletionMark(n) {
			g.drawIconSized(dst, "check", v.X+radius*.35, v.Y-radius*.65, radius*.45)
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

func (g *Game) drawHUDPeer(dst *ebiten.Image, r ui.Rect, p hudPeerView) {
	if p.SameRoom {
		g.drawBox(dst, r, color.RGBA{34, 52, 55, 244}, accent)
	}
	g.drawIconSized(dst, "person", r.X+9, r.Y+12, 18)
	g.fitText(dst, p.Name, r.X+36, r.Y+7, r.W-115, 13, false)
	g.fitText(dst, p.Location, r.X+r.W-81, r.Y+9, 72, 10, true)
	drawMeterSkin(dst, uiMeter{Bounds: ui.Rect{X: r.X + 37, Y: r.Y + 30, W: r.W - 132, H: 5}, Fraction: p.HP / mathMaxHP(p.MaxHP), Kind: "peer"})
}
