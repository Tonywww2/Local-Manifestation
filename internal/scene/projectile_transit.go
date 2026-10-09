package scene

import (
	"fmt"
	"maps"

	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
)

// ProjectileTransfer is a detached value. Its captured formula and hit history
// are private; only the receiving simulation can mutate them.
type ProjectileTransfer struct {
	state    shot
	previous model.Vec
}

func transferShot(s shot, previous model.Vec) ProjectileTransfer {
	s.hits = maps.Clone(s.hits)
	s.weapon = model.CloneWeapon(s.weapon)
	q := damage.CloneRequest(damage.Request{Effects: s.effects, Stats: s.statInput, FlatBonuses: s.flatBonuses})
	s.effects = q.Effects
	s.statInput, s.flatBonuses, s.sweepStart = q.Stats, q.FlatBonuses, nil
	return ProjectileTransfer{state: s, previous: previous}
}

func (p ProjectileTransfer) Path() geometry.Segment {
	return geometry.Segment{A: p.previous, B: p.state.view.Position}
}

func (p ProjectileTransfer) Reframe(from, to model.Node, a, b geometry.Arena) ProjectileTransfer {
	p.previous = geometry.TransferPoint(from, to, a, b, p.previous)
	p.state.view.Position = geometry.TransferPoint(from, to, a, b, p.state.view.Position)
	p.state.view.Velocity = p.state.view.Velocity.Scale(b.Radius / a.Radius)
	p.state.view.Radius *= b.Radius / a.Radius
	return p
}

func (w *world) receiveProjectile(p ProjectileTransfer) {
	s := transferShot(p.state, p.previous).state
	w.nextID++ // display IDs stay unique in this room; attack identity stays stable
	s.view.ID = w.nextID
	v := p.previous
	s.sweepStart = &v
	w.shots = append(w.shots, s)
}

func (a *Actor) EnableProjectileTransit() { a.call(command{kind: "projectile_transit"}) }
func (a *Actor) TakeProjectiles() []ProjectileTransfer {
	return a.call(command{kind: "take_projectiles"}).transfers
}
func (a *Actor) ReceiveProjectiles(p []ProjectileTransfer) error {
	copy := make([]ProjectileTransfer, len(p))
	for i, v := range p {
		copy[i] = transferShot(v.state, v.previous)
	}
	return a.call(command{kind: "receive_projectiles", transfers: copy}).err
}

func (a *Actor) SweepProjectiles(p ProjectileTransfer) ([]ProjectileTransfer, error) {
	r := a.call(command{kind: "sweep_projectiles", transfers: []ProjectileTransfer{transferShot(p.state, p.previous)}})
	return r.transfers, r.err
}
func (s *FreeSpace) EnableProjectileTransit() { s.world.transit = true }
func (s *FreeSpace) TakeProjectiles() []ProjectileTransfer {
	p := s.world.outgoing
	s.world.outgoing = nil
	return p
}
func (s *FreeSpace) ReleaseProjectiles() []ProjectileTransfer {
	p := s.TakeProjectiles()
	for _, shot := range s.world.shots {
		previous := shot.view.Position
		if shot.sweepStart != nil {
			previous = *shot.sweepStart
		}
		p = append(p, transferShot(shot, previous))
	}
	s.world.shots = nil
	return p
}
func (s *FreeSpace) ReceiveProjectiles(p []ProjectileTransfer) {
	for _, v := range p {
		s.world.receiveProjectile(v)
	}
}

func (w *world) shotIdentity(s shot) string {
	if s.identity != "" {
		return s.identity
	}
	origin := w.shotOrigin
	if origin == "" {
		origin = w.id
	}
	return fmt.Sprintf("%s:shot:%d", origin, s.view.ID)
}
