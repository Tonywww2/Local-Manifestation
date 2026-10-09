package client

import (
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"testing"
	"time"
)

func TestSelfMovesBeforeSnapshotAndPresentationIsFrameIndependent(t *testing.T) {
	a := geometry.NewArena(800, 500)
	at := time.Unix(100, 0)
	pl := model.Player{Node: "start", Position: a.Center, Life: "alive", PositionEpoch: 1}
	for _, fps := range []int{30, 60, 120, 240} {
		p := MovementPrediction{}.Reconcile(pl, a, at)
		p = p.Advance(model.Input{Move: model.Vec{X: 1}}, 100, a, at.Add(time.Second/time.Duration(fps)), true)
		if p.Position.X <= pl.Position.X {
			t.Fatal("awaited snapshot")
		}
		for i := 2; i <= fps; i++ {
			p = p.Advance(model.Input{Move: model.Vec{X: 1}}, 100, a, at.Add(time.Duration(float64(time.Second)*float64(i)/float64(fps))), true)
		}
		if p.Position.Sub(a.Center.Add(model.Vec{X: 100})).Length() > 1e-5 {
			t.Fatal("FPS affected movement", fps, p.Position)
		}
	}
}
func TestPredictionCorrectsBoundsAndDiscontinuities(t *testing.T) {
	a := geometry.NewArena(800, 500)
	at := time.Unix(100, 0)
	pl := model.Player{Node: "a", Position: a.Center, Life: "alive", PositionEpoch: 1}
	p := MovementPrediction{}.Reconcile(pl, a, at)
	for i := 1; i <= 120; i++ {
		p = p.Advance(model.Input{Move: model.Vec{X: 1}}, 1000, a, at.Add(time.Duration(i)*time.Second/60), true)
	}
	if !a.ContainsDisk(p.Position, 12) {
		t.Fatal("prediction escaped room")
	}
	pl.Node = "b"
	pl.PositionEpoch = 2
	p = p.Reconcile(pl, a, at.Add(2*time.Second))
	if p.Position != pl.Position || p.Correction != (model.Vec{}) {
		t.Fatal("cross-room prediction reused")
	}
	p = p.Advance(model.Input{Move: model.Vec{X: 1}}, 200, a, at.Add(2100*time.Millisecond), false)
	if p.Position != pl.Position {
		t.Fatal("paused or dead prediction moved")
	}
}

func TestOnlyFirstRingRoomsAreLoaded(t *testing.T) {
	from := model.Node{}
	for _, n := range []model.Node{{Q: 1}, {R: 1}, {Q: -1, R: 1}, {Q: -1}, {R: -1}, {Q: 1, R: -1}} {
		if !inFirstRing(from, n) {
			t.Fatal("neighbor missing")
		}
	}
	for _, n := range []model.Node{{Q: 2}, {Q: 1, R: 1}, {Q: -2, R: 1}} {
		if inFirstRing(from, n) {
			t.Fatal("distant room eagerly loaded")
		}
	}
}
