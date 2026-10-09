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

// A prop is drawn in world coordinates, so both cameras and picking share the
// same ground point. Static workshop furniture is baked into the room surface.
func worldBox(dst *ebiten.Image, c Camera, p model.Vec, w, depth, height float64, top color.RGBA) {
	points := [4]model.Vec{c.Project(p.Add(model.Vec{X: -w / 2, Y: -depth / 2})), c.Project(p.Add(model.Vec{X: w / 2, Y: -depth / 2})), c.Project(p.Add(model.Vec{X: w / 2, Y: depth / 2})), c.Project(p.Add(model.Vec{X: -w / 2, Y: depth / 2}))}
	lift := 0.0
	if c.Oblique {
		lift = height * c.Scale
	}
	upper := points
	for i := range upper {
		upper[i].Y -= lift
	}
	if lift > 0 {
		polygon(dst, []model.Vec{points[1], points[2], upper[2], upper[1]}, color.RGBA{uint8(float64(top.R) * .64), uint8(float64(top.G) * .64), uint8(float64(top.B) * .64), 255})
		polygon(dst, []model.Vec{points[2], points[3], upper[3], upper[2]}, color.RGBA{uint8(float64(top.R) * .8), uint8(float64(top.G) * .8), uint8(float64(top.B) * .8), 255})
	}
	polygon(dst, upper[:], top)
	for i := range upper {
		line(dst, upper[i], upper[(i+1)%4], color.RGBA{66, 78, 79, 255}, 1)
	}
}

func raisedBox(dst *ebiten.Image, c Camera, p model.Vec, w, depth, elevation, height float64, top color.RGBA) {
	if c.Oblique {
		c.Origin.Y -= elevation * c.Scale
	}
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

func (g *Game) decorateWorkshop(dst *ebiten.Image, c Camera) {
	for _, edge := range lobby.Footprint().Boundary() {
		start, end := c.Project(edge.A), c.Project(edge.B)
		lift := model.Vec{Y: -36 * c.Scale}
		if c.Oblique && edge.B.X > edge.A.X {
			polygon(dst, []model.Vec{start, end, end.Add(lift), start.Add(lift)}, color.RGBA{185, 184, 171, 255})
		}
		line(dst, start, end, color.RGBA{113, 125, 119, 255}, 5)
	}
	for _, d := range g.front.Devices {
		p := d.Position
		switch d.Action {
		case "solo":
			worldBox(dst, c, p, 108, 22, 74, workshopMetal)
			worldBox(dst, c, p.Add(model.Vec{Y: -2}), 80, 18, 76, color.RGBA{59, 73, 76, 255})
		case "warehouse":
			for _, x := range []float64{-72, 72} {
				for _, y := range []float64{-21, 21} {
					worldBox(dst, c, p.Add(model.Vec{X: x, Y: y}), 5, 5, 72, workshopMetal)
				}
			}
			for row := 0; row < 2; row++ {
				raisedBox(dst, c, p, 148, 46, float64(row)*35+2, 3, workshopMetal)
				for i := 0; i < 3; i++ {
					raisedBox(dst, c, p.Add(model.Vec{X: -46 + float64(i)*46, Y: -2}), 35, 32, float64(row)*35+5, 26, color.RGBA{110, 122, 96, 255})
				}
			}
		case "progression":
			workbench(dst, c, p, 150)
			raisedBox(dst, c, p.Add(model.Vec{X: 20, Y: -5}), 74, 35, 37, 1, color.RGBA{111, 139, 124, 255})
			raisedBox(dst, c, p.Add(model.Vec{X: -49, Y: -12}), 20, 20, 37, 10, workshopMetal)
		default:
			workbench(dst, c, p, 108)
			raisedBox(dst, c, p.Add(model.Vec{Y: -8}), 50, 12, 37, 22, workshopMetal)
			raisedBox(dst, c, p.Add(model.Vec{Y: -7}), 42, 10, 39, 19, color.RGBA{71, 135, 140, 255})
			raisedBox(dst, c, p.Add(model.Vec{Y: 16}), 42, 12, 37, 2, color.RGBA{68, 81, 84, 255})
		}
	}
	// Quiet storage and greenery give the hall a studio identity without adding
	// extra interactive facilities or implied gameplay systems.
	for _, p := range []model.Vec{{X: 70, Y: 280}, {X: 930, Y: 280}} {
		worldBox(dst, c, p, 34, 34, 22, color.RGBA{134, 144, 124, 255})
		raisedBox(dst, c, p.Add(model.Vec{Y: -3}), 28, 24, 22, 20, color.RGBA{88, 115, 89, 255})
	}
}

func buildHallSurface(camera Camera, texture *ebiten.Image) roomSurface {
	local := Camera{Scale: camera.Scale, Oblique: camera.Oblique}
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
			vertices[i] = meshVertex(local.Project(p), color.RGBA{192, 196, 184, 255})
		}
		img.DrawTriangles32(vertices[:], []uint32{0, 1, 2, 0, 2, 3, 0, 3, 4, 0, 4, 5}, whiteImage, nil)
		for i, p := range cell.Vertices() {
			vertices[i] = meshVertex(local.Project(p), color.RGBA{192, 196, 184, 255})
			if texture != nil {
				vertices[i].SrcX = float32(p.X / lobby.Width * float64(texture.Bounds().Dx()))
				vertices[i].SrcY = float32(p.Y / lobby.Height * float64(texture.Bounds().Dy()))
				vertices[i].ColorR = .9
				vertices[i].ColorG = .92
				vertices[i].ColorB = .9
				vertices[i].ColorA = .22
			}
		}
		source := texture
		if source == nil {
			source = whiteImage
		}
		img.DrawTriangles32(vertices[:], []uint32{0, 1, 2, 0, 2, 3, 0, 3, 4, 0, 4, 5}, source, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	}
	for _, m := range []arenaMesh{strokeMesh(footprint.Grid(26), local, 1, color.RGBA{91, 100, 94, 42}), strokeMesh(footprint.Boundary(), local, 3, workshopMetal)} {
		img.DrawTriangles32(m.Vertices, m.Indices, whiteImage, nil)
	}
	return roomSurface{Image: img, Camera: local, Width: lobby.Width, Height: lobby.Height}
}

func hallBadgeLift(action string, c Camera) float64 {
	height := 24.0
	if c.Oblique {
		height = 66
		if action == "warehouse" || action == "solo" {
			height = 86
		}
		if action == "progression" {
			height = 48
		}
	}
	return height*c.Scale + 18
}
