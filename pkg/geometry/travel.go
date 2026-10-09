package geometry

import "hsmultiplayer/pkg/model"

const DoorHalfWidth = 28.0

// WorldPoint embeds room-local coordinates in a shared hexagonal lattice.
// The cell radius is independent of camera zoom and of a custom room envelope.
func (a Arena) WorldPoint(n model.Node, p model.Vec, cellRadius float64) model.Vec {
	return AxialPoint(n.Q, n.R).Scale(cellRadius).Add(p.Sub(a.Center).Scale(cellRadius / a.Radius))
}

func (a Arena) LocalPoint(n model.Node, world model.Vec, cellRadius float64) model.Vec {
	return a.Center.Add(world.Sub(AxialPoint(n.Q, n.R).Scale(cellRadius)).Scale(a.Radius / cellRadius))
}

// TransferPoint changes the coordinate frame, preserving the exact world point,
// including lateral offset and the remainder of the movement step.
func TransferPoint(from, to model.Node, a, b Arena, position model.Vec) model.Vec {
	return b.LocalPoint(to, a.WorldPoint(from, position, a.Radius), a.Radius)
}

// ProjectileExit finds the first edge crossing along a swept trajectory.
// The coordinator decides whether that edge connects to another room.
func (a Arena) ProjectileExit(path Segment) (int, bool) {
	if a.ContainsDisk(path.B, 0) {
		return 0, false
	}
	clipped, ok := a.ClipSegment(path)
	if !ok {
		return 0, false
	}
	for side, n := range normals {
		offset := clipped.B.Sub(a.Center)
		if abs(dot(offset, n)-a.Radius*sqrt3/2) < 1e-6 {
			return side, true
		}
	}
	return 0, false
}

// ClampWalk treats each explicitly open doorway as a corridor through the wall.
// The rest of the wall still contains the full body disk. Lead permits only a
// short continuation into the neighbor while the server handoff is in flight.
func (a Arena) ClampWalk(p model.Vec, radius float64, open []int, lead float64) model.Vec {
	best := a.ClampDisk(p, radius)
	if !p.Finite() || radius < 0 || radius >= DoorHalfWidth || lead < 0 {
		return best
	}
	bestDistance := best.Sub(p).Length()
	for _, side := range open {
		if side < 0 || side >= len(normals) {
			continue
		}
		n := normals[side]
		tangent := model.Vec{X: -n.Y, Y: n.X}
		offset := p.Sub(a.Center)
		edge := a.Radius * sqrt3 / 2
		depth := max(edge-2*radius, min(edge+lead, dot(offset, n)))
		lateral := max(radius-DoorHalfWidth, min(DoorHalfWidth-radius, dot(offset, tangent)))
		candidate := a.Center.Add(n.Scale(depth)).Add(tangent.Scale(lateral))
		if distance := candidate.Sub(p).Length(); distance < bestDistance {
			best, bestDistance = candidate, distance
		}
	}
	return best
}

func (a Arena) CrossedDoor(position model.Vec, side int, radius float64) bool {
	if side < 0 || side >= len(normals) || !position.Finite() {
		return false
	}
	n := normals[side]
	tangent := model.Vec{X: -n.Y, Y: n.X}
	offset := position.Sub(a.Center)
	return dot(offset, n) >= a.Radius*sqrt3/2 &&
		abs(dot(offset, tangent)) <= DoorHalfWidth-radius+1e-9
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
