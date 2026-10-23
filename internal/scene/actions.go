package scene

import (
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func (a *Actor) Action(id, kind string, slot int, aim model.Vec) error {
	return a.call(command{kind: "action", id: id, player: model.Player{Node: kind, Upgrade: slot}, input: model.Input{Aim: aim}}).err
}
func (a *Actor) Support(p model.Player) error {
	return a.call(command{kind: "support", player: model.ClonePlayer(p)}).err
}
func (a *Actor) Pickup(id, item string) error {
	return a.call(command{kind: "pickup", id: id, player: model.Player{Weapon: item}}).err
}
func (a *Actor) DropItem(id, item string, revision uint64) (model.Player, model.Loot, error) {
	r := a.call(command{kind: "drop_item", id: id, inventory: protocol.Request{Item: item, Revision: revision}})
	if r.err != nil {
		return model.Player{}, model.Loot{}, r.err
	}
	return r.players[0], r.loot, nil
}
func (w *world) action(id, kind string, slot int, aim model.Vec) error {
	p := w.players[id]
	if p == nil {
		return fmt.Errorf("unknown actor")
	}
	var plan content.ActionPlan
	var err error
	switch kind {
	case "weapon_skill":
		plan, err = w.catalog.UseWeaponSkill(p.player)
	case "use_item":
		plan, err = w.catalog.UseItem(p.player, slot)
	default:
		return fmt.Errorf("unknown action")
	}
	if err != nil {
		return err
	}
	if plan.ClearCurse || plan.Support {
		return fmt.Errorf("this item requires expedition context")
	}
	if err := w.applyAction(p.player, aim, plan); err != nil {
		return err
	}
	p.player = plan.Player
	if kind == "weapon_skill" {
		p.playSkill(p.player, aim, plan.ClearProjectiles != nil)
	}
	return nil
}

func (w *world) applyAction(p model.Player, aim model.Vec, plan content.ActionPlan) error {
	direction := aim.Sub(p.Position).Unit()
	if (plan.Projectile != nil || plan.ClearProjectiles != nil) && (!aim.Finite() || direction.Length() == 0) {
		return fmt.Errorf("invalid aim")
	}
	if sweep := plan.ClearProjectiles; sweep != nil {
		kept := w.shots[:0]
		for _, s := range w.shots {
			if s.view.Kind == "enemy" && geometry.SectorContainsDisk(p.Position, direction, s.view.Position, max(4, s.view.Radius), sweep.Range, sweep.Arc) {
				continue
			}
			kept = append(kept, s)
		}
		w.nextID++
		kept = append(kept, shot{view: model.Projectile{ID: w.nextID, Owner: p.ID, Kind: "projectile-clear", Position: p.Position, Velocity: direction, Radius: sweep.Range, Arc: sweep.Arc, TTL: .2}})
		w.shots = kept
	}
	if plan.Projectile != nil {
		w.launch(p.ID, p.Position, direction, *plan.Projectile, false, false)
	}
	return nil
}
