package client

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"hsmultiplayer/internal/client/ui"
)

func hitControl(f uiFrame, p ui.Point) (uiControl, bool) {
	panel, ok := ui.HitPanel(f.Panels, p)
	if !ok {
		return uiControl{}, false
	}
	for i := len(f.Controls) - 1; i >= 0; i-- {
		c := f.Controls[i]
		if c.Owner == panel.ID && c.Bounds.Contains(p) && (c.Clip.W == 0 || c.Clip.Contains(p)) {
			return c, true
		}
	}
	return uiControl{}, false
}

// Hover hints never own input or window focus. Ordinary panels occlude world hints.
func hoverControl(f uiFrame, cursor ui.Point) (uiControl, bool) {
	if _, ok := ui.HitPanel(f.Panels, cursor); ok {
		return hitControl(f, cursor)
	}
	for i := len(f.Hints) - 1; i >= 0; i-- {
		if f.Hints[i].Bounds.Contains(cursor) {
			return f.Hints[i], true
		}
	}
	return uiControl{}, false
}

func (g *Game) updateHover(c uiControl, overControl bool) {
	if c.Caption != "" && len(c.Summary) == 0 && len(c.Details) == 0 {
		overControl = false
	}
	if overControl {
		g.u.hover = c.ID
	} else {
		g.u.hover = ""
	}
}

// Only panels and actionable controls participate in clicks. Tooltips and passive
// world hints are separate values and cannot start dragging or consume attacks.
func (g *Game) clickInterface(cursor ui.Point) bool {
	panel, overPanel := ui.HitPanel(g.u.frame.Panels, cursor)
	c, overControl := hitControl(g.u.frame, cursor)
	if !overPanel {
		return false
	}
	if panel.Floating {
		g.u.focus = panel.ID
		g.u.windows = slices.DeleteFunc(g.u.windows, func(id string) bool { return id == panel.ID })
		g.u.windows = append(g.u.windows, panel.ID)
	}
	if overControl && (g.u.editor == nil || panel.Floating || panel.ID == "editor") {
		g.performControl(c)
	} else if panel.ID != "editor" && (panel.Floating && panel.Bounds.Header().Contains(cursor) || g.u.editor != nil) {
		anchor := "top_left"
		layout := g.u.layout
		if g.u.editor != nil {
			layout = g.u.editor
		}
		if p, ok := layout[panel.ID]; ok {
			anchor = p.Anchor
		}
		g.u.drag = &ui.Drag{ID: panel.ID, Start: cursor, Bounds: panel.Bounds, Anchor: anchor}
		g.u.focus = panel.ID
	}
	return true
}
func (g *Game) captureBinding(code string) {
	s := g.u.settingsDraft
	if s == nil {
		return
	}
	next, conflict, err := rebind(s.Bindings, g.u.bindMode, g.u.capture, code)
	if err != nil {
		if conflict != "" {
			g.u.bindingError = g.tr("ui.binding.conflict", "action", g.tr("ui.action."+conflict))
		} else {
			g.u.bindingError = g.tr("ui.binding.reserved")
		}
		return
	}
	s.Bindings = next
	g.u.capture = ""
	g.u.bindingError = ""
}
func (g *Game) performControl(c uiControl) {
	if g.modAction(c) {
		return
	}
	if g.progressionAction(c) {
		return
	}
	if g.itemAction(c) {
		return
	}
	switch c.Action {
	case "inventory-page":
		key, direction, _ := strings.Cut(c.Value, ":")
		delta := 1
		if direction == "-1" {
			delta = -1
		}
		g.u.scroll[key] = min(max(0, g.u.scroll[key]+delta), max(0, c.Count-1))
	case "content-scroll":
		id, direction, _ := strings.Cut(c.Value, ":")
		amount := 100.0
		if direction == "-1" {
			amount = -amount
		}
		v := g.u.frame.Viewports[id]
		g.u.contentScroll[id] = max(0, min(v.MaxScroll, g.u.contentScroll[id]+amount))
	case "room-restore":
		g.front.Draft.Restore = !g.front.Draft.Restore
	case "room-special":
		g.front.Draft.SpecialMode = !g.front.Draft.SpecialMode
	case "room-meta":
		g.toggleMetaRule(c.Value)
	case "click-mode":
		s := g.preferences()
		s.ClickDirect = !s.ClickDirect
		if err := g.commitPreferences(s); err != nil {
			g.notifyError(err)
		}
		g.u.settingsDraft = &s
	case "front":
		g.performFront(c.Value)
	case "world-device":
		g.useWorldDevice(c.Value)
	case "front-field":
		g.field = 0
		if c.Value == "name" {
			g.field = 1
		}
		if c.Value == "admin" {
			g.field = 2
		}
	case "room-field":
		g.front.Editing = c.Value
		if c.Value == "port" {
			g.front.PortText = ""
		}
	case "chassis":
		g.chooseChassis(c.Value)
	case "chassis-preset":
		g.selectPreset(c.Value)
	case "room-adjust":
		g.adjustRoom(c.Value)
	case "room-save":
		g.saveRoomConfig()
	case "join":
		if !g.frontBusy() {
			g.connect()
		}
	case "identity-clear":
		delete(g.tokens, g.address)
		g.notify("notice.identity_cleared")
	case "shop-upgrade":
		g.command("upgrade", "", false)
	case "close":
		g.closeWindow(c.Value)
	case "window":
		g.toggleWindow(c.Value)
		g.u.hover = ""
	case "view":
		g.oblique = !g.oblique
	case "entrance":
		g.inspectInteraction(c.Value)
	case "enter-confirm":
		g.performControl(uiControl{Action: "node", Value: c.Value})
		g.closeWindow("entrance")
		g.u.entrance = ""
	case "vote":
		g.command("vote", "", c.Value == "true")
	case "attribute":
		g.command("attribute", c.Value, false)
	case "command":
		if c.Value == "exhaust" || c.Value == "enhanced" {
			g.command("propose", c.Value, false)
		} else {
			g.command(c.Value, "", false)
		}
	case "select-node":
		g.u.selectedNode = c.Value
	case "node":
		for _, n := range g.state.Nodes {
			if n.ID == c.Value {
				if n.Kind == "boss" || n.Payment == "shared" && !n.Open {
					g.command("propose", n.ID, false)
				} else {
					g.command("enter", n.ID, false)
				}
			}
		}
	case "settings-page":
		g.u.settingsPage = c.Value
		if c.Value == "layout" {
			g.u.editor = ui.Clone(g.u.layout)
			g.closeWindow("settings")
			for _, id := range []string{"backpack", "self", "map"} {
				g.openWindow(id)
			}
		}
	case "choose-language":
		g.u.settingsDraft.Language = c.Value
	case "binding-mode":
		if g.u.bindMode == "keyboard" {
			g.u.bindMode = "mouse"
		} else {
			g.u.bindMode = "keyboard"
		}
		g.u.settingsDraft.InputMode = g.u.bindMode
		g.u.capture = ""
		g.u.bindingError = ""
	case "binding-group":
		g.u.bindGroup = (g.u.bindGroup + 1) % len(bindingGroups)
		g.u.capture = ""
		g.u.bindingError = ""
	case "capture":
		g.u.capture = c.Value
		g.u.bindingError = ""
	case "settings-save":
		if err := g.commitPreferences(*g.u.settingsDraft); err != nil {
			g.notify("notice.settings_save", "detail", err.Error())
		} else {
			g.u.settingsPage = "home"
			s := g.preferences()
			g.u.settingsDraft = &s
			g.u.hover = ""
			g.notify("notice.settings_saved")
		}
	case "settings-cancel":
		s := g.preferences()
		g.u.settingsDraft = &s
		g.u.settingsPage = "home"
		g.u.capture = ""
		g.u.bindingError = ""
	case "settings-reset":
		g.u.settingsDraft.Bindings[g.u.bindMode] = defaultBindings()[g.u.bindMode]
		g.u.capture = ""
		g.u.bindingError = ""
	case "layout-save":
		s := g.preferences()
		s.Layout = ui.Clone(g.u.editor)
		if err := g.commitPreferences(s); err != nil {
			g.notify("notice.settings_save", "detail", err.Error())
		} else {
			g.u.editor = nil
			g.u.windows = nil
			g.showMap = false
			g.u.focus = ""
			g.notify("notice.settings_saved")
		}
	case "layout-cancel":
		g.u.editor = nil
		g.u.windows = nil
		g.showMap = false
		g.u.focus = ""
	case "layout-reset":
		g.u.editor = ui.Layout{}
	case "fps":
		g.cycleFPS()
		s := g.preferences()
		g.u.settingsDraft = &s
	case "vsync":
		g.toggleVSync()
		s := g.preferences()
		g.u.settingsDraft = &s
	}
}

// updateInterface consumes input before the host produces any gameplay intent.
func (g *Game) updateInterface() (mouseConsumed, keysConsumed bool) {
	mx, my := ebiten.CursorPosition()
	cursor := ui.Point{X: float64(mx), Y: float64(my)}
	g.u.cursor = cursor
	g.u.expanded = bindingDown(g.bound("detail")) || g.cfg.SmokeFrames > 0 && g.cfg.SmokeDetails
	g.buildInterface()
	if !ebiten.IsFocused() && g.cfg.SmokeFrames == 0 {
		g.u.hover = ""
		g.u.drag = nil
		g.u.capture = ""
		g.buildInterface()
		return true, true
	}
	if g.u.capture != "" {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.u.capture = ""
			g.u.bindingError = ""
		} else {
			keys := inpututil.AppendJustPressedKeys(nil)
			if len(keys) > 0 {
				key := keys[0]
				code := key.String()
				if key == ebiten.KeyShiftLeft || key == ebiten.KeyShiftRight {
					code = "Shift"
				}
				if key == ebiten.KeyControlLeft || key == ebiten.KeyControlRight {
					code = "Control"
				}
				if key == ebiten.KeyAltLeft || key == ebiten.KeyAltRight {
					code = "Alt"
				}
				g.captureBinding(code)
			}
			for _, b := range []ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonMiddle, ebiten.MouseButtonRight} {
				if inpututil.IsMouseButtonJustPressed(b) {
					g.captureBinding(fmt.Sprintf("Mouse%d", b))
					break
				}
			}
		}
		g.buildInterface()
		return true, true
	}
	if g.u.drag != nil {
		d := *g.u.drag
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if g.u.editor != nil {
				g.u.editor = ui.DragTo(g.u.editor, d, cursor, g.uiBounds())
			} else {
				g.u.layout = ui.DragTo(g.u.layout, d, cursor, g.uiBounds())
			}
		} else {
			g.u.drag = nil
			if g.u.editor == nil {
				if err := g.saveSettings(); err != nil {
					g.notify("notice.settings_save", "detail", err.Error())
				}
			}
		}
		g.buildInterface()
		return true, g.u.editor != nil || g.frontCapturesKeys()
	}
	_, pointerCaptured := ui.HitPanel(g.u.frame.Panels, cursor)
	if bindingAvailable(g.bound("settings"), pointerCaptured) && bindingPressed(g.bound("settings")) {
		if g.u.editor != nil {
			g.performControl(uiControl{Action: "layout-cancel"})
		} else if len(g.u.windows) > 0 {
			g.closeWindow(g.u.windows[len(g.u.windows)-1])
		} else {
			g.openWindow("settings")
		}
		g.u.hover = ""
		g.buildInterface()
		return true, true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF7) && g.self() != nil {
		g.toggleWindow("developer")
	}
	panel, overPanel := ui.HitPanel(g.u.frame.Panels, cursor)
	c, overControl := hoverControl(g.u.frame, cursor)
	if g.cfg.SmokeFrames > 0 && g.cfg.SmokeHover != "" {
		g.u.hover = g.cfg.SmokeHover
	} else {
		g.updateHover(c, overControl)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mouseConsumed = g.clickInterface(cursor)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) && overPanel {
		if slot, ok := hitControl(g.u.frame, cursor); ok && strings.HasPrefix(slot.Action, "inventory-") {
			g.inventoryClick(slot, true)
		}
		mouseConsumed = true
	}
	_, wheel := ebiten.Wheel()
	if overPanel {
		if v, ok := g.u.frame.Viewports[panel.ID]; ok && v.MaxScroll > 0 && cursor.Y < v.Bounds.Y+v.Bounds.H-v.Footer {
			g.u.contentScroll[panel.ID] = max(0, min(v.MaxScroll, g.u.contentScroll[panel.ID]-wheel*36))
		} else {
			g.u.scroll[panel.ID] = max(0, g.u.scroll[panel.ID]-int(wheel*3))
		}
	}
	if g.u.expanded && g.u.focus != "" {
		if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
			g.u.scroll[g.u.focus] += 10
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
			g.u.scroll[g.u.focus] = max(0, g.u.scroll[g.u.focus]-10)
		}
	}
	g.buildInterface()
	_, overPanel = ui.HitPanel(g.u.frame.Panels, cursor)
	return mouseConsumed || overPanel, g.u.editor != nil || g.frontCapturesKeys()
}
