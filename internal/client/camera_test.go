package client

import (
	"hsmultiplayer/pkg/model"
	"math"
	"testing"
)

func TestBothCameraInverses(t *testing.T) {
	for _, oblique := range []bool{false, true} {
		c := Camera{Origin: model.Vec{X: 137, Y: 80}, Scale: 1.4, Oblique: oblique}
		world := model.Vec{X: 700, Y: 420}
		got := c.Unproject(c.Project(world))
		if math.Abs(got.X-world.X) > 1e-9 || math.Abs(got.Y-world.Y) > 1e-9 {
			t.Fatal(oblique, got)
		}
	}
}
func TestContentDimensionsFitBothViews(t *testing.T) {
	for _, oblique := range []bool{false, true} {
		for _, size := range []model.Vec{{X: 800, Y: 500}, {X: 4096, Y: 200}, {X: 200, Y: 4096}} {
			c := FitCamera(size.X, size.Y, oblique)
			for _, p := range []model.Vec{{}, {X: size.X}, {Y: size.Y}, size} {
				v := c.Project(p)
				if v.X < 64.99 || v.X > 905.01 || v.Y < 189.99 || v.Y > 580.01 || c.Unproject(v).Sub(p).Length() > 1e-8 {
					t.Fatal(size, oblique, v)
				}
			}
		}
	}
	positions, radius := mapPositions([]model.Node{{ID: "a", Q: -100, R: -100}, {ID: "b", Q: 100, R: 100}})
	for _, v := range positions {
		if v.X-radius < 80 || v.X+radius > 890 || v.Y-radius < 160 || v.Y+radius > 540 {
			t.Fatal("map exceeds viewport", v, radius)
		}
	}
}
