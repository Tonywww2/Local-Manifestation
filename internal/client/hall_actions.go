package client

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/model"
)

type hallActions struct {
	space    *scene.FreeSpace
	controls scene.PlayerActions
	player   model.Player
	chassis  int
	digest   string
}

// Hall actions are local display/simulation only; they never award or save loot.
func (g *Game) stepHall(input model.Input) {
	if g.front.Page != "lobby" {
		return
	}
	a := &g.front.Actions
	if a.space == nil || a.chassis != g.chassis || a.digest != g.catalog.Digest {
		chassis := g.catalog.ChassisList()
		if g.chassis < 0 || g.chassis >= len(chassis) {
			return
		}
		player, err := content.Player(g.catalog, "hall", g.name, chassis[g.chassis].ID)
		if err != nil {
			g.notifyError(err)
			return
		}
		space, err := scene.NewShapedFreeSpace(g.catalog, scene.Config{ID: "space:hall", Floor: 1, TickRate: 60}, model.RoomDef{Width: lobby.Width, Height: lobby.Height, Obstacles: lobby.Obstacles()}, lobby.Footprint())
		if err != nil {
			g.notifyError(err)
			return
		}
		*a = hallActions{space: space, player: player, chassis: g.chassis, digest: g.catalog.Digest}
	}
	a.player.Position, a.player.Name = g.front.Position, g.name
	a.player.Speed = 210 // existing personal-hall walking speed
	player, err := a.space.StepPlayer(&a.controls, a.player, input, 1.0/60, nil)
	if err != nil {
		g.notifyError(err)
		return
	}
	a.player, g.front.Position = player, player.Position
	if err := a.space.Advance(1.0 / 60); err != nil {
		g.notifyError(err)
	}
}

func (g *Game) drawProjectiles(dst *ebiten.Image, projectiles []model.Projectile, c Camera) {
	for _, p := range projectiles {
		v := c.Project(p.Position)
		if p.Kind == "sword" {
			vector.StrokeCircle(dst, float32(v.X), float32(v.Y), float32(p.Radius*c.Scale), 3, color.RGBA{130, 220, 255, 255}, true)
		} else {
			col := color.RGBA{248, 216, 127, 255}
			if p.Kind == "enemy" {
				col = color.RGBA{255, 90, 90, 255}
			}
			switch p.DamageType {
			case "electromagnetic":
				col = color.RGBA{119, 206, 226, 255}
			case "chemical":
				col = color.RGBA{173, 206, 104, 255}
			case "cold":
				col = color.RGBA{167, 217, 243, 255}
			case "fire":
				col = color.RGBA{241, 151, 87, 255}
			case "magic":
				col = color.RGBA{184, 167, 222, 255}
			}
			if p.Kind == "enemy" {
				vector.StrokeCircle(dst, float32(v.X), float32(v.Y), float32(max(3.5, 6*c.Scale)), 1.5, color.RGBA{220, 100, 100, 255}, true)
			}
			vector.FillCircle(dst, float32(v.X), float32(v.Y), float32(max(2.5, 4*c.Scale)), col, true)
		}
	}
}
