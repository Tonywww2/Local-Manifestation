package model

// ContentFormat is independent of protocol, checkpoint and SDK versions.
const ContentFormat = 4

type RunDef struct {
	Floors    []string `json:"floors"`
	VoteRatio float64  `json:"vote_ratio"`
}
type AdventureDef struct {
	Format int              `json:"format"`
	Floors []FloorDef       `json:"floors"`
	Maps   []MapTemplateDef `json:"maps"`
	Nodes  []NodeDef        `json:"nodes"`
	Rooms  []RoomDef        `json:"rooms"`
}
type FloorDef struct {
	ExcludedRandomKinds []string `json:"excluded_random_kinds,omitempty"`
	ID                  string   `json:"id"`
	Map                 string   `json:"map"`
	PersonalAP          int      `json:"personal_ap"`
	SharedAP            int      `json:"shared_ap"`
	EnhancedRoom        string   `json:"enhanced_room"`
}
type MapTemplateDef struct {
	Generator *MapGeneratorDef `json:"generator,omitempty"`
	ID        string           `json:"id"`
	Start     string           `json:"start"`
	Boss      string           `json:"boss"`
	Nodes     []MapNodeDef     `json:"nodes"`
	Terrain   []Terrain        `json:"terrain,omitempty"`
}
type MapGeneratorDef struct {
	BossApproach    string   `json:"boss_approach,omitempty"`
	Count           CountDef `json:"count"`
	Pool            []string `json:"pool"`
	Passage         string   `json:"passage,omitempty"`
	ExtraEdgeChance float64  `json:"extra_edge_chance"`
}
type MapNodeDef struct {
	ID         string   `json:"id"`
	Definition string   `json:"definition"`
	Q          int      `json:"q"`
	R          int      `json:"r"`
	Edges      []string `json:"edges"`
}
type NodeDef struct {
	WeaponUpgrade bool           `json:"weapon_upgrade,omitempty"`
	Blocking      *bool          `json:"blocking,omitempty"`
	Hexes         []HexCell      `json:"hexes,omitempty"`
	Shop          []ShopEntry    `json:"shop,omitempty"`
	Facility      string         `json:"facility,omitempty"`
	FacilityCost  int            `json:"facility_cost,omitempty"`
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	NameKey       string         `json:"name_key"`
	Hint          string         `json:"hint"`
	HintKey       string         `json:"hint_key"`
	Payment       string         `json:"payment"`
	Cost          int            `json:"cost"`
	Room          string         `json:"room,omitempty"`
	Handler       string         `json:"handler,omitempty"`
	Parameters    map[string]any `json:"parameters,omitempty"`
}
type CountDef struct {
	Base      int `json:"base"`
	PerPlayer int `json:"per_player"`
	PerFloor  int `json:"per_floor"`
}

func (c CountDef) Resolve(floor, players int) int {
	return c.Base + c.PerPlayer*players + c.PerFloor*floor
}

type SpawnGroupDef struct {
	Enemy string   `json:"enemy"`
	Count CountDef `json:"count"`
}
type WeightedDrop struct {
	Item   string  `json:"item"`
	Weight float64 `json:"weight"`
}
type LootDraw struct {
	Rolls  int            `json:"rolls"`
	Chance float64        `json:"chance"`
	Items  []WeightedDrop `json:"items"`
}
type RewardDef struct {
	Draws       []LootDraw `json:"draws,omitempty"`
	Items       []DropDef  `json:"items,omitempty"`
	XPPolicy    string     `json:"xp_policy,omitempty"`
	CoinsPolicy string     `json:"coins_policy,omitempty"`
	XP          CountDef   `json:"xp"`
	Coins       CountDef   `json:"coins"`
}
type WaveDef struct {
	Enemies      []SpawnGroupDef `json:"enemies"`
	BreakSeconds float64         `json:"break_seconds"`
	Reward       RewardDef       `json:"reward"`
}
type ScalingDef struct {
	DamagePerFloor   float64 `json:"damage_per_floor"`
	HPFactor         float64 `json:"hp_factor"`
	HPVariation      float64 `json:"hp_variation,omitempty"`
	HPPerFloor       float64 `json:"hp_per_floor"`
	HPPerExtraPlayer float64 `json:"hp_per_extra_player"`
}
type RoomDef struct {
	Hexes            []HexCell  `json:"hexes,omitempty"`
	ObjectiveSeconds float64    `json:"objective_seconds,omitempty"`
	Obstacles        []Obstacle `json:"obstacles,omitempty"`
	ID               string     `json:"id"`
	Width            float64    `json:"width"`
	Height           float64    `json:"height"`
	Objective        string     `json:"objective"`
	Join             string     `json:"join"`
	InitialDelay     float64    `json:"initial_delay"`
	SpawnRadius      Vec        `json:"spawn_radius"`
	Scaling          ScalingDef `json:"scaling"`
	Waves            []WaveDef  `json:"waves"`
}

// HexCell is a room-local axial coordinate, independent of the strategic map.
// An omitted layout is one hexagon; explicit layouts must include (0, 0).
type HexCell struct {
	Q int `json:"q"`
	R int `json:"r"`
}

type Obstacle struct {
	Position Vec     `json:"position"`
	Radius   float64 `json:"radius"`
}
