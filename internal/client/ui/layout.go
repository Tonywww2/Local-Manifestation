// Package ui is the display-independent geometry and interaction core.
package ui

import (
	"fmt"
	"math"
)

type Point struct{ X, Y float64 }
type Size struct{ W, H float64 }
type Rect struct{ X, Y, W, H float64 }

func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.Y >= r.Y && p.X < r.X+r.W && p.Y < r.Y+r.H
}
func (r Rect) Inset(n float64) Rect { return Rect{r.X + n, r.Y + n, max(0, r.W-2*n), max(0, r.H-2*n)} }
func (r Rect) Header() Rect         { return Rect{r.X, r.Y, r.W, 30} }

type Placement struct {
	Anchor string  `json:"anchor"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}
type Layout map[string]Placement

func Clone(layout Layout) Layout {
	out := Layout{}
	for id, p := range layout {
		out[id] = p
	}
	return out
}
func Validate(layout Layout) error {
	if len(layout) > 256 {
		return fmt.Errorf("too many layout entries")
	}
	for id, p := range layout {
		if id == "" || len(id) > 128 || !finite(p.X) || !finite(p.Y) || math.Abs(p.X) > 10000 || math.Abs(p.Y) > 10000 {
			return fmt.Errorf("invalid layout entry %q", id)
		}
		switch p.Anchor {
		case "top_left", "top_right", "top_center", "bottom_left", "bottom_right", "bottom_center", "center":
		default:
			return fmt.Errorf("invalid anchor for %s", id)
		}
	}
	return nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func origin(anchor string, size, bounds Size) Point {
	x, y := 0.0, 0.0
	switch anchor {
	case "top_right", "bottom_right":
		x = bounds.W - size.W
	case "center", "top_center", "bottom_center":
		x = (bounds.W - size.W) / 2
	}
	switch anchor {
	case "bottom_left", "bottom_right", "bottom_center":
		y = bounds.H - size.H
	case "center":
		y = (bounds.H - size.H) / 2
	}
	return Point{x, y}
}

// Resolve clamps only the visible rectangle; it never rewrites the preference.
func Resolve(p Placement, size, bounds Size) Rect {
	size.W = min(size.W, max(1, bounds.W-16))
	size.H = min(size.H, max(1, bounds.H-16))
	o := origin(p.Anchor, size, bounds)
	return Rect{max(8, min(bounds.W-size.W-8, o.X+p.X)), max(8, min(bounds.H-size.H-8, o.Y+p.Y)), size.W, size.H}
}
func At(layout Layout, id string, fallback Placement, size, bounds Size) Rect {
	if p, ok := layout[id]; ok {
		fallback = p
	}
	return Resolve(fallback, size, bounds)
}
func Near(target Rect, size, bounds Size) Rect {
	x, y := target.X, target.Y-size.H-8
	if y < 8 {
		y = target.Y + target.H + 8
	}
	return Resolve(Placement{"top_left", x, y}, size, bounds)
}

// TooltipAt follows the pointer and flips at screen edges. It is display-only;
// callers never add it to hit testing, dragging or saved layout preferences.
func TooltipAt(cursor Point, size, bounds Size) Rect {
	x, y := cursor.X+16, cursor.Y+16
	if x+size.W > bounds.W-8 {
		x = cursor.X - size.W - 16
	}
	if y+size.H > bounds.H-8 {
		y = cursor.Y - size.H - 16
	}
	return Resolve(Placement{"top_left", x, y}, size, bounds)
}

type Panel struct {
	ID       string
	Bounds   Rect
	Floating bool
}

func HitPanel(panels []Panel, p Point) (Panel, bool) {
	for i := len(panels) - 1; i >= 0; i-- {
		if panels[i].Bounds.Contains(p) {
			return panels[i], true
		}
	}
	return Panel{}, false
}

type Drag struct {
	ID     string
	Start  Point
	Bounds Rect
	Anchor string
}

// DragTo returns a detached layout; one caller owns and commits the draft.
func DragTo(layout Layout, d Drag, cursor Point, bounds Size) Layout {
	r := d.Bounds
	r.X += cursor.X - d.Start.X
	r.Y += cursor.Y - d.Start.Y
	r = Resolve(Placement{"top_left", r.X, r.Y}, Size{r.W, r.H}, bounds)
	o := origin(d.Anchor, Size{r.W, r.H}, bounds)
	out := Clone(layout)
	out[d.ID] = Placement{d.Anchor, r.X - o.X, r.Y - o.Y}
	return out
}
