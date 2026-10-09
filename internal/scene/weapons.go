package scene

import (
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"math"
)

type preparedAttack struct {
	request   damage.Request
	direction model.Vec
	delay     float64
	critical  bool
}

func (w *world) beginAttack(p *fighter) error {
	q, err := content.Attack(w.catalog, p.player)
	if err != nil {
		return err
	}
	dir := p.input.Aim.Sub(p.player.Position).Unit()
	if dir.Length() == 0 {
		return nil
	}
	derived, err := stats.Evaluate(content.StatInput(p.player))
	if err != nil {
		return err
	}
	speed := max(.05, derived.Values.AttackSpeed)
	p.attack = &preparedAttack{request: q, direction: dir, delay: q.Weapon.Windup / speed, critical: w.rng.Float64() < .05}
	duration := q.Weapon.Cooldown
	if q.Weapon.Windup+q.Weapon.Recovery > 0 {
		duration = q.Weapon.Windup + q.Weapon.Recovery
	}
	p.cooldown = duration / speed
	return nil
}
func (w *world) releaseAttack(p *fighter, dt float64) {
	a := p.attack
	if a == nil {
		return
	}
	if p.player.Life != "alive" {
		p.attack = nil
		return
	}
	a.delay -= dt
	if a.delay > 1e-9 {
		return
	}
	w.launch(p.player.ID, p.player.Position, a.direction, a.request, a.critical, true)
	p.attack = nil
}
func (w *world) launch(owner string, position, direction model.Vec, q damage.Request, critical, attack bool) {
	w.nextID++
	weapon := q.Weapon
	view := model.Projectile{DamageType: weapon.Type, ID: w.nextID, Owner: owner, Position: position, Velocity: direction.Scale(weapon.ProjectileSpeed), TTL: 2}
	if weapon.Mode != "melee" && weapon.Range > 0 {
		view.TTL = weapon.Range / max(.01, weapon.ProjectileSpeed)
	}
	var sweepStart *model.Vec
	remaining := max(1, weapon.Pierce)
	if weapon.Mode == "melee" {
		view.Kind = "sword"
		view.Position = position.Add(direction.Scale(weapon.Range * .55))
		view.Velocity = model.Vec{}
		view.Radius = weapon.Range * .5
		view.TTL = max(.02, weapon.ActiveDuration)
		remaining = 128
		if !w.inside(view.Position) {
			v := position
			sweepStart = &v
		}
	}
	w.shots = append(w.shots, shot{effects: q.Effects, critMultiplier: q.CritMultiplier, sweepStart: sweepStart, identity: w.shotIdentity(shot{view: view}), source: q.Source, attack: attack, view: view, weapon: weapon, statInput: q.Stats, flatBonuses: q.FlatBonuses, attributes: q.Attributes, finalSource: q.FinalSource, multiplier: q.FinalMultiplier, critical: critical, hits: map[string]bool{}, remaining: remaining})
}
func steer(velocity, target model.Vec, rate, dt float64) model.Vec {
	if velocity.Length() == 0 || target.Length() == 0 {
		return velocity
	}
	a := math.Atan2(velocity.Y, velocity.X)
	b := math.Atan2(target.Y, target.X)
	delta := math.Atan2(math.Sin(b-a), math.Cos(b-a))
	delta = max(-rate*dt, min(rate*dt, delta))
	a += delta
	return model.Vec{X: math.Cos(a) * velocity.Length(), Y: math.Sin(a) * velocity.Length()}
}
func (w *world) updateShots(dt float64) error {
	kept := w.shots[:0]
	for _, s := range w.shots {
		s.view.Age += dt
		s.view.TTL -= dt
		if s.view.TTL <= 0 {
			continue
		}
		if s.identity == "" {
			s.identity = w.shotIdentity(s)
		}
		if s.view.Kind != "enemy" && s.weapon.Tracking > 0 {
			var target model.Vec
			best := math.MaxFloat64
			for _, e := range w.enemies {
				if e.HP > 0 {
					d := e.Position.Sub(s.view.Position)
					if d.Length() < best {
						best = d.Length()
						target = d
					}
				}
			}
			s.view.Velocity = steer(s.view.Velocity, target, s.weapon.Tracking, dt)
		}
		previous := s.view.Position
		if s.sweepStart != nil {
			previous = *s.sweepStart
			s.sweepStart = nil
		}
		s.view.Position = s.view.Position.Add(s.view.Velocity.Scale(dt))
		if s.view.Kind != "sword" {
			segment, intersects := w.clipShot(geometry.Segment{A: previous, B: s.view.Position})
			if !intersects || !w.clearObstacles(segment.A, segment.B, 4) || !w.transit && !w.inside(s.view.Position) {
				continue
			}
		}
		if s.remaining == 0 {
			s.remaining = max(1, s.weapon.Pierce)
		}
		if s.hits == nil {
			s.hits = map[string]bool{}
		}
		if s.view.Kind == "enemy" {
			for _, id := range w.playerIDs() {
				p := w.players[id]
				key := "player:" + id
				if p.player.Life != "alive" || p.invulnerable > 0 || s.hits[key] {
					continue
				}
				if swept(previous, s.view.Position, p.player.Position, 15) {
					s.hits[key] = true
					s.remaining--
					hit := s.identity + ":" + key
					_, err := w.resolveHit(damage.Request{Weapon: s.weapon, FinalMultiplier: 1}, model.Event{Kind: "hit", Attack: true, Source: s.view.Owner, SourceDefinition: s.weapon.ID, Target: id, HitID: hit, AttackID: s.identity, Tick: w.tick}, nil)
					if err != nil {
						return err
					}
					if s.remaining <= 0 {
						break
					}
				}
			}
		} else {
			// Visibility and trajectory are independent of battle participation.
			// Unauthorized shots pass through without damage or on-hit callbacks.
			owner := w.players[s.view.Owner]
			eligible := owner != nil && owner.player.Scene == w.id
			for i := range w.enemies {
				e := &w.enemies[i]
				id := fmt.Sprint(e.ID)
				key := w.id + ":enemy:" + id
				if !eligible || e.HP <= 0 || s.hits[key] {
					continue
				}
				radius := e.Radius + 4
				if s.view.Kind == "sword" {
					radius += s.view.Radius
				}
				hitStart := previous
				if s.view.Kind == "sword" {
					hitStart = s.view.Position
				}
				if swept(hitStart, s.view.Position, e.Position, radius) && (s.view.Kind != "sword" || w.clearObstacles(s.view.Position, e.Position, 0)) {
					s.hits[key] = true
					s.remaining--
					def := w.catalog.Enemy(e.Definition)
					hit := s.identity + ":" + key
					r, err := w.resolveHit(damage.Request{Source: s.source, Weapon: s.weapon, Attributes: s.attributes, Stats: s.statInput, FlatBonuses: s.flatBonuses, FinalSource: s.finalSource, FinalMultiplier: s.multiplier, Critical: s.critical, CritMultiplier: s.critMultiplier, Effects: s.effects, Defense: damage.Defense{Armor: def.Armor}}, model.Event{Kind: "hit", Attack: s.attack, Source: s.view.Owner, SourceDefinition: s.weapon.ID, Target: id, TargetDefinition: def.ID, AttackID: s.identity, HitID: hit, Tick: w.tick}, e)
					if err != nil {
						return err
					}
					w.lastDamage = &r
					w.addThreat(e.ID, s.view.Owner, r.HealthLoss)
					if s.remaining <= 0 {
						break
					}
				}
			}
		}
		if s.remaining > 0 {
			if w.inside(s.view.Position) {
				kept = append(kept, s)
			} else if w.transit {
				w.outgoing = append(w.outgoing, transferShot(s, previous))
			}
		}
	}
	w.shots = kept
	return nil
}
func swept(a, b, p model.Vec, radius float64) bool {
	d := b.Sub(a)
	t := 0.0
	if n := d.X*d.X + d.Y*d.Y; n > 0 {
		v := p.Sub(a)
		t = max(0, min(1, (v.X*d.X+v.Y*d.Y)/n))
	}
	return a.Add(d.Scale(t)).Sub(p).Length() <= radius
}
func (w *world) inside(v model.Vec) bool {
	if w.footprint != nil {
		return w.footprint.ContainsDisk(v, 0)
	}
	return geometry.NewArena(w.room.Width, w.room.Height).ContainsDisk(v, 0)
}

func (w *world) clipShot(segment geometry.Segment) (geometry.Segment, bool) {
	if w.footprint != nil {
		clipped, ok := w.footprint.ClipSegment(segment)
		return clipped, ok && clipped.B.Sub(segment.B).Length() < 1e-7
	}
	return geometry.NewArena(w.room.Width, w.room.Height).ClipSegment(segment)
}
