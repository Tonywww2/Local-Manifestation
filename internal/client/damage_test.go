package client

import (
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/damage"
	"path/filepath"
	"strings"
	"testing"
)

func TestDamageExplanationLocalizesSourcesAndKeepsAllRows(t *testing.T) {
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	p, err := content.Player(c, "p", "Alice", "base:scout")
	if err != nil {
		t.Fatal(err)
	}
	p, err = c.GrantExperience(p, 20)
	if err != nil {
		t.Fatal(err)
	}
	p.WeaponBonus = 3
	q, err := content.Attack(c, p)
	if err != nil {
		t.Fatal(err)
	}
	r, err := damage.Evaluate(q)
	if err != nil {
		t.Fatal(err)
	}
	g := New(c, Config{Language: "zh-CN", Profile: filepath.Join(t.TempDir(), "p.json")})
	for _, lang := range []string{"zh-CN", "en"} {
		g.language = lang
		lines := g.damageLines(r)
		joined := strings.Join(lines, "\n")
		if len(lines) <= 13 || !strings.Contains(joined, "engine:level:strength") || !strings.Contains(joined, "+1.00") {
			t.Fatal("source rows omitted", lang, joined)
		}
		want := "Weapon upgrade"
		if lang == "zh-CN" {
			want = "武器强化"
		}
		if !strings.Contains(joined, want) {
			t.Fatal("untranslated upgrade source", joined)
		}
	}
}
