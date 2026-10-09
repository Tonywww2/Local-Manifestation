package client

import (
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"reflect"
	"testing"
)

func TestStartingPlayerGetsAVisibleSpaceWithoutCombatScene(t *testing.T) {
	g := interfaceGame(t)
	a := geometry.NewArena(800, 500)
	g.state.Players[0].Position = a.Center
	g.state.Players = append(g.state.Players, model.Player{ID: "near", Node: "start", Active: true, Position: a.Spawn(1, 12)}, model.Player{ID: "far", Node: "room", Active: true})
	before := protocol.CloneState(*g.state)
	s := g.currentScene()
	if s == nil || s.State != "idle" || len(s.Players) != 2 || s.Players[0].ID != g.id {
		t.Fatal("starting avatar missing", s)
	}
	for _, oblique := range []bool{false, true} {
		g.oblique = oblique
		p := g.camera(*s).Project(s.Players[0].Position)
		if p.X < 24 || p.X > float64(g.width-24) || p.Y < 24 || p.Y > float64(g.height-24) {
			t.Fatal("avatar outside viewport", p)
		}
	}
	if !reflect.DeepEqual(before, *g.state) {
		t.Fatal("renderer mutated authority")
	}
}
func TestHexCameraFitsBothViewsAndLargeContent(t *testing.T) {
	for _, oblique := range []bool{false, true} {
		for _, size := range []model.Vec{{X: 800, Y: 500}, {X: 200, Y: 4096}, {X: 4096, Y: 200}, {X: 4096, Y: 4096}} {
			c := FitArenaCameraIn(size.X, size.Y, oblique, ui.Rect{X: 24, Y: 24, W: 976, H: 672})
			for _, p := range geometry.NewArena(size.X, size.Y).Vertices() {
				v := c.Project(p)
				if v.X < 23.99 || v.X > 1000.01 || v.Y < 23.99 || v.Y > 696.01 || c.Unproject(v).Sub(p).Length() > 1e-8 {
					t.Fatal(size, oblique, v)
				}
			}
			mesh := buildArenaDrawing(size.X, size.Y, c)
			for _, m := range []arenaMesh{mesh.Floor, mesh.Grid, mesh.Border} {
				if len(m.Vertices) == 0 || len(m.Indices) == 0 {
					t.Fatal("missing arena mesh")
				}
				for _, index := range m.Indices {
					if int(index) >= len(m.Vertices) {
						t.Fatal("invalid mesh index")
					}
				}
			}
		}
	}
}
