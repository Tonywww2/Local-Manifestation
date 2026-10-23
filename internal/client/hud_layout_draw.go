package client

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/model"
)

func (g *Game) drawHUDEditorGrid(dst *ebiten.Image) {
	rect(dst, 0, 0, float64(g.width), float64(g.height), color.RGBA{12, 20, 24, 150})
	grid := color.NRGBA{110, 143, 150, 28}
	for x := 0; x < g.width; x += 32 {
		rect(dst, float64(x), 0, 1, float64(g.height), grid)
	}
	for y := 0; y < g.height; y += 32 {
		rect(dst, 0, float64(y), float64(g.width), 1, grid)
	}
	axis := color.NRGBA{133, 190, 192, 100}
	rect(dst, float64(g.width)/2, 0, 1, float64(g.height), axis)
	rect(dst, 0, float64(g.height)/2, float64(g.width), 1, axis)
}

func (g *Game) drawHUDEditorHandles(dst *ebiten.Image) {
	draw := func(p ui.Panel, selected bool) {
		r := p.Bounds
		col, width := color.RGBA{120, 173, 183, 200}, float32(1)
		if selected {
			col, width = hudGold, 2
		}
		vector.StrokeRect(dst, float32(r.X), float32(r.Y), float32(r.W), float32(r.H), width, col, false)
		handle := hudHandle(p)
		rect(dst, handle.X, handle.Y, handle.W, handle.H, color.RGBA{22, 38, 43, 242})
		g.drawColoredText(dst, g.tr(combatHUDKey(p.ID)), handle.X+3, handle.Y-1, 11, col)
		if selected {
			for _, point := range []ui.Point{{X: r.X, Y: r.Y}, {X: r.X + r.W, Y: r.Y}, {X: r.X, Y: r.Y + r.H}, {X: r.X + r.W, Y: r.Y + r.H}} {
				rect(dst, point.X-2, point.Y-2, 4, 4, col)
			}
		}
	}
	for _, p := range g.u.frame.Panels {
		if combatHUDKey(p.ID) != "" && p.ID != g.u.hudEditor.Selected {
			draw(p, false)
		}
	}
	if p, ok := hudPanel(g.u.frame, g.u.hudEditor.Selected); ok {
		draw(p, true)
	}
}

func (g *Game) drawHUDPreviewMap(dst *ebiten.Image, bounds ui.Rect) {
	dst = clippedUI(dst, bounds)
	nodes := []model.Node{{ID: "a", Kind: "start", Q: 0, R: 0}, {ID: "b", Kind: "passage", Q: 1, R: 0}, {ID: "c", Kind: "combat", Q: 2, R: 0}, {ID: "d", Kind: "shop", Q: 1, R: -1}, {ID: "e", Kind: "boss", Q: 2, R: -1}}
	positions, radius := mapPositionsWithTerrain(nodes, nil, bounds, g.cameraAngle(), 1)
	for _, edge := range [][2]string{{"a", "b"}, {"a", "d"}, {"b", "c"}, {"b", "d"}, {"c", "e"}, {"d", "e"}} {
		drawMapConnection(dst, positions[edge[0]], positions[edge[1]], radius, g.cameraAngle(), color.RGBA{58, 66, 75, 255})
	}
	for _, n := range nodes {
		v := positions[n.ID]
		col, icon := mapNodePresentation(n, true)
		hexagon(dst, v, radius, col, g.cameraAngle())
		size := radius * 1.15
		g.drawIconSized(dst, icon, v.X-size/2, v.Y-size/2, size)
	}
}
