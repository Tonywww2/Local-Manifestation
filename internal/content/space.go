package content

import "hsmultiplayer/pkg/model"

const DefaultSpaceWidth = 2000.0
const DefaultSpaceHeight = 1250.0

// SpaceSize provides the same envelope to the coordinator and client.
// Non-combat nodes have a development default until explicit space definitions arrive.
func (c *Catalog) SpaceSize(node model.Node) (width, height float64) {
	if room, ok := c.rooms[node.Room]; ok {
		return room.Width, room.Height
	}
	return DefaultSpaceWidth, DefaultSpaceHeight
}
