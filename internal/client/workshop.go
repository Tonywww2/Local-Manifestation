package client

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/model"
)

var workshopMetal = color.RGBA{91, 104, 104, 255}
var workshopWood = color.RGBA{183, 157, 119, 255}

func polygon(dst *ebiten.Image, points []model.Vec, col color.RGBA) {
	if len(points) < 3 {
		return
	}
	vertices := make([]ebiten.Vertex, len(points))
	indices := make([]uint32, 0, (len(points)-2)*3)
	for i, p := range points {
		vertices[i] = meshVertex(p, col)
		if i >= 2 {
			indices = append(indices, 0, uint32(i-1), uint32(i))
		}
	}
	dst.DrawTriangles32(vertices, indices, whiteImage, nil)
}

// A prop is drawn in world coordinates, so rendering and picking share the
// same ground point. Static workshop furniture is baked into the room surface.
func worldBox(dst *ebiten.Image, c Camera, p model.Vec, w, depth, height float64, top color.RGBA) {
	points := [4]model.Vec{c.Project(p.Add(model.Vec{X: -w / 2, Y: -depth / 2})), c.Project(p.Add(model.Vec{X: w / 2, Y: -depth / 2})), c.Project(p.Add(model.Vec{X: w / 2, Y: depth / 2})), c.Project(p.Add(model.Vec{X: -w / 2, Y: depth / 2}))}
	polygon(dst, points[:], top)
	for i := range points {
		line(dst, points[i], points[(i+1)%4], color.RGBA{66, 78, 79, 255}, 1)
	}
}

func raisedBox(dst *ebiten.Image, c Camera, p model.Vec, w, depth, elevation, height float64, top color.RGBA) {
	worldBox(dst, c, p, w, depth, height, top)
}

func workbench(dst *ebiten.Image, c Camera, p model.Vec, width float64) {
	for _, x := range []float64{-width/2 + 5, width/2 - 5} {
		for _, y := range []float64{-20, 20} {
			worldBox(dst, c, p.Add(model.Vec{X: x, Y: y}), 6, 6, 32, workshopMetal)
		}
	}
	worldBox(dst, c, p.Add(model.Vec{X: -width/2 + 20}), 28, 42, 28, workshopMetal)
	raisedBox(dst, c, p, width, 56, 32, 5, workshopWood)
}

// Orthographic furniture is drawn as a readable floor plan. All detail is
// baked once and follows the same rotation/zoom as the walkable footprint.
func (g *Game) decorateWorkshop(dst *ebiten.Image, c Camera) {
	for _, d := range g.front.Devices {
		p := d.Position
		zone := color.RGBA{64, 76, 80, 255}
		if d.Action == "progression" || d.Action == "warehouse" {
			zone = color.RGBA{83, 89, 80, 255}
		}
		worldBox(dst, c, p, 280, 200, 0, zone)
		worldBox(dst, c, p.Add(model.Vec{Y: 84}), 220, 4, 0, color.RGBA{112, 173, 167, 255})
		if g.drawWorldProp(dst, "prop:hall-"+d.Action, c, p, 224, 152) {
			continue
		}
		// Separate pads leave the entire central hex and connecting seams open.
		worldBox(dst, c, p, 186, 76, 0, workshopWood)
		switch d.Action {
		case "solo":
			worldBox(dst, c, p, 154, 58, 0, workshopMetal)
			worldBox(dst, c, p, 118, 36, 0, color.RGBA{42, 59, 65, 255})
			for _, x := range []float64{-36, 0, 36} {
				worldBox(dst, c, p.Add(model.Vec{X: x}), 18, 4, 0, color.RGBA{116, 203, 190, 255})
			}
		case "warehouse":
			for _, x := range []float64{-58, 0, 58} {
				worldBox(dst, c, p.Add(model.Vec{X: x}), 48, 56, 0, color.RGBA{129, 139, 116, 255})
				worldBox(dst, c, p.Add(model.Vec{X: x}), 5, 56, 0, workshopMetal)
			}
		case "host":
			for _, x := range []float64{-56, 0, 56} {
				worldBox(dst, c, p.Add(model.Vec{X: x}), 44, 62, 0, workshopMetal)
				for row := 0; row < 4; row++ {
					worldBox(dst, c, p.Add(model.Vec{X: x, Y: -22 + float64(row)*14}), 30, 4, 0, color.RGBA{63, 133, 139, 255})
				}
			}
		case "progression":
			worldBox(dst, c, p.Add(model.Vec{X: 20}), 70, 48, 0, color.RGBA{203, 205, 181, 255})
			worldBox(dst, c, p.Add(model.Vec{X: 20}), 3, 48, 0, workshopMetal)
			worldBox(dst, c, p.Add(model.Vec{X: -58}), 24, 24, 0, color.RGBA{105, 147, 130, 255})
		default:
			worldBox(dst, c, p.Add(model.Vec{Y: -10}), 104, 36, 0, workshopMetal)
			worldBox(dst, c, p.Add(model.Vec{Y: -10}), 90, 24, 0, color.RGBA{72, 139, 146, 255})
			worldBox(dst, c, p.Add(model.Vec{Y: 23}), 65, 12, 0, color.RGBA{64, 76, 82, 255})
		}
		// Chair and small plant identify these as working spaces, not altars.
		worldBox(dst, c, p.Add(model.Vec{Y: 58}), 34, 25, 0, color.RGBA{109, 124, 133, 255})
		worldBox(dst, c, p.Add(model.Vec{X: 110, Y: -60}), 24, 24, 0, color.RGBA{126, 140, 117, 255})
		worldBox(dst, c, p.Add(model.Vec{X: 110, Y: -60}), 16, 16, 0, color.RGBA{78, 115, 91, 255})
	}
	center := lobby.Spawn()
	for _, offset := range []model.Vec{{X: -70, Y: -70}, {X: 70, Y: -70}, {X: 70, Y: 70}, {X: -70, Y: 70}} {
		worldBox(dst, c, center.Add(offset), 18, 3, 0, color.RGBA{112, 173, 167, 255})
	}
}

func buildHallSurface(camera Camera, texture *ebiten.Image) roomSurface {
	local := Camera{Scale: camera.Scale, Angle: camera.Angle}
	footprint := lobby.Footprint()
	cells := footprint.Cells()
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, cell := range cells {
		for _, p := range cell.Vertices() {
			v := local.Project(p)
			minX = min(minX, v.X)
			maxX = max(maxX, v.X)
			minY = min(minY, v.Y)
			maxY = max(maxY, v.Y)
		}
	}
	local.Origin = model.Vec{X: 45 - minX, Y: 65 - minY}
	img := ebiten.NewImage(int(math.Ceil(maxX-minX))+90, int(math.Ceil(maxY-minY))+110)
	for _, cell := range cells {
		var vertices [6]ebiten.Vertex
		for i, p := range cell.Vertices() {
			vertices[i] = meshVertex(local.Project(p), color.RGBA{158, 169, 170, 255})
		}
		img.DrawTriangles32(vertices[:], []uint32{0, 1, 2, 0, 2, 3, 0, 3, 4, 0, 4, 5}, whiteImage, nil)
		for i, p := range cell.Vertices() {
			vertices[i] = meshVertex(local.Project(p), color.RGBA{158, 169, 170, 255})
			if texture != nil {
				vertices[i].SrcX = float32(p.X / lobby.Width * float64(texture.Bounds().Dx()))
				vertices[i].SrcY = float32(p.Y / lobby.Height * float64(texture.Bounds().Dy()))
				vertices[i].ColorR = 1
				vertices[i].ColorG = 1
				vertices[i].ColorB = 1
				vertices[i].ColorA = 1
			}
		}
		source := texture
		if source == nil {
			source = whiteImage
		}
		img.DrawTriangles32(vertices[:], []uint32{0, 1, 2, 0, 2, 3, 0, 3, 4, 0, 4, 5}, source, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	}
	for _, m := range []arenaMesh{strokeMesh(footprint.Grid(52), local, 1, color.RGBA{91, 100, 94, 42}), strokeMesh(footprint.Boundary(), local, 3, workshopMetal)} {
		img.DrawTriangles32(m.Vertices, m.Indices, whiteImage, nil)
	}
	return roomSurface{Image: img, Camera: local, Width: lobby.Width, Height: lobby.Height}
}

func hallBadgeLift(action string, c Camera) float64 {
	return 48*c.Scale + 18
}
