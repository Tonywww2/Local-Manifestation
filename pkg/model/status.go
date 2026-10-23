package model

// StatusDef is immutable content. Level belongs to the whole status; individual
// layers only have identity, their own expiry and an application-time snapshot.
type StatusDef struct {
	ModifierPolicy string         `json:"modifier_policy,omitempty"`
	Combat         []CombatEffect `json:"combat,omitempty"`
	ShieldMaxHP    float64        `json:"shield_max_hp,omitempty"`
	Periodic       []PeriodicDef  `json:"periodic,omitempty"`
	Controls       []string       `json:"controls,omitempty"`
	Immunities     []string       `json:"immunities,omitempty"`
	LevelModifiers []StatModifier `json:"level_modifiers,omitempty"`
	Shield         float64        `json:"shield,omitempty"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	NameKey        string         `json:"name_key,omitempty"`
	Description    string         `json:"description"`
	DescriptionKey string         `json:"description_key,omitempty"`
	Level          int            `json:"level"`
	MaxLayers      int            `json:"max_layers"`
	DurationMillis int64          `json:"duration_ms"` // Zero with ClearOnLeave lasts until room departure.
	Unique         bool           `json:"unique,omitempty"`
	Conflicts      []string       `json:"conflicts,omitempty"`
	ClearOnLeave   bool           `json:"clear_on_leave,omitempty"`
	BoundToSource  bool           `json:"bound_to_source,omitempty"`
	Modifiers      []StatModifier `json:"modifiers"`
}

type StatusLayer struct {
	ShieldMaxHP float64            `json:"shield_max_hp,omitempty"`
	Periodic    []PeriodicSnapshot `json:"periodic,omitempty"`
	Shield      float64            `json:"shield,omitempty"`
	Capacity    float64            `json:"capacity,omitempty"`
	ID          uint64             `json:"id"`
	Applied     int64              `json:"applied"`
	Expires     int64              `json:"expires"`
	Modifiers   []StatModifier     `json:"modifiers"`
}

type StatusInstance struct {
	ModifierPolicy string        `json:"modifier_policy,omitempty"`
	Controls       []string      `json:"controls,omitempty"`
	Immunities     []string      `json:"immunities,omitempty"`
	Shield         bool          `json:"shield,omitempty"`
	ID             uint64        `json:"id"`
	Definition     string        `json:"definition"`
	Source         SourceRef     `json:"source"`
	Level          int           `json:"level"`
	ClearOnLeave   bool          `json:"clear_on_leave,omitempty"`
	BoundToSource  bool          `json:"bound_to_source,omitempty"`
	Conflicts      []string      `json:"conflicts,omitempty"`
	Layers         []StatusLayer `json:"layers"`
}

type StatusSet struct {
	Time      int64            `json:"time"`
	NextID    uint64           `json:"next_id,omitempty"`
	Revision  uint64           `json:"revision,omitempty"`
	Instances []StatusInstance `json:"instances,omitempty"`
}

func CloneStatuses(s StatusSet) StatusSet {
	s.Instances = append([]StatusInstance(nil), s.Instances...)
	for i := range s.Instances {
		v := &s.Instances[i]
		v.Controls = append([]string(nil), v.Controls...)
		v.Immunities = append([]string(nil), v.Immunities...)
		v.Conflicts = append([]string(nil), v.Conflicts...)
		v.Layers = append([]StatusLayer(nil), v.Layers...)
		for j := range v.Layers {
			v.Layers[j].Periodic = append([]PeriodicSnapshot(nil), v.Layers[j].Periodic...)
			v.Layers[j].Modifiers = append([]StatModifier(nil), v.Layers[j].Modifiers...)
		}
	}
	return s
}

type PeriodicDef struct {
	Kind           string     `json:"kind"`
	Type           string     `json:"type,omitempty"`
	Amount         float64    `json:"amount"`
	PerLevel       float64    `json:"per_level,omitempty"`
	Scaling        Attributes `json:"scaling"`
	IntervalMillis int64      `json:"interval_ms"`
}
type PeriodicSnapshot struct {
	HealingMultiplier float64    `json:"healing_multiplier,omitempty"`
	Attributes        Attributes `json:"attributes"`
	Kind              string     `json:"kind"`
	Type              string     `json:"type,omitempty"`
	Amount            float64    `json:"amount"`
	Interval          int64      `json:"interval"`
	Next              int64      `json:"next"`
}
