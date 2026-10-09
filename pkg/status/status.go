// Package status is the value-only core for independent status layers. Owners
// provide adventure time; this package performs no I/O or clock sampling.
package status

import (
	"errors"
	"fmt"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"math"
	"slices"
	"strconv"
)

// Exact integer steps at both 30 and 60 TPS, and exact millisecond durations.
const UnitsPerSecond int64 = 60000
const MaxTime int64 = 1<<53 - 1
const MaxInstances = 128
const MaxLayers = 32
const MaxTotalLayers = 512

var ErrUnique = errors.New("unique status already applied")

func ValidateDefinition(d model.StatusDef) error {
	if d.ModifierPolicy != "" && d.ModifierPolicy != "strongest" {
		return fmt.Errorf("invalid status modifier policy")
	}
	if math.IsNaN(d.ShieldMaxHP) || math.IsInf(d.ShieldMaxHP, 0) || d.ShieldMaxHP < 0 || d.ShieldMaxHP > 10 {
		return fmt.Errorf("invalid shield scaling")
	}
	if err := validateEffects(d); err != nil {
		return err
	}
	if math.IsNaN(d.Shield) || math.IsInf(d.Shield, 0) || d.Shield < 0 || d.Shield > 1e9 || (d.Shield > 0 || d.ShieldMaxHP > 0) && (d.Level != 0 || d.MaxLayers != 1) {
		return fmt.Errorf("invalid shield definition %s", d.ID)
	}
	if d.ID == "" || len(d.ID) > 128 || d.Name == "" || d.Shield == 0 && d.ShieldMaxHP == 0 && d.Level < 1 || d.Level > 10000 || d.MaxLayers < 1 || d.MaxLayers > MaxLayers || d.DurationMillis < 1 || d.DurationMillis > 86400000 || len(d.Modifiers) > 8 || len(d.Conflicts) > 16 {
		return fmt.Errorf("invalid status definition %s", d.ID)
	}
	seen := map[string]bool{}
	for _, m := range Modifiers(d) {
		if d.ModifierPolicy == "strongest" && (m.Operation != "percent" || m.Value > 0) {
			return fmt.Errorf("strongest policy requires percentage penalties")
		}
		if err := stats.ValidateModifier(m); err != nil {
			return err
		}
		if seen[m.ID] {
			return fmt.Errorf("duplicate status modifier %s", m.ID)
		}
		seen[m.ID] = true
	}
	seen = map[string]bool{}
	for _, id := range d.Conflicts {
		if id == "" || id == d.ID || seen[id] {
			return fmt.Errorf("invalid status conflict")
		}
		seen[id] = true
	}
	return nil
}

func Validate(s model.StatusSet) error {
	if s.Time < 0 || s.Time > MaxTime || s.NextID > uint64(MaxTime) || s.Revision > uint64(MaxTime) || len(s.Instances) > MaxInstances {
		return fmt.Errorf("status limits exceeded")
	}
	seen := map[uint64]bool{}
	total := 0
	for _, v := range s.Instances {
		if v.ModifierPolicy != "" && v.ModifierPolicy != "strongest" {
			return fmt.Errorf("invalid modifier policy snapshot")
		}
		if v.ID == 0 || v.ID > s.NextID || seen[v.ID] || v.Definition == "" || len(v.Definition) > 128 || !v.Shield && v.Level < 1 || v.Shield && (v.Level != 0 || len(v.Layers) != 1) || len(v.Layers) == 0 || len(v.Conflicts) > 16 {
			return fmt.Errorf("invalid status instance")
		}
		seen[v.ID] = true
		if err := stats.ValidateSource(v.Source); err != nil {
			return err
		}
		if v.Source.Actor == "" || v.Source.Instance == "" {
			return fmt.Errorf("status source needs actor and instance")
		}
		var highest uint64
		for _, l := range v.Layers {
			for _, p := range l.Periodic {
				if math.IsNaN(p.HealingMultiplier) || math.IsInf(p.HealingMultiplier, 0) || p.HealingMultiplier < 0 || p.HealingMultiplier > 1e6 {
					return fmt.Errorf("invalid periodic healing multiplier")
				}
				if p.Interval < 6000 || p.Next <= s.Time || math.IsNaN(p.Amount) || math.IsInf(p.Amount, 0) || p.Amount < 0 || p.Amount > 1e9 {
					return fmt.Errorf("invalid periodic state")
				}
			}
			if math.IsNaN(l.ShieldMaxHP) || math.IsInf(l.ShieldMaxHP, 0) || l.ShieldMaxHP < 0 || l.ShieldMaxHP > 1e9 {
				return fmt.Errorf("invalid shield health snapshot")
			}
			if math.IsNaN(l.Shield) || math.IsInf(l.Shield, 0) || math.IsNaN(l.Capacity) || math.IsInf(l.Capacity, 0) || l.Shield < 0 || l.Shield > l.Capacity || l.Capacity > 1e9 || v.Shield && l.Capacity <= 0 || !v.Shield && (l.Shield != 0 || l.Capacity != 0) {
				return fmt.Errorf("invalid shield snapshot")
			}
			total++
			if l.ID == 0 || l.ID > s.NextID || seen[l.ID] || l.ID <= highest || l.Applied < 0 || l.Applied > s.Time || l.Expires <= s.Time || l.Expires > MaxTime || len(l.Modifiers) > 8 {
				return fmt.Errorf("invalid status layer")
			}
			seen[l.ID], highest = true, l.ID
			ids := map[string]bool{}
			for _, m := range l.Modifiers {
				if err := stats.ValidateModifier(m); err != nil {
					return err
				}
				if ids[m.ID] {
					return fmt.Errorf("duplicate layer modifier")
				}
				ids[m.ID] = true
			}
		}
	}
	if total > MaxTotalLayers {
		return fmt.Errorf("status layer budget exceeded")
	}
	return nil
}

func Advance(s model.StatusSet, now int64) (model.StatusSet, bool, error) {
	if now < s.Time || now > MaxTime {
		return model.StatusSet{}, false, fmt.Errorf("invalid adventure time")
	}
	changed := false
	for _, v := range s.Instances {
		for _, l := range v.Layers {
			if l.Expires <= now {
				changed = true
			}
		}
	}
	if !changed {
		s.Time = now
		return s, false, nil
	}
	s = model.CloneStatuses(s)
	for i := range s.Instances {
		s.Instances[i].Layers = slices.DeleteFunc(s.Instances[i].Layers, func(l model.StatusLayer) bool { return l.Expires <= now })
	}
	s.Instances = slices.DeleteFunc(s.Instances, func(v model.StatusInstance) bool { return len(v.Layers) == 0 })
	s.Time, s.Revision = now, s.Revision+1
	return s, true, nil
}

func sameSource(a, b model.SourceRef) bool { return a.Actor == b.Actor && a.Instance == b.Instance }

// Apply snapshots definition values. Expiry is processed before application;
// errors publish no replacement, leaving the caller's original set untouched.
func Apply(s model.StatusSet, d model.StatusDef, source model.SourceRef, now int64) (model.StatusSet, error) {
	return ApplySnapshot(s, d, source, now, model.Attributes{})
}
func ApplySnapshot(s model.StatusSet, d model.StatusDef, source model.SourceRef, now int64, attributes model.Attributes) (model.StatusSet, error) {
	return ApplyShieldSnapshot(s, d, source, now, attributes, 0)
}
func ApplyShieldSnapshot(s model.StatusSet, d model.StatusDef, source model.SourceRef, now int64, attributes model.Attributes, maxHP float64) (model.StatusSet, error) {
	return ApplyEffectSnapshot(s, d, source, now, attributes, maxHP, 1)
}
func ApplyEffectSnapshot(s model.StatusSet, d model.StatusDef, source model.SourceRef, now int64, attributes model.Attributes, maxHP, healing float64) (model.StatusSet, error) {
	if math.IsNaN(healing) || math.IsInf(healing, 0) || healing < 1 || healing > 1e6 {
		return model.StatusSet{}, fmt.Errorf("invalid healing snapshot")
	}
	if math.IsNaN(maxHP) || math.IsInf(maxHP, 0) || maxHP < 0 || maxHP > 1e9 {
		return model.StatusSet{}, fmt.Errorf("invalid shield health snapshot")
	}
	shield := d.Shield + d.ShieldMaxHP*maxHP
	if err := ValidateDefinition(d); err != nil {
		return model.StatusSet{}, err
	}
	if err := stats.ValidateSource(source); err != nil {
		return model.StatusSet{}, err
	}
	if source.Actor == "" || source.Instance == "" {
		return model.StatusSet{}, fmt.Errorf("invalid status source identity")
	}
	var err error
	s, _, err = Advance(s, now)
	if err != nil {
		return model.StatusSet{}, err
	}
	if err = Validate(s); err != nil {
		return model.StatusSet{}, err
	}
	for _, v := range s.Instances {
		if v.Definition == d.ID && d.Unique && v.Source.Actor != source.Actor {
			return model.StatusSet{}, ErrUnique
		}
	}
	s = model.CloneStatuses(s)
	s.Instances = slices.DeleteFunc(s.Instances, func(v model.StatusInstance) bool {
		return slices.Contains(d.Conflicts, v.Definition) || slices.Contains(v.Conflicts, d.ID) ||
			v.Definition == d.ID && d.Unique && !sameSource(v.Source, source) || shield > 0 && v.Shield && sameSource(v.Source, source)
	})
	index := -1
	for i, v := range s.Instances {
		if v.Definition == d.ID && sameSource(v.Source, source) {
			index = i
			break
		}
	}
	if index < 0 {
		s.NextID++
		s.Instances = append(s.Instances, model.StatusInstance{ModifierPolicy: d.ModifierPolicy, ID: s.NextID, Definition: d.ID, Source: source, Shield: shield > 0, Level: d.Level, ClearOnLeave: d.ClearOnLeave, BoundToSource: d.BoundToSource, Conflicts: slices.Clone(d.Conflicts), Controls: slices.Clone(d.Controls), Immunities: slices.Clone(d.Immunities)})
		index = len(s.Instances) - 1
	}
	v := &s.Instances[index]
	if len(v.Layers) == d.MaxLayers {
		// Highest layer means the last surviving layer, never a layer level.
		v.Layers[len(v.Layers)-1].Expires = now + d.DurationMillis*60
	} else {
		s.NextID++
		periodic, err := SnapshotPeriodic(d, attributes, now)
		if err != nil {
			return model.StatusSet{}, err
		}
		for i := range periodic {
			if periodic[i].Kind == "heal" {
				periodic[i].HealingMultiplier = healing
				periodic[i].Amount *= healing
			}
		}
		v.Layers = append(v.Layers, model.StatusLayer{ID: s.NextID, Applied: now, Expires: now + d.DurationMillis*60, Shield: shield, Capacity: shield, ShieldMaxHP: maxHP, Modifiers: Modifiers(d), Periodic: periodic})
	}
	s.Revision++
	if err = Validate(s); err != nil {
		return model.StatusSet{}, err
	}
	return s, nil
}

func RemoveLeaving(s model.StatusSet) (model.StatusSet, bool) {
	return remove(s, func(v model.StatusInstance) bool { return v.ClearOnLeave })
}

func RemoveBoundSource(s model.StatusSet, source model.SourceRef) (model.StatusSet, bool) {
	return remove(s, func(v model.StatusInstance) bool { return v.BoundToSource && sameSource(v.Source, source) })
}

func remove(s model.StatusSet, test func(model.StatusInstance) bool) (model.StatusSet, bool) {
	if !slices.ContainsFunc(s.Instances, test) {
		return s, false
	}
	s = model.CloneStatuses(s)
	s.Instances = slices.DeleteFunc(s.Instances, test)
	s.Revision++
	return s, true
}

func Contributions(s model.StatusSet) []model.StatContribution {
	var out []model.StatContribution
	strongest := map[string]int{}
	for _, v := range s.Instances {
		for _, l := range v.Layers {
			origin := model.SourceRef{Kind: "status", Actor: v.Source.Actor, Instance: strconv.FormatUint(v.ID, 10) + "/" + strconv.FormatUint(l.ID, 10), Definition: v.Definition}
			for _, m := range l.Modifiers {
				if v.ModifierPolicy == "strongest" {
					key := m.Stat + ":" + m.Group
					if index, ok := strongest[key]; ok {
						if m.Value < out[index].Value {
							out[index] = model.StatContribution{StatModifier: m, Source: origin}
						}
						continue
					}
					strongest[key] = len(out)
				}
				out = append(out, model.StatContribution{StatModifier: m, Source: origin})
			}
		}
	}
	return out
}
