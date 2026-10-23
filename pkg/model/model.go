// Package model contains serializable values shared by the engine, SDK and protocol.
package model

import "math"

type Vec struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (v Vec) Add(w Vec) Vec       { return Vec{v.X + w.X, v.Y + w.Y} }
func (v Vec) Sub(w Vec) Vec       { return Vec{v.X - w.X, v.Y - w.Y} }
func (v Vec) Scale(n float64) Vec { return Vec{v.X * n, v.Y * n} }
func (v Vec) Length() float64     { return math.Hypot(v.X, v.Y) }
func (v Vec) Unit() Vec {
	if n := v.Length(); n > 0 {
		return v.Scale(1 / n)
	}
	return Vec{}
}
func (v Vec) Finite() bool {
	return !math.IsNaN(v.X) && !math.IsNaN(v.Y) && !math.IsInf(v.X, 0) && !math.IsInf(v.Y, 0)
}

type Attributes struct {
	Strength     float64 `json:"strength"`
	Agility      float64 `json:"agility"`
	Constitution float64 `json:"constitution"`
	Intelligence float64 `json:"intelligence"`
	Sense        float64 `json:"sense"`
	Mana         float64 `json:"mana"`
}

func (a Attributes) Dot(b Attributes) float64 {
	return a.Strength*b.Strength + a.Agility*b.Agility + a.Constitution*b.Constitution + a.Intelligence*b.Intelligence + a.Sense*b.Sense + a.Mana*b.Mana
}

type Modifier struct {
	Type    string    `json:"type,omitempty"`
	Group   string    `json:"group"`
	Source  string    `json:"source"`
	Percent float64   `json:"percent"`
	Origin  SourceRef `json:"origin,omitempty"`
}
type Weapon struct {
	Skill            string       `json:"weapon_skill,omitempty"`
	OnHitStatuses    []string     `json:"on_hit_statuses,omitempty"`
	Mode             string       `json:"mode,omitempty"`
	Windup           float64      `json:"windup,omitempty"`
	Recovery         float64      `json:"recovery,omitempty"`
	ActiveDuration   float64      `json:"active_duration,omitempty"`
	Range            float64      `json:"range,omitempty"`
	Tracking         float64      `json:"tracking,omitempty"`
	Pierce           int          `json:"pierce,omitempty"`
	RestrictSwap     bool         `json:"restrict_swap,omitempty"`
	AdditionalDamage []DamagePart `json:"additional_damage,omitempty"`
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Type             string       `json:"type"`
	Base             float64      `json:"base"`
	Scaling          Attributes   `json:"scaling"`
	Flat             float64      `json:"flat"`
	Modifiers        []Modifier   `json:"modifiers"`
	FinalFlat        float64      `json:"final_flat"`
	Cooldown         float64      `json:"cooldown"`
	ProjectileSpeed  float64      `json:"projectile_speed"`
}

type DamagePart struct {
	Bonuses   []DamageBonus `json:"bonuses,omitempty"`
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Base      float64       `json:"base"`
	Scaling   Attributes    `json:"scaling"`
	Modifiers []Modifier    `json:"modifiers,omitempty"`
}

type DamageBonus struct {
	Basis  DamageBasis `json:"basis,omitzero"`
	ID     string      `json:"id"`
	Amount float64     `json:"amount"`
	Source SourceRef   `json:"source"`
}

// DamageBasis is a value explanation of an appended pre-multiplier segment.
type DamageBasis struct {
	Base       float64 `json:"base"`
	Attributes float64 `json:"attributes"`
	Flat       float64 `json:"flat"`
	Upgrade    float64 `json:"upgrade"`
	Factor     float64 `json:"factor"`
}

func (b DamageBasis) Total() float64 { return b.Base + b.Attributes + b.Flat + b.Upgrade }

type Chassis struct {
	RequiredPerk     string         `json:"required_perk,omitempty"`
	GrowthPreference Attributes     `json:"growth_preference"`
	Modifiers        []StatModifier `json:"modifiers,omitempty"`
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Attributes       Attributes     `json:"attributes"`
	HP               float64        `json:"hp"`
	Armor            float64        `json:"armor"`
	Speed            float64        `json:"speed"`
	Weapon           string         `json:"weapon"`
}
type EnemyAttackDef struct {
	Kind     string  `json:"kind"`
	Windup   float64 `json:"windup"`
	Recovery float64 `json:"recovery"`
	Cooldown float64 `json:"cooldown"`
	Range    float64 `json:"range"`
	Active   float64 `json:"active,omitempty"`
	Speed    float64 `json:"speed,omitempty"`
}
type EnemyDef struct {
	Appearance         string             `json:"appearance,omitempty"`
	Attacks            []EnemyAttackDef   `json:"attacks,omitempty"`
	ArmorPerFloor      float64            `json:"armor_per_floor,omitempty"`
	ResistancePerFloor map[string]float64 `json:"resistance_per_floor,omitempty"`
	EnrageProjectiles  int                `json:"enrage_projectiles,omitempty"`
	AttackRange        float64            `json:"attack_range,omitempty"`
	FireInterval       float64            `json:"fire_interval,omitempty"`
	ProjectileSpeed    float64            `json:"projectile_speed,omitempty"`
	ProjectileCount    int                `json:"projectile_count,omitempty"`
	DamageType         string             `json:"damage_type,omitempty"`
	SpeedPerFloor      float64            `json:"speed_per_floor,omitempty"`
	FirePerFloor       float64            `json:"fire_per_floor,omitempty"`
	ThreatDistance     float64            `json:"threat_distance,omitempty"`
	ThreatDamage       float64            `json:"threat_damage,omitempty"`
	EnrageThreshold    float64            `json:"enrage_threshold,omitempty"`
	EnrageFactor       float64            `json:"enrage_factor,omitempty"`
	Resistance         map[string]float64 `json:"resistance,omitempty"`
	ControlResistance  float64            `json:"control_resistance,omitempty"`
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	HP                 float64            `json:"hp"`
	Damage             float64            `json:"damage"`
	Armor              float64            `json:"armor"`
	Speed              float64            `json:"speed"`
	Radius             float64            `json:"radius"`
	VisualRadius       float64            `json:"visual_radius,omitempty"`
	Behavior           string             `json:"behavior"`
}
type Node struct {
	Blocking   bool     `json:"blocking"`
	Definition string   `json:"definition"`
	Name       string   `json:"name"`
	NameKey    string   `json:"name_key"`
	Room       string   `json:"room,omitempty"`
	Cost       int      `json:"cost"`
	ID         string   `json:"id"`
	Q          int      `json:"q"`
	R          int      `json:"r"`
	Kind       string   `json:"kind"`
	Payment    string   `json:"payment"`
	Hint       string   `json:"hint"`
	HintKey    string   `json:"hint_key,omitempty"`
	Edges      []string `json:"edges"`
	Open       bool     `json:"open"`
	Completed  bool     `json:"completed"`
}

// Terrain occupies a map cell without creating a room or an entrance.
type Terrain struct {
	Q       int    `json:"q"`
	R       int    `json:"r"`
	Name    string `json:"name"`
	NameKey string `json:"name_key,omitempty"`
}
type Player struct {
	EnhancedWins      int                `json:"enhanced_wins"`
	KnockdownBase     int                `json:"knockdown_base"`
	MetaRanks         map[string]int     `json:"meta_ranks,omitempty"`
	Loot              []Loot             `json:"loot,omitempty"`
	Energy            float64            `json:"energy"`
	MaxEnergy         float64            `json:"max_energy"`
	EnergyRegen       float64            `json:"energy_regen"`
	ResourceTime      int64              `json:"resource_time"`
	Inventory         Inventory          `json:"inventory"`
	Statuses          StatusSet          `json:"statuses"`
	BaseStats         StatValues         `json:"base_stats"`
	StatContributions []StatContribution `json:"stat_contributions,omitempty"`
	PositionEpoch     uint64             `json:"position_epoch,omitempty"`
	WeaponBonus       float64            `json:"weapon_bonus"`
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Chassis           string             `json:"chassis"`
	Online            bool               `json:"online"`
	Active            bool               `json:"active"`
	Node              string             `json:"node"`
	Scene             string             `json:"scene"`
	AP                int                `json:"ap"`
	Position          Vec                `json:"position"`
	HP                float64            `json:"hp"`
	MaxHP             float64            `json:"max_hp"`
	BaseHP            float64            `json:"base_hp"`
	Armor             float64            `json:"armor"`
	Speed             float64            `json:"speed"`
	Attributes        Attributes         `json:"attributes"`
	Weapon            string             `json:"weapon"`
	Level             int                `json:"level"`
	XP                int                `json:"xp"`
	Coins             int                `json:"coins"`
	Upgrade           int                `json:"upgrade"`
	Downs             int                `json:"downs"`
	Knockdowns        int                `json:"knockdowns"`
	Life              string             `json:"life"`
	Dodges            int                `json:"dodges"`
}
type Enemy struct {
	Phase           string    `json:"phase,omitempty"`
	AttackKind      string    `json:"attack_kind,omitempty"`
	AttackDirection Vec       `json:"attack_direction,omitempty"`
	Statuses        StatusSet `json:"statuses"`
	PositionEpoch   uint64    `json:"position_epoch,omitempty"`
	ID              int       `json:"id"`
	Definition      string    `json:"definition"`
	Position        Vec       `json:"position"`
	HP              float64   `json:"hp"`
	MaxHP           float64   `json:"max_hp"`
	Radius          float64   `json:"radius"`
}
type Projectile struct {
	Arc        float64 `json:"arc,omitempty"`
	DamageType string  `json:"damage_type,omitempty"`
	Identity   string  `json:"identity,omitempty"` // Stable across simulation hosts; ID is room-local.
	Kind       string  `json:"kind,omitempty"`
	Radius     float64 `json:"radius,omitempty"`
	ID         int     `json:"id"`
	Owner      string  `json:"owner"`
	Position   Vec     `json:"position"`
	Velocity   Vec     `json:"velocity"`
	TTL        float64 `json:"ttl"`
	Age        float64 `json:"age,omitempty"` // Elapsed simulated lifetime; preserved when crossing rooms.
}
type Input struct {
	Move   Vec  `json:"move"`
	Aim    Vec  `json:"aim"`
	Attack bool `json:"attack"`
	Dodge  bool `json:"dodge"`
}
type Event struct {
	Critical         bool    `json:"critical,omitempty"`
	Attack           bool    `json:"attack,omitempty"`
	Absorbed         float64 `json:"absorbed,omitempty"`
	Overkill         float64 `json:"overkill,omitempty"`
	Scene            string  `json:"scene,omitempty"`
	SourceDefinition string  `json:"source_definition,omitempty"`
	TargetDefinition string  `json:"target_definition,omitempty"`
	AttackID         string  `json:"attack_id,omitempty"`
	HitID            string  `json:"hit_id,omitempty"`
	SegmentID        string  `json:"segment_id,omitempty"`
	DamageType       string  `json:"damage_type,omitempty"`
	Kind             string  `json:"kind"`
	Source           string  `json:"source,omitempty"`
	Target           string  `json:"target,omitempty"`
	Amount           float64 `json:"amount,omitempty"`
	Tick             uint64  `json:"tick"`
}
