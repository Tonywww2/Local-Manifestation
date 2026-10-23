package scene

import (
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

// Clone excludes borrowed players, input and room-local contact immunity.
func (a PlayerActions) Clone() PlayerActions {
	f := a.state
	if f.player.Life != "" && f.player.Life != "alive" {
		f.attack, f.dodgeTime, f.invulnerable = nil, 0, 0
		f.visual = animation.Cancel(f.visual)
	}
	f.player, f.input, f.contact, f.updated = model.Player{}, model.Input{}, nil, 0
	if f.attack != nil {
		v := *f.attack
		v.request = damage.CloneRequest(v.request)
		f.attack = &v
	}
	return PlayerActions{state: f}
}

func (a PlayerActions) EquipmentLocked(restrict bool) bool {
	return restrict && a.state.cooldown > 1e-9
}

type Departure struct {
	Scene       protocol.Scene
	Actions     map[string]PlayerActions
	Projectiles []ProjectileTransfer
}

func (a *Actor) AddWithActions(p model.Player, position *model.Vec, actions PlayerActions) error {
	if position != nil {
		v := *position
		position = &v
	}
	return a.call(command{kind: "add", player: model.ClonePlayer(p), arrival: position, actions: actions.Clone()}).err
}

func (a *Actor) SupportWithActions(p model.Player, actions PlayerActions) error {
	return a.call(command{kind: "support", player: model.ClonePlayer(p), actions: actions.Clone()}).err
}

// StopWithActions samples all transient state on its owner, then relinquishes it.
func (a *Actor) StopWithActions() Departure {
	r := a.call(command{kind: "stop"})
	<-a.done
	return r.departure
}
