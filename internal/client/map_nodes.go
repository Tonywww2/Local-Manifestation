package client

import (
	"fmt"
	"image/color"
	"slices"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func terrainID(t model.Terrain) string { return fmt.Sprintf("terrain:%d:%d", t.Q, t.R) }

func (g *Game) nodesVoteKind() string {
	if g.state != nil && g.state.Vote != nil {
		n, _ := g.mapNode(g.state.Vote.Node)
		return n.Kind
	}
	return ""
}

func (g *Game) visibleMapNodes() map[string]bool {
	if g.state == nil {
		return nil
	}
	from := g.currentNode().ID
	if g.u.visibilityState != g.state || g.u.visibilityFrom != from {
		g.u.visibleNodes = expedition.Visibility(g.state.Nodes, from)
		g.u.visibilityState, g.u.visibilityFrom = g.state, from
	}
	return g.u.visibleNodes
}

func mapNodeStyle(n model.Node) (color.RGBA, string) {
	switch n.Kind {
	case "combat":
		return color.RGBA{75, 48, 100, 255}, "node-combat"
	case "boss":
		return color.RGBA{125, 47, 57, 255}, "node-boss"
	case "shop":
		return color.RGBA{45, 101, 77, 255}, "node-shop"
	case "passage":
		return color.RGBA{74, 80, 85, 255}, "node-passage"
	case "start":
		return color.RGBA{50, 91, 113, 255}, "node-start"
	case "revival":
		return color.RGBA{108, 94, 63, 255}, "plus"
	case "shrine":
		return color.RGBA{69, 111, 111, 255}, "shield"
	default:
		if n.Room != "" {
			return color.RGBA{75, 48, 100, 255}, "node-combat"
		}
		return color.RGBA{101, 76, 49, 255}, "node-event"
	}
}

func (g *Game) mapNode(id string) (model.Node, bool) {
	if g.state != nil {
		for _, n := range g.state.Nodes {
			if n.ID == id {
				return n, true
			}
		}
	}
	return model.Node{}, false
}

func (g *Game) mapNodeLabel(n model.Node) string {
	if !g.visibleMapNodes()[n.ID] {
		return g.tr("ui.map.unknown", "type", g.nodeType(n))
	}
	return g.nodeName(n.ID)
}

func (g *Game) nodeType(n model.Node) string {
	kind := n.Kind
	if kind == "boss" {
		return g.tr("ui.node_type.boss")
	}
	if kind == "shop" {
		return g.tr("ui.node_type.shop")
	}
	if n.Room != "" {
		kind = "combat"
	}
	switch kind {
	case "combat", "shop", "passage", "start", "shrine", "revival":
		return g.tr("ui.node_type." + kind)
	default:
		return g.tr("ui.node_type.event")
	}
}

func (g *Game) canMapMove(n model.Node) bool {
	target := g.mapDestinations()[n.ID]
	return target.Direct || target.Vote && (g.u.movementItem != "" || sharedEntry(n))
}

func (g *Game) personalRoute(to string) []string {
	if g.state == nil {
		return nil
	}
	from := g.currentNode().ID
	if g.u.routeState != g.state || g.u.routeFrom != from {
		g.u.routes = expedition.Routes(g.state.Nodes, from)
		g.u.routeState, g.u.routeFrom = g.state, from
	}
	return g.u.routes[to]
}

func (g *Game) canTeamMapMove(n model.Node) bool {
	return g.mapDestinations()[n.ID].Vote
}

func sharedEntry(n model.Node) bool {
	return n.Kind == "boss" || n.Payment == "shared" && n.Kind != "shop"
}

func (g *Game) nodeHover(n model.Node) []string {
	lines := []string{g.tr("ui.map.node_type", "type", g.nodeType(n))}
	target := g.mapDestinations()[n.ID]
	if target.Direct {
		lines = append(lines, g.tr("ui.map.reachable_direct"))
	} else if target.Vote {
		lines = append(lines, g.tr("ui.map.reachable_vote"))
	}
	if !g.visibleMapNodes()[n.ID] {
		return lines
	}
	if _, d, ok := g.selectedMovementItem(); ok {
		if d.Movement.Team {
			lines = append(lines, g.tr("ui.movement.team_cost"))
		} else {
			cost := 1
			if d.Movement.Free {
				cost = 0
			}
			lines = append(lines, g.tr("ui.movement.cost", "cost", cost))
		}
		if n.Kind == "boss" && !d.Movement.Team {
			lines = append(lines, g.tr("ui.movement.boss_cost"))
		}
	} else {
		if sharedEntry(n) {
			lines = append(lines, g.tr("ui.map.shared_cost"))
		} else {
			lines = append(lines, g.tr("ui.map.personal_cost", "cost", max(1, len(g.personalRoute(n.ID)))))
		}
	}
	if n.Blocking && !n.Completed {
		lines = append(lines, g.tr("ui.map.blocking"))
	} else {
		lines = append(lines, g.tr("ui.map.transit"))
	}
	if g.currentNode().ID == n.ID || len(g.personalRoute(n.ID)) > 0 {
		if hint := g.localized(n.HintKey, n.Hint); hint != "" {
			lines = append(lines, hint)
		}
	}
	if n.ID != g.currentNode().ID && !target.Direct && !target.Vote {
		if p := g.self(); p != nil && p.Scene != "" {
			lines = append(lines, g.tr("ui.map.combat_locked"))
		} else {
			lines = append(lines, g.tr("ui.map.unavailable"))
		}
	}
	return lines
}

func (g *Game) confirmMapEntry(id string, shared bool) {
	n, ok := g.mapNode(id)
	_, d, tool := g.selectedMovementItem()
	if tool {
		shared = d.Movement.Team || n.Kind == "boss"
	} else {
		shared = shared || sharedEntry(n)
	}
	canMove := g.canMapMove(n)
	if shared {
		canMove = g.canTeamMapMove(n)
	}
	if !ok || !canMove {
		g.notify("error.unreachable")
		return
	}
	if shared && g.state.Vote != nil && g.state.Vote.Node == id && n.Kind != "boss" {
		g.closeWindow("node-actions")
		g.openWindow("vote")
		return
	}
	g.u.selectedNode, g.u.entrance, g.u.entryShared = id, id, shared
	g.closeWindow("node-actions")
	g.openWindow("entrance")
}

func (g *Game) mapNodeMenu(id string, cursor ui.Point) {
	g.u.selectedNode = id
	g.u.layout["node-actions"] = ui.Placement{Anchor: "top_left", X: cursor.X, Y: cursor.Y}
	g.openWindow("node-actions")
}

func (g *Game) addMinimapNodeControls(f *uiFrame, r ui.Rect) {
	bounds := minimapContentBounds(r)
	positions, radius := g.mapLayout(bounds, "minimap")
	for _, n := range g.state.Nodes {
		v := positions[n.ID]
		c := uiControl{ID: "map-node:mini:" + n.ID, Owner: "minimap", Action: "window", Value: "map", Title: g.mapNodeLabel(n),
			Bounds: ui.Rect{X: v.X - radius, Y: v.Y - radius, W: radius * 2, H: radius * 2}, Clip: bounds, Hex: true, HexAngle: g.cameraAngle()}
		if controlContains(c, g.u.cursor) {
			c.Summary = g.nodeHover(n)
		}
		f.Controls = append(f.Controls, c)
	}
}

func (g *Game) buildMapActionWindow(f *uiFrame, layout ui.Layout, id string) bool {
	if id == "vote" {
		g.buildMapVote(f, layout)
		return true
	}
	if id != "entrance" && id != "node-actions" {
		return false
	}
	target := g.u.selectedNode
	if id == "entrance" {
		target = g.u.entrance
	}
	n, ok := g.mapNode(target)
	if !ok {
		return true
	}
	r := g.windowPanel(f, layout, id, g.mapNodeLabel(n), ui.Size{W: 440, H: 245})
	lines := g.nodeHover(n)
	if id == "entrance" && g.u.entryShared {
		lines = append([]string{g.tr("ui.map.team_confirm")}, lines...)
	}
	g.addWindowLines(f, id, r, lines, r.H-100)
	y := r.Y + r.H - 44
	if id == "entrance" {
		title := g.tr("ui.map.enter_personal")
		if g.u.entryShared {
			title = g.tr("ui.map.propose")
		}
		if _, _, tool := g.selectedMovementItem(); tool {
			title = g.tr("ui.movement.use")
		}
		if n.Kind == "boss" {
			title = g.tr("ui.map.boss_join")
		}
		addControl(f, id, "map:confirm", "approve", "", "enter-confirm", target, title, ui.Rect{X: r.X + 14, Y: y, W: 42, H: 32})
		addControl(f, id, "map:cancel", "cancel", "", "close", id, g.tr("ui.cancel"), ui.Rect{X: r.X + 64, Y: y, W: 42, H: 32})
	} else {
		choices := []string{"map-personal", "map-team"}
		if _, _, tool := g.selectedMovementItem(); tool {
			choices = choices[:1]
		}
		if g.state.Vote != nil && g.state.Vote.Node == n.ID && n.Kind == "boss" && g.state.Vote.Item == "" && slices.Contains(g.state.Vote.Yes, g.id) {
			choices = []string{"map-team", "map-withdraw"}
		}
		for i, action := range choices {
			title, icon := g.tr("ui.map.enter_personal"), "door"
			if action == "map-team" {
				title, icon = g.tr("ui.map.propose"), "person"
			}
			if action == "map-withdraw" {
				title, icon = g.tr("ui.map.withdraw"), "cancel"
			}
			_, _, tool := g.selectedMovementItem()
			if tool && action != "map-withdraw" {
				title, icon = g.tr("ui.movement.use"), "movement"
			}
			if action == "map-personal" && (!g.canMapMove(n) || sharedEntry(n) && !tool) || action == "map-team" && !g.canTeamMapMove(n) {
				action = ""
			}
			x := r.X + 14 + float64(i)*210
			addControl(f, id, "map:choice:"+icon, icon, "", action, target, title, ui.Rect{X: x, Y: y, W: 36, H: 32})
			g.addLabel(f, id, ui.Rect{X: x + 42, Y: y + 5, W: 155, H: 28}, title)
		}
	}
	return true
}

func (g *Game) buildMapVote(f *uiFrame, layout ui.Layout) {
	if g.state == nil || g.state.Vote == nil {
		return
	}
	v := g.state.Vote
	r := g.windowPanel(f, layout, "vote", g.tr("ui.vote.title"), ui.Size{W: 460, H: 260})
	n, _ := g.mapNode(v.Node)
	lines := []string{g.mapNodeLabel(n), g.tr("ui.map.vote_count", "yes", len(v.Yes), "required", v.Required), g.tr("ui.map.vote_help"), g.tr("ui.map.entrants", "count", len(v.Entrants))}
	actions := []string{"vote-enter", "vote-only", "vote-no"}
	if n.Kind == "boss" && v.Item == "" {
		lines = lines[:2]
		lines = append(lines, g.tr("ui.map.boss_vote_help"))
		lines = append(lines, g.tr("ui.map.vote_map"))
		actions = []string{"vote-no"}
	} else if v.Item != "" {
		lines = append(lines[:2], g.tr("ui.movement.team_cost"), g.tr("ui.movement.team_help"))
		actions = []string{"vote-only", "vote-no"}
	}
	if slices.Contains(v.Yes, g.id) || slices.Contains(v.No, g.id) {
		lines = append(lines, g.tr("ui.map.voted"))
	} else {
		for i, action := range actions {
			title := g.tr("ui.map." + action)
			if n.Kind == "boss" && action == "vote-only" {
				title = g.tr("ui.approve")
			}
			// Joining someone else's ordinary vote uses adjacency and shared
			// fees, regardless of the movement tool currently selected in the map.
			g.mapDestinations()
			if action == "vote-enter" && !g.u.travelPreview.Ordinary[n.ID].Vote {
				action = ""
			}
			x := r.X + 14 + float64(i)*145
			icon := "approve"
			if action == "vote-no" {
				icon = "cancel"
			}
			addControl(f, "vote", "vote:choice:"+title, icon, "", action, "", title, ui.Rect{X: x, Y: r.Y + r.H - 54, W: 34, H: 32})
			g.addLabel(f, "vote", ui.Rect{X: x + 40, Y: r.Y + r.H - 51, W: 100, H: 36}, title)
		}
	}
	g.addWindowLines(f, "vote", r, lines, r.H-110)
}

func (g *Game) submitMapVote(accept, join bool) {
	if g.link != nil {
		g.link.Send(protocol.Request{Kind: "vote", Accept: accept, Join: join})
	}
}
