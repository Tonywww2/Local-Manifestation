// Package protocol carries intents and detached server results.
package protocol

import (
	"encoding/json"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
)

// Version 19 adds shared dropped items and skeletal enemy action timelines.
const Version = 19
const MaxMessage = 4 * 1024 * 1024

type Hello struct {
	Request    string         `json:"request,omitempty"`
	SHA256     string         `json:"sha256,omitempty"`
	AdminToken string         `json:"admin_token,omitempty"`
	MetaRanks  map[string]int `json:"meta_ranks,omitempty"`
	Version    int            `json:"version"`
	Name       string         `json:"name"`
	Chassis    string         `json:"chassis"`
	Digest     string         `json:"digest"`
	Token      string         `json:"token,omitempty"`
}
type Request struct {
	Probe    uint64      `json:"probe,omitempty"`
	Join     bool        `json:"join,omitempty"`
	Area     string      `json:"area,omitempty"`
	Right    bool        `json:"right,omitempty"`
	Item     string      `json:"item,omitempty"`
	Slot     int         `json:"slot,omitempty"`
	Peer     string      `json:"peer,omitempty"`
	Items    []string    `json:"items,omitempty"`
	Revision uint64      `json:"revision,omitempty"`
	Seq      uint64      `json:"seq"`
	Kind     string      `json:"kind"`
	Node     string      `json:"node,omitempty"`
	Accept   bool        `json:"accept,omitempty"`
	Input    model.Input `json:"input,omitempty"`
}
type Vote struct {
	Item     string            `json:"item,omitempty"`
	Owner    string            `json:"owner,omitempty"`
	Tools    map[string]string `json:"tools,omitempty"`
	Entrants []string          `json:"entrants,omitempty"`
	Node     string            `json:"node"`
	Eligible []string          `json:"eligible"`
	Yes      []string          `json:"yes"`
	No       []string          `json:"no"`
	Required int               `json:"required"`
}
type Scene struct {
	EnemyActions       []model.EnemyActionState  `json:"enemy_actions,omitempty"`
	Actions            []model.PlayerActionState `json:"actions,omitempty"`
	Performance        perf.Snapshot             `json:"performance,omitzero"`
	Hexes              []model.HexCell           `json:"hexes,omitempty"`
	ObjectiveRemaining float64                   `json:"objective_remaining,omitempty"`
	Obstacles          []model.Obstacle          `json:"obstacles,omitempty"`
	AdventureTime      int64                     `json:"adventure_time"`
	Countdown          float64                   `json:"countdown,omitempty"`
	TickRate           int                       `json:"tick_rate"`
	ActualTPS          float64                   `json:"actual_tps"`
	DroppedTicks       uint64                    `json:"dropped_ticks"`
	Room               string                    `json:"room"`
	ID                 string                    `json:"id"`
	State              string                    `json:"state"`
	Width              float64                   `json:"width"`
	Height             float64                   `json:"height"`
	Wave               int                       `json:"wave"`
	Waves              int                       `json:"waves"`
	Tick               uint64                    `json:"tick"`
	Players            []model.Player            `json:"players"`
	Enemies            []model.Enemy             `json:"enemies"`
	Projectiles        []model.Projectile        `json:"projectiles"`
	LastDamage         *damage.Result            `json:"last_damage,omitempty"`
	Error              string                    `json:"error,omitempty"`
}
type State struct {
	GroundLoot       []model.Loot    `json:"ground_loot,omitempty"`
	Performance      perf.Snapshot   `json:"performance,omitzero"`
	Process          perf.Process    `json:"process,omitzero"`
	LayerMembers     []string        `json:"layer_members"`
	SpecialMode      bool            `json:"special_mode"`
	BackpackCapacity int             `json:"backpack_capacity"`
	AccessorySlots   int             `json:"accessory_slots"`
	RunID            string          `json:"run_id"`
	Started          string          `json:"started"`
	ProgressionScope string          `json:"progression_scope"`
	MetaRules        model.MetaRules `json:"meta_rules"`
	MapPlayers       int             `json:"map_players"`
	Trades           []Trade         `json:"trades,omitempty"`
	AdventureTime    int64           `json:"adventure_time"`
	ServerTick       uint64          `json:"server_tick"`
	TickRate         int             `json:"tick_rate"`
	SnapshotRate     int             `json:"snapshot_rate"`
	ActualTPS        float64         `json:"actual_tps"`
	DroppedTicks     uint64          `json:"dropped_ticks"`
	Phase            string          `json:"phase"`
	Floor            int             `json:"floor"`
	Floors           int             `json:"floors"`
	SharedAP         int             `json:"shared_ap"`
	PersonalInitial  int             `json:"personal_initial"`
	SharedInitial    int             `json:"shared_initial"`
	Cursed           bool            `json:"cursed"`
	Nodes            []model.Node    `json:"nodes"`
	Terrain          []model.Terrain `json:"terrain,omitempty"`
	Players          []model.Player  `json:"players"`
	Scenes           []Scene         `json:"scenes"`
	Vote             *Vote           `json:"vote,omitempty"`
	Admin            string          `json:"admin"`
}

type Trade struct {
	ID                 string      `json:"id"`
	Players            [2]string   `json:"players"`
	Offers             [2][]string `json:"offers"`
	InventoryRevisions [2]uint64   `json:"inventory_revisions"`
	Accepted           [2]bool     `json:"accepted"`
	Revision           uint64      `json:"revision"`
}
type Message struct {
	Probe  uint64            `json:"probe,omitempty"`
	Size   int64             `json:"size,omitempty"`
	Export json.RawMessage   `json:"export,omitempty"`
	Kind   string            `json:"kind"`
	Player string            `json:"player,omitempty"`
	Token  string            `json:"token,omitempty"`
	Text   string            `json:"text,omitempty"`
	Code   string            `json:"code,omitempty"`
	Params map[string]string `json:"params,omitempty"`
	State  *State            `json:"state,omitempty"`
}
