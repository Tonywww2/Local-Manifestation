package client

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
)

var hudGold = color.RGBA{231, 207, 148, 255}
var uiEdge = color.RGBA{101, 117, 121, 255}
var uiInk = color.RGBA{238, 241, 239, 255}
var menuInk = color.RGBA{43, 51, 53, 255}

func unframedHUD(id string) bool {
	switch id {
	case "statuses", "skills", "consumables", "equipment", "tools", "resources", "party", "view", "main-menu", "hall-heading", "hall-wealth":
		return true
	}
	return false
}

// Shared display treatment; layout and hit testing remain value transformations.
func (g *Game) drawPanelSkin(dst *ebiten.Image, p ui.Panel) {
	if unframedHUD(p.ID) {
		return
	}
	r := p.Bounds
	g.drawBox(dst, ui.Rect{X: r.X + 2, Y: r.Y + 3, W: r.W, H: r.H}, color.RGBA{0, 0, 0, 40}, color.RGBA{0, 0, 0, 40})
	g.drawBox(dst, r, g.panelColor(), uiEdge)
	if p.Floating {
		rect(dst, r.X+12, r.Y+30, r.W-24, 1, color.RGBA{78, 92, 95, 255})
		for i := 0; i < 6; i++ {
			rect(dst, r.X+r.W/2+float64(i%3)*5, r.Y+11+float64(i/3)*5, 2, 2, muted)
		}
	}
}

func (g *Game) drawControlSkin(dst *ebiten.Image, c uiControl) {
	r := c.Bounds
	fill := color.RGBA{38, 47, 50, 238}
	edge := uiEdge
	if c.Icon == "empty" {
		fill = color.RGBA{32, 40, 43, 220}
		edge = color.RGBA{76, 89, 92, 255}
	}
	if c.ID == g.u.hover {
		fill, edge = color.RGBA{48, 66, 68, 250}, accent
	}
	if strings.HasPrefix(c.ID, "chassis:") && strings.HasPrefix(c.Caption, "[ ") {
		edge = accent
	}
	if strings.HasPrefix(c.ID, "main:") {
		fill, edge = color.RGBA{230, 228, 220, 255}, color.RGBA{117, 126, 124, 255}
		if c.ID == "main:start" {
			fill, edge = color.RGBA{159, 196, 194, 255}, color.RGBA{159, 196, 194, 255}
		}
		if c.ID == g.u.hover {
			fill, edge = color.RGBA{184, 213, 209, 255}, color.RGBA{86, 144, 143, 255}
		}
	}
	if strings.HasSuffix(c.ID, ":close") {
		if c.ID == g.u.hover {
			g.drawBox(dst, r, fill, edge)
		}
		return
	}
	g.drawBox(dst, r, fill, edge)
}

func drawMeterSkin(dst *ebiten.Image, meter uiMeter) {
	r := meter.Bounds
	fill := color.RGBA{210, 79, 86, 255}
	switch meter.Kind {
	case "shield":
		fill = color.RGBA{100, 210, 240, 255}
	case "energy":
		fill = color.RGBA{130, 116, 187, 255}
	case "xp":
		fill = accent
	case "peer":
		fill = color.RGBA{151, 207, 215, 255}
	}
	rect(dst, r.X-1, r.Y-1, r.W+2, r.H+2, uiEdge)
	rect(dst, r.X, r.Y, r.W, r.H, color.RGBA{25, 33, 36, 255})
	w := r.W * max(0, min(1, meter.Fraction))
	if w > 0 {
		rect(dst, r.X, r.Y, w, r.H, fill)
		rect(dst, r.X, r.Y, w, 1, color.NRGBA{255, 255, 255, 45})
	}
}
