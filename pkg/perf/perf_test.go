package perf

import (
	"testing"
	"time"
)

func TestRecorderWindowAndDetachedSnapshots(t *testing.T) {
	var r Recorder
	now := time.Now()
	r.Snapshot(now)
	for i := 0; i < 19; i++ {
		r.Observe("draw", time.Millisecond)
	}
	r.Observe("draw", 20*time.Millisecond)
	r.Observe("logic", 2*time.Millisecond)
	if got := r.Snapshot(now.Add(500 * time.Millisecond)); len(got.Stages) != 0 {
		t.Fatal("published a partial window")
	}
	s := r.Snapshot(now.Add(time.Second))
	if len(s.Stages) != 2 || s.Stages[0].Name != "draw" {
		t.Fatal(s)
	}
	draw := s.Stages[0]
	if draw.Calls != 20 || draw.TotalMS != 39 || draw.MeanMS != 1.95 || draw.MaxMS != 20 || draw.P95MS < 1 || draw.P95MS > 2 {
		t.Fatal(draw)
	}
	copy := Clone(s)
	copy.Stages[0].Name = "changed"
	if s.Stages[0].Name != "draw" {
		t.Fatal("clone aliases a published slice")
	}
	r.Observe("logic", 3*time.Millisecond)
	next := r.Snapshot(now.Add(2 * time.Second))
	if len(next.Stages) != 1 || next.Stages[0].TotalMS != 3 || s.Stages[0] != draw {
		t.Fatal("window retained old counters or mutated a publication")
	}
	var absent *Recorder
	absent.Since("optional", Start())
	if len(absent.Snapshot(now).Stages) != 0 {
		t.Fatal("nil recorder returned metrics")
	}
}

func TestHighResolutionClockAndProcessSample(t *testing.T) {
	start := Start()
	var elapsed time.Duration
	for i := 0; i < 10000 && elapsed == 0; i++ {
		elapsed = Elapsed(start)
	}
	if elapsed <= 0 || elapsed > time.Second {
		t.Fatal("monotonic clock did not advance", elapsed)
	}
	var sampler ProcessSampler
	now := time.Now()
	p := sampler.Sample(now)
	if p.HeapMB <= 0 || p.AllocatedMB != 0 || p.GCCount != 0 || sampler.Sample(now.Add(500*time.Millisecond)) != p {
		t.Fatal("invalid baseline/cached process sample", p)
	}
}
