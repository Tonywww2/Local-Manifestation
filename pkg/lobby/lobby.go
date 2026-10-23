// Package lobby contains personal-hall values and existing server configuration.
package lobby

import (
	"fmt"
	"hsmultiplayer/pkg/device"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
)

type Config struct {
	Checkpoint       string           `json:"checkpoint,omitempty"`
	Combination      string           `json:"combination,omitempty"`
	AdminToken       string           `json:"-"`
	Restore          bool             `json:"restore"`
	SpecialMode      bool             `json:"special_mode"`
	BackpackCapacity int              `json:"backpack_capacity"`
	AccessorySlots   int              `json:"accessory_slots"`
	MetaRules        *model.MetaRules `json:"meta_rules,omitempty"`
	Players          int              `json:"players"`
	VotePercent      int              `json:"vote_percent"`
	TickRate         int              `json:"tick_rate"`
	SnapshotRate     int              `json:"snapshot_rate"`
	Port             int              `json:"port"`
}

func DefaultConfig() Config {
	return Config{Players: 8, VotePercent: 70, TickRate: 30, SnapshotRate: 20, Port: 7777, BackpackCapacity: 27, AccessorySlots: 12}
}
func (c Config) Validate() error {
	if c.SpecialMode && (c.BackpackCapacity < 1 || c.BackpackCapacity > 4096 || c.AccessorySlots < -1 || c.AccessorySlots > 4096) {
		return fmt.Errorf("invalid inventory configuration")
	}
	if c.Players < 1 || c.Players > 128 || c.VotePercent < 1 || c.VotePercent > 100 || (c.TickRate != 30 && c.TickRate != 60) || c.SnapshotRate < 1 || c.SnapshotRate > 60 || c.Port < 1024 || c.Port > 65535 {
		return fmt.Errorf("invalid room configuration")
	}
	return nil
}
func CloneConfig(c Config) Config {
	if c.MetaRules != nil {
		r := *c.MetaRules
		c.MetaRules = &r
	}
	return c
}

const Width, Height = 1800.0, 1640.0

const hallRadius = 260.0

func Spawn() model.Vec { return model.Vec{X: Width / 2, Y: Height / 2} }

var hallFootprint = func() geometry.Footprint {
	f, _ := geometry.NewFootprint(Spawn(), hallRadius, []geometry.Cell{{}, {Q: -1}, {R: -1}, {Q: 1, R: -1}, {Q: 1}, {R: 1}, {Q: -1, R: 1}})
	return f
}()

func Footprint() geometry.Footprint { return hallFootprint }

func Devices() []device.Device {
	var values []device.Device
	cells := [6]geometry.Cell{{Q: -1}, {Q: -1, R: 1}, {Q: 1}, {R: -1}, {Q: 1, R: -1}, {R: 1}}
	for i, id := range []string{"solo", "room-settings", "progression", "host", "join", "warehouse"} {
		// Facilities surround a clear walking area; these are interaction points.
		point := Spawn().Add(geometry.AxialPoint(cells[i].Q, cells[i].R).Scale(hallRadius * .85))
		values = append(values, device.Device{ID: "hall:" + id, Kind: "hall", NameKey: "hall." + id, DescriptionKey: "hall." + id + ".description", Position: point, Range: 85, Radius: 24, Action: id, Target: id})
	}
	return values
}

func Move(point, intent model.Vec, speed, dt float64) model.Vec {
	if !point.Finite() || !intent.Finite() || dt < 0 || dt > 1 || speed < 0 {
		return point
	}
	return Footprint().ClampDisk(point.Add(intent.Unit().Scale(speed*dt)), 12)
}

func Obstacles() []model.Obstacle {
	values := make([]model.Obstacle, 0, 6)
	for _, d := range Devices() {
		values = append(values, model.Obstacle{Position: d.Position, Radius: 32})
	}
	return values
}
