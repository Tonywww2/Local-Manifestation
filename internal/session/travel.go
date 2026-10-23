package session

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/sdk"
)

func (t *team) reachable(from, to string) bool {
	return expedition.CanMove(t.nodes[from], t.nodes[to])
}

func (t *team) personalRoute(from, to string) []string {
	nodes := make([]model.Node, 0, len(t.nodes))
	for _, n := range t.nodes {
		nodes = append(nodes, n)
	}
	return expedition.Routes(nodes, from)[to]
}

func (t *team) enter(m *member, id string) error {
	if id == t.bossNode() {
		return t.selectBoss(m, "")
	}
	return t.enterGroup([]*member{m}, id, false)
}

// Stage all player and hook results, then admit the whole group on the scene
// owner. Only successful admission commits the entire route cost (or one shared fee).
func (t *team) enterGroup(members []*member, id string, shared bool) error {
	return t.arrive(members, id, arrivalMode{shared: shared})
}

type arrivalMode struct {
	shared      bool
	owner, item string
}

func (t *team) arrive(members []*member, id string, mode arrivalMode) error {
	shared := mode.shared
	usingTool := mode.item != ""
	n, ok := t.nodes[id]
	if !ok {
		return protocol.Reject("error.unreachable")
	}
	if len(members) == 0 {
		return protocol.Reject("error.no_entrants")
	}
	if n.Payment == "shared" && n.Kind != "shop" && !shared && !usingTool {
		return protocol.Reject("error.node_vote")
	}
	if shared && t.shared < 1 {
		return protocol.Reject("error.no_shared_ap")
	}
	prepared := make([]model.Player, len(members))
	costs := make([]int, len(members))
	var tool model.ItemDef
	if usingTool {
		owner := t.players[mode.owner]
		var err error
		tool, err = t.movementTool(owner, mode.item, id)
		if err != nil {
			return err
		}
		if tool.Movement.Team != shared {
			return protocol.Reject("error.movement_item")
		}
	}
	// Only downed players in paused rooms can be extracted. Reserve each owner
	// once, then finish all destination and resource checks before any commit.
	reserved, sourcePlayers, err := t.reserveTravelSources(members, usingTool && shared)
	if err != nil {
		return err
	}
	defer func() {
		for _, a := range reserved {
			a.Abort()
		}
	}()
	for i, m := range members {
		if m == nil || !m.player.Active || m.player.Life != "alive" && !(usingTool && shared && m.player.Life == "down") {
			return protocol.Reject("error.incapacitated")
		}
		p := m.player
		if v, ok := sourcePlayers[p.ID]; ok {
			p = v
			p.Scene = ""
		}
		if p.Scene != "" {
			return protocol.Reject("error.already_battle")
		}
		if shared && !usingTool && !t.reachable(p.Node, id) {
			return protocol.Reject("error.unreachable")
		}
		costs[i] = 1
		if !shared && !usingTool {
			costs[i] = len(t.personalRoute(m.player.Node, id))
			if costs[i] == 0 {
				return protocol.Reject("error.unreachable")
			}
		}
		if usingTool && !shared && tool.Movement.Free {
			costs[i] = 0
		}
		if !shared && p.AP < costs[i] {
			return protocol.Reject("error.no_personal_ap")
		}
		entry := *m
		var err error
		entry.player, err = content.LeaveStatuses(p)
		if err != nil {
			return err
		}
		first := !m.visited[id]
		player := entry.player
		if !n.Completed || n.Room == "" {
			player, err = t.interact(&entry, n, "enter", first)
			if err != nil {
				return err
			}
		}
		player.Node = id
		player.Position = t.spawnAt(id, player.ID)
		if !shared {
			player.AP -= costs[i]
		}
		if n.Room != "" && !n.Completed {
			player.Scene = id
		}
		if usingTool {
			if player.ID == mode.owner {
				player.Inventory, err = inventory.SpendDurability(player.Inventory, mode.item, t.catalog.Item)
				if err != nil {
					return err
				}
			}
			player, err = t.catalog.MovementPassives(player, t.floor, t.config.Seed, n.Kind == "combat" || n.Kind == "boss" || n.Room != "")
			if err != nil {
				return err
			}
		}
		if first {
			commands, err := t.runtime.Dispatch(sdk.EventContext{Scope: "session", Scene: player.Scene, Floor: t.floor, Event: model.Event{Kind: "node_entered", Scene: player.Scene, Source: player.ID, Target: n.ID}, Players: []model.Player{player}})
			if err != nil {
				return err
			}
			next, err := content.ApplyCommands(t.catalog, map[string]model.Player{player.ID: player}, commands, content.EventCommands)
			if err != nil {
				return err
			}
			player = next[player.ID]
		}
		prepared[i] = player
	}
	// The initiator may already be at the destination while pulling teammates.
	var stationaryOwner *model.Player
	if usingTool {
		found := false
		for _, p := range prepared {
			found = found || p.ID == mode.owner
		}
		if !found {
			p := t.players[mode.owner].player
			p.Inventory, err = inventory.SpendDurability(p.Inventory, mode.item, t.catalog.Item)
			if err != nil {
				return err
			}
			stationaryOwner = &p
		}
	}
	var actor *scene.Actor
	if n.Room != "" && !n.Completed {
		actor = t.actors[id]
		created := actor == nil
		if created {
			var err error
			actor, err = scene.New(t.catalog, scene.Config{ShotOrigin: t.projectileOrigin(id), ExternalClock: true, AdventureTime: t.adventureTime, TickRate: t.config.TickRate, ID: id, Room: n.Room, Floor: t.floor, Seed: content.Seed(t.config.Seed, t.floor, id+":"+n.Room)})
			if err != nil {
				return err
			}
		}
		entries := make([]scene.Admission, len(members))
		for i, m := range members {
			entries[i] = scene.Admission{Player: prepared[i], Actions: m.actions}
		}
		if err := actor.AddGroup(entries); err != nil {
			if created {
				actor.Stop()
			}
			return err
		}
	}
	for source, a := range reserved {
		var departing []model.Player
		for i, m := range members {
			if m.player.Scene == source {
				departing = append(departing, prepared[i])
			}
		}
		if err := a.Commit(departing); err != nil {
			return err
		}
		delete(t.views, source)
	}
	reserved = nil
	if stationaryOwner != nil {
		t.players[stationaryOwner.ID].player = *stationaryOwner
	}
	if shared {
		t.shared--
	}
	if actor != nil {
		t.actors[id] = actor
		if space := t.spaces[id]; space != nil {
			actor.ReceiveProjectiles(space.ReleaseProjectiles())
			delete(t.spaces, id)
		}
	} else {
		n.Completed = true
	}
	n.Open = true
	t.nodes[id] = n
	for i, m := range members {
		if m.visited == nil {
			m.visited = map[string]bool{}
		}
		m.visited[id] = true
		m.player, m.walk = prepared[i], model.Input{}
		if actor != nil {
			m.actions = scene.PlayerActions{}
		} else {
			t.placeInSpace(m, id)
		}
	}
	if actor != nil {
		t.merge(actor.Snapshot())
	}
	return nil
}
