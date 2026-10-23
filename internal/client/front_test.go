package client

import (
	"errors"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/model"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestMainMenuHasOnlyThreeActionsAndOpensWalkableHall(t *testing.T) {
	g := interfaceGame(t)
	g.state = nil
	g.buildInterface()
	var actions []string
	for _, c := range g.u.frame.Controls {
		actions = append(actions, c.ID)
	}
	if !reflect.DeepEqual(actions, []string{"main:start", "main:settings", "main:exit"}) {
		t.Fatal(actions)
	}
	g.performControl(uiControl{Action: "front", Value: "start"})
	g.buildInterface()
	if g.front.Page != "lobby" || len(g.front.Devices) != 6 || g.state != nil || g.pending != nil {
		t.Fatal("start bypassed hall")
	}
	for _, d := range g.front.Devices {
		if !slices.ContainsFunc(g.u.frame.Controls, func(c uiControl) bool { return c.Value == d.ID }) {
			t.Fatal("missing hall device", d.ID)
		}
	}
	g.performFront("exit")
	if !g.exitRequested {
		t.Fatal("exit has no effect")
	}
}
func TestDeviceHoverShowsOnlyNameUntilShiftAndIsReadOnly(t *testing.T) {
	g := interfaceGame(t)
	g.state = nil
	g.performFront("start")
	id := "world-device:hall:join"
	g.u.hover = id
	g.u.focus = "hover"
	g.buildInterface()
	if labelsFor(g, "hover") != "" || g.u.frame.Tooltip == nil || g.u.frame.Tooltip.Title != g.tr("hall.join") {
		t.Fatal("compact device hover contains extra text")
	}
	g.u.expanded = true
	g.buildInterface()
	if !strings.Contains(labelsFor(g, "hover"), "输入服务器地址") || slices.ContainsFunc(g.u.frame.Panels, func(p ui.Panel) bool { return p.ID == "hover" }) || slices.ContainsFunc(g.u.frame.Controls, func(c uiControl) bool { return c.Owner == "hover" }) {
		t.Fatal("missing Shift description or tooltip accepts input")
	}
	for _, language := range []string{"zh-CN", "en"} {
		g.language = language
		for _, d := range g.front.Devices {
			if g.deviceName(d) == "" || g.deviceDescription(d) == "" || strings.HasPrefix(g.deviceName(d), "hall.") {
				t.Fatal("missing locale", d)
			}
		}
	}
}
func TestLearningDeviceMouseHasNoRangeAndDoesNotSpendOnOpen(t *testing.T) {
	g := interfaceGame(t)
	g.state = nil
	g.performFront("start")
	g.useWorldDevice("hall:progression")
	g.buildInterface()
	if !slices.Contains(g.u.windows, "progression") || !strings.Contains(labelsFor(g, "progression"), "加成点") || g.pending != nil || g.profiles.Scopes[g.localScope()].Points != 0 {
		t.Fatal("learning window changed progress without a transaction")
	}
}
func TestRoomDraftCancelSaveAndRestart(t *testing.T) {
	g := interfaceGame(t)
	g.state = nil
	g.performFront("start")
	before := g.front.Config
	g.openWindow("room-settings")
	g.adjustRoom("players:1")
	g.front.Name = "Changed"
	g.chooseChassis("1")
	g.closeWindow("room-settings")
	if g.front.Config != before || g.name == "Changed" || g.chassis != 0 {
		t.Fatal("draft leaked")
	}
	g.openWindow("room-settings")
	g.adjustRoom("players:1")
	g.adjustRoom("tps:1")
	g.front.Name = "Saved name"
	g.chooseChassis("1")
	g.saveRoomConfig()
	if g.front.Config.Players != before.Players+1 || g.front.Config.TickRate != 60 {
		t.Fatal(g.front.Config)
	}
	restored := New(g.catalog, Config{Profile: g.cfg.Profile})
	if restored.front.Config != g.front.Config || restored.name != "Saved name" || restored.chassis != 1 {
		t.Fatal("room configuration was not restored")
	}
}
func TestSoloAndHostUseSavedConfigWithoutMutatingSharedDefaults(t *testing.T) {
	for _, solo := range []bool{true, false} {
		g := interfaceGame(t)
		g.state = nil
		options := lobby.DefaultConfig()
		options.Players = 12
		options.VotePercent = 80
		options.TickRate = 60
		options.Port = 18778
		g.front.Config = options
		address := "127.0.0.1:18778"
		g.tokens[address] = "existing-identity"
		seen := make(chan lobby.Config, 1)
		g.cfg.Host = func(c lobby.Config, private bool) error {
			if private != solo {
				t.Error("wrong listen mode")
			}
			seen <- c
			return errors.New("intentional startup failure")
		}
		g.startHosted(solo)
		select {
		case c := <-seen:
			if c.VotePercent != 80 || c.TickRate != 60 || c.Port != 18778 || solo && c.Players != 1 || !solo && c.Players != 12 {
				t.Fatal(c)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("host did not receive configuration")
		}
		select {
		case r := <-g.pending:
			if r.err == nil {
				t.Fatal("unexpected successful host")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("missing failure")
		}
		if g.front.Config != options {
			t.Fatal("solo permanently changed room capacity")
		}
		if g.tokens[address] != "existing-identity" {
			t.Fatal("failed startup deleted reconnect identity")
		}
		g.Close()
	}
}
func TestWorldDevicePickingUsesSameProjectionAndHUDWins(t *testing.T) {
	g := interfaceGame(t)
	g.state = nil
	g.performFront("start")
	for _, angle := range []float64{0, defaultCameraAngle, 1.17, -2.4} {
		g.cameraControls.Rotation = angle
		g.buildInterface()
		point := g.hallCamera().Project(g.front.Devices[4].Position)
		c, ok := hitControl(g.u.frame, ui.Point{X: point.X, Y: point.Y})
		if !ok || c.Value != "hall:join" {
			t.Fatal(c)
		}
		g.u.frame.Panels = append(g.u.frame.Panels, ui.Panel{ID: "cover", Bounds: c.Bounds, Floating: true})
		if _, ok := hitControl(g.u.frame, ui.Point{X: point.X, Y: point.Y}); ok {
			t.Fatal("device clicked through a floating window")
		}
	}
	g.front.Position = model.Vec{X: 500, Y: 380}
}
