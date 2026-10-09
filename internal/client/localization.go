package client

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/locales"
	"hsmultiplayer/pkg/localization"
	"hsmultiplayer/pkg/protocol"
)

var defaultTranslations = func() *localization.Catalog {
	c, err := localization.Load(locales.Files)
	if err != nil {
		panic(err)
	}
	return c
}()

func settingsPath(profile string) string {
	return strings.TrimSuffix(profile, ".json") + ".settings.json"
}

func (g *Game) initializeLanguage() {
	g.translations = g.cfg.Translations
	if g.translations == nil {
		g.translations = defaultTranslations
	}
	g.language = "zh-CN"
	if b, err := os.ReadFile(settingsPath(g.cfg.Profile)); err == nil {
		var saved settings
		if json.Unmarshal(b, &saved) == nil && g.translations.HasLanguage(saved.Language) {
			g.language = saved.Language
		}
	}
	if g.translations.HasLanguage(g.cfg.Language) {
		g.language = g.cfg.Language
	}
	ebiten.SetWindowTitle(g.tr("ui.window"))
}

func params(pairs ...any) map[string]string {
	if len(pairs)%2 != 0 {
		panic("translation needs named parameter pairs")
	}
	p := map[string]string{}
	for i := 0; i < len(pairs); i += 2 {
		p[fmt.Sprint(pairs[i])] = fmt.Sprint(pairs[i+1])
	}
	return p
}

func (g *Game) tr(key string, pairs ...any) string {
	return g.translations.Text(g.language, key, params(pairs...))
}

func (g *Game) localized(key, fallback string) string {
	if value, ok := g.translations.Lookup(g.language, key); ok {
		return value
	}
	return fallback
}

func (g *Game) contentName(id, fallback string) string {
	return g.localized("content."+id+".name", fallback)
}

func (g *Game) nodeName(id string) string {
	if id == "enhanced" || id == "exhaust" {
		return g.tr("node." + id)
	}
	for _, n := range g.state.Nodes {
		if n.ID == id {
			return g.localized(n.NameKey, n.Name)
		}
	}
	return id
}

func (g *Game) switchLanguage() {
	ids := g.translations.Languages()
	s := g.preferences()
	for i, id := range ids {
		if id == g.language {
			s.Language = ids[(i+1)%len(ids)]
			break
		}
	}
	if err := g.commitPreferences(s); err != nil {
		g.notify("notice.settings_save", "detail", err.Error())
	}
}

func (g *Game) notifyError(err error) { g.notifyMessage(protocol.ErrorMessage(err)) }

func (g *Game) notifyMessage(message protocol.Message) {
	if message.Code != "" {
		g.notify(message.Code)
		g.noticeParams = message.Params
		return
	}
	g.notify("error.detail", "detail", message.Text)
}

func (g *Game) damageSource(source string) string {
	if strings.HasSuffix(source, ":base") {
		return g.tr("damage.base")
	}
	if strings.HasPrefix(source, "reduction ") {
		return g.tr("damage.reduction", "index", strings.TrimPrefix(source, "reduction "))
	}
	key := map[string]string{"ordinary flat": "ordinary_flat", "final flat": "final_flat", "final damage": "final_damage", "armor": "armor", "resistance": "resistance", "minimum damage": "minimum", "critical": "critical", "immunity": "immunity"}[source]
	if key != "" {
		return g.tr("damage." + key)
	}
	// Extensions can localize their own multiplier/source IDs without changing evaluation.
	return g.localized("damage.source."+source, source)
}
