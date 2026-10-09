package session

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"reflect"
	"testing"
)

func standAt(t *testing.T, tm *team, m *member, from, to string) {
	t.Helper()
	m.player.Node = from
	m.player.Scene = ""
	m.player.Active = true
	m.player.Online = true
	side, ok := geometry.DoorSide(tm.nodes[from], tm.nodes[to])
	if !ok {
		t.Fatal(from, to)
	}
	m.player.Position = tm.spaceArena(from).DoorPoint(side, 24)
}

func TestRemoteAndNonconnectedEntrancesCannotMoveOrCharge(t *testing.T) {
	tm := fixture(t)
	m := tm.players["a"]
	before := model.ClonePlayer(m.player)
	for _, id := range []string{"a", "passage-a-start", "shop", "missing"} {
		if err := tm.request(m, protocol.Request{Kind: "enter", Node: id}); err == nil {
			t.Fatal("remote entry", id)
		}
		if !reflect.DeepEqual(m.player, before) || len(tm.actors) != 0 {
			t.Fatal("failed entry changed ownership")
		}
	}
}

func TestFreeWalkingAndPaidConfirmationHandoff(t *testing.T) {
	tm := fixture(t)
	m := tm.players["a"]
	m.player.Online = true
	side, _ := geometry.DoorSide(tm.nodes["start"], tm.nodes["passage-a-start"])
	m.player.Position = tm.spaceArena("start").DoorPoint(side, 2)
	m.walk = model.Input{Move: m.player.Position.Sub(tm.spaceArena("start").Center).Unit()}
	before := m.player.AP
	epoch := m.player.PositionEpoch
	from := tm.nodes["start"]
	initialWorld := tm.spaceArena("start").WorldPoint(from, m.player.Position, tm.spaceArena("start").Radius)
	move := m.walk.Move
	tm.stepSpaces()
	if m.player.Node != "passage-a-start" || m.player.AP != before || m.player.PositionEpoch != epoch || m.walk.Move != move {
		t.Fatal(m.player)
	}
	world := tm.spaceArena(m.player.Node).WorldPoint(tm.nodes[m.player.Node], m.player.Position, tm.spaceArena("start").Radius)
	if world.Sub(initialWorld.Add(move.Scale(m.player.Speed/30))).Length() > 1e-7 {
		t.Fatal("free entrance snapped or lost movement remainder")
	}
	standAt(t, tm, m, "passage-a-start", "a")
	m.walk = model.Input{Move: model.Vec{X: 1, Y: .5}}
	tm.stepSpaces()
	if m.player.Node != "passage-a-start" || m.player.AP != before {
		t.Fatal("paid door auto charged")
	}
	if err := tm.request(m, protocol.Request{Kind: "enter", Node: "a"}); err != nil {
		t.Fatal(err)
	}
	if m.player.Scene != "a" || m.player.AP != before-1 || !tm.spaceArena("a").ContainsDisk(m.player.Position, 12) {
		t.Fatal(m.player)
	}
	position := m.player.Position
	tm.stepSpaces()
	if m.player.Position != position {
		t.Fatal("coordinator wrote combat-owned player")
	}
	if err := tm.request(m, protocol.Request{Kind: "enter", Node: "a"}); err == nil || m.player.AP != before-1 {
		t.Fatal("duplicate entry charged")
	}
}

func TestSharedVoteRequiresAllMembersAtAnyEntrance(t *testing.T) {
	tm := fixture(t)
	a, b := tm.players["a"], tm.players["b"]
	standAt(t, tm, a, "passage-b-shop", "shop")
	b.player.Online = true
	before := tm.shared
	if err := tm.request(a, protocol.Request{Kind: "propose", Node: "shop"}); err == nil || tm.vote != nil || tm.shared != before {
		t.Fatal("remote team opened room")
	}
	standAt(t, tm, b, "passage-c-shop", "shop")
	if err := tm.request(a, protocol.Request{Kind: "propose", Node: "shop"}); err != nil {
		t.Fatal(err)
	}
	b.player.Position = tm.spaceArena(b.player.Node).Center
	if err := tm.request(b, protocol.Request{Kind: "vote", Accept: true}); err == nil || tm.shared != before {
		t.Fatal("left entrance during vote")
	}
	standAt(t, tm, b, "passage-c-shop", "shop")
	if err := tm.request(b, protocol.Request{Kind: "vote", Accept: true}); err != nil {
		t.Fatal(err)
	}
	if tm.shared != before-1 || !tm.nodes["shop"].Open || a.player.Node == "shop" || b.player.Node == "shop" {
		t.Fatal("vote conflated participation")
	}
	if err := tm.request(a, protocol.Request{Kind: "enter", Node: "shop"}); err != nil {
		t.Fatal(err)
	}
	if b.player.Node == "shop" || tm.shared != before-1 {
		t.Fatal("extra charge or forced entry")
	}
}

func TestEntranceScriptFailureRollsBackSpaceAndFees(t *testing.T) {
	tm := fixture(t)
	c := testcontent.Load(t, nil, `sdk.on("base:bad-entry","session","node_entered",function(ctx){throw new Error("entry failed");});`)
	installContent(t, tm, c)
	m := tm.players["a"]
	standAt(t, tm, m, "passage-a-start", "a")
	before := model.ClonePlayer(m.player)
	if err := tm.request(m, protocol.Request{Kind: "enter", Node: "a"}); err == nil || !reflect.DeepEqual(m.player, before) || len(tm.actors) != 0 || m.visited["a"] {
		t.Fatal("partial admission")
	}
}

func TestPassageHasNoRewardOrRepeatedCharge(t *testing.T) {
	tm := fixture(t)
	m := tm.players["a"]
	before := m.player
	if err := tm.enter(m, "passage-a-start"); err != nil {
		t.Fatal(err)
	}
	if m.player.Coins != before.Coins || m.player.XP != before.XP || m.player.AP != before.AP {
		t.Fatal("passage reward")
	}
	if err := content.ValidatePlayerStats(tm.catalog, m.player, 1); err != nil {
		t.Fatal(err)
	}
}
