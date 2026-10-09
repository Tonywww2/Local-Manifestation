package model

type SkillDef struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	Cost           float64      `json:"cost"`
	CooldownMillis int64        `json:"cooldown_ms"`
	Weapon         string       `json:"weapon,omitempty"`
	Effects        []ItemEffect `json:"effects,omitempty"`
}
type ResourceDef struct {
	Maximum      float64 `json:"maximum"`
	Regeneration float64 `json:"regeneration"`
}
