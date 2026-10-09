package client

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/internal/client/ui"
)

type boxStyle struct{ Fill, Edge color.RGBA }

func roundedPath(x, y, w, h, radius float32) vector.Path {
	var p vector.Path
	p.MoveTo(x+radius, y)
	p.LineTo(x+w-radius, y)
	p.QuadTo(x+w, y, x+w, y+radius)
	p.LineTo(x+w, y+h-radius)
	p.QuadTo(x+w, y+h, x+w-radius, y+h)
	p.LineTo(x+radius, y+h)
	p.QuadTo(x, y+h, x, y+h-radius)
	p.LineTo(x, y+radius)
	p.QuadTo(x, y, x+radius, y)
	p.Close()
	return p
}

func fillPath(dst *ebiten.Image, path vector.Path, col color.RGBA) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(col)
	vector.FillPath(dst, &path, nil, op)
}

// One tiny nine-slice per palette entry. Window dimensions and pointer movement
// never allocate a full-size panel texture or rebuild its rounded geometry.
func (g *Game) drawBox(dst *ebiten.Image, r ui.Rect, fill, edge color.RGBA) {
	if r.W < 12 || r.H < 12 {
		rect(dst, r.X, r.Y, r.W, r.H, fill)
		return
	}
	key := boxStyle{fill, edge}
	if g.resources.boxes == nil {
		g.resources.boxes = map[boxStyle]*ebiten.Image{}
	}
	img := g.resources.boxes[key]
	if img == nil {
		img = ebiten.NewImage(16, 16)
		path := roundedPath(.5, .5, 15, 15, 3.5)
		fillPath(img, path, fill)
		op := &vector.DrawPathOptions{AntiAlias: true}
		op.ColorScale.ScaleWithColor(edge)
		vector.StrokePath(img, &path, &vector.StrokeOptions{Width: 1}, op)
		g.resources.boxes[key] = img
	}
	x, y := [4]float64{r.X, r.X + 6, r.X + r.W - 6, r.X + r.W}, [4]float64{r.Y, r.Y + 6, r.Y + r.H - 6, r.Y + r.H}
	src := [4]float32{0, 6, 10, 16}
	var vertices [16]ebiten.Vertex
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			vertices[row*4+col] = ebiten.Vertex{DstX: float32(x[col]), DstY: float32(y[row]), SrcX: src[col], SrcY: src[row], ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}
		}
	}
	dst.DrawTriangles32(vertices[:], boxIndices[:], img, nil)
}

var boxIndices = [54]uint32{0, 1, 4, 1, 5, 4, 1, 2, 5, 2, 6, 5, 2, 3, 6, 3, 7, 6, 4, 5, 8, 5, 9, 8, 5, 6, 9, 6, 10, 9, 6, 7, 10, 7, 11, 10, 8, 9, 12, 9, 13, 12, 9, 10, 13, 10, 14, 13, 10, 11, 14, 11, 15, 14}
