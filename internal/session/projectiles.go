package session

import (
	"slices"

	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/protocol"
)

func (t *team) spaceIDs() []string {
	ids := make([]string, 0, len(t.spaces))
	for id := range t.spaces {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (t *team) keepSceneProjectiles(node string, shots []scene.ProjectileTransfer) {
	if len(shots) == 0 {
		return
	}
	if space, err := t.freeSpace(node); err == nil {
		space.ReceiveProjectiles(shots)
	}
}

// Drain all owners before delivering, so an iteration never advances a shot
// twice simply because its destination occurs later in the map iteration.
func (t *team) exchangeProjectiles() {
	type outgoing struct {
		node  string
		shots []scene.ProjectileTransfer
	}
	var batches []outgoing
	for _, id := range t.sceneIDs() {
		if p := t.actors[id].TakeProjectiles(); len(p) > 0 {
			batches = append(batches, outgoing{id, p})
		}
	}
	for _, id := range t.spaceIDs() {
		if p := t.spaces[id].TakeProjectiles(); len(p) > 0 {
			batches = append(batches, outgoing{id, p})
		}
	}
	for _, batch := range batches {
		for _, shot := range batch.shots {
			t.routeProjectile(batch.node, shot)
		}
	}
}

func (t *team) routeProjectile(node string, shot scene.ProjectileTransfer) {
	type journey struct {
		node string
		shot scene.ProjectileTransfer
		hops int
	}
	queue := []journey{{node: node, shot: shot}}
	for len(queue) > 0 {
		j := queue[0]
		queue = queue[1:]
		if j.hops > len(t.nodes) {
			continue
		}
		from, ok := t.nodes[j.node]
		if !ok {
			continue
		}
		a := t.spaceArena(j.node)
		side, ok := a.ProjectileExit(j.shot.Path())
		if !ok {
			continue
		}
		target := ""
		for _, id := range from.Edges {
			if s, adjacent := geometry.DoorSide(from, t.nodes[id]); adjacent && s == side {
				target = id
				break
			}
		}
		if target == "" {
			continue
		}
		incoming := j.shot.Reframe(from, t.nodes[target], a, t.spaceArena(target))
		var outgoing []scene.ProjectileTransfer
		var err error
		if actor := t.actors[target]; actor != nil {
			outgoing, err = actor.SweepProjectiles(incoming)
		} else if space, loadErr := t.freeSpace(target); loadErr == nil {
			values, onward, sweepErr := space.SweepProjectiles([]scene.ProjectileTransfer{incoming}, t.spacePlayers(target), t.adventureTime)
			outgoing, err = onward, sweepErr
			for _, p := range values {
				t.players[p.ID].player = p
			}
		} else {
			err = loadErr
		}
		if err != nil {
			for _, m := range t.players {
				if m.player.Node == target {
					t.send(m, protocol.ErrorMessage(err))
				}
			}
			continue
		}
		for _, p := range outgoing {
			queue = append(queue, journey{target, p, j.hops + 1})
		}
	}
}

func (t *team) spacePlayers(node string) []scene.SpacePlayer {
	var players []scene.SpacePlayer
	for _, id := range t.ids() {
		m := t.players[id]
		if m.player.Node == node && m.player.Scene == "" && m.player.Active {
			players = append(players, scene.SpacePlayer{Player: m.player, Actions: &m.actions})
		}
	}
	return players
}
