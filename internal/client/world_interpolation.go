package client

import (
	"fmt"
	"time"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
)

type worldPlayerPose struct {
	Position model.Vec
	Epoch    uint64
	Life     string
}
type worldShotPose struct {
	Position, Velocity model.Vec
	Age                float64
}
type worldFrame struct {
	Seconds      float64
	Scenes       map[string]protocol.Scene
	Nodes        map[string]model.Node
	Players      map[string]worldPlayerPose
	Shots        map[string]worldShotPose
	EnemyActions map[int]model.EnemyActionState
}
type worldHistory struct {
	frames      []worldFrame
	floor, rate int
	arrival     time.Time
	renderTime  float64
}

func (h *worldHistory) Clear() { *h = worldHistory{} }

func worldPosition(n model.Node, s protocol.Scene, p model.Vec) model.Vec {
	a := geometry.NewArena(s.Width, s.Height)
	return a.WorldPoint(n, p, geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight).Radius)
}
func localPosition(n model.Node, s protocol.Scene, p model.Vec) model.Vec {
	a := geometry.NewArena(s.Width, s.Height)
	radius := geometry.NewArena(content.DefaultSpaceWidth, content.DefaultSpaceHeight).Radius
	return a.LocalPoint(n, p, radius)
}
func projectileDisplayID(s protocol.Scene, p model.Projectile) string {
	if p.Identity != "" {
		return p.Identity
	}
	return fmt.Sprintf("%s:shot:%d", s.ID, p.ID)
}

// Capture only the current independent room. Received scene/player trees remain immutable;
// interpolation owns positions and never copies the entire mutable game world.
func (g *Game) captureWorldFrame() worldFrame {
	rate := g.state.TickRate
	if !timing.LogicRate(rate) {
		rate = timing.DefaultLogicRate
	}
	f := worldFrame{Seconds: float64(g.state.ServerTick) / float64(rate), Scenes: map[string]protocol.Scene{}, Nodes: map[string]model.Node{}, Players: map[string]worldPlayerPose{}, Shots: map[string]worldShotPose{}, EnemyActions: map[int]model.EnemyActionState{}}
	from := g.currentNode()
	for _, n := range g.state.Nodes {
		if n.ID != from.ID {
			continue
		}
		s := g.sceneForNode(n)
		f.Scenes[n.ID], f.Nodes[n.ID] = s, n
		for _, action := range s.EnemyActions {
			f.EnemyActions[action.ID] = action
		}
		for _, p := range s.Players {
			f.Players[p.ID] = worldPlayerPose{worldPosition(n, s, p.Position), p.PositionEpoch, p.Life}
		}
		for _, p := range s.Projectiles {
			position := worldPosition(n, s, p.Position)
			velocity := worldPosition(n, s, p.Position.Add(p.Velocity)).Sub(position)
			f.Shots[projectileDisplayID(s, p)] = worldShotPose{position, velocity, p.Age}
		}
	}
	return f
}
func (h *worldHistory) Push(f worldFrame, floor, rate int, at time.Time, snapshotRate int) {
	gap := 500 * time.Millisecond
	if snapshotRate > 0 {
		gap = max(gap, 3*time.Second/time.Duration(snapshotRate))
	}
	if n := len(h.frames); n > 0 {
		last := h.frames[n-1]
		if floor != h.floor || rate != h.rate || f.Seconds < last.Seconds || f.Seconds-last.Seconds > gap.Seconds() || at.Sub(h.arrival) > gap {
			h.Clear()
		} else if f.Seconds == last.Seconds {
			h.frames[n-1] = f
			return
		}
	}
	h.floor, h.rate, h.arrival = floor, rate, at
	h.frames = append(h.frames, f)
	// Two seconds covers the configured maximum display delay. Keep one older
	// bracket and release discarded snapshots rather than retaining projectile trees.
	cutoff := f.Seconds - 2.1
	discard := 0
	for discard+2 < len(h.frames) && h.frames[discard+1].Seconds < cutoff {
		discard++
	}
	if discard > 0 {
		copy(h.frames, h.frames[discard:])
		clear(h.frames[len(h.frames)-discard:])
		h.frames = h.frames[:len(h.frames)-discard]
	}
	if len(h.frames) > 128 {
		copy(h.frames, h.frames[len(h.frames)-128:])
		h.frames = h.frames[:128]
	}
}
func (h *worldHistory) View(at time.Time, delay time.Duration) map[string]protocol.Scene {
	if len(h.frames) == 0 {
		return nil
	}
	last := h.frames[len(h.frames)-1]
	target := last.Seconds
	if delay > 0 {
		target = min(last.Seconds, max(h.frames[0].Seconds, max(h.renderTime, last.Seconds+at.Sub(h.arrival).Seconds()-delay.Seconds())))
	}
	h.renderTime = target
	view := interpolateWorld(h.frames, target)
	if target >= last.Seconds {
		// Only cosmetic timelines can advance between the newest samples.
		// Bound stalls and freeze paused rooms; never extrapolate game outcomes.
		dt := max(0, min(.1, at.Sub(h.arrival).Seconds()))
		for id, s := range view {
			if s.State != "paused" && s.State != "faulted" && s.State != "cleared" {
				for i := range s.Actions {
					s.Actions[i] = animation.Advance(s.Actions[i], dt)
				}
				for i := range s.EnemyActions {
					s.EnemyActions[i] = animation.AdvanceEnemy(s.EnemyActions[i], dt)
				}
			} else {
				for i := range s.Actions {
					s.Actions[i].Life = animation.AdvanceAction(s.Actions[i].Life, dt)
				}
				for i := range s.EnemyActions {
					s.EnemyActions[i].Death = animation.AdvanceAction(s.EnemyActions[i].Death, dt)
				}
			}
			view[id] = s
		}
	}
	return view
}

// A shared sample time and world coordinates preserve motion even when the
// latest snapshot assigns a player or projectile to a different room owner.
func interpolateWorld(frames []worldFrame, target float64) map[string]protocol.Scene {
	latest := frames[len(frames)-1]
	a, b := frames[0], latest
	for i := 1; i < len(frames); i++ {
		if frames[i].Seconds >= target {
			a, b = frames[i-1], frames[i]
			break
		}
	}
	alpha := 1.0
	if b.Seconds > a.Seconds {
		alpha = min(1.0, max(0.0, (target-a.Seconds)/(b.Seconds-a.Seconds)))
	}
	out := make(map[string]protocol.Scene, len(latest.Scenes))
	for id, s := range latest.Scenes {
		n := latest.Nodes[id]
		result := copyScenePositions(s)
		sa, okA := a.Scenes[id]
		sb, okB := b.Scenes[id]
		if okA && okB {
			for i, action := range result.Actions {
				pa, foundA := sceneAction(sa.Actions, action.Player)
				pb, foundB := sceneAction(sb.Actions, action.Player)
				if foundA && foundB {
					result.Actions[i] = animation.Sample(pa, pb, target-a.Seconds, b.Seconds-a.Seconds)
				}
			}
		}
		if okA && okB {
			for i, action := range result.EnemyActions {
				pa, foundA := a.EnemyActions[action.ID]
				pb, foundB := b.EnemyActions[action.ID]
				if foundA && foundB {
					result.EnemyActions[i] = animation.SampleEnemy(pa, pb, target-a.Seconds, b.Seconds-a.Seconds)
				}
			}
		}
		for i, p := range result.Players {
			pa, okA := a.Players[p.ID]
			pb, okB := b.Players[p.ID]
			if okA && okB && pa.Epoch == pb.Epoch && pb.Epoch == p.PositionEpoch && pa.Life == pb.Life && pb.Life == p.Life {
				result.Players[i].Position = localPosition(n, s, lerp(pa.Position, pb.Position, alpha))
			}
		}
		shots := result.Projectiles[:0]
		for _, p := range result.Projectiles {
			key := projectileDisplayID(s, p)
			pa, okA := a.Shots[key]
			pb, okB := b.Shots[key]
			if okA && okB {
				p.Position = localPosition(n, s, lerp(pa.Position, pb.Position, alpha))
			} else {
				position, visible := firstShotPosition(frames, key, target)
				if !visible {
					continue
				}
				p.Position = localPosition(n, s, position)
			}
			shots = append(shots, p)
		}
		result.Projectiles = shots
		if okA && okB && sa.ID == s.ID && sb.ID == s.ID && sa.State == sb.State && sb.State == s.State && sa.Wave == sb.Wave && sb.Wave == s.Wave {
			ea, eb := map[int]model.Enemy{}, map[int]model.Enemy{}
			for _, e := range sa.Enemies {
				ea[e.ID] = e
			}
			for _, e := range sb.Enemies {
				eb[e.ID] = e
			}
			for i, e := range result.Enemies {
				pa, okA := ea[e.ID]
				pb, okB := eb[e.ID]
				if okA && okB && pa.Definition == pb.Definition && pb.Definition == e.Definition && pa.PositionEpoch == pb.PositionEpoch && pb.PositionEpoch == e.PositionEpoch {
					result.Enemies[i].Position = lerp(pa.Position, pb.Position, alpha)
				}
			}
		}
		out[id] = result
	}
	return out
}

// New projectiles must not briefly use the latest position then jump backwards
// when two interpolation samples become available. Reconstruct only their first
// observed step, bounded by authoritative age. Before its sampled birth time,
// hold at the launch point, preserving immediate feedback for short melee arcs.
// Never extrapolate beyond a sample. Existing motion stays sampled, including
// steering and cross-room transfers. Snapshot removal remains immediate.
func firstShotPosition(frames []worldFrame, key string, target float64) (model.Vec, bool) {
	for _, frame := range frames {
		if p, ok := frame.Shots[key]; ok && frame.Seconds+1e-9 >= target {
			dt := min(max(0, p.Age), max(0, frame.Seconds-target))
			return p.Position.Sub(p.Velocity.Scale(dt)), true
		}
	}
	return model.Vec{}, false
}
