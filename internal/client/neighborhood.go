package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
	"math"
	"slices"
)

type roomSurface struct {
	Image         *ebiten.Image
	Camera        Camera
	Width, Height float64
	Hexes         []model.HexCell
	Room, Art     string
	Obstacles     []model.Obstacle
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
	c := g.worldBaseCamera()
	c.Angle = g.cameraAngle()
	c.Scale *= g.cameraZoom("world")
	c.Origin = model.Vec{}
	if p := g.self(); p != nil {
		position := p.Position
		if g.prediction.Valid && g.prediction.Node == p.Node && g.prediction.Epoch == p.PositionEpoch {
			position = g.prediction.Position
		}
		c.Origin = model.Vec{X: float64(g.width) / 2, Y: float64(g.height) / 2}.Sub(c.Project(position))
	}
	return c
}

func (g *Game) worldBaseCamera() Camera {
	return FitArenaCameraIn(800, 500, defaultCameraAngle, ui.Rect{X: 24, Y: 24, W: float64(g.width - 48), H: float64(g.height - 48)})
}
func (g *Game) sceneForNode(n model.Node) protocol.Scene {
	for _, s := range g.state.Scenes {
		if s.ID == n.ID {
			return s
		}
	}
	w, h := g.catalog.SpaceSize(n)
	s := protocol.Scene{ID: "space:" + n.ID, State: "idle", Room: n.Room, Width: w, Height: h, Hexes: g.catalog.SpaceHexes(n), Tick: g.state.ServerTick, TickRate: g.state.TickRate}
	for _, view := range g.state.Scenes {
		if view.ID == s.ID {
			s.Projectiles = view.Projectiles
			s.Actions = view.Actions
			s.EnemyActions = view.EnemyActions
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

func buildRoomSurface(s protocol.Scene, c Camera, texture, obstacle *ebiten.Image) roomSurface {
	// Cache room-local pixels, not a full-screen image whose origin changes with
	// the camera. Movement needs only a translated DrawImage.
	local := Camera{Scale: c.Scale, Angle: c.Angle}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	a := compileSceneArena(s)
	for _, edge := range a.Boundary() {
		for _, p := range []model.Vec{edge.A, edge.B} {
			v := local.Project(p)
			minX = min(minX, v.X)
			maxX = max(maxX, v.X)
			minY = min(minY, v.Y)
			maxY = max(maxY, v.Y)
		}
	}
	local.Origin = model.Vec{X: 43 - minX, Y: 43 - minY}
	image := ebiten.NewImage(int(math.Ceil(maxX-minX))+86, int(math.Ceil(maxY-minY))+86)
	drawing := buildRoomDrawing(a, s.Width, s.Height, local)
	if texture != nil {
		image.DrawTriangles32(drawing.Floor.Vertices, drawing.Floor.Indices, whiteImage, nil)
		mesh := drawing.Floor
		for i, vertex := range mesh.Vertices {
			p := local.Unproject(model.Vec{X: float64(vertex.DstX), Y: float64(vertex.DstY)})
			mesh.Vertices[i].SrcX = float32(p.X / s.Width * float64(texture.Bounds().Dx()))
			mesh.Vertices[i].SrcY = float32(p.Y / s.Height * float64(texture.Bounds().Dy()))
			mesh.Vertices[i].ColorR = 1
			mesh.Vertices[i].ColorG = 1
			mesh.Vertices[i].ColorB = 1
			mesh.Vertices[i].ColorA = .7
		}
		image.DrawTriangles32(mesh.Vertices, mesh.Indices, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear, Address: ebiten.AddressRepeat})
	} else {
		image.DrawTriangles32(drawing.Floor.Vertices, drawing.Floor.Indices, whiteImage, nil)
	}
	for _, m := range []arenaMesh{drawing.Grid, drawing.Border} {
		image.DrawTriangles32(m.Vertices, m.Indices, whiteImage, nil)
	}
	for _, o := range s.Obstacles {
		if !drawWorldArt(image, obstacle, local, o.Position, o.Radius*1.6, o.Radius*1.6) {
			worldBox(image, local, o.Position, o.Radius*1.6, o.Radius*1.6, 24, workshopMetal)
		}
	}
	return roomSurface{Image: image, Camera: local, Width: s.Width, Height: s.Height, Hexes: slices.Clone(s.Hexes), Room: s.Room, Obstacles: slices.Clone(s.Obstacles)}
}
func matchingSurface(surface roomSurface, s protocol.Scene, c Camera) bool {
	return surface.Image != nil && surface.Width == s.Width && surface.Height == s.Height && surface.Room == s.Room && slices.Equal(surface.Hexes, s.Hexes) && slices.Equal(surface.Obstacles, s.Obstacles) && surface.Camera.Scale == c.Scale && surface.Camera.Angle == c.Angle
}

func (g *Game) roomFloorArt(n model.Node, s protocol.Scene) string {
	for _, key := range [...]string{"floor:" + n.Definition, "floor:" + s.Room, "floor:" + n.Kind, "floor:default"} {
		if g.resources.images[key] != nil {
			return key
		}
	}
	return ""
}

// Rotation/zoom transform cached pixels on the GPU. They never regenerate the
// hex grid or allocate a new room texture on every mouse/key sample.
func drawRoomSurface(dst *ebiten.Image, surface roomSurface, c Camera) {
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-surface.Camera.Origin.X, -surface.Camera.Origin.Y)
	ratio := c.Scale / surface.Camera.Scale
	op.GeoM.Scale(ratio, ratio)
	op.GeoM.Rotate(c.Angle - surface.Camera.Angle)
	op.GeoM.Translate(c.Origin.X, c.Origin.Y)
	dst.DrawImage(surface.Image, op)
}

// Room surfaces may be prepared ahead of entry, but each room is an isolated scene.
func (g *Game) drawNeighborhood(dst *ebiten.Image, current protocol.Scene) {
	defer g.diagnostics.Since("client.render.rooms", perf.Start())
	from := g.currentNode()
	if g.roomSurfaces == nil {
		g.roomSurfaces = map[string]roomSurface{}
	}
	nodes := []model.Node{from}
	keep := map[string]bool{from.ID: true}
	for _, n := range g.state.Nodes {
		if n.ID != from.ID && inFirstRing(from, n) {
			nodes = append(nodes, n)
			keep[n.ID] = true
		}
	}
	for id, surface := range g.roomSurfaces {
		if !keep[id] {
			surface.Image.Dispose()
			delete(g.roomSurfaces, id)
		}
	}
	preparedNeighbor := false
	cacheCamera := Camera{Scale: g.worldBaseCamera().Scale}
	for _, n := range nodes {
		s := current
		if n.ID != from.ID {
			s = g.sceneForNode(n)
		}
		surface := g.roomSurfaces[n.ID]
		art := g.roomFloorArt(n, s)
		if !matchingSurface(surface, s, cacheCamera) || surface.Art != art {
			if n.ID != from.ID && preparedNeighbor {
				continue
			}
			if surface.Image != nil {
				surface.Image.Dispose()
			}
			texture := g.resources.images[art]
			loadStart := perf.Start()
			surface = buildRoomSurface(s, cacheCamera, texture, g.resources.sprites["prop:obstacle"])
			surface.Art = art
			g.diagnostics.Since("client.load.room_surface", loadStart)
			g.roomSurfaces[n.ID] = surface
			preparedNeighbor = preparedNeighbor || n.ID != from.ID
		}
		if n.ID == from.ID {
			c := g.camera(s)
			floorStart := perf.Start()
			drawRoomSurface(dst, surface, c)
			g.diagnostics.Since("client.render.terrain", floorStart)
			g.drawRoomEntities(dst, s, c)
			g.drawDevices(dst, n, s, c)
		}
	}
}
