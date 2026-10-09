package client

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/model"
)

func (g *Game) drawInventoryStatus(dst *ebiten.Image, c uiControl) {
	r, info := c.Bounds, c.InventoryStatus
	g.drawControlSkin(dst, c)
	rect(dst, r.X, r.Y, 3, r.H, accent)
	icon := ui.Rect{X: r.X + 12, Y: r.Y + 12, W: 40, H: 40}
	g.drawBox(dst, icon, color.RGBA{29, 38, 41, 255}, uiEdge)
	g.drawIconSized(dst, c.Icon, icon.X+6, icon.Y+6, 28)
	fitText(dst, c.Caption, r.X+62, r.Y+10, r.W-100, 14, false)
	g.drawIconSized(dst, "backpack", r.X+r.W-30, r.Y+10, 20)
	g.drawIconSized(dst, "coins", r.X+62, r.Y+35, 15)
	fitText(dst, info.Coins, r.X+82, r.Y+33, r.W-92, 13, false)
	fitText(dst, info.Level, r.X+12, r.Y+61, 64, 12, false)
	fitText(dst, "EXP  "+info.Experience, r.X+76, r.Y+61, r.W-88, 11, true)
	drawMeterSkin(dst, uiMeter{Bounds: ui.Rect{X: r.X + 13, Y: r.Y + r.H - 12, W: r.W - 26, H: 4}, Fraction: info.Fraction, Kind: "xp"})
}

func (g *Game) drawCharacterPortrait(dst *ebiten.Image, c uiControl) {
	r := c.Bounds
	g.drawBox(dst, r, color.RGBA{28, 37, 40, 230}, color.RGBA{65, 83, 86, 255})
	for y := r.Y + 18; y < r.Y+r.H-34; y += 24 {
		rect(dst, r.X+8, y, r.W-16, 1, color.RGBA{41, 54, 57, 255})
	}
	center := model.Vec{X: r.X + r.W/2, Y: r.Y + (r.H-32)/2}
	if !g.drawSprite(dst, "portrait:"+c.Icon, center, r.W-12, r.H-38) && !g.drawSprite(dst, "player:"+c.Icon, center, r.W-12, r.H-38) {
		drawExplorerSilhouette(dst, center, min(r.W*.68, (r.H-48)*.46))
	}
	fitText(dst, c.Caption, r.X+5, r.Y+r.H-26, r.W-10, 11, true)
}

// Until a chassis supplies portrait:<id>, use a neutral contemporary explorer.
// This is presentation only; equipment and chassis rules remain server-owned.
func drawExplorerSilhouette(dst *ebiten.Image, c model.Vec, width float64) {
	x, y, s := c.X, c.Y-width*.87, width/100
	cloth := color.RGBA{122, 151, 154, 255}
	shadow := color.RGBA{58, 79, 85, 255}
	skin := color.RGBA{210, 203, 184, 255}
	vector.DrawFilledCircle(dst, float32(x), float32(y+19*s), float32(15*s), skin, true)
	rect(dst, x-14*s, y+4*s, 28*s, 10*s, shadow)
	rect(dst, x-7*s, y+31*s, 14*s, 10*s, skin)
	rect(dst, x-23*s, y+39*s, 46*s, 63*s, cloth)
	rect(dst, x-17*s, y+43*s, 34*s, 49*s, shadow)
	rect(dst, x-34*s, y+43*s, 11*s, 62*s, cloth)
	rect(dst, x+23*s, y+43*s, 11*s, 62*s, cloth)
	rect(dst, x-34*s, y+103*s, 11*s, 12*s, skin)
	rect(dst, x+23*s, y+103*s, 11*s, 12*s, skin)
	rect(dst, x-22*s, y+99*s, 19*s, 65*s, shadow)
	rect(dst, x+3*s, y+99*s, 19*s, 65*s, shadow)
	rect(dst, x-25*s, y+162*s, 22*s, 10*s, cloth)
	rect(dst, x+3*s, y+162*s, 22*s, 10*s, cloth)
	rect(dst, x-14*s, y+65*s, 11*s, 15*s, cloth)
	rect(dst, x+3*s, y+65*s, 11*s, 15*s, cloth)
}
