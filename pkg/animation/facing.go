package animation

import "hsmultiplayer/pkg/model"

// PlayerFacing uses accepted action directions through recovery. Movement and
// locally pending attack input share this rule without changing combat aim.
func PlayerFacing(a model.PlayerActionState, movement, aim model.Vec, attacking bool) model.Vec {
	candidates := [5]model.Vec{}
	if Active(a.Skill) {
		candidates[0] = a.Skill.Direction
	}
	if Active(a.Attack) {
		candidates[1] = a.Attack.Direction
	}
	if attacking {
		candidates[2] = aim
	}
	candidates[3], candidates[4] = movement, a.Facing
	for _, direction := range candidates {
		if direction.Finite() && direction.Length() > 1e-6 {
			return direction.Unit()
		}
	}
	return model.Vec{X: 1}
}
