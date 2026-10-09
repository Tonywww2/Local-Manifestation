package client

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"slices"
	"strconv"
	"strings"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
)

type uiControl struct {
	InventoryStatus                            *inventoryStatusCard
	ID, Owner, Icon, Key, Action, Value, Title string
	Badge                                      string
	Caption                                    string
	Count                                      int
	Bounds                                     ui.Rect
	Clip                                       ui.Rect
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
	Viewports map[string]windowViewport
	Panels    []ui.Panel
	Titles    map[string]string
	Controls  []uiControl
	Labels    []uiLabel
	Meters    []uiMeter
	Dots      []uiDot
	Hints     []uiControl
	Tooltip   *uiTooltip
}
type uiTooltip struct {
	Bounds   ui.Rect
	Title    string
	Rows     []string
	RichRows [][]tooltipSpan
}
type clientUI struct {
	returnCursorPending                 bool
	hoverStyles                         map[hoverStyleKey][][]tooltipSpan
	mouseInventory                      *model.Inventory
	previewContributions                string
	contentScroll                       map[string]float64
	selectedItem                        string
	tradeItems                          []string
	statusControls                      []uiControl
	wrapped                             map[wrapKey][]string
	layout                              ui.Layout
	bindings                            bindings
	windows                             []string
	focus, hover                        string
	cursor                              ui.Point
	drag                                *ui.Drag
	editor                              ui.Layout
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
	Width, Size float64
}

func (g *Game) cachedWrap(lines []string, width, size float64) []string {
	if len(lines) == 0 {
		return nil
	}
	key := wrapKey{strings.Join(lines, "\n"), width, size}
	if rows, ok := g.u.wrapped[key]; ok {
		return rows
	}
	if g.u.wrapped == nil || len(g.u.wrapped) > 512 {
		g.u.wrapped = map[wrapKey][]string{}
	}
	rows := wrapLines(lines, width, size)
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
		g.mouseMode = s.InputMode == "mouse"
		g.clickDirect = s.ClickDirect
	}
}
func (g *Game) uiBounds() ui.Size { return ui.Size{W: float64(g.width), H: float64(g.height)} }
func (g *Game) refreshPreview() {
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
	if id == "backpack" {
		g.returnInventoryCursor()
	}
	g.u.windows = slices.DeleteFunc(g.u.windows, func(s string) bool { return s == id })
	if g.u.focus == id {
		g.u.focus = ""
	}
	if id == "map" {
		g.showMap = false
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
		lines := []string{g.tr("ui.stats", "name", p.Name, "hp", fmt.Sprintf("%.2f", p.HP), "max_hp", fmt.Sprintf("%.2f", p.MaxHP), "ap", p.AP, "level", p.Level, "xp", p.XP, "coins", p.Coins, "upgrade", p.Upgrade, "life", g.tr("life."+p.Life), "downs", p.Downs, "dodges", p.Dodges), g.tr("ui.points", "count", p.Points)}
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
	g.refreshPreview()
	if g.u.entrance != "" {
		if door, ok := g.nearbyDoor(); !ok || door.ID != g.u.entrance {
			g.closeWindow("entrance")
			g.u.entrance = ""
		}
	}
	if g.state != nil && g.u.phase != g.state.Phase {
		g.u.phase = g.state.Phase
		if g.u.phase == "lobby" || g.u.phase == "preparation" {
			g.openWindow("preparation")
		} else {
			g.closeWindow("preparation")
		}
	}
	f := uiFrame{Titles: map[string]string{}}
	layout := g.u.layout
	if len(g.resources.theme.Layout) > 0 {
		layout = ui.Clone(g.resources.theme.Layout)
		for id, p := range g.u.layout {
			layout[id] = p
		}
	}
	if g.u.editor != nil {
		layout = g.u.editor
	}
	if g.state == nil && g.u.editor == nil {
		g.buildFrontInterface(&f, layout)
		for _, id := range g.u.windows {
			g.buildWindow(&f, layout, id)
		}
		if g.u.hover != "" {
			g.buildHover(&f)
		}
		g.u.frame = f
		return
	}
	if scene := g.currentScene(); scene != nil {
		g.addDeviceControls(&f, g.worldDevices(g.currentNode(), *scene), g.camera(*scene))
	}
	bounds := g.uiBounds()
	addPanel := func(id, title string, size ui.Size, p ui.Placement, floating bool) ui.Rect {
		r := ui.At(layout, id, p, size, bounds)
		f.Panels = append(f.Panels, ui.Panel{ID: id, Bounds: r, Floating: floating})
		f.Titles[id] = title
		return r
	}
	label := func(r ui.Rect, value string) {
		f.Labels = append(f.Labels, uiLabel{Owner: f.Panels[len(f.Panels)-1].ID, Bounds: r, Text: value, Size: 12})
	}
	control := func(owner, id, icon, key, action, value, title string, r ui.Rect, summary, details []string) {
		f.Controls = append(f.Controls, uiControl{ID: id, Owner: owner, Icon: icon, Key: key, Action: action, Value: value, Title: title, Bounds: r, Summary: summary, Details: details})
	}
	if p := g.self(); p != nil {
		for gi, group := range []string{"skills", "consumables"} {
			r := addPanel(group, g.tr("ui."+group), ui.Size{W: 248, H: 60}, ui.Placement{Anchor: "top_left", X: 18, Y: 18 + float64(gi)*74}, false)
			for i := 0; i < 4; i++ {
				prefix := "skill"
				if gi == 1 {
					prefix = "item"
				}
				id := fmt.Sprintf("%s%d", prefix, i+1)
				control(group, id, "empty", g.bindingLabel(g.bound(id)), "", "", g.tr("ui.action."+id), ui.Rect{X: r.X + float64(i)*62, Y: r.Y, W: 56, H: 60}, []string{g.tr("ui.slot.empty")}, nil)
				index := len(f.Controls) - 1
				if gi == 1 {
					it, _ := inventory.Find(p.Inventory, p.Inventory.Equipment[5+i])
					c := g.itemControl(group, id, it, f.Controls[index].Bounds, "item-use")
					c.Value = strconv.Itoa(i)
					c.Key = f.Controls[index].Key
					f.Controls[index] = c
				} else if d, ok := g.catalog.Skill(p.Skills[i]); ok {
					c := &f.Controls[index]
					c.Icon = skillIcon(d.ID)
					c.Title = g.contentName(d.ID, d.Name)
					c.Action = "skill-use"
					c.Value = strconv.Itoa(i)
					c.Summary = []string{g.localized("content."+d.ID+".description", d.Description), g.tr("ui.skill.cost", "cost", d.Cost, "seconds", float64(d.CooldownMillis)/1000)}
					if remaining := float64(p.SkillReady[i]-p.Statuses.Time) / float64(status.UnitsPerSecond); remaining > 0 {
						c.Badge = fmt.Sprintf("%.1f", remaining)
					}
				}
			}
		}
		journeyPlacement, journeyWidth := ui.Placement{Anchor: "top_center", Y: 18}, 260.0
		if g.width < 820 {
			journeyPlacement = ui.Placement{Anchor: "top_left", X: 18, Y: 166}
			journeyWidth = 208
		}
		r := addPanel("journey", g.tr("ui.journey"), ui.Size{W: journeyWidth, H: 42}, journeyPlacement, false)
		label(ui.Rect{X: r.X + 10, Y: r.Y + 5, W: r.W - 20, H: 18}, g.tr("ui.journey.floor", "floor", g.state.Floor, "floors", g.state.Floors))
		label(ui.Rect{X: r.X + 10, Y: r.Y + 23, W: r.W - 20, H: 16}, g.nodeName(p.Node))
		control("journey", "journey", "map", "", "", "", g.tr("ui.journey"), r, []string{g.tr("phase." + g.state.Phase)}, nil)
		if s := g.currentScene(); s != nil && s.Waves > 0 {
			f.Controls[len(f.Controls)-1].Summary = append(f.Controls[len(f.Controls)-1].Summary, g.tr("ui.wave.progress", "wave", s.Wave, "waves", s.Waves))
			f.Labels[len(f.Labels)-1].Text = g.tr("ui.wave.progress", "wave", s.Wave, "waves", s.Waves)
		}
		r = addPanel("minimap", g.tr("ui.map"), ui.Size{W: 224, H: 152}, ui.Placement{Anchor: "top_right", X: -18, Y: 18}, false)
		label(ui.Rect{X: r.X + 10, Y: r.Y + r.H - 25, W: r.W - 20, H: 20}, g.tr("ui.minimap.ap", "personal", p.AP, "shared", g.state.SharedAP))
		control("minimap", "minimap", "map", g.bindingLabel(g.bound("map")), "window", "map", g.tr("ui.map"), r, nil, nil)
		r = addPanel("view", g.tr("ui.view"), ui.Size{W: 90, H: 36}, ui.Placement{Anchor: "top_right", X: -152, Y: 180}, false)
		view := "view.top_down"
		if g.oblique {
			view = "view.oblique"
		}
		control("view", "view", "view", g.bindingLabel(g.bound("view")), "view", "", g.tr("ui.view"), r, []string{g.tr(view)}, nil)
		peers := g.orderedPeers()
		rows := min(len(peers), max(1, (g.height-356)/52))
		r = addPanel("party", g.tr("ui.team"), ui.Size{W: 224, H: float64(max(1, rows) * 52)}, ui.Placement{Anchor: "top_right", X: -18, Y: 226}, false)
		start := min(g.u.scroll["party"], max(0, len(peers)-rows))
		for i, v := range peers[start:min(start+rows, len(peers))] {
			row := ui.Rect{X: r.X, Y: r.Y + float64(i)*52, W: r.W, H: 46}
			title, short, detail := g.windowInfo("peer:" + v.ID)
			control("party", "peer:"+v.ID, "person", "", "window", "peer:"+v.ID, title, row, short, detail)
		}
		cardWidth := min(264, float64(g.width)*.325)
		r = addPanel("resources", g.tr("ui.resources"), ui.Size{W: min(400, float64(g.width)-2*(cardWidth+30)), H: 50}, ui.Placement{Anchor: "bottom_center", Y: -18}, false)
		barWidth := r.W - 60
		healthText := fmt.Sprintf("%.2f / %.2f", p.HP, p.MaxHP)
		f.Meters = append(f.Meters, uiMeter{Owner: "resources", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 3, W: barWidth, H: 19}, Fraction: p.HP / mathMaxHP(p.MaxHP)})
		if labelFits(healthText, barWidth-12, 12) {
			f.Labels = append(f.Labels, uiLabel{Owner: "resources", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 3, W: barWidth, H: 19}, Text: healthText, Size: 12, Centered: true})
		}
		if shield := status.ShieldTotal(p.Statuses); shield > 0 {
			f.Meters = append(f.Meters, uiMeter{Owner: "resources", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 25, W: barWidth, H: 3}, Fraction: min(1, shield/max(1, p.MaxHP)), Kind: "shield"})
		}
		f.Meters = append(f.Meters, uiMeter{Owner: "resources", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 32, W: barWidth, H: 12}, Fraction: p.Energy / max(1, p.MaxEnergy), Kind: "energy"})
		f.Labels = append(f.Labels, uiLabel{Owner: "resources", Bounds: ui.Rect{X: r.X + 4, Y: r.Y + 29, W: barWidth, H: 16}, Text: fmt.Sprintf("%.0f / %.0f", p.Energy, p.MaxEnergy), Size: 11, Centered: true})
		for i := 0; i < 2; i++ {
			f.Dots = append(f.Dots, uiDot{Owner: "resources", Center: ui.Point{X: r.X + r.W - 32 + float64(i)*21, Y: r.Y + 13}, Available: i < p.Dodges})
		}
		title, short, detail := g.windowInfo("self")
		control("resources", "self", "heart", "", "", "", title, r, short, detail)
		r = addPanel("statuses", g.tr("ui.statuses"), ui.Size{W: min(348, float64(g.width)-2*(cardWidth+30)), H: 48}, ui.Placement{Anchor: "bottom_center", X: 0, Y: -80}, false)
		statusControlStart := len(f.Controls)
		i := 0
		for _, id := range []string{"penalty", "curse"} {
			if id == "penalty" && p.Downs == 0 || id == "curse" && !g.state.Cursed {
				continue
			}
			title, short, detail := g.windowInfo(id)
			control("statuses", id, "status", "", "", "", title, ui.Rect{X: r.X + float64(i)*54, Y: r.Y, W: 48, H: 46}, short, detail)
			f.Controls[len(f.Controls)-1].Badge = "∞"
			i++
		}
		statusSlots := max(3, int((r.W+6)/54)) - i
		moreStatuses := len(g.u.statusControls) > statusSlots
		if moreStatuses {
			statusSlots--
		}
		statusStart := min(g.u.scroll["statuses"], max(0, len(g.u.statusControls)-statusSlots))
		for _, status := range g.u.statusControls[statusStart:min(statusStart+statusSlots, len(g.u.statusControls))] {
			status.Bounds = ui.Rect{X: r.X + float64(i)*54, Y: r.Y, W: 48, H: 46}
			f.Controls = append(f.Controls, status)
			i++
		}
		if moreStatuses {
			summary := []string{g.tr("ui.status.scroll")}
			for _, row := range g.u.statusControls {
				summary = append(summary, row.Title+" · "+row.Badge)
			}
			control("statuses", "statuses:more", "status", "", "", "", g.tr("ui.status.more"), ui.Rect{X: r.X + float64(i)*54, Y: r.Y, W: 48, H: 46}, summary, nil)
			f.Controls[len(f.Controls)-1].Badge = fmt.Sprintf("+%d", len(g.u.statusControls)-statusSlots)
		}
		statusOffset := (r.W - float64(len(f.Controls)-statusControlStart)*54 + 6) / 2
		for j := statusControlStart; j < len(f.Controls); j++ {
			f.Controls[j].Bounds.X += statusOffset
		}
		r = addPanel("equipment", g.tr("ui.backpack_status"), ui.Size{W: cardWidth, H: 98}, ui.Placement{Anchor: "bottom_right", X: -18, Y: -18}, false)
		f.Controls = append(f.Controls, g.inventoryStatusControl(r, *p))
		if d, ok := g.nearInteraction(); ok {
			r = addPanel("entrance-prompt", "", ui.Size{W: 320, H: 44}, ui.Placement{Anchor: "bottom_center", Y: -140}, false)
			label(ui.Rect{X: r.X + 8, Y: r.Y + 4, W: 250, H: 36}, g.deviceName(d))
			control("entrance-prompt", "entrance-open", "door", g.bindingLabel(g.bound("interact")), "world-device", d.ID, g.deviceName(d), ui.Rect{X: r.X + 268, Y: r.Y + 4, W: 44, H: 36}, nil, []string{g.deviceDescription(d)})
		}
	}
	r := addPanel("tools", "", ui.Size{W: 92, H: 38}, ui.Placement{Anchor: "bottom_left", X: 18, Y: -18}, false)
	control("tools", "settings", "settings", g.bindingLabel(g.bound("settings")), "window", "settings", g.tr("ui.settings"), ui.Rect{X: r.X, Y: r.Y, W: 38, H: 38}, nil, nil)
	if g.self() != nil && g.performanceVisible {
		control("tools", "developer", "tools", "F7", "window", "developer", g.tr("ui.developer"), ui.Rect{X: r.X + 46, Y: r.Y, W: 38, H: 38}, nil, nil)
	}
	for _, id := range g.u.windows {
		g.buildWindow(&f, layout, id)
	}
	if g.u.editor != nil {
		g.buildEditor(&f)
	}
	if g.u.hover != "" {
		g.buildHover(&f)
	}
	g.u.frame = f
}
func mathMaxHP(v float64) float64 { return max(1, v) }
