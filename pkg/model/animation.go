package model

// ActionAnimation is a detached, simulated action timeline, not a hitbox or a
// client animation clock. Sequence identifies repetitions of the same action.
type ActionAnimation struct {
	Sequence  uint64  `json:"sequence"`
	Weapon    string  `json:"weapon,omitempty"`
	Kind      string  `json:"kind,omitempty"`
	Direction Vec     `json:"direction"`
	Elapsed   float64 `json:"elapsed"`
	Duration  float64 `json:"duration"`
	Windup    float64 `json:"windup,omitempty"`
	Active    float64 `json:"active,omitempty"`
}

type PlayerActionState struct {
	Player string          `json:"player"`
	Facing Vec             `json:"facing"`
	Move   Vec             `json:"move,omitzero"`
	Walk   float64         `json:"walk,omitempty"`
	Attack ActionAnimation `json:"attack,omitzero"`
	Skill  ActionAnimation `json:"skill,omitzero"`
	Dodge  ActionAnimation `json:"dodge,omitzero"`
	Hit    ActionAnimation `json:"hit,omitzero"`
	Life   ActionAnimation `json:"life,omitzero"`
}

// EnemyActionState is an all-value display record. A dead enemy can remain in
// this slice briefly after the combat owner removes its collidable instance.
type EnemyActionState struct {
	ID         int             `json:"id"`
	Definition string          `json:"definition"`
	Position   Vec             `json:"position"`
	Radius     float64         `json:"radius"`
	Facing     Vec             `json:"facing"`
	Move       Vec             `json:"move,omitzero"`
	Walk       float64         `json:"walk,omitempty"`
	Attack     ActionAnimation `json:"attack,omitzero"`
	Hit        ActionAnimation `json:"hit,omitzero"`
	Death      ActionAnimation `json:"death,omitzero"`
}
