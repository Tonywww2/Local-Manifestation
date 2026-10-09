package localization_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"hsmultiplayer/locales"
	"hsmultiplayer/pkg/localization"
)

func TestBundledLanguagesAndNamedParameters(t *testing.T) {
	c, err := localization.Load(locales.Files)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"en", "zh-CN"} {
		if !c.HasLanguage(id) {
			t.Fatalf("missing language %s", id)
		}
	}
	for _, key := range []string{"ui.floor", "error.coins", "ui.damage.stat"} {
		p := map[string]string{}
		value, _ := c.Lookup("en", key)
		for _, name := range localization.Parameters(value) {
			p[name] = "25.00"
		}
		zh, en := c.Text("zh-CN", key, p), c.Text("en", key, p)
		if zh == key || en == key || strings.Contains(zh+en, "{") {
			t.Fatalf("bad translation %s: %s / %s", key, zh, en)
		}
	}
	if got := c.Text("unknown", "error.coins", map[string]string{"cost": "42"}); got != "You need 42 coins." {
		t.Fatal(got)
	}
	if got := c.Text("zh-CN", "extension.unknown", nil); got != "extension.unknown" {
		t.Fatal(got)
	}
	// Values are opaque, even if a player name itself contains a placeholder.
	if got := c.Text("en", "ui.location.battle", map[string]string{"id": "{another}"}); got != "{another} [battle]" {
		t.Fatal(got)
	}
}

func TestDefaultLanguagePacksHaveMatchingKeys(t *testing.T) {
	var packs [2]localization.Pack
	for i, file := range []string{"en.json", "zh-CN.json"} {
		b, err := locales.Files.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &packs[i]); err != nil {
			t.Fatal(err)
		}
	}
	if len(packs[0].Messages) != len(packs[1].Messages) {
		t.Fatal("unequal translation counts")
	}
	for key := range packs[0].Messages {
		if _, ok := packs[1].Messages[key]; !ok {
			t.Fatalf("Chinese key missing: %s", key)
		}
	}
}

func TestExternalPacksOverlayAndFallback(t *testing.T) {
	source := fstest.MapFS{"override.json": {Data: []byte(`{"_comment":"ordinary JSON","id":"zh-CN","name":"简体中文","messages":{"ui.start":"出发"}}`)}, "de.json": {Data: []byte(`{"id":"de","name":"Deutsch","messages":{"ui.start":"Starten"}}`)}}
	c, err := localization.Load(locales.Files, source)
	if err != nil {
		t.Fatal(err)
	}
	if c.Text("zh-CN", "ui.start", nil) != "出发" || c.Text("de", "ui.start", nil) != "Starten" {
		t.Fatal("overlay ignored")
	}
	if c.Text("de", "ui.menu.name", nil) != "Player name" {
		t.Fatal("missing English fallback")
	}
}

func TestMalformedPacksAreRejected(t *testing.T) {
	for _, bad := range []string{`{bad}`, `{"id":"../bad","name":"Bad","messages":{"key":"value"}}`, `{"id":"en","name":"Empty","messages":{}}`, `{"id":"zh-CN","name":"Bad","messages":{"error.coins":"需要 {different} 金币"}}`} {
		t.Run(fmt.Sprint(len(bad)), func(t *testing.T) {
			if _, err := localization.Load(locales.Files, fstest.MapFS{"bad.json": {Data: []byte(bad)}}); err == nil {
				t.Fatal("accepted invalid pack")
			}
		})
	}
	if _, err := localization.Load(locales.Files, fstest.MapFS{"bad.json": {Data: []byte{0xff, 0xfe}}}); err == nil {
		t.Fatal("accepted non-UTF-8 pack")
	}
	if _, err := localization.Load(locales.Files, fstest.MapFS{
		"en.json": {Data: []byte(`{"id":"en","name":"English","messages":{"error.coins":"You need {wrong}."}}`)},
		"zh.json": {Data: []byte(`{"id":"zh-CN","name":"中文","messages":{"error.coins":"需要 {wrong}。"}}`)},
	}); err == nil {
		t.Fatal("overrides changed parameters used by the caller")
	}
}
