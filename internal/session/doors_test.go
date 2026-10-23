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

// These travel checks use the authored topology so node identities are stable.
func authoredTravelMap(tm *team) {
	tm.mapTemplate = tm.catalog.Map(tm.catalog.Floor(tm.floor).Map)
	tm.nodes = tm.makeMap()
	tm.terrain = tm.mapTemplate.Terrain
	tm.spaces = nil
	for _, m := range tm.players {
		m.player.Node, m.player.Scene = tm.mapTemplate.Start, ""
		m.player.Online = true
		m.visited = map[string]bool{}
		tm.placeInSpace(m, m.player.Node)
	}
}

func mapTravelFixture(t *testing.T) *team {
	t.Helper()
	tm := fixture(t)
	authoredTravelMap(tm)
	return tm
}

func TestNonconnectedMapMovementCannotCharge(t *testing.T) {
	tm := mapTravelFixture(t)
	m := tm.players["a"]
	before := model.ClonePlayer(m.player)
	for _, id := range []string{"a", "shop", "missing", "start"} {
		if err := tm.request(m, protocol.Request{Kind: "enter", Node: id}); err == nil {
			t.Fatal("nonadjacent entry", id)
		}
		if !reflect.DeepEqual(m.player, before) || len(tm.actors) != 0 {
			t.Fatal("failed entry changed ownership")
		}
	}
}

func TestMapMovementPaysOnceAndWalkingStaysInRoom(t *testing.T) {
	tm := mapTravelFixture(t)
	m := tm.players["a"]
	side, _ := geometry.DoorSide(tm.nodes["start"], tm.nodes["passage-a-start"])
	m.player.Position = tm.spaceArena("start").DoorPoint(side, 2)
	m.walk = model.Input{Move: m.player.Position.Sub(tm.spaceArena("start").Center).Unit()}
	before, epoch := m.player.AP, m.player.PositionEpoch
	tm.stepSpaces()
	if m.player.Node != "start" || m.player.AP != before || m.player.PositionEpoch != epoch {
		t.Fatal("walking crossed an independent room boundary")
	}
	if err := tm.request(m, protocol.Request{Kind: "enter", Node: "passage-a-start"}); err != nil {
		t.Fatal(err)
	}
	if m.player.AP != before-1 || m.player.Node != "passage-a-start" || m.player.PositionEpoch <= epoch || m.walk != (model.Input{}) {
		t.Fatal("map movement did not commit its fee and new position")
	}
	if err := tm.request(m, protocol.Request{Kind: "enter", Node: "a"}); err != nil {
		t.Fatal(err)
	}
	if m.player.Scene != "a" || m.player.AP != before-2 || !tm.spaceArena("a").ContainsDisk(m.player.Position, 12) {
		t.Fatal(m.player)
	}
	position := m.player.Position
	tm.stepSpaces()
	if m.player.Position != position {
		t.Fatal("coordinator wrote combat-owned player")
	}
	if err := tm.request(m, protocol.Request{Kind: "enter", Node: "a"}); err == nil || m.player.AP != before-2 {
		t.Fatal("duplicate entry charged")
	}
}

func TestSharedTravelSeparatesApprovalAndParticipation(t *testing.T) {
	tm := mapTravelFixture(t)
	a, b := tm.players["a"], tm.players["b"]
	a.player.Node = "passage-b-shop"
	before, personal := tm.shared, a.player.AP
	if err := tm.request(a, protocol.Request{Kind: "propose", Node: "shop"}); err != nil {
		t.Fatal(err)
	}
	// A distant teammate may approve without travelling or standing at a door.
	if err := tm.request(b, protocol.Request{Kind: "vote", Accept: true}); err != nil {
		t.Fatal(err)
	}
	if tm.shared != before-1 || a.player.Node != "shop" || b.player.Node != "start" || a.player.AP != personal {
		t.Fatal("approval forced participation or charged personal AP")
	}
	if err := tm.enter(a, "passage-b-shop"); err != nil {
		t.Fatal(err)
	}
	before = tm.shared
	if err := tm.enter(a, "shop"); err == nil || tm.shared != before {
		t.Fatal("opened shared node became free personal travel")
	}
	b.player.Node = "passage-c-shop"
	personalA, personalB := a.player.AP, b.player.AP
	if err := tm.request(a, protocol.Request{Kind: "propose", Node: "shop"}); err != nil {
		t.Fatal(err)
	}
	if err := tm.request(b, protocol.Request{Kind: "vote", Accept: true, Join: true}); err != nil {
		t.Fatal(err)
	}
	if tm.shared != before-1 || a.player.Node != "shop" || b.player.Node != "shop" || a.player.AP != personalA || b.player.AP != personalB {
		t.Fatal("repeat group movement did not charge exactly one shared AP")
	}
}

func TestEntranceScriptFailureRollsBackSpaceAndFees(t *testing.T) {
	tm := mapTravelFixture(t)
	c := testcontent.Load(t, nil, `sdk.on("base:bad-entry","session","node_entered",function(ctx){throw new Error("entry failed");});`)
	installContent(t, tm, c)
	authoredTravelMap(tm)
	m := tm.players["a"]
	m.player.Node = "passage-a-start"
	before := model.ClonePlayer(m.player)
	if err := tm.request(m, protocol.Request{Kind: "enter", Node: "a"}); err == nil || !reflect.DeepEqual(m.player, before) || len(tm.actors) != 0 || m.visited["a"] {
		t.Fatal("partial admission")
	}
}

func TestPassageReturnChargesEveryMovementWithoutRewards(t *testing.T) {
	tm := mapTravelFixture(t)
	m := tm.players["a"]
	before := model.ClonePlayer(m.player)
	for i, id := range []string{"passage-a-start", "start", "passage-a-start"} {
		if err := tm.enter(m, id); err != nil {
			t.Fatal(err)
		}
		if m.player.Coins != before.Coins || m.player.XP != before.XP || m.player.AP != before.AP-i-1 {
			t.Fatal("passage gave rewards or a repeated movement discount")
		}
	}
	if err := content.ValidatePlayerStats(tm.catalog, m.player, 1); err != nil {
		t.Fatal(err)
	}
}
