package scene

import (
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
)

// View returns only scalar display values; borrowed player trees never escape.
func (a PlayerActions) View(player string, life string) model.PlayerActionState {
	return actionView(a.state.visual, player, life)
}

func actionView(v model.PlayerActionState, player, life string) model.PlayerActionState {
	v.Player = player
	if v.Facing.Length() == 0 {
		v.Facing = model.Vec{X: 1}
	}
	if life != "alive" {
		v = animation.Cancel(v)
	}
	return v
}

func (a *PlayerActions) PlaySkill(p model.Player, aim model.Vec, clear bool) {
	a.state.playSkill(p, aim, clear)
}

func (p *fighter) playSkill(player model.Player, aim model.Vec, clear bool) {
	direction := aim.Sub(player.Position).Unit()
	if direction.Length() == 0 || !direction.Finite() {
		direction = p.visual.Facing
	}
	kind := "cast"
	if clear {
		kind = "clear"
	}
	p.visual.Facing = direction
	p.visual.Skill = model.ActionAnimation{Sequence: p.visual.Skill.Sequence + 1, Weapon: player.Weapon, Kind: kind, Direction: direction, Duration: .32, Active: .14}
}

func (p *fighter) syncVisualLife() {
	if p.player.Life == "alive" {
		if p.visual.Life.Kind != "" {
			p.visual.Life = model.ActionAnimation{Sequence: p.visual.Life.Sequence + 1}
		}
		return
	}
	if p.visual.Life.Kind != p.player.Life {
		duration := .36
		if p.player.Life == "dead" {
			duration = .65
		}
		p.visual.Life = model.ActionAnimation{Sequence: p.visual.Life.Sequence + 1, Kind: p.player.Life, Duration: duration}
	}
}

func (w *world) enemyVisual(e model.Enemy) model.EnemyActionState {
	v, ok := w.enemyVisuals[e.ID]
	if !ok {
		v = model.EnemyActionState{ID: e.ID, Definition: e.Definition, Radius: e.Radius, Facing: model.Vec{Y: 1}}
	}
	v.Position = e.Position
	return v
}
func (w *world) setEnemyVisual(v model.EnemyActionState) {
	if w.enemyVisuals == nil {
		w.enemyVisuals = map[int]model.EnemyActionState{}
	}
	w.enemyVisuals[v.ID] = v
}
func (w *world) advanceEnemyVisuals(dt float64) {
	for id, v := range w.enemyVisuals {
		if v.Death.Duration > 0 {
			v = animation.AdvanceEnemy(v, dt)
			if !animation.Active(v.Death) {
				delete(w.enemyVisuals, id)
				continue
			}
		} else if w.state != "paused" && w.state != "faulted" && w.state != "cleared" {
			v = animation.AdvanceEnemy(v, dt)
		}
		w.enemyVisuals[id] = v
	}
}

func (w *world) recordEnemyHit(e model.Enemy, direction model.Vec, killed bool) {
	v := w.enemyVisual(e)
	v.Hit = model.ActionAnimation{Sequence: v.Hit.Sequence + 1, Direction: direction, Duration: .16}
	if killed && v.Death.Duration == 0 {
		v.Move = model.Vec{}
		v.Attack.Duration = 0
		v.Death = model.ActionAnimation{Sequence: 1, Kind: "dead", Duration: .65}
	}
	w.setEnemyVisual(v)
}
