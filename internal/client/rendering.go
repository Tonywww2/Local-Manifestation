package client

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/timing"
)

func ValidateTiming(fps, inputRate int, delay time.Duration) error {
	if fps < 0 || fps > 1000 {
		return fmt.Errorf("FPS limit must be 0 (uncapped) or 1..1000")
	}
	if !timing.LogicRate(inputRate) {
		return fmt.Errorf("input rate must be 30 or 60")
	}
	if delay < 0 || delay > 2*time.Second {
		return fmt.Errorf("interpolation delay must be 0..2000 ms")
	}
	return nil
}
func (g *Game) initializeRendering() {
	g.frameLimit = 60
	g.vsync = true
	if b, err := os.ReadFile(settingsPath(g.cfg.Profile)); err == nil {
		var saved settings
		if json.Unmarshal(b, &saved) == nil {
			if saved.FrameLimit != nil && *saved.FrameLimit >= 0 && *saved.FrameLimit <= 1000 {
				g.frameLimit = *saved.FrameLimit
			}
			if saved.VSync != nil {
				g.vsync = *saved.VSync
			}
		}
	}
	if g.cfg.FPSOverride {
		g.frameLimit = g.cfg.FrameLimit
	}
	if g.cfg.VSyncOverride {
		g.vsync = g.cfg.VSync
	}
	if g.cfg.InputRate == 0 {
		g.cfg.InputRate = timing.DefaultLogicRate
	}
	g.pacer.Set(g.frameLimit)
	ebiten.SetVsyncEnabled(g.vsync)
	if b, err := os.ReadFile(settingsPath(g.cfg.Profile)); err == nil {
		var saved settings
		if json.Unmarshal(b, &saved) == nil {
			g.showFPS, g.showLatency, g.performanceVisible = saved.ShowFPS, saved.ShowLatency, saved.ShowDiagnostics
		}
	}
	g.performanceVisible = g.performanceVisible || g.cfg.ShowPerformance
}

func (g *Game) cycleFPS() {
	values := []int{30, 60, 120, 144, 240, 0}
	next := 60
	for i, v := range values {
		if v == g.frameLimit {
			next = values[(i+1)%len(values)]
			break
		}
	}
	s := g.preferences()
	s.FrameLimit = &next
	if err := g.commitPreferences(s); err != nil {
		g.notifyError(err)
	}
}
func (g *Game) toggleVSync() {
	s := g.preferences()
	v := !g.vsync
	s.VSync = &v
	if err := g.commitPreferences(s); err != nil {
		g.notifyError(err)
	}
}
func applyDisplayPreferences(g *Game) {
	ebiten.SetVsyncEnabled(g.vsync)
	ebiten.SetWindowTitle(g.tr("ui.window"))
}
func (g *Game) frameLimitLabel() string {
	if g.frameLimit == 0 {
		return g.tr("ui.fps.unlimited")
	}
	return fmt.Sprint(g.frameLimit)
}
