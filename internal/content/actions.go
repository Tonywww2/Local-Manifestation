package content

import (
	"fmt"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"math"
)

type ActionPlan struct {
	Player           model.Player
	ClearProjectiles *model.ProjectileClearDef
	Projectile       *damage.Request
	ClearCurse       bool
	Support          bool
}

func (c *Catalog) actionEffects(p model.Player, effects []model.ItemEffect, source model.SourceRef) (ActionPlan, error) {
	plan := ActionPlan{Player: model.ClonePlayer(p)}
	for _, e := range effects {
		var err error
		switch e.Kind {
		case "heal":
			plan.Player.HP = damage.Round(min(plan.Player.MaxHP, plan.Player.HP+c.HealingAmount(plan.Player, e.Amount+e.Scaling.Dot(p.Attributes), false)))
		case "apply_status":
			plan.Player, err = ApplyStatus(c, plan.Player, e.Status, source)
		case "remove_penalty":
			plan.Player, err = ReducePenalty(plan.Player, int(e.Amount))
			if err == nil {
				plan.Player.HP = plan.Player.MaxHP
			}
		case "clear_curse":
			plan.ClearCurse = true
		case "support":
			plan.Support = true
		default:
			err = fmt.Errorf("unknown action effect")
		}
		if err != nil {
			return ActionPlan{}, err
		}
	}
	return plan, nil
}
func (c *Catalog) UseItem(p model.Player, slot int) (ActionPlan, error) {
	if p.Life != "alive" || slot < 0 || slot > 3 {
		return ActionPlan{}, fmt.Errorf("invalid consumable action")
	}
	it, ok := inventory.Find(p.Inventory, p.Inventory.Equipment[5+slot])
	if !ok {
		return ActionPlan{}, fmt.Errorf("empty consumable slot")
	}
	d, _ := c.Item(it.Definition)
	plan, err := c.actionEffects(p, d.Use, model.SourceRef{Kind: "item", Actor: p.ID, Definition: d.ID, Instance: it.ID})
	if err != nil {
		return ActionPlan{}, err
	}
	plan.Player.Inventory, err = inventory.Consume(plan.Player.Inventory, it.ID, c.Item)
	if err != nil {
		return ActionPlan{}, err
	}
	plan.Player, err = c.RefreshEquipment(plan.Player)
	if err != nil {
		return ActionPlan{}, err
	}
	return plan, nil
}
func validateActionScaling(e model.ItemEffect) error {
	for _, n := range []float64{e.Scaling.Strength, e.Scaling.Agility, e.Scaling.Constitution, e.Scaling.Intelligence, e.Scaling.Sense, e.Scaling.Mana} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e6 || e.Kind != "heal" && n != 0 {
			return fmt.Errorf("invalid effect scaling")
		}
	}
	return nil
}
