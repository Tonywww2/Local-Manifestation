package scene

import (
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
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
func (w *world) action(id, kind string, slot int, aim model.Vec) error {
	p := w.players[id]
	if p == nil {
		return fmt.Errorf("unknown actor")
	}
	var plan content.ActionPlan
	var err error
	if kind == "skill" {
		plan, err = w.catalog.UseSkill(p.player, slot)
	} else {
		plan, err = w.catalog.UseItem(p.player, slot)
	}
	if err != nil {
		return err
	}
	if plan.ClearCurse || plan.Support {
		return fmt.Errorf("this item requires expedition context")
	}
	if plan.Projectile != nil {
		dir := aim.Sub(p.player.Position).Unit()
		if !aim.Finite() || dir.Length() == 0 {
			return fmt.Errorf("invalid aim")
		}
		q := *plan.Projectile
		w.launch(id, p.player.Position, dir, q, false, false)
	}
	p.player = plan.Player
	return nil
}
