package session

import (
	"slices"

	"hsmultiplayer/internal/scene"
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
