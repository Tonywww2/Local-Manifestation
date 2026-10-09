package session

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"math"
	"testing"
)

func TestStartingSpaceMovementIsAuthoritativeAndTickIndependent(t *testing.T) {
	for _, rate := range []int{30, 60} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			tm := fixture(t)
			tm.config.TickRate = rate
			m := tm.players["a"]
			m.player.Online = true
			initial := m.player.Position
			ap, shared := m.player.AP, tm.shared
			for i := 0; i < rate; i++ {
				if err := tm.request(m, protocol.Request{Kind: "input", Input: model.Input{Move: model.Vec{X: 100}}}); err != nil {
					t.Fatal(err)
				}
				tm.serverTick++
				tm.stepSpaces()
			}
			if math.Abs(m.player.Position.X-initial.X-m.player.Speed) > 1e-7 || m.player.AP != ap || tm.shared != shared || m.player.Node != "start" || m.player.Scene != "" {
				t.Fatal("room movement altered resources or depended on render/input rate", m.player)
			}
			last := m.player.Position
			tm.serverTick += uint64(rate)
			tm.stepSpaces()
			if m.player.Position != last {
				t.Fatal("stale input moved player")
			}
		})
	}
}
func TestSpaceMovementClampsCornersAndBlocksDownedOrOfflinePlayers(t *testing.T) {
	tm := fixture(t)
	m := tm.players["a"]
	m.player.Online = true
	for i := 0; i < 200; i++ {
		m.walk = model.Input{Move: model.Vec{X: -1, Y: -1}}
		m.walkUpdated = tm.serverTick
		tm.serverTick++
		tm.stepSpaces()
		if !tm.spaceArena("start").ContainsDisk(m.player.Position, 12) {
			t.Fatal("escaped hex corner")
		}
	}
	for _, life := range []string{"down", "dead"} {
		m.player.Life = life
		m.walk = model.Input{Move: model.Vec{X: 1}}
		m.walkUpdated = tm.serverTick
		before := m.player.Position
		tm.serverTick++
		tm.stepSpaces()
		if m.player.Position != before {
			t.Fatal("incapacitated player moved")
		}
	}
	m.player.Life = "alive"
	m.player.Online = false
	m.walk = model.Input{Move: model.Vec{X: 1}}
	before := m.player.Position
	tm.stepSpaces()
	if m.player.Position != before {
		t.Fatal("offline player moved")
	}
}
