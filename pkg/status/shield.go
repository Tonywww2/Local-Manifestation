package status

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"math"
)

type Absorption struct {
	Instance uint64          `json:"instance"`
	Source   model.SourceRef `json:"source"`
	Amount   float64         `json:"amount"`
}

func ShieldTotal(s model.StatusSet) float64 {
	var total float64
	for _, v := range s.Instances {
		if v.Shield {
			for _, l := range v.Layers {
				total += l.Shield
			}
		}
	}
	return total
}

// ConsumeShield is detached and ordered by application sequence, newest first.
// Exhausted shields remain as zero-balance timed effects, never regenerate.
func ConsumeShield(s model.StatusSet, amount float64) (model.StatusSet, []Absorption, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return model.StatusSet{}, nil, fmt.Errorf("invalid shield damage")
	}
	if err := Validate(s); err != nil {
		return model.StatusSet{}, nil, err
	}
	if amount == 0 || ShieldTotal(s) == 0 {
		return s, nil, nil
	}
	s = model.CloneStatuses(s)
	var rows []Absorption
	for amount > 0 {
		index := -1
		for i, v := range s.Instances {
			if v.Shield && v.Layers[0].Shield > 0 && (index < 0 || v.Layers[0].ID > s.Instances[index].Layers[0].ID) {
				index = i
			}
		}
		if index < 0 {
			break
		}
		v := &s.Instances[index]
		used := math.Min(amount, v.Layers[0].Shield)
		v.Layers[0].Shield -= used
		amount -= used
		rows = append(rows, Absorption{Instance: v.ID, Source: v.Source, Amount: used})
	}
	if len(rows) > 0 {
		s.Revision++
	}
	return s, rows, nil
}
