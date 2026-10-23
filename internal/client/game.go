package client

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/persistence"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/localization"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/status"
	"hsmultiplayer/pkg/timing"
)

var bg = color.RGBA{64, 70, 68, 255}
var panel = color.RGBA{36, 44, 47, 242}
var accent = color.RGBA{112, 211, 208, 255}
var muted = color.RGBA{143, 157, 159, 255}

type Config struct {
	ProjectileRenderer string
	ModRoot            string
	Combination        string
	ReloadContent      func(string, *content.Catalog) (*content.Catalog, error)
	BuildContent       func(string) error
	FrameLimit         int
	FPSOverride        bool
	VSync              bool
	VSyncOverride      bool
	InputRate          int
	InterpolationDelay time.Duration
	ShowPerformance    bool
	Metrics            string
	SmokeAutoplay      bool
	SmokeNode          string
	Language           string
	Translations       *localization.Catalog
	Address            string
	Name               string
	NameOverride       bool
	Profile            string
	Host               func(lobby.Config, bool) error
	StopHost           func()
	RoomConfig         lobby.Config
	SmokePage          string
	SmokeAction        string
	SmokeDuration      time.Duration
	SmokeFrames        int
	Screenshot         string
	SmokeState         string
	SmokeMap           bool
	SmokeRotation      float64
	SmokeZoom          float64
	SmokeDetails       bool
	SmokeDetailOffset  int
	SmokeContentOffset int
	SmokeWindow        string
	SmokeHover         string
}
type dialResult struct {
	link *Link
	err  error
}
type Game struct {
	projectilePipeline   projectilePipeline
	renderWork           renderWork
	actorSprites         []actorSprite
	projectileArt        projectileArt
	diagnostics          perf.Recorder
	processSampler       perf.ProcessSampler
	showFPS, showLatency bool
	lastFrameAt          time.Time
	frameSpikes          uint64
	performanceRoom      string
	arenaShapes          map[string]sceneGeometry
	worldHistory         worldHistory
	renderRooms          map[string]protocol.Scene
	itemViews            map[itemViewKey]itemView
	mods                 modManager
	resources            resourceBundle
	adminToken           string
	profiles             persistence.Profiles
	progressionError     error
	extensions           *script.Runtime
	waypoints            []model.Vec
	pathAt               time.Time
	clickDirect          bool
	closed               chan struct{}
	front                frontState
	exitRequested        bool
	autoStart            bool
	hosted               bool
	roomSurfaces         map[string]roomSurface
	prediction           MovementPrediction
	localInput           model.Input
	deviceRing           *ebiten.Image
	selfMarker           *ebiten.Image
	selfMarkerRadius     float64
	u                    clientUI
	width, height        int
	frameLimit           int
	vsync                bool
	pacer                timing.Pacer
	frameMeter           timing.Meter
	snapshotMeter        timing.Meter
	performanceVisible   bool
	performanceStart     time.Time
	nextMetrics          time.Time
	performance          performanceSample
	performanceSamples   []performanceSample
	renderedFrames       int
	interpolatedFrames   int
	lastRenderView       protocol.Scene
	smokeNextAction      time.Time
	catalog              *content.Catalog
	cfg                  Config
	link                 *Link
	pending              chan dialResult
	state                *protocol.State
	id                   string
	tokens               map[string]string
	address              string
	name                 string
	chassis              int
	field                int
	translations         *localization.Catalog
	language             string
	noticeCode           string
	noticeParams         map[string]string
	noticeUntil          time.Time
	cameraControls       cameraControls
	mouseMode            bool
	showMap              bool
	tick                 int
	destination          *model.Vec
	mouseFrames          int
	screenshotDone       bool
	previewOnly          bool
}

func New(c *content.Catalog, cfg Config) *Game {
	if cfg.Name == "" {
		cfg.Name = "Player"
	}
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:7777"
	}
	g := &Game{catalog: c, cfg: cfg, address: cfg.Address, name: cfg.Name, tokens: map[string]string{}, closed: make(chan struct{})}
	g.initializeInterface()
	g.initializeLanguage()
	g.initializeRendering()
	g.initializeFront()
	g.initializeProgression()
	g.initializeMods()
	g.initializeProjectilePipeline()
	if cfg.SmokeFrames > 0 {
		if cfg.SmokeAction != "" {
			g.front.Page = "lobby"
		}
		g.cameraControls.Rotation = cfg.SmokeRotation
		g.cameraControls.Zoom = cfg.SmokeZoom
	}
	if b, err := os.ReadFile(cfg.Profile); err == nil {
		_ = json.Unmarshal(b, &g.tokens)
	}
	if cfg.SmokeState != "" && cfg.SmokeFrames > 0 {
		b, err := os.ReadFile(cfg.SmokeState)
		var s protocol.State
		if err == nil {
			err = json.Unmarshal(b, &s)
		}
		if err == nil && len(s.Players) > 0 {
			g.state = &s
			g.id = s.Players[0].ID
			g.previewOnly = true
			g.showMap = cfg.SmokeMap
			g.cameraControls.Rotation = cfg.SmokeRotation
			g.cameraControls.Zoom = cfg.SmokeZoom
		} else {
			g.notify("notice.render_state")
		}
	}
	if cfg.SmokeFrames > 0 {
		if cfg.SmokeMap {
			g.openWindow("map")
			g.smokeSelectMovement(cfg.SmokeNode)
		}
		if cfg.SmokeWindow == "layout" {
			g.openWindow("settings")
			g.beginHUDLayoutEditor()
			if combatHUDKey(cfg.SmokeNode) != "" {
				g.u.hudEditor.Selected = cfg.SmokeNode
			}
		} else if cfg.SmokeWindow == "item-details" && g.self() != nil {
			if item, ok := inventory.Find(g.self().Inventory, g.self().Inventory.Equipment[0]); ok {
				g.openItemDetails(item)
				if cfg.SmokeDetails {
					for _, view := range g.u.itemDetails {
						g.refreshItemDetails(view)
						if len(view.Equations) > 0 {
							for _, part := range view.Equations[0].Parts {
								if len(part.Children) > 1 {
									g.u.cursor = ui.Point{X: 690, Y: 120}
									g.openInspection(part)
									break
								}
							}
						}
						break
					}
				}
			}
		} else if cfg.SmokeWindow != "" {
			id := cfg.SmokeWindow
			if id == "entrance" {
				if n, ok := g.mapNode(cfg.SmokeNode); ok {
					g.u.entrance = n.ID
				} else if n := g.currentNode(); len(n.Edges) > 0 {
					g.u.entrance = n.Edges[0]
				}
			}
			if id == "language" || id == "bindings" || id == "performance-settings" {
				g.openWindow("settings")
				g.u.settingsPage = strings.TrimSuffix(id, "-settings")
			} else {
				g.openWindow(id)
			}
		}
		g.u.scroll["weapon"] = cfg.SmokeDetailOffset
		if cfg.SmokeWindow != "" {
			g.u.contentScroll[cfg.SmokeWindow] = float64(max(0, cfg.SmokeContentOffset))
		}
		if cfg.SmokePage != "" && !g.previewOnly {
			g.front.Page = cfg.SmokePage
		}
	}
	return g
}
func (g *Game) Close() {
	if g.extensions != nil {
		g.extensions.Close()
		g.extensions = nil
	}
	if g.resources.music != nil {
		g.resources.music.Close()
	}
	for _, p := range g.resources.playing {
		p.Close()
	}
	for _, value := range g.resources.images {
		value.Deallocate()
	}
	for _, value := range g.resources.glyphs {
		value.Deallocate()
	}
	for _, value := range g.resources.boxes {
		value.Deallocate()
	}
	if g.front.Surface.Image != nil {
		g.front.Surface.Image.Deallocate()
	}
	select {
	case <-g.closed:
		return
	default:
		close(g.closed)
	}
	if g.cfg.StopHost != nil {
		g.cfg.StopHost()
	}
	for _, s := range g.roomSurfaces {
		s.Image.Dispose()
	}
	if g.deviceRing != nil {
		g.deviceRing.Dispose()
	}
	if g.selfMarker != nil {
		g.selfMarker.Dispose()
	}
	g.releaseProjectileArt()
	g.releaseProjectilePipeline()
	g.saveMetrics()
	if g.link != nil {
		g.link.Close()
	}
}
func (g *Game) connect() {
	if g.frontBusy() {
		return
	}
	g.autoStart = false
	g.connectWithRetry(false, nil)
}
func (g *Game) resetConnectionView() {
	g.u.movementItem, g.u.movementMenu = "", false
	g.u.arrivalNode, g.u.arrivalUntil = "", time.Time{}
	if g.link != nil {
		g.link.Close()
		g.link = nil
	}
	g.state = nil
	g.prediction = MovementPrediction{}
	g.cameraControls.resetRotationInput()
	g.cameraControls.MapOffset = model.Vec{}
	g.cameraControls.MiniOffset = model.Vec{}
	g.localInput = model.Input{}
	g.u.windows = nil
	g.clearInspections()
	g.u.phase = ""
	g.u.hover = ""
	g.worldHistory.Clear()
	g.id = ""
	g.noticeCode = "notice.connecting"
	g.noticeParams = nil
	g.noticeUntil = time.Time{}
}
func (g *Game) command(kind, node string, accept bool) {
	if g.link == nil {
		return
	}
	g.link.Send(protocol.Request{Kind: kind, Node: node, Accept: accept})
}
func (g *Game) notify(key string, pairs ...any) {
	g.noticeCode = key
	g.noticeParams = params(pairs...)
	g.noticeUntil = time.Now().Add(8 * time.Second)
}
func (g *Game) Update() error {
	defer g.diagnostics.Since("client.logic.total", perf.Start())
	g.pollMods()
	g.updateAudio()
	if g.exitRequested {
		return ebiten.Termination
	}
	g.tick++
	if g.cfg.SmokeFrames > 0 && (g.tick > g.cfg.SmokeFrames || g.cfg.SmokeDuration > 0 && !g.performanceStart.IsZero() && time.Since(g.performanceStart) >= g.cfg.SmokeDuration) {
		return ebiten.Termination
	}
	if g.previewOnly {
		g.updateInterface()
		return nil
	}
	if g.cfg.SmokeAutoplay && g.cfg.SmokeFrames > 0 && g.link == nil && g.pending == nil && g.state == nil {
		g.connect()
	}
	if g.pending != nil {
		select {
		case r := <-g.pending:
			g.pending = nil
			if r.err != nil {
				g.notifyError(r.err)
				g.autoStart = false
				if g.hosted && g.cfg.StopHost != nil {
					g.cfg.StopHost()
					g.hosted = false
				}
			} else {
				g.link = r.link
			}
		default:
		}
	}
	if g.link != nil {
		networkStart := perf.Start()
	drain:
		for {
			select {
			case received := <-g.link.Messages:
				m := received.Message
				switch m.Kind {
				case "checkpoint":
					g.receiveCheckpoint(m.Export)
				case "welcome":
					g.id = m.Player
					g.tokens[g.address] = m.Token
					_ = os.MkdirAll(filepath.Dir(g.cfg.Profile), 0755)
					b, _ := json.MarshalIndent(g.tokens, "", "  ")
					if err := os.WriteFile(g.cfg.Profile, b, 0600); err != nil {
						g.notify("notice.identity_save", "detail", err.Error())
					} else {
						g.noticeCode = ""
					}
				case "state":
					g.receiveState(m.State, received.At)
					g.dispatchClientSnapshot()
					if g.autoStart && g.state != nil && g.state.Phase == "lobby" && g.state.Admin == g.id {
						g.autoStart = false
						g.command("start", "", false)
					}
				case "error":
					g.u.mouseInventory = nil
					g.notifyMessage(m)
				case "disconnected", "replaced":
					g.u.returnCursorPending = false
					g.u.mouseInventory = nil
					g.worldHistory.Clear()
					// Preserve a handshake rejection instead of replacing it with EOF.
					if m.Kind == "replaced" || !strings.HasPrefix(g.noticeCode, "error.") || time.Now().After(g.noticeUntil) {
						g.notifyMessage(m)
					}
					g.link.Close()
					g.link = nil
					g.state = nil
					g.u.windows = nil
					g.clearInspections()
					g.u.phase = ""
					g.u.hover = ""
					g.front.Page = "lobby"
					g.autoStart = false
					if g.hosted && g.cfg.StopHost != nil {
						g.cfg.StopHost()
					}
					g.hosted = false
				}
			default:
				break drain
			}
			if g.link == nil {
				break
			}
		}
		g.diagnostics.Since("client.logic.network_apply", networkStart)
	}
	g.smokeActions()
	g.smokeFrontActions()
	if g.u.returnCursorPending {
		g.returnInventoryCursor()
	}
	mouseConsumed, keysConsumed := g.updateInterface()
	if g.updateCameraControls(mouseConsumed, keysConsumed, time.Now()) {
		mouseConsumed = true
	}
	if !keysConsumed {
		if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
			g.cycleFPS()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
			g.toggleVSync()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF6) {
			g.performanceVisible = !g.performanceVisible
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
			g.switchLanguage()
		}
	}
	if g.state == nil || (g.link == nil && !g.previewOnly) {
		g.updateFront(mouseConsumed, keysConsumed)
	} else {
		if !keysConsumed && inpututil.IsKeyJustPressed(ebiten.KeyF2) {
			g.mouseMode = !g.mouseMode
			g.destination = nil
			if err := g.saveSettings(); err != nil {
				g.notify("notice.settings_save", "detail", err.Error())
			}
		}
		if !keysConsumed && bindingAvailable(g.bound("map"), mouseConsumed) && bindingPressed(g.bound("map")) {
			g.toggleWindow("map")
		}
		p := g.self()
		if p != nil {
			in := model.Input{}
			s := g.currentScene()
			if s != nil {
				mx, my := ebiten.CursorPosition()
				in.Aim = g.camera(*s).Unproject(model.Vec{X: float64(mx), Y: float64(my)})
				if !keysConsumed {
					g.actionKeys(in.Aim, mouseConsumed)
					if bindingAvailable(g.bound("interact"), mouseConsumed) && bindingPressed(g.bound("interact")) {
						if drop, ok := g.nearLoot(); ok {
							g.sendItem(protocol.Request{Kind: "pickup", Item: drop.Item.ID})
						} else if d, ok := g.nearInteraction(); ok {
							g.inspectInteraction(d.Target)
						}
					}
					in.Attack = !mouseConsumed && bindingDown(g.bound("attack"))
					if g.mouseMode {
						if mouseConsumed {
							g.mouseFrames = 0
							g.destination = nil
						}
						if !mouseConsumed && bindingDown(g.bound("move")) {
							g.mouseFrames++
							g.destination = nil
							in.Move = in.Aim.Sub(p.Position).Unit()
						} else if g.mouseFrames > 0 {
							if g.mouseFrames < 10 {
								target := g.sceneArena(*s).ClampDisk(in.Aim, 12)
								g.destination = &target
								g.waypoints = nil
								g.pathAt = time.Time{}
							}
							g.mouseFrames = 0
						}
						if g.destination != nil {
							d := g.navigationDirection(*p, *s)
							if d.Length() > 5 {
								in.Move = d.Unit()
							} else {
								g.destination = nil
							}
						}
					} else {
						if bindingAvailable(g.bound("up"), mouseConsumed) && bindingDown(g.bound("up")) {
							in.Move.Y--
						}
						if bindingAvailable(g.bound("down"), mouseConsumed) && bindingDown(g.bound("down")) {
							in.Move.Y++
						}
						if bindingAvailable(g.bound("left"), mouseConsumed) && bindingDown(g.bound("left")) {
							in.Move.X--
						}
						if bindingAvailable(g.bound("right"), mouseConsumed) && bindingDown(g.bound("right")) {
							in.Move.X++
						}
						in.Move = g.camera(*s).WorldDirection(in.Move)
					}
					in.Dodge = bindingAvailable(g.bound("dodge"), mouseConsumed) && bindingPressed(g.bound("dodge"))
				} else {
					g.mouseFrames = 0
					g.destination = nil
				}
				if g.cfg.SmokeAutoplay && g.cfg.SmokeFrames > 0 {
					in = g.smokeInput(*s)
				}
				g.link.SetInput(in)
				g.localInput = in
				if s.State != "paused" && s.State != "faulted" && s.State != "cleared" && g.state.Phase != "won" && g.state.Phase != "lost" {
					g.prediction = g.prediction.BeginDodge(in, *p)
				}
			} else {
				g.link.ClearInput()
				g.localInput = model.Input{}
			}
		} else {
			g.link.ClearInput()
			g.localInput = model.Input{}
		}
	}
	g.buildInterface()
	return nil
}
func (g *Game) Layout(w, h int) (int, int) {
	g.width = max(640, w)
	g.height = max(480, h)
	return g.width, g.height
}
func rect(dst *ebiten.Image, x, y, w, h float64, c color.Color) {
	vector.FillRect(dst, float32(x), float32(y), float32(w), float32(h), c, false)
}
func line(dst *ebiten.Image, a, b model.Vec, c color.Color, width float32) {
	vector.StrokeLine(dst, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), width, c, true)
}
func (g *Game) Draw(dst *ebiten.Image) {
	paceStart := perf.Start()
	now := g.pacer.Wait()
	g.diagnostics.Since("client.render.pacing_wait", paceStart)
	defer g.diagnostics.Since("client.render.cpu_total", perf.Start())
	if !g.lastFrameAt.IsZero() {
		interval := now.Sub(g.lastFrameAt)
		g.diagnostics.Observe("client.frame.interval", interval)
		if interval > max(33*time.Millisecond, 2*time.Second/time.Duration(max(1, g.frameLimit))) && g.frameLimit > 0 || g.frameLimit == 0 && interval > 33*time.Millisecond {
			g.frameSpikes++
		}
	}
	g.lastFrameAt = now
	g.observeFrame(now)
	g.renderWork = renderWork{ProjectileRenderer: "sprites"}
	if g.projectilePipeline.shader != nil {
		g.renderWork.ProjectileRenderer = "batch"
	}
	g.warmProjectilePipeline()
	col := bg
	if v := g.resources.theme.Background; v[3] != 0 {
		col = color.RGBA{v[0], v[1], v[2], v[3]}
	}
	dst.Fill(col)
	if g.u.editor != nil && g.state == nil {
		// HUD editing from the menu has its own neutral preview background.
	} else if g.state == nil || (g.link == nil && !g.previewOnly) {
		g.drawFront(dst)
	} else {
		g.drawGame(dst)
	}
	uiStart := perf.Start()
	g.drawInterface(dst)
	g.diagnostics.Since("client.render.ui", uiStart)
	if g.u.editor == nil {
		g.drawPerformanceHUD(dst)
	}

	if g.noticeCode != "" && (g.noticeUntil.IsZero() || time.Now().Before(g.noticeUntil)) {
		rect(dst, 12, 92, float64(g.width-244), 30, color.RGBA{63, 35, 43, 255})
		g.fitText(dst, g.translations.Text(g.language, g.noticeCode, g.noticeParams), 20, 96, float64(g.width-260), 14, false)
	}
	if g.cfg.Screenshot != "" && !g.screenshotDone && g.tick > 10 && (g.cfg.SmokeAction == "" || g.state != nil && (g.cfg.SmokeAction != "solo" || g.state.Phase == "exploring")) && (g.cfg.Metrics == "" || now.Sub(g.performanceStart) > 4*time.Second) {
		f, err := os.Create(g.cfg.Screenshot)
		if err == nil {
			pixels := image.NewRGBA(dst.Bounds())
			dst.ReadPixels(pixels.Pix)
			err = png.Encode(f, pixels)
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			g.notifyError(err)
		}
		g.screenshotDone = true
	}
}
func (g *Game) self() *model.Player {
	if g.state == nil {
		return nil
	}
	for i := range g.state.Players {
		if g.state.Players[i].ID == g.id {
			return &g.state.Players[i]
		}
	}
	return nil
}
func (g *Game) currentScene() *protocol.Scene {
	p := g.self()
	if p == nil {
		return nil
	}
	for i := range g.state.Scenes {
		if g.state.Scenes[i].ID == p.Scene {
			return &g.state.Scenes[i]
		}
	}
	return g.spaceView()
}
func (g *Game) drawGame(dst *ebiten.Image) {
	defer g.diagnostics.Since("client.render.world", perf.Start())
	if p := g.self(); p == nil {
		return
	}
	if sc := g.currentScene(); sc != nil {
		view := *sc
		interpolationStart := perf.Start()
		g.renderRooms = g.worldHistory.View(time.Now(), g.cfg.InterpolationDelay)
		g.diagnostics.Since("client.render.interpolation", interpolationStart)
		if interpolated, ok := g.renderRooms[g.currentNode().ID]; ok {
			view = interpolated
		}
		if p := g.self(); p != nil {
			a := g.sceneArena(*sc)
			if !g.prediction.Valid {
				g.prediction = g.prediction.Reconcile(*p, a, time.Now())
			}
			movable := p.Life == "alive" && (sc.State == "idle" && g.state.Phase != "won" && g.state.Phase != "lost" || sc.State == "fighting" || sc.State == "waiting" || sc.State == "between_waves")
			if status.Controlled(p.Statuses, "stun") || status.Controlled(p.Statuses, "root") {
				movable = false
			}
			previous := g.prediction.Position
			g.prediction = g.prediction.Advance(g.localInput, p.Speed, a, time.Now(), movable)
			if !g.clearWalk(*sc, previous, g.prediction.Position) {
				g.prediction.Position = previous
			}
			for i := range view.Players {
				if view.Players[i].ID == g.id {
					view.Players[i] = *p
					view.Players[i].Position = g.prediction.Position
				}
			}
		}
		if g.lastRenderView.ID == view.ID && g.lastRenderView.Tick == view.Tick && displayPositionsChanged(g.lastRenderView, view) {
			g.interpolatedFrames++
		}
		g.lastRenderView = view
		g.drawBattle(dst, view)
	} else {
		p := g.self()
		g.drawText(dst, g.nodeName(p.Node), float64(g.width)/2-120, float64(g.height)/2-40, 18)
		if g.state.Phase == "won" || g.state.Phase == "lost" {
			g.drawText(dst, g.tr("ui.finished.title", "phase", g.tr("phase."+g.state.Phase)), 30, 150, 18)
		}
	}
}
func hexagon(dst *ebiten.Image, c model.Vec, r float64, col color.Color, rotation float64) {
	var p vector.Path
	for i := 0; i < 6; i++ {
		a := math.Pi/3*float64(i) + rotation
		x, y := float32(c.X+r*math.Cos(a)), float32(c.Y+r*math.Sin(a))
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].ColorR = float32(col.(color.RGBA).R) / 255
		vs[i].ColorG = float32(col.(color.RGBA).G) / 255
		vs[i].ColorB = float32(col.(color.RGBA).B) / 255
		vs[i].ColorA = 1
		vs[i].SrcX = 1
		vs[i].SrcY = 1
	}
	dst.DrawTriangles(vs, is, whiteImage, nil)
}

var whiteImage = func() *ebiten.Image { i := ebiten.NewImage(3, 3); i.Fill(color.White); return i }()

func (g *Game) camera(s protocol.Scene) Camera {
	return g.worldCamera()
}
func (g *Game) drawBattle(dst *ebiten.Image, s protocol.Scene) {
	g.drawNeighborhood(dst, s)
	c := g.camera(s)
	if s.State == "waiting" || s.State == "between_waves" {
		v := c.Project(g.sceneArena(s).Center)
		g.fitText(dst, g.tr("ui.wave.countdown", "seconds", int(math.Ceil(s.Countdown))), v.X-200, v.Y-68, 400, 26, true)
	}
	if s.State == "paused" {
		g.fitText(dst, g.tr("ui.paused"), float64(g.width)/2-220, 80, 440, 16, true)
	}
	if s.State == "fighting" && s.ObjectiveRemaining > 0 {
		g.fitText(dst, g.tr("ui.objective.survive", "seconds", int(math.Ceil(s.ObjectiveRemaining))), float64(g.width)/2-220, 80, 440, 16, true)
	}
	if s.Error != "" {
		g.fitText(dst, g.tr("ui.script_error", "detail", s.Error), 18, 230, float64(g.width)-270, 14, false)
	}
}
func (g *Game) drawRoomEntities(dst *ebiten.Image, s protocol.Scene, c Camera) {
	defer g.diagnostics.Since("client.render.actors", perf.Start())
	if p := g.self(); p != nil {
		for _, list := range g.lootLists() {
			for _, drop := range list {
				if drop.Node == strings.TrimPrefix(s.ID, "space:") {
					v := c.Project(drop.Position)
					hexagon(dst, v, 7*c.Scale, color.RGBA{100, 230, 170, 255}, c.Angle)
					if g.resources.images["icon:"+drop.Item.Definition] != nil {
						g.drawIconSized(dst, drop.Item.Definition, v.X-11*c.Scale, v.Y-18*c.Scale, 22*c.Scale)
					}
				}
			}
		}
	}
	g.drawSortedActors(dst, s, c)
	g.drawProjectiles(dst, s.Projectiles, c)
}
