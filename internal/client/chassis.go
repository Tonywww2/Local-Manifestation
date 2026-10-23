package client

import (
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/model"
)

func (g *Game) buildChassisWindow(f *uiFrame, layout ui.Layout) {
	chassis := g.catalog.ChassisList()
	ranks := g.joinRanks()
	rows := (len(chassis) + 1) / 2
	r := g.windowPanel(f, layout, "chassis-select", g.tr("hall.chassis"), ui.Size{W: 520, H: float64(rows*126 + 100)})
	for i, ch := range chassis {
		name := g.contentName(ch.ID, ch.Name)
		c := uiControl{Owner: "chassis-select", ID: "chassis:" + strconv.Itoa(i), Icon: ch.ID, Action: "chassis-preset", Value: strconv.Itoa(i), Title: name, Caption: name,
			Bounds: ui.Rect{X: r.X + 14 + float64(i%2)*246, Y: r.Y + 44 + float64(i/2)*126, W: 232, H: 116}}
		if i == g.chassis {
			c.Caption = "[ " + name + " ]"
		}
		if ch.RequiredPerk != "" && ranks[ch.RequiredPerk] == 0 {
			c.Badge = g.tr("ui.preset.locked")
		}
		c.Summary = []string{g.localized("content."+ch.ID+".description", ""), g.tr("ui.preset.base", "hp", ch.HP, "armor", ch.Armor), g.contentName(ch.Weapon, g.catalog.Weapon(ch.Weapon).Name)}
		c.Details = []string{g.tr("ui.preset.attributes", "strength", ch.Attributes.Strength, "agility", ch.Attributes.Agility, "constitution", ch.Attributes.Constitution, "intelligence", ch.Attributes.Intelligence, "sense", ch.Attributes.Sense, "mana", ch.Attributes.Mana)}
		c.Summary = append(c.Summary, g.tr("ui.growth.base"), g.growthPreferenceLine(ch))
		if c.Badge != "" {
			c.Summary = append(c.Summary, c.Badge)
		}
		f.Controls = append(f.Controls, c)
	}
	g.addLabel(f, "chassis-select", ui.Rect{X: r.X + 14, Y: r.Y + r.H - 42, W: r.W - 28, H: 36}, g.tr("ui.preset.help"))
}

func (g *Game) selectPreset(value string) {
	i, err := strconv.Atoi(value)
	chassis := g.catalog.ChassisList()
	if err != nil || i < 0 || i >= len(chassis) || g.state != nil || g.frontBusy() {
		return
	}
	ch := chassis[i]
	if ch.RequiredPerk != "" && g.joinRanks()[ch.RequiredPerk] == 0 {
		return
	}
	s := g.preferences()
	s.Chassis = ch.ID
	if err := g.commitPreferences(s); err != nil {
		g.notifyError(err)
		return
	}
	g.closeWindow("chassis-select")
}

func (g *Game) drawPresetCard(dst *ebiten.Image, c uiControl) {
	r := c.Bounds
	center := model.Vec{X: r.X + 43, Y: r.Y + 48}
	if !g.drawSprite(dst, "portrait:"+c.Icon, center, 60, 74) && !g.drawSprite(dst, "player:"+c.Icon, center, 60, 74) {
		icon := "weapon"
		if c.Icon == "base:arcanist" || c.Icon == "base:renewal-frame" {
			icon = "status"
		}
		g.drawIcon(dst, icon, center.X-9, center.Y-9)
	}
	g.fitText(dst, strings.Trim(c.Caption, "[] "), r.X+80, r.Y+16, r.W-90, 14, false)
	if strings.HasPrefix(c.Caption, "[ ") {
		g.drawIcon(dst, "check", r.X+r.W-26, r.Y+4)
	}
	if ch, ok := g.catalog.Chassis(c.Icon); ok {
		g.fitText(dst, g.contentName(ch.Weapon, g.catalog.Weapon(ch.Weapon).Name), r.X+80, r.Y+44, r.W-90, 12, false)
		g.fitText(dst, g.tr("ui.preset.base", "hp", ch.HP, "armor", ch.Armor), r.X+12, r.Y+90, r.W-24, 11, false)
	}
	if c.Badge != "" {
		g.fitText(dst, c.Badge, r.X+80, r.Y+67, r.W-90, 11, false)
	}
}
