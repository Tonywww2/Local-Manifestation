package client

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

func (g *Game) drawTooltip(dst *ebiten.Image) {
	tip := g.u.frame.Tooltip
	if tip == nil {
		return
	}
	r := tip.Bounds
	g.drawBox(dst, r, color.RGBA{35, 44, 47, 250}, uiEdge)
	g.fitText(dst, tip.Title, r.X+16, r.Y+9, r.W-32, 16, false)
	if len(tip.Rows) == 0 {
		return
	}
	rect(dst, r.X+16, r.Y+36, r.W-32, 1, color.RGBA{79, 96, 98, 255})
	for i, row := range tip.RichRows {
		x, y := r.X+16, r.Y+43+float64(i)*24
		for _, span := range row {
			drawTextSpan(dst, span.Text, x, y, 14, semanticColor(span.Role, uiInk))
			x += measuredText(span.Text, 14)
		}
	}
}
