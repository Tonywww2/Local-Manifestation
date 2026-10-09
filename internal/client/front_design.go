package client

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
)

func menuBounds(bounds ui.Size) ui.Rect {
	width := min(430, max(270, bounds.W*.36))
	height := min(370, bounds.H-88)
	return ui.Rect{X: bounds.W * .043, Y: max(28, (bounds.H-height)/2-12), W: width, H: height}
}

func (g *Game) buildMainMenu(f *uiFrame) {
	r := menuBounds(g.uiBounds())
	f.Panels = append(f.Panels, ui.Panel{ID: "main-menu", Bounds: r})
	size := min(42, r.W/8)
	title := g.tr("main.title")
	if width := measuredText(title, size); width > r.W {
		size *= r.W / width
	}
	f.Labels = append(f.Labels, uiLabel{Owner: "main-menu", Bounds: ui.Rect{X: r.X, Y: r.Y, W: r.W, H: 60}, Text: title, Size: size, Color: menuInk})
	for i, id := range []string{"start", "settings", "exit"} {
		g.caption(f, "main-menu", "main:"+id, id, g.tr("main."+id), ui.Rect{X: r.X + 4, Y: r.Y + 106 + float64(i)*76, W: r.W - 20, H: 60})
	}
	f.Labels = append(f.Labels, uiLabel{Owner: "main-menu", Bounds: ui.Rect{X: 30, Y: float64(g.height) - 40, W: 150, H: 24}, Text: g.catalog.Manifest().Version, Size: 13, Color: color.RGBA{203, 210, 205, 255}})
}

func (g *Game) drawMainBackdrop(dst *ebiten.Image) {
	img := g.resources.images["ui:main-background"]
	if img == nil {
		dst.Fill(color.RGBA{220, 220, 210, 255})
		return
	}
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	scale := max(float64(g.width)/w, float64(g.height)/h)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(scale, scale)
	// Keep the quiet left side usable at every aspect ratio; crop at the right.
	op.GeoM.Translate(0, (float64(g.height)-h*scale)/2)
	dst.DrawImage(img, op)
}

func (g *Game) buildHallHeading(f *uiFrame) {
	r := ui.Rect{X: 20, Y: 18, W: max(260, float64(g.width)-260), H: 68}
	f.Panels = append(f.Panels, ui.Panel{ID: "hall-heading", Bounds: r})
	f.Labels = append(f.Labels, uiLabel{Owner: "hall-heading", Bounds: ui.Rect{X: r.X + 12, Y: r.Y, W: r.W - 12, H: 34}, Text: g.tr("hall.title"), Size: 25})
	chassis := g.catalog.ChassisList()
	name := g.name
	if g.chassis >= 0 && g.chassis < len(chassis) {
		name += " · " + g.contentName(chassis[g.chassis].ID, chassis[g.chassis].Name)
	}
	f.Labels = append(f.Labels, uiLabel{Owner: "hall-heading", Bounds: ui.Rect{X: r.X + 12, Y: r.Y + 39, W: r.W - 12, H: 24}, Text: name, Size: 13})
	r = ui.Rect{X: float64(g.width) - 218, Y: 18, W: 200, H: 64}
	f.Panels = append(f.Panels, ui.Panel{ID: "hall-wealth", Bounds: r})
	points := g.profiles.Scopes[g.localScope()].Points
	f.Labels = append(f.Labels, uiLabel{Owner: "hall-wealth", Bounds: ui.Rect{X: r.X + 34, Y: r.Y + 4, W: 90, H: 24}, Text: fmt.Sprint(points), Size: 20})
	f.Labels = append(f.Labels, uiLabel{Owner: "hall-wealth", Bounds: ui.Rect{X: r.X + 34, Y: r.Y + 32, W: 110, H: 20}, Text: g.tr("hall.points"), Size: 11, Color: muted})
	addControl(f, "hall-wealth", "hall:chassis", "armor", "", "window", "chassis-select", g.tr("hall.chassis"), ui.Rect{X: r.X + 126, Y: r.Y + 6, W: 30, H: 30})
	addControl(f, "hall-wealth", "hall:settings", "settings", g.bindingLabel(g.bound("settings")), "window", "settings", g.tr("ui.settings"), ui.Rect{X: r.X + 164, Y: r.Y + 6, W: 36, H: 50})
	if g.pending != nil {
		g.addLabel(f, "hall-heading", ui.Rect{X: 32, Y: 88, W: 300, H: 24}, g.tr("notice.connecting"))
	}
	r = ui.Resolve(ui.Placement{Anchor: "bottom_center", Y: -18}, ui.Size{W: min(440, float64(g.width)-36), H: 36}, g.uiBounds())
	f.Panels = append(f.Panels, ui.Panel{ID: "hall-help", Bounds: r})
	message := g.tr("hall.walk", "key", g.bindingLabel(g.bound("interact")))
	f.Labels = append(f.Labels, uiLabel{Owner: "hall-help", Bounds: ui.Rect{X: r.X + 8, Y: r.Y + 7, W: r.W - 16, H: 22}, Text: message, Size: 13, Centered: true})
}
