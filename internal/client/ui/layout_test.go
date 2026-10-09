package ui

import (
	"math"
	"reflect"
	"testing"
)

func TestDragResizeRestoresAnchorAndDoesNotMutate(t *testing.T) {
	saved := Layout{"bag": {"bottom_right", -20, -20}}
	before := Clone(saved)
	size := Size{300, 200}
	bounds := Size{1280, 800}
	r := At(saved, "bag", Placement{}, size, bounds)
	next := DragTo(saved, Drag{"bag", Point{r.X, r.Y}, r, "bottom_right"}, Point{r.X - 100, r.Y - 50}, bounds)
	if !reflect.DeepEqual(saved, before) {
		t.Fatal("draft mutated saved layout")
	}
	if next["bag"].X != -120 || next["bag"].Y != -70 {
		t.Fatal(next)
	}
	small := At(next, "bag", Placement{}, size, Size{320, 240})
	if small.X < 8 || small.Y < 8 {
		t.Fatal(small)
	}
	large := At(next, "bag", Placement{}, size, bounds)
	if large.X != r.X-100 || large.Y != r.Y-50 {
		t.Fatal("temporary clamp destroyed preference", large)
	}
}
func TestTopmostWindowConsumesAndTooltipFits(t *testing.T) {
	panels := []Panel{{"world", Rect{0, 0, 1280, 800}, false}, {"bag", Rect{100, 100, 300, 200}, true}, {"settings", Rect{200, 110, 200, 180}, true}}
	p, ok := HitPanel(panels, Point{230, 130})
	if !ok || p.ID != "settings" {
		t.Fatal(p)
	}
	r := TooltipAt(Point{1250, 5}, Size{400, 300}, Size{1280, 800})
	if r.X+r.W > 1272 || r.Y < 8 || r.Y+r.H > 792 {
		t.Fatal(r)
	}
}
func TestRejectInvalidLayout(t *testing.T) {
	for _, p := range []Placement{{"top_left", math.Inf(1), 0}, {"top_left", 0, math.NaN()}, {"unknown", 0, 0}} {
		if Validate(Layout{"weapon": p}) == nil {
			t.Fatal(p)
		}
	}
}
