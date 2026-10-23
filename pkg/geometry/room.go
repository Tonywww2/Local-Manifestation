package geometry

import (
	"fmt"
	"hsmultiplayer/pkg/model"
)

// NewRoomArena compiles a read-only union once. Width and height describe each
// base hexagon, rather than stretching a bounding hexagon around the layout.
func NewRoomArena(width, height float64, cells []model.HexCell) (Arena, error) {
	if width <= 0 || height <= 0 || !(model.Vec{X: width, Y: height}).Finite() {
		return Arena{}, fmt.Errorf("invalid room envelope")
	}
	a := NewArena(width, height)
	if len(cells) == 0 {
		return a, nil
	}
	origin := false
	for _, cell := range cells {
		origin = origin || cell == (model.HexCell{})
	}
	if !origin {
		return Arena{}, fmt.Errorf("room layout needs origin hex (0, 0)")
	}
	f, err := NewFootprint(a.Center, a.Radius, cells)
	if err != nil {
		return Arena{}, err
	}
	a.shape = &f
	return a, nil
}

func (a Arena) Cells() []Arena {
	if a.shape != nil {
		return a.shape.Cells()
	}
	return []Arena{a}
}

func (a Arena) Boundary() []Segment {
	if a.shape != nil {
		return a.shape.Boundary()
	}
	v := a.Vertices()
	result := make([]Segment, 6)
	for i := range v {
		result[i] = Segment{v[i], v[(i+1)%6]}
	}
	return result
}

// ClearStep checks the swept disk, including concave exterior corners. Checking
// only the endpoint would let a dodge or a smoothed path cut across the void.
func (a Arena) ClearStep(from, to model.Vec, radius float64) bool {
	if a.shape != nil {
		return a.shape.ClearStep(from, to, radius)
	}
	return a.ContainsDisk(from, radius) && a.ContainsDisk(to, radius)
}

func (f Footprint) ClearStep(from, to model.Vec, radius float64) bool {
	if !f.ContainsDisk(from, radius) || !f.ContainsDisk(to, radius) {
		return false
	}
	path := Segment{from, to}
	if radius == 0 {
		clipped, ok := f.ClipSegment(path)
		if !ok || clipped.A.Sub(from).Length() > 1e-7 || clipped.B.Sub(to).Length() > 1e-7 {
			return false
		}
	}
	// A positive-radius disk cannot cross an exterior edge. Its clearance
	// check also rejects exterior shortcuts without sorting intersection spans.
	for _, edge := range f.boundary {
		if segmentsDistance(path, edge)+1e-8 < radius {
			return false
		}
	}
	return true
}

func segmentsDistance(a, b Segment) float64 {
	// When segments cross, the distances between endpoints and edges need not
	// be zero, so detect the intersection before measuring their endpoints.
	cross := func(u, v model.Vec) float64 { return u.X*v.Y - u.Y*v.X }
	u, v, offset := a.B.Sub(a.A), b.B.Sub(b.A), b.A.Sub(a.A)
	den := cross(u, v)
	if den != 0 {
		t, s := cross(offset, v)/den, cross(offset, u)/den
		if t >= 0 && t <= 1 && s >= 0 && s <= 1 {
			return 0
		}
	}
	return min(nearestOnSegment(a.A, b).Sub(a.A).Length(), nearestOnSegment(a.B, b).Sub(a.B).Length(),
		nearestOnSegment(b.A, a).Sub(b.A).Length(), nearestOnSegment(b.B, a).Sub(b.B).Length())
}
