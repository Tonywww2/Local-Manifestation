package session

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"math"
	"slices"
)

func (t *team) bossTravelReady(m *member, item string) error {
	if m == nil || !m.player.Active || m.player.Life != "alive" {
		return protocol.Reject("error.incapacitated")
	}
	if m.player.Scene != "" {
		return protocol.Reject("error.outside_combat")
	}
	if item == "" {
		if !t.reachable(m.player.Node, t.bossNode()) {
			return protocol.Reject("error.boss_route")
		}
		return nil
	}
	d, err := t.movementTool(m, item, t.bossNode())
	if err != nil {
		return err
	}
	if d.Movement.Team {
		return protocol.Reject("error.movement_item")
	}
	if !d.Movement.Free && m.player.AP < 1 {
		return protocol.Reject("error.no_personal_ap")
	}
	return nil
}

// BOSS votes are travel commitments. A distant teammate must supply their own
// valid movement tool; no generic approval silently bypasses that travel check.
func (t *team) selectBoss(m *member, item string) error {
	if t.bossRoute != "" {
		return protocol.Reject("error.boss_committed")
	}
	if t.shared < 1 {
		return protocol.Reject("error.no_shared_ap")
	}
	if err := t.bossTravelReady(m, item); err != nil {
		return err
	}
	if t.vote != nil && (t.vote.Node != t.bossNode() || t.vote.Item != "") {
		return protocol.Reject("error.vote_open")
	}
	if t.vote == nil {
		v := &protocol.Vote{Node: t.bossNode(), Tools: map[string]string{}}
		for _, id := range t.ids() {
			p := t.players[id].player
			if p.Active && p.Online && p.Life == "alive" {
				v.Eligible = append(v.Eligible, id)
			}
		}
		t.vote = v
	}
	v := t.vote
	v.Eligible = slices.DeleteFunc(v.Eligible, func(id string) bool {
		p := t.players[id]
		return p == nil || !p.player.Active || !p.player.Online || p.player.Life != "alive"
	})
	v.Yes = slices.DeleteFunc(v.Yes, func(id string) bool { return !slices.Contains(v.Eligible, id) })
	v.No = slices.DeleteFunc(v.No, func(id string) bool { return !slices.Contains(v.Eligible, id) })
	v.Required = max(1, int(math.Ceil(float64(len(v.Eligible))*t.config.VoteRatio)))
	if !slices.Contains(v.Eligible, m.player.ID) {
		return protocol.Reject("error.not_eligible")
	}
	if slices.Contains(v.No, m.player.ID) {
		return protocol.Reject("error.already_voted")
	}
	if !slices.Contains(v.Yes, m.player.ID) {
		v.Yes = append(v.Yes, m.player.ID)
		v.Entrants = append(v.Entrants, m.player.ID)
	}
	v.Tools[m.player.ID] = item
	if len(v.Yes) < v.Required {
		return nil
	}
	// Keep the configured voting threshold. Everyone pulled into the mandatory
	// BOSS must still have a route: adjacent, or a committed valid personal tool.
	for _, id := range v.Eligible {
		if err := t.bossTravelReady(t.players[id], v.Tools[id]); err != nil {
			return nil
		}
	}
	t.vote = nil
	if err := t.normalBossReady(); err != nil {
		return err
	}
	return t.launchNormalBoss(*v)
}

func (t *team) startToolBoss(v protocol.Vote) error {
	// Ongoing combat participants are never pulled into a normal BOSS by a tool.
	if err := t.normalBossReady(); err != nil {
		return err
	}
	return t.launchNormalBoss(v)
}

func (t *team) launchNormalBoss(v protocol.Vote) error {
	if t.shared < 1 {
		return protocol.Reject("error.no_shared_ap")
	}
	id := t.bossNode()
	var entries []scene.Admission
	var members []*member
	for _, pid := range t.ids() {
		m := t.players[pid]
		if !m.player.Active || m.player.Life == "dead" {
			continue
		}
		if m.player.Life == "alive" && v.Item == "" {
			if err := t.bossTravelReady(m, v.Tools[pid]); err != nil {
				return err
			}
		}
		p, err := content.LeaveStatuses(m.player)
		if err != nil {
			return err
		}
		p.Node, p.Scene = id, id
		p.Position = t.spawnAt(id, p.ID)
		tool := v.Tools[pid]
		if m.player.Life != "alive" {
			tool = ""
		}
		if v.Item != "" && pid == v.Owner {
			tool = v.Item
		}
		if tool != "" {
			it, _ := inventory.Find(p.Inventory, tool)
			d, _ := t.catalog.Item(it.Definition)
			p.Inventory, err = inventory.SpendDurability(p.Inventory, tool, t.catalog.Item)
			if err != nil {
				return err
			}
			if !d.Movement.Team && !d.Movement.Free {
				p.AP--
			}
		}
		if tool != "" || v.Item != "" {
			p, err = t.catalog.MovementPassives(p, t.floor, t.config.Seed, true)
			if err != nil {
				return err
			}
		}
		entries = append(entries, scene.Admission{Player: p, Actions: m.actions})
		members = append(members, m)
	}
	a, err := scene.New(t.catalog, scene.Config{ShotOrigin: t.projectileOrigin(id), ExternalClock: true, AdventureTime: t.adventureTime, TickRate: t.config.TickRate, ID: id, Room: t.nodes[id].Room, Floor: t.floor, Seed: content.Seed(t.config.Seed, t.floor, id+":"+t.nodes[id].Room)})
	if err != nil {
		return err
	}
	if err = a.AddGroup(entries); err != nil {
		a.Stop()
		return err
	}
	// Only paused rooms remain here. Preserve earned rewards before settling them.
	t.stopScenes()
	t.shared--
	t.bossRoute = "normal"
	t.actors[id] = a
	if space := t.spaces[id]; space != nil {
		a.ReceiveProjectiles(space.ReleaseProjectiles())
		delete(t.spaces, id)
	}
	n := t.nodes[id]
	n.Open = true
	t.nodes[id] = n
	for i, m := range members {
		m.player = entries[i].Player
		m.actions = scene.PlayerActions{}
		m.walk = model.Input{}
		if m.visited == nil {
			m.visited = map[string]bool{}
		}
		m.visited[id] = true
	}
	t.merge(a.Snapshot())
	return nil
}
