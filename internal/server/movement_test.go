package server

import (
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"testing"
	"time"
)

func TestLiveFreeDoorPreservesWorldMotionAndHeldInput(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := startTestServer(t, c)
	p := connectPeer(t, addr, c, "Walker", "")
	p.send(t, protocol.Request{Kind: "start"})
	s := waitFor(t, []*testPeer{p}, func(s *protocol.State) bool { return s.Phase == "exploring" }, false)
	var from, to model.Node
	for _, n := range s.Nodes {
		if n.ID == "start" {
			from = n
		}
		if n.ID == "passage-a-start" {
			to = n
		}
	}
	a := geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight)
	side, _ := geometry.DoorSide(from, to)
	move := a.DoorPoint(side, 0).Sub(a.Center).Unit()
	before := findPlayer(s, p.id)
	last := before
	lastTick := s.ServerTick
	end := time.Now().Add(8 * time.Second)
	for time.Now().Before(end) {
		p.send(t, protocol.Request{Kind: "input", Input: model.Input{Move: move}})
		current := p.state.Load()
		now := findPlayer(current, p.id)
		if now.Node == to.ID {
			if last.Node == from.ID {
				oldWorld := a.WorldPoint(from, last.Position, a.Radius)
				newWorld := a.WorldPoint(to, now.Position, a.Radius)
				distance := newWorld.Sub(oldWorld).Length()
				maxDistance := now.Speed*float64(current.ServerTick-lastTick)/float64(current.TickRate) + .001
				if distance > maxDistance || now.PositionEpoch != before.PositionEpoch || now.AP != before.AP {
					t.Fatal("server free crossing teleported or charged", distance, maxDistance)
				}
				land := newWorld
				// One movement intent remains live across the ownership/frame change.
				waitFor(t, []*testPeer{p}, func(next *protocol.State) bool {
					pl := findPlayer(next, p.id)
					return pl.Node == to.ID && a.WorldPoint(to, pl.Position, a.Radius).Sub(land).Length() > 15
				}, false)
				p.send(t, protocol.Request{Kind: "input"})
				return
			}
		}
		last, lastTick = now, current.ServerTick
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("free door crossing did not occur")
}

func TestLiveJSStatusExpiresDuringCountdown(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		d.Statuses = []model.StatusDef{{ID: "base:live-vigor", Name: "Live vigor", Description: "Test", Level: 1, MaxLayers: 2, DurationMillis: 1500, Modifiers: []model.StatModifier{{ID: "base:live-health", Stat: "max_hp", Operation: "add", Value: 60}}}}
		a.Rooms[0].InitialDelay = 10
	}, `sdk.on("base:live-status","session","node_entered",function(ctx){var p=ctx.players[0];return [{kind:"apply_status",target:p.id,status:"base:live-vigor",source:{kind:"weapon",actor:p.id,instance:"held-weapon",definition:p.weapon}}];});`)
	addr, _ := startTestServer(t, c)
	p := connectPeer(t, addr, c, "Status walker", "")
	peers := []*testPeer{p}
	p.send(t, protocol.Request{Kind: "start"})
	waitFor(t, peers, func(s *protocol.State) bool { return s.Phase == "exploring" }, false)
	travel(t, peers, map[string]string{p.id: "a"}, true)
	s := waitFor(t, peers, func(s *protocol.State) bool { return len(findPlayer(s, p.id).Statuses.Instances) == 1 }, false)
	before := findPlayer(s, p.id)
	if before.MaxHP != 180 {
		t.Fatal("live status not applied", before.MaxHP)
	}
	p.send(t, protocol.Request{Kind: "input", Input: model.Input{Move: geometry.NewArena(2000, 1250).Center.Sub(before.Position).Unit()}})
	s = waitFor(t, peers, func(s *protocol.State) bool { return len(findPlayer(s, p.id).Statuses.Instances) == 0 }, false)
	after := findPlayer(s, p.id)
	if after.MaxHP != 120 || after.HP != 120 || after.Position.Sub(before.Position).Length() < 10 || len(s.Scenes) != 1 || s.Scenes[0].State != "waiting" {
		t.Fatal("countdown blocked movement or expiry")
	}
}
