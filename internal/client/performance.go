package client

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

type performanceSample struct {
	SceneState   string  `json:"scene_state"`
	LoadedRooms  int     `json:"loaded_rooms"`
	Seconds      float64 `json:"seconds"`
	FPS          float64 `json:"fps"`
	ClientTPS    float64 `json:"client_tps"`
	ServerTPS    float64 `json:"server_tps"`
	SceneTPS     float64 `json:"scene_tps"`
	SnapshotHz   float64 `json:"snapshot_hz"`
	InputHz      float64 `json:"input_hz"`
	DroppedTicks uint64  `json:"dropped_ticks"`
}
type performanceReport struct {
	FrameLimit         int                 `json:"frame_limit"`
	VSync              bool                `json:"vsync"`
	InputRate          int                 `json:"input_rate"`
	InterpolationMS    int64               `json:"interpolation_ms"`
	RenderedFrames     int                 `json:"rendered_frames"`
	InterpolatedFrames int                 `json:"interpolated_frames"`
	Samples            []performanceSample `json:"samples"`
}

func (g *Game) receiveState(s *protocol.State, at time.Time) {
	if s == nil {
		return
	}
	old := g.self()
	previousTrade, _, hadTrade := g.currentTrade()
	from := g.currentNode()
	g.state = s
	if p := g.self(); p != nil && g.u.mouseInventory != nil && p.Inventory.Revision >= g.u.mouseInventory.Revision {
		g.u.mouseInventory = nil
	}
	if trade, side, ok := g.currentTrade(); ok && (!hadTrade || previousTrade.ID != trade.ID) {
		g.u.tradeItems = append([]string(nil), trade.Offers[side]...)
		g.openWindow("trade")
	}
	g.settleProgression()
	continuous := false
	if p := g.self(); p != nil {
		to := g.currentNode()
		_, adjacent := geometry.DoorSide(from, to)
		continuous = old != nil && old.Node != p.Node && old.PositionEpoch == p.PositionEpoch && old.Scene == "" && p.Scene == "" && adjacent && expedition.FreeTransit(to)
		if continuous {
			aw, ah := g.catalog.SpaceSize(from)
			bw, bh := g.catalog.SpaceSize(to)
			a, b := geometry.NewArena(aw, ah), geometry.NewArena(bw, bh)
			g.prediction = g.prediction.Rebase(from, to, a, b)
			if g.destination != nil {
				position := geometry.TransferPoint(from, to, a, b, *g.destination)
				g.destination = &position
			}
			g.localInput.Aim = geometry.TransferPoint(from, to, a, b, g.localInput.Aim)
		}
		if room := g.currentScene(); room != nil {
			g.prediction = g.prediction.Reconcile(*p, geometry.NewArena(room.Width, room.Height), at, g.walkingSides()...)
		}
	}
	if next := g.self(); !continuous && old != nil && next != nil && (old.Node != next.Node || old.PositionEpoch != next.PositionEpoch) {
		g.destination = nil
		g.mouseFrames = 0
		g.localInput = model.Input{}
		g.closeWindow("entrance")
		g.u.entrance = ""
		if g.link != nil {
			g.link.ClearInput()
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
	s := performanceSample{LoadedRooms: len(g.roomSurfaces), Seconds: now.Sub(g.performanceStart).Seconds(), FPS: g.frameMeter.Rate(), ClientTPS: ebiten.ActualTPS(), SnapshotHz: g.snapshotMeter.Rate()}
	if g.state != nil {
		s.ServerTPS = g.state.ActualTPS
		s.DroppedTicks = g.state.DroppedTicks
	}
	if sc := g.currentScene(); sc != nil {
		s.SceneState = sc.State
		s.SceneTPS = sc.ActualTPS
		s.DroppedTicks += sc.DroppedTicks
	}
	if g.link != nil {
		s.InputHz = g.link.InputHz()
	}
	g.performance = s
	if g.cfg.Metrics != "" {
		g.performanceSamples = append(g.performanceSamples, s)
		if len(g.performanceSamples) > 120 {
			g.performanceSamples = g.performanceSamples[len(g.performanceSamples)-120:]
		}
	}
}
func (g *Game) saveMetrics() {
	if g.cfg.Metrics == "" {
		return
	}
	r := performanceReport{g.frameLimit, g.vsync, g.cfg.InputRate, g.cfg.InterpolationDelay.Milliseconds(), g.renderedFrames, g.interpolatedFrames, g.performanceSamples}
	b, err := json.MarshalIndent(r, "", "  ")
	if err == nil {
		err = os.WriteFile(g.cfg.Metrics, b, 0600)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "save performance metrics: %v\n", err)
	}
}
