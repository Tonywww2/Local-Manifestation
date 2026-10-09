package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/pkg/model"
	"image/color"
	"math"
)

// Readable placeholder silhouettes retain the contemporary visual direction.
// They are code-native so texture packs can replace each enemy:<id> asset.
func (g *Game) drawEnemy(dst *ebiten.Image, c Camera, e model.Enemy) {
	appearance := g.catalog.EnemyAppearance(e.Definition)
	p := c.Project(e.Position)
	r := max(4, e.Radius*c.Scale)
	ink := color.RGBA{95, 70, 77, 255}
	edge := color.RGBA{201, 144, 125, 255}
	if e.Phase == "windup" {
		length := 220.0
		if e.AttackKind == "rush" {
			length = 420
		}
		end := c.Project(e.Position.Add(e.AttackDirection.Scale(length)))
		line(dst, p, end, color.RGBA{230, 174, 94, 150}, 3)
		vector.StrokeCircle(dst, float32(p.X), float32(p.Y), float32(r+5), 2, color.RGBA{235, 181, 99, 255}, true)
	}
	if g.drawAsset(dst, "enemy:"+e.Definition, p, r*2, r*2) {
		return
	}
	vector.FillCircle(dst, float32(p.X), float32(p.Y+r*.45), float32(r), color.RGBA{24, 32, 37, 55}, true)
	switch appearance {
	case "load", "encased", "carrier":
		rect(dst, p.X-r, p.Y-r, r*2, r*1.7, ink)
		line(dst, model.Vec{X: p.X - r, Y: p.Y - r*.35}, model.Vec{X: p.X + r, Y: p.Y - r*.35}, edge, 2)
		line(dst, model.Vec{X: p.X, Y: p.Y - r}, model.Vec{X: p.X, Y: p.Y + r*.7}, edge, 2)
		if appearance == "carrier" {
			rect(dst, p.X-r*.8, p.Y-r*1.4, r*1.6, r*.4, edge)
		}
	case "lens", "array":
		vector.FillCircle(dst, float32(p.X), float32(p.Y), float32(r), ink, true)
		vector.StrokeCircle(dst, float32(p.X), float32(p.Y), float32(r*.65), 2, color.RGBA{118, 184, 194, 255}, true)
		vector.FillCircle(dst, float32(p.X), float32(p.Y), float32(r*.25), color.RGBA{197, 223, 216, 255}, true)
		if appearance == "array" {
			for i := 0; i < 3; i++ {
				angle := float64(i) * math.Pi * 2 / 3
				vector.FillCircle(dst, float32(p.X+math.Cos(angle)*r), float32(p.Y+math.Sin(angle)*r), float32(r*.3), edge, true)
			}
		}
	case "bag":
		polygon(dst, []model.Vec{{X: p.X - r, Y: p.Y + r*.7}, {X: p.X - r*.7, Y: p.Y - r*.65}, {X: p.X + r*.5, Y: p.Y - r}, {X: p.X + r, Y: p.Y + r*.6}}, ink)
		line(dst, model.Vec{X: p.X - r*.6, Y: p.Y - r*.4}, model.Vec{X: p.X + r*.5, Y: p.Y - r*.4}, edge, 2)
	case "leaker":
		rect(dst, p.X-r*.7, p.Y-r, r*1.4, r*1.8, ink)
		rect(dst, p.X-r*.45, p.Y-r*.6, r*.9, r, color.RGBA{137, 165, 95, 255})
		line(dst, p, model.Vec{X: p.X + r*1.2, Y: p.Y + r*.7}, edge, 3)
	case "spitter":
		polygon(dst, []model.Vec{{X: p.X - r, Y: p.Y + r*.8}, {X: p.X, Y: p.Y - r}, {X: p.X + r, Y: p.Y + r*.8}}, ink)
		rect(dst, p.X-r*.45, p.Y-r*.05, r*.9, r*.3, edge)
	default:
		vector.FillCircle(dst, float32(p.X), float32(p.Y-r*.5), float32(r*.55), ink, true)
		rect(dst, p.X-r*.5, p.Y-r*.1, r, r*.95, ink)
		line(dst, model.Vec{X: p.X - r*.3, Y: p.Y + r*.5}, model.Vec{X: p.X - r*1.3, Y: p.Y + r}, edge, 3)
	}
}
