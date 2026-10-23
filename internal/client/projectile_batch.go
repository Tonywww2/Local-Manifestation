package client

import (
	_ "embed"
	"fmt"
	"image/color"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
)

//go:embed shaders/projectile.kage
var projectileShaderSource []byte

// A bounded transfer avoids a large one-off scene permanently growing staging
// memory. Indices are constant; vertices are reused by this render owner.
const projectileBatchQuads = 2048
const projectileQuadBytes = 4*12*4 + 6*4

type projectileMesh struct {
	vertices []ebiten.Vertex
	indices  []uint32
}

func newProjectileMesh(quads int) projectileMesh {
	m := projectileMesh{vertices: make([]ebiten.Vertex, 0, quads*4), indices: make([]uint32, quads*6)}
	for i := 0; i < quads; i++ {
		v, at := uint32(i*4), i*6
		copy(m.indices[at:at+6], []uint32{v, v + 1, v + 2, v, v + 2, v + 3})
	}
	return m
}

type projectileShape struct {
	fill, ring, width float64
	enemy             bool
	ink               color.RGBA
}

func projectileColor(p model.Projectile) color.RGBA {
	col := color.RGBA{248, 216, 127, 255}
	if p.Kind == "enemy" {
		col = color.RGBA{255, 90, 90, 255}
	}
	switch p.DamageType {
	case "electromagnetic":
		return color.RGBA{119, 206, 226, 255}
	case "chemical":
		return color.RGBA{173, 206, 104, 255}
	case "cold":
		return color.RGBA{167, 217, 243, 255}
	case "fire":
		return color.RGBA{241, 151, 87, 255}
	case "magic":
		return color.RGBA{184, 167, 222, 255}
	}
	return col
}

func projectileAppearance(p model.Projectile, scale float64) projectileShape {
	if p.Kind == "sword" {
		r := p.Radius * scale
		return projectileShape{ring: r, width: r * 1.1 / 12, ink: color.RGBA{130, 220, 255, 255}}
	}
	s := projectileShape{fill: max(2.5, 4*scale), ink: projectileColor(p)}
	if p.Kind == "enemy" {
		s.enemy, s.ring = true, max(3.5, 6*scale)
		s.width = s.ring / 4
	}
	return s
}

func (s projectileShape) extent() float64 { return max(s.fill, s.ring+s.width/2) + 1 }

func projectileVisible(center model.Vec, extent float64, width, height int) bool {
	return center.X+extent >= 0 && center.Y+extent >= 0 && center.X-extent <= float64(width) && center.Y-extent <= float64(height)
}

// append never retains the projectile or snapshot. All corners in a quad carry
// the same shape parameters; only local pixel position is interpolated.
func (m *projectileMesh) append(center model.Vec, s projectileShape) {
	e := float32(s.extent())
	base := ebiten.Vertex{ColorR: float32(s.ink.R) / 255, ColorG: float32(s.ink.G) / 255, ColorB: float32(s.ink.B) / 255, ColorA: float32(s.ink.A) / 255, Custom0: float32(s.fill), Custom1: float32(s.ring), Custom2: float32(s.width)}
	if s.enemy {
		base.Custom3 = 1
	}
	for _, corner := range [4][2]float32{{-e, -e}, {e, -e}, {e, e}, {-e, e}} {
		v := base
		v.DstX, v.DstY = float32(center.X)+corner[0], float32(center.Y)+corner[1]
		v.SrcX, v.SrcY = corner[0], corner[1]
		m.vertices = append(m.vertices, v)
	}
}

type projectilePipeline struct {
	shader *ebiten.Shader
	warmup *ebiten.Image
	mesh   projectileMesh
	warmed bool
}

type renderWork struct {
	ProjectileRenderer string `json:"projectile_renderer"`
	Projectiles        int    `json:"projectiles"`
	Visible            int    `json:"visible"`
	Culled             int    `json:"culled"`
	Quads              int    `json:"quads"`
	Batches            int    `json:"batch_submissions"`
	VertexIndexBytes   int    `json:"vertex_index_bytes"`
	SpriteCalls        int    `json:"sprite_calls"`
}

func (g *Game) initializeProjectilePipeline() {
	if g.cfg.ProjectileRenderer == "sprites" {
		return
	}
	defer g.diagnostics.Since("client.load.projectile_shader", perf.Start())
	shader, err := ebiten.NewShader(projectileShaderSource)
	if err != nil {
		// A source compilation failure must not make bullets invisible.
		fmt.Fprintf(os.Stderr, "projectile batch shader: %v; using cached sprites\n", err)
		return
	}
	g.projectilePipeline = projectilePipeline{shader: shader, mesh: newProjectileMesh(projectileBatchQuads), warmup: ebiten.NewImage(2, 2)}
}

func (g *Game) warmProjectilePipeline() {
	p := &g.projectilePipeline
	if p.shader == nil || p.warmed {
		return
	}
	defer g.diagnostics.Since("client.load.projectile_pipeline", perf.Start())
	// Submit a tiny offscreen draw during the initial frame to request initial
	// driver shader work before combat. GPU execution itself is asynchronous.
	p.mesh.append(model.Vec{X: 1, Y: 1}, projectileShape{fill: 1, ink: color.RGBA{255, 255, 255, 255}})
	p.warmup.DrawTrianglesShader32(p.mesh.vertices, p.mesh.indices[:6], p.shader, nil)
	p.mesh.vertices = p.mesh.vertices[:0]
	p.warmed = true
}

func (g *Game) releaseProjectilePipeline() {
	p := &g.projectilePipeline
	if p.warmup != nil {
		p.warmup.Deallocate()
	}
	if p.shader != nil {
		p.shader.Deallocate()
	}
	*p = projectilePipeline{}
}

func (g *Game) submitProjectileBatch(dst *ebiten.Image) {
	p := &g.projectilePipeline
	quads := len(p.mesh.vertices) / 4
	if quads == 0 {
		return
	}
	defer g.diagnostics.Since("client.render.projectile_submit", perf.Start())
	dst.DrawTrianglesShader32(p.mesh.vertices, p.mesh.indices[:quads*6], p.shader, nil)
	g.renderWork.Batches++
	g.renderWork.Quads += quads
	g.renderWork.VertexIndexBytes += quads * projectileQuadBytes
	p.mesh.vertices = p.mesh.vertices[:0]
}

func (g *Game) drawProjectiles(dst *ebiten.Image, projectiles []model.Projectile, c Camera) {
	defer g.diagnostics.Since("client.render.projectiles", perf.Start())
	g.renderWork.Projectiles += len(projectiles)
	if g.projectilePipeline.shader == nil {
		g.drawProjectileSprites(dst, projectiles, c)
		return
	}
	build := perf.Start()
	sin, cos := math.Sincos(c.Angle)
	for _, p := range projectiles {
		v := model.Vec{X: c.Origin.X + (p.Position.X*cos-p.Position.Y*sin)*c.Scale, Y: c.Origin.Y + (p.Position.X*sin+p.Position.Y*cos)*c.Scale}
		shape := projectileAppearance(p, c.Scale)
		extent := shape.extent()
		if p.Kind == "projectile-clear" {
			extent = p.Radius*c.Scale + 4
		}
		if !projectileVisible(v, extent, g.width, g.height) {
			g.renderWork.Culled++
			continue
		}
		g.renderWork.Visible++
		if p.Kind == "projectile-clear" {
			g.diagnostics.Since("client.render.projectile_build", build)
			g.submitProjectileBatch(dst)
			g.drawProjectileClear(dst, p, c)
			build = perf.Start()
			continue
		}
		g.projectilePipeline.mesh.append(v, shape)
		if len(g.projectilePipeline.mesh.vertices) == projectileBatchQuads*4 {
			g.diagnostics.Since("client.render.projectile_build", build)
			g.submitProjectileBatch(dst)
			build = perf.Start()
		}
	}
	g.diagnostics.Since("client.render.projectile_build", build)
	g.submitProjectileBatch(dst)
}
