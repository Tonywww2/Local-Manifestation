package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
)

func TestRenderingSettingsPreserveLanguageAndCLIOverride(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(t.TempDir(), "identity.json")
	identity := []byte(`{"server":"token"}`)
	if err = os.WriteFile(profile, identity, 0600); err != nil {
		t.Fatal(err)
	}
	g := New(c, Config{Profile: profile, Language: "en", FrameLimit: 144, FPSOverride: true, VSync: false, VSyncOverride: true})
	if err = g.saveSettings(); err != nil {
		t.Fatal(err)
	}
	loaded := New(c, Config{Profile: profile})
	if loaded.frameLimit != 144 || loaded.vsync || loaded.language != "en" {
		t.Fatal("settings not restored")
	}
	override := New(c, Config{Profile: profile, FrameLimit: 0, FPSOverride: true, VSync: true, VSyncOverride: true})
	if override.frameLimit != 0 || !override.vsync || override.language != "en" {
		t.Fatal("CLI override lost")
	}
	if b, _ := os.ReadFile(profile); string(b) != string(identity) {
		t.Fatal("identity overwritten")
	}
}
func TestInvalidTimingRejected(t *testing.T) {
	for _, v := range []struct {
		fps, rate int
		delay     time.Duration
	}{{-1, 30, 0}, {60, 40, 0}, {60, 30, -time.Second}, {60, 30, 3 * time.Second}} {
		if ValidateTiming(v.fps, v.rate, v.delay) == nil {
			t.Fatal("accepted invalid timing", v)
		}
	}
}
