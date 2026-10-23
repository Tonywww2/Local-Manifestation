package client

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
)

type windowViewport struct {
	Bounds                           ui.Rect
	VirtualHeight, Footer, MaxScroll float64
}

func (g *Game) finishWindowContent(f *uiFrame, id string) {
	v, ok := f.Viewports[id]
	if !ok {
		return
	}
	v.MaxScroll = max(0, v.VirtualHeight-v.Bounds.H)
	bodyBottom := v.Bounds.Y + 32
	measure := func(r ui.Rect) {
		if r.Y >= v.Bounds.Y+30 && r.Y < v.Bounds.Y+v.VirtualHeight-v.Footer {
			bodyBottom = max(bodyBottom, r.Y+r.H)
		}
	}
	for _, c := range f.Controls {
		if c.Owner == id {
			measure(c.Bounds)
		}
	}
	for _, l := range f.Labels {
		if l.Owner == id {
			measure(l.Bounds)
		}
	}
	v.MaxScroll = max(v.MaxScroll, bodyBottom+4-(v.Bounds.Y+v.Bounds.H-v.Footer))
	if g.u.contentScroll == nil {
		g.u.contentScroll = map[string]float64{}
	}
	offset := max(0, min(v.MaxScroll, g.u.contentScroll[id]))
	g.u.contentScroll[id] = offset
	clip := ui.Rect{X: v.Bounds.X, Y: v.Bounds.Y + 32, W: v.Bounds.W, H: max(0, v.Bounds.H-32-v.Footer)}
	transform := func(r ui.Rect) (ui.Rect, ui.Rect) {
		if r.Y < v.Bounds.Y+30 {
			return r, ui.Rect{}
		}
		if r.Y >= v.Bounds.Y+v.VirtualHeight-v.Footer {
			r.Y -= max(0, v.VirtualHeight-v.Bounds.H)
			return r, ui.Rect{}
		}
		r.Y -= offset
		return r, clip
	}
	for i := range f.Controls {
		c := &f.Controls[i]
		if c.Owner == id {
			originalClip := c.Clip
			c.Bounds, c.Clip = transform(c.Bounds)
			if isFormulaToken(c.ID) {
				originalClip.Y -= offset
				c.Clip = intersectUI(originalClip, clip)
			}
		}
	}
	for i := range f.Labels {
		l := &f.Labels[i]
		if l.Owner == id {
			l.Bounds, l.Clip = transform(l.Bounds)
		}
	}
	for i := range f.Formulas {
		formula := &f.Formulas[i]
		if formula.Owner == id {
			formula.Bounds.Y -= offset
			formula.Clip.Y -= offset
			formula.Clip = intersectUI(formula.Clip, clip)
		}
	}
	f.Viewports[id] = v
	if v.MaxScroll > 0 {
		for i, direction := range []string{"-1", "1"} {
			addControl(f, id, id+":content:"+direction, []string{"scroll-up", "scroll-down"}[i], "", "content-scroll", id+":"+direction, g.tr("ui.window.scroll"), ui.Rect{X: v.Bounds.X + v.Bounds.W - 162 + float64(i)*36, Y: v.Bounds.Y + 3, W: 30, H: 24})
		}
	}
}

func clippedUI(dst *ebiten.Image, clip ui.Rect) *ebiten.Image {
	if clip.W <= 0 || clip.H <= 0 {
		return dst
	}
	r := image.Rect(int(math.Ceil(clip.X)), int(math.Ceil(clip.Y)), int(math.Floor(clip.X+clip.W)), int(math.Floor(clip.Y+clip.H))).Intersect(dst.Bounds())
	return dst.SubImage(r).(*ebiten.Image)
}

func intersectUI(a, b ui.Rect) ui.Rect {
	x, y := max(a.X, b.X), max(a.Y, b.Y)
	return ui.Rect{X: x, Y: y, W: max(0, min(a.X+a.W, b.X+b.W)-x), H: max(0, min(a.Y+a.H, b.Y+b.H)-y)}
}
