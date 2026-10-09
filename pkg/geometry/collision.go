package geometry

import "hsmultiplayer/pkg/model"

// ObstaclesClear checks scene content only. The caller separately chooses its
// boundary policy (closed arena, open passage or projectile transit).
func ObstaclesClear(from, to model.Vec, radius float64, obstacles []model.Obstacle) bool {
	if !from.Finite() || !to.Finite() || radius < 0 {
		return false
	}
	for _, obstacle := range obstacles {
		if !DiskStepClear(from, to, obstacle.Position, obstacle.Radius+radius) {
			return false
		}
	}
	return true
}

// DiskStepClear blocks entry into an obstacle, but permits escape from an
// existing overlap. Checking the whole step also prevents crossing a small body.
func DiskStepClear(from, to, center model.Vec, clearance float64) bool {
	start := from.Sub(center)
	step := to.Sub(from)
	length2 := step.X*step.X + step.Y*step.Y
	if length2 == 0 {
		return true
	}
	dot := start.X*step.X + start.Y*step.Y
	if start.Length() < clearance {
		// Distance must not initially decrease, even if the endpoint is outside.
		return dot >= -1e-9
	}
	t := max(0, min(1, -dot/length2))
	return start.Add(step.Scale(t)).Length() >= clearance-1e-9
}
