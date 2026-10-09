package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"math"
)

type roomSurface struct {
	Image         *ebiten.Image
	Camera        Camera
	Width, Height float64
}

func inFirstRing(a, b model.Node) bool {
	dq, dr := a.Q-b.Q, a.R-b.R
	return max(absInt(dq), absInt(dr), absInt(dq+dr)) <= 1
}
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func (g *Game) currentNode() model.Node {
	if p := g.self(); p != nil {
		for _, n := range g.state.Nodes {
			if n.ID == p.Node {
				return n
			}
		}
	}
	return model.Node{}
}

func (g *Game) worldCamera() Camera {
	a := geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight)
	// Keep the previous viewing scale: larger rooms extend beyond the viewport.
	c := FitArenaCameraIn(800, 500, g.oblique, ui.Rect{X: 24, Y: 24, W: float64(g.width - 48), H: float64(g.height - 48)})
	c.Origin = model.Vec{}
	if p := g.self(); p != nil {
		n := g.currentNode()
		w, h := g.catalog.SpaceSize(n)
		if s := g.currentScene(); s != nil {
			w, h = s.Width, s.Height
		}
		room := geometry.NewArena(w, h)
		position := p.Position
		if g.prediction.Valid && g.prediction.Node == p.Node && g.prediction.Epoch == p.PositionEpoch {
			position = g.prediction.Position
		}
		world := room.WorldPoint(n, position, a.Radius)
		c.Origin = model.Vec{X: float64(g.width) / 2, Y: float64(g.height) / 2}.Sub(c.Project(world))
	}
	return c
}
func (g *Game) roomCamera(n model.Node, s protocol.Scene, world Camera) Camera {
	a := geometry.NewArena(s.Width, s.Height)
	factor := geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight).Radius / a.Radius
	origin := geometry.AxialPoint(n.Q, n.R).Scale(a.Radius * factor).Sub(a.Center.Scale(factor))
	return Camera{Origin: world.Project(origin), Scale: world.Scale * factor, Oblique: world.Oblique}
}

func (g *Game) sceneForNode(n model.Node) protocol.Scene {
	for _, s := range g.state.Scenes {
		if s.ID == n.ID {
			return s
		}
	}
	w, h := g.catalog.SpaceSize(n)
	s := protocol.Scene{ID: "space:" + n.ID, State: "idle", Room: n.Room, Width: w, Height: h, Tick: g.state.ServerTick, TickRate: g.state.TickRate}
	for _, view := range g.state.Scenes {
		if view.ID == s.ID {
			s.Projectiles = view.Projectiles
			break
		}
	}
	if room, ok := g.catalog.Room(n.Room); ok {
		s.Obstacles = room.Obstacles
	}
	for _, p := range g.state.Players {
		if p.Node == n.ID && p.Scene == "" && (p.Active || p.Online) {
			s.Players = append(s.Players, p)
		}
	}
	return s
}

func buildRoomSurface(s protocol.Scene, c Camera, texture *ebiten.Image) roomSurface {
	// Cache room-local pixels, not a full-screen image whose origin changes with
	// the camera. Movement needs only a translated DrawImage.
	local := Camera{Scale: c.Scale, Oblique: c.Oblique}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range geometry.NewArena(s.Width, s.Height).Vertices() {
		v := local.Project(p)
		minX = min(minX, v.X)
		maxX = max(maxX, v.X)
		minY = min(minY, v.Y)
		maxY = max(maxY, v.Y)
	}
	local.Origin = model.Vec{X: 43 - minX, Y: 43 - minY}
	image := ebiten.NewImage(int(math.Ceil(maxX-minX))+86, int(math.Ceil(maxY-minY))+86)
	drawing := buildArenaDrawing(s.Width, s.Height, local)
	if texture != nil {
		image.DrawTriangles32(drawing.Floor.Vertices, drawing.Floor.Indices, whiteImage, nil)
		points := geometry.NewArena(s.Width, s.Height).Vertices()
		mesh := drawing.Floor
		for i, p := range points {
			mesh.Vertices[i].SrcX = float32(p.X / s.Width * float64(texture.Bounds().Dx()))
			mesh.Vertices[i].SrcY = float32(p.Y / s.Height * float64(texture.Bounds().Dy()))
			mesh.Vertices[i].ColorR = .82
			mesh.Vertices[i].ColorG = .84
			mesh.Vertices[i].ColorB = .82
			mesh.Vertices[i].ColorA = .22
		}
		image.DrawTriangles32(mesh.Vertices, mesh.Indices, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	} else {
		image.DrawTriangles32(drawing.Floor.Vertices, drawing.Floor.Indices, whiteImage, nil)
	}
	for _, m := range []arenaMesh{drawing.Grid, drawing.Border} {
		image.DrawTriangles32(m.Vertices, m.Indices, whiteImage, nil)
	}
	for _, o := range s.Obstacles {
		worldBox(image, local, o.Position, o.Radius*1.6, o.Radius*1.6, 24, workshopMetal)
	}
	return roomSurface{Image: image, Camera: local, Width: s.Width, Height: s.Height}
}
func matchingSurface(surface roomSurface, s protocol.Scene, c Camera) bool {
	return surface.Image != nil && surface.Width == s.Width && surface.Height == s.Height && surface.Camera.Scale == c.Scale && surface.Camera.Oblique == c.Oblique
}

func (g *Game) drawNeighborhood(dst *ebiten.Image, current protocol.Scene) {
	from := g.currentNode()
	world := g.worldCamera()
	if g.roomSurfaces == nil {
		g.roomSurfaces = map[string]roomSurface{}
	}
	visible := []model.Node{from}
	keep := map[string]bool{from.ID: true}
	for _, n := range g.state.Nodes {
		if n.ID != from.ID && inFirstRing(from, n) {
			visible = append(visible, n)
			keep[n.ID] = true
		}
	}
	for id, surface := range g.roomSurfaces {
		if !keep[id] {
			surface.Image.Dispose()
			delete(g.roomSurfaces, id)
		}
	}
	loadedNeighbor := false
	for _, n := range visible {
		s := current
		if n.ID != from.ID {
			s = g.sceneForNode(n)
			if view, ok := g.renderRooms[n.ID]; ok {
				s = view
			}
		}
		c := g.roomCamera(n, s, world)
		surface := g.roomSurfaces[n.ID]
		if !matchingSurface(surface, s, c) {
			// Prioritize the current room and prepare one neighbor per frame. Rooms
			// are loaded before walking reaches them; no enemy actors start early.
			if n.ID != from.ID && loadedNeighbor {
				continue
			}
			if surface.Image != nil {
				surface.Image.Dispose()
			}
			texture := g.resources.images["floor:"+n.Definition]
			if texture == nil {
				texture = g.resources.images["floor:"+s.Room]
			}
			if texture == nil {
				texture = g.resources.images["floor:default"]
			}
			surface = buildRoomSurface(s, c, texture)
			g.roomSurfaces[n.ID] = surface
			if n.ID != from.ID {
				loadedNeighbor = true
			}
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(c.Origin.X-surface.Camera.Origin.X, c.Origin.Y-surface.Camera.Origin.Y)
		dst.DrawImage(surface.Image, op)
		g.drawDoorsForNode(dst, n, s, c)
		g.drawRoomEntities(dst, s, c)
		g.drawDevices(dst, n, s, c)
	}
	// Terrain is a visible cell, never a preloaded room or movement destination.
	for _, t := range g.state.Terrain {
		n := model.Node{Q: t.Q, R: t.R}
		if !inFirstRing(from, n) {
			continue
		}
		center := world.Project(geometry.AxialPoint(t.Q, t.R).Scale(geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight).Radius))
		fitText(dst, g.localized(t.NameKey, t.Name), center.X-80, center.Y-10, 160, 16, true)
	}
}
