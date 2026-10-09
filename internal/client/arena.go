package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"image/color"
)

type arenaMesh struct {
	Vertices []ebiten.Vertex
	Indices  []uint32
}
type arenaDrawing struct {
	Width, Height       float64
	Camera              Camera
	Floor, Grid, Border arenaMesh
}

func meshVertex(p model.Vec, col color.RGBA) ebiten.Vertex {
	return ebiten.Vertex{DstX: float32(p.X), DstY: float32(p.Y), SrcX: 1, SrcY: 1, ColorR: float32(col.R) / 255, ColorG: float32(col.G) / 255, ColorB: float32(col.B) / 255, ColorA: float32(col.A) / 255}
}
func strokeMesh(lines []geometry.Segment, c Camera, width float64, col color.RGBA) arenaMesh {
	m := arenaMesh{Vertices: make([]ebiten.Vertex, 0, len(lines)*4), Indices: make([]uint32, 0, len(lines)*6)}
	for _, line := range lines {
		a, b := c.Project(line.A), c.Project(line.B)
		d := b.Sub(a).Unit().Scale(width / 2)
		n := model.Vec{X: -d.Y, Y: d.X}
		index := uint32(len(m.Vertices))
		for _, p := range []model.Vec{a.Add(n), a.Sub(n), b.Sub(n), b.Add(n)} {
			m.Vertices = append(m.Vertices, meshVertex(p, col))
		}
		m.Indices = append(m.Indices, index, index+1, index+2, index, index+2, index+3)
	}
	return m
}

// Static geometry is rebuilt only when its envelope or projection changes.
// The render host draws the grid in one batch, not one call per hex edge.
func buildArenaDrawing(width, height float64, c Camera) arenaDrawing {
	a := geometry.NewArena(width, height)
	points := a.Vertices()
	floor := arenaMesh{}
	var edges []geometry.Segment
	for i, p := range points {
		floor.Vertices = append(floor.Vertices, meshVertex(c.Project(p), color.RGBA{R: 170, G: 177, B: 169, A: 255}))
		edges = append(edges, geometry.Segment{A: p, B: points[(i+1)%6]})
	}
	for i := uint32(1); i < 5; i++ {
		floor.Indices = append(floor.Indices, 0, i, i+1)
	}
	return arenaDrawing{Width: width, Height: height, Camera: c, Floor: floor, Grid: strokeMesh(a.Grid(26), c, 1, color.RGBA{R: 91, G: 100, B: 94, A: 42}), Border: strokeMesh(edges, c, 3, color.RGBA{93, 109, 103, 255})}
}

// Space views use server-owned player positions, including at the starting node.
// They never invent a client-side avatar or mutate the received snapshot.
func (g *Game) spaceView() *protocol.Scene {
	p := g.self()
	if p == nil || p.Scene != "" {
		return nil
	}
	var node model.Node
	for _, n := range g.state.Nodes {
		if n.ID == p.Node {
			node = n
			break
		}
	}
	w, h := g.catalog.SpaceSize(node)
	s := protocol.Scene{ID: "space:" + p.Node, Room: node.Room, State: "idle", Width: w, Height: h, Tick: g.state.ServerTick, TickRate: g.state.TickRate, ActualTPS: g.state.ActualTPS}
	if room, ok := g.catalog.Room(node.Room); ok {
		s.Obstacles = room.Obstacles
	}
	for _, view := range g.state.Scenes {
		if view.ID == s.ID {
			s.Projectiles = view.Projectiles
			break
		}
	}
	for _, v := range g.state.Players {
		if v.Node == p.Node && v.Scene == "" && (v.Active || v.Online || v.ID == g.id) {
			s.Players = append(s.Players, v)
		}
	}
	return &s
}
