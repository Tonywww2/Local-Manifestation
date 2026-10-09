// Package stats is a pure, source-aware evaluator shared by gameplay and explanations.
package stats

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"hsmultiplayer/pkg/model"
)

type Input struct {
	Base          model.StatValues
	BaseSource    model.SourceRef
	Contributions []model.StatContribution
}
type Step struct {
	Stat          string                   `json:"stat"`
	Operation     string                   `json:"operation"`
	Group         string                   `json:"group,omitempty"`
	Input         float64                  `json:"input"`
	Operand       float64                  `json:"operand"`
	Output        float64                  `json:"output"`
	Contributions []model.StatContribution `json:"contributions,omitempty"`
}
type Result struct {
	Values model.StatValues `json:"values"`
	Steps  []Step           `json:"steps"`
}

var fields = [...]string{"strength", "agility", "constitution", "intelligence", "sense", "mana", "max_hp", "armor", "speed", "attack_speed"}

func value(v model.StatValues, stat string) (float64, bool) {
	switch stat {
	case "attack_speed":
		return v.AttackSpeed, true
	case "max_hp":
		return v.MaxHP, true
	case "armor":
		return v.Armor, true
	case "speed":
		return v.Speed, true
	default:
		return v.Attributes.Value(stat)
	}
}
func with(v model.StatValues, stat string, n float64) model.StatValues {
	switch stat {
	case "attack_speed":
		v.AttackSpeed = n
	case "max_hp":
		v.MaxHP = n
	case "armor":
		v.Armor = n
	case "speed":
		v.Speed = n
	default:
		v.Attributes, _ = v.Attributes.With(stat, n)
	}
	return v
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) && math.Abs(n) <= 1e100 }
func ValidateModifier(c model.StatModifier) error {
	if _, ok := value(model.StatValues{}, c.Stat); !ok || c.ID == "" || len(c.ID) > 128 || !finite(c.Value) || math.Abs(c.Value) > 1e9 {
		return fmt.Errorf("invalid stat modifier %q", c.ID)
	}
	if c.Operation != "add" && c.Operation != "percent" && c.Operation != "final_add" {
		return fmt.Errorf("invalid stat operation %q", c.Operation)
	}
	if (c.Operation == "percent") != (c.Group != "") || len(c.Group) > 128 {
		return fmt.Errorf("modifier %s: percent requires a group; additions cannot have one", c.ID)
	}
	return nil
}
func ValidateSource(s model.SourceRef) error {
	if s.Kind == "" || s.Definition == "" || len(s.Kind) > 128 || len(s.Actor) > 128 || len(s.Instance) > 128 || len(s.Definition) > 128 {
		return fmt.Errorf("invalid contribution source")
	}
	return nil
}

type identity struct {
	ID     string
	Source model.SourceRef
}

func Validate(contributions []model.StatContribution) error {
	if len(contributions) > 4096 {
		return fmt.Errorf("stat contribution budget exceeded")
	}
	seen := map[identity]bool{}
	for _, c := range contributions {
		if err := ValidateModifier(c.StatModifier); err != nil {
			return err
		}
		if err := ValidateSource(c.Source); err != nil {
			return err
		}
		key := identity{c.ID, c.Source}
		if seen[key] {
			return fmt.Errorf("duplicate stat contribution %s", c.ID)
		}
		seen[key] = true
	}
	return nil
}
func order(a, b model.StatContribution) int {
	return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Source.Kind, b.Source.Kind), cmp.Compare(a.Source.Definition, b.Source.Definition), cmp.Compare(a.Source.Actor, b.Source.Actor), cmp.Compare(a.Source.Instance, b.Source.Instance))
}

// Evaluate always starts from immutable base values. Removing a contribution means
// evaluating the remaining set, never subtracting a previous rounded result.
func Evaluate(q Input) (Result, error) {
	if err := Validate(q.Contributions); err != nil {
		return Result{}, err
	}
	for _, key := range fields {
		n, _ := value(q.Base, key)
		if !finite(n) || n < 0 || key == "max_hp" && n < 1 {
			return Result{}, fmt.Errorf("invalid base stat %s", key)
		}
	}
	source := q.BaseSource
	if source.Kind == "" {
		source = model.SourceRef{Kind: "base", Definition: "engine:base"}
	}
	if err := ValidateSource(source); err != nil {
		return Result{}, err
	}
	contributions := slices.Clone(q.Contributions)
	slices.SortFunc(contributions, order)
	r := Result{Values: q.Base}
	for _, key := range fields {
		n, _ := value(q.Base, key)
		r.Steps = append(r.Steps, Step{Stat: key, Operation: "base", Operand: n, Output: n, Contributions: []model.StatContribution{{StatModifier: model.StatModifier{ID: "engine:base:" + key, Stat: key, Operation: "base", Value: n}, Source: source}}})
		apply := func(op, group string, operand float64, members []model.StatContribution) {
			before := n
			if op == "multiply" {
				n *= operand
			} else {
				n += operand
			}
			r.Steps = append(r.Steps, Step{Stat: key, Operation: op, Group: group, Input: before, Operand: operand, Output: n, Contributions: slices.Clone(members)})
		}
		for _, c := range contributions {
			if c.Stat == key && c.Operation == "add" {
				apply("add", "", c.Value, []model.StatContribution{c})
			}
		}
		groups := map[string][]model.StatContribution{}
		for _, c := range contributions {
			if c.Stat == key && c.Operation == "percent" {
				groups[c.Group] = append(groups[c.Group], c)
			}
		}
		keys := make([]string, 0, len(groups))
		for group := range groups {
			keys = append(keys, group)
		}
		slices.Sort(keys)
		for _, group := range keys {
			percent := 0.0
			for _, c := range groups[group] {
				percent += c.Value
			}
			if !finite(percent) || percent < -1 {
				return Result{}, fmt.Errorf("stat %s: invalid factor for %s", key, group)
			}
			apply("multiply", group, 1+percent, groups[group])
		}
		for _, c := range contributions {
			if c.Stat == key && c.Operation == "final_add" {
				apply("add", "", c.Value, []model.StatContribution{c})
			}
		}
		if !finite(n) {
			return Result{}, fmt.Errorf("stat %s overflow", key)
		}
		minimum := 0.0
		if key == "max_hp" {
			minimum = 1
		}
		if n < minimum {
			r.Steps = append(r.Steps, Step{Stat: key, Operation: "minimum", Input: n, Operand: minimum, Output: minimum})
			n = minimum
		}
		r.Values = with(r.Values, key, n)
	}
	return r, nil
}

// Upsert returns a detached contribution set; identity includes the instance source.
func Upsert(current []model.StatContribution, contribution model.StatContribution) ([]model.StatContribution, error) {
	next := slices.Clone(current)
	key := identity{contribution.ID, contribution.Source}
	for i, c := range next {
		if (identity{c.ID, c.Source}) == key {
			next[i] = contribution
			if err := Validate(next); err != nil {
				return nil, err
			}
			return next, nil
		}
	}
	next = append(next, contribution)
	if err := Validate(next); err != nil {
		return nil, err
	}
	return next, nil
}
func CloneSteps(steps []Step) []Step {
	next := slices.Clone(steps)
	for i := range next {
		next[i].Contributions = slices.Clone(next[i].Contributions)
	}
	return next
}
func ResizeHealth(hp, oldMax, newMax float64) (float64, error) {
	if !finite(hp) || !finite(oldMax) || !finite(newMax) || oldMax < 1 || newMax < 1 || hp < 0 || hp > oldMax {
		return 0, fmt.Errorf("invalid health resize")
	}
	return min(newMax, math.Round((hp/oldMax*newMax)*100)/100), nil
}
