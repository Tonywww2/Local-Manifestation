package client

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"image/color"
)

func (g *Game) walkingSides() []int {
	if g.state == nil || g.state.Phase != "exploring" {
		return nil
	}
	p := g.self()
	if p == nil || p.Scene != "" {
		return nil
	}
	from := g.currentNode()
	var sides []int
	for _, id := range from.Edges {
		for _, to := range g.state.Nodes {
			if to.ID == id && expedition.FreeTransit(to) {
				if side, ok := geometry.DoorSide(from, to); ok {
					sides = append(sides, side)
				}
			}
		}
	}
	return sides
}

func (g *Game) nearbyDoor() (model.Node, bool) {
	p := g.self()
	if p == nil || p.Scene != "" || p.Life != "alive" || g.state.Phase != "exploring" {
		return model.Node{}, false
	}
	var from model.Node
	for _, n := range g.state.Nodes {
		if n.ID == p.Node {
			from = n
			break
		}
	}
	w, h := g.catalog.SpaceSize(from)
	a := geometry.NewArena(w, h)
	for _, id := range from.Edges {
		for _, to := range g.state.Nodes {
			if to.ID != id {
				continue
			}
			if side, ok := geometry.DoorSide(from, to); ok && a.NearDoor(p.Position, side) {
				return to, true
			}
		}
	}
	return model.Node{}, false
}

func (g *Game) doorSummary(n model.Node) string {
	for _, s := range g.state.Scenes {
		if s.ID == n.ID && (s.State == "fighting" || s.State == "faulted") {
			return g.tr("ui.door.locked")
		}
	}
	if n.Completed || n.Payment == "none" {
		return g.tr("ui.door.free")
	}
	if n.Payment == "shared" {
		if n.Open {
			return g.tr("ui.door.open")
		}
		return g.tr("ui.door.shared", "cost", n.Cost)
	}
	return g.tr("ui.door.personal", "cost", n.Cost)
}

func (g *Game) doorName(n model.Node) string {
	p := g.self()
	if n.Kind == "passage" && p != nil {
		for _, id := range n.Edges {
			if id != p.Node {
				return g.nodeName(n.ID) + " · " + g.nodeName(id)
			}
		}
	}
	return g.nodeName(n.ID)
}

func (g *Game) drawDoorsForNode(dst *ebiten.Image, from model.Node, s protocol.Scene, c Camera) {
	p := g.self()
	if p == nil {
		return
	}
	a := geometry.NewArena(s.Width, s.Height)
	var strokes []geometry.Segment
	var indices []uint32
	var vertices []ebiten.Vertex
	for _, id := range from.Edges {
		for _, to := range g.state.Nodes {
			if to.ID != id {
				continue
			}
			side, ok := geometry.DoorSide(from, to)
			if !ok {
				continue
			}
			col := accent
			if !to.Completed && to.Payment != "none" {
				col = color.RGBA{235, 195, 110, 255}
			}
			if p.Scene != "" && p.Node == from.ID {
				col = muted
			}
			for _, scene := range g.state.Scenes {
				if scene.ID == to.ID && scene.State == "fighting" {
					col = color.RGBA{225, 100, 110, 255}
				}
			}
			point := a.DoorPoint(side, 0)
			direction := point.Sub(a.Center).Unit()
			tangent := model.Vec{X: -direction.Y, Y: direction.X}.Scale(28)
			mesh := strokeMesh([]geometry.Segment{{A: point.Sub(tangent), B: point.Add(tangent)}}, c, 5, col)
			base := uint32(len(vertices))
			vertices = append(vertices, mesh.Vertices...)
			for _, i := range mesh.Indices {
				indices = append(indices, base+i)
			}
			strokes = append(strokes, geometry.Segment{A: point, B: a.DoorPoint(side, 36)})
		}
	}
	if len(indices) > 0 {
		dst.DrawTriangles32(vertices, indices, whiteImage, nil)
	}
	mesh := strokeMesh(strokes, c, 2, muted)
	if len(mesh.Indices) > 0 {
		dst.DrawTriangles32(mesh.Vertices, mesh.Indices, whiteImage, nil)
	}
}

func terrainID(t model.Terrain) string { return fmt.Sprintf("terrain:%d:%d", t.Q, t.R) }
