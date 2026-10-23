package client

import (
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/navigation"
	"hsmultiplayer/pkg/protocol"
	"time"
)

func obstacles(s protocol.Scene) []model.Obstacle {
	v := append([]model.Obstacle(nil), s.Obstacles...)
	for _, e := range s.Enemies {
		if e.HP > 0 {
			v = append(v, model.Obstacle{Position: e.Position, Radius: e.Radius})
		}
	}
	return v
}
func (g *Game) clearWalk(s protocol.Scene, from, to model.Vec) bool {
	// Prediction uses the same swept boundary as the authoritative simulation.
	if !g.sceneArena(s).ClearStep(from, to, 12) {
		return false
	}
	for _, o := range s.Obstacles {
		if !geometry.DiskStepClear(from, to, o.Position, o.Radius+12) {
			return false
		}
	}
	for _, e := range s.Enemies {
		if e.HP > 0 && !geometry.DiskStepClear(from, to, e.Position, e.Radius+12) {
			return false
		}
	}
	return true
}
func (g *Game) navigationDirection(p model.Player, s protocol.Scene) model.Vec {
	if g.destination == nil {
		return model.Vec{}
	}
	goal := *g.destination
	if g.clickDirect {
		return goal.Sub(p.Position)
	}
	now := time.Now()
	if g.pathAt.IsZero() || now.Sub(g.pathAt) > 300*time.Millisecond {
		g.pathAt = now
		path, err := navigation.Path(navigation.Space{Arena: g.sceneArena(s), Obstacles: obstacles(s), Radius: 12}, p.Position, goal)
		if err == nil {
			g.waypoints = path
		} else {
			g.waypoints = nil
		}
	}
	for len(g.waypoints) > 0 && g.waypoints[0].Sub(p.Position).Length() < 8 {
		g.waypoints = g.waypoints[1:]
	}
	if len(g.waypoints) == 0 {
		return model.Vec{}
	}
	return g.waypoints[0].Sub(p.Position)
}
