package scene

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func (a *Actor) Equip(id, item string, slot int) error {
	return a.call(command{kind: "equip", id: id, player: model.Player{Weapon: item, Upgrade: slot}}).err
}

func (a *Actor) InventoryMouse(id string, request protocol.Request) (model.Player, error) {
	r := a.call(command{kind: "inventory_mouse", id: id, inventory: request})
	if r.err != nil {
		return model.Player{}, r.err
	}
	return r.players[0], nil
}

// Reserve is a brief owner-side transaction boundary, never a gameplay pause.
// No mutable world reference crosses it. The coordinator always defers Abort.
func (a *Actor) Reserve(ids []string) ([]model.Player, error) {
	r := a.call(command{kind: "prepare_transaction", ids: append([]string(nil), ids...)})
	return r.players, r.err
}
func (a *Actor) Commit(players []model.Player) error {
	n := make([]model.Player, len(players))
	for i, p := range players {
		n[i] = model.ClonePlayer(p)
	}
	return a.call(command{kind: "commit_transaction", players: n}).err
}
func (a *Actor) Abort() { a.call(command{kind: "abort_transaction"}) }
func (a *Actor) transaction(w *world, c command) {
	r := response{}
	reserved := map[string]bool{}
	for _, id := range c.ids {
		p := w.players[id]
		if p == nil || reserved[id] {
			r.err = fmt.Errorf("invalid transaction participant")
			c.reply <- r
			return
		}
		reserved[id] = true
		r.players = append(r.players, model.ClonePlayer(p.player))
	}
	c.reply <- r
	for {
		select {
		case next := <-a.commands:
			r := response{}
			switch next.kind {
			case "commit_transaction":
				if len(next.players) != len(reserved) {
					r.err = fmt.Errorf("transaction participant mismatch")
				} else {
					seen := map[string]bool{}
					for _, p := range next.players {
						if !reserved[p.ID] || seen[p.ID] {
							r.err = fmt.Errorf("transaction participant mismatch")
							break
						}
						seen[p.ID] = true
					}
					if r.err == nil {
						for _, p := range next.players {
							if p.Scene != w.id {
								delete(w.players, p.ID)
							} else {
								w.players[p.ID].player = model.ClonePlayer(p)
							}
						}
					}
				}
				next.reply <- r
				return
			case "abort_transaction":
				next.reply <- r
				return
			case "snapshot":
				r.snapshot = w.snapshot()
				next.reply <- r
			default:
				r.err = fmt.Errorf("transaction busy")
				next.reply <- r
			}
		}
	}
}
