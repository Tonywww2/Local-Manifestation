package geometry

import "hsmultiplayer/pkg/model"

// AxialDelta identifies a shared edge of flat-top hexagonal rooms.
var axialSteps = [6][2]int{{1, 0}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}, {1, -1}}

func DoorSide(from, to model.Node) (int, bool) {
	for side, step := range axialSteps {
		if to.Q-from.Q == step[0] && to.R-from.R == step[1] {
			return side, true
		}
	}
	return 0, false
}

func AxialPoint(q, r int) model.Vec {
	return model.Vec{X: 1.5 * float64(q), Y: sqrt3 * (float64(r) + float64(q)/2)}
}

func (a Arena) DoorPoint(side int, inset float64) model.Vec {
	if side < 0 || side >= len(normals) {
		return a.Center
	}
	return a.Center.Add(normals[side].Scale(max(0, a.Radius*sqrt3/2-inset)))
}

// The interaction zone lies inside the wall; walking through a solid wall never
// changes rooms. Both the renderer and coordinator use this world-space zone.
func (a Arena) NearDoor(position model.Vec, side int) bool {
	return a.ContainsDisk(position, 12) && position.Sub(a.DoorPoint(side, 24)).Length() <= 42
}

func (a Arena) Arrival(side int) model.Vec {
	return a.ClampDisk(a.DoorPoint(side, 12), 12)
}

func (a Arena) ExitDevice() model.Vec      { return a.DoorPoint(1, 36) }
func (a Arena) EmergencyDevice() model.Vec { return a.DoorPoint(4, 60) }
func (a Arena) NearExit(position model.Vec) bool {
	return a.ContainsDisk(position, 12) && position.Sub(a.ExitDevice()).Length() <= 42
}
