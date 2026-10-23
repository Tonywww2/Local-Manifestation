package client

import (
	"fmt"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/model"
)

type cameraControls struct {
	Rotation, Zoom, MapZoom, MiniZoom float64
	MapOffset, MiniOffset             model.Vec
	LastSample                        time.Time
	Drag                              *cameraRotationDrag
	MapDrag                           *mapPanDrag
	LastDirection                     float64
	ButtonClick                       cameraRotationClick
	KeyTap                            cameraRotationKeyTap
	Snap                              *cameraRotationSnap
	SnapKey                           string
}

type mapPanDrag struct {
	Start  ui.Point
	Offset model.Vec
	Moved  bool
}

func panMap(d mapPanDrag, cursor ui.Point) (model.Vec, bool) {
	delta := model.Vec{X: cursor.X - d.Start.X, Y: cursor.Y - d.Start.Y}
	moved := d.Moved || delta.Length() >= 6
	if !moved {
		return d.Offset, false
	}
	return d.Offset.Add(delta), true
}

type cameraRotationDrag struct {
	Start           ui.Point
	LastX, Rotation float64
	Started         time.Time
	Moved           bool
}

func mapContentBounds(r ui.Rect) ui.Rect {
	return ui.Rect{X: r.X + 8, Y: r.Y + 34, W: r.W - 16, H: r.H - 42}
}

func minimapContentBounds(r ui.Rect) ui.Rect {
	return ui.Rect{X: r.X + 6, Y: r.Y + 6, W: r.W - 12, H: r.H - 40}
}

func clampZoom(zoom, low, high float64) float64 {
	if zoom <= 0 || math.IsNaN(zoom) || math.IsInf(zoom, 0) {
		zoom = 1
	}
	return max(low, min(high, zoom))
}

func scrollZoom(zoom, wheel, low, high float64) float64 {
	return clampZoom(clampZoom(zoom, low, high)*math.Exp(max(-20, min(20, wheel))*.12), low, high)
}

func (g *Game) cameraAngle() float64 { return defaultCameraAngle + g.cameraControls.Rotation }

// Adventure bounds come from the same cached evaluation that explains stats.
// Its inputs are authoritative snapshot values, not client preferences.
func (g *Game) cameraZoomLimits() (float64, float64) {
	v := g.u.statPreview.Values
	if g.self() == nil {
		v = g.front.Actions.cameraStats
	}
	if v.ZoomMin <= 0 || v.ZoomMax < v.ZoomMin {
		return model.DefaultZoomMin, model.DefaultZoomMax
	}
	return v.ZoomMin, v.ZoomMax
}

func (g *Game) cameraZoom(target string) float64 {
	low, high := g.cameraZoomLimits()
	zoom := g.cameraControls.Zoom
	if target == "map" {
		zoom = g.cameraControls.MapZoom
	} else if target == "minimap" {
		zoom = g.cameraControls.MiniZoom
	}
	return clampZoom(zoom, low, high)
}

func (g *Game) zoomCamera(target string, wheel float64) {
	low, high := g.cameraZoomLimits()
	zoom := &g.cameraControls.Zoom
	if target == "map" {
		zoom = &g.cameraControls.MapZoom
	} else if target == "minimap" {
		zoom = &g.cameraControls.MiniZoom
	}
	*zoom = scrollZoom(*zoom, wheel, low, high)
}

func (g *Game) zoomMapAt(target string, wheel float64, cursor ui.Point, bounds ui.Rect) {
	previous := g.cameraZoom(target)
	g.zoomCamera(target, wheel)
	ratio := g.cameraZoom(target) / previous
	offset := &g.cameraControls.MapOffset
	if target == "minimap" {
		offset = &g.cameraControls.MiniOffset
	}
	anchor := model.Vec{X: cursor.X - bounds.X - bounds.W/2, Y: cursor.Y - bounds.Y - bounds.H/2}
	*offset = anchor.Add(offset.Sub(anchor).Scale(ratio))
}

func (g *Game) mapLayout(bounds ui.Rect, target string) (map[string]model.Vec, float64) {
	positions, radius := mapPositionsWithTerrain(g.state.Nodes, g.state.Terrain, bounds, g.cameraAngle(), g.cameraZoom(target))
	offset := g.cameraControls.MapOffset
	if target == "minimap" {
		offset = g.cameraControls.MiniOffset
	}
	for id, position := range positions {
		positions[id] = position.Add(offset)
	}
	return positions, radius
}

func (g *Game) beginCameraRotation() {
	x, y := ebiten.CursorPosition()
	g.cameraControls.pressRotationButton(ui.Point{X: float64(x), Y: float64(y)}, time.Now())
}

// Input/time sampling stays in Update. Rendering only reads the resulting view.
func (g *Game) updateCameraControls(mouseCaptured, keysCaptured bool, now time.Time) bool {
	c := &g.cameraControls
	dt := 0.0
	if !c.LastSample.IsZero() {
		dt = max(0, min(.05, now.Sub(c.LastSample).Seconds()))
	}
	c.LastSample = now
	low, high := g.cameraZoomLimits()
	c.Zoom = clampZoom(c.Zoom, low, high)
	c.MapZoom = clampZoom(c.MapZoom, low, high)
	c.MiniZoom = clampZoom(c.MiniZoom, low, high)
	if c.SnapKey != "" && !bindingDown(c.SnapKey) {
		c.SnapKey = ""
	}
	if c.Drag != nil {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			c.dragRotationButton(ui.Point{X: float64(x), Y: float64(y)})
			return true
		}
		c.releaseRotationButton(now)
	}
	if g.u.editor == nil && !keysCaptured && (g.state != nil || g.front.Page == "lobby") {
		direction := 0.0
		code := ""
		if code := g.bound("view_left"); bindingAvailable(code, mouseCaptured) && bindingDown(code) {
			direction--
		}
		if code := g.bound("view_right"); bindingAvailable(code, mouseCaptured) && bindingDown(code) {
			direction++
		}
		if direction < 0 {
			code = g.bound("view_left")
		} else if direction > 0 {
			code = g.bound("view_right")
		}
		if code != "" && code != c.SnapKey {
			if bindingPressed(code) && c.KeyTap.Code == code && !c.KeyTap.At.IsZero() && now.Sub(c.KeyTap.At) <= cameraDoubleClickWindow {
				c.startRotationSnap(direction, code, now)
			} else {
				if bindingPressed(code) {
					c.KeyTap = cameraRotationKeyTap{At: now, Code: code}
				}
				c.Snap = nil
				c.SnapKey = ""
				c.ButtonClick = cameraRotationClick{}
				c.LastDirection = direction
				c.Rotation = math.Remainder(c.Rotation+direction*dt*math.Pi/2, 2*math.Pi)
			}
		}
	} else {
		c.KeyTap = cameraRotationKeyTap{}
	}
	if c.Snap != nil {
		angle, done := sampleCameraSnap(*c.Snap, now)
		c.Rotation = math.Remainder(angle, 2*math.Pi)
		if done {
			c.Snap = nil
		}
	}
	return false
}

func (g *Game) buildCameraControl(f *uiFrame, layout ui.Layout) {
	placement := ui.Placement{Anchor: "top_right", X: -152, Y: 180}
	r := ui.At(layout, "view", placement, ui.Size{W: 90, H: 38}, g.uiBounds())
	f.Panels = append(f.Panels, ui.Panel{ID: "view", Bounds: r})
	angle := math.Mod(g.cameraControls.Rotation*180/math.Pi+360, 360)
	low, high := g.cameraZoomLimits()
	left, right := g.bindingLabel(g.bound("view_left")), g.bindingLabel(g.bound("view_right"))
	f.Controls = append(f.Controls, uiControl{Owner: "view", ID: "view", Icon: "rotate-camera", Key: left + "/" + right, Action: "view", Bounds: r, Title: g.tr("ui.view"),
		Summary: []string{g.tr("ui.camera.rotate_help", "left", left, "right", right), g.tr("ui.camera.values", "angle", fmt.Sprintf("%.0f", angle), "zoom", fmt.Sprintf("%.2f", g.cameraZoom("world")))},
		Details: []string{g.tr("ui.camera.range", "min", fmt.Sprintf("%.2f", low), "max", fmt.Sprintf("%.2f", high))}})
}
