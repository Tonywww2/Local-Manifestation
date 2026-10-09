package content

import (
	"fmt"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"math"
)

type ActionPlan struct {
	Player     model.Player
	Projectile *damage.Request
	ClearCurse bool
	Support    bool
}

func (c *Catalog) Skill(id string) (model.SkillDef, bool) {
	d, ok := c.skills[id]
	d.Effects = append([]model.ItemEffect(nil), d.Effects...)
	return d, ok
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
func (c *Catalog) UseSkill(p model.Player, slot int) (ActionPlan, error) {
	if status.Controlled(p.Statuses, "stun") || status.Controlled(p.Statuses, "silence") {
		return ActionPlan{}, fmt.Errorf("skill blocked by control")
	}
	if p.Life != "alive" || slot < 0 || slot > 3 {
		return ActionPlan{}, fmt.Errorf("invalid skill action")
	}
	d, ok := c.Skill(p.Skills[slot])
	if !ok {
		return ActionPlan{}, fmt.Errorf("empty skill slot")
	}
	if p.Statuses.Time < p.SkillReady[slot] {
		return ActionPlan{}, fmt.Errorf("skill cooling down")
	}
	if p.Energy < d.Cost {
		return ActionPlan{}, fmt.Errorf("insufficient resource")
	}
	plan, err := c.actionEffects(p, d.Effects, model.SourceRef{Kind: "skill", Actor: p.ID, Definition: d.ID, Instance: fmt.Sprintf("skill:%d", slot)})
	if err != nil {
		return ActionPlan{}, err
	}
	plan.Player.Energy = damage.Round(plan.Player.Energy - d.Cost)
	plan.Player.SkillReady[slot] = p.Statuses.Time + d.CooldownMillis*60
	if d.Weapon != "" {
		attacker := model.ClonePlayer(plan.Player)
		attacker.Weapon = d.Weapon
		attacker.WeaponBonus = 0
		q, err := Attack(c, attacker)
		if err != nil {
			return ActionPlan{}, err
		}
		q.Source = model.SourceRef{Kind: "skill", Actor: p.ID, Definition: d.ID, Instance: fmt.Sprintf("skill:%d", slot)}
		plan.Projectile = &q
	}
	return plan, nil
}
func (c *Catalog) validateSkills(claim func(string) bool) error {
	c.skills = map[string]model.SkillDef{}
	for _, v := range []float64{c.definitions.Resource.Maximum, c.definitions.Resource.Regeneration} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1e6 {
			return fmt.Errorf("invalid resource")
		}
	}
	for _, d := range c.definitions.Skills {
		if !claim(d.ID) || d.Name == "" || math.IsNaN(d.Cost) || math.IsInf(d.Cost, 0) || d.Cost < 0 || d.Cost > 1e6 || d.CooldownMillis < 1 || d.CooldownMillis > 86400000 {
			return fmt.Errorf("invalid skill")
		}
		if d.Weapon != "" && c.weapons[d.Weapon].ID == "" {
			return fmt.Errorf("unknown skill weapon")
		}
		for _, e := range d.Effects {
			if err := validateActionScaling(e); err != nil {
				return err
			}
			if e.Kind != "heal" && e.Kind != "apply_status" && e.Kind != "remove_penalty" {
				return fmt.Errorf("invalid skill effect")
			}
			if e.Kind == "apply_status" {
				if _, ok := c.statuses[e.Status]; !ok {
					return fmt.Errorf("unknown skill status")
				}
			}
			if math.IsNaN(e.Amount) || math.IsInf(e.Amount, 0) || e.Amount < 0 || e.Amount > 1e6 {
				return fmt.Errorf("invalid skill amount")
			}
		}
		c.skills[d.ID] = d
	}
	return nil
}
func validateActionScaling(e model.ItemEffect) error {
	for _, n := range []float64{e.Scaling.Strength, e.Scaling.Agility, e.Scaling.Constitution, e.Scaling.Intelligence, e.Scaling.Sense, e.Scaling.Mana} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e6 || e.Kind != "heal" && n != 0 {
			return fmt.Errorf("invalid effect scaling")
		}
	}
	return nil
}
