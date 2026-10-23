package client

import (
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"slices"
)

type sceneGeometry struct {
	Width, Height float64
	Hexes         []model.HexCell
	Arena         geometry.Arena
}

func compileSceneArena(s protocol.Scene) geometry.Arena {
	a, err := geometry.NewRoomArena(s.Width, s.Height, s.Hexes)
	if err != nil {
		return geometry.NewArena(s.Width, s.Height)
	}
	return a
}

// Snapshots are values; compiled shapes belong to the client and are reused by
// prediction, navigation and interactions without per-frame reconstruction.
func (g *Game) sceneArena(s protocol.Scene) geometry.Arena {
	if cached, ok := g.arenaShapes[s.ID]; ok && cached.Width == s.Width && cached.Height == s.Height && slices.Equal(cached.Hexes, s.Hexes) {
		return cached.Arena
	}
	a := compileSceneArena(s)
	if g.arenaShapes == nil {
		g.arenaShapes = map[string]sceneGeometry{}
	}
	g.arenaShapes[s.ID] = sceneGeometry{Width: s.Width, Height: s.Height, Hexes: slices.Clone(s.Hexes), Arena: a}
	return a
}
