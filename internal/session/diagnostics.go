package session

import (
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
	"time"
)

type RoomDiagnostics struct {
	ID           string        `json:"id"`
	State        string        `json:"state"`
	TPS          float64       `json:"tps"`
	DroppedTicks uint64        `json:"dropped_ticks"`
	Enemies      int           `json:"enemies"`
	Projectiles  int           `json:"projectiles"`
	Timings      perf.Snapshot `json:"timings"`
}

type Diagnostics struct {
	At           time.Time         `json:"at"`
	TPS          float64           `json:"tps"`
	DroppedTicks uint64            `json:"dropped_ticks"`
	Timings      perf.Snapshot     `json:"timings"`
	Process      perf.Process      `json:"process"`
	Rooms        []RoomDiagnostics `json:"rooms,omitempty"`
}

// ObserveTransport accepts only durations. Network goroutines cannot access game state.
func (a *Coordinator) ObserveTransport(stage string, duration time.Duration) {
	a.performance.Observe(stage, duration)
}

// Diagnostics requests detached values from the coordinator owner. It neither
// reads a live world from another goroutine nor builds a full gameplay snapshot.
func (a *Coordinator) Diagnostics() (Diagnostics, error) {
	reply := make(chan Diagnostics, 1)
	select {
	case a.events <- event{kind: "diagnostics", diagnostics: reply}:
	case <-a.done:
		return Diagnostics{}, protocol.Reject("error.server_stopped")
	}
	select {
	case value := <-reply:
		return value, nil
	case <-a.done:
		return Diagnostics{}, protocol.Reject("error.server_stopped")
	}
}

func (t *team) diagnostics(now time.Time) Diagnostics {
	d := Diagnostics{At: now, TPS: t.actualTPS.Rate(), DroppedTicks: t.droppedTicks, Timings: perf.Clone(t.performance.Snapshot(now)), Process: t.processSampler.Sample(now)}
	for _, id := range t.sceneIDs() {
		s := t.views[id]
		d.Rooms = append(d.Rooms, RoomDiagnostics{ID: id, State: s.State, TPS: s.ActualTPS, DroppedTicks: s.DroppedTicks, Enemies: len(s.Enemies), Projectiles: len(s.Projectiles), Timings: perf.Clone(s.Performance)})
	}
	return d
}
