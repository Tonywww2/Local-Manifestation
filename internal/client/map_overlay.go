package client

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
)

func (g *Game) mapOverlayLines() []string {
	var lines []string
	if n, ok := g.mapNode(g.u.selectedNode); ok {
		info := g.mapNodeLabel(n) + " · " + g.nodeType(n)
		if route := g.personalRoute(n.ID); g.u.movementItem == "" && len(route) > 0 && !sharedEntry(n) {
			info += " · " + g.tr("ui.map.personal_cost", "cost", len(route))
		}
		lines = append(lines, info)
		if g.visibleMapNodes()[n.ID] && (n.ID == g.currentNode().ID || len(g.personalRoute(n.ID)) > 0) {
			if hint := g.localized(n.HintKey, n.Hint); hint != "" {
				lines = append(lines, hint)
			}
		}
		if g.state.Vote != nil && g.state.Vote.Node == n.ID {
			lines = append(lines, g.tr("ui.map.vote_count", "yes", len(g.state.Vote.Yes), "required", g.state.Vote.Required))
		}
	}
	return append(lines, g.tr("ui.map.reachable_legend"), g.tr("ui.map.drag_help"))
}

// Transparent text uses no reserved footer space or hit target. Nodes under
// the instructions still receive pointer input.
func (g *Game) drawMapOverlay(dst *ebiten.Image, bounds ui.Rect) {
	dst = clippedUI(dst, bounds)
	rows := g.cachedWrap(g.mapOverlayLines(), bounds.W-44, 12)
	y := bounds.Y + bounds.H - 12 - float64(len(rows))*18
	for _, row := range rows {
		g.drawColoredText(dst, row, bounds.X+9, y+1, 12, color.RGBA{10, 16, 18, 255})
		g.drawColoredText(dst, row, bounds.X+8, y, 12, color.RGBA{209, 222, 221, 255})
		y += 18
	}
}
