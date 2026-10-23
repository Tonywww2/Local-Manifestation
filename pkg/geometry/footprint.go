package geometry

import (
	"fmt"
	"math"
	"slices"

	"hsmultiplayer/pkg/model"
)

type Cell = model.HexCell

// Footprint is an immutable union of adjacent hexagons. Only exterior edges
// collide; shared seams remain open even for a disk with nonzero radius.
type Footprint struct {
	cells    []Arena
	boundary []Segment
}

func NewFootprint(center model.Vec, radius float64, cells []Cell) (Footprint, error) {
	if !center.Finite() || radius <= 0 || math.IsInf(radius, 0) || math.IsNaN(radius) || len(cells) == 0 || len(cells) > 128 {
		return Footprint{}, fmt.Errorf("invalid hex footprint")
	}
	occupied := map[Cell]bool{}
	for _, cell := range cells {
		if occupied[cell] || cell.Q < -128 || cell.Q > 128 || cell.R < -128 || cell.R > 128 {
			return Footprint{}, fmt.Errorf("invalid footprint cell")
		}
		occupied[cell] = true
	}
	steps := [6]Cell{{1, 0}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}, {1, -1}}
	seen := map[Cell]bool{cells[0]: true}
	queue := []Cell{cells[0]}
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		for _, d := range steps {
			next := Cell{cell.Q + d.Q, cell.R + d.R}
			if occupied[next] && !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	if len(seen) != len(cells) {
		return Footprint{}, fmt.Errorf("disconnected footprint")
	}
	var result Footprint
	for _, cell := range cells {
		a := Arena{Center: center.Add(AxialPoint(cell.Q, cell.R).Scale(radius)), Radius: radius}
		result.cells = append(result.cells, a)
		vertices := a.Vertices()
		for side, d := range steps {
			if !occupied[Cell{cell.Q + d.Q, cell.R + d.R}] {
				result.boundary = append(result.boundary, Segment{vertices[side], vertices[(side+1)%6]})
			}
		}
	}
	return result, nil
}

func (f Footprint) Valid() bool         { return len(f.cells) > 0 }
func (f Footprint) Cells() []Arena      { return append([]Arena(nil), f.cells...) }
func (f Footprint) Boundary() []Segment { return append([]Segment(nil), f.boundary...) }

func nearestOnSegment(p model.Vec, s Segment) model.Vec {
	d := s.B.Sub(s.A)
	if dot(d, d) == 0 {
		return s.A
	}
	return s.A.Add(d.Scale(max(0, min(1, dot(p.Sub(s.A), d)/dot(d, d)))))
}
func (f Footprint) ContainsDisk(p model.Vec, radius float64) bool {
	if !p.Finite() || radius < 0 || math.IsNaN(radius) || math.IsInf(radius, 0) {
		return false
	}
	inside := false
	for _, a := range f.cells {
		if a.ContainsDisk(p, 0) {
			inside = true
			break
		}
	}
	if !inside {
		return false
	}
	for _, edge := range f.boundary {
		if nearestOnSegment(p, edge).Sub(p).Length()+1e-8 < radius {
			return false
		}
	}
	return true
}
func (f Footprint) ClampDisk(p model.Vec, radius float64) model.Vec {
	if !f.Valid() {
		return p
	}
	if f.ContainsDisk(p, radius) {
		return p
	}
	best := f.cells[0].Center
	distance := math.Inf(1)
	for _, a := range f.cells {
		candidate := a.ClampDisk(p, radius)
		d := candidate.Sub(p).Length()
		if d < distance {
			best, distance = candidate, d
		}
	}
	return best
}

// ClipSegment returns the first continuous portion in the union. A projectile
// cannot skip across an exterior notch into a second intersection.
func (f Footprint) ClipSegment(s Segment) (Segment, bool) {
	d := s.B.Sub(s.A)
	length := dot(d, d)
	if length < 1e-16 {
		return s, f.ContainsDisk(s.A, 0)
	}
	type span struct{ lo, hi float64 }
	var storage [128]span
	spans := storage[:0]
	for _, a := range f.cells {
		if v, ok := a.ClipSegment(s); ok {
			spans = append(spans, span{dot(v.A.Sub(s.A), d) / length, dot(v.B.Sub(s.A), d) / length})
		}
	}
	if len(spans) == 0 {
		return Segment{}, false
	}
	slices.SortFunc(spans, func(a, b span) int {
		if a.lo < b.lo {
			return -1
		}
		if a.lo > b.lo {
			return 1
		}
		return 0
	})
	lo, hi := spans[0].lo, spans[0].hi
	for _, v := range spans[1:] {
		if v.lo > hi+1e-8 {
			break
		}
		hi = max(hi, v.hi)
	}
	return Segment{A: s.A.Add(d.Scale(lo)), B: s.A.Add(d.Scale(hi))}, true
}

func (f Footprint) Grid(cellRadius float64) []Segment {
	if !f.Valid() {
		return nil
	}
	seed := f.cells[0]
	limit := seed.Radius
	for _, a := range f.cells {
		limit = max(limit, a.Center.Sub(seed.Center).Length()+a.Radius)
	}
	seed.Radius = limit * 1.2
	var result []Segment
	for _, s := range seed.Grid(cellRadius) {
		if clipped, ok := f.ClipSegment(s); ok && clipped.B.Sub(clipped.A).Length() > 1e-8 {
			result = append(result, clipped)
		}
	}
	return result
}
