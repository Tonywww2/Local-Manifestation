package client

import (
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/device"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"slices"
	"testing"
	"time"
)

func TestMapConfirmationAndRoomChangeClearLocalIntent(t *testing.T) {
	g := interfaceGame(t)
	g.confirmMapEntry("room", false)
	g.buildInterface()
	if !slices.Contains(g.u.windows, "entrance") || !slices.ContainsFunc(g.u.frame.Panels, func(p ui.Panel) bool { return p.ID == "entrance" && p.Floating }) {
		t.Fatal("map admission lacks a draggable confirmation")
	}
	target := model.Vec{X: 500, Y: 200}
	g.destination, g.mouseFrames = &target, 8
	next := protocol.CloneState(*g.state)
	next.Players[0].Node = "room"
	next.Players[0].PositionEpoch++
	g.receiveState(&next, time.Now())
	if g.destination != nil || g.mouseFrames != 0 || slices.Contains(g.u.windows, "entrance") {
		t.Fatal("old-room intent survived independent room travel")
	}
	if g.canMapMove(next.Nodes[0]) {
		t.Fatal("created an unauthored reverse route")
	}
}

func TestMapVoteAndIndependentRoomDevices(t *testing.T) {
	g := interfaceGame(t)
	next := protocol.CloneState(*g.state)
	next.Nodes[1].Payment = "shared"
	next.Vote = &protocol.Vote{Node: "room", Yes: []string{g.id}, Entrants: []string{g.id}, Required: 2}
	g.receiveState(&next, time.Now())
	g.buildInterface()
	if !slices.Contains(g.u.windows, "vote") {
		t.Fatal("map vote has no response window")
	}
	scene := g.sceneForNode(g.currentNode())
	if slices.ContainsFunc(g.worldDevices(g.currentNode(), scene), func(d device.Device) bool { return d.Kind == "entrance" }) {
		t.Fatal("independent room still has a physical travel entrance")
	}
}

func TestMapTravelClearsHistoryEvenWithoutEpochChange(t *testing.T) {
	g := interfaceGame(t)
	g.localInput = model.Input{Move: model.Vec{X: 1}}
	next := protocol.CloneState(*g.state)
	next.Players[0].Node = "room"
	next.Players[0].Position = model.Vec{X: 100, Y: 200}
	g.receiveState(&next, time.Now())
	if g.localInput != (model.Input{}) || len(g.worldHistory.frames) != 1 || g.prediction.Position != next.Players[0].Position {
		t.Fatal("map travel continued motion from another room")
	}
	if g.state.Players[0].Position != next.Players[0].Position {
		t.Fatal("display mutated authority")
	}
}

func TestOldCustomBindingsPreservedWithNewInteractionAction(t *testing.T) {
	old := defaultBindings()
	for _, mode := range []string{"keyboard", "mouse"} {
		delete(old[mode], "interact")
		old[mode]["map"] = "G"
	}
	next := migrateBindings(old)
	if err := validateBindings(next); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"keyboard", "mouse"} {
		if next[mode]["map"] != "G" || next[mode]["interact"] != "F" || old[mode]["interact"] != "" {
			t.Fatal("migration changed existing binding")
		}
	}
}
