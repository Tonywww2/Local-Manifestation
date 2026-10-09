package client

import (
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"math"
)

type Camera struct {
	Origin  model.Vec
	Scale   float64
	Oblique bool
}

// FitCamera fits the projection, so drawing and cursor input use the same bounds.
func FitCamera(width, height float64, oblique bool) Camera {
	return FitCameraIn(width, height, oblique, ui.Rect{X: 65, Y: 190, W: 840, H: 390})
}
func FitCameraIn(width, height float64, oblique bool, bounds ui.Rect) Camera {
	return fitPoints([]model.Vec{{}, {X: width}, {Y: height}, {X: width, Y: height}}, oblique, bounds)
}
func FitArenaCameraIn(width, height float64, oblique bool, bounds ui.Rect) Camera {
	points := geometry.NewArena(width, height).Vertices()
	return fitPoints(points[:], oblique, bounds)
}
func fitPoints(points []model.Vec, oblique bool, bounds ui.Rect) Camera {
	c := Camera{Scale: 1, Oblique: oblique}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range points {
		v := c.Project(p)
		minX = math.Min(minX, v.X)
		maxX = math.Max(maxX, v.X)
		minY = math.Min(minY, v.Y)
		maxY = math.Max(maxY, v.Y)
	}
	c.Scale = math.Min(bounds.W/math.Max(1, maxX-minX), bounds.H/math.Max(1, maxY-minY))
	c.Origin = model.Vec{X: bounds.X + bounds.W/2 - (minX+maxX)*c.Scale/2, Y: bounds.Y + bounds.H/2 - (minY+maxY)*c.Scale/2}
	return c
}

func mapPositionsIn(nodes []model.Node, bounds ui.Rect) (map[string]model.Vec, float64) {
	positions := map[string]model.Vec{}
	if len(nodes) == 0 {
		return positions, 8
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, n := range nodes {
		v := model.Vec{X: 1.5 * float64(n.Q), Y: math.Sqrt(3) * (float64(n.R) + float64(n.Q)/2)}
		positions[n.ID] = v
		minX = min(minX, v.X)
		maxX = max(maxX, v.X)
		minY = min(minY, v.Y)
		maxY = max(maxY, v.Y)
	}
	scale := math.Min(bounds.W/(maxX-minX+2.8), bounds.H/(maxY-minY+2.8))
	for id, v := range positions {
		positions[id] = model.Vec{X: bounds.X + bounds.W/2 + (v.X-(minX+maxX)/2)*scale, Y: bounds.Y + bounds.H/2 + (v.Y-(minY+maxY)/2)*scale}
	}
	return positions, scale * .94
}

func mapPositionsWithTerrain(nodes []model.Node, terrain []model.Terrain, bounds ui.Rect) (map[string]model.Vec, float64) {
	cells := make([]model.Node, 0, len(nodes)+len(terrain))
	cells = append(cells, nodes...)
	for _, t := range terrain {
		cells = append(cells, model.Node{ID: terrainID(t), Q: t.Q, R: t.R})
	}
	return mapPositionsIn(cells, bounds)
}

func mapPositions(nodes []model.Node) (map[string]model.Vec, float64) {
	positions := map[string]model.Vec{}
	if len(nodes) == 0 {
		return positions, 43
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, n := range nodes {
		v := model.Vec{X: float64(n.Q)*175 + float64(n.R)*85, Y: float64(n.R) * 110}
		positions[n.ID] = v
		minX = math.Min(minX, v.X)
		maxX = math.Max(maxX, v.X)
		minY = math.Min(minY, v.Y)
		maxY = math.Max(maxY, v.Y)
	}
	scale := math.Min(1, math.Min(760/math.Max(1, maxX-minX+120), 330/math.Max(1, maxY-minY+120)))
	for id, v := range positions {
		positions[id] = model.Vec{X: 485 + (v.X-(minX+maxX)/2)*scale, Y: 350 + (v.Y-(minY+maxY)/2)*scale}
	}
	return positions, 43 * scale
}

func (c Camera) Project(p model.Vec) model.Vec {
	if c.Oblique {
		return model.Vec{X: c.Origin.X + (p.X-.38*p.Y)*c.Scale, Y: c.Origin.Y + p.Y*.68*c.Scale}
	}
	return c.Origin.Add(p.Scale(c.Scale))
}
func (c Camera) Unproject(p model.Vec) model.Vec {
	v := p.Sub(c.Origin).Scale(1 / c.Scale)
	if c.Oblique {
		v.Y /= .68
		v.X += .38 * v.Y
	}
	return v
}
