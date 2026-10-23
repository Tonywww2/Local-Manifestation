package session

import (
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/device"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"slices"
	"sort"
)

func (t *team) inventoryRequest(m *member, r protocol.Request) (bool, error) {
	switch r.Kind {
	case "inventory_click", "inventory_quick", "inventory_return":
		var n model.Player
		var err error
		if a := t.actors[m.player.Scene]; a != nil {
			n, err = a.InventoryMouse(m.player.ID, r)
		} else {
			if r.Kind != "inventory_return" && (r.Area != "bag" || r.Kind == "inventory_quick") && m.actions.EquipmentLocked(t.catalog.Weapon(m.player.Weapon).RestrictSwap) {
				return true, protocol.Reject("error.equipment_cooldown")
			}
			n, err = t.catalog.InventoryMouse(m.player, r)
		}
		if err == nil {
			m.player = n
		}
		return true, err
	case "revive":
		return true, t.revive(m, r.Peer)
	case "pickup":
		return true, t.pickup(m, r.Item)
	case "drop_item":
		return true, t.dropItem(m, r)
	case "buy":
		if !t.atShop(m) {
			return true, protocol.Reject("error.shop_required")
		}
		d := t.catalog.Node(t.nodes[m.player.Node].Definition)
		for _, offer := range d.Shop {
			if offer.Item == r.Item {
				if m.player.Coins < offer.Price {
					return true, protocol.Reject("error.coins", "cost", fmt.Sprint(offer.Price))
				}
				n, err := t.catalog.GrantItem(m.player, offer.Item)
				if err != nil {
					return true, err
				}
				n.Coins -= offer.Price
				m.player = n
				return true, nil
			}
		}
		return true, fmt.Errorf("shop offer unavailable")
	case "weapon_skill", "use_item":
		if a := t.actors[m.player.Scene]; a != nil {
			if r.Kind == "use_item" && r.Slot >= 0 && r.Slot < 4 {
				item, ok := inventory.Find(m.player.Inventory, m.player.Inventory.Equipment[5+r.Slot])
				def, _ := t.catalog.Item(item.Definition)
				global := ok && slices.ContainsFunc(def.Use, func(e model.ItemEffect) bool { return e.Kind == "clear_curse" })
				if global {
					values, err := a.Reserve([]string{m.player.ID})
					if err != nil {
						return true, err
					}
					defer a.Abort()
					plan, err := t.catalog.UseItem(values[0], r.Slot)
					if err != nil {
						return true, err
					}
					if !plan.ClearCurse || plan.Support || plan.Projectile != nil {
						return true, fmt.Errorf("global item changed")
					}
					if err = a.Commit([]model.Player{plan.Player}); err != nil {
						return true, err
					}
					m.player = plan.Player
					t.cursed = false
					return true, nil
				}
			}
			return true, a.Action(m.player.ID, r.Kind, r.Slot, r.Input.Aim)
		}
		var plan content.ActionPlan
		var err error
		if r.Kind == "weapon_skill" {
			plan, err = t.catalog.UseWeaponSkill(m.player)
		} else {
			plan, err = t.catalog.UseItem(m.player, r.Slot)
		}
		if err != nil {
			return true, err
		}
		if plan.Support && plan.Projectile != nil {
			return true, fmt.Errorf("support and projectile effects require separate actions")
		}
		if plan.Projectile != nil || plan.ClearProjectiles != nil {
			space, err := t.freeSpace(m.player.Node)
			if err != nil {
				return true, err
			}
			if err := space.ApplyAction(m.player, r.Input.Aim, plan); err != nil {
				return true, err
			}
		}
		if plan.Support {
			a := t.actors[r.Node]
			if a == nil {
				return true, fmt.Errorf("choose an unfinished battle room on the map")
			}
			plan.Player, err = content.LeaveStatuses(plan.Player)
			if err != nil {
				return true, err
			}
			plan.Player.Node = r.Node
			plan.Player.Scene = r.Node
			if err = a.SupportWithActions(plan.Player, m.actions); err != nil {
				return true, err
			}
			m.actions = scene.PlayerActions{}
		}
		m.player = plan.Player
		if r.Kind == "weapon_skill" {
			m.actions.PlaySkill(m.player, r.Input.Aim, plan.ClearProjectiles != nil)
		}
		if plan.ClearCurse {
			t.cursed = false
		}
		return true, nil
	case "equip":
		if a := t.actors[m.player.Scene]; a != nil {
			return true, a.Equip(m.player.ID, r.Item, r.Slot)
		}
		if m.actions.EquipmentLocked(t.catalog.Weapon(m.player.Weapon).RestrictSwap) {
			return true, protocol.Reject("error.equipment_cooldown")
		}
		n, err := t.catalog.Equip(m.player, r.Item, r.Slot)
		if err == nil {
			m.player = n
		}
		return true, err
	case "sell":
		if m.player.Scene != "" || !t.atShop(m) {
			return true, protocol.Reject("error.shop_required")
		}
		n, err := t.catalog.Sell(m.player, r.Item)
		if err == nil {
			m.player = n
		}
		return true, err
	case "trade_offer", "trade_accept", "trade_cancel":
		return true, t.tradeRequest(m, r)
	}
	return false, nil
}
func (t *team) revive(m *member, id string) error {
	node := t.nodes[m.player.Node]
	def := t.catalog.Node(node.Definition)
	d, ok := device.Find(device.World(device.Context{Node: node, Arena: t.spaceArena(node.ID), Phase: t.phase, SharedAP: t.shared}), "revival:"+node.ID)
	if !ok || !d.Near(m.player.Position) || def.Facility != "revive" || m.player.Scene != "" {
		return fmt.Errorf("resurrection facility required")
	}
	peer := t.players[id]
	if peer == nil || peer.player.Life != "dead" {
		return fmt.Errorf("dead player unavailable")
	}
	if m.player.Coins < def.FacilityCost {
		return protocol.Reject("error.coins", "cost", fmt.Sprint(def.FacilityCost))
	}
	p := peer.player
	a := t.actors[p.Scene]
	if a != nil {
		values, err := a.Reserve([]string{id})
		if err != nil {
			return err
		}
		defer a.Abort()
		p = values[0]
	}
	n, err := content.RevivePlayer(p)
	if err != nil {
		return err
	}
	n.Scene = ""
	n.Node = node.ID
	n.Position = t.spawnAt(node.ID, n.ID)
	n.PositionEpoch++
	if a != nil {
		if err = a.Commit([]model.Player{n}); err != nil {
			return err
		}
	}
	m.player.Coins -= def.FacilityCost
	peer.player = n
	return nil
}
func (t *team) tradeRequest(m *member, r protocol.Request) error {
	if t.trades == nil {
		t.trades = map[string]protocol.Trade{}
	}
	id := r.Node
	v, ok := t.trades[id]
	if r.Kind == "trade_offer" && !ok {
		peer := t.players[r.Peer]
		if peer == nil || peer == m || !peer.player.Online {
			return fmt.Errorf("unavailable trade participant")
		}
		for _, old := range t.trades {
			if slices.Contains(old.Players[:], m.player.ID) || slices.Contains(old.Players[:], peer.player.ID) {
				return fmt.Errorf("participant already trading")
			}
		}
		t.nextTrade++
		id = fmt.Sprint(t.nextTrade)
		v = protocol.Trade{ID: id, Players: [2]string{m.player.ID, peer.player.ID}}
		v.InventoryRevisions = [2]uint64{m.player.Inventory.Revision, peer.player.Inventory.Revision}
	} else if !ok {
		return fmt.Errorf("trade closed")
	}
	i := slices.Index(v.Players[:], m.player.ID)
	if i < 0 {
		return fmt.Errorf("not a trade participant")
	}
	if r.Kind == "trade_cancel" {
		delete(t.trades, id)
		return nil
	}
	if r.Kind == "trade_offer" {
		if len(r.Items) > m.player.Inventory.Capacity {
			return fmt.Errorf("invalid offer")
		}
		seen := map[string]bool{}
		for _, it := range r.Items {
			if seen[it] || inventory.Equipped(m.player.Inventory, it) || m.player.Inventory.Cursor == it {
				return fmt.Errorf("invalid offer item")
			}
			if _, ok := inventory.Find(m.player.Inventory, it); !ok {
				return fmt.Errorf("item not owned")
			}
			seen[it] = true
		}
		v.Offers[i] = slices.Clone(r.Items)
		v.InventoryRevisions[i] = m.player.Inventory.Revision
		v.Accepted = [2]bool{}
		v.Revision++
	} else {
		if r.Revision != v.Revision {
			return fmt.Errorf("trade offer changed")
		}
		v.Accepted[i] = true
	}
	t.trades[id] = v
	if !v.Accepted[0] || !v.Accepted[1] {
		return nil
	}
	err := t.commitTrade(v)
	if err != nil {
		v.Accepted = [2]bool{}
		v.Revision++
		t.trades[id] = v
		return err
	}
	delete(t.trades, id)
	return nil
}
func (t *team) commitTrade(v protocol.Trade) error {
	// Group participants by owner; a scene is reserved once, even for two players.
	owners := map[string][]string{}
	values := map[string]model.Player{}
	for _, id := range v.Players {
		m := t.players[id]
		if m == nil || !m.player.Online {
			return fmt.Errorf("participant disconnected")
		}
		if a := t.actors[m.player.Scene]; a != nil {
			owners[m.player.Scene] = append(owners[m.player.Scene], id)
		} else {
			values[id] = model.ClonePlayer(m.player)
		}
	}
	var keys []string
	for key := range owners {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	reserved := []*scene.Actor{}
	defer func() {
		for _, a := range reserved {
			a.Abort()
		}
	}()
	for _, key := range keys {
		a := t.actors[key]
		players, err := a.Reserve(owners[key])
		if err != nil {
			return err
		}
		reserved = append(reserved, a)
		for _, p := range players {
			values[p.ID] = p
		}
	}
	a, b := values[v.Players[0]], values[v.Players[1]]
	if a.Inventory.Revision != v.InventoryRevisions[0] || b.Inventory.Revision != v.InventoryRevisions[1] {
		return fmt.Errorf("inventory changed; update both offers")
	}
	x, y, err := inventory.Exchange(a.Inventory, b.Inventory, v.Offers[0], v.Offers[1], t.catalog.Item)
	if err != nil {
		return err
	}
	a.Inventory = x
	b.Inventory = y
	values[a.ID] = a
	values[b.ID] = b
	for i, key := range keys {
		var players []model.Player
		for _, id := range owners[key] {
			players = append(players, values[id])
		}
		if err := reserved[i].Commit(players); err != nil {
			return err
		}
	}
	reserved = nil
	for _, p := range values {
		t.players[p.ID].player = p
	}
	return nil
}
func (t *team) tradeSnapshot() []protocol.Trade {
	var out []protocol.Trade
	for id, v := range t.trades {
		valid := true
		for _, p := range v.Players {
			if t.players[p] == nil || !t.players[p].player.Online {
				valid = false
			}
		}
		if !valid {
			delete(t.trades, id)
			continue
		}
		for i := 0; i < 2; i++ {
			v.Offers[i] = slices.Clone(v.Offers[i])
		}
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b protocol.Trade) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out
}
