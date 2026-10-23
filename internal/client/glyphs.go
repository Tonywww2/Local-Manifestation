package client

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/pkg/model"
)

func (g *Game) drawIconSized(dst *ebiten.Image, id string, x, y, size float64) {
	if texture := g.resources.sprites["icon:"+id]; texture != nil {
		// Long weapons use the diagonal of the slot rather than becoming a thin
		// horizontal line. Crop/metadata is cached at resource publication.
		const angle = -math.Pi / 5
		b := texture.Bounds()
		w, h := float64(b.Dx()), float64(b.Dy())
		scale := size / max(w*math.Cos(angle)+h*math.Abs(math.Sin(angle)), w*math.Abs(math.Sin(angle))+h*math.Cos(angle))
		drawActorTexture(dst, texture, model.Vec{X: x + size/2, Y: y + size/2}, w*scale, h*scale, angle, 1, false)
		return
	}
	if g.drawAsset(dst, "icon:"+id, model.Vec{X: x + size/2, Y: y + size/2}, size, size) {
		return
	}
	if g.resources.glyphs == nil {
		g.resources.glyphs = map[string]*ebiten.Image{}
	}
	img := g.resources.glyphs[id]
	if img == nil {
		img = ebiten.NewImage(64, 64)
		renderGlyph(img, id, 64)
		g.resources.glyphs[id] = img
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(size/64, size/64)
	op.GeoM.Translate(x, y)
	dst.DrawImage(img, op)
}

// Coordinates are a small value vocabulary, rendered once at high resolution.
// Icon appearance carries no content behavior or gameplay lookup.
func renderGlyph(dst *ebiten.Image, id string, size float64) {
	s := size / 20
	point := func(x, y float64) model.Vec { return model.Vec{X: (x + 1) * s, Y: (y + 1) * s} }
	a := func(x, y, u, v float64) { line(dst, point(x, y), point(u, v), uiInk, float32(1.35*s)) }
	path := func(coords ...float64) {
		for i := 2; i < len(coords); i += 2 {
			a(coords[i-2], coords[i-1], coords[i], coords[i+1])
		}
	}
	circle := func(x, y, r float64) {
		p := point(x, y)
		vector.StrokeCircle(dst, float32(p.X), float32(p.Y), float32(r*s), float32(1.35*s), uiInk, true)
	}
	box := func(x, y, w, h float64) {
		p := point(x, y)
		vector.StrokeRect(dst, float32(p.X), float32(p.Y), float32(w*s), float32(h*s), float32(1.3*s), uiInk, false)
	}
	switch id {
	case "resize":
		a(6, 17, 17, 6)
		a(11, 17, 17, 11)
		a(16, 17, 17, 16)
	case "projectile-cut":
		path(3, 16, 14, 2, 17, 1, 17, 5, 7, 14)
		a(3, 10, 9, 15)
		circle(4, 3, 1.4)
		circle(1, 8, 1.1)
		path(9, 2, 5, 6, 2, 12)
	case "haste":
		circle(11, 2, 1.5)
		path(3, 7, 8, 7, 11, 5, 14, 8, 17, 8)
		path(10, 6, 8, 11, 4, 13, 2, 17)
		path(8, 11, 12, 12, 10, 17)
		a(0, 10, 4, 10)
	case "burst":
		path(9, 1, 11, 7, 17, 5, 13, 10, 17, 15, 11, 13, 8, 18, 7, 12, 1, 14, 5, 9, 1, 4, 7, 6, 9, 1)
	case "heart":
		p := point(7, 1)
		rect(dst, p.X, p.Y, 4*s, 16*s, uiInk)
		p = point(1, 7)
		rect(dst, p.X, p.Y, 16*s, 4*s, uiInk)
	case "root", "progression":
		path(9, 17, 9, 3)
		path(3, 5, 3, 9, 15, 9, 15, 5)
		circle(3, 3, 2)
		circle(15, 3, 2)
		circle(9, 15, 2)
	case "consumable":
		path(6, 1, 12, 1, 12, 6, 15, 10, 15, 17, 3, 17, 3, 10, 6, 6, 6, 1)
		a(4, 12, 14, 12)
	case "coins", "meta-points":
		p := point(9, 9)
		vector.StrokeCircle(dst, float32(p.X), float32(p.Y), float32(8*s), float32(1.5*s), hudGold, true)
		if id == "meta-points" {
			path(9, 3, 11, 7, 15, 8, 12, 11, 13, 15, 9, 13, 5, 15, 6, 11, 3, 8, 7, 7, 9, 3)
		} else {
			a(7, 5, 7, 13)
			a(11, 5, 11, 13)
			a(5, 8, 13, 8)
			a(5, 11, 13, 11)
		}
	case "close", "cancel", "reject":
		a(3, 3, 15, 15)
		a(15, 3, 3, 15)
	case "check", "approve":
		path(2, 9, 7, 14, 17, 3)
	case "plus":
		a(9, 2, 9, 16)
		a(2, 9, 16, 9)
	case "weapon":
		path(3, 16, 6, 12, 14, 2, 17, 1, 17, 5, 7, 14)
		a(3, 10, 9, 15)
	case "ranged":
		path(2, 4, 8, 9, 2, 14)
		a(2, 4, 2, 14)
		a(5, 9, 17, 9)
		path(14, 6, 17, 9, 14, 12)
	case "firearm":
		path(1, 6, 5, 6, 5, 5, 13, 5, 13, 7, 17, 7, 17, 9, 11, 9, 11, 11, 8, 11, 7, 16, 4, 16, 5, 10, 1, 12, 1, 6)
		a(8, 5, 8, 3)
		a(10, 9, 10, 14)
	case "head":
		path(2, 11, 3, 5, 6, 2, 12, 2, 15, 5, 16, 11, 2, 11)
		path(4, 11, 5, 16, 13, 16, 14, 11)
		a(8, 3, 8, 10)
	case "legs":
		path(4, 2, 14, 2, 15, 17, 10, 17, 9, 8, 8, 17, 3, 17, 4, 2)
		a(4, 5, 14, 5)
	case "feet":
		path(5, 2, 12, 2, 12, 11, 16, 13, 16, 17, 3, 17, 3, 12, 5, 10, 5, 2)
		a(4, 14, 15, 14)
	case "armor", "body":
		path(3, 1, 6, 2, 7, 5, 11, 5, 12, 2, 15, 1, 18, 6, 15, 8, 15, 17, 3, 17, 3, 8, 0, 6, 3, 1)
		a(3, 12, 15, 12)
	case "shield":
		path(9, 1, 16, 4, 15, 11, 12, 15, 9, 18, 6, 15, 3, 11, 2, 4, 9, 1)
		path(9, 4, 13, 6, 12, 11, 9, 14, 6, 11, 5, 6, 9, 4)
	case "backpack":
		box(3, 5, 12, 12)
		path(6, 5, 6, 2, 12, 2, 12, 5)
		box(5, 11, 8, 4)
		a(1, 8, 1, 16)
		a(17, 8, 17, 16)
	case "save":
		path(2, 1, 14, 1, 17, 4, 17, 17, 2, 17, 2, 1)
		box(5, 1, 8, 6)
		box(5, 11, 9, 6)
	case "rotate-camera":
		for i := 0; i < 12; i++ {
			start := -math.Pi/2 + float64(i)*math.Pi/8
			end := start + math.Pi/8
			a(9+7*math.Cos(start), 9+7*math.Sin(start), 9+7*math.Cos(end), 9+7*math.Sin(end))
		}
		path(2, 13, 2, 8, 7, 8)
	case "movement":
		path(2, 14, 7, 9, 11, 9, 16, 4)
		path(10, 4, 16, 4, 16, 10)
		circle(3, 14, 2)
	case "passive":
		path(9, 1, 16, 4, 15, 11, 9, 17, 3, 11, 2, 4, 9, 1)
		path(5, 9, 8, 12, 13, 6)
	case "node-combat":
		path(2, 2, 15, 15, 17, 13, 4, 1)
		path(16, 2, 3, 15, 1, 13, 14, 1)
		a(1, 10, 8, 17)
		a(10, 17, 17, 10)
	case "node-shop":
		path(2, 8, 3, 3, 15, 3, 16, 8, 2, 8, 2, 16, 16, 16, 16, 8)
		box(7, 11, 4, 5)
	case "node-boss":
		path(2, 6, 5, 9, 9, 2, 13, 9, 16, 6, 15, 16, 3, 16, 2, 6)
		a(6, 12, 7, 12)
		a(11, 12, 12, 12)
	case "node-passage":
		p := point(9, 9)
		vector.DrawFilledCircle(dst, float32(p.X), float32(p.Y), float32(2.6*s), uiInk, true)
	case "node-unknown":
		path(5, 5, 7, 2, 11, 2, 14, 5, 14, 7, 9, 11)
		p := point(9, 15)
		vector.DrawFilledCircle(dst, float32(p.X), float32(p.Y), float32(1*s), uiInk, true)
	case "node-start":
		path(1, 9, 9, 2, 17, 9, 15, 9, 15, 17, 3, 17, 3, 9)
		box(7, 11, 4, 6)
	case "node-event":
		circle(9, 9, 7)
		path(6, 6, 8, 4, 11, 5, 12, 7, 9, 10, 9, 11)
		a(9, 14, 9, 15)
	case "view", "map":
		path(1, 4, 6, 2, 12, 4, 17, 2, 17, 14, 12, 16, 6, 14, 1, 16, 1, 4)
		a(6, 2, 6, 14)
		a(12, 4, 12, 16)
	case "door":
		path(3, 17, 3, 1, 15, 1, 15, 17)
		a(11, 9, 13, 9)
	case "solo", "person":
		circle(9, 4, 3)
		path(3, 17, 3, 13, 5, 10, 13, 10, 15, 13, 15, 17, 3, 17)
	case "host", "join":
		circle(3, 4, 2)
		circle(15, 4, 2)
		circle(9, 15, 2)
		path(3, 6, 3, 9, 15, 9, 15, 6)
		a(9, 9, 9, 13)
		if id == "host" {
			a(9, 1, 9, 7)
			a(6, 4, 12, 4)
		} else {
			path(6, 3, 9, 6, 12, 3)
		}
	case "warehouse":
		path(1, 5, 9, 1, 17, 5, 17, 15, 9, 18, 1, 15, 1, 5, 9, 9, 17, 5)
		a(9, 9, 9, 18)
		path(5, 3, 13, 7, 13, 10)
	case "key":
		box(1, 3, 16, 12)
		a(4, 7, 6, 7)
		a(9, 7, 11, 7)
		a(4, 11, 14, 11)
	case "scroll-left":
		path(12, 3, 6, 9, 12, 15)
	case "scroll-right":
		path(6, 3, 12, 9, 6, 15)
	case "scroll-up":
		path(3, 11, 9, 5, 15, 11)
	case "scroll-down":
		path(3, 6, 9, 12, 15, 6)
	case "room-settings":
		a(2, 4, 16, 4)
		a(2, 9, 16, 9)
		a(2, 14, 16, 14)
		a(6, 2, 6, 6)
		a(12, 7, 12, 11)
		a(7, 12, 7, 16)
	case "empty":
		p := point(9, 9)
		vector.DrawFilledCircle(dst, float32(p.X), float32(p.Y), float32(1*s), muted, true)
	default:
		circle(9, 9, 5)
		for i := 0; i < 8; i++ {
			angle := float64(i) * math.Pi / 4
			a(9+6*math.Cos(angle), 9+6*math.Sin(angle), 9+8.5*math.Cos(angle), 9+8.5*math.Sin(angle))
		}
	}
}
