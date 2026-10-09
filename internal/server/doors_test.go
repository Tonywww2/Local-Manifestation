package server

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"testing"
	"time"
)

func TestLiveLockedDoorAndBetweenWaveJoin(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		for i := range d.Weapons {
			d.Weapons[i].Base = 10000
			d.Weapons[i].Cooldown = .02
		}
		for i := range d.Enemies {
			d.Enemies[i].HP = 1
			d.Enemies[i].Speed = 1
			d.Enemies[i].Damage = 1
		}
		a.Rooms[0].InitialDelay = 0
		a.Rooms[0].Waves[0].BreakSeconds = 3
	}, "")
	addr, _ := startTestServer(t, c)
	a := connectPeer(t, addr, c, "Fighter", "")
	b := connectPeer(t, addr, c, "Joining player", "")
	peers := []*testPeer{a, b}
	a.send(t, protocol.Request{Kind: "start"})
	waitFor(t, peers, func(s *protocol.State) bool { return s.Phase == "exploring" }, false)
	travel(t, peers, map[string]string{a.id: "a", b.id: "a"}, false)
	a.send(t, protocol.Request{Kind: "enter", Node: "a"})
	s := waitFor(t, peers, func(s *protocol.State) bool { return len(s.Scenes) == 1 && s.Scenes[0].State == "fighting" }, false)
	before := findPlayer(s, b.id)
	b.send(t, protocol.Request{Kind: "enter", Node: "a"})
	select {
	case e := <-b.errors:
		if e != "error.join_window" {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("locked door accepted")
	}
	s = waitFor(t, peers, func(s *protocol.State) bool {
		return findPlayer(s, b.id).Node == before.Node && findPlayer(s, b.id).AP == before.AP
	}, false)
	if len(s.Scenes[0].Players) != 1 {
		t.Fatal("partial handoff")
	}
	waitFor(t, []*testPeer{a}, func(s *protocol.State) bool { return len(s.Scenes) == 1 && s.Scenes[0].State == "between_waves" }, true)
	b.send(t, protocol.Request{Kind: "enter", Node: "a"})
	s = waitFor(t, peers, func(s *protocol.State) bool { return findPlayer(s, b.id).Scene == "a" }, false)
	if findPlayer(s, b.id).AP != before.AP-1 || len(s.Scenes[0].Players) != 2 {
		t.Fatal("wave admission charge")
	}
}
