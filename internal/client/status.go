package client

import (
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
	"math"
	"strconv"
	"strings"
)

func (g *Game) statusControls(p model.Player) []uiControl {
	var out []uiControl
	evaluated, _ := stats.Evaluate(content.StatInput(p))
	for _, v := range p.Statuses.Instances {
		d, ok := g.catalog.Status(v.Definition)
		if !ok {
			continue
		}
		c := uiControl{Owner: "statuses", ID: "status:" + strconv.FormatUint(v.ID, 10), Icon: "status", Title: g.localized(d.NameKey, d.Name)}
		c.Summary = []string{g.localized(d.DescriptionKey, d.Description), g.tr("ui.status.layers", "count", len(v.Layers))}
		c.Details = []string{g.sourceName(v.Source), g.tr("ui.status.snapshot")}
		earliest := int64(math.MaxInt64)
		for i, l := range v.Layers {
			for index, pulse := range l.Periodic {
				c.Details = append(c.Details, fmt.Sprintf("%s %.2f %s / %.2fs", g.localized("ui.pulse."+pulse.Kind, pulse.Kind), pulse.Amount, g.localized("damage."+pulse.Type, pulse.Type), float64(pulse.Interval)/float64(status.UnitsPerSecond)))
				if index < len(d.Periodic) {
					formula := d.Periodic[index]
					parts := []string{fmt.Sprintf("%.2f", formula.Amount)}
					if formula.PerLevel != 0 && v.Level > 1 {
						parts = append(parts, fmt.Sprintf("%.2f × %d", formula.PerLevel, v.Level-1))
					}
					for _, key := range []string{"strength", "agility", "constitution", "intelligence", "sense", "mana"} {
						weight, _ := formula.Scaling.Value(key)
						score, _ := pulse.Attributes.Value(key)
						if weight != 0 {
							parts = append(parts, fmt.Sprintf("%.2f × %.2f %s", weight, score, g.tr("attribute."+key)))
						}
					}
					c.Details = append(c.Details, "("+strings.Join(parts, " + ")+")"+fmt.Sprintf(" × %.2f = %.2f", max(1, pulse.HealingMultiplier), pulse.Amount))
				}
			}
			remaining := max(0, float64(l.Expires-p.Statuses.Time)/float64(status.UnitsPerSecond))
			c.Summary = append(c.Summary, g.tr("ui.status.layer_time", "layer", i+1, "seconds", fmt.Sprintf("%.2f", remaining)))
			earliest = min(earliest, l.Expires-p.Statuses.Time)
		}
		for _, step := range evaluated.Steps {
			relevant := false
			for _, contribution := range step.Contributions {
				if contribution.Source.Kind == "status" && contribution.Source.Definition == v.Definition && contribution.Source.Actor == v.Source.Actor {
					for _, l := range v.Layers {
						if contribution.Source.Instance == strconv.FormatUint(v.ID, 10)+"/"+strconv.FormatUint(l.ID, 10) {
							relevant = true
						}
					}
				}
			}
			if relevant {
				statName := g.localized("attribute."+step.Stat, step.Stat)
				operation := "+"
				if step.Operation == "multiply" {
					operation = "×"
				}
				c.Details = append(c.Details, g.tr("ui.status.stat_step", "stat", statName, "input", fmt.Sprintf("%.2f", step.Input), "operation", operation, "operand", fmt.Sprintf("%.2f", step.Operand), "output", fmt.Sprintf("%.2f", step.Output)))
			}
		}
		c.Badge = g.tr("ui.status.badge", "level", v.Level, "seconds", int(math.Ceil(float64(max(0, earliest))/float64(status.UnitsPerSecond))))
		if v.Shield {
			c.Badge = fmt.Sprintf("%ds", int(math.Ceil(float64(max(0, earliest))/float64(status.UnitsPerSecond))))
			c.Summary = append(c.Summary, g.tr("ui.shield.balance", "remaining", fmt.Sprintf("%.2f", v.Layers[0].Shield), "capacity", fmt.Sprintf("%.2f", v.Layers[0].Capacity)))
			c.Details = append(c.Details, g.tr("ui.shield.order"))
			if d.ShieldMaxHP > 0 {
				c.Details = append(c.Details, fmt.Sprintf("%.2f × %.2f = %.2f", v.Layers[0].ShieldMaxHP, d.ShieldMaxHP, v.Layers[0].Capacity))
			}
		}
		out = append(out, c)
	}
	return out
}
