package client

import (
	"math"
	"testing"
	"time"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func TestProjectilePresentationContinuousFromBirth(t *testing.T) {
	const key = "origin:shot:1"
	frame := func(seconds, x, age float64, node model.Node, present bool) worldFrame {
		s := protocol.Scene{ID: "space:" + node.ID, Width: content.DefaultSpaceWidth, Height: content.DefaultSpaceHeight}
		poses := map[string]worldShotPose{}
		if present {
			position := model.Vec{X: x}
			s.Projectiles = []model.Projectile{{ID: int(seconds * 100), Identity: key, Age: age, Position: localPosition(node, s, position), Velocity: model.Vec{X: 100}}}
			poses[key] = worldShotPose{Position: position, Velocity: model.Vec{X: 100}, Age: age}
		}
		return worldFrame{Seconds: seconds, Scenes: map[string]protocol.Scene{node.ID: s}, Nodes: map[string]model.Node{node.ID: node}, Shots: poses}
	}
	a, b := model.Node{ID: "a"}, model.Node{ID: "b", Q: 1}
	frames := []worldFrame{frame(1, 0, 0, a, false), frame(1.05, 103, .03, a, true), frame(1.10, 108, .08, a, true), frame(1.15, 113, .13, b, true)}
	// The latest assignment may already be in another room with a different
	// local ID. It must not pull a new projectile forward then backwards.
	previous := math.Inf(-1)
	for tick := 0; tick <= 15; tick++ {
		target := 1 + float64(tick)/100
		view := interpolateWorld(frames, target)[b.ID]
		if len(view.Projectiles) != 1 {
			t.Fatal("new projectile disappeared")
		}
		x := worldPosition(b, view, view.Projectiles[0].Position).X
		want := 100 + max(0, target-1.02)*100
		if math.Abs(x-want) > 1e-8 || x < previous {
			t.Fatalf("time %.2f: x %.3f, want %.3f, previous %.3f", target, x, want, previous)
		}
		previous = x
	}
	if frames[3].Shots[key].Position.X != 113 {
		t.Fatal("presentation mutated the snapshot")
	}
	// A short stationary melee arc remains visible even when its whole lifetime
	// is shorter than the interpolation delay, and removal remains immediate.
	frames[1].Shots[key] = worldShotPose{Position: model.Vec{X: 103}, Age: .03}
	v := interpolateWorld(frames[:2], 1)[a.ID]
	if len(v.Projectiles) != 1 || math.Abs(worldPosition(a, v, v.Projectiles[0].Position).X-103) > 1e-8 {
		t.Fatal("melee arc hidden or moved")
	}
	frames = append(frames, frame(1.2, 0, 0, b, false))
	if len(interpolateWorld(frames, 1.1)[b.ID].Projectiles) != 0 {
		t.Fatal("removed shot resurrected")
	}
}

func TestWorldHistoryReleasesOldSnapshotsAndKeepsDelayBracket(t *testing.T) {
	var h worldHistory
	now := time.Now()
	for i := 0; i <= 200; i++ {
		h.Push(worldFrame{Seconds: float64(i) / 20, Scenes: map[string]protocol.Scene{"room": {ID: "room"}}}, 1, 30, now.Add(time.Duration(i)*50*time.Millisecond), 20)
	}
	if len(h.frames) < 42 || len(h.frames) > 45 || h.frames[0].Seconds > 7.9 || h.frames[1].Seconds < 7.9 {
		t.Fatal("lost interpolation bracket or retained excessive history", len(h.frames), h.frames[0].Seconds)
	}
	for _, f := range h.frames[len(h.frames):cap(h.frames)] {
		if f.Scenes != nil {
			t.Fatal("discarded scene tree retained in backing array")
		}
	}
	h.Push(worldFrame{Seconds: 10.05}, 2, 30, now.Add(201*50*time.Millisecond), 20)
	if len(h.frames) != 1 {
		t.Fatal("floor transition retained old history")
	}
}
