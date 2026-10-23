package content

import (
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"slices"
)

const DefaultSpaceWidth = 800.0
const DefaultSpaceHeight = 500.0

// SpaceSize provides one base hexagon envelope, not the composite bounds.
// Non-combat nodes use the development default; their hexes can extend it.
func (c *Catalog) SpaceSize(node model.Node) (width, height float64) {
	if room, ok := c.rooms[node.Room]; ok {
		return room.Width, room.Height
	}
	return DefaultSpaceWidth, DefaultSpaceHeight
}

// SpaceArena returns compiled immutable geometry, safe to share by value.
func (c *Catalog) SpaceArena(node model.Node) geometry.Arena {
	if a, ok := c.roomArenas[node.Room]; ok {
		return a
	}
	if a, ok := c.nodeArenas[node.Definition]; ok {
		return a
	}
	return geometry.NewArena(DefaultSpaceWidth, DefaultSpaceHeight)
}

func (c *Catalog) RoomArena(id string) (geometry.Arena, bool) {
	a, ok := c.roomArenas[id]
	return a, ok
}

func (c *Catalog) SpaceHexes(node model.Node) []model.HexCell {
	if room, ok := c.rooms[node.Room]; ok {
		return slices.Clone(room.Hexes)
	}
	return slices.Clone(c.nodes[node.Definition].Hexes)
}

func (c *Catalog) SpaceRoom(node model.Node) model.RoomDef {
	if room, ok := c.Room(node.Room); ok {
		return room
	}
	return model.RoomDef{Width: DefaultSpaceWidth, Height: DefaultSpaceHeight, Hexes: c.SpaceHexes(node)}
}
