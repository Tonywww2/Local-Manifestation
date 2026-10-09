package geometry

import (
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestSixDoorTransfersPreserveWorldPositionAndStepRemainder(t *testing.T) {
	a := NewArena(2000, 1250)
	b := NewArena(1200, 300) // custom envelopes share the same map lattice
	from := model.Node{Q: 3, R: -2}
	for side, step := range axialSteps {
		to := model.Node{Q: from.Q + step[0], R: from.R + step[1]}
		n := normals[side]
		tangent := model.Vec{X: -n.Y, Y: n.X}
		p := a.DoorPoint(side, -3).Add(tangent.Scale(11))
		if !a.CrossedDoor(p, side, 12) {
			t.Fatal("crossing not detected", side)
		}
		land := TransferPoint(from, to, a, b, p)
		if a.WorldPoint(from, p, a.Radius).Sub(b.WorldPoint(to, land, a.Radius)).Length() > 1e-8 {
			t.Fatal("teleported across shared edge", side)
		}
		if TransferPoint(to, from, b, a, land).Sub(p).Length() > 1e-8 {
			t.Fatal("inverse transfer", side)
		}
	}
}

func TestWalkOpensOnlyTheApertureAndNeverOtherWalls(t *testing.T) {
	a := NewArena(2000, 1250)
	for side, n := range normals {
		p := a.DoorPoint(side, -4)
		if a.ClampWalk(p, 12, []int{side}, 8).Sub(p).Length() > 1e-8 {
			t.Fatal("door blocked", side)
		}
		if !a.ContainsDisk(a.ClampWalk(p, 12, nil, 8), 12) {
			t.Fatal("closed wall escaped")
		}
		tangent := model.Vec{X: -n.Y, Y: n.X}
		wide := p.Add(tangent.Scale(100))
		if a.CrossedDoor(a.ClampWalk(wide, 12, []int{side}, 8), side, 12) {
			t.Fatal("crossed solid wall")
		}
		far := a.DoorPoint(side, -100)
		if a.ClampWalk(far, 12, []int{side}, 8).Sub(a.DoorPoint(side, 0)).Length() > 8.0001 {
			t.Fatal("unbounded prediction")
		}
	}
}
