package client

import (
	"math"
	"testing"
	"time"

	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func movingFrame(tick uint64, x float64) protocol.Scene {
	return protocol.Scene{ID: "a", Room: "test", State: "fighting", Wave: 1, TickRate: 60, Tick: tick,
		Players:     []model.Player{{ID: "p", Scene: "a", Life: "alive", HP: 50, Position: model.Vec{X: x}}},
		Enemies:     []model.Enemy{{ID: 1, Definition: "test:enemy", Position: model.Vec{X: x}}},
		Projectiles: []model.Projectile{{ID: 2, Owner: "p", Position: model.Vec{X: x}}}}
}
func TestRenderingInterpolatesWithoutMutatingAuthority(t *testing.T) {
	start := time.Unix(1, 0)
	a, b := movingFrame(60, 0), movingFrame(63, 10)
	h := sceneHistory{}
	h.Push(a, 1, start)
	h.Push(b, 1, start.Add(50*time.Millisecond))
	view := h.View(start.Add(75*time.Millisecond), 50*time.Millisecond)
	for _, x := range []float64{view.Players[0].Position.X, view.Enemies[0].Position.X, view.Projectiles[0].Position.X} {
		if math.Abs(x-5) > 1e-8 {
			t.Fatal("not interpolated", x)
		}
	}
	view.Players[0].HP = 1
	if b.Players[0].Position.X != 10 || b.Players[0].HP != 50 {
		t.Fatal("mutated authoritative snapshot")
	}
	later := h.View(start.Add(80*time.Millisecond), 50*time.Millisecond)
	if later.Players[0].Position.X <= 5 || later.Tick != b.Tick {
		t.Fatal("render time depends on receiving another frame")
	}
}
func TestHistoryBoundariesAndDiscontinuousPositions(t *testing.T) {
	start := time.Unix(1, 0)
	for _, boundary := range []string{"floor", "room", "pause", "wave", "rewind", "stall", "rate"} {
		h := sceneHistory{}
		h.Push(movingFrame(60, 0), 1, start)
		b, floor, arrival := movingFrame(63, 100), 1, start.Add(50*time.Millisecond)
		switch boundary {
		case "floor":
			floor = 2
		case "room":
			b.ID = "b"
		case "pause":
			b.State = "paused"
		case "wave":
			b.Wave = 2
		case "rewind":
			b.Tick = 0
		case "stall":
			arrival = start.Add(time.Second)
		case "rate":
			b.TickRate = 30
		}
		h.Push(b, floor, arrival)
		if len(h.frames) != 1 || h.View(arrival, 100*time.Millisecond).Players[0].Position.X != 100 {
			t.Fatal("stale trajectory", boundary)
		}
	}
	h := sceneHistory{}
	a, b := movingFrame(60, 0), movingFrame(63, 100)
	b.Players[0].PositionEpoch = 1
	b.Enemies[0].PositionEpoch = 1
	b.Projectiles = nil
	h.Push(a, 1, start)
	h.Push(b, 1, start.Add(50*time.Millisecond))
	v := h.View(start.Add(75*time.Millisecond), 50*time.Millisecond)
	if v.Players[0].Position.X != 100 || v.Enemies[0].Position.X != 100 || len(v.Projectiles) != 0 {
		t.Fatal("teleport/deleted projectile interpolated")
	}
}
func TestDuplicateTickKeepsAnchorAndNeverExtrapolates(t *testing.T) {
	start := time.Unix(1, 0)
	h := sceneHistory{}
	h.Push(movingFrame(60, 0), 1, start)
	h.Push(movingFrame(63, 10), 1, start.Add(50*time.Millisecond))
	h.Push(movingFrame(63, 10), 1, start.Add(65*time.Millisecond))
	if !h.arrival.Equal(start.Add(50*time.Millisecond)) || len(h.frames) != 2 {
		t.Fatal("duplicate delayed trajectory")
	}
	v := h.View(start.Add(time.Second), 50*time.Millisecond)
	if v.Players[0].Position.X != 10 {
		t.Fatal("extrapolated through walls")
	}
}
