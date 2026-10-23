package client

import (
	"fmt"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
)

func (g *Game) sourceName(s model.SourceRef) string {
	name := g.localized("source.definition."+s.Definition, s.Definition)
	if v, ok := g.catalog.Status(s.Definition); ok {
		name = g.localized(v.NameKey, v.Name)
	}
	if v := g.catalog.Node(s.Definition); v.ID != "" {
		name = g.localized(v.NameKey, v.Name)
	}
	if v, ok := g.catalog.Chassis(s.Definition); ok {
		name = g.contentName(v.ID, v.Name)
	}
	if w := g.catalog.Weapon(s.Definition); w.ID != "" {
		name = g.contentName(w.ID, w.Name)
	}
	if v, ok := g.catalog.Item(s.Definition); ok {
		name = g.contentName(v.ID, v.Name)
	}
	if v, ok := g.catalog.WeaponSkill(s.Definition); ok {
		name = g.contentName(v.ID, v.Name)
	}
	name = g.localized("source.kind."+s.Kind, s.Kind) + ": " + name
	if s.Actor != "" {
		actor := s.Actor
		if g.state != nil {
			for _, p := range g.state.Players {
				if p.ID == s.Actor {
					actor = p.Name
					break
				}
			}
		}
		name += " / " + actor
	}
	return name
}

// Numeric explanations remain data. This adapter only localizes that data.
func (g *Game) damageLines(r damage.Result) []string {
	var lines []string
	if len(r.Segments) > 0 {
		for _, v := range r.Segments {
			lines = append(lines, g.tr("ui.damage.segment", "id", v.ID, "type", g.localized("damage."+v.Damage.Type, v.Damage.Type), "damage", fmt.Sprintf("%.2f", v.Damage.Total)))
			lines = append(lines, g.damageLines(v.Damage)...)
			if r.Settled {
				lines = append(lines, g.tr("ui.damage.settled", "shield", fmt.Sprintf("%.2f", v.Absorbed), "health", fmt.Sprintf("%.2f", v.HealthLoss), "overkill", fmt.Sprintf("%.2f", v.Overkill)))
				for _, s := range v.Shields {
					lines = append(lines, fmt.Sprintf("  %s: −%.2f", g.sourceName(s.Source), s.Amount))
				}
			}
		}
		return lines
	}
	operations := map[string]string{"add": "+", "multiply": "×", "subtract": "−", "minimum": "max", "override": "=", "base": "="}
	for _, st := range r.Steps {
		if b := st.Basis; b.Factor > 0 {
			lines = append(lines, g.tr("ui.damage.append_basis", "base", fmt.Sprintf("%.2f", b.Base), "attributes", fmt.Sprintf("%.2f", b.Attributes), "upgrade", fmt.Sprintf("%.2f", b.Upgrade), "flat", fmt.Sprintf("%.2f", b.Flat), "factor", fmt.Sprintf("%.2f", b.Factor), "total", fmt.Sprintf("%.2f", b.Total()*b.Factor)))
		}
		// Neutral additions/multipliers and inactive defense steps add no information.
		// Keep the base and meaningful effects; the full result remains available.
		if st.Stat != "" && st.Weight == 0 || st.Stat == "" && st.Operation != "base" && st.Input == st.Output && len(st.Members) == 0 {
			continue
		}
		source := g.damageSource(st.Source)
		if st.Group != "" {
			source = g.localized("damage.group."+st.Group, st.Group)
		}
		if st.Origin.Kind != "" {
			source = g.sourceName(st.Origin)
		}
		label := g.tr("ui.damage.step", "input", fmt.Sprintf("%.2f", st.Input), "operation", operations[st.Operation], "operand", fmt.Sprintf("%.2f", st.Operand), "output", fmt.Sprintf("%.2f", st.Output), "source", source)
		if st.Stat != "" {
			label = g.tr("ui.damage.stat", "input", fmt.Sprintf("%.2f", st.Input), "stat", g.tr("attribute."+st.Stat), "score", fmt.Sprintf("%.2f", st.Score), "weight", fmt.Sprintf("%.2f", st.Weight), "output", fmt.Sprintf("%.2f", st.Output))
		}
		lines = append(lines, label)
		for _, attr := range st.AttributeSteps {
			baseOnly := len(attr.Contributions) > 0
			for _, contribution := range attr.Contributions {
				if contribution.Source.Kind != "chassis" {
					baseOnly = false
					break
				}
			}
			if baseOnly {
				continue
			} // Already represented by the attribute score above.
			if attr.Operation != "base" && attr.Input == attr.Output {
				continue
			}
			lines = append(lines, "  "+g.tr("ui.damage.attribute_step", "stat", g.tr("attribute."+attr.Stat), "input", fmt.Sprintf("%.2f", attr.Input), "operation", operations[attr.Operation], "operand", fmt.Sprintf("%.2f", attr.Operand), "output", fmt.Sprintf("%.2f", attr.Output)))
			for _, c := range attr.Contributions {
				v := fmt.Sprintf("%+.2f", c.Value)
				if c.Operation == "percent" {
					v = fmt.Sprintf("%+.2f%%", c.Value*100)
				}
				lines = append(lines, "    "+g.tr("ui.damage.contribution", "id", c.ID, "source", g.sourceName(c.Source), "value", v))
			}
		}
		for _, m := range st.Members {
			label := g.damageSource(m.Source)
			if m.Origin.Kind != "" {
				label += " / " + g.sourceName(m.Origin)
			}
			lines = append(lines, "  "+g.tr("ui.damage.contribution", "id", st.Group, "source", label, "value", fmt.Sprintf("%+.2f%%", m.Percent*100)))
		}
	}
	return lines
}
