package damage

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"math"
)

type SegmentResult struct {
	ID         string              `json:"id"`
	Damage     Result              `json:"damage"`
	Shields    []status.Absorption `json:"shields,omitempty"`
	Absorbed   float64             `json:"absorbed"`
	HealthLoss float64             `json:"health_loss"`
	Overkill   float64             `json:"overkill"`
}

func evaluateParts(q Request) (Result, error) {
	if len(q.Weapon.AdditionalDamage) > 15 {
		return Result{}, fmt.Errorf("too many damage segments")
	}
	parts := q.Weapon.AdditionalDamage
	q.Weapon.AdditionalDamage = nil
	base, err := Evaluate(q)
	if err != nil {
		return Result{}, err
	}
	r := Result{Type: base.Type, Raw: base.Raw, Total: base.Total, Critical: q.Critical, Segments: []SegmentResult{{ID: "base", Damage: base}}}
	seen := map[string]bool{"base": true}
	for _, p := range parts {
		if p.ID == "" || seen[p.ID] {
			return Result{}, fmt.Errorf("invalid segment identity")
		}
		seen[p.ID] = true
		next := q
		next.Weapon = model.Weapon{ID: q.Weapon.ID, Type: p.Type, Base: p.Base, Scaling: p.Scaling, Modifiers: p.Modifiers}
		next.FlatBonuses = nil
		for _, b := range p.Bonuses {
			next.FlatBonuses = append(next.FlatBonuses, Bonus{ID: b.ID, Value: b.Amount, Origin: b.Source, Basis: b.Basis})
		}
		next.Defense.IgnoreArmor = q.Defense.IgnoreArmor || p.Type == "magic" || p.Type == "electromagnetic"
		part, err := Evaluate(next)
		if err != nil {
			return Result{}, err
		}
		r.Segments = append(r.Segments, SegmentResult{ID: p.ID, Damage: part})
		r.Raw += part.Raw
		r.Total = Round(r.Total + part.Total)
		if r.Type != part.Type {
			r.Type = "mixed"
		}
	}
	return r, nil
}

// Settle consumes each segment in order. Validation and shield effects remain
// detached until the authoritative owner commits the complete replacement.
func Settle(r Result, hp float64, s model.StatusSet) (Result, float64, model.StatusSet, error) {
	if math.IsNaN(hp) || math.IsInf(hp, 0) || hp < 0 {
		return Result{}, 0, model.StatusSet{}, fmt.Errorf("invalid health")
	}
	if err := status.Validate(s); err != nil {
		return Result{}, 0, model.StatusSet{}, err
	}
	r = CloneResult(r)
	r.Settled = true
	if len(r.Segments) == 0 {
		r.Segments = []SegmentResult{{ID: "base", Damage: Result{Type: r.Type, Raw: r.Raw, Total: r.Total, Critical: r.Critical, Steps: r.Steps}}}
	}
	r.Absorbed, r.HealthLoss, r.Overkill = 0, 0, 0
	for i := range r.Segments {
		v := &r.Segments[i]
		v.Absorbed = 0
		v.HealthLoss = 0
		v.Overkill = 0
		amount := v.Damage.Total
		var err error
		s, v.Shields, err = status.ConsumeShield(s, amount)
		if err != nil {
			return Result{}, 0, model.StatusSet{}, err
		}
		for _, a := range v.Shields {
			v.Absorbed += a.Amount
		}
		remaining := max(0, Round(amount-v.Absorbed))
		v.HealthLoss = math.Min(hp, remaining)
		v.Overkill = Round(remaining - v.HealthLoss)
		hp = max(0, Round(hp-v.HealthLoss))
		r.Absorbed = Round(r.Absorbed + v.Absorbed)
		r.HealthLoss = Round(r.HealthLoss + v.HealthLoss)
		r.Overkill = Round(r.Overkill + v.Overkill)
	}
	return r, hp, s, nil
}
