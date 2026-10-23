package session

import (
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"reflect"
	"testing"
)

// A focused ownership check: shared drops must never disappear on rejection or
// be duplicated by two serialized pickup requests. It also covers scene owners.
func TestGroundLootOwnershipAndRollback(t *testing.T) {
	tm := fixture(t)
	a, b := tm.players["a"], tm.players["b"]
	id := a.player.Inventory.Equipment[0]
	for i := range a.player.Inventory.Items {
		if a.player.Inventory.Items[i].ID == id {
			a.player.Inventory.Items[i].Upgrade = 3
			a.player.Inventory.Items[i].WeaponBonus = 17
		}
	}
	var err error
	a.player, err = tm.catalog.RefreshEquipment(a.player)
	if err != nil {
		t.Fatal(err)
	}
	before := model.ClonePlayer(a.player)
	if err = tm.dropItem(a, protocol.Request{Item: id, Revision: before.Inventory.Revision + 1}); err == nil || !reflect.DeepEqual(before, a.player) || len(tm.groundLoot) != 0 {
		t.Fatal("stale drop changed ownership")
	}
	if err = tm.dropItem(a, protocol.Request{Item: id, Revision: before.Inventory.Revision}); err != nil {
		t.Fatal(err)
	}
	if a.player.Weapon != "" || a.player.Inventory.Equipment[0] != "" || len(tm.groundLoot) != 1 {
		t.Fatal("equipped item was not detached")
	}
	drop := tm.groundLoot[0]
	if drop.Item.Upgrade != 3 || drop.Item.WeaponBonus != 17 {
		t.Fatal("item components changed")
	}
	b.player.Node, b.player.Position = drop.Node, drop.Position.Add(model.Vec{X: 1000})
	if err = tm.pickup(b, id); err == nil || len(tm.groundLoot) != 1 {
		t.Fatal("distant pickup consumed drop")
	}
	b.player.Position = drop.Position
	unfilled := model.ClonePlayer(b.player)
	for inventory.BagCount(b.player.Inventory) < b.player.Inventory.Capacity {
		b.player, err = tm.catalog.GrantItem(b.player, drop.Item.Definition)
		if err != nil {
			t.Fatal(err)
		}
	}
	full := model.ClonePlayer(b.player)
	if err = tm.pickup(b, id); err == nil || len(tm.groundLoot) != 1 || !reflect.DeepEqual(full, b.player) {
		t.Fatal("full bag changed either owner")
	}
	b.player = unfilled
	if err = tm.pickup(b, id); err != nil {
		t.Fatal(err)
	}
	it, ok := inventory.Find(b.player.Inventory, id)
	if !ok || it != drop.Item || len(tm.groundLoot) != 0 {
		t.Fatal("shared pickup lost item components")
	}
	if err = tm.pickup(a, id); err == nil {
		t.Fatal("second player duplicated drop")
	}
	target, steps := "", 100000
	for _, node := range tm.nodes {
		route := tm.personalRoute(a.player.Node, node.ID)
		if node.Kind == "combat" && len(route) > 0 && (len(route) < steps || len(route) == steps && node.ID < target) {
			target, steps = node.ID, len(route)
		}
	}
	if target == "" {
		t.Fatal("no reachable battle in generated map")
	}
	if err = tm.enterGroup([]*member{a, b}, target, false); err != nil {
		t.Fatal(err)
	}
	// Both now have the combat actor as their authoritative inventory owner.
	view := tm.actors[target].Snapshot()
	for _, p := range view.Players {
		if p.ID == b.player.ID {
			b.player = p
		}
	}
	if err = tm.dropItem(b, protocol.Request{Item: id, Revision: b.player.Inventory.Revision}); err != nil {
		t.Fatal(err)
	}
	if err = tm.pickup(b, id); err != nil {
		t.Fatal(err)
	}
	view = tm.actors[target].Snapshot()
	for _, p := range view.Players {
		if p.ID == b.player.ID {
			if _, ok := inventory.Find(p.Inventory, id); !ok {
				t.Fatal("scene owner did not receive item")
			}
		}
	}
}
