package scene

import (
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
)

func (w *world) arenaGeometry() geometry.Arena {
	if w.arena.Radius > 0 {
		return w.arena
	}
	// Small developer fixtures may construct a world directly.
	a, _ := geometry.NewRoomArena(w.room.Width, w.room.Height, w.room.Hexes)
	return a
}

func (w *world) clearMove(from, to model.Vec, radius float64) bool {
	clear := w.arenaGeometry().ClearStep(from, to, radius)
	if w.footprint != nil {
		clear = w.footprint.ClearStep(from, to, radius)
	}
	return clear && w.clearObstacles(from, to, radius)
}
