package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/go-text/typesetting/font"

	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/locales"
	"hsmultiplayer/pkg/localization"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func TestEmbeddedFontCoversDefaultTranslations(t *testing.T) {
	face := fontSource.UnsafeInternal().(*font.Face)
	for _, file := range []string{"zh-CN.json", "en.json"} {
		b, err := locales.Files.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var pack localization.Pack
		if err := json.Unmarshal(b, &pack); err != nil {
			t.Fatal(err)
		}
		for key, value := range pack.Messages {
			for _, r := range value {
				if unicode.IsControl(r) {
					continue
				}
				if _, ok := face.NominalGlyph(r); !ok {
					t.Errorf("font missing %q in %s / %s", r, file, key)
				}
			}
		}
	}
}
func TestContentNodeNamesAndConfiguredPrices(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	g := New(c, Config{Language: "zh-CN", Profile: filepath.Join(t.TempDir(), "p.json")})
	n := c.Node("base:combat-a")
	g.state = &protocol.State{Nodes: []model.Node{{ID: "a", Name: n.Name, NameKey: n.NameKey}}}
	if g.nodeName("a") != "战斗 A" || g.tr("ui.payment.personal", "cost", 3) != " [个 3]" {
		t.Fatal("Chinese content translation failed")
	}
	g.language = "en"
	if g.nodeName("a") != "Battle A" || g.tr("ui.payment.shared", "cost", 5) != " [S 5]" {
		t.Fatal("English content translation failed")
	}
	g.state.Nodes[0].NameKey = "unknown"
	g.state.Nodes[0].Name = "External room"
	if g.nodeName("a") != "External room" {
		t.Fatal("missing external content fallback")
	}
}

func TestLanguageSwitchPreservesIdentityAndGameplay(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(t.TempDir(), "player.json")
	identity := []byte(`{"127.0.0.1:7777":"existing-token"}`)
	if err := os.WriteFile(profile, identity, 0600); err != nil {
		t.Fatal(err)
	}
	g := New(c, Config{Profile: profile, Language: "zh-CN"})
	g.notifyMessage(protocol.ErrorMessage(protocol.Reject("error.coins", "cost", "40")))
	if !strings.Contains(g.translations.Text(g.language, g.noticeCode, g.noticeParams), "40") {
		t.Fatal("lost error parameter")
	}
	g.switchLanguage()
	if g.language != "en" || g.tr("ui.start") != "Start layer" {
		t.Fatal("language did not change")
	}
	if got := g.translations.Text(g.language, g.noticeCode, g.noticeParams); got != "You need 40 coins." {
		t.Fatal(got)
	}
	if c.Digest != g.catalog.Digest || g.tokens["127.0.0.1:7777"] != "existing-token" {
		t.Fatal("language changed gameplay identity")
	}
	after, err := os.ReadFile(profile)
	if err != nil || string(after) != string(identity) {
		t.Fatal("identity file was changed")
	}
	restored := New(c, Config{Profile: profile})
	if restored.language != "en" {
		t.Fatal("language setting not restored")
	}
	if explicit := New(c, Config{Profile: profile, Language: "zh-CN"}); explicit.language != "zh-CN" {
		t.Fatal("flag did not override preference")
	}
}
