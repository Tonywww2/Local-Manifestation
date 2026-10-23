package client

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/lobby"
)

type settings struct {
	ShowFPS         bool          `json:"show_fps"`
	ShowLatency     bool          `json:"show_latency"`
	ShowDiagnostics bool          `json:"show_diagnostics"`
	ClickDirect     bool          `json:"click_direct"`
	Version         int           `json:"version,omitempty"`
	FrameLimit      *int          `json:"frame_limit,omitempty"`
	VSync           *bool         `json:"vsync,omitempty"`
	Language        string        `json:"language"`
	InputMode       string        `json:"input_mode,omitempty"`
	Layout          ui.Layout     `json:"layout,omitempty"`
	WindowSizes     windowSizes   `json:"window_sizes,omitempty"`
	Bindings        bindings      `json:"bindings,omitempty"`
	Room            *lobby.Config `json:"room,omitempty"`
	PlayerName      string        `json:"player_name,omitempty"`
	Chassis         string        `json:"chassis,omitempty"`
}

func (g *Game) preferences() settings {
	fps, vsync := g.frameLimit, g.vsync
	room := lobby.CloneConfig(g.front.Config)
	return settings{ShowFPS: g.showFPS, ShowLatency: g.showLatency, ShowDiagnostics: g.performanceVisible, ClickDirect: g.clickDirect, Version: 1, FrameLimit: &fps, VSync: &vsync, Language: g.language, InputMode: g.inputMode(), Layout: ui.Clone(g.u.layout), WindowSizes: maps.Clone(g.u.windowSizes), Bindings: cloneBindings(g.u.bindings), Room: &room, PlayerName: g.name, Chassis: g.catalog.ChassisList()[g.chassis].ID}
}
func validateSettings(s settings) error {
	if len([]rune(s.PlayerName)) > 24 {
		return fmt.Errorf("invalid player name")
	}
	if s.Room != nil {
		if err := s.Room.Validate(); err != nil {
			return err
		}
	}
	if s.Version != 1 || s.FrameLimit == nil || *s.FrameLimit < 0 || *s.FrameLimit > 1000 || s.VSync == nil || s.Language == "" {
		return fmt.Errorf("invalid client preferences")
	}
	if s.InputMode != "mouse" && s.InputMode != "keyboard" {
		return fmt.Errorf("invalid input mode")
	}
	if err := ui.Validate(s.Layout); err != nil {
		return err
	}
	if err := validateWindowSizes(s.WindowSizes); err != nil {
		return err
	}
	return validateBindings(s.Bindings)
}

// Same-directory replacement preserves the old file on validation/write failure.
func writeSettings(path string, s settings) error {
	if err := validateSettings(s); err != nil {
		return err
	}
	values := map[string]json.RawMessage{}
	if old, err := os.ReadFile(path); err == nil {
		if err = json.Unmarshal(old, &values); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var known map[string]json.RawMessage
	if values == nil {
		values = map[string]json.RawMessage{}
	}
	if err = json.Unmarshal(b, &known); err != nil {
		return err
	}
	for k, v := range known {
		values[k] = v
	}
	// Empty layouts must replace the previous custom layout as well.
	if len(s.Layout) == 0 {
		delete(values, "layout")
	}
	if len(s.WindowSizes) == 0 {
		delete(values, "window_sizes")
	}
	b, err = json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
func (g *Game) commitPreferences(s settings) error {
	if !g.translations.HasLanguage(s.Language) {
		return fmt.Errorf("unknown language")
	}
	if err := writeSettings(settingsPath(g.cfg.Profile), s); err != nil {
		return err
	}
	g.showFPS, g.showLatency, g.performanceVisible = s.ShowFPS, s.ShowLatency, s.ShowDiagnostics
	g.language = s.Language
	g.frameLimit = *s.FrameLimit
	g.vsync = *s.VSync
	g.mouseMode = s.InputMode == "mouse"
	g.clickDirect = s.ClickDirect
	g.u.layout = ui.Clone(s.Layout)
	g.u.windowSizes = maps.Clone(s.WindowSizes)
	g.u.bindings = cloneBindings(s.Bindings)
	if s.Room != nil {
		g.front.Config = lobby.CloneConfig(*s.Room)
	}
	if s.PlayerName != "" {
		g.name = s.PlayerName
	}
	for i, ch := range g.catalog.ChassisList() {
		if ch.ID == s.Chassis {
			g.chassis = i
			break
		}
	}
	g.pacer.Set(g.frameLimit)
	applyDisplayPreferences(g)
	return nil
}
func (g *Game) saveSettings() error {
	return writeSettings(settingsPath(g.cfg.Profile), g.preferences())
}
