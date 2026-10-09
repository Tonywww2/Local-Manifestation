package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/session"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

type testPeer struct {
	catalog *content.Catalog
	conn    net.Conn
	seq     uint64
	id      string
	token   string
	state   atomic.Pointer[protocol.State]
	welcome chan protocol.Message
	errors  chan string
}

func connectPeer(t *testing.T, addr string, c *content.Catalog, name, token string) *testPeer {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	p := &testPeer{catalog: c, conn: conn, welcome: make(chan protocol.Message, 1), errors: make(chan string, 16)}
	t.Cleanup(func() { conn.Close() })
	if err = json.NewEncoder(conn).Encode(protocol.Hello{Version: protocol.Version, Name: name, Chassis: "base:scout", Digest: c.Digest, Token: token}); err != nil {
		t.Fatal(err)
	}
	go func() {
		s := bufio.NewScanner(conn)
		s.Buffer(make([]byte, 4096), 4<<20)
		for s.Scan() {
			var m protocol.Message
			if json.Unmarshal(s.Bytes(), &m) != nil {
				return
			}
			switch m.Kind {
			case "welcome":
				p.welcome <- m
			case "state":
				p.state.Store(m.State)
			case "error":
				select {
				case p.errors <- m.Text:
				default:
				}
			}
		}
	}()
	select {
	case w := <-p.welcome:
		p.id = w.Player
		p.token = w.Token
	case err := <-p.errors:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("handshake timed out")
	}
	return p
}
func (p *testPeer) send(t *testing.T, r protocol.Request) {
	t.Helper()
	p.seq++
	r.Seq = p.seq
	if err := json.NewEncoder(p.conn).Encode(r); err != nil {
		t.Fatal(err)
	}
}
func startTestServer(t *testing.T, c *content.Catalog) (string, *session.Coordinator) {
	return startTestServerWithCapacity(t, c, 8)
}
func startTestServerWithCapacity(t *testing.T, c *content.Catalog, capacity int) (string, *session.Coordinator) {
	t.Helper()
	co, err := session.New(c, session.Config{MaxPlayers: capacity, Seed: 42, Checkpoint: filepath.Join(t.TempDir(), "checkpoint.json")})
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, l, co) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not shut down")
		}
		co.Close()
	})
	return l.Addr().String(), co
}
func waitFor(t *testing.T, peers []*testPeer, predicate func(*protocol.State) bool, shoot bool) *protocol.State {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, p := range peers {
			select {
			case e := <-p.errors:
				t.Fatalf("server rejected request: %s", e)
			default:
			}
			s := p.state.Load()
			if shoot && s != nil {
				for _, pl := range s.Players {
					if pl.ID != p.id || pl.Scene == "" {
						continue
					}
					for _, sc := range s.Scenes {
						if sc.ID != pl.Scene || len(sc.Enemies) == 0 {
							continue
						}
						target := sc.Enemies[0].Position
						in := model.Input{Aim: target, Attack: true}
						if target.Sub(pl.Position).Length() > p.catalog.Weapon(pl.Weapon).ProjectileSpeed*1.5 {
							in.Move = target.Sub(pl.Position).Unit()
						}
						p.send(t, protocol.Request{Kind: "input", Input: in})
					}
				}
			}
		}
		s := peers[0].state.Load()
		if s != nil && predicate(s) {
			return s
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("state transition timed out; last=%+v", peers[0].state.Load())
	return nil
}
func findPlayer(s *protocol.State, id string) model.Player {
	for _, p := range s.Players {
		if p.ID == id {
			return p
		}
	}
	return model.Player{}
}
func completed(s *protocol.State, id string) bool {
	for _, n := range s.Nodes {
		if n.ID == id {
			return n.Completed
		}
	}
	return false
}
func TestTCPIndependentScenesAndReconnect(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := startTestServer(t, c)
	a := connectPeer(t, addr, c, "Alice", "")
	b := connectPeer(t, addr, c, "Bob", "")
	peers := []*testPeer{a, b}
	waitFor(t, peers, func(s *protocol.State) bool { return len(s.Players) == 2 }, false)
	a.send(t, protocol.Request{Kind: "start"})
	waitFor(t, peers, func(s *protocol.State) bool { return s.Phase == "exploring" }, false)
	travel(t, peers, map[string]string{a.id: "a", b.id: "b"}, true)
	s := waitFor(t, peers, func(s *protocol.State) bool {
		return len(s.Scenes) == 2 && len(s.Scenes[0].Enemies) > 0 && len(s.Scenes[1].Enemies) > 0
	}, false)
	if path := os.Getenv("HSM_RENDER_STATE"); path != "" {
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if findPlayer(s, a.id).AP != 3 || findPlayer(s, b.id).AP != 3 {
		t.Fatal("wrong AP debit")
	}
	before := findPlayer(s, a.id).AP
	// Replay a previously accepted request; it must not consume again.
	if err := json.NewEncoder(a.conn).Encode(protocol.Request{Seq: a.seq, Kind: "enter", Node: "a"}); err != nil {
		t.Fatal(err)
	}
	a.conn.Close()
	again := connectPeer(t, addr, c, "Alice", a.token)
	if again.id != a.id {
		t.Fatal("reconnect duplicated player")
	}
	s = waitFor(t, []*testPeer{again, b}, func(s *protocol.State) bool { return findPlayer(s, again.id).Online }, false)
	if len(s.Players) != 2 || findPlayer(s, again.id).AP != before {
		t.Fatal("reconnect changed roster or AP")
	}
}
func TestTCPThreeFloorLoop(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		for i := range d.Weapons {
			d.Weapons[i].Base = 10000
			d.Weapons[i].Cooldown = .04
		}
		for i := range d.Enemies {
			d.Enemies[i].HP = 1
			d.Enemies[i].Speed = 1
			d.Enemies[i].Damage = 1
		}
		for i := range a.Rooms {
			a.Rooms[i].Waves = a.Rooms[i].Waves[:1]
		}
	}, "")
	addr, _ := startTestServer(t, c)
	a := connectPeer(t, addr, c, "Alice", "")
	b := connectPeer(t, addr, c, "Bob", "")
	peers := []*testPeer{a, b}
	waitFor(t, peers, func(s *protocol.State) bool { return len(s.Players) == 2 }, false)
	for floor := 1; floor <= 3; floor++ {
		if floor == 1 {
			a.send(t, protocol.Request{Kind: "start"})
		}
		waitFor(t, peers, func(s *protocol.State) bool { return s.Phase == "exploring" && s.Floor == floor }, false)
		travel(t, peers, map[string]string{a.id: "a", b.id: "b"}, true)
		waitFor(t, peers, func(s *protocol.State) bool { return completed(s, "a") && completed(s, "b") }, true)
		travel(t, peers, map[string]string{a.id: "c", b.id: "c"}, true)
		waitFor(t, peers, func(s *protocol.State) bool { return completed(s, "c") }, true)
		travel(t, peers, map[string]string{a.id: "boss", b.id: "boss"}, false)
		a.send(t, protocol.Request{Kind: "propose", Node: "boss"})
		waitFor(t, peers, func(s *protocol.State) bool { return s.Vote != nil && s.Vote.Node == "boss" }, false)
		b.send(t, protocol.Request{Kind: "vote", Accept: true})
		waitFor(t, peers, func(s *protocol.State) bool {
			if floor == 3 {
				return s.Phase == "won"
			}
			return s.Phase == "intermission" && s.Floor == floor
		}, true)
		if floor < 3 {
			walkToDevice(t, c, peers, true)
			a.send(t, protocol.Request{Kind: "device", Node: "next"})
			waitFor(t, peers, func(s *protocol.State) bool { return s.Vote != nil }, false)
			if path := os.Getenv("HSM_EXIT_RENDER_STATE"); path != "" && floor == 1 {
				state := protocol.CloneState(*a.state.Load())
				for i, p := range state.Players {
					if p.ID == a.id {
						state.Players[0], state.Players[i] = state.Players[i], state.Players[0]
						break
					}
				}
				data, err := json.MarshalIndent(state, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			b.send(t, protocol.Request{Kind: "device", Node: "next"})
			waitFor(t, peers, func(s *protocol.State) bool { return s.Phase == "exploring" && s.Floor == floor+1 }, false)
		}
	}
}

func TestTCPJSONRoomAndCustomMapIdentities(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		// This fixture verifies template/identity routing; keep the players alive
		// while they physically approach the emergency device.
		for i := range d.Enemies {
			d.Enemies[i].Damage = 1
			d.Enemies[i].Speed = 1
			d.Enemies[i].Radius = 10
		}
		d.Enemies[0], d.Enemies[1] = d.Enemies[1], d.Enemies[0]
		room := a.Rooms[0]
		room.ID = "base:wide-room"
		room.Width = 1200
		room.Height = 300
		room.SpawnRadius = model.Vec{X: 450, Y: 100}
		room.Waves = []model.WaveDef{{Enemies: []model.SpawnGroupDef{{Enemy: "base:warden", Count: model.CountDef{Base: 1}}}, Reward: model.RewardDef{XP: model.CountDef{Base: 7}}}}
		a.Rooms = append(a.Rooms, room)
		for i := range a.Nodes {
			if a.Nodes[i].ID == "base:combat-a" {
				a.Nodes[i].Room = room.ID
			}
		}
		for i := range a.Maps {
			m := &a.Maps[i]
			m.Start = "arrival"
			m.Boss = "exit"
			for j := range m.Nodes {
				n := &m.Nodes[j]
				if n.ID == "start" {
					n.ID = "arrival"
				}
				if n.ID == "boss" {
					n.ID = "exit"
				}
				for k, edge := range n.Edges {
					if edge == "start" {
						n.Edges[k] = "arrival"
					}
					if edge == "boss" {
						n.Edges[k] = "exit"
					}
				}
			}
		}
	}, "")
	addr, _ := startTestServer(t, c)
	a := connectPeer(t, addr, c, "Alice", "")
	b := connectPeer(t, addr, c, "Bob", "")
	peers := []*testPeer{a, b}
	waitFor(t, peers, func(s *protocol.State) bool { return len(s.Players) == 2 }, false)
	a.send(t, protocol.Request{Kind: "start"})
	waitFor(t, peers, func(s *protocol.State) bool { return s.Phase == "exploring" }, false)
	travel(t, peers, map[string]string{a.id: "a", b.id: "b"}, true)
	state := waitFor(t, peers, func(s *protocol.State) bool {
		return len(s.Scenes) == 2 && len(s.Scenes[0].Enemies) > 0 && len(s.Scenes[1].Enemies) > 0
	}, false)
	for _, sc := range state.Scenes {
		if sc.ID == "a" && (sc.Room != "base:wide-room" || sc.Width != 1200 || sc.Height != 300 || len(sc.Enemies) != 1 || sc.Enemies[0].Definition != "base:warden") {
			t.Fatal("JSON-only room was not used", sc)
		}
	}
	if path := os.Getenv("HSM_CUSTOM_RENDER_STATE"); path != "" {
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Normal boss is unreachable while scenes are pending, but enhanced routing uses
	// the configured exit identity and the same centralized gather transaction.
	walkToDevice(t, c, peers, false)
	a.send(t, protocol.Request{Kind: "propose", Node: "exhaust"})
	waitFor(t, peers, func(s *protocol.State) bool { return s.Vote != nil }, false)
	b.send(t, protocol.Request{Kind: "vote", Accept: true})
	waitFor(t, peers, func(s *protocol.State) bool { return s.SharedAP == 0 }, false)
	a.send(t, protocol.Request{Kind: "propose", Node: "enhanced"})
	waitFor(t, peers, func(s *protocol.State) bool { return s.Vote != nil }, false)
	b.send(t, protocol.Request{Kind: "vote", Accept: true})
	state = waitFor(t, peers, func(s *protocol.State) bool { return len(s.Scenes) == 1 && s.Scenes[0].ID == "exit" }, false)
	if state.Scenes[0].Room != "base:guardian-enhanced" || !state.Cursed || findPlayer(state, a.id).Scene != "exit" || findPlayer(state, b.id).Scene != "exit" {
		t.Fatal("custom boss identity ignored", state)
	}
}
