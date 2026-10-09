package client

import (
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"slices"
	"testing"
	"time"
)

func TestEntranceConfirmationAndRoomChangeClearLocalIntent(t *testing.T) {
	g := interfaceGame(t)
	g.state.Players[0].Position = geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight).DoorPoint(0, 24)
	if door, ok := g.nearbyDoor(); !ok || door.ID != "room" {
		t.Fatal("missing nearby opening")
	}
	g.performControl(uiControl{Action: "entrance", Value: "room"})
	g.buildInterface()
	if !slices.Contains(g.u.windows, "entrance") || !slices.ContainsFunc(g.u.frame.Panels, func(p ui.Panel) bool { return p.ID == "entrance" && p.Floating }) {
		t.Fatal("confirmation not draggable")
	}
	target := model.Vec{X: 500, Y: 200}
	g.destination = &target
	g.mouseFrames = 8
	next := protocol.CloneState(*g.state)
	next.Players[0].Node = "room"
	next.Players[0].PositionEpoch++
	g.receiveState(&next, time.Now())
	if g.destination != nil || g.mouseFrames != 0 || slices.Contains(g.u.windows, "entrance") {
		t.Fatal("old-room intent survived transition")
	}
	if _, ok := g.nearbyDoor(); ok {
		t.Fatal("generated reverse entrance without explicit edge")
	}
}

func TestSharedDeviceUsesWorldInteractionAndNoVotePanel(t *testing.T) {
	g := interfaceGame(t)
	g.state.Players[0].Position = geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight).DoorPoint(0, 24)
	g.state.Nodes[1].Payment = "shared"
	g.state.Nodes[1].Cost = 1
	g.inspectInteraction("room")
	g.state.Vote = &protocol.Vote{Node: "room", Yes: []string{g.id}, Required: 2}
	g.buildInterface()
	if slices.Contains(g.u.windows, "entrance") || slices.ContainsFunc(g.u.frame.Panels, func(p ui.Panel) bool { return p.ID == "vote" }) {
		t.Fatal("shared device opened a voting window")
	}
}

func TestFreeHandoffKeepsPredictedWorldPointAndHeldInput(t *testing.T) {
	g := interfaceGame(t)
	a := geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight)
	g.state.Players[0].Position = a.DoorPoint(0, -3)
	g.state.Players[0].PositionEpoch = 10
	g.state.Nodes[1].Payment = "none"
	g.state.Nodes[1].Edges = []string{"start"}
	at := time.Now()
	g.prediction = MovementPrediction{}.Reconcile(g.state.Players[0], a, at, 0)
	g.localInput = model.Input{Move: model.Vec{X: 1}}
	g.mouseFrames = 20
	before := a.WorldPoint(g.state.Nodes[0], g.prediction.Position, a.Radius)
	next := protocol.CloneState(*g.state)
	next.Players[0].Node = "room"
	next.Players[0].Position = geometry.TransferPoint(next.Nodes[0], next.Nodes[1], a, a, next.Players[0].Position)
	g.receiveState(&next, at.Add(50*time.Millisecond))
	after := a.WorldPoint(next.Nodes[1], g.prediction.Position, a.Radius)
	if before.Sub(after).Length() > 1e-7 || g.localInput.Move.X != 1 || g.mouseFrames != 20 {
		t.Fatal("seamless handoff reset presentation or input")
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
		if next[mode]["map"] != "G" || next[mode]["interact"] != "T" || old[mode]["interact"] != "" {
			t.Fatal("migration changed existing binding")
		}
	}
}
