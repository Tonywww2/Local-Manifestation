package client

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hsmultiplayer/pkg/perf"
	"image/color"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/device"
	"hsmultiplayer/pkg/lobby"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

type frontState struct {
	Actions    hallActions
	Name       string
	Chassis    int
	PortText   string
	Editing    string
	Page       string
	Position   model.Vec
	Devices    []device.Device
	Config     lobby.Config
	Draft      *lobby.Config
	Surface    roomSurface
	Camera     Camera
	CameraSize ui.Size
}

func (g *Game) initializeFront() {
	options := g.cfg.RoomConfig
	if options.Validate() != nil {
		options = lobby.DefaultConfig()
	}
	var saved settings
	if b, err := os.ReadFile(settingsPath(g.cfg.Profile)); err == nil && json.Unmarshal(b, &saved) == nil && saved.Room != nil && saved.Room.Validate() == nil {
		options = *saved.Room
		if !g.cfg.NameOverride && saved.PlayerName != "" && len([]rune(saved.PlayerName)) <= 24 {
			g.name = saved.PlayerName
		}
		for i, ch := range g.catalog.ChassisList() {
			if ch.ID == saved.Chassis {
				g.chassis = i
				break
			}
		}
	}
	g.front = frontState{Page: "main", Position: lobby.Spawn(), Config: options, Devices: lobby.Devices()}
}

func (g *Game) hallCamera() Camera {
	c := g.hallBaseCamera()
	c.Angle = g.cameraAngle()
	c.Scale *= g.cameraZoom("world")
	c.Origin = model.Vec{}
	c.Origin = model.Vec{X: float64(g.width) / 2, Y: float64(g.height)/2 + 20}.Sub(c.Project(g.front.Position))
	return c
}

func (g *Game) hallBaseCamera() Camera {
	if g.front.CameraSize == g.uiBounds() && g.front.Camera.Scale > 0 {
		return g.front.Camera
	}
	var points []model.Vec
	for _, cell := range lobby.Footprint().Cells() {
		vertices := cell.Vertices()
		points = append(points, vertices[:]...)
	}
	g.front.Camera = fitPoints(points, defaultCameraAngle, ui.Rect{X: 24, Y: 105, W: float64(g.width - 48), H: float64(g.height - 180)})
	// The larger base remains walkable at a useful local scale; the follow
	// camera reveals the outer stations as the player explores or zooms out.
	g.front.Camera.Scale = max(g.front.Camera.Scale, .7*min(float64(g.width)/1280, float64(g.height)/800))
	g.front.CameraSize = g.uiBounds()
	return g.front.Camera
}

func (g *Game) drawFront(dst *ebiten.Image) {
	defer g.diagnostics.Since("client.render.front", perf.Start())
	if g.front.Page != "lobby" {
		g.drawMainBackdrop(dst)
		return
	}
	c := g.hallCamera()
	cacheCamera := Camera{Scale: g.hallBaseCamera().Scale}
	s := protocol.Scene{Width: lobby.Width, Height: lobby.Height}
	if !matchingSurface(g.front.Surface, s, cacheCamera) {
		if g.front.Surface.Image != nil {
			g.front.Surface.Image.Deallocate()
		}
		texture := g.resources.images["floor:hall"]
		if texture == nil {
			texture = g.resources.images["floor:default"]
		}
		g.front.Surface = buildHallSurface(cacheCamera, texture)
		g.decorateWorkshop(g.front.Surface.Image, g.front.Surface.Camera)
	}
	drawRoomSurface(dst, g.front.Surface, c)
	for _, d := range g.front.Devices {
		g.drawDevice(dst, c, d)
	}
	if g.front.Actions.space != nil {
		g.drawProjectiles(dst, g.front.Actions.space.Snapshot().Projectiles, c)
	}
	v := c.Project(g.front.Position)
	g.drawSelfMarker(dst, v, max(7, 12*c.Scale))
	if chassis := g.catalog.ChassisList(); g.chassis >= 0 && g.chassis < len(chassis) {
		p := g.front.Actions.player
		p.Chassis, p.Life = chassis[g.chassis].ID, "alive"
		action := g.front.Actions.controls.View(p.ID, p.Life)
		if !g.front.Actions.updated.IsZero() {
			action = animation.Advance(action, max(0, min(1.0/60, g.lastFrameAt.Sub(g.front.Actions.updated).Seconds())))
		}
		g.drawAnimatedPlayer(dst, p, action, c, v, max(7, 12*c.Scale), color.RGBA{118, 166, 247, 255})
	}
	g.fitText(dst, g.name, v.X-100, v.Y+20, 200, 13, true)
}

func (g *Game) caption(f *uiFrame, owner, id, value, title string, r ui.Rect) {
	f.Controls = append(f.Controls, uiControl{ID: id, Owner: owner, Action: "front", Value: value, Title: title, Caption: title, Bounds: r})
}
func (g *Game) buildFrontInterface(f *uiFrame, layout ui.Layout) {
	if g.front.Page == "main" {
		g.buildMainMenu(f)
		return
	}
	g.addDeviceControls(f, g.front.Devices, g.hallCamera())
	g.buildHallHeading(f)
	g.buildCameraControl(f, layout)
}

func (g *Game) frontCapturesKeys() bool {
	for _, id := range []string{"settings", "entrance", "join", "room-settings", "room-capacity", "room-meta", "mods", "progression", "warehouse", "chassis-select", developerItemsWindow} {
		if slices.Contains(g.u.windows, id) {
			return true
		}
	}
	return g.frontBusy()
}
func (g *Game) frontBusy() bool {
	return g.mods.pending != nil || g.pending != nil || g.link != nil && g.state == nil
}
func (g *Game) performFront(action string) {
	switch action {
	case "start":
		g.front.Page = "lobby"
		g.u.windows = nil
		g.clearInspections()
		g.u.hover = ""
	case "settings":
		g.openWindow("settings")
	case "exit":
		g.exitRequested = true
	case "main":
		if g.pending == nil {
			g.front.Page = "main"
			g.u.windows = nil
			g.clearInspections()
			g.u.hover = ""
		}
	case "return":
		g.returnPersonalHall()
	}
}
func (g *Game) returnPersonalHall() {
	if g.pending != nil {
		return
	}
	if g.link != nil {
		g.link.Close()
		g.link = nil
	}
	if g.hosted && g.cfg.StopHost != nil {
		g.cfg.StopHost()
	}
	g.hosted, g.autoStart = false, false
	g.state = nil
	g.id = ""
	g.worldHistory.Clear()
	g.u.mouseInventory = nil
	g.u.returnCursorPending = false
	g.prediction = MovementPrediction{}
	g.localInput = model.Input{}
	g.destination = nil
	g.mouseFrames = 0
	g.front.Page = "lobby"
	g.u.windows = nil
	g.clearInspections()
	g.u.hover = ""
	g.u.phase = ""
}
func (g *Game) useHallDevice(action string) {
	if g.frontBusy() {
		return
	}
	switch action {
	case "solo":
		g.startHosted(true)
	case "host":
		g.startHosted(false)
	default:
		g.openWindow(action)
	}
}
func (g *Game) startHosted(solo bool) {
	if g.cfg.Host == nil {
		g.notify("notice.host_unavailable")
		return
	}
	options := lobby.CloneConfig(g.front.Config)
	if options.Combination != g.cfg.Combination {
		g.notify("notice.mods_load_first")
		return
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		g.notifyError(err)
		return
	}
	g.adminToken = hex.EncodeToString(secret[:])
	options.AdminToken = g.adminToken
	if err := options.Validate(); err != nil {
		g.notifyError(err)
		return
	}
	if solo {
		options.Players = 1
	}
	g.hosted = true
	g.autoStart = solo
	g.address = fmt.Sprintf("127.0.0.1:%d", options.Port)
	start := g.cfg.Host
	g.connectWithRetry(true, func() error { return start(options, solo) })
}

// Server startup is asynchronous from the rendering/input owner. Retry only TCP
// establishment; a rejected handshake is processed normally and never retried.
func (g *Game) connectWithRetry(retry bool, start func() error) {
	if g.pending != nil {
		return
	}
	g.resetConnectionView()
	g.pending = make(chan dialResult)
	h := protocol.Hello{Version: protocol.Version, Name: g.name, Chassis: g.catalog.ChassisList()[g.chassis].ID, Digest: g.catalog.Digest, Token: g.tokens[g.address]}
	h.MetaRanks = g.joinRanks()
	h.AdminToken = g.adminToken
	// New servers receive a fresh identity. Keep the old token until welcome so
	// a rejected startup (for example a busy port) cannot erase reconnect access.
	if start != nil && !g.front.Config.Restore {
		h.Token = ""
	}
	addr, result, rate := g.address, g.pending, g.cfg.InputRate
	go func() {
		var l *Link
		var err error
		select {
		case <-g.closed:
			return
		default:
		}
		if start != nil {
			err = start()
		}
		attempts := 1
		if retry {
			attempts = 30
		}
		for i := 0; err == nil && i < attempts; i++ {
			select {
			case <-g.closed:
				if start != nil && g.cfg.StopHost != nil {
					g.cfg.StopHost()
				}
				return
			default:
			}
			l, err = Dial(addr, h, rate)
			if err == nil {
				break
			}
			if i+1 < attempts {
				time.Sleep(100 * time.Millisecond)
				err = nil
			}
		}
		select {
		case <-g.closed:
			if l != nil {
				l.Close()
			}
			if start != nil && g.cfg.StopHost != nil {
				g.cfg.StopHost()
			}
		case result <- dialResult{l, err}:
		}
	}()
}

func (g *Game) updateFront(mouseConsumed, keysConsumed bool) {
	defer g.diagnostics.Since("client.logic.front", perf.Start())
	intent := model.Input{}
	weaponSkill := false
	defer func() {
		g.stepHall(intent)
		if weaponSkill {
			g.useHallWeaponSkill(intent.Aim)
		}
	}()
	if slices.Contains(g.u.windows, "mods") {
		g.updateModFields()
		return
	}
	if slices.Contains(g.u.windows, "room-settings") {
		if g.front.Editing == "name" {
			g.editText(&g.front.Name, 24)
		}
		if g.front.Editing == "checkpoint" {
			g.editText(&g.front.Draft.Checkpoint, 1024)
		}
		if g.front.Editing == "port" {
			for _, r := range ebiten.AppendInputChars(nil) {
				if r >= '0' && r <= '9' && len(g.front.PortText) < 5 {
					g.front.PortText += string(r)
				}
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.front.PortText) > 0 {
				g.front.PortText = g.front.PortText[:len(g.front.PortText)-1]
			}
		}
		return
	}
	if slices.Contains(g.u.windows, "join") {
		if g.frontBusy() {
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			g.field = (g.field + 1) % 3
		}
		g.editIdentity()
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.connect()
		}
		return
	}
	if g.front.Page != "lobby" || keysConsumed || g.pending != nil {
		return
	}
	if bindingAvailable(g.bound("interact"), mouseConsumed) && bindingPressed(g.bound("interact")) {
		if d, ok := device.Nearest(g.front.Devices, g.front.Position); ok {
			g.useHallDevice(d.Action)
		}
	}
	in := model.Vec{}
	mx, my := ebiten.CursorPosition()
	intent.Aim = g.hallCamera().Unproject(model.Vec{X: float64(mx), Y: float64(my)})
	weaponSkill = bindingAvailable(g.bound("weapon_skill"), mouseConsumed) && bindingPressed(g.bound("weapon_skill"))
	intent.Attack = !mouseConsumed && bindingDown(g.bound("attack"))
	intent.Dodge = bindingAvailable(g.bound("dodge"), mouseConsumed) && bindingPressed(g.bound("dodge"))
	if g.mouseMode {
		mx, my := ebiten.CursorPosition()
		target := g.hallCamera().Unproject(model.Vec{X: float64(mx), Y: float64(my)})
		if !mouseConsumed && bindingDown(g.bound("move")) {
			in = target.Sub(g.front.Position)
		}
	} else {
		for _, v := range []struct {
			action string
			vec    model.Vec
		}{{"up", model.Vec{Y: -1}}, {"down", model.Vec{Y: 1}}, {"left", model.Vec{X: -1}}, {"right", model.Vec{X: 1}}} {
			if bindingAvailable(g.bound(v.action), mouseConsumed) && bindingDown(g.bound(v.action)) {
				in = in.Add(v.vec)
			}
		}
		in = g.hallCamera().WorldDirection(in)
	}
	intent.Move = in
}
func (g *Game) editIdentity() {
	value := &g.address
	limit := 120
	if g.field == 1 {
		value = &g.name
		limit = 24
	}
	if g.field == 2 {
		value = &g.adminToken
		limit = 128
	}
	g.editText(value, limit)
}
func (g *Game) editText(value *string, limit int) {
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 32 && r != 127 && len([]rune(*value)) < limit {
			*value += string(r)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		rs := []rune(*value)
		if len(rs) > 0 {
			*value = string(rs[:len(rs)-1])
		}
	}
}
func (g *Game) chooseChassis(value string) {
	i, err := strconv.Atoi(value)
	if err == nil && i >= 0 && i < len(g.catalog.ChassisList()) {
		if slices.Contains(g.u.windows, "room-settings") {
			g.front.Chassis = i
		} else {
			g.chassis = i
		}
	}
}

// Explicit native validation exercises the same walking/range/action path as UI.
func (g *Game) smokeFrontActions() {
	if g.cfg.SmokeFrames <= 0 || g.cfg.SmokeAction == "" || g.front.Page != "lobby" || g.state != nil || g.pending != nil || g.hosted {
		return
	}
	d, ok := device.Find(g.front.Devices, "hall:"+g.cfg.SmokeAction)
	if !ok {
		return
	}
	if d.Near(g.front.Position) {
		if d.Action == "join" {
			g.connect()
		} else {
			g.useWorldDevice(d.ID)
		}
	} else {
		g.front.Position = lobby.Move(g.front.Position, d.Position.Sub(g.front.Position), 210, 1.0/60)
	}
}
func (g *Game) adjustRoom(value string) {
	if g.front.Draft == nil {
		return
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return
	}
	delta, err := strconv.Atoi(parts[1])
	if err != nil || delta != 1 && delta != -1 {
		return
	}
	c := lobby.CloneConfig(*g.front.Draft)
	switch parts[0] {
	case "backpack":
		c.BackpackCapacity = max(1, min(4096, c.BackpackCapacity+delta))
	case "accessories":
		c.AccessorySlots = max(-1, min(4096, c.AccessorySlots+delta))
	case "players":
		c.Players = max(1, min(128, c.Players+delta))
	case "vote":
		c.VotePercent = max(1, min(100, c.VotePercent+delta*5))
	case "tps":
		if c.TickRate == 30 {
			c.TickRate = 60
		} else {
			c.TickRate = 30
		}
	case "snapshots":
		c.SnapshotRate = max(1, min(60, c.SnapshotRate+delta))
	case "port":
		c.Port = max(1024, min(65535, c.Port+delta))
		g.front.PortText = strconv.Itoa(c.Port)
	}
	if c.Validate() == nil {
		*g.front.Draft = c
	}
}
func (g *Game) saveRoomConfig() {
	if g.front.Draft == nil {
		return
	}
	s := g.preferences()
	options := *g.front.Draft
	port, err := strconv.Atoi(g.front.PortText)
	if err != nil {
		g.notify("notice.room_invalid")
		return
	}
	options.Port = port
	if options.Validate() != nil || len([]rune(g.front.Name)) < 1 || len([]rune(g.front.Name)) > 24 {
		g.notify("notice.room_invalid")
		return
	}
	s.Room = &options
	s.PlayerName = g.front.Name
	s.Chassis = g.catalog.ChassisList()[g.front.Chassis].ID
	if err := g.commitPreferences(s); err != nil {
		g.notify("notice.settings_save", "detail", err.Error())
		return
	}
	g.closeWindow("room-settings")
	g.notify("notice.settings_saved")
}
