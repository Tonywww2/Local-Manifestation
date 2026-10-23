package client

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/pkg/model"
)

// Slot diagrams are separate from item art, including their resource-pack keys.
// Empty slots have a large diagram; occupied slots keep only a corner type mark.
func (g *Game) drawEquipmentSlot(dst *ebiten.Image, c uiControl) {
	r, kind := c.Bounds, c.Slot
	if strings.HasPrefix(kind, "consumable") {
		kind = "consumable"
	}
	empty := c.Count == 0
	key := "slot:" + kind
	if empty && g.drawAsset(dst, key, model.Vec{X: r.X + r.W/2, Y: r.Y + r.H/2}, r.W, r.H) {
		return
	}
	key = "slot-background:" + kind
	if g.resources.glyphs == nil {
		g.resources.glyphs = map[string]*ebiten.Image{}
	}
	texture := g.resources.glyphs[key]
	if texture == nil {
		texture = ebiten.NewImage(64, 64)
		tint := slotTint(kind)
		rect(texture, 3, 3, 58, 58, color.RGBA{29, 38, 41, 255})
		rect(texture, 3, 3, 3, 58, tint)
		for _, p := range [][4]float64{{10, 8, 14, 1}, {10, 8, 1, 6}, {40, 55, 14, 1}, {53, 49, 1, 6}} {
			rect(texture, p[0], p[1], p[2], p[3], tint)
		}

		g.resources.glyphs[key] = texture
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(r.W/64, r.H/64)
	op.GeoM.Translate(r.X, r.Y)
	dst.DrawImage(texture, op)
	symbolKey := "slot-symbol:" + kind
	symbol := g.resources.glyphs[symbolKey]
	if symbol == nil {
		symbol = ebiten.NewImage(64, 64)
		renderSlotSymbol(symbol, kind, 64, slotTint(kind))
		g.resources.glyphs[symbolKey] = symbol
	}
	size, x, y := 12.0, r.X+4, r.Y+4
	if empty {
		available := r.H - 8
		if c.Key != "" {
			available -= 14
		}
		size = min(r.W-12, available) * .88
		x, y = r.X+(r.W-size)/2, r.Y+4+(available-size)/2
	}
	symbolOptions := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	symbolOptions.GeoM.Scale(size/64, size/64)
	symbolOptions.GeoM.Translate(x, y)
	if empty {
		symbolOptions.ColorScale.ScaleAlpha(.65)
	}
	dst.DrawImage(symbol, symbolOptions)

	if c.ID == g.u.hover {
		g.drawBox(dst, r, color.RGBA{}, accent)
	}
}

// Neutral mounting diagrams deliberately do not reuse renderGlyph(item kind).
func renderSlotSymbol(dst *ebiten.Image, kind string, size float64, tint color.RGBA) {
	s := size / 24
	p := func(x, y float64) model.Vec { return model.Vec{X: (x + 2) * s, Y: (y + 2) * s} }
	a := func(x, y, u, v float64) { line(dst, p(x, y), p(u, v), tint, float32(1.1*s)) }
	path := func(coords ...float64) {
		for i := 2; i < len(coords); i += 2 {
			a(coords[i-2], coords[i-1], coords[i], coords[i+1])
		}
	}
	circle := func(x, y, r float64) {
		v := p(x, y)
		vector.StrokeCircle(dst, float32(v.X), float32(v.Y), float32(r*s), float32(1.1*s), tint, true)
	}
	switch kind {
	case "weapon":
		path(1, 8, 17, 8, 19, 10, 17, 12, 1, 12)
		a(5, 5, 5, 15)
		a(14, 5, 14, 15)
		a(1, 7, 1, 13)
	case "head":
		path(3, 13, 3, 7, 6, 3, 14, 3, 17, 7, 17, 13)
		path(5, 13, 7, 17, 13, 17, 15, 13)
		a(3, 9, 7, 9)
		a(13, 9, 17, 9)
		a(8, 14, 12, 14)
		a(9, 1, 11, 1)
	case "body":
		path(10, 1, 10, 4, 2, 9, 18, 9, 10, 4)
		path(5, 11, 5, 18, 15, 18, 15, 11)
		a(10, 11, 10, 16)
	case "legs":
		path(3, 2, 17, 2, 17, 5, 3, 5, 3, 2)
		path(5, 8, 4, 18, 8, 18, 10, 8, 12, 18, 16, 18, 15, 8)
		a(7, 3, 7, 4)
		a(13, 3, 13, 4)
	case "feet":
		path(3, 3, 7, 3, 8, 14, 7, 18, 3, 18, 2, 14, 3, 3)
		path(13, 3, 17, 3, 18, 14, 17, 18, 13, 18, 12, 14, 13, 3)
		a(3, 14, 7, 14)
		a(13, 14, 17, 14)
	case "accessory":
		path(10, 2, 17, 9, 10, 17, 3, 9, 10, 2)
		circle(10, 9, 3)
	default:
		path(7, 2, 13, 2, 13, 5, 15, 7, 15, 18, 5, 18, 5, 7, 7, 5, 7, 2)
		a(7, 11, 13, 11)
		a(10, 8, 10, 14)
	}
}

func slotTint(kind string) color.RGBA {
	switch kind {
	case "weapon":
		return color.RGBA{200, 151, 100, 255}
	case "head":
		return color.RGBA{118, 169, 205, 255}
	case "body":
		return color.RGBA{116, 186, 181, 255}
	case "legs":
		return color.RGBA{154, 184, 130, 255}
	case "feet":
		return color.RGBA{197, 181, 124, 255}
	case "accessory":
		return hudGold
	default:
		return color.RGBA{171, 149, 202, 255}
	}
}
