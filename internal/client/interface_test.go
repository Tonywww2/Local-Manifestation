package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	base "hsmultiplayer/content"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func interfaceGame(t *testing.T) *Game {
	t.Helper()
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	g := New(c, Config{Profile: filepath.Join(t.TempDir(), "player.json")})
	ch := c.ChassisList()[0]
	p := model.Player{ID: "me", Name: "Me", Chassis: ch.ID, Weapon: ch.Weapon, Node: "start", Life: "alive", HP: 50, MaxHP: 100, BaseHP: 100, Downs: 2, AP: 3}
	p.BaseStats = model.StatValues{Attributes: ch.Attributes, MaxHP: ch.HP, Armor: ch.Armor, Speed: ch.Speed}
	g.id = p.ID
	g.state = &protocol.State{Phase: "exploring", Floor: 1, Floors: 3, SharedAP: 3, Players: []model.Player{p}, Nodes: []model.Node{{ID: "start", Name: "Start", Edges: []string{"room"}}, {ID: "room", Name: "Room", Q: 1}}}
	return g
}
func labelsFor(g *Game, id string) string {
	var lines []string
	if id == "hover" && g.u.frame.Tooltip != nil {
		return strings.Join(g.u.frame.Tooltip.Rows, "\n")
	}
	for _, l := range g.u.frame.Labels {
		if l.Owner == id {
			lines = append(lines, l.Text)
		}
	}
	return strings.Join(lines, "\n")
}
func TestStatusLeftClickAndFocusedExpansion(t *testing.T) {
	g := interfaceGame(t)
	g.u.hover = "penalty"
	g.u.focus = "hover"
	g.buildInterface()
	var target uiControl
	for _, c := range g.u.frame.Controls {
		if c.ID == "penalty" {
			target = c
		}
	}
	before := slices.Clone(g.u.windows)
	if target.Key != "" || target.Badge != "∞" {
		t.Fatal("status lifetime was mistaken for a shortcut")
	}
	g.performControl(target)
	if !reflect.DeepEqual(before, g.u.windows) {
		t.Fatal("status click opened a page")
	}
	if strings.Contains(labelsFor(g, "hover"), "max(1") {
		t.Fatal("compact hover leaked calculations")
	}
	g.u.expanded = true
	g.buildInterface()
	if !strings.Contains(labelsFor(g, "hover"), "max(1") {
		t.Fatal("focused hover did not expand")
	}
	if slices.Contains(g.u.windows, "weapon") {
		t.Fatal("Shift opened unrelated weapon page")
	}
	g.u.hover = ""
	g.openWindow("backpack")
	g.buildInterface()
	if strings.Contains(labelsFor(g, "backpack"), "ui.damage") || strings.Contains(labelsFor(g, "backpack"), "max(1") {
		t.Fatal("unrelated detail leaked")
	}
}
func TestEveryFloatingWindowAndMapSelection(t *testing.T) {
	g := interfaceGame(t)
	for _, id := range []string{"settings", "map", "weapon", "armor", "backpack", "self", "peer:me", "developer"} {
		g.openWindow(id)
	}
	g.u.hover = "penalty"
	g.u.focus = "hover"
	g.buildInterface()
	for _, p := range g.u.frame.Panels {
		if p.Floating {
			if p.Bounds.Header().H != 30 {
				t.Fatal(p)
			}
			found := false
			for _, c := range g.u.frame.Controls {
				if c.Owner == p.ID && c.Action == "close" {
					found = true
				}
			}
			if !found {
				t.Fatal("no close control", p.ID)
			}
		}
	}
	before := protocol.CloneState(*g.state)
	for _, c := range g.u.frame.Controls {
		if c.ID == "map-node:room" {
			if c.Action != "select-node" {
				t.Fatal("map issues travel intent")
			}
			g.performControl(c)
		}
	}
	if g.u.selectedNode != "room" || !reflect.DeepEqual(before, *g.state) {
		t.Fatal("map changed authority state")
	}
}
func TestBindingsAreIndependentAndAtomic(t *testing.T) {
	g := interfaceGame(t)
	g.openWindow("settings")
	g.u.settingsPage = "bindings"
	g.u.capture = "skill1"
	g.captureBinding("E")
	if g.u.settingsDraft.Bindings["keyboard"]["skill1"] != "Q" || g.u.bindingError == "" {
		t.Fatal("conflict replaced a binding")
	}
	g.captureBinding("J")
	if g.bound("skill1") != "Q" {
		t.Fatal("draft applied before save")
	}
	g.performControl(uiControl{Action: "settings-save"})
	if g.bound("skill1") != "J" {
		t.Fatal("save did not apply")
	}
	restored := New(g.catalog, Config{Profile: g.cfg.Profile})
	if restored.bound("skill1") != "J" || restored.u.bindings["mouse"]["skill1"] != "Q" {
		t.Fatal("binding restore/mode isolation failed")
	}
	g.u.settingsDraft.Bindings["keyboard"]["skill1"] = "K"
	g.performControl(uiControl{Action: "settings-cancel"})
	if g.u.settingsDraft.Bindings["keyboard"]["skill1"] != "J" {
		t.Fatal("cancel kept draft")
	}
}
func TestSettingsPreserveFieldsAndFailureLeavesLivePreferences(t *testing.T) {
	g := interfaceGame(t)
	path := settingsPath(g.cfg.Profile)
	if err := os.WriteFile(path, []byte(`{"language":"zh-CN","custom":{"kept":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := g.preferences()
	s.Layout = ui.Layout{"backpack": {Anchor: "bottom_right", X: -50, Y: -70}}
	s.Language = "en"
	if err := g.commitPreferences(s); err != nil {
		t.Fatal(err)
	}
	var loaded map[string]json.RawMessage
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &loaded); err != nil || loaded["custom"] == nil {
		t.Fatal("lost existing settings", err)
	}
	bad := g.preferences()
	bad.Language = "zh-CN"
	bad.Layout["backpack"] = ui.Placement{Anchor: "bad"}
	if err := g.commitPreferences(bad); err == nil {
		t.Fatal("accepted invalid preference")
	}
	if g.language != "en" || g.u.layout["backpack"].Anchor != "bottom_right" {
		t.Fatal("partially committed")
	}
	g.cfg.Profile = filepath.Join(path, "impossible.json")
	if err := g.commitPreferences(s); err == nil {
		t.Fatal("unexpected successful write through file")
	}
	if g.language != "en" {
		t.Fatal("write failure changed live settings")
	}
}
func TestSameRoomSortingUsesSpaceNotEmptyScene(t *testing.T) {
	g := interfaceGame(t)
	g.state.Players = append(g.state.Players, model.Player{ID: "other", Node: "room"}, model.Player{ID: "same", Node: "start"})
	if peers := g.orderedPeers(); peers[0].ID != "same" || g.sameRoom(peers[1]) {
		t.Fatal("empty scene treated as same room", peers)
	}
}
func TestHudFitsNativeWindowSizes(t *testing.T) {
	g := interfaceGame(t)
	for i := 0; i < 7; i++ {
		g.state.Players = append(g.state.Players, model.Player{ID: string(rune('a' + i)), Name: "Team", Node: "start", Life: "alive", MaxHP: 100})
	}
	for _, size := range []ui.Size{{W: 1024, H: 720}, {W: 1280, H: 800}, {W: 1920, H: 1080}} {
		g.width, g.height = int(size.W), int(size.H)
		g.buildInterface()
		for _, p := range g.u.frame.Panels {
			r := p.Bounds
			if r.X < 0 || r.Y < 0 || r.X+r.W > size.W || r.Y+r.H > size.H {
				t.Fatal(size, p)
			}
		}
	}
}

func TestHoverFollowsPointerWithoutCapturingWindowFocus(t *testing.T) {
	g := interfaceGame(t)
	g.openWindow("weapon")
	g.u.cursor = ui.Point{X: 900, Y: 50}
	g.updateHover(uiControl{ID: "penalty", Owner: "statuses"}, true)
	g.buildInterface()
	first := g.u.frame.Tooltip.Bounds
	g.u.cursor.X += 20
	g.buildInterface()
	if g.u.frame.Tooltip.Bounds.X != first.X+20 || g.u.focus != "weapon" || slices.ContainsFunc(g.u.frame.Panels, func(p ui.Panel) bool { return p.ID == "hover" }) {
		t.Fatal("tooltip captured input or stopped following the cursor")
	}
	if g.clickInterface(ui.Point{X: first.X + 20, Y: first.Y + 5}) || g.u.drag != nil {
		t.Fatal("tooltip accepted a click")
	}
	g.updateHover(uiControl{}, false)
	if g.u.hover != "" || g.u.focus != "weapon" {
		t.Fatal("leaving the target retained Hover or changed window focus")
	}
}

func TestActionPointsAndDodgeIndicatorsUseRequestedHudRegions(t *testing.T) {
	g := interfaceGame(t)
	g.state.Players[0].Dodges = 1
	g.buildInterface()
	if !strings.Contains(labelsFor(g, "minimap"), "3") || strings.Contains(labelsFor(g, "resources"), "共享") {
		t.Fatal("action points are outside the minimap")
	}
	if len(g.u.frame.Dots) != 2 || !g.u.frame.Dots[0].Available || g.u.frame.Dots[1].Available {
		t.Fatal("dodge charges do not reflect the authoritative snapshot")
	}
	for _, d := range g.u.frame.Dots {
		m := g.u.frame.Meters[0]
		if d.Owner != m.Owner || d.Center.X <= m.Bounds.X+m.Bounds.W || d.Center.Y < m.Bounds.Y || d.Center.Y > m.Bounds.Y+m.Bounds.H {
			t.Fatal("dodge dot is not to the right of the health bar")
		}
	}
}

func TestUiCaptureAlsoSuppressesReboundMouseActions(t *testing.T) {
	for _, code := range []string{"Mouse0", "Mouse1", "Mouse2"} {
		if bindingAvailable(code, true) || !bindingAvailable(code, false) {
			t.Fatal("UI mouse input leaked to a rebound action", code)
		}
	}
	if !bindingAvailable("Space", true) {
		t.Fatal("mouse capture swallowed independent keyboard input")
	}
}
