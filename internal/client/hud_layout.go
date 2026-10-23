package client

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
)

// Only these combat elements belong to this editor. Ordinary window positions
// remain independent, even when the combat HUD is reset.
var combatHUDElements = []struct{ ID, Key, Anchor string }{
	{"minimap", "ui.layout.minimap", "top_right"},
	{"equipment", "ui.backpack_status", "bottom_right"},
	{"health", "ui.layout.health", "bottom_center"},
	{"energy", "ui.layout.energy", "bottom_center"},
	{"dodges", "ui.layout.dodges", "bottom_center"},
	{"statuses", "ui.statuses", "bottom_center"},
	{"consumables", "ui.consumables", "top_left"},
	{"weapon-skill", "ui.weapon_skill", "bottom_right"},
	{"party", "ui.layout.party", "top_right"},
	{"journey", "ui.journey", "top_center"},
	{"view", "ui.view", "top_right"},
	{"tools", "ui.settings", "top_right"},
	{"entrance-prompt", "ui.layout.interaction", "bottom_center"},
	{"performance-hud", "ui.layout.performance", "bottom_left"},
}

type hudEditorState struct {
	Selected    string
	Windows     []string
	Focus       string
	ShowMap     bool
	Initialized bool
	Toolbar     ui.Rect
	Axis, Text  string
	ReplaceText bool
	Preview     model.Player
}

func combatHUDKey(id string) string {
	for _, e := range combatHUDElements {
		if e.ID == id {
			return e.Key
		}
	}
	return ""
}

func (g *Game) effectiveLayout(saved ui.Layout) ui.Layout {
	if len(g.resources.theme.Layout) == 0 {
		return saved
	}
	layout := ui.Clone(g.resources.theme.Layout)
	for id, p := range saved {
		layout[id] = p
	}
	return layout
}

func (g *Game) beginHUDLayoutEditor() {
	if g.u.editor != nil {
		return
	}
	e := &hudEditorState{Selected: "minimap", Windows: slices.Clone(g.u.windows), Focus: g.u.focus, ShowMap: g.showMap}
	// This detached player is presentation data only; it does not start a run.
	var err error
	e.Preview, err = content.Player(g.catalog, "hud-preview", g.tr("ui.layout.preview_room"), g.catalog.ChassisList()[g.chassis].ID)
	if err != nil {
		e.Preview = model.Player{MaxHP: 100, MaxEnergy: 100, Life: "alive", Dodges: 2, Level: 1}
	}
	e.Preview.HP = e.Preview.MaxHP * .7
	e.Preview.Energy = e.Preview.MaxEnergy * .65
	e.Preview.AP, e.Preview.Coins, e.Preview.Level, e.Preview.XP = 4, 120, 3, 20
	g.u.hudEditor, g.u.editor = e, ui.Clone(g.u.layout)
	g.u.windows, g.u.drag = nil, nil
	g.u.hover, g.u.capture = "", ""
	g.u.itemMenu, g.u.movementMenu = nil, false
	g.cameraControls.resetRotationInput()
	g.cameraControls.MapDrag = nil
	g.showMap = false
	g.destination, g.waypoints = nil, nil
	g.localInput = model.Input{}
	if g.link != nil {
		g.link.ClearInput()
	}
}

func (g *Game) finishHUDLayoutEditor() {
	e := g.u.hudEditor
	if e == nil {
		return
	}
	g.u.windows, g.u.focus, g.showMap = e.Windows, e.Focus, e.ShowMap
	g.u.editor, g.u.hudEditor, g.u.drag = nil, nil, nil
	g.u.hover = ""
	if g.u.settingsDraft != nil {
		g.u.settingsDraft.Layout = ui.Clone(g.u.layout)
		g.u.settingsPage = "home"
	}
}

func hudPanel(f uiFrame, id string) (ui.Panel, bool) {
	for _, p := range f.Panels {
		if p.ID == id {
			return p, true
		}
	}
	return ui.Panel{}, false
}

func hudHandle(p ui.Panel) ui.Rect {
	return ui.Rect{X: p.Bounds.X, Y: max(8, p.Bounds.Y-18), W: p.Bounds.W, H: 18}
}

func (g *Game) hudAnchor(id string) string {
	if p, ok := g.u.editor[id]; ok {
		return p.Anchor
	}
	if p, ok := g.resources.theme.Layout[id]; ok {
		return p.Anchor
	}
	for _, e := range combatHUDElements {
		if e.ID == id {
			return e.Anchor
		}
	}
	return "top_left"
}

func (g *Game) prepareHUDEditorFrame(f *uiFrame) {
	e := g.u.hudEditor
	if e.Initialized {
		return
	}
	// Materialize each visible position once, including old combined resource
	// placements. Moving the minimap never moves its former companion buttons.
	for _, element := range combatHUDElements {
		if p, ok := hudPanel(*f, element.ID); ok {
			d := ui.Drag{ID: p.ID, Bounds: p.Bounds, Anchor: g.hudAnchor(p.ID)}
			g.u.editor = ui.DragTo(g.u.editor, d, ui.Point{}, g.uiBounds())
		}
	}
	delete(g.u.editor, "resources")
	e.Initialized = true
}

func resetCombatHUD(layout ui.Layout) ui.Layout {
	next := ui.Clone(layout)
	for _, e := range combatHUDElements {
		delete(next, e.ID)
	}
	delete(next, "resources")
	return next
}

func (g *Game) setHUDPosition(x, y float64) {
	e := g.u.hudEditor
	if p, ok := hudPanel(g.u.frame, e.Selected); ok {
		d := ui.Drag{ID: p.ID, Bounds: p.Bounds, Anchor: g.hudAnchor(p.ID)}
		g.u.editor = ui.DragTo(g.u.editor, d, ui.Point{X: math.Round(x) - p.Bounds.X, Y: math.Round(y) - p.Bounds.Y}, g.uiBounds())
	}
}

func (g *Game) commitHUDCoordinate() {
	e := g.u.hudEditor
	if e.Axis == "" {
		return
	}
	if n, err := strconv.Atoi(e.Text); err == nil {
		if p, ok := hudPanel(g.u.frame, e.Selected); ok {
			x, y := p.Bounds.X, p.Bounds.Y
			if e.Axis == "x" {
				x = float64(n)
			} else {
				y = float64(n)
			}
			g.setHUDPosition(x, y)
		}
	}
	e.Axis, e.Text = "", ""
	e.ReplaceText = false
}

func (g *Game) buildEditor(f *uiFrame) {
	e := g.u.hudEditor
	placement := ui.Placement{Anchor: "center"}
	if e.Toolbar.W > 0 {
		placement = ui.Placement{Anchor: "top_left", X: e.Toolbar.X, Y: e.Toolbar.Y}
	}
	r := ui.Resolve(placement, ui.Size{W: 344, H: 448}, g.uiBounds())
	e.Toolbar = r
	f.Panels = append(f.Panels, ui.Panel{ID: "editor", Bounds: r})
	f.Titles["editor"] = g.tr("ui.layout.title")
	g.addLabel(f, "editor", ui.Rect{X: r.X + 12, Y: r.Y + 8, W: r.W - 24, H: 22}, g.tr("ui.layout.title"))
	g.addLabel(f, "editor", ui.Rect{X: r.X + 12, Y: r.Y + 34, W: r.W - 24, H: 40}, g.tr("ui.layout.help"))
	button := func(id, action, value, text string, rect ui.Rect) {
		f.Controls = append(f.Controls, uiControl{Owner: "editor", ID: id, Action: action, Value: value, Caption: text, Bounds: rect})
	}
	for i, element := range combatHUDElements {
		button("layout:select:"+element.ID, "layout-select", element.ID, g.tr(element.Key), ui.Rect{X: r.X + 12 + float64(i%2)*162, Y: r.Y + 80 + float64(i/2)*28, W: 158, H: 25})
	}
	g.addLabel(f, "editor", ui.Rect{X: r.X + 12, Y: r.Y + 280, W: r.W - 24, H: 22}, g.tr("ui.layout.selected", "name", g.tr(combatHUDKey(e.Selected))))
	if p, ok := hudPanel(*f, e.Selected); ok {
		for i, axis := range []string{"x", "y"} {
			value := p.Bounds.X
			if axis == "y" {
				value = p.Bounds.Y
			}
			text := fmt.Sprintf("%s: %.0f", []string{"X", "Y"}[i], value)
			if e.Axis == axis {
				text = []string{"X: ", "Y: "}[i] + e.Text + "|"
			}
			button("layout:coordinate:"+axis, "layout-coordinate", axis, text, ui.Rect{X: r.X + 12 + float64(i)*162, Y: r.Y + 306, W: 158, H: 30})
		}
	}
	g.addLabel(f, "editor", ui.Rect{X: r.X + 12, Y: r.Y + 342, W: r.W - 24, H: 38}, g.tr("ui.layout.nudge"))
	button("layout:reset-one", "layout-reset-one", "", g.tr("ui.layout.reset_one"), ui.Rect{X: r.X + 12, Y: r.Y + 382, W: 158, H: 26})
	button("layout:reset", "layout-reset", "", g.tr("ui.layout.reset_all"), ui.Rect{X: r.X + 174, Y: r.Y + 382, W: 158, H: 26})
	button("layout:save", "layout-save", "", g.tr("ui.save"), ui.Rect{X: r.X + 12, Y: r.Y + 414, W: 158, H: 26})
	button("layout:cancel", "layout-cancel", "", g.tr("ui.cancel"), ui.Rect{X: r.X + 174, Y: r.Y + 414, W: 158, H: 26})
}
