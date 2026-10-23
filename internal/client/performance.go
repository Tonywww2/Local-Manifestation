package client

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
)

type performanceSample struct {
	Render           renderWork               `json:"render"`
	Client           perf.Snapshot            `json:"client"`
	Server           perf.Snapshot            `json:"server"`
	Scene            perf.Snapshot            `json:"scene"`
	Rooms            map[string]perf.Snapshot `json:"rooms,omitempty"`
	Network          perf.Snapshot            `json:"network"`
	Process          perf.Process             `json:"process"`
	ServerProcess    perf.Process             `json:"server_process"`
	RTTMS            *float64                 `json:"rtt_ms,omitempty"`
	Enemies          int                      `json:"enemies"`
	Projectiles      int                      `json:"projectiles"`
	EnemyProjectiles int                      `json:"enemy_projectiles"`
	FrameSpikes      uint64                   `json:"frame_spikes"`
	SceneState       string                   `json:"scene_state"`
	LoadedRooms      int                      `json:"loaded_rooms"`
	Seconds          float64                  `json:"seconds"`
	FPS              float64                  `json:"fps"`
	ClientTPS        float64                  `json:"client_tps"`
	ServerTPS        float64                  `json:"server_tps"`
	SceneTPS         float64                  `json:"scene_tps"`
	SnapshotHz       float64                  `json:"snapshot_hz"`
	InputHz          float64                  `json:"input_hz"`
	DroppedTicks     uint64                   `json:"dropped_ticks"`
}
type performanceReport struct {
	TimingNote         string              `json:"timing_note"`
	FrameLimit         int                 `json:"frame_limit"`
	VSync              bool                `json:"vsync"`
	InputRate          int                 `json:"input_rate"`
	InterpolationMS    int64               `json:"interpolation_ms"`
	RenderedFrames     int                 `json:"rendered_frames"`
	InterpolatedFrames int                 `json:"interpolated_frames"`
	Samples            []performanceSample `json:"samples"`
}

func (g *Game) receiveState(s *protocol.State, at time.Time) {
	defer g.diagnostics.Since("client.logic.snapshot", perf.Start())
	if s == nil {
		return
	}
	old := g.self()
	newLayer := g.state == nil || g.state.Floor != s.Floor || g.state.RunID != s.RunID
	previousVote := ""
	if g.state != nil && g.state.Vote != nil {
		previousVote = g.state.Vote.Node
	}
	if newLayer {
		g.cameraControls.MapOffset = model.Vec{}
		g.cameraControls.MiniOffset = model.Vec{}
	}
	previousTrade, _, hadTrade := g.currentTrade()
	g.state = s
	if s.Vote == nil {
		g.closeWindow("vote")
	} else if _, ok := g.mapNode(s.Vote.Node); ok {
		if previousVote != s.Vote.Node && (g.state.Vote.Item != "" || g.nodesVoteKind() != "boss") {
			g.openWindow("vote")
		}
	} else {
		g.closeWindow("vote")
	}
	if p := g.self(); p != nil && g.u.mouseInventory != nil && p.Inventory.Revision >= g.u.mouseInventory.Revision {
		g.u.mouseInventory = nil
	}
	if trade, side, ok := g.currentTrade(); ok && (!hadTrade || previousTrade.ID != trade.ID) {
		g.u.tradeItems = append([]string(nil), trade.Offers[side]...)
		g.openWindow("trade")
	}
	g.settleProgression()
	changedRoom := old != nil && g.self() != nil && (old.Node != g.self().Node || old.PositionEpoch != g.self().PositionEpoch)
	if p := g.self(); p != nil && (newLayer || old == nil || old.Node != p.Node) {
		g.closeWindow("map")
		g.u.movementMenu = false
		g.u.arrivalNode, g.u.arrivalUntil = p.Node, at.Add(3*time.Second)
	}
	if changedRoom {
		g.destination = nil
		g.mouseFrames = 0
		g.localInput = model.Input{}
		g.closeWindow("entrance")
		g.closeWindow("node-actions")
		g.u.entrance = ""
		g.worldHistory.Clear()
		if g.link != nil {
			g.link.ClearInput()
		}
	}
	if p := g.self(); p != nil {
		if room := g.currentScene(); room != nil {
			g.prediction = g.prediction.Reconcile(*p, g.sceneArena(*room), at)
		}
	}
	g.snapshotMeter.Add(1, at)
	g.worldHistory.Push(g.captureWorldFrame(), s.Floor, s.TickRate, at, s.SnapshotRate)
}
func (g *Game) observeFrame(now time.Time) {
	if g.performanceStart.IsZero() {
		g.performanceStart = now
		g.nextMetrics = now.Add(time.Second)
	}
	g.renderedFrames++
	g.frameMeter.Add(1, now)
	if now.Before(g.nextMetrics) {
		return
	}
	g.nextMetrics = now.Add(time.Second)
	s := performanceSample{Render: g.renderWork, LoadedRooms: len(g.roomSurfaces), Seconds: now.Sub(g.performanceStart).Seconds(), FPS: g.frameMeter.Rate(), ClientTPS: ebiten.ActualTPS(), SnapshotHz: g.snapshotMeter.Rate()}
	if g.state != nil {
		s.Server = g.state.Performance
		s.ServerProcess = g.state.Process
		s.ServerTPS = g.state.ActualTPS
		s.DroppedTicks = g.state.DroppedTicks
		s.Rooms = make(map[string]perf.Snapshot, len(g.state.Scenes))
		for _, room := range g.state.Scenes {
			if len(room.Performance.Stages) > 0 {
				s.Rooms[room.ID] = room.Performance
			}
		}
	}
	if sc := g.currentScene(); sc != nil {
		s.Scene = sc.Performance
		s.Enemies, s.Projectiles = len(sc.Enemies), len(sc.Projectiles)
		for _, p := range sc.Projectiles {
			if p.Kind == "enemy" {
				s.EnemyProjectiles++
			}
		}
		s.SceneState = sc.State
		s.SceneTPS = sc.ActualTPS
		s.DroppedTicks += sc.DroppedTicks
	}
	if g.link != nil {
		s.InputHz = g.link.InputHz()
		s.Network = g.link.diagnostics.Snapshot(now)
		if latency, ok := g.link.Latency(now); ok {
			v := float64(latency) / 1e6
			s.RTTMS = &v
		}
	}
	s.Client = g.diagnostics.Snapshot(now)
	s.Process = g.processSampler.Sample(now)
	s.FrameSpikes = g.frameSpikes
	g.performance = s
	g.performanceSamples = append(g.performanceSamples, s)
	if len(g.performanceSamples) > 120 {
		g.performanceSamples = g.performanceSamples[len(g.performanceSamples)-120:]
	}
	if g.cfg.Metrics != "" {
		start := perf.Start()
		if err := g.writeMetrics(g.cfg.Metrics); err != nil {
			fmt.Fprintf(os.Stderr, "save performance metrics: %v\n", err)
		}
		g.diagnostics.Since("client.diagnostics.export", start)
	}
}
func (g *Game) saveMetrics() {
	if g.cfg.Metrics == "" {
		return
	}
	if err := g.writeMetrics(g.cfg.Metrics); err != nil {
		fmt.Fprintf(os.Stderr, "save performance metrics: %v\n", err)
	}
}
func (g *Game) writeMetrics(path string) error {
	r := performanceReport{TimingNote: "Inclusive wall-clock host durations; do not add parents to children. Render timings cover CPU submission, not GPU completion. Render work is the last completed frame; vertex_index_bytes estimates host draw inputs, not measured driver upload traffic. P95 is a histogram upper bound. GC pause is an estimated process-wide total. RTT is monotonic client/server round trip.", FrameLimit: g.frameLimit, VSync: g.vsync, InputRate: g.cfg.InputRate, InterpolationMS: g.cfg.InterpolationDelay.Milliseconds(), RenderedFrames: g.renderedFrames, InterpolatedFrames: g.interpolatedFrames, Samples: g.performanceSamples}
	b, err := json.MarshalIndent(r, "", "  ")
	if err == nil {
		err = os.WriteFile(path, b, 0600)
	}
	return err
}
