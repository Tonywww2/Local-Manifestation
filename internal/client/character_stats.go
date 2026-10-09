package client

import (
	"fmt"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/progression"
	"hsmultiplayer/pkg/status"
)

func (g *Game) buildCharacterStats(f *uiFrame, layout ui.Layout) {
	p := g.self()
	if p == nil {
		return
	}
	const id = "self"
	r := g.windowPanel(f, layout, id, g.tr("ui.character.attributes"), ui.Size{W: 490, H: 620})
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 42, W: r.W - 28, H: 24}, p.Name+" · "+g.tr("ui.level.short", "level", p.Level))
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 70, W: r.W - 28, H: 24}, g.tr("ui.points", "count", p.Points))
	column := (r.W - 28) / 2
	for i, key := range []string{"strength", "agility", "constitution", "intelligence", "sense", "mana"} {
		value, _ := p.Attributes.Value(key)
		bounds := ui.Rect{X: r.X + 14 + float64(i%2)*column, Y: r.Y + 108 + float64(i/2)*48, W: column - 10, H: 38}
		g.addLabel(f, id, ui.Rect{X: bounds.X + 8, Y: bounds.Y + 7, W: bounds.W - 46, H: 24}, fmt.Sprintf("%s  %.2f", g.tr("attribute."+key), value))
		f.Controls = append(f.Controls, uiControl{ID: "attribute-info:" + key, Owner: id, Icon: "", Bounds: bounds, Title: g.tr("attribute." + key), Summary: []string{fmt.Sprintf("%.2f", value)}, Details: g.attributeDetailLines(key)})
		if p.Points > 0 {
			addControl(f, id, "stat:"+key, "plus", "", "attribute", key, g.tr("ui.character.invest", "attribute", g.tr("attribute."+key)), ui.Rect{X: bounds.X + bounds.W - 30, Y: bounds.Y + 5, W: 28, H: 28})
		}
	}
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 252, W: r.W - 28, H: 24}, g.tr("ui.character.derived"))
	lines := g.u.characterDetails
	if g.u.expanded && g.u.focus == id {
		lines = append(append([]string(nil), lines...), g.tr("ui.character.calculation"))
		for _, key := range []string{"strength", "agility", "constitution", "intelligence", "sense", "mana", "max_hp", "armor", "speed", "attack_speed"} {
			lines = append(lines, g.attributeDetailLines(key)...)
		}
		lines = append(lines, g.u.previewLines...)
	}
	body := r
	body.Y += 238
	body.H -= 238
	g.addWindowLines(f, id, body, lines, max(20, body.H-72))
}

func (g *Game) attributeDetailLines(key string) []string {
	var lines []string
	for _, step := range g.u.statPreview.Steps {
		if step.Stat != key {
			continue
		}
		operation := "+"
		if step.Operation == "multiply" {
			operation = "×"
		}
		name := g.localized("attribute."+key, key)
		switch step.Operation {
		case "base":
			lines = append(lines, g.tr("ui.character.base", "stat", name, "value", fmt.Sprintf("%.2f", step.Output)))
		case "minimum":
			lines = append(lines, fmt.Sprintf("%s: max(%.2f, %.2f) = %.2f", name, step.Input, step.Operand, step.Output))
		default:
			lines = append(lines, g.tr("ui.status.stat_step", "stat", name, "input", fmt.Sprintf("%.2f", step.Input), "operation", operation, "operand", fmt.Sprintf("%.2f", step.Operand), "output", fmt.Sprintf("%.2f", step.Output)))
		}
		for _, c := range step.Contributions {
			lines = append(lines, g.sourceName(c.Source))
		}
	}
	return lines
}

// The snapshot and content catalog are read-only inputs. This display projection
// is cached when a snapshot or language changes, never evaluated by Draw.
func (g *Game) characterDetailLines(p model.Player, values model.StatValues, criticalMultiplier float64) []string {
	line := func(key string, value float64) string {
		return g.tr(key) + "  " + fmt.Sprintf("%.2f", value)
	}
	lines := []string{
		fmt.Sprintf("%s  %.2f / %.2f", g.tr("ui.character.health"), p.HP, p.MaxHP),
		line("attribute.armor", p.Armor), line("attribute.speed", p.Speed),
		g.tr("ui.character.attack_speed", "factor", fmt.Sprintf("%.2f", values.AttackSpeed)),
		fmt.Sprintf("%s  %.2f / %.2f", g.tr("ui.character.energy"), p.Energy, p.MaxEnergy),
		line("ui.character.energy_regen", p.EnergyRegen),
		line("ui.character.shield", status.ShieldTotal(p.Statuses)),
		g.tr("ui.character.healing", "factor", fmt.Sprintf("%.2f", g.catalog.HealingFactor(p))),
		g.tr("ui.xp.progress", "xp", p.XP, "next", progression.NextLevelXP(p.Level)),
		g.tr("ui.shop.balance", "coins", p.Coins),
		g.tr("ui.ap.personal", "value", p.AP), g.tr("life." + p.Life),
		g.tr("ui.status.downs.record", "count", p.Knockdowns),
		g.tr("ui.status.downs.current", "count", p.Knockdowns-p.KnockdownBase),
		g.tr("ui.character.penalty", "layers", p.Downs, "factor", fmt.Sprintf("%.2f", life.PenaltyFactor(p.Downs))),
	}
	if criticalMultiplier > 0 {
		lines = append(lines, g.tr("ui.character.critical", "factor", fmt.Sprintf("%.2f", criticalMultiplier*100)))
	}
	if g.u.previewValid {
		lines = append(lines, g.tr("ui.character.damage", "damage", fmt.Sprintf("%.2f", g.u.preview.Total)))
	}
	defenses := g.catalog.PlayerDefense(p)
	for _, kind := range content.DamageTypes {
		d := defenses[kind]
		factor := 1 - d.Resistance
		for _, reduction := range d.Reductions {
			factor *= 1 - reduction
		}
		lines = append(lines, g.tr("ui.character.defense", "type", g.localized("damage."+kind, kind), "resistance", fmt.Sprintf("%.2f", d.Resistance*100), "reduction", fmt.Sprintf("%.2f", (1-factor)*100)))
		if d.Immune {
			lines = append(lines, g.tr("ui.character.immune", "type", g.localized("damage."+kind, kind)))
		}
	}
	return lines
}
