package client

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/perf"
)

func (g *Game) drawPerformanceHUD(dst *ebiten.Image) {
	var lines []string
	if g.showFPS || g.performanceVisible {
		lines = append(lines, fmt.Sprintf("FPS %.0f", g.performance.FPS))
	}
	if g.showLatency || g.performanceVisible {
		latency := "--"
		if g.link != nil {
			if rtt, ok := g.link.Latency(time.Now()); ok {
				latency = fmt.Sprintf("%.1f ms", float64(rtt)/1e6)
			}
		}
		lines = append(lines, g.tr("ui.performance.latency", "value", latency))
	}
	if len(lines) == 0 {
		return
	}
	text := strings.Join(lines, "  ·  ")
	w := measuredText(text, 13) + 20
	r := ui.At(g.effectiveLayout(g.u.layout), "performance-hud", ui.Placement{Anchor: "bottom_left", X: 18, Y: -68}, ui.Size{W: max(246, w), H: 26}, g.uiBounds())
	g.drawPerformanceText(dst, r, text)
}

func (g *Game) drawPerformanceText(dst *ebiten.Image, r ui.Rect, text string) {
	rect(dst, r.X, r.Y, r.W, r.H, color.RGBA{27, 36, 39, 220})
	g.fitText(dst, text, r.X+10, r.Y+3, r.W-20, 13, false)
}

func (g *Game) buildPerformanceWindow(f *uiFrame, layout ui.Layout, id string) {
	s := g.performance
	roomID, room, nextRoom := g.performanceRoomView()
	sceneTPS, enemies, projectiles := s.SceneTPS, s.Enemies, s.Projectiles
	if g.state != nil {
		for _, sc := range g.state.Scenes {
			if sc.ID == roomID {
				sceneTPS, enemies, projectiles = sc.ActualTPS, len(sc.Enemies), len(sc.Projectiles)
				break
			}
		}
	}
	r := g.windowPanel(f, layout, id, g.tr("ui.performance.open"), ui.Size{W: 1080, H: min(650, float64(g.height)-16)})
	left := make([]perf.Stage, 0, len(s.Client.Stages)+len(s.Network.Stages))
	var interval, wait float64
	for _, stage := range s.Client.Stages {
		switch stage.Name {
		case "client.frame.interval":
			interval = stage.MeanMS
		case "client.render.pacing_wait":
			wait = stage.MeanMS
		default:
			left = append(left, stage)
		}
	}
	left = append(left, s.Network.Stages...)
	sort.Slice(left, func(i, j int) bool { return left[i].TotalMS > left[j].TotalMS })
	columns := []struct {
		title  string
		stages []perf.Stage
	}{
		{g.tr("ui.performance.client"), left},
		{g.tr("ui.performance.server"), s.Server.Stages},
		{g.tr("ui.performance.scene") + " · " + roomID, room.Stages},
	}
	rows := max(len(left), len(s.Server.Stages), len(room.Stages))
	height := max(r.H, 278+float64(rows)*64)
	if f.Viewports == nil {
		f.Viewports = map[string]windowViewport{}
	}
	f.Viewports[id] = windowViewport{Bounds: r, VirtualHeight: height, Footer: 46}
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 40, W: r.W - 28, H: 22}, fmt.Sprintf("FPS %.1f   Client %.1f TPS   Server %.1f TPS   Scene %.1f TPS   ·   %s %d / %d", s.FPS, s.ClientTPS, s.ServerTPS, sceneTPS, g.tr("ui.performance.entities"), enemies, projectiles))
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 66, W: r.W - 28, H: 38}, g.tr("ui.performance.memory", "heap", fmt.Sprintf("%.1f", s.Process.HeapMB), "alloc", fmt.Sprintf("%.1f", s.Process.AllocatedMB), "gc", fmt.Sprintf("%.2f", s.Process.GCPauseMS), "count", s.Process.GCCount, "spikes", s.FrameSpikes))
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 108, W: r.W - 28, H: 22}, g.tr("ui.performance.wait", "interval", fmt.Sprintf("%.2f", interval), "wait", fmt.Sprintf("%.2f", wait)))
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 132, W: r.W - 28, H: 22}, g.tr("ui.performance.batch", "renderer", s.Render.ProjectileRenderer, "visible", s.Render.Visible, "culled", s.Render.Culled, "batches", s.Render.Batches, "bytes", fmt.Sprintf("%.1f", float64(s.Render.VertexIndexBytes)/1024), "sprites", s.Render.SpriteCalls))
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 156, W: r.W - 28, H: 22}, g.tr("ui.performance.columns"))
	w := (r.W - 40) / 3
	for col, group := range columns {
		x := r.X + 14 + float64(col)*(w+6)
		g.addLabel(f, id, ui.Rect{X: x, Y: r.Y + 186, W: w - 34, H: 22}, group.title)
		if col == 2 && nextRoom != "" {
			addControl(f, id, "performance:room", "right", "", "performance-room", nextRoom, g.tr("ui.performance.next_room"), ui.Rect{X: x + w - 30, Y: r.Y + 182, W: 28, H: 26})
		}
		for i, stage := range group.stages {
			y := r.Y + 214 + float64(i)*64
			name := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(stage.Name, "client."), "server."), "scene.")
			g.addLabel(f, id, ui.Rect{X: x, Y: y, W: w, H: 20}, name)
			g.addLabel(f, id, ui.Rect{X: x, Y: y + 20, W: w, H: 40}, fmt.Sprintf("Σ %.2f · μ %.3f ms · n %d\nP95 %.2f · max %.2f ms", stage.TotalMS, stage.MeanMS, stage.Calls, stage.P95MS, stage.MaxMS))
		}
	}
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + height - 36, W: r.W - 102, H: 32}, g.tr("ui.performance.inclusive"))
	addControl(f, id, "performance:export", "save", "", "performance-export", "", g.tr("ui.performance.export"), ui.Rect{X: r.X + r.W - 58, Y: r.Y + height - 38, W: 40, H: 30})
}

// Scene snapshots are read-only values published by their simulation owners.
func (g *Game) performanceRoomView() (string, perf.Snapshot, string) {
	if g.state == nil {
		return "--", perf.Snapshot{}, ""
	}
	ids := make([]string, 0, len(g.performance.Rooms))
	for id := range g.performance.Rooms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	selected := g.performanceRoom
	if selected == "" {
		if room := g.currentScene(); room != nil {
			selected = room.ID
		}
	}
	index := 0
	for i, id := range ids {
		if id == selected {
			index = i
			break
		}
	}
	if len(ids) == 0 {
		return "--", perf.Snapshot{}, ""
	}
	next := ""
	if len(ids) > 1 {
		next = ids[(index+1)%len(ids)]
	}
	return ids[index], g.performance.Rooms[ids[index]], next
}

func (g *Game) exportPerformance() {
	path := filepath.Join("data", "diagnostics", time.Now().Format("20060102-150405.000")+".json")
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err == nil {
		err = g.writeMetrics(path)
	}
	if err != nil {
		g.notifyError(err)
		return
	}
	g.notify("notice.performance_export", "path", path)
}
