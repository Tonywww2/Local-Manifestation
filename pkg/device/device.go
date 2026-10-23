// Package device defines world interactions as detached values. Hosts own actions;
// the same positions and ranges drive rendering, picking and server validation.
package device

import (
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
)

type Device struct {
	ID, Kind, Name, NameKey, Description, DescriptionKey string
	Position                                             model.Vec
	Range, Radius                                        float64
	Action, Target                                       string
}

func (d Device) Near(point model.Vec) bool {
	return point.Finite() && point.Sub(d.Position).Length() <= d.Range
}

func (d Device) Interactive() bool { return d.Action != "" }

// Entrance specializes Device using composition, Go's equivalent of a base type.
// Payment and battle rules remain in the authoritative expedition/scene owners.
type Entrance struct {
	Device
	RoomKind, Payment string
	Cost              int
}

func RoomEntrance(from, to model.Node, a geometry.Arena) (Entrance, bool) {
	side, ok := geometry.DoorSide(from, to)
	if !ok {
		return Entrance{}, false
	}
	action := "enter"
	if to.Kind == "boss" || to.Payment == "shared" && !to.Open && !to.Completed {
		action = "vote"
	}
	if expedition.FreeTransit(to) {
		action = ""
	}
	kind := "event"
	if to.Room != "" {
		kind = "battle"
	}
	if to.Kind == "boss" {
		kind = "boss"
	}
	if to.Kind == "passage" || to.Kind == "start" {
		kind = "passage"
	}
	d := Device{ID: "entrance:" + to.ID, Kind: "entrance", Name: to.Name, NameKey: to.NameKey,
		Description: to.Hint, DescriptionKey: to.HintKey, Position: a.DoorPoint(side, 24), Range: 42, Radius: 18, Action: action, Target: to.ID}
	if d.Description == "" && d.DescriptionKey == "" {
		d.DescriptionKey = "device.entrance." + kind
	}
	return Entrance{Device: d, RoomKind: kind, Payment: to.Payment, Cost: to.Cost}, true
}

type Context struct {
	Node      model.Node
	Nodes     []model.Node
	Arena     geometry.Arena
	Phase     string
	SharedAP  int
	Emergency string
}

func World(c Context) []Device {
	var result []Device
	if c.Node.Kind == "shop" && c.Node.Open {
		result = append(result, Device{ID: "shop:" + c.Node.ID, Kind: "shop", Name: c.Node.Name, NameKey: c.Node.NameKey, DescriptionKey: "device.shop.description", Position: c.Arena.Center, Range: 64, Radius: 24, Action: "shop", Target: c.Node.ID})
	}
	if c.Node.Kind == "revival" && c.Node.Open {
		result = append(result, Device{ID: "revival:" + c.Node.ID, Kind: "revival", NameKey: "ui.revival", DescriptionKey: "device.revival.description", Position: c.Arena.Center, Range: 64, Radius: 24, Action: "revival", Target: c.Node.ID})
	}
	if c.Phase == "intermission" && c.Node.Kind == "boss" {
		result = append(result, Device{ID: "next", Kind: "exit", NameKey: "ui.next", DescriptionKey: "device.next.description", Position: c.Arena.ExitDevice(), Range: 42, Radius: 18, Action: "vote", Target: "next"})
	}
	emergency := c.Emergency
	if c.Phase == "exploring" && c.SharedAP == 0 {
		emergency = "enhanced"
	}
	if emergency != "" {
		result = append(result, Device{ID: emergency, Kind: "exit", NameKey: "ui." + emergency, DescriptionKey: "device." + emergency + ".description", Position: c.Arena.EmergencyDevice(), Range: 42, Radius: 18, Action: "vote", Target: emergency})
	}
	return result
}

func Find(values []Device, id string) (Device, bool) {
	for _, d := range values {
		if d.ID == id {
			return d, true
		}
	}
	return Device{}, false
}

func Nearest(values []Device, point model.Vec) (Device, bool) {
	var result Device
	distance := 0.0
	for _, d := range values {
		if !d.Interactive() {
			continue
		}
		n := point.Sub(d.Position).Length()
		if d.Near(point) && (result.ID == "" || n < distance) {
			result, distance = d, n
		}
	}
	return result, result.ID != ""
}
