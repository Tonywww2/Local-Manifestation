// Package perf records bounded host timings. It never retains game state.
package perf

import (
	"math"
	"sort"
	"sync"
	"time"
)

type Stage struct {
	Name    string  `json:"name"`
	Calls   uint64  `json:"calls"`
	TotalMS float64 `json:"total_ms"`
	MeanMS  float64 `json:"mean_ms"`
	P95MS   float64 `json:"p95_ms"`
	MaxMS   float64 `json:"max_ms"`
}

// Snapshot is immutable once published. Nested stages are inclusive timings;
// parent and child totals must not be added together.
type Snapshot struct {
	Seconds float64 `json:"seconds"`
	Stages  []Stage `json:"stages,omitempty"`
}

type counter struct {
	calls          uint64
	total, maximum time.Duration
	histogram      [24]uint64
}

// Recorder has one state owner behind its lock. Transport goroutines may submit
// durations, while simulation/render owners receive detached, read-only values.
// No allocation occurs per observation after the finite stage names are seen.
type Recorder struct {
	mu       sync.Mutex
	counters map[string]counter
	start    time.Time
	last     Snapshot
}

func (r *Recorder) Observe(name string, elapsed time.Duration) {
	if r == nil {
		return
	}
	if elapsed < 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.counters == nil {
		r.counters = map[string]counter{}
	}
	if r.start.IsZero() {
		r.start = time.Now()
	}
	if _, ok := r.counters[name]; !ok && len(r.counters) >= 128 {
		return
	}
	c := r.counters[name]
	c.calls++
	c.total += elapsed
	c.maximum = max(c.maximum, elapsed)
	bin := 0
	for micros := uint64(elapsed / time.Microsecond); micros > 1 && bin < len(c.histogram)-1; micros >>= 1 {
		bin++
	}
	c.histogram[bin]++
	r.counters[name] = c
}

func (r *Recorder) Since(name string, start Stamp) { r.Observe(name, Elapsed(start)) }

func (r *Recorder) Snapshot(now time.Time) Snapshot {
	if r == nil {
		return Snapshot{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.start.IsZero() {
		r.start = now
		return r.last
	}
	if now.Sub(r.start) < time.Second {
		return r.last
	}
	next := Snapshot{Seconds: now.Sub(r.start).Seconds()}
	for name, c := range r.counters {
		if c.calls == 0 {
			continue
		}
		p95, count, threshold := time.Duration(0), uint64(0), uint64(math.Ceil(float64(c.calls)*.95))
		for bin, n := range c.histogram {
			count += n
			if count >= threshold {
				p95 = min(c.maximum, time.Duration(uint64(1)<<(bin+1))*time.Microsecond)
				break
			}
		}
		next.Stages = append(next.Stages, Stage{Name: name, Calls: c.calls, TotalMS: float64(c.total) / 1e6, MeanMS: float64(c.total) / 1e6 / float64(c.calls), P95MS: float64(p95) / 1e6, MaxMS: float64(c.maximum) / 1e6})
		r.counters[name] = counter{}
	}
	sort.Slice(next.Stages, func(i, j int) bool {
		if next.Stages[i].TotalMS == next.Stages[j].TotalMS {
			return next.Stages[i].Name < next.Stages[j].Name
		}
		return next.Stages[i].TotalMS > next.Stages[j].TotalMS
	})
	r.start, r.last = now, next
	return next
}

func Clone(s Snapshot) Snapshot { s.Stages = append([]Stage(nil), s.Stages...); return s }
