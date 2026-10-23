package client

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/model"
)

type projectileArt struct{ disk, ring, sword *ebiten.Image }

// Small immutable masks replace per-frame antialiased vector paths. Hundreds
// of overlapping bullets now submit simple atlas quads instead of a combined
// path arrangement whose tessellation grows with every intersecting outline.
func projectileMask(ring bool, width float64) *ebiten.Image {
	const size, radius = 32, 12.0
	pixels := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)+.5-size/2, float64(y)+.5-size/2)
			coverage := max(0, min(1, radius+.5-d))
			if ring {
				coverage = max(0, min(1, width/2+.5-math.Abs(d-radius)))
			}
			a := uint8(math.Round(coverage * 255))
			pixels.SetRGBA(x, y, color.RGBA{a, a, a, a})
		}
	}
	return ebiten.NewImageFromImage(pixels)
}

func (g *Game) prepareProjectileArt() {
	if g.projectileArt.disk != nil {
		return
	}
	g.projectileArt = projectileArt{disk: projectileMask(false, 0), ring: projectileMask(true, 3), sword: projectileMask(true, 1.1)}
}

func projectileSprite(dst, texture *ebiten.Image, center model.Vec, radius float64, ink color.Color) {
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-16, -16)
	op.GeoM.Scale(radius/12, radius/12)
	op.GeoM.Translate(center.X, center.Y)
	op.ColorScale.ScaleWithColor(ink)
	dst.DrawImage(texture, op)
}

func (g *Game) releaseProjectileArt() {
	for _, image := range []*ebiten.Image{g.projectileArt.disk, g.projectileArt.ring, g.projectileArt.sword} {
		if image != nil {
			image.Deallocate()
		}
	}
	g.projectileArt = projectileArt{}
}
