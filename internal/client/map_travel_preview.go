package client

import (
	"slices"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

type mapTravelTarget struct {
	Direct bool
	Vote   bool
}

type mapTravelPreview struct {
	State             *protocol.State
	Catalog           *content.Catalog
	Player, Item      string
	InventoryRevision uint64
	Targets           map[string]mapTravelTarget
	Ordinary          map[string]mapTravelTarget
}

type mapTravelContext struct {
	CanLeave             bool
	Player               string
	From                 model.Node
	PersonalAP, SharedAP int
	Tool                 *model.MovementItemDef
	ToolReady            bool
	Vote                 *protocol.Vote
	Closed               map[string]bool
	TeamOrigins          []string
}

// A display-only projection of routes, fees and admission windows. It neither
// deducts resources nor promises that a pending vote or server hook will succeed.
func mapTravelTargets(nodes []model.Node, routes map[string][]string, c mapTravelContext) map[string]mapTravelTarget {
	out := make(map[string]mapTravelTarget)
	if !c.CanLeave || c.From.ID == "" || c.Tool != nil && !c.ToolReady {
		return out
	}
	for _, n := range nodes {
		if c.Closed[n.ID] && !n.Completed {
			continue
		}
		voteOpen := c.Vote == nil || c.Vote.Node == n.ID && c.Vote.Item == "" && slices.Contains(c.Vote.Eligible, c.Player) && !slices.Contains(c.Vote.No, c.Player)
		var target mapTravelTarget
		if c.Tool != nil {
			team := c.Tool.Team
			if !expedition.MovementRange(c.From, n, *c.Tool) && !(team && n.ID == c.From.ID) {
				continue
			}
			if team {
				// The owner can remain in place while pulling other teammates.
				entrants := slices.ContainsFunc(c.TeamOrigins, func(id string) bool { return id != n.ID })
				target.Vote = entrants && c.SharedAP > 0 && c.Vote == nil
			} else if c.Tool.Free || c.PersonalAP > 0 {
				if n.Kind == "boss" {
					target.Vote = c.SharedAP > 0 && voteOpen
				} else {
					target.Direct = true
				}
			}
		} else {
			cost := len(routes[n.ID])
			target.Direct = !sharedEntry(n) && cost > 0 && c.PersonalAP >= cost
			target.Vote = c.SharedAP > 0 && voteOpen && expedition.CanMove(c.From, n)
		}
		if target.Direct || target.Vote {
			out[n.ID] = target
		}
	}
	return out
}

// Recompute only when a detached snapshot, content catalog, inventory projection
// or selected tool changes. Rendering consumes the resulting values in uiFrame.
func (g *Game) mapDestinations() map[string]mapTravelTarget {
	p := g.self()
	if p == nil {
		return nil
	}
	revision := g.displayInventory().Revision
	cache := &g.u.travelPreview
	if cache.State == g.state && cache.Catalog == g.catalog && cache.Player == p.ID && cache.Item == g.u.movementItem && cache.InventoryRevision == revision {
		return cache.Targets
	}
	c := mapTravelContext{CanLeave: p.Active && p.Online && p.Life == "alive" && p.Scene == "" && g.state.Phase == "exploring", Player: p.ID,
		From: g.currentNode(), PersonalAP: p.AP, SharedAP: g.state.SharedAP, Vote: g.state.Vote, Closed: make(map[string]bool)}
	if it, d, ok := g.selectedMovementItem(); ok {
		c.Tool, c.ToolReady = d.Movement, it.Durability > 0 || it.Durability == -1
	}
	paused := make(map[string]bool)
	for _, s := range g.state.Scenes {
		// Match scene.admitGroup: paused rooms permit rescue, fighting rooms
		// are locked, and between-wave admission follows the room definition.
		c.Closed[s.ID] = s.State == "fighting" || s.State == "cleared" || s.State == "faulted" || s.State == "between_waves" && g.catalog.RoomJoin(s.Room) != "between_waves"
		paused[s.ID] = s.State == "paused"
	}
	if c.Tool != nil && c.Tool.Team {
		for _, peer := range g.state.Players {
			if peer.Active && peer.Life != "dead" && (peer.Scene == "" || peer.Life == "down" && paused[peer.Scene]) {
				c.TeamOrigins = append(c.TeamOrigins, peer.Node)
			}
		}
	}
	g.personalRoute("") // Populate the shared shortest-route cache once.
	targets := mapTravelTargets(g.state.Nodes, g.u.routes, c)
	ordinary := targets
	if c.Tool != nil {
		c.Tool = nil
		ordinary = mapTravelTargets(g.state.Nodes, g.u.routes, c)
	}
	*cache = mapTravelPreview{State: g.state, Catalog: g.catalog, Player: p.ID, Item: g.u.movementItem, InventoryRevision: revision, Targets: targets, Ordinary: ordinary}
	return targets
}
