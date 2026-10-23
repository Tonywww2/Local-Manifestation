package client

import (
	"fmt"
	"strconv"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
)

// Combat HUD uses the same controls for play and editing. Preview values never
// enter the authoritative snapshot or produce gameplay commands.
func (g *Game) buildCombatHUD(f *uiFrame, layout ui.Layout, p model.Player, preview bool) {
	floor, floors, sharedAP, cursed := 1, 5, 3, false
	phase := "exploring"
	roomName := g.tr("ui.layout.preview_room")
	if !preview {
		floor, floors, sharedAP, cursed = g.state.Floor, g.state.Floors, g.state.SharedAP, g.state.Cursed
		roomName = g.nodeName(p.Node)
		phase = g.state.Phase
	}
	bounds := g.uiBounds()
	addPanel := func(id, title string, size ui.Size, p ui.Placement, floating bool) ui.Rect {
		r := ui.At(layout, id, p, size, bounds)
		f.Panels = append(f.Panels, ui.Panel{ID: id, Bounds: r, Floating: floating})
		f.Titles[id] = title
		return r
	}
	label := func(r ui.Rect, value string) {
		f.Labels = append(f.Labels, uiLabel{Owner: f.Panels[len(f.Panels)-1].ID, Bounds: r, Text: value, Size: 12})
	}
	control := func(owner, id, icon, key, action, value, title string, r ui.Rect, summary, details []string) {
		f.Controls = append(f.Controls, uiControl{ID: id, Owner: owner, Icon: icon, Key: key, Action: action, Value: value, Title: title, Bounds: r, Summary: summary, Details: details})
	}
	{
		r := addPanel("consumables", g.tr("ui.consumables"), ui.Size{W: 248, H: 60}, ui.Placement{Anchor: "top_left", X: 18, Y: 18}, false)
		for i := 0; i < 4; i++ {
			id := fmt.Sprintf("item%d", i+1)
			it, _ := inventory.Find(p.Inventory, p.Inventory.Equipment[5+i])
			c := g.itemControl("consumables", id, it, ui.Rect{X: r.X + float64(i)*62, Y: r.Y, W: 56, H: 60}, "item-use")
			c.Value, c.Key, c.Slot = strconv.Itoa(i), g.bindingLabel(g.bound(id)), "consumable"
			f.Controls = append(f.Controls, c)
		}
		journeyPlacement, journeyWidth := ui.Placement{Anchor: "top_center", Y: 18}, 260.0
		if g.width < 820 {
			journeyPlacement = ui.Placement{Anchor: "top_left", X: 18, Y: 166}
			journeyWidth = 208
		}
		r = addPanel("journey", g.tr("ui.journey"), ui.Size{W: journeyWidth, H: 42}, journeyPlacement, false)
		label(ui.Rect{X: r.X + 10, Y: r.Y + 5, W: r.W - 20, H: 18}, g.tr("ui.journey.floor", "floor", floor, "floors", floors))
		label(ui.Rect{X: r.X + 10, Y: r.Y + 23, W: r.W - 20, H: 16}, roomName)
		control("journey", "journey", "map", "", "", "", g.tr("ui.journey"), r, []string{g.tr("phase." + phase)}, nil)
		if s := g.currentScene(); !preview && s != nil && s.Waves > 0 {
			f.Controls[len(f.Controls)-1].Summary = append(f.Controls[len(f.Controls)-1].Summary, g.tr("ui.wave.progress", "wave", s.Wave, "waves", s.Waves))
			f.Labels[len(f.Labels)-1].Text = g.tr("ui.wave.progress", "wave", s.Wave, "waves", s.Waves)
		}
		r = addPanel("minimap", g.tr("ui.map"), ui.Size{W: 224, H: 152}, ui.Placement{Anchor: "top_right", X: -18, Y: 18}, false)
		label(ui.Rect{X: r.X + 10, Y: r.Y + r.H - 25, W: r.W - 20, H: 20}, g.tr("ui.minimap.ap", "personal", p.AP, "shared", sharedAP))
		control("minimap", "minimap", "map", g.bindingLabel(g.bound("map")), "window", "map", g.tr("ui.map"), r, nil, nil)
		if !preview {
			g.addMinimapNodeControls(f, r)
		}
		g.buildCameraControl(f, layout)
		tools := addPanel("tools", "", ui.Size{W: 38, H: 38}, ui.Placement{Anchor: "top_right", X: -18, Y: 180}, false)
		control("tools", "settings", "settings", g.bindingLabel(g.bound("settings")), "window", "settings", g.tr("ui.settings"), tools, nil, nil)
		peers := g.orderedPeers()
		rows := min(len(peers), max(1, (g.height-356)/52))
		if g.u.editor != nil {
			rows = max(2, rows)
		}
		r = addPanel("party", g.tr("ui.team"), ui.Size{W: 224, H: float64(max(1, rows) * 52)}, ui.Placement{Anchor: "top_right", X: -18, Y: 226}, false)
		start := min(g.u.scroll["party"], max(0, len(peers)-rows))
		for i, v := range peers[start:min(start+rows, len(peers))] {
			row := ui.Rect{X: r.X, Y: r.Y + float64(i)*52, W: r.W, H: 46}
			title, short, detail := g.windowInfo("peer:" + v.ID)
			control("party", "peer:"+v.ID, "person", "", "window", "peer:"+v.ID, title, row, short, detail)
			f.Controls[len(f.Controls)-1].Peer = g.hudPeer(v)
		}
		if g.u.editor != nil && len(peers) == 0 {
			for i := 0; i < rows; i++ {
				f.Controls = append(f.Controls, uiControl{Owner: "party", ID: fmt.Sprintf("hud-preview:peer:%d", i), Bounds: ui.Rect{X: r.X, Y: r.Y + float64(i)*52, W: r.W, H: 46}, Peer: &hudPeerView{Name: g.tr("ui.layout.preview_peer", "number", i+1), Location: g.tr("ui.peer.same"), HP: 75, MaxHP: 100, SameRoom: i == 0}})
			}
		}
		cardWidth := min(264, float64(g.width)*.325)
		// Keep the skill and status card clear of the resource bars at small widths.
		hudRight := bounds.W - cardWidth - 106
		resourceWidth := min(400, hudRight-18)
		resourceCenter := min(bounds.W/2, hudRight-resourceWidth/2)
		barWidth := resourceWidth - 60
		legacy := ui.At(layout, "resources", ui.Placement{Anchor: "bottom_left", X: resourceCenter - resourceWidth/2, Y: -18}, ui.Size{W: resourceWidth, H: 50}, bounds)
		r = addPanel("health", g.tr("ui.layout.health"), ui.Size{W: barWidth + 8, H: 29}, ui.Placement{Anchor: "bottom_center", X: legacy.X + 4 + barWidth/2 - bounds.W/2, Y: legacy.Y + 29 - bounds.H}, false)
		healthText := fmt.Sprintf("%.2f / %.2f", p.HP, p.MaxHP)
		f.Meters = append(f.Meters, uiMeter{Owner: "health", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 3, W: barWidth, H: 19}, Fraction: p.HP / mathMaxHP(p.MaxHP)})
		if labelFits(healthText, barWidth-12, 12) {
			f.Labels = append(f.Labels, uiLabel{Owner: "health", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 3, W: barWidth, H: 19}, Text: healthText, Size: 12, Centered: true})
		}
		if shield := status.ShieldTotal(p.Statuses); shield > 0 {
			f.Meters = append(f.Meters, uiMeter{Owner: "health", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 25, W: barWidth, H: 3}, Fraction: min(1, shield/max(1, p.MaxHP)), Kind: "shield"})
		}
		healthBounds := r
		r = addPanel("energy", g.tr("ui.layout.energy"), ui.Size{W: barWidth + 8, H: 20}, ui.Placement{Anchor: "bottom_center", X: legacy.X + 4 + barWidth/2 - bounds.W/2, Y: legacy.Y + 50 - bounds.H}, false)
		f.Meters = append(f.Meters, uiMeter{Owner: "energy", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 3, W: barWidth, H: 12}, Fraction: p.Energy / max(1, p.MaxEnergy), Kind: "energy"})
		f.Labels = append(f.Labels, uiLabel{Owner: "energy", Bounds: ui.Rect{X: r.X + 4, Y: r.Y, W: barWidth, H: 16}, Text: fmt.Sprintf("%.0f / %.0f", p.Energy, p.MaxEnergy), Size: 11, Centered: true})
		r = addPanel("dodges", g.tr("ui.layout.dodges"), ui.Size{W: 50, H: 29}, ui.Placement{Anchor: "bottom_center", X: legacy.X + resourceWidth - 25 - bounds.W/2, Y: legacy.Y + 29 - bounds.H}, false)
		for i := 0; i < 2; i++ {
			f.Dots = append(f.Dots, uiDot{Owner: "dodges", Center: ui.Point{X: r.X + 12 + float64(i)*21, Y: r.Y + 13}, Available: i < p.Dodges})
		}
		title, short, detail := g.windowInfo("self")
		control("health", "self", "heart", "", "", "", title, healthBounds, short, detail)
		statusWidth := min(348, resourceWidth)
		r = addPanel("statuses", g.tr("ui.statuses"), ui.Size{W: statusWidth, H: 48}, ui.Placement{Anchor: "bottom_left", X: resourceCenter - statusWidth/2, Y: -80}, false)
		statusControlStart := len(f.Controls)
		i := 0
		for _, id := range []string{"penalty", "curse"} {
			if id == "penalty" && p.Downs == 0 || id == "curse" && !cursed {
				continue
			}
			title, short, detail := g.windowInfo(id)
			control("statuses", id, "status", "", "", "", title, ui.Rect{X: r.X + float64(i)*54, Y: r.Y, W: 48, H: 46}, short, detail)
			f.Controls[len(f.Controls)-1].Badge = "∞"
			i++
		}
		statusSlots := max(3, int((r.W+6)/54)) - i
		moreStatuses := len(g.u.statusControls) > statusSlots
		if moreStatuses {
			statusSlots--
		}
		statusStart := min(g.u.scroll["statuses"], max(0, len(g.u.statusControls)-statusSlots))
		for _, status := range g.u.statusControls[statusStart:min(statusStart+statusSlots, len(g.u.statusControls))] {
			status.Bounds = ui.Rect{X: r.X + float64(i)*54, Y: r.Y, W: 48, H: 46}
			f.Controls = append(f.Controls, status)
			i++
		}
		if moreStatuses {
			summary := []string{g.tr("ui.status.scroll")}
			for _, row := range g.u.statusControls {
				summary = append(summary, row.Title+" · "+row.Badge)
			}
			control("statuses", "statuses:more", "status", "", "", "", g.tr("ui.status.more"), ui.Rect{X: r.X + float64(i)*54, Y: r.Y, W: 48, H: 46}, summary, nil)
			f.Controls[len(f.Controls)-1].Badge = fmt.Sprintf("+%d", len(g.u.statusControls)-statusSlots)
		}
		statusOffset := (r.W - float64(len(f.Controls)-statusControlStart)*54 + 6) / 2
		for j := statusControlStart; j < len(f.Controls); j++ {
			f.Controls[j].Bounds.X += statusOffset
		}
		if g.u.editor != nil && len(f.Controls) == statusControlStart {
			for i := 0; i < 3; i++ {
				control("statuses", fmt.Sprintf("hud-preview:status:%d", i), "status", "", "", "", "", ui.Rect{X: r.X + r.W/2 - 78 + float64(i)*54, Y: r.Y, W: 48, H: 46}, nil, nil)
				f.Controls[len(f.Controls)-1].Badge = "1 · 30s"
			}
		}
		r = addPanel("equipment", g.tr("ui.backpack_status"), ui.Size{W: cardWidth, H: 98}, ui.Placement{Anchor: "bottom_right", X: -18, Y: -18}, false)
		f.Controls = append(f.Controls, g.inventoryStatusControl(r, p))
		r = addPanel("weapon-skill", g.tr("ui.weapon_skill"), ui.Size{W: 64, H: 60}, ui.Placement{Anchor: "bottom_right", X: -30 - cardWidth, Y: -18}, false)
		f.Controls = append(f.Controls, g.weaponSkillControl(p, "weapon-skill", "weapon_skill", r))
		if drop, ok := g.nearLoot(); !preview && ok {
			r = addPanel("entrance-prompt", "", ui.Size{W: 320, H: 44}, ui.Placement{Anchor: "bottom_center", Y: -140}, false)
			item := g.itemControl("entrance-prompt", "loot-prompt", drop.Item, ui.Rect{X: r.X + 268, Y: r.Y + 4, W: 44, H: 36}, "item-pickup")
			item.Key = g.bindingLabel(g.bound("interact"))
			label(ui.Rect{X: r.X + 8, Y: r.Y + 4, W: 250, H: 36}, item.Title)
			f.Controls = append(f.Controls, item)
		} else if d, ok := g.nearInteraction(); !preview && ok {
			r = addPanel("entrance-prompt", "", ui.Size{W: 320, H: 44}, ui.Placement{Anchor: "bottom_center", Y: -140}, false)
			label(ui.Rect{X: r.X + 8, Y: r.Y + 4, W: 250, H: 36}, g.deviceName(d))
			control("entrance-prompt", "entrance-open", "door", g.bindingLabel(g.bound("interact")), "world-device", d.ID, g.deviceName(d), ui.Rect{X: r.X + 268, Y: r.Y + 4, W: 44, H: 36}, nil, []string{g.deviceDescription(d)})
		} else if g.u.editor != nil {
			r = addPanel("entrance-prompt", "", ui.Size{W: 320, H: 44}, ui.Placement{Anchor: "bottom_center", Y: -140}, false)
			label(r.Inset(8), g.tr("ui.layout.interaction"))
			control("entrance-prompt", "hud-preview:interaction", "door", g.bindingLabel(g.bound("interact")), "", "", "", ui.Rect{X: r.X + 268, Y: r.Y + 4, W: 44, H: 36}, nil, nil)
		}
	}
	if g.u.editor != nil || g.showFPS || g.showLatency || g.performanceVisible {
		addPanel("performance-hud", "", ui.Size{W: 246, H: 26}, ui.Placement{Anchor: "bottom_left", X: 18, Y: -68}, false)
	}
	if g.performanceVisible {
		r := addPanel("developer-tools", "", ui.Size{W: 38, H: 38}, ui.Placement{Anchor: "bottom_left", X: 18, Y: -18}, false)
		control("developer-tools", "developer", "tools", "F7", "window", "developer", g.tr("ui.developer"), r, nil, nil)
	}
}

type hudPeerView struct {
	Name, Location string
	HP, MaxHP      float64
	SameRoom       bool
}

func (g *Game) hudPeer(p model.Player) *hudPeerView {
	v := &hudPeerView{Name: p.Name, HP: p.HP, MaxHP: p.MaxHP, SameRoom: g.sameRoom(p), Location: g.tr("ui.peer.other")}
	if !p.Online {
		v.Name += " · " + g.tr("online.no")
	}
	if v.SameRoom {
		v.Location = g.tr("ui.peer.same")
	}
	if p.Life != "alive" {
		v.Location = g.tr("life." + p.Life)
	}
	return v
}
