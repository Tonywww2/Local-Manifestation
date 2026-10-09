package geometry

import (
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestFootprintOpenSeamAndExterior(t *testing.T) {
	f, err := NewFootprint(model.Vec{}, 100, []Cell{{0, 0}, {-1, 0}, {1, -1}})
	if err != nil {
		t.Fatal(err)
	}
	seam := model.Vec{X: -75, Y: -sqrt3 * 25}
	if !f.ContainsDisk(seam, 12) || f.ClampDisk(seam, 12) != seam {
		t.Fatal("shared seam blocked")
	}
	if f.ContainsDisk(model.Vec{Y: -120}, 0) || f.ContainsDisk(model.Vec{Y: 84}, 12) {
		t.Fatal("exterior notch or disk border ignored")
	}
	start, end := model.Vec{X: -150, Y: -120}, model.Vec{X: 150, Y: -120}
	clipped, ok := f.ClipSegment(Segment{start, end})
	if !ok || clipped.B.Sub(end).Length() < 1 {
		t.Fatal("projectile skipped exterior notch")
	}
	path := Segment{model.Vec{}, model.Vec{X: -150, Y: -sqrt3 * 50}}
	clipped, ok = f.ClipSegment(path)
	if !ok || clipped.B.Sub(path.B).Length() > 1e-7 {
		t.Fatal("projectile seam blocked")
	}
}
