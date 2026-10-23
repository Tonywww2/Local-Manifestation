package client

import (
	"fmt"
	"strings"

	"hsmultiplayer/pkg/damage"
)

func (g *Game) weaponDamageSummary(r damage.Result, equipped bool) string {
	segments := r.Segments
	if len(segments) == 0 {
		segments = []damage.SegmentResult{{Damage: r}}
	}
	parts := make([]string, 0, len(segments))
	for _, s := range segments {
		parts = append(parts, g.tr("ui.weapon.damage.part", "value", fmt.Sprintf("%.2f", s.Damage.Total), "type", g.localized("damage."+s.Damage.Type, s.Damage.Type)))
	}
	key := "ui.weapon.damage.current"
	if !equipped {
		key = "ui.weapon.damage.preview"
	}
	return g.tr(key, "damage", strings.Join(parts, " + "))
}
