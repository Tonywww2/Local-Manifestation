package client

import (
	"fmt"
	"maps"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
)

type windowSizes map[string]ui.Size

type windowResizeDrag struct {
	ID     string
	Start  ui.Point
	Bounds ui.Rect
}

func validateWindowSizes(sizes windowSizes) error {
	if len(sizes) > 128 {
		return fmt.Errorf("too many window sizes")
	}
	for id, size := range sizes {
		if id == "" || len(id) > 128 || math.IsNaN(size.W) || math.IsNaN(size.H) || math.IsInf(size.W, 0) || math.IsInf(size.H, 0) || size.W < 1 || size.H < 1 || size.W > 16384 || size.H > 16384 {
			return fmt.Errorf("invalid window size for %s", id)
		}
	}
	return nil
}

func defaultMapWindowSize(bounds ui.Size) ui.Size {
	return ui.Size{W: max(420, bounds.W*.86), H: max(300, bounds.H*.86)}
}

// Resize keeps the visible top-left corner fixed. Screen changes clamp only
// the displayed rectangle, never the saved preferred size.
func resizeMapWindow(d windowResizeDrag, cursor ui.Point, bounds ui.Size) ui.Rect {
	r := d.Bounds
	r.X = max(8, min(r.X, bounds.W-16))
	r.Y = max(8, min(r.Y, bounds.H-16))
	width, height := max(1, bounds.W-r.X-8), max(1, bounds.H-r.Y-8)
	r.W = max(min(420, width), min(width, d.Bounds.W+cursor.X-d.Start.X))
	r.H = max(min(300, height), min(height, d.Bounds.H+cursor.Y-d.Start.Y))
	return r
}

func (g *Game) addMapResizeControl(f *uiFrame, r ui.Rect) {
	addControl(f, "map", "map:resize", "resize", "", "window-resize", "map", g.tr("ui.map.resize"), ui.Rect{X: r.X + r.W - 24, Y: r.Y + r.H - 24, W: 22, H: 22})
}

func (g *Game) beginWindowResize(id string) {
	if id != "map" {
		return
	}
	for _, p := range g.u.frame.Panels {
		if p.ID == id {
			g.u.resize = &windowResizeDrag{ID: id, Start: g.u.cursor, Bounds: p.Bounds}
			g.u.drag = nil
			g.cameraControls.MapDrag = nil
			return
		}
	}
}

func (g *Game) updateWindowResize(cursor ui.Point) {
	d := *g.u.resize
	r := resizeMapWindow(d, cursor, g.uiBounds())
	if g.u.windowSizes == nil {
		g.u.windowSizes = windowSizes{}
	}
	g.u.windowSizes[d.ID] = ui.Size{W: r.W, H: r.H}
	g.u.layout[d.ID] = ui.Placement{Anchor: "top_left", X: r.X, Y: r.Y}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.u.resize = nil
		if err := g.saveSettings(); err != nil {
			g.notify("notice.settings_save", "detail", err.Error())
		}
		if draft := g.u.settingsDraft; draft != nil {
			draft.WindowSizes = maps.Clone(g.u.windowSizes)
			if draft.Layout == nil {
				draft.Layout = ui.Layout{}
			}
			draft.Layout[d.ID] = g.u.layout[d.ID]
		}
		return
	}
}
