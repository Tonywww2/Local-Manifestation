package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/pkg/model"
	"image/color"
	"math"
)

// A small cached texture keeps the vector stroke's shader work off the frame hot path.
// Pixel size, rather than world position, determines when the marker is rebuilt.
func (g *Game) drawSelfMarker(dst *ebiten.Image, position model.Vec, bodyRadius float64) {
	radius := math.Ceil(bodyRadius + 3)
	if g.selfMarker == nil || g.selfMarkerRadius != radius {
		if g.selfMarker != nil {
			g.selfMarker.Dispose()
		}
		size := int(radius+2) * 2
		g.selfMarker = ebiten.NewImage(size, size)
		g.selfMarkerRadius = radius
		vector.StrokeCircle(g.selfMarker, float32(size)/2, float32(size)/2, float32(radius), 2, color.RGBA{R: 235, G: 240, B: 249, A: 255}, true)
	}
	size := float64(g.selfMarker.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(position.X-size/2, position.Y-size/2)
	dst.DrawImage(g.selfMarker, op)
}
