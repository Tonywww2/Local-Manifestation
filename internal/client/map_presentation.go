package client

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/model"
)

// Presentation depends only on disclosed node information; it never changes
// reachability, completion or the authoritative graph.
func mapNodePresentation(n model.Node, known bool) (color.RGBA, string) {
	col, icon := mapNodeStyle(n)
	if !known {
		return col, "node-unknown"
	}
	if n.Completed {
		fade := func(value uint8, target int) uint8 { return uint8((int(value)*2 + target) / 3) }
		col = color.RGBA{R: fade(col.R, 156), G: fade(col.G, 161), B: fade(col.B, 166), A: 255}
	}
	return col, icon
}

func (g *Game) mapCompletionMark(n model.Node) bool {
	if !n.Completed || g.catalog.NodeFacility(n.Definition) != "" {
		return false
	}
	switch n.Kind {
	case "passage", "shop", "start", "revival", "shrine":
		return false
	default:
		return true
	}
}

// A connection occupies the entire facing side of each hexagon. Unconnected
// pairs have no polygon at all. Rotation uses the same geometry as hit testing.
func mapConnectionQuad(from, to model.Vec, radius, angle float64) [4]model.Vec {
	delta := to.Sub(from)
	side := math.Round((math.Atan2(delta.Y, delta.X) - angle - math.Pi/6) / (math.Pi / 3))
	vertex := func(center model.Vec, side float64) model.Vec {
		a := angle + side*math.Pi/3
		return center.Add(model.Vec{X: math.Cos(a) * radius, Y: math.Sin(a) * radius})
	}
	return [4]model.Vec{vertex(from, side), vertex(from, side+1), vertex(to, side+3), vertex(to, side+4)}
}

func drawMapConnection(dst *ebiten.Image, from, to model.Vec, radius, angle float64, col color.RGBA) {
	quad := mapConnectionQuad(from, to, radius, angle)
	var vertices [4]ebiten.Vertex
	for i, p := range quad {
		vertices[i] = ebiten.Vertex{DstX: float32(p.X), DstY: float32(p.Y), SrcX: 1, SrcY: 1,
			ColorR: float32(col.R) / 255, ColorG: float32(col.G) / 255, ColorB: float32(col.B) / 255, ColorA: 1}
	}
	dst.DrawTriangles(vertices[:], []uint16{0, 1, 2, 0, 2, 3}, whiteImage, nil)
}
