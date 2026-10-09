package geometry

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"math"
	"testing"
)

func TestRegularHexAndDiskBoundaries(t *testing.T) {
	for _, size := range []model.Vec{{X: 800, Y: 500}, {X: 200, Y: 4096}, {X: 4096, Y: 200}} {
		a := NewArena(size.X, size.Y)
		v := a.Vertices()
		for i, p := range v {
			if math.Abs(p.Sub(a.Center).Length()-a.Radius) > 1e-8 || math.Abs(p.Sub(v[(i+1)%6]).Length()-a.Radius) > 1e-8 {
				t.Fatal("hexagon was stretched", size)
			}
		}
		if a.ContainsDisk(model.Vec{}, 0) || !a.ContainsDisk(a.Center, 12) {
			t.Fatal("rectangular corner accepted")
		}
		for x := -500.0; x <= size.X+500; x += 71 {
			for y := -500.0; y <= size.Y+500; y += 73 {
				p := a.ClampDisk(model.Vec{X: x, Y: y}, 12)
				if !a.ContainsDisk(p, 12) {
					t.Fatal("collision body outside hex", size, p)
				}
				if a.ClampDisk(p, 12).Sub(p).Length() > 1e-8 {
					t.Fatal("clamp is not stable")
				}
			}
		}
	}
}
func TestGridTilingHasNoDuplicateEdgesOrVoidLines(t *testing.T) {
	a := NewArena(800, 500)
	lines := a.Grid(26)
	if len(lines) < 100 {
		t.Fatal("missing grid")
	}
	seen := map[string]bool{}
	for _, s := range lines {
		if !a.ContainsDisk(s.A, 0) || !a.ContainsDisk(s.B, 0) {
			t.Fatal("grid escaped room", s)
		}
		d := s.B.Sub(s.A).Unit()
		angle := math.Atan2(d.Y, d.X) / (math.Pi / 3)
		if math.Abs(angle-math.Round(angle)) > 1e-8 {
			t.Fatal("grid is not hexagonal", angle)
		}
		p, q := s.A, s.B
		if p.X > q.X || p.X == q.X && p.Y > q.Y {
			p, q = q, p
		}
		key := fmt.Sprintf("%.6f:%.6f:%.6f:%.6f", p.X, p.Y, q.X, q.Y)
		if seen[key] {
			t.Fatal("duplicate grid edge", key)
		}
		seen[key] = true
	}
	clipped, ok := a.ClipSegment(Segment{A: a.Center.Add(model.Vec{X: -1000}), B: a.Center.Add(model.Vec{X: 1000})})
	if !ok || math.Abs(clipped.B.Sub(clipped.A).Length()-2*a.Radius) > 1e-8 {
		t.Fatal("incorrect room clipping")
	}
}
func TestEightSpawnPointsAreInsideAndSeparate(t *testing.T) {
	a := NewArena(800, 500)
	for i := 0; i < 8; i++ {
		p := a.Spawn(i, 12)
		if !a.ContainsDisk(p, 12) {
			t.Fatal(p)
		}
		for j := 0; j < i; j++ {
			if p.Sub(a.Spawn(j, 12)).Length() < 24 {
				t.Fatal("overlapping players", i, j)
			}
		}
	}
}
