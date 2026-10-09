package status

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"math"
	"slices"
)

type Pulse struct {
	Source                 model.SourceRef
	Definition, Kind, Type string
	Amount                 float64
	Count                  int64
}

func Modifiers(d model.StatusDef) []model.StatModifier {
	out := slices.Clone(d.Modifiers)
	for _, m := range d.LevelModifiers {
		m.Value *= float64(max(0, d.Level-1))
		out = append(out, m)
	}
	return out
}
func SnapshotPeriodic(d model.StatusDef, attributes model.Attributes, now int64) ([]model.PeriodicSnapshot, error) {
	var out []model.PeriodicSnapshot
	for _, v := range d.Periodic {
		for _, n := range []float64{v.Scaling.Strength, v.Scaling.Agility, v.Scaling.Constitution, v.Scaling.Intelligence, v.Scaling.Sense, v.Scaling.Mana} {
			if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e6 {
				return nil, fmt.Errorf("invalid periodic scaling")
			}
		}
		amount := v.Amount + v.PerLevel*float64(max(0, d.Level-1)) + v.Scaling.Dot(attributes)
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > 1e9 {
			return nil, fmt.Errorf("invalid periodic snapshot")
		}
		interval := v.IntervalMillis * 60
		out = append(out, model.PeriodicSnapshot{Attributes: attributes, Kind: v.Kind, Type: v.Type, Amount: amount, Interval: interval, Next: now + interval})
	}
	return out, nil
}

// Pulses advances independent cursors before expiry. Catch-up count is explicit;
// the owner chooses how to settle repeated hits without losing armor applications.
func Pulses(s model.StatusSet, now int64) (model.StatusSet, []Pulse, error) {
	if now < s.Time || now > MaxTime {
		return model.StatusSet{}, nil, fmt.Errorf("invalid pulse time")
	}
	var out []Pulse
	changed := false
	for _, v := range s.Instances {
		for _, l := range v.Layers {
			for _, p := range l.Periodic {
				if p.Next <= min(now, l.Expires-1) {
					changed = true
				}
			}
		}
	}
	if !changed {
		return s, nil, nil
	}
	s = model.CloneStatuses(s)
	for i := range s.Instances {
		v := &s.Instances[i]
		for j := range v.Layers {
			l := &v.Layers[j]
			for k := range l.Periodic {
				p := &l.Periodic[k]
				limit := min(now, l.Expires-1)
				if p.Interval <= 0 {
					return model.StatusSet{}, nil, fmt.Errorf("invalid periodic interval")
				}
				if p.Next <= limit {
					count := 1 + (limit-p.Next)/p.Interval
					out = append(out, Pulse{v.Source, v.Definition, p.Kind, p.Type, p.Amount, count})
					p.Next += count * p.Interval
				}
			}
		}
	}
	s.Revision++
	return s, out, nil
}
func Controlled(s model.StatusSet, control string) bool {
	for _, v := range s.Instances {
		if slices.Contains(v.Controls, control) {
			return true
		}
	}
	return false
}
func Immune(s model.StatusSet, kind string) bool {
	for _, v := range s.Instances {
		if slices.Contains(v.Immunities, "all") || slices.Contains(v.Immunities, kind) {
			return true
		}
	}
	return false
}
func validateEffects(d model.StatusDef) error {
	if len(d.Periodic) > 4 || len(d.Controls) > 4 || len(d.Immunities) > 7 || len(d.LevelModifiers)+len(d.Modifiers) > 8 {
		return fmt.Errorf("status effect budget")
	}
	for _, v := range d.Controls {
		if v != "stun" && v != "root" && v != "silence" {
			return fmt.Errorf("unknown control")
		}
	}
	for _, v := range d.Immunities {
		if v != "all" && v != "physical" && v != "fire" && v != "cold" && v != "electromagnetic" && v != "chemical" && v != "magic" {
			return fmt.Errorf("unknown immunity")
		}
	}
	for _, v := range d.Periodic {
		if v.Kind != "damage" && v.Kind != "heal" || v.Kind == "damage" && v.Type != "physical" && v.Type != "fire" && v.Type != "cold" && v.Type != "electromagnetic" && v.Type != "chemical" && v.Type != "magic" || v.IntervalMillis < 100 || v.IntervalMillis > 86400000 || math.IsNaN(v.Amount) || math.IsInf(v.Amount, 0) || v.Amount < 0 || v.Amount > 1e6 || math.IsNaN(v.PerLevel) || math.IsInf(v.PerLevel, 0) || v.PerLevel < 0 || v.PerLevel > 1e6 {
			return fmt.Errorf("invalid periodic definition")
		}
		if _, err := SnapshotPeriodic(d, model.Attributes{}, 0); err != nil {
			return err
		}
	}
	return nil
}
