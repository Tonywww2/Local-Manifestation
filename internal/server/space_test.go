package server

import (
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"testing"
)

func TestLiveStartingSpaceHasAnAvatarAndAcceptsMovement(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := startTestServer(t, c)
	p := connectPeer(t, addr, c, "Starting player", "")
	peers := []*testPeer{p}
	p.send(t, protocol.Request{Kind: "start"})
	s := waitFor(t, peers, func(s *protocol.State) bool { return s.Phase == "exploring" }, false)
	initial := s.Players[0]
	a := geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight)
	if !a.ContainsDisk(initial.Position, 12) || initial.Scene != "" || initial.Node != "start" {
		t.Fatal("starting avatar not in the starting room", initial)
	}
	p.send(t, protocol.Request{Kind: "input", Input: model.Input{Move: model.Vec{X: 1}}})
	s = waitFor(t, peers, func(s *protocol.State) bool { return s.Players[0].Position.Sub(initial.Position).Length() > 5 }, false)
	if s.Players[0].AP != initial.AP || s.SharedAP != 3 || !a.ContainsDisk(s.Players[0].Position, 12) {
		t.Fatal("invalid movement transaction", s.Players[0])
	}
}
