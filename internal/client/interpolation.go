package client

import (
	"time"

	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
)

type sceneFrame struct {
	scene   protocol.Scene
	seconds float64
}

// sceneHistory owns immutable received frames, never the authoritative state.
type sceneHistory struct {
	frames     []sceneFrame
	floor      int
	arrival    time.Time
	renderTime float64
}

func sceneSeconds(s protocol.Scene) float64 {
	rate := s.TickRate
	if !timing.LogicRate(rate) {
		rate = timing.DefaultLogicRate
	}
	return float64(s.Tick) / float64(rate)
}
func (h *sceneHistory) Clear() { *h = sceneHistory{} }
func (h *sceneHistory) Push(s protocol.Scene, floor int, arrival time.Time, snapshotRates ...int) {
	seconds := sceneSeconds(s)
	gap := 500 * time.Millisecond
	if len(snapshotRates) > 0 && snapshotRates[0] > 0 {
		gap = max(gap, 3*time.Second/time.Duration(snapshotRates[0]))
	}
	if n := len(h.frames); n > 0 {
		last := h.frames[n-1]
		if h.floor != floor || last.scene.ID != s.ID || last.scene.Room != s.Room ||
			last.scene.TickRate != s.TickRate || last.scene.State != s.State ||
			last.scene.Wave != s.Wave || s.Tick < last.scene.Tick ||
			seconds-last.seconds > gap.Seconds() || arrival.Sub(h.arrival) > gap {
			h.Clear()
		} else if s.Tick == last.scene.Tick {
			// A higher snapshot rate can repeat a simulation tick. Keep its time anchor.
			h.frames[n-1].scene = s
			return
		}
	}
	h.floor = floor
	h.arrival = arrival
	h.frames = append(h.frames, sceneFrame{s, seconds})
	if len(h.frames) > 128 {
		copy(h.frames, h.frames[len(h.frames)-128:])
		h.frames = h.frames[:128]
	}
}

func copyScenePositions(s protocol.Scene) protocol.Scene {
	// Packet contents remain immutable. Only the position-bearing slices are
	// copied for presentation; inventory/status trees are never edited by rendering.
	s.Players = append([]model.Player(nil), s.Players...)
	s.Enemies = append([]model.Enemy(nil), s.Enemies...)
	s.Projectiles = append([]model.Projectile(nil), s.Projectiles...)
	return s
}
func lerp(a, b model.Vec, alpha float64) model.Vec { return a.Add(b.Sub(a).Scale(alpha)) }

func (h *sceneHistory) View(now time.Time, delay time.Duration) protocol.Scene {
	n := len(h.frames)
	if n == 0 {
		return protocol.Scene{}
	}
	latest := h.frames[n-1]
	if n < 2 || delay <= 0 || latest.scene.State != "fighting" && latest.scene.State != "idle" && latest.scene.State != "waiting" && latest.scene.State != "between_waves" {
		return copyScenePositions(latest.scene)
	}
	target := latest.seconds + now.Sub(h.arrival).Seconds() - delay.Seconds()
	target = min(latest.seconds, max(h.frames[0].seconds, max(h.renderTime, target)))
	h.renderTime = target
	return interpolateScene(h.frames, target)
}

// interpolateScene is a pure value transformation: no clock, I/O or history mutation.
func interpolateScene(frames []sceneFrame, target float64) protocol.Scene {
	n := len(frames)
	result := copyScenePositions(frames[n-1].scene)
	a, b := frames[0], frames[n-1]
	for i := 1; i < n; i++ {
		if frames[i].seconds >= target {
			a, b = frames[i-1], frames[i]
			break
		}
	}
	alpha := 1.0
	if b.seconds > a.seconds {
		alpha = min(1.0, max(0.0, (target-a.seconds)/(b.seconds-a.seconds)))
	}
	playersA, playersB := map[string]model.Player{}, map[string]model.Player{}
	for _, p := range a.scene.Players {
		playersA[p.ID] = p
	}
	for _, p := range b.scene.Players {
		playersB[p.ID] = p
	}
	for i, p := range result.Players {
		pa, okA := playersA[p.ID]
		pb, okB := playersB[p.ID]
		if okA && okB && pa.Scene == pb.Scene && pa.Life == pb.Life && pb.Life == p.Life && pa.PositionEpoch == pb.PositionEpoch && pb.PositionEpoch == p.PositionEpoch {
			result.Players[i].Position = lerp(pa.Position, pb.Position, alpha)
		}
	}
	enemiesA, enemiesB := map[int]model.Enemy{}, map[int]model.Enemy{}
	for _, e := range a.scene.Enemies {
		enemiesA[e.ID] = e
	}
	for _, e := range b.scene.Enemies {
		enemiesB[e.ID] = e
	}
	for i, e := range result.Enemies {
		ea, okA := enemiesA[e.ID]
		eb, okB := enemiesB[e.ID]
		if okA && okB && ea.Definition == eb.Definition && eb.Definition == e.Definition && ea.PositionEpoch == eb.PositionEpoch && eb.PositionEpoch == e.PositionEpoch {
			result.Enemies[i].Position = lerp(ea.Position, eb.Position, alpha)
		}
	}
	shotsA, shotsB := map[int]model.Projectile{}, map[int]model.Projectile{}
	for _, p := range a.scene.Projectiles {
		shotsA[p.ID] = p
	}
	for _, p := range b.scene.Projectiles {
		shotsB[p.ID] = p
	}
	for i, p := range result.Projectiles {
		pa, okA := shotsA[p.ID]
		pb, okB := shotsB[p.ID]
		if okA && okB && pa.Owner == pb.Owner && pb.Owner == p.Owner {
			result.Projectiles[i].Position = lerp(pa.Position, pb.Position, alpha)
		}
	}
	return result
}

// Compare matching identities only; creation/removal is not interpolated motion.
func displayPositionsChanged(a, b protocol.Scene) bool {
	for i, p := range b.Players {
		if i < len(a.Players) && a.Players[i].ID == p.ID && a.Players[i].Position != p.Position {
			return true
		}
	}
	for i, e := range b.Enemies {
		if i < len(a.Enemies) && a.Enemies[i].ID == e.ID && a.Enemies[i].Position != e.Position {
			return true
		}
	}
	for i, p := range b.Projectiles {
		if i < len(a.Projectiles) && a.Projectiles[i].ID == p.ID && a.Projectiles[i].Position != p.Position {
			return true
		}
	}
	return false
}
