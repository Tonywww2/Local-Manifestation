package server

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"testing"
	"time"
)

// Traverse authored connections through the map protocol, paying per movement.
func travel(t *testing.T, peers []*testPeer, targets map[string]string, enter bool) {
	t.Helper()
	pending := map[string]string{}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		done := true
		for _, p := range peers {
			select {
			case e := <-p.errors:
				t.Fatal(e)
			default:
			}
			target, needed := targets[p.id]
			if !needed {
				continue
			}
			s := p.state.Load()
			if s == nil {
				done = false
				continue
			}
			pl := findPlayer(s, p.id)
			if pl.Node == target {
				continue
			}
			nodes := map[string]model.Node{}
			for _, n := range s.Nodes {
				nodes[n.ID] = n
			}
			previous := map[string]string{pl.Node: ""}
			queue := []string{pl.Node}
			for len(queue) > 0 {
				id := queue[0]
				queue = queue[1:]
				for _, next := range nodes[id].Edges {
					if _, seen := previous[next]; seen {
						continue
					}
					if next != target && !nodes[next].Completed {
						continue
					}
					previous[next] = id
					queue = append(queue, next)
				}
			}
			if _, ok := previous[target]; !ok {
				t.Fatalf("no cleared route %s -> %s", pl.Node, target)
			}
			next := target
			for previous[next] != pl.Node {
				next = previous[next]
			}
			if !enter && next == target {
				continue // Stop in an adjacent room, ready for a map confirmation.
			}
			done = false
			if pending[p.id] == pl.Node {
				continue
			}
			p.send(t, protocol.Request{Kind: "input"})
			p.send(t, protocol.Request{Kind: "enter", Node: next})
			pending[p.id] = pl.Node
		}
		if done {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("map travel timed out")
}

func walkToDevice(t *testing.T, c *content.Catalog, peers []*testPeer, exit bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		done := true
		for _, p := range peers {
			select {
			case e := <-p.errors:
				t.Fatal(e)
			default:
			}
			s := p.state.Load()
			if s == nil {
				done = false
				continue
			}
			pl := findPlayer(s, p.id)
			var n model.Node
			for _, node := range s.Nodes {
				if node.ID == pl.Node {
					n = node
				}
			}
			w, h := c.SpaceSize(n)
			for _, sc := range s.Scenes {
				if sc.ID == pl.Scene {
					w, h = sc.Width, sc.Height
				}
			}
			a := geometry.NewArena(w, h)
			point := a.EmergencyDevice()
			if exit {
				point = a.ExitDevice()
			}
			if pl.Position.Sub(point).Length() <= 30 {
				p.send(t, protocol.Request{Kind: "input"})
				continue
			}
			done = false
			p.send(t, protocol.Request{Kind: "input", Input: model.Input{Move: point.Sub(pl.Position).Unit()}})
		}
		if done {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("device approach timed out")
}
