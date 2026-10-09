package session

import (
	"hsmultiplayer/pkg/device"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func (t *team) walkingSides(node string) []int {
	from := t.nodes[node]
	var sides []int
	for _, id := range from.Edges {
		to := t.nodes[id]
		if side, ok := geometry.DoorSide(from, to); ok && expedition.FreeTransit(to) {
			sides = append(sides, side)
		}
	}
	return sides
}

func (t *team) doorSide(m *member, target string) (int, bool) {
	from, ok := t.nodes[m.player.Node]
	to, exists := t.nodes[target]
	if !ok || !exists {
		return 0, false
	}
	for _, id := range from.Edges {
		if id == target {
			return geometry.DoorSide(from, to)
		}
	}
	return 0, false
}

func (t *team) atDoor(m *member, target string) bool {
	a := t.spaceArena(m.player.Node)
	e, ok := device.RoomEntrance(t.nodes[m.player.Node], t.nodes[target], a)
	_, adjacent := t.doorSide(m, target)
	return ok && adjacent && m.player.Scene == "" && a.ContainsDisk(m.player.Position, 12) && e.Near(m.player.Position)
}

func (t *team) sharedDoorReady(target string) error {
	for _, m := range t.players {
		p := m.player
		if p.Active && p.Online && p.Life == "alive" && !t.atDoor(m, target) {
			return protocol.Reject("error.team_at_door")
		}
	}
	return nil
}

func (t *team) enterDoor(m *member, target string) error {
	if !m.player.Active || !m.player.Online || m.player.Life != "alive" {
		return protocol.Reject("error.incapacitated")
	}
	if m.player.Scene != "" {
		return protocol.Reject("error.already_battle")
	}
	if !t.atDoor(m, target) {
		return protocol.Reject("error.door_distance")
	}
	side, _ := t.doorSide(m, target)
	arrival := t.spaceArena(target).Arrival((side + 3) % 6)
	return t.enterAt(m, target, &arrival)
}

// Free, cleared rooms require an outward walking intent at their actual opening.
// Paid doors never charge or start votes merely because someone approaches them.
func (t *team) passFreeDoor(m *member, movement model.Vec) {
	from := t.nodes[m.player.Node]
	a := t.spaceArena(from.ID)
	for _, id := range from.Edges {
		to := t.nodes[id]
		if !expedition.FreeTransit(to) {
			continue
		}
		side, ok := geometry.DoorSide(from, to)
		if !ok || !a.CrossedDoor(m.player.Position, side, 12) {
			continue
		}
		out := a.DoorPoint(side, 0).Sub(a.Center).Unit()
		move := movement.Unit()
		if move.X*out.X+move.Y*out.Y <= 0 {
			continue
		}
		arrival := geometry.TransferPoint(from, to, a, t.spaceArena(id), m.player.Position)
		walk, epoch := m.walk, m.player.PositionEpoch
		if err := t.enterAt(m, id, &arrival); err != nil {
			m.player.Position = a.ClampDisk(m.player.Position, 12)
			t.send(m, protocol.ErrorMessage(err))
		} else {
			// A coordinate-frame change is continuous movement, not a teleport.
			walk.Aim = geometry.TransferPoint(from, to, a, t.spaceArena(id), walk.Aim)
			m.walk, m.player.PositionEpoch = walk, epoch
		}
		return
	}
}
