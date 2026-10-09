package damage

import "hsmultiplayer/pkg/model"

// CloneRequest detaches the captured inputs before crossing an ownership boundary.
func CloneRequest(q Request) Request {
	q.Weapon = model.CloneWeapon(q.Weapon)
	q.Effects = append([]model.CombatContribution(nil), q.Effects...)
	q.FlatBonuses = append([]Bonus(nil), q.FlatBonuses...)
	q.Defense.Reductions = append([]float64(nil), q.Defense.Reductions...)
	if q.Stats != nil {
		v := *q.Stats
		v.Contributions = append([]model.StatContribution(nil), v.Contributions...)
		q.Stats = &v
	}
	if q.TypedDefense != nil {
		copy := make(map[string]Defense, len(q.TypedDefense))
		for k, v := range q.TypedDefense {
			v.Reductions = append([]float64(nil), v.Reductions...)
			copy[k] = v
		}
		q.TypedDefense = copy
	}
	return q
}
