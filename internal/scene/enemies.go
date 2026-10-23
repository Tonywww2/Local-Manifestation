package scene

import (
	"fmt"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
	"hsmultiplayer/sdk"
	"math"
)

// Definitions are copied from the immutable catalog once per actor, then read
// only by its owner. No per-tick clone of attack patterns or resistance maps.
func (w *world) enemyDefinition(id string) model.EnemyDef {
	if w.enemyDefinitions == nil {
		w.enemyDefinitions = map[string]model.EnemyDef{}
	}
	if d, ok := w.enemyDefinitions[id]; ok {
		return d
	}
	d := w.catalog.Enemy(id)
	w.enemyDefinitions[id] = d
	return d
}

func (w *world) addThreat(enemy int, player string, amount float64) {
	if w.threat == nil {
		w.threat = map[int]map[string]float64{}
	}
	if w.threat[enemy] == nil {
		w.threat[enemy] = map[string]float64{}
	}
	w.threat[enemy][player] += amount
}
func (w *world) enemyTarget(e model.Enemy, def model.EnemyDef) (*fighter, float64) {
	var best *fighter
	distance := 0.0
	score := -1.0
	distanceWeight := def.ThreatDistance
	if distanceWeight == 0 {
		distanceWeight = 300
	}
	for _, id := range w.playerIDs() {
		p := w.players[id]
		if p.player.Life != "alive" {
			continue
		}
		d := p.player.Position.Sub(e.Position).Length()
		value := distanceWeight/(1+d) + w.threat[e.ID][id]*def.ThreatDamage
		if value > score {
			score = value
			best = p
			distance = d
		}
	}
	return best, distance
}
func (w *world) updateEnemies(dt float64) error {
	defer w.performance.Since("scene.logic.enemies", perf.Start())
	if w.enemyAttacks == nil {
		w.enemyAttacks = map[int]enemyAttackState{}
	}
	if w.enemyFire == nil {
		w.enemyFire = map[int]float64{}
	}
	for i := range w.enemies {
		e := &w.enemies[i]
		if e.HP <= 0 {
			continue
		}
		visual := w.enemyVisual(*e)
		visual.Move = model.Vec{}
		w.setEnemyVisual(visual)
		before := e.Position
		def := w.enemyDefinition(e.Definition)
		target, distance := w.enemyTarget(*e, def)
		if target == nil {
			continue
		}
		if status.Controlled(e.Statuses, "stun") {
			continue
		}
		aiStart := perf.Start()
		a, err := w.runtime.Decide(def.Behavior, sdk.Context{Self: e.Position, Target: target.player.Position, Distance: distance, Time: w.clock, Delta: dt})
		if err != nil {
			return err
		}
		w.performance.Since("scene.logic.enemy_ai", aiStart)
		statsStart := perf.Start()
		derived, err := stats.Evaluate(stats.Input{Base: model.StatValues{MaxHP: e.MaxHP, Speed: def.Speed, Armor: def.Armor, AttackSpeed: 1}, Contributions: status.Contributions(e.Statuses)})
		if err != nil {
			return err
		}
		w.performance.Since("scene.logic.enemy_stats", statsStart)
		factor := 1.0
		if def.EnrageThreshold > 0 && e.HP/e.MaxHP <= def.EnrageThreshold {
			factor = max(1, def.EnrageFactor)
		}
		previous := w.enemyAttacks[e.ID]
		tempo := factor * (1 + def.FirePerFloor*float64(w.floor-1))
		control := advanceEnemyAttack(previous, def.Attacks, target.player.Position.Sub(e.Position), distance, dt, tempo)
		visual.Facing = target.player.Position.Sub(e.Position).Unit()
		if control.State.Phase == "windup" && previous.Phase != "windup" && len(def.Attacks) > 0 {
			pattern := def.Attacks[control.State.Index]
			visual.Attack = model.ActionAnimation{Sequence: visual.Attack.Sequence + 1, Kind: pattern.Kind, Direction: control.State.Direction, Windup: pattern.Windup / tempo, Active: pattern.Active / tempo, Duration: max(.12, (pattern.Windup+pattern.Active+pattern.Recovery)/tempo)}
		}
		w.enemyAttacks[e.ID] = control.State
		e.Phase = control.State.Phase
		e.AttackDirection = control.State.Direction
		if len(def.Attacks) > 0 {
			e.AttackKind = def.Attacks[control.State.Index].Kind
		}
		if !status.Controlled(e.Statuses, "root") && control.Override {
			if control.Move.Length() > 0 {
				next := w.clamp(e.Position.Add(control.Move.Scale(dt)), e.Radius)
				if w.clearObstacles(e.Position, next, e.Radius) {
					e.Position = next
				}
			}
		} else if !status.Controlled(e.Statuses, "root") {
			e.Position = w.enemyMove(*e, a.Move, target.player.Position, derived.Values.Speed*(1+def.SpeedPerFloor*float64(w.floor-1))*factor, dt)
		}
		visual.Position = e.Position
		if dt > 0 {
			visual.Move = e.Position.Sub(before).Scale(1 / dt)
		}
		if control.Fire || len(def.Attacks) == 0 && def.FireInterval > 0 && w.clock >= w.enemyFire[e.ID] {
			// While moving, any nearby attackable player can be fired on; closer
			// eligible targets take priority for this actual attack opportunity.
			var nearby *fighter
			best := def.AttackRange
			if control.Fire {
				best = 4096
			}
			for _, id := range w.playerIDs() {
				p := w.players[id]
				if p.player.Life == "alive" {
					d := p.player.Position.Sub(e.Position).Length()
					if d < best {
						best = d
						nearby = p
					}
				}
			}
			if nearby != nil {
				fireStart := perf.Start()
				dir := nearby.player.Position.Sub(e.Position).Unit()
				if control.Fire {
					dir = control.State.Direction
				}
				if !control.Fire {
					visual.Attack = model.ActionAnimation{Sequence: visual.Attack.Sequence + 1, Kind: "shoot", Direction: dir, Duration: .28, Active: .08}
				}
				count := max(1, def.ProjectileCount)
				if factor > 1 {
					count += def.EnrageProjectiles
				}
				base := math.Atan2(dir.Y, dir.X)
				kind := def.DamageType
				if kind == "" {
					kind = "physical"
				}
				for j := 0; j < count; j++ {
					angle := base + (float64(j)-float64(count-1)/2)*.18
					w.nextID++
					w.shots = append(w.shots, shot{identity: w.shotIdentity(shot{view: model.Projectile{ID: w.nextID}}), attack: true, remaining: 1, hits: map[string]bool{}, view: model.Projectile{ID: w.nextID, Owner: fmt.Sprint(e.ID), Kind: "enemy", DamageType: kind, Position: e.Position, Velocity: model.Vec{X: math.Cos(angle) * def.ProjectileSpeed, Y: math.Sin(angle) * def.ProjectileSpeed}, TTL: 5}, weapon: model.Weapon{ID: def.ID, Type: kind, Base: def.Damage * (1 + w.room.Scaling.DamagePerFloor*float64(w.floor-1))}, multiplier: 1})
				}
				w.performance.Since("scene.logic.enemy_fire", fireStart)
				w.enemyFire[e.ID] = w.clock + def.FireInterval/(factor*(1+def.FirePerFloor*float64(w.floor-1)))
			}
		}
		w.setEnemyVisual(visual)
		for _, id := range w.playerIDs() {
			p := w.players[id]
			if p.player.Life != "alive" || p.invulnerable > 0 || p.player.Position.Sub(e.Position).Length() >= e.Radius+12 || w.clock < p.contact[e.ID] {
				continue
			}
			p.contact[e.ID] = w.clock + .8
			attackID := fmt.Sprintf("%s:contact:%d:%s:%d", w.id, e.ID, id, w.tick)
			_, err = w.resolveHit(damage.Request{Weapon: model.Weapon{ID: def.ID, Type: "physical", Base: def.Damage * (1 + w.room.Scaling.DamagePerFloor*float64(w.floor-1))}, FinalMultiplier: 1}, model.Event{Kind: "hit", Attack: true, Source: fmt.Sprint(e.ID), SourceDefinition: def.ID, Target: id, AttackID: attackID, HitID: attackID, Tick: w.tick}, nil)
			if err != nil {
				return err
			}
			if !animation.Active(visual.Attack) {
				visual.Attack = model.ActionAnimation{Sequence: visual.Attack.Sequence + 1, Kind: "melee", Direction: p.player.Position.Sub(e.Position).Unit(), Duration: .26, Active: .1}
				w.setEnemyVisual(visual)
			}
		}
	}
	return nil
}
