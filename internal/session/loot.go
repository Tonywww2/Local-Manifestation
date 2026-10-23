package session

import (
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"slices"
)

// The coordinator owns shared ground items. Battle rewards remain personal loot
// in each player's simulation owner. Requests are serialized before any commit.
func (t *team) dropItem(m *member, r protocol.Request) error {
	if len(t.groundLoot) >= 4096 {
		return protocol.Reject("error.loot_limit")
	}
	var next model.Player
	var drop model.Loot
	var err error
	if a := t.actors[m.player.Scene]; a != nil {
		next, drop, err = a.DropItem(m.player.ID, r.Item, r.Revision)
	} else {
		if inventory.Equipped(m.player.Inventory, r.Item) && m.actions.EquipmentLocked(t.catalog.Weapon(m.player.Weapon).RestrictSwap) {
			return protocol.Reject("error.equipment_cooldown")
		}
		next, drop, err = t.catalog.DropItem(m.player, r.Item, r.Revision)
	}
	if err != nil {
		return err
	}
	m.player = next
	t.groundLoot = append(t.groundLoot, drop)
	return nil
}

func (t *team) pickup(m *member, id string) error {
	index := slices.IndexFunc(t.groundLoot, func(drop model.Loot) bool { return drop.Item.ID == id })
	if index < 0 {
		if a := t.actors[m.player.Scene]; a != nil {
			return a.Pickup(m.player.ID, id)
		}
		next, err := t.catalog.Pickup(m.player, id)
		if err == nil {
			m.player = next
		}
		return err
	}
	p := m.player
	a := t.actors[p.Scene]
	if a != nil {
		values, err := a.Reserve([]string{p.ID})
		if err != nil {
			return err
		}
		defer a.Abort()
		p = values[0]
	}
	next, err := t.catalog.PickupDrop(p, t.groundLoot[index])
	if err != nil {
		return err
	}
	if a != nil {
		if err := a.Commit([]model.Player{next}); err != nil {
			return err
		}
	}
	m.player = next
	t.groundLoot = slices.Delete(t.groundLoot, index, index+1)
	return nil
}
