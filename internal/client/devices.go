package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/device"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"image/color"
	"math"
)

func (g *Game) worldDevices(n model.Node, s protocol.Scene) []device.Device {
	emergency := ""
	if g.state.Vote != nil && (g.state.Vote.Node == "enhanced" || g.state.Vote.Node == "exhaust") {
		emergency = g.state.Vote.Node
	}
	return device.World(device.Context{Node: n, Nodes: g.state.Nodes, Arena: geometry.NewArena(s.Width, s.Height), Phase: g.state.Phase, SharedAP: g.state.SharedAP, Emergency: emergency})
}
func (g *Game) nearInteraction() (device.Device, bool) {
	p := g.self()
	s := g.currentScene()
	if p == nil || s == nil || p.Life != "alive" {
		return device.Device{}, false
	}
	values := g.worldDevices(g.currentNode(), *s)
	if p.Scene != "" || g.state.Phase != "exploring" && g.state.Phase != "intermission" {
		var allowed []device.Device
		for _, d := range values {
			if d.Kind != "entrance" && d.Kind != "shop" {
				allowed = append(allowed, d)
			}
		}
		values = allowed
	}
	return device.Nearest(values, p.Position)
}
func (g *Game) inspectInteraction(target string) {
	d, ok := g.nearInteraction()
	if !ok || d.Target != target {
		g.notify("error.door_distance")
		return
	}
	switch d.Action {
	case "vote":
		g.command("device", target, false)
	case "shop":
		g.openWindow("shop")
	case "revival":
		g.openWindow("revival")
	case "enter":
		g.u.entrance = target
		g.openWindow("entrance")
		g.destination = nil
	}
}
func (g *Game) deviceName(d device.Device) string { return g.localized(d.NameKey, d.Name) }
func (g *Game) deviceDescription(d device.Device) string {
	return g.localized(d.DescriptionKey, d.Description)
}
func (g *Game) drawDevice(dst *ebiten.Image, c Camera, d device.Device) {
	if !d.Interactive() {
		return
	}
	if g.deviceRing == nil {
		g.deviceRing = ebiten.NewImage(96, 96)
		vector.StrokeCircle(g.deviceRing, 48, 48, 42, 1.5, color.RGBA{R: 89, G: 221, B: 188, A: 100}, true)
	}
	v := c.Project(d.Position)
	if d.Kind == "hall" {
		v.Y -= hallBadgeLift(d.Action, c)
		size := max(24, 32*c.Scale)
		g.drawBox(dst, ui.Rect{X: v.X - size/2 - 4, Y: v.Y - size/2 - 4, W: size + 8, H: size + 8}, panel, uiEdge)
		g.drawIconSized(dst, d.Action, v.X-size/2, v.Y-size/2, size)
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-48, -48)
	if c.Oblique {
		op.GeoM.Skew(-math.Atan(.38), 0)
		op.GeoM.Scale(c.Scale, c.Scale*.68)
	} else {
		op.GeoM.Scale(c.Scale, c.Scale)
	}
	op.GeoM.Translate(v.X, v.Y)
	dst.DrawImage(g.deviceRing, op)
	worldBox(dst, c, d.Position, 34, 26, 18, workshopMetal)
	icon := "door"
	if d.Kind == "shop" {
		icon = "weapon"
	}
	if d.Kind == "hall" {
		icon = d.Action
	}
	g.drawBox(dst, ui.Rect{X: v.X - 15, Y: v.Y - 33, W: 30, H: 30}, panel, uiEdge)
	g.drawIconSized(dst, icon, v.X-10, v.Y-28, 20)
	if g.state != nil && g.state.Vote != nil && g.state.Vote.Node == d.Target {
		fitText(dst, g.tr("ui.device.vote", "yes", len(g.state.Vote.Yes), "required", g.state.Vote.Required), v.X-150, v.Y-37, 300, 14, true)
	}
}
func (g *Game) drawDevices(dst *ebiten.Image, n model.Node, s protocol.Scene, c Camera) {
	for _, d := range g.worldDevices(n, s) {
		g.drawDevice(dst, c, d)
	}
}

// World picking is below HUD/window panels. Passive entrances only provide hints.
func (g *Game) addDeviceControls(f *uiFrame, values []device.Device, c Camera) {
	for _, d := range values {
		v := c.Project(d.Position)
		radius := max(16, d.Radius*c.Scale)
		r := ui.Rect{X: v.X - radius, Y: v.Y - radius, W: radius * 2, H: radius * 2}
		if d.Kind == "hall" {
			r = ui.Rect{X: v.X - max(radius, 76*c.Scale), Y: v.Y - hallBadgeLift(d.Action, c) - 24, W: 2 * max(radius, 76*c.Scale), H: hallBadgeLift(d.Action, c) + 48}
		}
		x1, y1 := max(0, r.X), max(0, r.Y)
		x2, y2 := min(float64(g.width), r.X+r.W), min(float64(g.height), r.Y+r.H)
		if x2 <= x1 || y2 <= y1 {
			continue
		}
		r = ui.Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
		owner := "world-device:" + d.ID
		control := uiControl{ID: owner, Owner: owner, Value: d.ID, Title: g.deviceName(d), Details: []string{g.deviceDescription(d)}, Bounds: r}
		if !d.Interactive() {
			f.Hints = append(f.Hints, control)
			continue
		}
		control.Action = "world-device"
		f.Panels = append(f.Panels, ui.Panel{ID: owner, Bounds: r})
		f.Controls = append(f.Controls, control)
	}
}
func (g *Game) useWorldDevice(id string) {
	if g.state == nil {
		d, ok := device.Find(g.front.Devices, id)
		if ok && !d.Interactive() {
			return
		}
		if !ok || !d.Near(g.front.Position) {
			g.notify("error.door_distance")
			return
		}
		g.useHallDevice(d.Action)
		return
	}
	s := g.currentScene()
	if s == nil {
		return
	}
	if d, ok := device.Find(g.worldDevices(g.currentNode(), *s), id); ok && d.Interactive() {
		g.inspectInteraction(d.Target)
	}
}
