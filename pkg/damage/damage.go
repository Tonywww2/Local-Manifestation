// Package damage evaluates damage and its explanation in a single pass.
// It has no dependency on a renderer, network transport or script VM.
package damage

import (
	"fmt"
	"math"
	"sort"

	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
)

type Step struct {
	Basis          model.DamageBasis `json:"basis,omitzero"`
	Group          string            `json:"group,omitempty"`
	Origin         model.SourceRef   `json:"origin"`
	Members        []model.Modifier  `json:"members,omitempty"`
	AttributeSteps []stats.Step      `json:"attribute_steps,omitempty"`
	Operation      string            `json:"operation"`
	Source         string            `json:"source"`
	Input          float64           `json:"input"`
	Operand        float64           `json:"operand"`
	Output         float64           `json:"output"`
	Stat           string            `json:"stat,omitempty"`
	Score          float64           `json:"score,omitempty"`
	Weight         float64           `json:"weight,omitempty"`
}
type Bonus struct {
	Basis  model.DamageBasis `json:"basis,omitzero"`
	ID     string            `json:"id"`
	Value  float64           `json:"value"`
	Origin model.SourceRef   `json:"origin"`
}
type Defense struct {
	Armor       float64
	IgnoreArmor bool
	Resistance  float64
	Reductions  []float64
	Immune      bool
}
type Request struct {
	Effects         []model.CombatContribution
	Source          model.SourceRef
	TypedDefense    map[string]Defense
	Stats           *stats.Input
	FlatBonuses     []Bonus
	FinalSource     model.SourceRef
	Weapon          model.Weapon
	Attributes      model.Attributes
	Defense         Defense
	Critical        bool
	CritMultiplier  float64
	FinalMultiplier float64
}
type Result struct {
	Settled    bool            `json:"settled,omitempty"`
	Segments   []SegmentResult `json:"segments,omitempty"`
	Absorbed   float64         `json:"absorbed,omitempty"`
	HealthLoss float64         `json:"health_loss,omitempty"`
	Overkill   float64         `json:"overkill,omitempty"`
	Type       string          `json:"type"`
	Raw        float64         `json:"raw"`
	Total      float64         `json:"total"`
	Critical   bool            `json:"critical"`
	Steps      []Step          `json:"steps"`
}

func Round(n float64) float64 { return math.Round(n*100) / 100 }
func Evaluate(q Request) (Result, error) {
	if len(q.Weapon.AdditionalDamage) > 0 {
		return evaluateParts(q)
	}
	q.Defense.IgnoreArmor = q.Defense.IgnoreArmor || q.Weapon.Type == "magic" || q.Weapon.Type == "electromagnetic"
	if d, ok := q.TypedDefense[q.Weapon.Type]; ok {
		q.Defense = d
		q.Defense.IgnoreArmor = q.Defense.IgnoreArmor || q.Weapon.Type == "magic" || q.Weapon.Type == "electromagnetic"
	}
	r := Result{Type: q.Weapon.Type, Critical: q.Critical}
	var attributeSteps []stats.Step
	if q.Stats != nil {
		derived, err := stats.Evaluate(*q.Stats)
		if err != nil {
			return Result{}, err
		}
		q.Attributes = derived.Values.Attributes
		attributeSteps = derived.Steps
	}
	values := []float64{q.Weapon.Base, q.Weapon.Flat, q.Weapon.FinalFlat, q.Weapon.Scaling.Dot(q.Attributes), q.Defense.Armor, q.Defense.Resistance, q.CritMultiplier, q.FinalMultiplier}
	for _, m := range q.Weapon.Modifiers {
		values = append(values, m.Percent)
	}
	for _, b := range q.FlatBonuses {
		if b.ID == "" {
			return Result{}, fmt.Errorf("missing damage bonus identity")
		}
		if err := stats.ValidateSource(b.Origin); err != nil {
			return Result{}, err
		}
		values = append(values, b.Value)
	}
	values = append(values, q.Defense.Reductions...)
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e100 {
			return r, fmt.Errorf("invalid damage input")
		}
	}
	apply := func(op, src string, n float64) {
		before := r.Raw
		switch op {
		case "add":
			r.Raw += n
		case "multiply":
			r.Raw *= n
		case "subtract":
			r.Raw -= n
		case "minimum":
			r.Raw = math.Max(r.Raw, n)
		case "override":
			r.Raw = n
		}
		r.Steps = append(r.Steps, Step{Operation: op, Source: src, Input: before, Operand: n, Output: r.Raw})
	}
	apply("add", q.Weapon.ID+":base", q.Weapon.Base)
	r.Steps[len(r.Steps)-1].Origin = model.SourceRef{Kind: "weapon", Definition: q.Weapon.ID}
	for _, term := range []struct {
		name          string
		score, weight float64
	}{
		{"strength", q.Attributes.Strength, q.Weapon.Scaling.Strength},
		{"agility", q.Attributes.Agility, q.Weapon.Scaling.Agility},
		{"constitution", q.Attributes.Constitution, q.Weapon.Scaling.Constitution},
		{"intelligence", q.Attributes.Intelligence, q.Weapon.Scaling.Intelligence},
		{"sense", q.Attributes.Sense, q.Weapon.Scaling.Sense},
		{"mana", q.Attributes.Mana, q.Weapon.Scaling.Mana},
	} {
		if term.weight != 0 {
			apply("add", "attribute:"+term.name, term.score*term.weight)
			step := &r.Steps[len(r.Steps)-1]
			step.Stat = term.name
			step.Score = term.score
			step.Weight = term.weight
			for _, s := range attributeSteps {
				if s.Stat == term.name {
					step.AttributeSteps = append(step.AttributeSteps, s)
				}
			}
		}
	}
	for _, b := range q.FlatBonuses {
		apply("add", b.ID, b.Value)
		r.Steps[len(r.Steps)-1].Origin = b.Origin
		r.Steps[len(r.Steps)-1].Basis = b.Basis
	}
	apply("add", "ordinary flat", q.Weapon.Flat)
	groups := map[string]float64{}
	sources := map[string]string{}
	members := map[string][]model.Modifier{}
	for _, m := range q.Weapon.Modifiers {
		if m.Type != "" && m.Type != q.Weapon.Type {
			continue
		}
		groups[m.Group] += m.Percent
		members[m.Group] = append(members[m.Group], m)
		if sources[m.Group] != "" {
			sources[m.Group] += " + "
		}
		sources[m.Group] += m.Source
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		apply("multiply", k+" ["+sources[k]+"]", 1+groups[k])
		r.Steps[len(r.Steps)-1].Group = k
		r.Steps[len(r.Steps)-1].Members = append([]model.Modifier(nil), members[k]...)
	}
	apply("add", "final flat", q.Weapon.FinalFlat)
	if q.Critical {
		if q.CritMultiplier < 1 {
			return r, fmt.Errorf("critical multiplier must be >= 1")
		}
		apply("multiply", "critical", q.CritMultiplier)
	}
	apply("multiply", "final damage", q.FinalMultiplier)
	r.Steps[len(r.Steps)-1].Origin = q.FinalSource
	offense := r.Raw
	if !q.Defense.IgnoreArmor {
		apply("subtract", "armor", q.Defense.Armor)
	}
	if q.Defense.Resistance < 0 || q.Defense.Resistance > 1 {
		return r, fmt.Errorf("resistance outside [0,1]")
	}
	apply("multiply", "resistance", 1-q.Defense.Resistance)
	for i, v := range q.Defense.Reductions {
		if v < 0 || v > 1 {
			return r, fmt.Errorf("reduction outside [0,1]")
		}
		apply("multiply", fmt.Sprintf("reduction %d", i+1), 1-v)
	}
	apply("minimum", "minimum damage", 1)
	if q.Defense.Immune {
		apply("override", "immunity", 0)
	}
	if math.IsNaN(r.Raw) || math.IsInf(r.Raw, 0) || math.Abs(r.Raw) > 1e100 {
		return Result{}, fmt.Errorf("damage overflow")
	}
	r.Total = Round(r.Raw)
	r.Raw = offense
	return r, nil
}

func CloneResult(r Result) Result {
	r.Segments = append([]SegmentResult(nil), r.Segments...)
	for i := range r.Segments {
		r.Segments[i].Damage = CloneResult(r.Segments[i].Damage)
		r.Segments[i].Shields = append([]status.Absorption(nil), r.Segments[i].Shields...)
	}
	r.Steps = append([]Step(nil), r.Steps...)
	for i := range r.Steps {
		r.Steps[i].Members = append([]model.Modifier(nil), r.Steps[i].Members...)
		r.Steps[i].AttributeSteps = stats.CloneSteps(r.Steps[i].AttributeSteps)
	}
	return r
}
