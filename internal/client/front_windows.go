package client

import (
	"fmt"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/model"
	"strconv"
	"strings"
)

// Front-end facilities share the draggable native window implementation.
func (g *Game) buildFrontWindow(f *uiFrame, layout ui.Layout, id string) bool {
	if id == "chassis-select" {
		g.buildChassisWindow(f, layout)
		return true
	}
	if id != "join" && id != "room-settings" && id != "progression" && id != "warehouse" && id != "achievements" && id != "room-capacity" && id != "room-meta" {
		return false
	}
	height := 420.0
	if id == "room-settings" {
		height = 570
	}
	width := 520.0
	if id == "room-settings" {
		width = 440
	}
	r := g.windowPanel(f, layout, id, g.tr("hall."+id), ui.Size{W: width, H: height})
	if id == "room-capacity" || id == "room-meta" {
		g.buildRoomExtras(f, id, r)
		return true
	}
	if id == "progression" || id == "warehouse" || id == "achievements" {
		g.buildProgression(f, id, r)
		return true
	}
	if id == "join" {
		for i, v := range []struct{ id, title, value string }{{"address", g.tr("ui.menu.address"), g.address}, {"name", g.tr("ui.menu.name"), g.name}, {"admin", g.tr("ui.menu.admin"), strings.Repeat("•", min(24, len(g.adminToken)))}} {
			y := r.Y + 45 + float64(i)*78
			g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y, W: r.W - 28, H: 24}, v.title)
			addControl(f, id, "join:"+v.id, "key", "", "front-field", v.id, v.title, ui.Rect{X: r.X + 14, Y: y + 26, W: r.W - 28, H: 36})
			f.Controls[len(f.Controls)-1].Caption = v.value
			if i == g.field {
				f.Controls[len(f.Controls)-1].Caption = "▸ " + v.value
			}
		}
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 307, W: r.W - 28, H: 42}, g.tr("hall.join.help"))
		addControl(f, id, "join:connect", "approve", "Enter", "join", "", g.tr("ui.menu.connect"), ui.Rect{X: r.X + 14, Y: r.Y + r.H - 52, W: 42, H: 32})
		addControl(f, id, "join:identity", "cancel", "", "identity-clear", "", g.tr("ui.menu.forget"), ui.Rect{X: r.X + 68, Y: r.Y + r.H - 52, W: 42, H: 32})
		return true
	}
	c := g.front.Draft
	if c == nil {
		return true
	}
	for i, v := range []struct {
		id    string
		value int
	}{{"players", c.Players}, {"vote", c.VotePercent}, {"tps", c.TickRate}, {"snapshots", c.SnapshotRate}, {"port", c.Port}} {
		y := r.Y + 44 + float64(i)*42
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y + 5, W: r.W - 122, H: 28}, g.tr("room."+v.id, "value", v.value))
		if v.id == "port" {
			addControl(f, id, "room:port-field", "key", "", "room-field", "port", g.tr("room.port", "value", v.value), ui.Rect{X: r.X + r.W - 220, Y: y, W: 112, H: 32})
			f.Controls[len(f.Controls)-1].Caption = g.front.PortText
		}
		for j, sign := range []string{"-1", "1"} {
			addControl(f, id, fmt.Sprintf("room:%s:%s", v.id, sign), "plus", "", "room-adjust", v.id+":"+sign, g.tr("room."+v.id, "value", v.value), ui.Rect{X: r.X + r.W - 96 + float64(j)*40, Y: y, W: 32, H: 32})
			f.Controls[len(f.Controls)-1].Caption = []string{"−", "+"}[j]
		}
	}
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 252, W: 150, H: 28}, g.tr("ui.menu.name"))
	addControl(f, id, "room:name-field", "key", "", "room-field", "name", g.tr("ui.menu.name"), ui.Rect{X: r.X + 164, Y: r.Y + 248, W: r.W - 178, H: 34})
	f.Controls[len(f.Controls)-1].Caption = g.front.Name
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 298, W: r.W - 28, H: 28}, g.tr("hall.chassis"))
	for i, ch := range g.catalog.ChassisList() {
		cardWidth := (r.W - 38) / 2
		addControl(f, id, "chassis:"+strconv.Itoa(i), presetIcon(ch.ID), "", "chassis", strconv.Itoa(i), g.contentName(ch.ID, ch.Name), ui.Rect{X: r.X + 14 + float64(i%2)*(cardWidth+10), Y: r.Y + 326 + float64(i/2)*38, W: cardWidth, H: 32})
		f.Controls[len(f.Controls)-1].Caption = g.contentName(ch.ID, ch.Name)
		if i == g.front.Chassis {
			f.Controls[len(f.Controls)-1].Caption = "[ " + f.Controls[len(f.Controls)-1].Caption + " ]"
		}
	}
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 412, W: 150, H: 30}, g.tr("room.checkpoint"))
	addControl(f, id, "room:checkpoint-field", "key", "", "room-field", "checkpoint", g.tr("room.checkpoint"), ui.Rect{X: r.X + 164, Y: r.Y + 410, W: r.W - 178, H: 34})
	f.Controls[len(f.Controls)-1].Caption = c.Checkpoint
	addControl(f, id, "room:save", "save", "", "room-save", "", g.tr("ui.save"), ui.Rect{X: r.X + 14, Y: r.Y + r.H - 52, W: 42, H: 32})
	for i, key := range []string{"room-capacity", "room-meta"} {
		addControl(f, id, key, "settings", "", "window", key, g.tr("hall."+key), ui.Rect{X: r.X + r.W - 100 + float64(i)*44, Y: r.Y + r.H - 52, W: 38, H: 32})
	}
	addControl(f, id, "room:restore", "save", "", "room-restore", "", g.tr("room.restore", "value", c.Restore), ui.Rect{X: r.X + r.W - 144, Y: r.Y + r.H - 52, W: 38, H: 32})
	g.addLabel(f, id, ui.Rect{X: r.X + 70, Y: r.Y + r.H - 52, W: r.W - 230, H: 38}, g.tr("room.before-start"))
	return true
}
func (g *Game) buildRoomExtras(f *uiFrame, id string, r ui.Rect) {
	c := g.front.Draft
	if c == nil {
		return
	}
	if id == "room-capacity" {
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 40, W: r.W - 28, H: 65}, g.tr("room.special.help"))
		addControl(f, id, "special", "settings", "", "room-special", "", g.tr("room.special", "value", c.SpecialMode), ui.Rect{X: r.X + 14, Y: r.Y + 110, W: 38, H: 32})
		g.addLabel(f, id, ui.Rect{X: r.X + 64, Y: r.Y + 110, W: r.W - 78, H: 30}, g.tr("room.special", "value", c.SpecialMode))
		for i, key := range []string{"backpack", "accessories"} {
			value := c.BackpackCapacity
			if i == 1 {
				value = c.AccessorySlots
			}
			y := r.Y + 170 + float64(i)*48
			g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y, W: r.W - 120, H: 30}, g.tr("room."+key, "value", value))
			for j, sign := range []string{"-1", "1"} {
				addControl(f, id, key+sign, "plus", "", "room-adjust", key+":"+sign, g.tr("room."+key, "value", value), ui.Rect{X: r.X + r.W - 94 + float64(j)*40, Y: y, W: 32, H: 32})
				f.Controls[len(f.Controls)-1].Caption = []string{"−", "+"}[j]
			}
		}
		return
	}
	rules := model.DefaultMetaRules()
	if c.MetaRules != nil {
		rules = *c.MetaRules
	}
	values := []bool{rules.Stats, rules.Coins, rules.Equipment, rules.AP, rules.Nodes, rules.Unlocks}
	for i, key := range []string{"stats", "coins", "equipment", "ap", "nodes", "unlocks"} {
		y := r.Y + 45 + float64(i)*44
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y, W: r.W - 75, H: 30}, g.tr("room.meta."+key, "value", values[i]))
		addControl(f, id, "meta:"+key, "settings", "", "room-meta", key, g.tr("room.meta."+key, "value", values[i]), ui.Rect{X: r.X + r.W - 52, Y: y, W: 38, H: 32})
	}
}
