package model

type WeaponSkillDef struct {
	Icon             string              `json:"icon,omitempty"`
	ClearProjectiles *ProjectileClearDef `json:"clear_projectiles,omitempty"`
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	Cost             float64             `json:"cost"`
	CooldownMillis   int64               `json:"cooldown_ms"`
	Weapon           string              `json:"weapon,omitempty"`
	Effects          []ItemEffect        `json:"effects,omitempty"`
}

// ProjectileClearDef describes an instantaneous forward sector in world coordinates.
type ProjectileClearDef struct {
	Range float64 `json:"range"`
	Arc   float64 `json:"arc"` // Total angle, in degrees.
}

type ResourceDef struct {
	Maximum      float64 `json:"maximum"`
	Regeneration float64 `json:"regeneration"`
}
