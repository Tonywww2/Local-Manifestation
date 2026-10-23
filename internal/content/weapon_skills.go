package content

import (
	"fmt"
	"math"

	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/status"
)

func (c *Catalog) WeaponSkill(id string) (model.WeaponSkillDef, bool) {
	d, ok := c.weaponSkills[id]
	d.Effects = append([]model.ItemEffect(nil), d.Effects...)
	if d.ClearProjectiles != nil {
		v := *d.ClearProjectiles
		d.ClearProjectiles = &v
	}
	return d, ok
}

func (c *Catalog) EquippedWeaponSkill(p model.Player) (model.WeaponSkillDef, model.ItemInstance, bool) {
	it, ok := inventory.Find(p.Inventory, p.Inventory.Equipment[0])
	if !ok {
		return model.WeaponSkillDef{}, it, false
	}
	d, _ := c.Item(it.Definition)
	skill, found := c.WeaponSkill(c.weapons[d.Weapon].Skill)
	return skill, it, found
}

// The item instance owns its cooldown; unequipping or trading cannot reset it.
// The host validates aim before publishing this detached action plan.
func (c *Catalog) UseWeaponSkill(p model.Player) (ActionPlan, error) {
	if p.Life != "alive" || status.Controlled(p.Statuses, "stun") || status.Controlled(p.Statuses, "silence") {
		return ActionPlan{}, protocol.Reject("error.weapon_skill_blocked")
	}
	d, it, ok := c.EquippedWeaponSkill(p)
	if !ok {
		return ActionPlan{}, protocol.Reject("error.weapon_skill_none")
	}
	if p.Statuses.Time < it.SkillReady {
		return ActionPlan{}, protocol.Reject("error.weapon_skill_cooldown")
	}
	if p.Energy < d.Cost {
		return ActionPlan{}, protocol.Reject("error.weapon_skill_resource")
	}
	source := model.SourceRef{Kind: "weapon_skill", Actor: p.ID, Definition: d.ID, Instance: it.ID}
	plan, err := c.actionEffects(p, d.Effects, source)
	if err != nil {
		return ActionPlan{}, err
	}
	plan.Player.Energy = damage.Round(plan.Player.Energy - d.Cost)
	for i := range plan.Player.Inventory.Items {
		if plan.Player.Inventory.Items[i].ID == it.ID {
			plan.Player.Inventory.Items[i].SkillReady = p.Statuses.Time + d.CooldownMillis*(status.UnitsPerSecond/1000)
		}
	}
	plan.Player.Inventory.Revision++
	plan.ClearProjectiles = d.ClearProjectiles
	if d.Weapon != "" {
		attacker := model.ClonePlayer(plan.Player)
		attacker.Weapon, attacker.WeaponBonus = d.Weapon, 0
		q, err := Attack(c, attacker)
		if err != nil {
			return ActionPlan{}, err
		}
		q.Source = source
		plan.Projectile = &q
	}
	return plan, nil
}

func (c *Catalog) validateWeaponSkills(claim func(string) bool) error {
	c.weaponSkills = map[string]model.WeaponSkillDef{}
	valid := func(v, low, high float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= low && v <= high }
	if !valid(c.definitions.Resource.Maximum, 0, 1e6) || !valid(c.definitions.Resource.Regeneration, 0, 1e6) {
		return fmt.Errorf("invalid resource")
	}
	for _, d := range c.definitions.WeaponSkills {
		if !claim(d.ID) || d.Name == "" || !valid(d.Cost, 0, 1e6) || d.CooldownMillis < 1 || d.CooldownMillis > 86400000 || len(d.Effects) > 32 {
			return fmt.Errorf("invalid weapon skill")
		}
		if d.Weapon != "" && c.weapons[d.Weapon].ID == "" {
			return fmt.Errorf("unknown skill projectile weapon")
		}
		if d.Weapon == "" && d.ClearProjectiles == nil && len(d.Effects) == 0 {
			return fmt.Errorf("empty weapon skill")
		}
		if v := d.ClearProjectiles; v != nil && (!valid(v.Range, 1, 4096) || !valid(v.Arc, 1, 360)) {
			return fmt.Errorf("invalid projectile clear sector")
		}
		for _, e := range d.Effects {
			if err := validateActionScaling(e); err != nil {
				return err
			}
			if e.Kind != "heal" && e.Kind != "apply_status" && e.Kind != "remove_penalty" {
				return fmt.Errorf("invalid weapon skill effect")
			}
			if e.Kind == "apply_status" && c.statuses[e.Status].ID == "" {
				return fmt.Errorf("unknown weapon skill status")
			}
			if !valid(e.Amount, 0, 1e6) {
				return fmt.Errorf("invalid weapon skill amount")
			}
		}
		c.weaponSkills[d.ID] = d
	}
	for _, w := range c.definitions.Weapons {
		if w.Skill != "" && c.weaponSkills[w.Skill].ID == "" {
			return fmt.Errorf("unknown weapon skill on %s", w.ID)
		}
	}
	return nil
}
