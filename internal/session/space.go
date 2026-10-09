package session

import (
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
)

func (t *team) spaceArena(node string) geometry.Arena {
	w, h := t.catalog.SpaceSize(t.nodes[node])
	return geometry.NewArena(w, h)
}
func (t *team) spawnAt(node, except string) model.Vec {
	a := t.spaceArena(node)
	for slot := 0; slot <= len(t.players); slot++ {
		p := a.Spawn(slot, 12)
		free := true
		for id, m := range t.players {
			if id != except && m.player.Node == node && m.player.Scene == "" && m.player.Position.Sub(p).Length() < 24 {
				free = false
				break
			}
		}
		if free {
			return p
		}
	}
	return a.Center
}
func (t *team) placeInSpace(m *member, node string) {
	m.player.Node = node
	m.player.Position = t.spawnAt(node, m.player.ID)
	m.player.PositionEpoch++
	m.walk = model.Input{}
}

func (t *team) freeSpace(node string) (*scene.FreeSpace, error) {
	if s := t.spaces[node]; s != nil {
		return s, nil
	}
	definition := t.nodes[node]
	room, _ := t.catalog.Room(definition.Room)
	room.Width, room.Height = t.catalog.SpaceSize(definition)
	s, err := scene.NewFreeSpace(t.catalog, scene.Config{ShotOrigin: t.projectileOrigin("space:" + node), AdventureTime: t.adventureTime, ID: "space:" + node, Floor: max(1, t.floor), Seed: content.Seed(t.config.Seed, t.floor, "space:"+node), TickRate: t.config.TickRate}, room)
	if err != nil {
		return nil, err
	}
	s.EnableProjectileTransit()
	s.SetRuntime(t.runtime)
	if t.spaces == nil {
		t.spaces = map[string]*scene.FreeSpace{}
	}
	t.spaces[node] = s
	return s, nil
}

func spaceActionsAllowed(phase string) bool {
	return phase == "lobby" || phase == "preparation" || phase == "exploring" || phase == "intermission"
}

func (t *team) projectileOrigin(id string) string {
	t.nextHost++
	return fmt.Sprintf("%s:%d:%d", id, t.floor, t.nextHost)
}

// The coordinator owns non-battle actions; rendering and input-send frequency
// do not determine simulation speed. Stale input stops intent, not recharge.
func (t *team) stepSpaces() {
	rate := t.config.TickRate
	if !timing.LogicRate(rate) {
		rate = timing.DefaultLogicRate
	}
	dt := 1 / float64(rate)
	for _, id := range t.ids() {
		m := t.players[id]
		p := m.player
		if !spaceActionsAllowed(t.phase) || !p.Online || !p.Active || p.Life != "alive" || p.Scene != "" {
			m.walk = model.Input{}
			continue
		}
		if t.serverTick-m.walkUpdated > uint64(rate/2) {
			m.walk = model.Input{}
		}
		open := t.walkingSides(p.Node)
		if t.phase != "exploring" {
			open = nil
		}
		s, err := t.freeSpace(p.Node)
		if err != nil {
			t.send(m, protocol.ErrorMessage(err))
			continue
		}
		next, err := s.StepPlayer(&m.actions, p, m.walk, dt, open)
		m.walk.Dodge = false // an edge is consumed once, even if no charge was available
		if err != nil {
			t.send(m, protocol.ErrorMessage(err))
			continue
		}
		m.player = next
		if t.phase == "exploring" {
			t.passFreeDoor(m, next.Position.Sub(p.Position))
		}
	}
	for _, node := range t.spaceIDs() {
		s := t.spaces[node]
		values, err := s.AdvancePlayers(t.spacePlayers(node), dt, t.adventureTime)
		for _, p := range values {
			m := t.players[p.ID]
			m.player = p
			if err != nil {
				t.send(m, protocol.ErrorMessage(err))
			}
		}
	}
	t.exchangeProjectiles()
	t.withdrawDepartedVotes()
}
