package client

import (
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"hsmultiplayer/internal/client/ui"
)

func editorKeyRepeat(key ebiten.Key) bool {
	duration := inpututil.KeyPressDuration(key)
	return duration == 1 || duration > 18 && duration%3 == 0
}

// A separate input boundary consumes every edit gesture. HUD buttons cannot
// consume items, open windows or rotate the camera while being positioned.
func (g *Game) updateHUDLayoutEditor(cursor ui.Point) {
	e := g.u.hudEditor
	if e.Axis != "" {
		if (ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)) && inpututil.IsKeyJustPressed(ebiten.KeyA) {
			e.ReplaceText = true
		}
		for _, ch := range ebiten.AppendInputChars(nil) {
			if ch >= '0' && ch <= '9' && e.ReplaceText {
				e.Text, e.ReplaceText = "", false
			}
			if ch >= '0' && ch <= '9' && len(e.Text) < 5 {
				e.Text += string(ch)
			}
		}
		if editorKeyRepeat(ebiten.KeyBackspace) && len(e.Text) > 0 {
			if e.ReplaceText {
				e.Text, e.ReplaceText = "", false
			} else {
				e.Text = e.Text[:len(e.Text)-1]
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			e.Axis, e.Text = "", ""
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) || inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			g.commitHUDCoordinate()
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.finishHUDLayoutEditor()
		return
	}
	if d := g.u.drag; d != nil {
		if d.ID == "editor" {
			e.Toolbar = ui.Resolve(ui.Placement{Anchor: "top_left", X: d.Bounds.X + cursor.X - d.Start.X, Y: d.Bounds.Y + cursor.Y - d.Start.Y}, ui.Size{W: d.Bounds.W, H: d.Bounds.H}, g.uiBounds())
		} else {
			g.u.editor = ui.DragTo(g.u.editor, *d, ui.Point{X: d.Start.X + math.Round(cursor.X-d.Start.X), Y: d.Start.Y + math.Round(cursor.Y-d.Start.Y)}, g.uiBounds())
		}
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			g.u.drag = nil
		}
		return
	}
	if e.Axis == "" {
		dx, dy := 0.0, 0.0
		if editorKeyRepeat(ebiten.KeyArrowLeft) {
			dx--
		}
		if editorKeyRepeat(ebiten.KeyArrowRight) {
			dx++
		}
		if editorKeyRepeat(ebiten.KeyArrowUp) {
			dy--
		}
		if editorKeyRepeat(ebiten.KeyArrowDown) {
			dy++
		}
		step := 1.0
		if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
			step = 10
		}
		if dx != 0 || dy != 0 {
			if p, ok := hudPanel(g.u.frame, e.Selected); ok {
				g.setHUDPosition(p.Bounds.X+dx*step, p.Bounds.Y+dy*step)
			}
		}
	}
	c, over := hitControl(g.u.frame, cursor)
	g.u.hover = ""
	if over && c.Owner == "editor" {
		g.u.hover = c.ID
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	g.commitHUDCoordinate()
	if e.Toolbar.Contains(cursor) {
		if over && c.Owner == "editor" {
			switch c.Action {
			case "layout-select":
				e.Selected = c.Value
			case "layout-coordinate":
				if p, ok := hudPanel(g.u.frame, e.Selected); ok {
					n := p.Bounds.X
					if c.Value == "y" {
						n = p.Bounds.Y
					}
					e.Axis, e.Text = c.Value, strconv.Itoa(int(math.Round(n)))
					e.ReplaceText = true
				}
			case "layout-reset-one":
				delete(g.u.editor, e.Selected)
			default:
				g.performControl(c)
			}
		} else if e.Toolbar.Header().Contains(cursor) {
			g.u.drag = &ui.Drag{ID: "editor", Start: cursor, Bounds: e.Toolbar, Anchor: "top_left"}
		}
		return
	}
	// Selected handles win overlaps; the list can select a fully covered widget.
	panels := g.u.frame.Panels
	for pass := 0; pass < 2; pass++ {
		for i := len(panels) - 1; i >= 0; i-- {
			p := panels[i]
			if combatHUDKey(p.ID) == "" || (pass == 0) != (p.ID == e.Selected) {
				continue
			}
			if p.Bounds.Contains(cursor) || hudHandle(p).Contains(cursor) {
				e.Selected = p.ID
				g.u.drag = &ui.Drag{ID: p.ID, Start: cursor, Bounds: p.Bounds, Anchor: g.hudAnchor(p.ID)}
				return
			}
		}
	}
}
