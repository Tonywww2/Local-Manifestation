package client

import (
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/status"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
)

type hallActions struct {
	updated     time.Time
	cameraStats model.StatValues
	space       *scene.FreeSpace
	controls    scene.PlayerActions
	player      model.Player
	chassis     int
	digest      string
}

// Hall actions are local display/simulation only; they never award or save loot.
func (g *Game) stepHall(input model.Input) {
	defer g.diagnostics.Since("client.logic.hall_combat", perf.Start())
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
		derived, err := stats.Evaluate(content.StatInput(player))
		if err != nil {
			g.notifyError(err)
			return
		}
		*a = hallActions{space: space, player: player, chassis: g.chassis, digest: g.catalog.Digest, cameraStats: derived.Values}
	}
	advanced, err := g.catalog.AdvancePlayerStatuses(a.player, a.player.Statuses.Time+status.UnitsPerSecond/60)
	if err != nil {
		g.notifyError(err)
		return
	}
	a.player = advanced
	a.player.Position, a.player.Name = g.front.Position, g.name
	a.player.Speed = 210 // existing personal-hall walking speed
	player, err := a.space.StepPlayer(&a.controls, a.player, input, 1.0/60, nil)
	if err != nil {
		g.notifyError(err)
		return
	}
	a.player, g.front.Position = player, player.Position
	a.updated = time.Now()
	if err := a.space.Advance(1.0 / 60); err != nil {
		g.notifyError(err)
	}
}

func (g *Game) useHallWeaponSkill(aim model.Vec) {
	a := &g.front.Actions
	if a.space == nil {
		return
	}
	d, it, ok := g.catalog.EquippedWeaponSkill(a.player)
	if !ok || it.SkillReady > a.player.Statuses.Time || a.player.Energy < d.Cost {
		return
	}
	plan, err := g.catalog.UseWeaponSkill(a.player)
	if err == nil {
		err = a.space.ApplyAction(a.player, aim, plan)
	}
	if err != nil {
		g.notifyError(err)
		return
	}
	a.player = plan.Player
	a.controls.PlaySkill(a.player, aim, plan.ClearProjectiles != nil)
}

func (g *Game) drawProjectileClear(dst *ebiten.Image, p model.Projectile, c Camera) {
	angle := math.Atan2(p.Velocity.Y, p.Velocity.X)
	half := p.Arc * math.Pi / 360
	center := c.Project(p.Position)
	var first, last model.Vec
	col := color.NRGBA{R: 143, G: 230, B: 228, A: uint8(220 * min(1, p.TTL/.2))}
	for i := 0; i <= 24; i++ {
		a := angle - half + 2*half*float64(i)/24
		next := c.Project(p.Position.Add(model.Vec{X: math.Cos(a) * p.Radius, Y: math.Sin(a) * p.Radius}))
		if i == 0 {
			first = next
		} else {
			line(dst, last, next, col, 2)
		}
		last = next
	}
	line(dst, center, first, col, 1)
	line(dst, center, last, col, 1)
}

func (g *Game) drawProjectileSprites(dst *ebiten.Image, projectiles []model.Projectile, c Camera) {
	if len(projectiles) == 0 {
		return
	}
	g.prepareProjectileArt()
	for _, p := range projectiles {
		v := c.Project(p.Position)
		extent := projectileAppearance(p, c.Scale).extent()
		if p.Kind == "projectile-clear" {
			extent = p.Radius*c.Scale + 4
		}
		if !projectileVisible(v, extent, g.width, g.height) {
			g.renderWork.Culled++
			continue
		}
		g.renderWork.Visible++
		if p.Kind == "projectile-clear" {
			g.drawProjectileClear(dst, p, c)
		} else if p.Kind == "sword" {
			g.renderWork.SpriteCalls++
			projectileSprite(dst, g.projectileArt.sword, v, p.Radius*c.Scale, color.RGBA{130, 220, 255, 255})
		} else {
			col := projectileColor(p)
			if p.Kind == "enemy" {
				g.renderWork.SpriteCalls++
				projectileSprite(dst, g.projectileArt.ring, v, max(3.5, 6*c.Scale), color.RGBA{220, 100, 100, 255})
			}
			g.renderWork.SpriteCalls++
			projectileSprite(dst, g.projectileArt.disk, v, max(2.5, 4*c.Scale), col)
		}
	}
}
