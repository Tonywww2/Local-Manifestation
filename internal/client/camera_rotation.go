package client

import (
	"math"
	"time"

	"hsmultiplayer/internal/client/ui"
)

const cameraDoubleClickWindow = 300 * time.Millisecond
const cameraSnapDuration = 160 * time.Millisecond
const cameraClickSlop = 6.0

type cameraRotationSnap struct {
	From, Target float64
	Started      time.Time
}

type cameraRotationClick struct {
	At       time.Time
	Position ui.Point
}

type cameraRotationKeyTap struct {
	At   time.Time
	Code string
}

// Angles are offsets from the default view. Keep the target unwrapped so an
// interpolation across 0/360 degrees follows the requested direction.
func snapCameraRotation(angle, direction float64) float64 {
	const step = math.Pi / 3
	turns := angle / step
	nearest := math.Round(turns)
	if math.Abs(turns-nearest) < 1e-9 {
		return nearest * step
	}
	switch {
	case direction > 0:
		return math.Ceil(turns) * step
	case direction < 0:
		return math.Floor(turns) * step
	default:
		return nearest * step
	}
}

func sampleCameraSnap(s cameraRotationSnap, now time.Time) (angle float64, done bool) {
	t := max(0, min(1, float64(now.Sub(s.Started))/float64(cameraSnapDuration)))
	ease := 1 - (1-t)*(1-t)*(1-t)
	return s.From + (s.Target-s.From)*ease, t >= 1
}

func cameraDoubleClick(first cameraRotationClick, position ui.Point, now time.Time) bool {
	elapsed := now.Sub(first.At)
	return !first.At.IsZero() && elapsed >= 0 && elapsed <= cameraDoubleClickWindow && math.Hypot(position.X-first.Position.X, position.Y-first.Position.Y) <= cameraClickSlop
}

func (c *cameraControls) startRotationSnap(direction float64, key string, now time.Time) {
	c.Snap = &cameraRotationSnap{From: c.Rotation, Target: snapCameraRotation(c.Rotation, direction), Started: now}
	c.SnapKey = key
	c.Drag = nil
	c.ButtonClick, c.KeyTap = cameraRotationClick{}, cameraRotationKeyTap{}
	if direction != 0 {
		c.LastDirection = direction
	}
}

func (c *cameraControls) resetRotationInput() {
	c.Drag, c.Snap = nil, nil
	c.SnapKey = ""
	c.ButtonClick, c.KeyTap = cameraRotationClick{}, cameraRotationKeyTap{}
}

func (c *cameraControls) pressRotationButton(position ui.Point, now time.Time) {
	if cameraDoubleClick(c.ButtonClick, position, now) {
		c.startRotationSnap(c.LastDirection, "", now)
		return
	}
	c.Snap, c.SnapKey = nil, ""
	c.ButtonClick, c.KeyTap = cameraRotationClick{}, cameraRotationKeyTap{}
	c.Drag = &cameraRotationDrag{Start: position, LastX: position.X, Rotation: c.Rotation, Started: now}
}

func (c *cameraControls) releaseRotationButton(now time.Time) {
	d := c.Drag
	if d != nil && !d.Moved && now.Sub(d.Started) <= cameraDoubleClickWindow {
		c.ButtonClick = cameraRotationClick{At: d.Started, Position: d.Start}
	}
	c.Drag = nil
}

func (c *cameraControls) dragRotationButton(position ui.Point) {
	d := c.Drag
	d.Moved = d.Moved || math.Hypot(position.X-d.Start.X, position.Y-d.Start.Y) > cameraClickSlop
	if d.Moved {
		if delta := position.X - d.LastX; delta != 0 {
			c.LastDirection = math.Copysign(1, delta)
		}
		c.Rotation = math.Remainder(d.Rotation+(position.X-d.Start.X)*.01, 2*math.Pi)
	}
	d.LastX = position.X
}
