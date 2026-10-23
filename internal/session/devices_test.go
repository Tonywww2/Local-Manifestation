package session

import (
	"hsmultiplayer/pkg/protocol"
	"path/filepath"
	"reflect"
	"testing"
)

func TestShopDeviceRejectsRemotePurchaseAtomically(t *testing.T) {
	tm := fixture(t)
	m := tm.players["a"]
	n := tm.nodes["shop"]
	n.Open = true
	tm.nodes["shop"] = n
	m.player.Node = "shop"
	m.player.Position = tm.spaceArena("shop").DoorPoint(0, 24)
	m.player.Coins = 100
	before := m.player
	if err := tm.request(m, protocol.Request{Kind: "upgrade"}); err == nil || !reflect.DeepEqual(before, m.player) {
		t.Fatal("remote purchase accepted or mutated player")
	}
	m.player.Position = tm.spaceArena("shop").Center
	if err := tm.request(m, protocol.Request{Kind: "upgrade"}); err != nil || m.player.Coins != 90 || m.player.Upgrade != 1 {
		t.Fatal(m.player, err)
	}
}

func TestMapVotePersistsWithinRoomAndRepeatedInteractionWithdraws(t *testing.T) {
	tm := mapTravelFixture(t)
	a, b := tm.players["a"], tm.players["b"]
	standAt(t, tm, a, "passage-b-shop", "shop")
	standAt(t, tm, b, "passage-c-shop", "shop")
	before := tm.shared
	if err := tm.request(a, protocol.Request{Kind: "device", Node: "shop"}); err != nil {
		t.Fatal(err)
	}
	if tm.vote == nil || len(tm.vote.Yes) != 1 {
		t.Fatal("missing vote")
	}
	a.player.Position = tm.spaceArena(a.player.Node).Center
	tm.stepSpaces()
	if tm.vote == nil || len(tm.vote.Yes) != 1 || tm.shared != before {
		t.Fatal("map vote depends on physical door position")
	}
	if err := tm.request(a, protocol.Request{Kind: "device", Node: "shop"}); err != nil {
		t.Fatal(err)
	}
	if tm.vote != nil || tm.shared != before {
		t.Fatal("repeat interaction did not withdraw")
	}
}
func TestNextLayerUsesExitDeviceAndCheckpointFailureIsAtomic(t *testing.T) {
	tm := fixture(t)
	tm.phase = "intermission"
	tm.bossRoute = "normal"
	for _, m := range tm.players {
		m.player.Node = "boss"
		m.player.Scene = ""
		m.player.Online = true
		m.player.Position = tm.spaceArena("boss").ExitDevice()
	}
	a, b := tm.players["a"], tm.players["b"]
	tm.config.Checkpoint = t.TempDir() // Existing directory cannot be replaced with a checkpoint file.
	if err := tm.request(a, protocol.Request{Kind: "device", Node: "next"}); err != nil {
		t.Fatal(err)
	}
	before := tm.shared
	if err := tm.request(b, protocol.Request{Kind: "device", Node: "next"}); err == nil {
		t.Fatal("write unexpectedly succeeded")
	}
	if tm.floor != 1 || tm.phase != "intermission" || tm.shared != before || a.player.Node != "boss" {
		t.Fatal("failed transition partially committed")
	}
	tm.config.Checkpoint = filepath.Join(t.TempDir(), "checkpoint.json")
	if err := tm.request(a, protocol.Request{Kind: "device", Node: "next"}); err != nil {
		t.Fatal(err)
	}
	if err := tm.request(b, protocol.Request{Kind: "device", Node: "next"}); err != nil {
		t.Fatal(err)
	}
	if tm.floor != 2 || tm.phase != "exploring" || tm.players["a"].player.Node != "start" || tm.vote != nil {
		t.Fatal("exit did not advance layer")
	}
}
