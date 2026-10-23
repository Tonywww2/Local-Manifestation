package client

import (
	"time"

	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
)

// Map render checks may name a bag movement item by definition or instance.
// This changes only the display selection and never sends a travel command.
func (g *Game) smokeSelectMovement(id string) {
	if g.cfg.SmokeFrames <= 0 || id == "" {
		return
	}
	for _, it := range inventory.Bag(g.displayInventory()) {
		if it.ID == id || it.Definition == id {
			if d, ok := g.catalog.Item(it.Definition); ok && d.Movement != nil {
				g.performControl(uiControl{Action: "movement-select", Value: it.ID})
			}
			return
		}
	}
}

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
		if next := g.smokeNextNode(); next != "" {
			n, _ := g.mapNode(next)
			if n.Payment == "shared" || p.AP == 0 {
				g.command("propose", next, false)
			} else {
				g.command("enter", next, false)
			}
			g.smokeNextAction = time.Now().Add(time.Second)
		}
	}
}

func (g *Game) smokeNextNode() string {
	p := g.self()
	if p == nil {
		return ""
	}
	nodes := map[string]model.Node{}
	target := g.cfg.SmokeNode
	for _, n := range g.state.Nodes {
		nodes[n.ID] = n
		if target == "" && n.Kind == "combat" && !n.Completed {
			target = n.ID
		}
	}
	if target == "" || nodes[target].Completed || target == p.Node {
		return ""
	}
	previous := map[string]string{p.Node: ""}
	queue := []string{p.Node}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range nodes[id].Edges {
			if _, seen := previous[next]; seen || (next != target && !nodes[next].Completed && nodes[next].Kind != "passage") {
				continue
			}
			previous[next] = id
			queue = append(queue, next)
		}
	}
	if _, ok := previous[target]; !ok {
		return ""
	}
	next := target
	for previous[next] != p.Node {
		next = previous[next]
	}
	return next
}

func (g *Game) smokeInput(s protocol.Scene) model.Input {
	if s.State == "idle" {
		return model.Input{}
	}
	rate := g.state.TickRate
	if !timing.LogicRate(rate) {
		rate = 30
	}
	directions := []model.Vec{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	return model.Input{Move: directions[int(g.state.ServerTick/uint64(rate*2))%4], Aim: model.Vec{X: s.Width - 24, Y: s.Height - 24}, Attack: true}
}
