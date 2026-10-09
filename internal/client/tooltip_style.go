package client

import (
	"image/color"
	"slices"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
)

type tooltipSpan struct{ Text, Role string }
type tooltipTerm struct{ Text, Role string }
type hoverStyleKey struct{ Text, Language string }

// One semantic palette is shared by all tooltips and both languages. The text
// remains fully readable without color; no gameplay meaning depends on color alone.
var tooltipColors = map[string]color.RGBA{
	"text": {225, 232, 240, 255}, "number": {247, 213, 132, 255},
	"attribute": {126, 206, 244, 255}, "health": {247, 147, 160, 255},
	"shield": {111, 219, 223, 255}, "resource": {199, 161, 244, 255},
	"physical": {228, 197, 163, 255}, "fire": {255, 154, 107, 255},
	"cold": {123, 208, 255, 255}, "electromagnetic": {244, 223, 116, 255},
	"chemical": {150, 219, 141, 255}, "magic": {201, 166, 255, 255},
}

func (g *Game) tooltipTerms() []tooltipTerm {
	var terms []tooltipTerm
	for _, k := range []string{"strength", "agility", "constitution", "intelligence", "sense", "mana", "armor", "speed", "attack_speed"} {
		terms = append(terms, tooltipTerm{g.tr("attribute." + k), "attribute"})
	}
	for _, k := range []string{"physical", "fire", "cold", "electromagnetic", "chemical", "magic"} {
		terms = append(terms, tooltipTerm{g.tr("damage." + k), k})
	}
	for _, k := range []string{"health", "shield", "resource"} {
		terms = append(terms, tooltipTerm{g.tr("ui.keyword." + k), k})
	}
	slices.SortFunc(terms, func(a, b tooltipTerm) int { return len(b.Text) - len(a.Text) })
	return terms
}
func wordRune(r rune) bool { return unicode.IsLetter(r) && r < 128 }

// Pure tokenization of localized text: numbers and known semantic terms only.
// Longest terms win; English substrings inside other words are never highlighted.
func styleTooltip(line string, terms []tooltipTerm) []tooltipSpan {
	runes := []rune(line)
	var spans []tooltipSpan
	appendSpan := func(value, role string) {
		if len(spans) > 0 && spans[len(spans)-1].Role == role {
			spans[len(spans)-1].Text += value
		} else {
			spans = append(spans, tooltipSpan{value, role})
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
func (g *Game) styledTooltipRows(rows []string) [][]tooltipSpan {
	key := hoverStyleKey{strings.Join(rows, "\n"), g.language}
	if out, ok := g.u.hoverStyles[key]; ok {
		return out
	}
	if g.u.hoverStyles == nil || len(g.u.hoverStyles) > 512 {
		g.u.hoverStyles = map[hoverStyleKey][][]tooltipSpan{}
	}
	terms := g.tooltipTerms()
	out := make([][]tooltipSpan, len(rows))
	for i, row := range rows {
		out[i] = styleTooltip(row, terms)
	}
	g.u.hoverStyles[key] = out
	return out
}
func (g *Game) drawTooltip(dst *ebiten.Image) {
	tip := g.u.frame.Tooltip
	if tip == nil {
		return
	}
	r := tip.Bounds
	g.drawBox(dst, r, color.RGBA{35, 44, 47, 250}, uiEdge)
	fitText(dst, tip.Title, r.X+16, r.Y+9, r.W-32, 16, false)
	if len(tip.Rows) == 0 {
		return
	}
	rect(dst, r.X+16, r.Y+36, r.W-32, 1, color.RGBA{79, 96, 98, 255})
	for i, row := range tip.RichRows {
		x, y := r.X+16, r.Y+43+float64(i)*24
		for _, span := range row {
			drawColoredText(dst, span.Text, x, y, 14, tooltipColors[span.Role])
			x += measuredText(span.Text, 14)
		}
	}
}
