package session

import (
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

// Granting is a normal inventory transaction. A scene owns its combat players;
// reserve a detached value before planning, then commit only a complete grant.
func (t *team) debugGrantItem(m *member, definition string) error {
	if m.player.ID != t.admin {
		return protocol.Reject("error.admin_only")
	}
	p := m.player
	actor := t.actors[p.Scene]
	if actor != nil {
		players, err := actor.Reserve([]string{p.ID})
		if err != nil {
			return err
		}
		defer actor.Abort()
		p = players[0]
	}
	next, err := t.catalog.GrantItem(p, definition)
	if err != nil {
		return err
	}
	if actor != nil {
		if err := actor.Commit([]model.Player{next}); err != nil {
			return err
		}
	}
	m.player = next
	return nil
}
