package scene

import (
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/navigation"
	"hsmultiplayer/pkg/perf"
)

type enemyPath struct {
	at     float64
	target model.Vec
	points []model.Vec
}

func (w *world) clearObstacles(a, b model.Vec, radius float64) bool {
	return geometry.ObstaclesClear(a, b, radius, w.room.Obstacles)
}
func (w *world) enemyMove(e model.Enemy, move, target model.Vec, speed, dt float64) model.Vec {
	defer w.performance.Since("scene.logic.enemy_navigation", perf.Start())
	candidate := w.clamp(e.Position.Add(move.Unit().Scale(speed*dt)), e.Radius)
	if w.clearMove(e.Position, candidate, e.Radius) {
		return candidate
	}
	if w.paths == nil {
		w.paths = map[int]enemyPath{}
	}
	cached, ok := w.paths[e.ID]
	if !ok || w.clock-cached.at > .3 || target.Sub(cached.target).Length() > 96 {
		path, err := navigation.Path(navigation.Space{Arena: w.arenaGeometry(), Obstacles: w.room.Obstacles, Radius: e.Radius}, e.Position, target)
		if err != nil {
			path = nil
		}
		cached = enemyPath{at: w.clock, target: target, points: path}
	}
	for len(cached.points) > 0 && cached.points[0].Sub(e.Position).Length() < 8 {
		cached.points = cached.points[1:]
	}
	w.paths[e.ID] = cached
	if len(cached.points) == 0 {
		return e.Position
	}
	next := e.Position.Add(cached.points[0].Sub(e.Position).Unit().Scale(speed * dt))
	if w.clearMove(e.Position, next, e.Radius) {
		return next
	}
	return e.Position
}
