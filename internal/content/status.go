package content

import (
	"fmt"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"reflect"
)

func (c *Catalog) knownStatusSource(id string) bool {
	_, w := c.weapons[id]
	_, n := c.nodes[id]
	_, s := c.statuses[id]
	_, e := c.enemies[id]
	_, ch := c.chassis[id]
	_, it := c.items[id]
	_, sk := c.weaponSkills[id]
	_, af := c.affixes[id]
	_, perk := c.perks[id]
	return w || n || s || e || ch || it || sk || af || perk
}

func ApplyStatus(c *Catalog, p model.Player, id string, source model.SourceRef) (model.Player, error) {
	return ApplyStatusSnapshot(c, p, id, source, p.Attributes)
}
func ApplyStatusSnapshot(c *Catalog, p model.Player, id string, source model.SourceRef, attributes model.Attributes) (model.Player, error) {
	if c == nil {
		return model.Player{}, fmt.Errorf("status requires a content catalog")
	}
	d, ok := c.Status(id)
	if !ok {
		return model.Player{}, fmt.Errorf("unknown status %s", id)
	}
	if !c.knownStatusSource(source.Definition) {
		return model.Player{}, fmt.Errorf("unknown status source definition")
	}
	next := p
	var err error
	next.Statuses, err = status.ApplyEffectSnapshot(p.Statuses, d, source, p.Statuses.Time, attributes, p.MaxHP, c.HealingFactor(p))
	if err != nil {
		return model.Player{}, err
	}
	// Expiring a positive layer must not expose an invalid negative factor.
	negative := map[string]float64{}
	for _, m := range StatInput(next).Contributions {
		if m.Operation == "percent" && m.Value < 0 {
			negative[m.Stat+":"+m.Group] += m.Value
		}
	}
	for _, n := range negative {
		if n < -1 {
			return model.Player{}, fmt.Errorf("status expiry could expose an invalid stat factor")
		}
	}
	return RefreshPlayer(next)
}

// AdvancePlayerStatuses avoids recomputing attributes while nothing expires.
func AdvancePlayerStatuses(p model.Player, now int64) (model.Player, error) {
	return advancePlayerStatuses(p, now, nil, nil)
}
func (c *Catalog) AdvancePlayerStatuses(p model.Player, now int64) (model.Player, error) {
	return advancePlayerStatuses(p, now, c.PlayerDefense, c)
}
func advancePlayerStatuses(p model.Player, now int64, defenses func(model.Player) map[string]damage.Defense, c *Catalog) (model.Player, error) {
	next := p
	if now < p.ResourceTime {
		return model.Player{}, fmt.Errorf("resource clock moved backwards")
	}
	next.Energy = min(p.MaxEnergy, p.Energy+p.EnergyRegen*float64(now-p.ResourceTime)/float64(status.UnitsPerSecond))
	next.ResourceTime = now
	var changed bool
	var err error
	var pulses []status.Pulse
	next.Statuses, pulses, err = status.Pulses(next.Statuses, now)
	if err != nil {
		return model.Player{}, err
	}
	for _, pulse := range pulses {
		if next.Life != "alive" {
			break
		}
		if pulse.Kind == "heal" {
			next.HP = damage.Round(min(next.MaxHP, next.HP+pulse.Amount*float64(pulse.Count)))
			continue
		}
		for i := int64(0); i < pulse.Count && next.Life == "alive"; i++ {
			q := damage.Request{Weapon: model.Weapon{ID: pulse.Definition, Type: pulse.Type, Base: pulse.Amount}, Defense: damage.Defense{Armor: next.Armor, Immune: status.Immune(next.Statuses, pulse.Type)}, FinalMultiplier: 1}
			if defenses != nil {
				q.TypedDefense = defenses(next)
			}
			r, e := damage.Evaluate(q)
			if e != nil {
				return model.Player{}, e
			}
			if c != nil {
				_, next, e = c.SettlePlayerDamage(r, next)
			} else {
				_, next.HP, next.Statuses, e = damage.Settle(r, next.HP, next.Statuses)
			}
			if e != nil {
				return model.Player{}, e
			}
			if next.HP == 0 {
				next, e = life.Incapacitate(next)
				if e != nil {
					return model.Player{}, e
				}
			}
		}
	}
	next.Statuses, changed, err = status.Advance(next.Statuses, now)
	if err != nil {
		return model.Player{}, err
	}
	if changed {
		return RefreshPlayer(next)
	}
	return next, nil
}

func LeaveStatuses(p model.Player) (model.Player, error) {
	next := p
	var changed bool
	next.Statuses, changed = status.RemoveLeaving(p.Statuses)
	if changed {
		return RefreshPlayer(next)
	}
	return next, nil
}

func ValidatePlayerStatuses(c *Catalog, p model.Player) error {
	if err := status.Validate(p.Statuses); err != nil {
		return err
	}
	for i, v := range p.Statuses.Instances {
		if !c.knownStatusSource(v.Source.Definition) {
			return fmt.Errorf("invalid saved status source")
		}
		d, ok := c.Status(v.Definition)
		if !ok || v.ModifierPolicy != d.ModifierPolicy || !reflect.DeepEqual(v.Controls, d.Controls) || !reflect.DeepEqual(v.Immunities, d.Immunities) || v.Shield != (d.Shield > 0 || d.ShieldMaxHP > 0) || v.Level != d.Level || v.ClearOnLeave != d.ClearOnLeave || v.BoundToSource != d.BoundToSource || !reflect.DeepEqual(v.Conflicts, d.Conflicts) || len(v.Layers) > d.MaxLayers {
			return fmt.Errorf("invalid status definition snapshot")
		}
		for _, l := range v.Layers {
			roomLifetime := d.DurationMillis == 0 && d.ClearOnLeave && l.Expires == status.MaxTime
			if l.Capacity != d.Shield+d.ShieldMaxHP*l.ShieldMaxHP || !reflect.DeepEqual(l.Modifiers, status.Modifiers(d)) || len(l.Periodic) != len(d.Periodic) || !roomLifetime && l.Expires-p.Statuses.Time > d.DurationMillis*60 {
				return fmt.Errorf("invalid status effect snapshot")
			}
			for i, pulse := range l.Periodic {
				expected := d.Periodic[i]
				amount := expected.Amount + expected.PerLevel*float64(max(0, d.Level-1)) + expected.Scaling.Dot(pulse.Attributes)
				if pulse.Kind == "heal" {
					amount *= max(1, pulse.HealingMultiplier)
				}
				if amount != pulse.Amount {
					return fmt.Errorf("periodic calculation snapshot mismatch")
				}
				if pulse.Kind != expected.Kind || pulse.Type != expected.Type || pulse.Interval != expected.IntervalMillis*60 || pulse.Next < l.Applied || (pulse.Next-l.Applied)%pulse.Interval != 0 {
					return fmt.Errorf("invalid periodic snapshot")
				}
			}
		}
		for _, other := range p.Statuses.Instances[:i] {
			if other.Definition == v.Definition && (d.Unique || other.Source.Actor == v.Source.Actor && other.Source.Instance == v.Source.Instance) {
				return fmt.Errorf("duplicate status source or unique status")
			}
			for _, conflict := range other.Conflicts {
				if conflict == v.Definition {
					return fmt.Errorf("conflicting saved statuses")
				}
			}
			for _, conflict := range v.Conflicts {
				if conflict == other.Definition {
					return fmt.Errorf("conflicting saved statuses")
				}
			}
		}
	}
	return nil
}
