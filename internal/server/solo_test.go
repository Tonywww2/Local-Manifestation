package server

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/locales"
	"hsmultiplayer/pkg/localization"
	"hsmultiplayer/pkg/protocol"
)

func TestSoloServerStartsAndVotesNormally(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := startTestServerWithCapacity(t, c, 1)
	p := connectPeer(t, addr, c, "单人玩家", "")
	peers := []*testPeer{p}
	p.send(t, protocol.Request{Kind: "start"})
	s := waitFor(t, peers, func(s *protocol.State) bool { return s.Phase == "exploring" }, false)
	if len(s.Players) != 1 || s.Players[0].AP != s.PersonalInitial {
		t.Fatal("single-player layer initialization failed")
	}
	walkToDevice(t, c, peers, false)
	p.send(t, protocol.Request{Kind: "propose", Node: "exhaust"})
	waitFor(t, peers, func(s *protocol.State) bool { return s.SharedAP == 0 && s.Vote == nil }, false)
	p.send(t, protocol.Request{Kind: "propose", Node: "enhanced"})
	waitFor(t, peers, func(s *protocol.State) bool { return len(s.Scenes) == 1 && s.Scenes[0].ID == "boss" && s.Cursed }, false)
}

func TestHandshakeRejectionCanBeTranslatedPerClient(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := startTestServer(t, c)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if err := json.NewEncoder(conn).Encode(protocol.Hello{Version: protocol.Version, Digest: "wrong", Name: "Player"}); err != nil {
		t.Fatal(err)
	}
	var m protocol.Message
	if err := json.NewDecoder(conn).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m.Kind != "error" || m.Code != "error.content_mismatch" {
		t.Fatalf("missing error code: %+v", m)
	}
	translations, err := localization.Load(locales.Files)
	if err != nil {
		t.Fatal(err)
	}
	zh, en := translations.Text("zh-CN", m.Code, m.Params), translations.Text("en", m.Code, m.Params)
	if zh == en || zh == m.Code || en == m.Code {
		t.Fatalf("rejection was not localized: %s / %s", zh, en)
	}
}
