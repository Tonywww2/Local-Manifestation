package geometry

import (
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestDoorSidesAndArrivalAgreeWithHexTopology(t *testing.T) {
	a := NewArena(800, 500)
	for side, step := range axialSteps {
		from := model.Node{}
		to := model.Node{Q: step[0], R: step[1]}
		s, ok := DoorSide(from, to)
		if !ok || s != side {
			t.Fatal(side, s, ok)
		}
		back, ok := DoorSide(to, from)
		if !ok || back != (side+3)%6 {
			t.Fatal("wrong reverse door")
		}
		if !a.NearDoor(a.DoorPoint(side, 24), side) || a.NearDoor(a.Center, side) {
			t.Fatal("invalid interaction zone")
		}
		if !a.ContainsDisk(a.Arrival(back), 12) {
			t.Fatal("invalid landing point")
		}
		out := AxialPoint(step[0], step[1]).Unit()
		wall := a.DoorPoint(side, 0).Sub(a.Center).Unit()
		if out.Sub(wall).Length() > 1e-9 {
			t.Fatal("map and world disagree")
		}
	}
	if _, ok := DoorSide(model.Node{}, model.Node{Q: 2}); ok {
		t.Fatal("accepted distant room")
	}
}
