package client

import (
	"fmt"
	"slices"
	"strings"

	"hsmultiplayer/internal/client/ui"
)

func (g *Game) addLabel(f *uiFrame, owner string, r ui.Rect, value string) {
	lines := g.cachedWrap([]string{value}, r.W, 13)
	limit := max(1, int(r.H/18))
	for i, line := range lines[:min(len(lines), limit)] {
		if i == limit-1 && len(lines) > limit {
			line += "…"
		}
		f.Labels = append(f.Labels, uiLabel{Owner: owner, Bounds: ui.Rect{X: r.X, Y: r.Y + float64(i)*18, W: r.W, H: 18}, Text: line, Size: 13})
	}
}
func addControl(f *uiFrame, owner, id, icon, key, action, value, title string, r ui.Rect) {
	f.Controls = append(f.Controls, uiControl{ID: id, Owner: owner, Icon: icon, Key: key, Action: action, Value: value, Title: title, Bounds: r})
}
func (g *Game) windowPanel(f *uiFrame, layout ui.Layout, id, title string, size ui.Size) ui.Rect {
	placement := ui.Placement{Anchor: "center"}
	if inspectionWindowID(id) {
		layout = g.u.inspectionLayout
	}
	if id == "room-settings" {
		placement = ui.Placement{Anchor: "top_right", X: -18, Y: 104}
	}
	if id == "map" {
		if saved, ok := g.u.windowSizes[id]; ok {
			size = ui.Size{W: max(420, saved.W), H: max(300, saved.H)}
		}
	}
	r := ui.At(layout, id, placement, size, g.uiBounds())
	f.Panels = append(f.Panels, ui.Panel{ID: id, Bounds: r, Floating: true})
	f.Titles[id] = title
	addControl(f, id, id+":close", "close", "", "close", id, g.tr("ui.close"), ui.Rect{X: r.X + r.W - 30, Y: r.Y + 3, W: 24, H: 24})
	if id == "backpack" || id == "trade" || id == "room-settings" || id == "settings" || id == "chassis-select" {
		if f.Viewports == nil {
			f.Viewports = map[string]windowViewport{}
		}
		f.Viewports[id] = windowViewport{Bounds: r, VirtualHeight: size.H, Footer: 60}
		r.H = size.H
	}
	return r
}
func (g *Game) buildWindow(f *uiFrame, layout ui.Layout, id string) {
	defer g.finishWindowContent(f, id)
	if id == developerItemsWindow {
		g.buildDeveloperItems(f, layout)
		return
	}
	if id == "performance" {
		g.buildPerformanceWindow(f, layout, id)
		return
	}
	if g.buildInspectionWindow(f, layout, id) {
		return
	}
	if g.buildMapActionWindow(f, layout, id) {
		return
	}
	if id == "mods" {
		g.buildMods(f, layout)
		return
	}
	if g.buildInventory(f, layout, id) {
		return
	}
	if id == "self" {
		g.buildCharacterStats(f, layout)
		return
	}
	if g.buildFrontWindow(f, layout, id) {
		return
	}
	size := ui.Size{W: 480, H: 320}
	if id == "developer" {
		size.H = 400
	}
	if id == "settings" {
		size = ui.Size{W: 520, H: 490}
	}
	if id == "map" {
		size = defaultMapWindowSize(g.uiBounds())
	}
	if id == "weapon" && g.u.expanded && g.u.focus == id {
		size = ui.Size{W: 620, H: 450}
	}
	title, short, detail := g.windowInfo(id)
	if id == "settings" {
		title = g.tr("ui.settings")
	}
	if id == "map" {
		title = g.tr("ui.map")
	}
	if id == "developer" {
		title = g.tr("ui.developer")
	}
	r := g.windowPanel(f, layout, id, title, size)
	if id == "shop" {
		if p := g.self(); p != nil {
			g.addWindowLines(f, id, r, []string{g.tr("ui.shop.balance", "coins", p.Coins), g.tr("ui.shop.upgrade")}, 140)
			if g.catalog.Node(g.currentNode().Definition).WeaponUpgrade {
				addControl(f, id, "shop:upgrade", "weapon", "", "shop-upgrade", "", g.tr("ui.upgrade"), ui.Rect{X: r.X + 14, Y: r.Y + r.H - 48, W: 38, H: 32})
			}
		}
		return
	}
	if id == "preparation" {
		if g.state == nil {
			return
		}
		f.Titles[id] = g.tr("ui.preparation.title")
		g.addWindowLines(f, id, r, []string{g.tr("ui.preparation.help")}, r.H-120)
		if g.state.Admin == g.id {
			addControl(f, id, "export", "save", "", "command", "export", g.tr("ui.checkpoint.export"), ui.Rect{X: r.X + 65, Y: r.Y + r.H - 48, W: 38, H: 32})
			addControl(f, id, "start", "approve", "", "command", "start", g.tr("ui.start"), ui.Rect{X: r.X + 14, Y: r.Y + r.H - 48, W: 38, H: 32})
		}
		return
	}
	if id == "settings" {
		g.buildSettings(f, r)
		return
	}
	if id == "map" {
		if g.state == nil {
			return
		}
		mapBounds := mapContentBounds(r)
		positions, radius := g.mapLayout(mapBounds, "map")
		for _, n := range g.state.Nodes {
			v := positions[n.ID]
			addControl(f, id, "map-node:"+n.ID, "", "", "map-node", n.ID, g.mapNodeLabel(n), ui.Rect{X: v.X - radius, Y: v.Y - radius, W: radius * 2, H: radius * 2})
			f.Controls[len(f.Controls)-1].Summary = g.nodeHover(n)
			f.Controls[len(f.Controls)-1].Clip = mapBounds
			f.Controls[len(f.Controls)-1].Hex = true
			f.Controls[len(f.Controls)-1].HexAngle = g.cameraAngle()
		}
		for _, t := range g.state.Terrain {
			v := positions[terrainID(t)]
			addControl(f, id, "map-node:"+terrainID(t), "", "", "", "", g.localized(t.NameKey, t.Name), ui.Rect{X: v.X - radius, Y: v.Y - radius, W: radius * 2, H: radius * 2})
			f.Controls[len(f.Controls)-1].Clip = mapBounds
			f.Controls[len(f.Controls)-1].Hex = true
			f.Controls[len(f.Controls)-1].HexAngle = g.cameraAngle()
		}
		g.buildMovementSelector(f, mapBounds)
		g.addMapResizeControl(f, r)
		return
	}
	if id == "developer" {
		g.addLabel(f, id, ui.Rect{X: r.X + 12, Y: r.Y + 38, W: r.W - 24, H: 48}, g.tr("ui.developer.help"))
		f.Controls = append(f.Controls, uiControl{ID: "dev:items", Owner: id, Action: "window", Value: developerItemsWindow, Title: g.tr("ui.developer.items"), Caption: g.tr("ui.developer.items"), Bounds: ui.Rect{X: r.X + 12, Y: r.Y + 292, W: r.W - 24, H: 34}})
		for i, action := range []string{"start", "rescue", "upgrade", "exhaust", "enhanced"} {
			g.addLabel(f, id, ui.Rect{X: r.X + 12, Y: r.Y + 98 + float64(i)*36, W: r.W - 64, H: 28}, g.tr("ui."+action))
			addControl(f, id, "dev:"+action, "tools", "", "command", action, g.tr("ui."+action), ui.Rect{X: r.X + r.W - 46, Y: r.Y + 96 + float64(i)*36, W: 30, H: 30})
		}
		if g.u.selectedNode != "" {
			g.addLabel(f, id, ui.Rect{X: r.X + 12, Y: r.Y + r.H - 46, W: r.W - 100, H: 30}, g.nodeName(g.u.selectedNode))
			addControl(f, id, "dev:enter", "door", "", "node", g.u.selectedNode, g.tr("ui.developer.enter"), ui.Rect{X: r.X + r.W - 88, Y: r.Y + r.H - 46, W: 30, H: 30})
		}
		return
	}
	if id == "armor" {
		for _, slot := range []string{"head", "body", "legs", "feet"} {
			short = append(short, g.tr("ui.armor."+slot)+": "+g.tr("ui.slot.empty"))
		}
	}
	if g.u.expanded && g.u.focus == id {
		short = append(short, detail...)
	}
	bottom := r.H - 64
	g.addWindowLines(f, id, r, short, bottom)
}
func (g *Game) addWindowLines(f *uiFrame, id string, r ui.Rect, lines []string, height float64) {
	wrapped := g.cachedWrap(lines, r.W-26, 13)
	visible := max(1, int(height/20))
	start := min(g.u.scroll[id], max(0, len(wrapped)-visible))
	for i, line := range wrapped[start:min(start+visible, len(wrapped))] {
		g.addLabel(f, id, ui.Rect{X: r.X + 12, Y: r.Y + 40 + float64(i)*20, W: r.W - 24, H: 20}, line)
	}
	if len(wrapped) > visible {
		g.addLabel(f, id, ui.Rect{X: r.X + 12, Y: r.Y + r.H - 24, W: r.W - 24, H: 20}, g.tr("ui.detail.scroll", "first", start+1, "last", min(start+visible, len(wrapped)), "count", len(wrapped)))
	}
}
func (g *Game) buildSettings(f *uiFrame, r ui.Rect) {
	id := "settings"
	s := g.u.settingsDraft
	if s == nil {
		return
	}
	row := func(i int, label, action, value, icon, key string) {
		y := r.Y + 42 + float64(i)*40
		w := 48.0
		if key != "" {
			w = 86
		}
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y + 5, W: r.W - w - 40, H: 30}, label)
		addControl(f, id, "settings:"+action+":"+value, icon, key, action, value, label, ui.Rect{X: r.X + r.W - w - 18, Y: y, W: w, H: 32})
	}
	switch g.u.settingsPage {
	case "home":
		for i, page := range []string{"language", "bindings", "layout", "performance"} {
			row(i, g.tr("ui.settings."+page), "settings-page", page, "settings", "")
		}

		row(6, g.tr("ui.mouse.click", "direct", g.tr("bool."+fmt.Sprint(g.clickDirect))), "click-mode", "", "view", "")
		row(7, g.tr("ui.mods"), "window", "mods", "settings", "")
		row(9, g.tr("ui.settings.rules"), "settings-page", "rules", "map", "")
		if g.state != nil && g.state.Admin == g.id {
			row(8, g.tr("ui.checkpoint.export"), "command", "export", "save", "")
		}
		if g.state != nil {
			row(5, g.tr("hall.return"), "front", "return", "door", "")
		} else if g.front.Page == "lobby" {
			row(5, g.tr("main.return"), "front", "main", "door", "")
		}
	case "performance":
		row(0, g.tr("ui.fps_limit", "limit", g.frameLimitLabel()), "fps", "", "view", "F4")
		row(1, g.tr("ui.vsync", "enabled", g.tr("bool."+fmt.Sprint(g.vsync))), "vsync", "", "view", "F5")
		for i, option := range []struct {
			key     string
			enabled bool
		}{{"fps", g.showFPS}, {"latency", g.showLatency}, {"diagnostics", g.performanceVisible}} {
			row(i+2, g.tr("ui.performance.show_"+option.key, "enabled", g.tr("bool."+fmt.Sprint(option.enabled))), "performance-toggle", option.key, "view", "")
		}
		row(5, g.tr("ui.performance.open"), "window", "performance", "tools", "")
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 294, W: r.W - 28, H: 76}, g.tr("ui.performance.note"))
	case "language":
		for i, lang := range g.translations.Languages() {
			name := g.translations.Name(lang)
			if lang == s.Language {
				name = "[ " + name + " ]"
			}
			row(i, name, "choose-language", lang, "check", "")
		}
	case "rules":
		g.addWindowLines(f, id, r, strings.Split(g.tr("ui.rules.body"), "\n"), r.H-100)
		addControl(f, id, "rules:back", "cancel", "", "settings-page", "home", g.tr("ui.back"), ui.Rect{X: r.X + 14, Y: r.Y + r.H - 42, W: 38, H: 30})
		return
	case "bindings":
		row(0, g.tr("ui.settings.mode", "mode", g.tr("ui.mode."+g.u.bindMode)), "binding-mode", "", "view", "")
		row(1, g.tr("ui.settings.group", "group", g.tr(fmt.Sprintf("ui.binding.group%d", g.u.bindGroup))), "binding-group", "", "settings", "")
		items := bindingGroups[g.u.bindGroup]
		index := 0
		for _, action := range items {
			code, ok := s.Bindings[g.u.bindMode][action]
			if !ok {
				continue
			}
			key := g.bindingLabel(code)
			if action == g.u.capture {
				key = "..."
			}
			row(index+2, g.tr("ui.action."+action), "capture", action, "key", key)
			index++
		}
		message := g.tr("ui.binding.help")
		if g.u.capture != "" {
			message = g.tr("ui.binding.capture")
		}
		if g.u.bindingError != "" {
			message = g.u.bindingError
		}
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + r.H - 94, W: r.W - 28, H: 40}, message)
	}
	if g.u.settingsPage != "home" {
		for i, action := range []string{"settings-save", "settings-cancel", "settings-reset"} {
			if action == "settings-reset" && g.u.settingsPage != "bindings" {
				continue
			}
			addControl(f, id, action, strings.TrimPrefix(action, "settings-"), "", action, "", g.tr("ui."+strings.TrimPrefix(action, "settings-")), ui.Rect{X: r.X + 14 + float64(i)*46, Y: r.Y + r.H - 42, W: 38, H: 30})
		}
	}
}
func (g *Game) buildHover(f *uiFrame) {
	var target *uiControl
	for i := range f.Controls {
		if f.Controls[i].ID == g.u.hover {
			target = &f.Controls[i]
			break
		}
	}
	for i := range f.Hints {
		if f.Hints[i].ID == g.u.hover {
			target = &f.Hints[i]
			break
		}
	}
	if target == nil {
		g.u.hover = ""
		return
	}
	lines := slices.Clone(target.Summary)
	if g.u.expanded {
		lines = append(lines, target.Details...)
	}
	if len(target.Details) > 0 && !g.u.expanded && !strings.HasPrefix(target.ID, "world-device:") {
		lines = append(lines, g.tr("ui.hover.shift", "key", g.bindingLabel(g.bound("detail"))))
	}
	if target.Key != "" {
		lines = append(lines, g.tr("ui.hover.shortcut", "key", target.Key))
	}
	// Size short hints to their content; keep formula rows readable at one font size.
	width := 220.0
	for _, line := range append(slices.Clone(lines), target.Title) {
		width = max(width, measuredText(line, 14)+32)
	}
	size := ui.Size{W: min(width, min(560, float64(g.width)-24))}
	rows := g.cachedWrap(lines, size.W-32, 14)
	limit := max(1, (g.height-80)/24)
	if len(rows) > limit {
		rows = slices.Clone(rows[:limit])
		rows[limit-1] += "…"
	}
	size.H = float64(len(rows)*24 + 58)
	if len(lines) == 0 || strings.HasPrefix(target.ID, "world-device:") && !g.u.expanded {
		size.H = 40
	}
	f.Tooltip = &uiTooltip{Bounds: ui.TooltipAt(g.u.cursor, size, g.uiBounds()), Title: target.Title, Rows: rows, RichRows: g.styledTooltipRows(rows)}
}
