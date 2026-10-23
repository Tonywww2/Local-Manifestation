package client

import (
	"bytes"
	_ "embed"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	textv2 "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Original unmodified Noto Sans CJK SC, licensed under SIL OFL 1.1 (fonts/LICENSE).
//
//go:embed fonts/NotoSansCJKsc-Regular.otf
var fontBytes []byte

var fontSource = func() *textv2.GoTextFaceSource {
	source, err := textv2.NewGoTextFaceSource(bytes.NewReader(fontBytes))
	if err != nil {
		panic(err)
	}
	return source
}()

func labelFits(value string, width, size float64) bool {
	measured, _ := textv2.Measure(value, &textv2.GoTextFace{Source: fontSource, Size: size}, 18)
	return measured <= width
}

// Wrap uses the same embedded glyph metrics as rendering; it never shrinks text.
func wrapLines(values []string, width, size float64, terms []textTerm) []string {
	face := &textv2.GoTextFace{Source: fontSource, Size: size}
	out := []string{}
	for _, value := range values {
		for _, line := range strings.Split(value, "\n") {
			runes := []rune(line)
			if len(runes) == 0 {
				out = append(out, "")
				continue
			}
			for len(runes) > 0 {
				lo, hi := 1, len(runes)
				for lo < hi {
					mid := (lo + hi + 1) / 2
					w, _ := textv2.Measure(string(runes[:mid]), face, 20)
					if w <= width {
						lo = mid
					} else {
						hi = mid - 1
					}
				}
				n := lo
				if n < len(runes) {
					for j := n - 1; j > n/2; j-- {
						if runes[j] == ' ' {
							n = j + 1
							break
						}
					}
				}
				n = termBoundary(runes, n, terms)
				out = append(out, strings.TrimSpace(string(runes[:n])))
				runes = runes[n:]
			}
		}
	}
	return out
}

func (g *Game) drawText(dst *ebiten.Image, value string, x, y, size float64) {
	g.drawColoredText(dst, value, x, y, size, color.RGBA{235, 240, 249, 255})
}
func measuredText(value string, size float64) float64 {
	w, _ := textv2.Measure(value, &textv2.GoTextFace{Source: fontSource, Size: size}, 18)
	return w
}
func drawTextSpan(dst *ebiten.Image, value string, x, y, size float64, col color.Color) {
	face := &textv2.GoTextFace{Source: fontSource, Size: size}
	op := &textv2.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.LineSpacing = 18
	op.ColorScale.ScaleWithColor(col)
	textv2.Draw(dst, value, face, op)
}

// Fit labels using glyph metrics, including Chinese, instead of byte lengths.
func (g *Game) fitText(dst *ebiten.Image, value string, x, y, width, size float64, centered bool) {
	face := &textv2.GoTextFace{Source: fontSource, Size: size}
	w, _ := textv2.Measure(value, face, 18)
	if w > width {
		runes := []rune(value)
		lo, hi := 0, len(runes)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			measured, _ := textv2.Measure(string(runes[:mid])+"…", face, 18)
			if measured <= width {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		g.ensureTextTerms()
		lo = termBoundary(runes, lo, g.u.textTerms)
		value = string(runes[:lo]) + "…"
		w, _ = textv2.Measure(value, face, 18)
	}
	if centered {
		x += (width - w) / 2
	}
	g.drawText(dst, value, x, y, size)
}
