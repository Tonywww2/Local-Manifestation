package client

import (
	"cmp"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

type actorSprite struct {
	position model.Vec
	index    int
	player   bool
	death    bool
}

// The renderer owns scratch storage. Entries contain only indices/positions,
// never closures that retain a player, enemy or received snapshot tree.
func sortedActorSprites(reuse []actorSprite, s protocol.Scene, c Camera) []actorSprite {
	entries := reuse[:0]
	for i, e := range s.Enemies {
		if e.HP <= 0 {
			continue
		}
		entries = append(entries, actorSprite{position: c.Project(e.Position), index: i})
	}
	for i, a := range s.EnemyActions {
		if animation.Active(a.Death) {
			entries = append(entries, actorSprite{position: c.Project(a.Position), index: i, death: true})
		}
	}
	for i, p := range s.Players {
		entries = append(entries, actorSprite{position: c.Project(p.Position), index: i, player: true})
	}
	slices.SortStableFunc(entries, func(a, b actorSprite) int { return cmp.Compare(a.position.Y, b.position.Y) })
	return entries
}

func (g *Game) drawSortedActors(dst *ebiten.Image, s protocol.Scene, c Camera) {
	if g.resources.skeletons.Actions == nil {
		g.resources.skeletons.Actions = map[int]model.EnemyActionState{}
	}
	clear(g.resources.skeletons.Actions)
	for _, a := range s.EnemyActions {
		g.resources.skeletons.Actions[a.ID] = a
	}
	g.actorSprites = sortedActorSprites(g.actorSprites, s, c)
	for _, entry := range g.actorSprites {
		v := entry.position
		if !entry.player {
			if entry.death {
				a := s.EnemyActions[entry.index]
				g.drawEnemy(dst, c, model.Enemy{ID: a.ID, Definition: a.Definition, Position: a.Position, Radius: a.Radius}, a)
				continue
			}
			e := s.Enemies[entry.index]
			a, ok := g.resources.skeletons.Actions[e.ID]
			if !ok {
				a = model.EnemyActionState{ID: e.ID, Definition: e.Definition, Position: e.Position, Radius: e.Radius, Facing: model.Vec{Y: 1}}
			}
			height := g.drawEnemy(dst, c, e, a)
			rect(dst, v.X-20, v.Y-height-10, 40, 4, muted)
			rect(dst, v.X-20, v.Y-height-10, 40*e.HP/mathMaxHP(e.MaxHP), 4, accent)
			continue
		}
		p := s.Players[entry.index]
		action, _ := sceneAction(s.Actions, p.ID)
		if p.Life == "dead" && !animation.Active(action.Life) {
			continue
		}
		col := accent
		if p.ID != g.id {
			col = color.RGBA{118, 166, 247, 255}
		}
		if p.Life != "alive" {
			col = muted
		}
		radius := max(4, 12*c.Scale)
		if p.ID == g.id {
			radius = max(7, radius)
			g.drawSelfMarker(dst, v, radius)
		}
		if p.ID == g.id && p.Life == "alive" {
			aim := model.Vec{}
			if g.localInput.Aim != (model.Vec{}) {
				aim = g.localInput.Aim.Sub(p.Position)
			}
			action.Facing = animation.PlayerFacing(action, g.localInput.Move, aim, g.localInput.Attack && p.Weapon != "")
		}
		height := g.drawAnimatedPlayer(dst, p, action, c, v, radius, col)
		g.fitText(dst, p.Name, v.X-60, v.Y-height-17, 120, 12, true)
		rect(dst, v.X-20, v.Y+radius+7, 40*p.HP/mathMaxHP(p.MaxHP), 4, col)
	}
}
