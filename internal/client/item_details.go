package client

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
)

type itemContextMenu struct {
	Item     model.ItemInstance
	Position ui.Point
}

// Inspection values contain only localized text and detached children. Pinning
// captures the shown calculation, so later snapshots cannot change its sources.
type inspectionNode struct {
	Title            string
	Summary, Details []string
	Children         []inspectionNode
}
type itemDetailsView struct {
	Item      model.ItemInstance
	Key       itemViewKey
	Valid     bool
	Title     string
	Lines     []string
	Equations []damageEquation
}
type damageEquation struct {
	Label  string
	Tokens []formulaToken
	Parts  []inspectionNode
}
type formulaToken struct {
	damage.FormulaToken
	Role string
}
type uiFormula struct {
	Owner        string
	Bounds, Clip ui.Rect
	Size         float64
	Tokens       []formulaGlyph
}
type formulaGlyph struct {
	Text, Role string
	X          float64
}

func cloneInspection(n inspectionNode) inspectionNode {
	n.Summary = append([]string(nil), n.Summary...)
	n.Details = append([]string(nil), n.Details...)
	children := make([]inspectionNode, len(n.Children))
	for i, c := range n.Children {
		children[i] = cloneInspection(c)
	}
	n.Children = children
	return n
}
func (g *Game) openInspection(node inspectionNode) {
	if g.u.inspections == nil {
		g.u.inspections = map[string]inspectionNode{}
	}
	g.u.inspectNext++
	id := fmt.Sprintf("inspect:%d", g.u.inspectNext)
	g.u.inspections[id] = cloneInspection(node)
	if g.u.inspectionLayout == nil {
		g.u.inspectionLayout = ui.Layout{}
	}
	g.u.inspectionLayout[id] = ui.Placement{Anchor: "top_left", X: g.u.cursor.X + 16, Y: g.u.cursor.Y + 16}
	g.openWindow(id)
}
func (g *Game) openItemDetails(item model.ItemInstance) {
	if item.ID == "" {
		return
	}
	if g.u.itemDetails == nil {
		g.u.itemDetails = map[string]*itemDetailsView{}
	}
	for id, v := range g.u.itemDetails {
		if v.Item.ID == item.ID && v.Item.Definition == item.Definition {
			g.openWindow(id)
			return
		}
	}
	g.u.inspectNext++
	id := fmt.Sprintf("item-details:%d", g.u.inspectNext)
	g.u.itemDetails[id] = &itemDetailsView{Item: item}
	g.openWindow(id)
}
func (g *Game) buildItemContextMenu(f *uiFrame) {
	menu := g.u.itemMenu
	if menu == nil {
		return
	}
	d, _ := g.catalog.Item(menu.Item.Definition)
	v := g.displayInventory()
	owned, hasOwned := inventory.Find(v, menu.Item.ID)
	canDrop := hasOwned && v.Cursor == "" && g.self() != nil && g.self().Life == "alive" && g.self().Node != ""
	movement := d.Kind == "movement" && inventory.InBag(v, menu.Item.ID)
	height := 42.0
	if movement {
		height += 38
	}
	if canDrop {
		height += 38
	}
	r := ui.Resolve(ui.Placement{Anchor: "top_left", X: menu.Position.X, Y: menu.Position.Y}, ui.Size{W: 180, H: height}, g.uiBounds())
	f.Panels = append(f.Panels, ui.Panel{ID: "item-context", Bounds: r})
	f.Controls = append(f.Controls, uiControl{Owner: "item-context", ID: "item-context:details", Caption: g.tr("ui.item.details"), Title: g.tr("ui.item.details"), Action: "item-details", Item: menu.Item, Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: 34}})
	y := r.Y + 42
	if movement {
		f.Controls = append(f.Controls, uiControl{Owner: "item-context", ID: "item-context:movement", Caption: g.tr("ui.movement.select"), Title: g.tr("ui.movement.select"), Action: "movement-select", Value: menu.Item.ID, Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 42, W: r.W - 8, H: 34}})
		y += 38
	}
	if canDrop {
		caption := g.tr("ui.item.drop")
		if inventory.Count(owned) > 1 {
			caption = g.tr("ui.item.drop_stack", "count", inventory.Count(owned))
		}
		f.Controls = append(f.Controls, uiControl{Owner: "item-context", ID: "item-context:drop", Caption: caption, Title: caption, Action: "item-drop", Value: owned.ID, Bounds: ui.Rect{X: r.X + 4, Y: y, W: r.W - 8, H: 34}})
	}
}
func (g *Game) sourceInspection(source model.SourceRef) inspectionNode {
	node := inspectionNode{Title: g.sourceName(source)}
	if d, ok := g.catalog.Item(source.Definition); ok {
		node.Summary = append(node.Summary, g.localized("content."+d.ID+".description", d.Description))
		for _, m := range d.Modifiers {
			node.Details = append(node.Details, g.statModifierText(m))
		}
	}
	if d, ok := g.catalog.Status(source.Definition); ok {
		node.Summary = append(node.Summary, g.localized(d.DescriptionKey, d.Description))
	}
	if source.Instance != "" {
		node.Details = append(node.Details, g.tr("ui.formula.instance", "id", source.Instance))
	}
	return node
}
func (g *Game) damageStepInspection(step damage.Step) inspectionNode {
	title := g.damageSource(step.Source)
	if step.Group != "" {
		title = g.localized("damage.group."+step.Group, step.Group)
	}
	if step.Stat != "" {
		title = g.tr("attribute." + step.Stat)
	}
	node := inspectionNode{Title: title, Summary: g.damageLines(damage.Result{Steps: []damage.Step{step}})}
	if step.Group != "" {
		terms := []string{"1"}
		for _, member := range step.Members {
			terms = append(terms, fmt.Sprintf("%+.2f", member.Percent))
		}
		node.Summary = append([]string{strings.Join(terms, " ") + fmt.Sprintf(" = %.2f", step.Operand)}, node.Summary...)
	}
	if len(node.Summary) == 0 {
		node.Summary = []string{fmt.Sprintf("%.2f → %.2f", step.Input, step.Output)}
	}
	// The short Hover presents a concise composition. The pinned window exposes
	// each original member as a selectable child rather than an opaque paragraph.
	for _, m := range step.Members {
		child := g.sourceInspection(m.Origin)
		child.Title = g.damageSource(m.Source)
		if m.Origin.Kind != "" {
			child.Title = g.sourceName(m.Origin)
		}
		child.Summary = append([]string{fmt.Sprintf("%+.2f%%", m.Percent*100)}, child.Summary...)
		node.Children = append(node.Children, child)
	}
	for _, s := range step.AttributeSteps {
		stage := s.Operation
		if stage == "add" && len(s.Contributions) > 0 && s.Contributions[0].Operation == "final_add" {
			stage = "final_add"
		}
		stageTitle := g.tr("ui.formula.stage." + stage)
		if s.Group != "" {
			stageTitle += " · " + g.localized("damage.group."+s.Group, s.Group)
		}
		child := inspectionNode{Title: g.tr("attribute."+s.Stat) + " · " + stageTitle, Summary: []string{fmt.Sprintf("%.2f → %.2f", s.Input, s.Output)}}
		for _, c := range s.Contributions {
			source := g.sourceInspection(c.Source)
			source.Summary = append([]string{g.statModifierText(c.StatModifier)}, source.Summary...)
			child.Children = append(child.Children, source)
		}
		node.Children = append(node.Children, child)
	}
	if step.Origin.Kind != "" {
		node.Children = append(node.Children, g.sourceInspection(step.Origin))
	}
	// Keep full evaluator lines available when Shift is held, but trim the
	// default Hover to the step and a small source list.
	if len(node.Summary) > 4 {
		node.Details = append([]string(nil), node.Summary[4:]...)
		node.Summary = node.Summary[:4]
	}
	return node
}
func (g *Game) damageEquations(result damage.Result, label string) []damageEquation {
	segments := result.Segments
	if len(segments) == 0 {
		segments = []damage.SegmentResult{{ID: "base", Damage: result}}
	}
	var equations []damageEquation
	for _, segment := range segments {
		tokens, err := outgoingFormulaTokens(segment.Damage)
		if err != nil {
			continue
		}
		parts := make([]inspectionNode, len(segment.Damage.Steps))
		for i, step := range segment.Damage.Steps {
			parts[i] = g.damageStepInspection(step)
		}
		equations = append(equations, damageEquation{Label: label + " · " + g.localized("damage."+segment.Damage.Type, segment.Damage.Type), Tokens: tokens, Parts: parts})
	}
	return equations
}

// Color follows the evaluator's meaning, never a parsed numeric literal. Equal
// scores in different attributes can therefore keep their distinct colors.
func outgoingFormulaTokens(r damage.Result) ([]formulaToken, error) {
	tokens, err := damage.Formula(r)
	if err != nil {
		return nil, err
	}
	out := make([]formulaToken, len(tokens))
	for i, token := range tokens {
		role := ""
		if token.Step >= 0 && token.Step < len(r.Steps) {
			step := r.Steps[token.Step]
			role = r.Type
			if step.Stat != "" {
				role = step.Stat
			} else if step.Source == "critical" {
				role = "number"
			}
		} else if i == len(tokens)-1 {
			role = r.Type
		}
		out[i] = formulaToken{FormulaToken: token, Role: role}
	}
	return out, nil
}
func (g *Game) refreshItemDetails(view *itemDetailsView) {
	item := view.Item
	if current, ok := inventory.Find(g.displayInventory(), item.ID); ok {
		item = current
	}
	key := g.itemViewKey(item)
	if view.Valid && view.Key == key {
		return
	}
	view.Key, view.Valid, view.Item = key, true, item
	c := g.itemControl("", "", item, ui.Rect{}, "")
	view.Title = c.Title
	view.Lines = append(append([]string(nil), c.Summary...), c.Details...)
	view.Equations = nil
	d, ok := g.catalog.Item(item.Definition)
	p := g.self()
	if !ok || d.Kind != "weapon" || p == nil {
		return
	}
	q, err := g.catalog.PreviewWeaponAttack(*p, item)
	if err != nil {
		view.Lines = append(view.Lines, g.tr("ui.compare.unavailable"))
		return
	}
	for _, critical := range []bool{false, true} {
		q.Critical = critical
		result, err := content.EvaluatePreview(q)
		if err != nil {
			continue
		}
		label := g.tr("ui.formula.normal")
		if critical {
			label = g.tr("ui.formula.critical")
		}
		view.Equations = append(view.Equations, g.damageEquations(result, label)...)
		if len(result.Segments) > 1 {
			view.Lines = append(view.Lines, g.tr("ui.formula.total", "kind", label, "value", fmt.Sprintf("%.2f", result.Total)))
		}
	}
}
func (g *Game) buildInspectionWindow(f *uiFrame, layout ui.Layout, id string) bool {
	if id == "weapon" && g.self() != nil {
		if item, ok := inventory.Find(g.self().Inventory, g.self().Inventory.Equipment[0]); ok {
			if g.u.itemDetails == nil {
				g.u.itemDetails = map[string]*itemDetailsView{}
			}
			if g.u.itemDetails[id] == nil || g.u.itemDetails[id].Item.ID != item.ID {
				g.u.itemDetails[id] = &itemDetailsView{Item: item}
			}
		}
	}
	if view := g.u.itemDetails[id]; view != nil {
		g.refreshItemDetails(view)
		lines := g.cachedWrap(view.Lines, min(1020, float64(g.width)-16)-28, 13)
		height := max(460, 150+float64(len(lines))*20+float64(len(view.Equations))*76)
		r := g.windowPanel(f, layout, id, view.Title, ui.Size{W: 1020, H: min(620, float64(g.height)-16)})
		if f.Viewports == nil {
			f.Viewports = map[string]windowViewport{}
		}
		height = max(height, r.H)
		f.Viewports[id] = windowViewport{Bounds: r, VirtualHeight: height, Footer: 42}
		y := r.Y + 44
		for _, line := range lines {
			g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y, W: r.W - 28, H: 20}, line)
			y += 20
		}
		if len(view.Equations) > 0 {
			y += 8
			for _, line := range g.cachedWrap([]string{g.tr("ui.formula.preview")}, r.W-28, 13) {
				g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y, W: r.W - 28, H: 20}, line)
				y += 20
			}
			y += 10
			for i, e := range view.Equations {
				g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y, W: r.W - 28, H: 20}, e.Label)
				g.addFormulaRow(f, id, fmt.Sprintf("%s:%d", id, i), e, ui.Rect{X: r.X + 14, Y: y + 24, W: r.W - 100, H: 34})
				y += 76
			}
		}
		// Fixed footer belongs to the actual viewport, not the virtual body.
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + height - 34, W: r.W - 28, H: 26}, g.tr("ui.formula.help"))
		return true
	}
	node, ok := g.u.inspections[id]
	if !ok {
		return false
	}
	lines := append(append([]string(nil), node.Summary...), node.Details...)
	wrapped := g.cachedWrap(lines, 490, 13)
	height := max(300, 120+float64(len(wrapped))*20+float64(len(node.Children))*38)
	r := g.windowPanel(f, layout, id, node.Title, ui.Size{W: 520, H: min(height, min(560, float64(g.height)-16))})
	if f.Viewports == nil {
		f.Viewports = map[string]windowViewport{}
	}
	height = max(height, r.H)
	f.Viewports[id] = windowViewport{Bounds: r, VirtualHeight: height, Footer: 42}
	y := r.Y + 42
	for _, line := range wrapped {
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y, W: r.W - 28, H: 20}, line)
		y += 20
	}
	for i := range node.Children {
		child := node.Children[i]
		f.Controls = append(f.Controls, uiControl{Owner: id, ID: fmt.Sprintf("%s:child:%d", id, i), Caption: "› " + child.Title, Title: child.Title, Summary: child.Summary, Details: child.Details, Action: "formula-pin", Inspection: &child, Bounds: ui.Rect{X: r.X + 14, Y: y + 8, W: r.W - 28, H: 30}})
		y += 38
	}
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + height - 34, W: r.W - 28, H: 24}, g.tr("ui.formula.pinned"))
	return true
}
func (g *Game) addFormulaRow(f *uiFrame, owner, key string, e damageEquation, r ui.Rect) {
	size, total := 15.0, 0.0
	for _, t := range e.Tokens {
		total += measuredText(t.Text, size)
	}
	if total > r.W {
		size = max(12, size*r.W/total)
	}
	total = 0
	for _, t := range e.Tokens {
		total += measuredText(t.Text, size)
	}
	maximum := max(0, total-r.W)
	offset := max(0, min(maximum, g.u.formulaOffsets[key]))
	view := uiFormula{Owner: owner, Bounds: r, Clip: r, Size: size}
	x := r.X - offset
	for i, t := range e.Tokens {
		width := measuredText(t.Text, size)
		view.Tokens = append(view.Tokens, formulaGlyph{Text: t.Text, Role: t.Role, X: x})
		if t.Step >= 0 && t.Step < len(e.Parts) {
			part := e.Parts[t.Step]
			f.Controls = append(f.Controls, uiControl{Owner: owner, ID: fmt.Sprintf("formula-token:%s:%d", key, i), Title: part.Title, Summary: part.Summary, Details: part.Details, Action: "formula-pin", Inspection: &part, Bounds: ui.Rect{X: x, Y: r.Y, W: width, H: r.H}, Clip: r})
		}
		x += width
	}
	f.Formulas = append(f.Formulas, view)
	if maximum > 0 {
		for i, action := range []string{"formula-left", "formula-right"} {
			addControl(f, owner, key+":"+action, []string{"scroll-left", "scroll-right"}[i], "", action, key, g.tr("ui.formula.scroll"), ui.Rect{X: r.X + r.W + 8 + float64(i)*32, Y: r.Y, W: 28, H: 30})
			f.Controls[len(f.Controls)-1].Count = int(math.Ceil(maximum))
		}
	}
}
func (g *Game) drawFormula(dst *ebiten.Image, view uiFormula) {
	if view.Clip.W <= 0 || view.Clip.H <= 0 {
		return
	}
	dst = clippedUI(dst, view.Clip)
	for _, t := range view.Tokens {
		if t.Role != "" {
			drawTextSpan(dst, t.Text, t.X, view.Bounds.Y+5, view.Size, semanticColor(t.Role, uiInk))
		} else {
			g.drawText(dst, t.Text, t.X, view.Bounds.Y+5, view.Size)
		}
	}
}

// Formula and source windows share the standard window lifecycle; regular Hover
// remains pointer-bound, read-only and excluded from picking.
func isFormulaToken(id string) bool { return strings.HasPrefix(id, "formula-token:") }

func inspectionWindowID(id string) bool {
	return strings.HasPrefix(id, "inspect:") || strings.HasPrefix(id, "item-details:")
}

func (g *Game) clearInspections() {
	g.u.itemMenu = nil
	g.u.itemDetails = nil
	g.u.inspections = nil
	g.u.inspectionLayout = nil
	g.u.formulaOffsets = nil
}
