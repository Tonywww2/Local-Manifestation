// Package navigation computes paths as values on a bounded hex navigation mesh.
package navigation

import (
	"container/heap"
	"fmt"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"math"
)

type Space struct {
	Arena           geometry.Arena
	Obstacles       []model.Obstacle
	Radius, Spacing float64
}

func Clear(s Space, a, b model.Vec) bool {
	if !s.Arena.ClearStep(a, b, s.Radius) {
		return false
	}
	return geometry.ObstaclesClear(a, b, s.Radius, s.Obstacles)
}

type cell struct{ q, r int }
type entry struct {
	key   cell
	score float64
}
type queue []entry

func (q queue) Len() int           { return len(q) }
func (q queue) Less(i, j int) bool { return q[i].score < q[j].score }
func (q queue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(v any)        { *q = append(*q, v.(entry)) }
func (q *queue) Pop() any          { v := (*q)[len(*q)-1]; *q = (*q)[:len(*q)-1]; return v }
func Path(s Space, start, goal model.Vec) ([]model.Vec, error) {
	if !start.Finite() || !goal.Finite() || s.Radius < 0 {
		return nil, fmt.Errorf("invalid path input")
	}
	if s.Spacing <= 0 {
		s.Spacing = 48
	}
	goal = s.Arena.ClampDisk(goal, s.Radius)
	if Clear(s, start, goal) {
		return []model.Vec{goal}, nil
	}
	point := func(c cell) model.Vec {
		return s.Arena.Center.Add(model.Vec{X: s.Spacing * (float64(c.q) + float64(c.r)/2), Y: s.Spacing * math.Sqrt(3) / 2 * float64(c.r)})
	}
	nearest := func(v model.Vec) (cell, bool) {
		r := int(math.Round((v.Y - s.Arena.Center.Y) / (s.Spacing * math.Sqrt(3) / 2)))
		q := int(math.Round((v.X-s.Arena.Center.X)/s.Spacing - float64(r)/2))
		best := cell{}
		distance := math.MaxFloat64
		found := false
		for dr := -2; dr <= 2; dr++ {
			for dq := -2; dq <= 2; dq++ {
				c := cell{q + dq, r + dr}
				p := point(c)
				if Clear(s, v, p) && p.Sub(v).Length() < distance {
					best = c
					distance = p.Sub(v).Length()
					found = true
				}
			}
		}
		return best, found
	}
	a, ok := nearest(start)
	if !ok {
		return nil, fmt.Errorf("no navigable start")
	}
	b, ok := nearest(goal)
	if !ok {
		return nil, fmt.Errorf("no navigable goal")
	}
	dirs := [6]cell{{1, 0}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}, {1, -1}}
	open := &queue{{a, 0}}
	heap.Init(open)
	cost := map[cell]float64{a: 0}
	previous := map[cell]cell{}
	closed := map[cell]bool{}
	for open.Len() > 0 && len(closed) < 8192 {
		cur := heap.Pop(open).(entry).key
		if closed[cur] {
			continue
		}
		closed[cur] = true
		if cur == b {
			var reverse []model.Vec
			for c := b; c != a; c = previous[c] {
				reverse = append(reverse, point(c))
			}
			reverse = append(reverse, point(a))
			path := make([]model.Vec, 0, len(reverse)+1)
			for i := len(reverse) - 1; i >= 0; i-- {
				path = append(path, reverse[i])
			}
			path = append(path, goal)
			// Greedy line-of-sight smoothing preserves obstacle clearance.
			out := []model.Vec{}
			origin := start
			for i := 0; i < len(path); {
				last := i
				for j := i + 1; j < len(path); j++ {
					if Clear(s, origin, path[j]) {
						last = j
					}
				}
				out = append(out, path[last])
				origin = path[last]
				i = last + 1
			}
			return out, nil
		}
		for _, d := range dirs {
			next := cell{cur.q + d.q, cur.r + d.r}
			if closed[next] || !Clear(s, point(cur), point(next)) {
				continue
			}
			g := cost[cur] + s.Spacing
			if old, ok := cost[next]; !ok || g < old {
				cost[next] = g
				previous[next] = cur
				heap.Push(open, entry{next, g + point(next).Sub(point(b)).Length()})
			}
		}
	}
	return nil, fmt.Errorf("no path")
}
