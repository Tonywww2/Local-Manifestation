package client

import (
	"image/color"
	"slices"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/model"
)

type textSpan struct{ Text, Role string }
type textTerm struct{ Text, Role string }

// Semantic colors are values shared by every UI, formula and Hover draw.
// Attribute names remain readable labels; color never replaces the name.
func semanticColor(role string, fallback color.Color) color.Color {
	switch role {
	case "number":
		return color.RGBA{247, 213, 132, 255}
	case "strength":
		return color.RGBA{239, 141, 129, 255}
	case "agility":
		return color.RGBA{123, 211, 163, 255}
	case "constitution":
		return color.RGBA{229, 183, 119, 255}
	case "intelligence":
		return color.RGBA{124, 187, 241, 255}
	case "sense":
		return color.RGBA{225, 156, 209, 255}
	case "mana":
		return color.RGBA{180, 159, 245, 255}
	case "attribute":
		return color.RGBA{126, 206, 244, 255}
	case "health":
		return color.RGBA{247, 147, 160, 255}
	case "shield":
		return color.RGBA{111, 219, 223, 255}
	case "resource":
		return color.RGBA{199, 161, 244, 255}
	case "physical":
		return color.RGBA{228, 197, 163, 255}
	case "fire":
		return color.RGBA{255, 154, 107, 255}
	case "cold":
		return color.RGBA{123, 208, 255, 255}
	case "electromagnetic":
		return color.RGBA{244, 223, 116, 255}
	case "chemical":
		return color.RGBA{150, 219, 141, 255}
	case "magic":
		return color.RGBA{201, 166, 255, 255}
	default:
		return fallback
	}
}

func (g *Game) semanticTerms() []textTerm {
	var terms []textTerm
	seen := map[string]bool{}
	for _, lang := range append([]string{g.language}, g.translations.Languages()...) {
		for _, key := range model.AttributeKeys() {
			value := g.translations.Text(lang, "attribute."+key, nil)
			if !seen[value] {
				terms = append(terms, textTerm{value, key})
				seen[value] = true
			}
		}
	}
	for _, key := range []string{"armor", "speed", "attack_speed"} {
		terms = append(terms, textTerm{g.tr("attribute." + key), "attribute"})
	}
	for _, key := range []string{"physical", "fire", "cold", "electromagnetic", "chemical", "magic"} {
		terms = append(terms, textTerm{g.tr("damage." + key), key})
	}
	for _, key := range []string{"health", "shield", "resource"} {
		terms = append(terms, textTerm{g.tr("ui.keyword." + key), key})
	}
	slices.SortStableFunc(terms, func(a, b textTerm) int { return len([]rune(b.Text)) - len([]rune(a.Text)) })
	return terms
}

func wordRune(r rune) bool { return unicode.IsLetter(r) && r < 128 }

// Pure localized tokenization. Longest terms win, with English word boundaries.
func styleText(line string, terms []textTerm) []textSpan {
	runes := []rune(line)
	var spans []textSpan
	appendSpan := func(value, role string) {
		if len(spans) > 0 && spans[len(spans)-1].Role == role {
			spans[len(spans)-1].Text += value
		} else {
			spans = append(spans, textSpan{value, role})
		}
	}
	for i := 0; i < len(runes); {
		matched := false
		for _, term := range terms {
			t := []rune(term.Text)
			if len(t) == 0 || i+len(t) > len(runes) || !strings.EqualFold(string(runes[i:i+len(t)]), term.Text) {
				continue
			}
			if wordRune(t[0]) && i > 0 && wordRune(runes[i-1]) || wordRune(t[len(t)-1]) && i+len(t) < len(runes) && wordRune(runes[i+len(t)]) {
				continue
			}
			appendSpan(string(runes[i:i+len(t)]), term.Role)
			i += len(t)
			matched = true
			break
		}
		if matched {
			continue
		}
		if unicode.IsDigit(runes[i]) {
			j := i + 1
			for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '.' || runes[j] == '%') {
				j++
			}
			appendSpan(string(runes[i:j]), "number")
			i = j
		} else {
			appendSpan(string(runes[i]), "text")
			i++
		}
	}
	return spans
}

// Cache ownership stays on the client; parsing does not repeat every render tick.
func (g *Game) ensureTextTerms() {
	if g.u.textTerms == nil || g.u.textLanguage != g.language {
		g.u.textTerms = g.semanticTerms()
		g.u.textLanguage = g.language
		g.u.textStyles = nil
	}
}

func (g *Game) styledText(value string) []textSpan {
	g.ensureTextTerms()
	if spans, ok := g.u.textStyles[value]; ok {
		return spans
	}
	if g.u.textStyles == nil || len(g.u.textStyles) >= 2048 {
		g.u.textStyles = map[string][]textSpan{}
	}
	spans := styleText(value, g.u.textTerms)
	g.u.textStyles[value] = spans
	return spans
}
func (g *Game) styledTooltipRows(rows []string) [][]textSpan {
	out := make([][]textSpan, len(rows))
	for i, row := range rows {
		out[i] = g.styledText(row)
	}
	return out
}
func (g *Game) drawColoredText(dst *ebiten.Image, value string, x, y, size float64, base color.Color) {
	// Split before styling so line placement agrees with the plain-text renderer.
	for i, row := range strings.Split(value, "\n") {
		dx := x
		for _, span := range g.styledText(row) {
			drawTextSpan(dst, span.Text, dx, y+float64(i)*18, size, semanticColor(span.Role, base))
			dx += measuredText(span.Text, size)
		}
	}
}

// Prefer a boundary before a semantic term, preserving its color when wrapped.
// If a whole term is wider than the line, retain the original split to progress.
func termBoundary(runes []rune, split int, terms []textTerm) int {
	if split <= 0 || split >= len(runes) {
		return split
	}
	for _, term := range terms {
		t := []rune(term.Text)
		for start := max(0, split-len(t)+1); start < split; start++ {
			end := start + len(t)
			if len(t) == 0 || end > len(runes) || !strings.EqualFold(string(runes[start:end]), term.Text) {
				continue
			}
			if wordRune(t[0]) && start > 0 && wordRune(runes[start-1]) || wordRune(t[len(t)-1]) && end < len(runes) && wordRune(runes[end]) {
				continue
			}
			if start > 0 {
				return start
			}
		}
	}
	return split
}
