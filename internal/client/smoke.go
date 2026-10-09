package client

import (
	"time"

	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
)

// Only explicit smoke mode exercises these operations.
func (g *Game) smokeActions() {
	if !g.cfg.SmokeAutoplay || g.cfg.SmokeFrames <= 0 || g.link == nil || g.state == nil || time.Now().Before(g.smokeNextAction) {
		return
	}
	p := g.self()
	if p == nil {
		return
	}
	if (g.state.Phase == "lobby" || g.state.Phase == "preparation") && g.state.Admin == g.id {
		g.command("start", "", false)
		g.smokeNextAction = time.Now().Add(time.Second)
		return
	}
	if g.state.Phase == "exploring" && p.Scene == "" {
		if door, ok := g.nearbyDoor(); ok && door.Kind == "combat" && !door.Completed && (g.cfg.SmokeNode == "" || door.ID == g.cfg.SmokeNode) {
			g.command("enter", door.ID, false)
			g.smokeNextAction = time.Now().Add(time.Second)
		}
	}
}

func (g *Game) smokeRoomGoal() (model.Vec, bool) {
	p := g.self()
	if p == nil || p.Scene != "" {
		return model.Vec{}, false
	}
	nodes := map[string]model.Node{}
	target := g.cfg.SmokeNode
	for _, n := range g.state.Nodes {
		nodes[n.ID] = n
		if target == "" && n.Kind == "combat" && !n.Completed {
			target = n.ID
		}
	}
	if target == "" || nodes[target].Completed {
		return model.Vec{}, false
	}
	previous := map[string]string{p.Node: ""}
	queue := []string{p.Node}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range nodes[id].Edges {
			if _, seen := previous[next]; seen || (next != target && !nodes[next].Completed) {
				continue
			}
			previous[next] = id
			queue = append(queue, next)
		}
	}
	if _, ok := previous[target]; !ok || target == p.Node {
		return model.Vec{}, false
	}
	next := target
	for previous[next] != p.Node {
		next = previous[next]
	}
	side, ok := geometry.DoorSide(nodes[p.Node], nodes[next])
	if !ok {
		return model.Vec{}, false
	}
	w, h := g.catalog.SpaceSize(nodes[p.Node])
	inset := -8.0
	if next == target {
		inset = 24
	}
	return geometry.NewArena(w, h).DoorPoint(side, inset), true
}

func (g *Game) smokeInput(s protocol.Scene) model.Input {
	if s.State == "idle" {
		if point, ok := g.smokeRoomGoal(); ok {
			return model.Input{Move: point.Sub(g.self().Position).Unit()}
		}
		return model.Input{}
	}
	rate := g.state.TickRate
	if !timing.LogicRate(rate) {
		rate = 30
	}
	directions := []model.Vec{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	return model.Input{Move: directions[int(g.state.ServerTick/uint64(rate*2))%4], Aim: model.Vec{X: s.Width - 24, Y: s.Height - 24}, Attack: true}
}
