// Package geometry contains pure world-space room geometry shared by simulation and display.
package geometry

import (
	"hsmultiplayer/pkg/model"
	"math"
)

const sqrt3 = 1.7320508075688772

// Arena is a regular, flat-top hexagon centered in its content envelope.
// Width/height limit its size; they do not stretch the hexagon or its grid.
type Arena struct {
	Center model.Vec
	Radius float64
}
type Segment struct{ A, B model.Vec }

var normals = [6]model.Vec{{X: sqrt3 / 2, Y: .5}, {Y: 1}, {X: -sqrt3 / 2, Y: .5}, {X: -sqrt3 / 2, Y: -.5}, {Y: -1}, {X: sqrt3 / 2, Y: -.5}}
var directions = [6]model.Vec{{X: 1}, {X: .5, Y: sqrt3 / 2}, {X: -.5, Y: sqrt3 / 2}, {X: -1}, {X: -.5, Y: -sqrt3 / 2}, {X: .5, Y: -sqrt3 / 2}}

func NewArena(width, height float64) Arena {
	return Arena{Center: model.Vec{X: width / 2, Y: height / 2}, Radius: min(width/2, height/sqrt3)}
}
func dot(a, b model.Vec) float64 { return a.X*b.X + a.Y*b.Y }
func (a Arena) Vertices() [6]model.Vec {
	var out [6]model.Vec
	for i, d := range directions {
		out[i] = a.Center.Add(d.Scale(a.Radius))
	}
	return out
}
func (a Arena) ContainsDisk(p model.Vec, radius float64) bool {
	if !p.Finite() || radius < 0 || math.IsNaN(radius) || math.IsInf(radius, 0) {
		return false
	}
	limit := a.Radius*sqrt3/2 - radius
	if limit < 0 {
		return false
	}
	for _, n := range normals {
		if dot(p.Sub(a.Center), n) > limit+1e-9 {
			return false
		}
	}
	return true
}

// ClampDisk projects to the closest point of the inset hexagon, including corners.
// Its result keeps the whole collision disk inside, rather than just its center.
func (a Arena) ClampDisk(p model.Vec, radius float64) model.Vec {
	if !p.Finite() || radius < 0 || math.IsNaN(radius) || radius >= a.Radius*sqrt3/2 {
		return a.Center
	}
	if a.ContainsDisk(p, radius) {
		return p
	}
	inner := a
	inner.Radius -= radius * 2 / sqrt3
	v := inner.Vertices()
	best := a.Center
	bestDistance := math.Inf(1)
	for i, start := range v {
		end := v[(i+1)%6]
		d := end.Sub(start)
		t := max(0, min(1, dot(p.Sub(start), d)/dot(d, d)))
		candidate := start.Add(d.Scale(t))
		distance := dot(candidate.Sub(p), candidate.Sub(p))
		if distance < bestDistance {
			bestDistance, best = distance, candidate
		}
	}
	return best
}

// ClipSegment clips a line to the convex room in world coordinates.
func (a Arena) ClipSegment(s Segment) (Segment, bool) {
	lo, hi := 0.0, 1.0
	d := s.B.Sub(s.A)
	for _, n := range normals {
		available := a.Radius*sqrt3/2 - dot(s.A.Sub(a.Center), n)
		change := dot(d, n)
		if math.Abs(change) < 1e-12 {
			if available < -1e-9 {
				return Segment{}, false
			}
			continue
		}
		t := available / change
		if change > 0 {
			hi = min(hi, t)
		} else {
			lo = max(lo, t)
		}
		if lo > hi {
			return Segment{}, false
		}
	}
	return Segment{A: s.A.Add(d.Scale(lo)), B: s.A.Add(d.Scale(hi))}, true
}

// Grid uses a flat-top axial tiling. Three edges per cell avoid duplicate strokes.
// Boundary cells are clipped; no rectangular lines extend into the void.
func (a Arena) Grid(cellRadius float64) []Segment {
	if cellRadius <= 0 || math.IsNaN(cellRadius) || math.IsInf(cellRadius, 0) || a.Radius <= 0 {
		return nil
	}
	columns := int(math.Ceil(a.Radius/(1.5*cellRadius))) + 2
	rows := int(math.Ceil(a.Radius/(sqrt3*cellRadius))) + columns/2 + 2
	var out []Segment
	for q := -columns; q <= columns; q++ {
		for r := -rows; r <= rows; r++ {
			center := a.Center.Add(model.Vec{X: 1.5 * cellRadius * float64(q), Y: sqrt3 * cellRadius * (float64(r) + float64(q)/2)})
			for i := 0; i < 3; i++ {
				s := Segment{A: center.Add(directions[i].Scale(cellRadius)), B: center.Add(directions[i+1].Scale(cellRadius))}
				if clipped, ok := a.ClipSegment(s); ok && clipped.B.Sub(clipped.A).Length() > 1e-8 {
					out = append(out, clipped)
				}
			}
		}
	}
	return out
}

// Spawn follows an axial spiral, keeping the first eight players separately visible.
func (a Arena) Spawn(slot int, bodyRadius float64) model.Vec {
	if slot <= 0 {
		return a.Center
	}
	index := slot - 1
	ring := 1
	for index >= 6*ring {
		index -= 6 * ring
		ring++
	}
	q, r := -ring, ring
	steps := [6][2]int{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}
	for side := 0; side < 6; side++ {
		n := min(index, ring)
		q += steps[side][0] * n
		r += steps[side][1] * n
		index -= n
		if index == 0 {
			break
		}
	}
	spacing := max(36, bodyRadius*3)
	p := a.Center.Add(model.Vec{X: spacing * sqrt3 / 2 * float64(q), Y: spacing * (float64(r) + float64(q)/2)})
	return a.ClampDisk(p, bodyRadius)
}
