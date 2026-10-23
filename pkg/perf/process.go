package perf

import (
	"runtime/metrics"
	"time"
)

type Process struct {
	HeapMB      float64 `json:"heap_mb"`
	AllocatedMB float64 `json:"allocated_mb"`
	GCPauseMS   float64 `json:"gc_pause_ms"`
	GCCount     uint64  `json:"gc_count"`
}

type ProcessSampler struct {
	samples     []metrics.Sample
	alloc       uint64
	pauses      float64
	gc          uint64
	initialized bool
	last        time.Time
	value       Process
}

// Sampling uses the runtime's metrics API, at most once per second. Allocation
// and GC counters describe this process, not an individual stage or goroutine.
func (s *ProcessSampler) Sample(now time.Time) Process {
	if !s.last.IsZero() && now.Sub(s.last) < time.Second {
		return s.value
	}
	if s.samples == nil {
		s.samples = []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}, {Name: "/gc/heap/allocs:bytes"}, {Name: "/gc/pauses:seconds"}, {Name: "/gc/cycles/total:gc-cycles"}}
	}
	metrics.Read(s.samples)
	alloc, gc := s.samples[1].Value.Uint64(), s.samples[3].Value.Uint64()
	h := s.samples[2].Value.Float64Histogram()
	pauses := 0.0
	for i, n := range h.Counts {
		if n > 0 {
			pauses += h.Buckets[i] * float64(n)
		}
	}
	p := Process{HeapMB: float64(s.samples[0].Value.Uint64()) / (1 << 20)}
	if s.initialized {
		p.AllocatedMB = float64(alloc-s.alloc) / (1 << 20)
		p.GCPauseMS = max(0, pauses-s.pauses) * 1000
		p.GCCount = gc - s.gc
	}
	s.alloc, s.gc, s.pauses, s.last, s.value, s.initialized = alloc, gc, pauses, now, p, true
	return p
}
