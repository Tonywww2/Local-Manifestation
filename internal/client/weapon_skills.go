package client

import (
	"fmt"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"strings"
)

func (g *Game) weaponSkillControl(p model.Player, owner, id string, bounds ui.Rect) uiControl {
	c := uiControl{Owner: owner, ID: id, Icon: "empty", Bounds: bounds, Key: g.bindingLabel(g.bound("weapon_skill")), Title: g.tr("ui.weapon_skill"), Summary: []string{g.tr("ui.weapon_skill.none")}}
	d, it, ok := g.catalog.EquippedWeaponSkill(p)
	if !ok {
		return c
	}
	c.Icon, c.Title = d.Icon, g.contentName(d.ID, d.Name)
	if c.Icon == "" {
		c.Icon = "weapon"
	}
	c.Summary = g.weaponSkillLines(d)
	c.Action = "weapon-skill-use"
	remaining := float64(it.SkillReady-p.Statuses.Time) / float64(status.UnitsPerSecond)
	if remaining > 0 {
		c.Badge = fmt.Sprintf("%.1f", remaining)
		c.Action = ""
	}
	if p.Life != "alive" {
		c.Action = ""
	}
	if p.Energy < d.Cost {
		c.Action = ""
		c.Summary = append(c.Summary, g.tr("ui.weapon_skill.no_resource"))
	}
	if d.ClearProjectiles != nil {
		c.Details = []string{g.tr("ui.weapon_skill.sector", "range", d.ClearProjectiles.Range, "arc", d.ClearProjectiles.Arc)}
	}
	return c
}

func (g *Game) weaponSkillLines(d model.WeaponSkillDef) []string {
	return []string{g.localized("content."+d.ID+".description", d.Description), g.tr("ui.skill.cost", "cost", d.Cost, "seconds", float64(d.CooldownMillis)/1000)}
}

func (g *Game) growthPreferenceLine(ch model.Chassis) string {
	var parts []string
	for _, key := range model.AttributeKeys() {
		if value, _ := ch.GrowthPreference.Value(key); value > 0 {
			parts = append(parts, g.tr("attribute."+key)+fmt.Sprintf(" +%.0f", value))
		}
	}
	if len(parts) == 0 {
		parts = append(parts, g.tr("ui.growth.none"))
	}
	return g.tr("ui.growth.preference", "values", strings.Join(parts, " · "))
}
