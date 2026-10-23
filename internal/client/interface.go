package client

import (
	"encoding/json"
	"fmt"
	"hsmultiplayer/pkg/perf"
	"image/color"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/stats"
)

type uiControl struct {
	Item                                       model.ItemInstance
	ItemKind                                   string
	Inspection                                 *inspectionNode
	InventoryStatus                            *inventoryStatusCard
	Peer                                       *hudPeerView
	ID, Owner, Icon, Key, Action, Value, Title string
	Badge                                      string
	Slot                                       string
	Caption                                    string
	Count                                      int
	Bounds                                     ui.Rect
	Clip                                       ui.Rect
	Hex                                        bool
	HexAngle                                   float64
	Summary, Details                           []string
}
type uiLabel struct {
	Owner    string
	Bounds   ui.Rect
	Text     string
	Size     float64
	Clip     ui.Rect
	Color    color.RGBA
	Centered bool
}
type uiMeter struct {
	Owner    string
	Bounds   ui.Rect
	Fraction float64
	Kind     string
}
type uiDot struct {
	Owner     string
	Center    ui.Point
	Available bool
}
type uiFrame struct {
	MapTargets map[string]mapTravelTarget
	Viewports  map[string]windowViewport
	Panels     []ui.Panel
	Titles     map[string]string
	Controls   []uiControl
	Labels     []uiLabel
	Formulas   []uiFormula
	Meters     []uiMeter
	Dots       []uiDot
	Hints      []uiControl
	Tooltip    *uiTooltip
}
type uiTooltip struct {
	Bounds   ui.Rect
	Title    string
	Rows     []string
	RichRows [][]textSpan
}
type clientUI struct {
	travelPreview                       mapTravelPreview
	developerItems                      developerItemBrowser
	arrivalNode                         string
	arrivalUntil                        time.Time
	movementItem                        string
	movementMenu                        bool
	inspectionLayout                    ui.Layout
	itemMenu                            *itemContextMenu
	itemDetails                         map[string]*itemDetailsView
	inspections                         map[string]inspectionNode
	inspectNext                         uint64
	formulaOffsets                      map[string]float64
	routeState                          *protocol.State
	routeFrom                           string
	routes                              map[string][]string
	visibilityState                     *protocol.State
	visibilityFrom                      string
	visibleNodes                        map[string]bool
	entryShared                         bool
	returnCursorPending                 bool
	textStyles                          map[string][]textSpan
	textTerms                           []textTerm
	textLanguage                        string
	mouseInventory                      *model.Inventory
	previewContributions                string
	contentScroll                       map[string]float64
	selectedItem                        string
	tradeItems                          []string
	statusControls                      []uiControl
	wrapped                             map[wrapKey][]string
	layout                              ui.Layout
	windowSizes                         windowSizes
	bindings                            bindings
	windows                             []string
	focus, hover                        string
	cursor                              ui.Point
	drag                                *ui.Drag
	resize                              *windowResizeDrag
	editor                              ui.Layout
	hudEditor                           *hudEditorState
	settingsPage                        string
	settingsDraft                       *settings
	bindMode                            string
	bindGroup                           int
	capture, bindingError, selectedNode string
	entrance                            string
	scroll                              map[string]int
	frame                               uiFrame
	previewState                        *protocol.State
	previewPlayer                       string
	previewLanguage                     string
	preview                             damage.Result
	previewValid                        bool
	previewLines                        []string
	statPreview                         stats.Result
	characterDetails                    []string
	expanded                            bool
	phase                               string
}
type wrapKey struct {
	Text        string
	Language    string
	Width, Size float64
}

func (g *Game) cachedWrap(lines []string, width, size float64) []string {
	if len(lines) == 0 {
		return nil
	}
	key := wrapKey{strings.Join(lines, "\n"), g.language, width, size}
	if rows, ok := g.u.wrapped[key]; ok {
		return rows
	}
	if g.u.wrapped == nil || len(g.u.wrapped) > 512 {
		g.u.wrapped = map[wrapKey][]string{}
	}
	g.ensureTextTerms()
	rows := wrapLines(lines, width, size, g.u.textTerms)
	g.u.wrapped[key] = rows
	return rows
}

func (g *Game) initializeInterface() {
	g.width, g.height = 1280, 800
	g.u = clientUI{layout: ui.Layout{}, bindings: defaultBindings(), scroll: map[string]int{}, contentScroll: map[string]float64{}, bindMode: "keyboard"}
	var s settings
	if b, err := os.ReadFile(settingsPath(g.cfg.Profile)); err == nil && json.Unmarshal(b, &s) == nil {
		if s.Version <= 1 && ui.Validate(s.Layout) == nil {
			g.u.layout = ui.Clone(s.Layout)
			delete(g.u.layout, "hover")
		}
		if s.Version <= 1 && s.Bindings != nil && validateBindings(migrateBindings(s.Bindings)) == nil {
			g.u.bindings = migrateBindings(s.Bindings)
		}
		if s.Version <= 1 && validateWindowSizes(s.WindowSizes) == nil {
			g.u.windowSizes = maps.Clone(s.WindowSizes)
		}
		g.mouseMode = s.InputMode == "mouse"
		g.clickDirect = s.ClickDirect
	}
}
func (g *Game) uiBounds() ui.Size { return ui.Size{W: float64(g.width), H: float64(g.height)} }
func (g *Game) refreshPreview() {
	defer g.diagnostics.Since("client.logic.ui_stats", perf.Start())
	if g.u.previewState == g.state && g.u.previewPlayer == g.id && g.u.previewLanguage == g.language {
		return
	}
	g.u.previewState, g.u.previewPlayer, g.u.previewLanguage = g.state, g.id, g.language
	g.u.previewContributions = ""
	g.u.previewValid = false
	g.u.statusControls = nil
	g.u.previewLines = nil
	g.u.characterDetails = nil
	g.u.statPreview = stats.Result{}
	if p := g.self(); p != nil {
		g.u.statPreview, _ = stats.Evaluate(content.StatInput(*p))
		g.u.previewContributions = fmt.Sprint(p.StatContributions)
		g.u.statusControls = g.statusControls(*p)
		criticalMultiplier := 0.0
		q, err := content.Attack(g.catalog, *p)
		if err == nil {
			criticalMultiplier = q.CritMultiplier
			r, err := content.EvaluatePreview(q)
			if err == nil {
				g.u.preview = r
				g.u.previewValid = true
				g.u.previewLines = g.damageLines(r)
			}
		}
		g.u.characterDetails = g.characterDetailLines(*p, g.u.statPreview.Values, criticalMultiplier)
	}
}
func (g *Game) openWindow(id string) {
	if id == "hover" {
		return
	}
	g.u.windows = slices.DeleteFunc(g.u.windows, func(s string) bool { return s == id })
	g.u.windows = append(g.u.windows, id)
	g.u.focus = id
	if id == "settings" {
		s := g.preferences()
		g.u.settingsDraft = &s
		g.u.settingsPage = "home"
		g.u.bindMode = g.inputMode()
		g.u.capture = ""
	}
	if id == "map" {
		g.showMap = true
	}
	if id == "room-settings" {
		s := lobby.CloneConfig(g.front.Config)
		g.front.Draft = &s
		g.front.PortText = strconv.Itoa(s.Port)
		g.front.Editing = ""
		g.front.Name = g.name
		g.front.Chassis = g.chassis
	}
}
func (g *Game) closeWindow(id string) {
	if g.u.resize != nil && g.u.resize.ID == id {
		g.u.resize = nil
	}
	delete(g.u.itemDetails, id)
	delete(g.u.inspections, id)
	delete(g.u.inspectionLayout, id)
	for key := range g.u.formulaOffsets {
		if strings.HasPrefix(key, id+":") {
			delete(g.u.formulaOffsets, key)
		}
	}
	if id == "backpack" {
		g.u.itemMenu = nil
	}
	if id == "backpack" {
		g.returnInventoryCursor()
	}
	g.u.windows = slices.DeleteFunc(g.u.windows, func(s string) bool { return s == id })
	if g.u.focus == id {
		g.u.focus = ""
	}
	if id == "map" {
		g.showMap = false
		g.u.movementMenu = false
	}
	if id == "settings" {
		g.u.settingsDraft = nil
		g.u.capture = ""
		g.u.bindingError = ""
	}
}
func (g *Game) toggleWindow(id string) {
	if slices.Contains(g.u.windows, id) {
		g.closeWindow(id)
	} else {
		g.openWindow(id)
	}
}
func (g *Game) orderedPeers() []model.Player {
	p := g.self()
	if p == nil {
		return nil
	}
	out := []model.Player{}
	for _, v := range g.state.Players {
		if v.ID != g.id {
			out = append(out, v)
		}
	}
	same := func(v model.Player) bool {
		if p.Scene != "" {
			return v.Scene == p.Scene
		}
		return v.Scene == "" && p.Node != "" && v.Node == p.Node
	}
	slices.SortStableFunc(out, func(a, b model.Player) int {
		if same(a) && !same(b) {
			return -1
		}
		if same(b) && !same(a) {
			return 1
		}
		return 0
	})
	return out
}
func (g *Game) sameRoom(v model.Player) bool {
	p := g.self()
	return p != nil && (p.Scene != "" && v.Scene == p.Scene || p.Scene == "" && p.Node != "" && v.Scene == "" && v.Node == p.Node)
}
func (g *Game) windowInfo(id string) (string, []string, []string) {
	p := g.self()
	if p == nil {
		return g.tr("ui." + id), nil, nil
	}
	switch id {
	case "weapon":
		w := g.catalog.Weapon(p.Weapon)
		title := g.contentName(w.ID, w.Name)
		if !g.u.previewValid {
			return title, nil, nil
		}
		short := []string{g.tr("ui.hover.damage", "damage", fmt.Sprintf("%.2f", g.u.preview.Total), "type", g.localized("damage."+g.u.preview.Type, g.u.preview.Type))}
		details := g.u.previewLines
		if scene := g.currentScene(); scene != nil && scene.LastDamage != nil {
			last := scene.LastDamage
			details = append(slices.Clone(details), g.tr("ui.damage.last", "damage", fmt.Sprintf("%.2f", last.Total), "type", g.localized("damage."+last.Type, last.Type), "critical", g.tr("bool."+fmt.Sprint(last.Critical))))
		}
		return title, short, details
	case "penalty":
		return g.tr("ui.status.penalty"), []string{g.tr("ui.status.penalty.short", "layers", p.Downs), g.tr("ui.status.downs.record", "count", p.Knockdowns), g.tr("ui.status.permanent")}, []string{g.tr("ui.status.penalty.calc", "base", fmt.Sprintf("%.2f", p.BaseHP), "layers", p.Downs, "max", fmt.Sprintf("%.2f", p.MaxHP)), g.tr("ui.status.penalty.damage", "layers", p.Downs, "factor", fmt.Sprintf("%.2f", life.PenaltyFactor(p.Downs)))}
	case "curse":
		return g.tr("ui.status.curse"), []string{g.tr("ui.curse"), g.tr("ui.status.permanent")}, []string{g.tr("ui.status.curse.calc")}
	case "armor":
		return g.tr("ui.armor"), []string{g.tr("ui.armor.total", "value", fmt.Sprintf("%.2f", p.Armor))}, nil
	case "backpack":
		return g.tr("ui.backpack"), []string{g.tr("ui.inventory.capacity", "count", inventory.BagCount(p.Inventory), "capacity", p.Inventory.Capacity)}, nil
	case "self":
		lines := []string{g.tr("ui.stats", "name", p.Name, "hp", fmt.Sprintf("%.2f", p.HP), "max_hp", fmt.Sprintf("%.2f", p.MaxHP), "ap", p.AP, "level", p.Level, "xp", p.XP, "coins", p.Coins, "upgrade", p.Upgrade, "life", g.tr("life."+p.Life), "downs", p.Downs, "dodges", p.Dodges)}
		for _, key := range []string{"strength", "agility", "constitution", "intelligence", "sense", "mana"} {
			value, _ := p.Attributes.Value(key)
			lines = append(lines, fmt.Sprintf("%s: %.2f", g.tr("attribute."+key), value))
		}
		lines = append(lines, g.tr("ui.status.downs.record", "count", p.Knockdowns), g.tr("ui.status.downs.current", "count", p.Knockdowns-p.KnockdownBase))
		return p.Name, lines, nil
	}
	if strings.HasPrefix(id, "peer:") {
		for _, v := range g.state.Players {
			if v.ID == strings.TrimPrefix(id, "peer:") {
				return v.Name, []string{g.nodeName(v.Node), g.tr("life." + v.Life), fmt.Sprintf("%.2f / %.2f", v.HP, v.MaxHP), g.tr("ui.ap.personal", "value", v.AP)}, nil
			}
		}
	}
	return g.localized("ui."+id, id), nil, nil
}
func (g *Game) buildInterface() {
	defer g.diagnostics.Since("client.logic.ui_build", perf.Start())
	g.refreshPreview()
	if g.u.entrance != "" {
		n, ok := g.mapNode(g.u.entrance)
		canMove := g.canMapMove(n)
		if g.u.entryShared {
			canMove = g.canTeamMapMove(n)
		}
		if !ok || !canMove {
			g.closeWindow("entrance")
			g.u.entrance = ""
		}
	}
	if g.u.editor == nil && g.state != nil && g.u.phase != g.state.Phase {
		g.u.phase = g.state.Phase
		if g.u.phase == "lobby" || g.u.phase == "preparation" {
			g.openWindow("preparation")
		} else {
			g.closeWindow("preparation")
		}
	}
	f := uiFrame{Titles: map[string]string{}, MapTargets: g.mapDestinations()}
	layout := g.effectiveLayout(g.u.layout)
	if g.u.editor != nil {
		layout = g.effectiveLayout(g.u.editor)
	}
	if g.state == nil && g.u.editor == nil {
		g.buildFrontInterface(&f, layout)
		for _, id := range g.u.windows {
			g.buildWindow(&f, layout, id)
		}
		g.buildItemContextMenu(&f)
		if g.u.hover != "" {
			g.buildHover(&f)
		}
		g.u.frame = f
		return
	}
	if scene := g.currentScene(); g.u.editor == nil && scene != nil {
		g.addDeviceControls(&f, g.worldDevices(g.currentNode(), *scene), g.camera(*scene))
		g.addLootControls(&f, g.camera(*scene))
	}
	if p := g.self(); p != nil {
		g.buildCombatHUD(&f, layout, *p, false)
	} else if g.u.editor != nil {
		g.buildCombatHUD(&f, layout, g.u.hudEditor.Preview, true)
	}
	for _, id := range g.u.windows {
		g.buildWindow(&f, layout, id)
	}
	g.buildItemContextMenu(&f)
	g.buildMovementMenu(&f)
	if g.u.editor != nil {
		g.prepareHUDEditorFrame(&f)
		g.buildEditor(&f)
	}
	if g.u.hover != "" && g.u.editor == nil {
		g.buildHover(&f)
	}
	g.u.frame = f
}
func mathMaxHP(v float64) float64 { return max(1, v) }
