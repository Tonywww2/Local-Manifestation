// Package session serializes team transactions and owns each player's scene assignment.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"time"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/persistence"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
	"hsmultiplayer/sdk"
)

type Config struct {
	AdminToken       string
	SpecialMode      bool
	BackpackCapacity int
	AccessorySlots   int
	MetaRules        *model.MetaRules
	ProgressionScope string
	TickRate         int
	SnapshotRate     int
	ContentStore     *content.Store
	MaxPlayers       int
	Seed             int64
	Checkpoint       string
	Restore          *persistence.Checkpoint
	VoteRatio        float64
}
type member struct {
	actions     scene.PlayerActions
	walk        model.Input
	walkUpdated uint64
	player      model.Player
	token       string
	out         chan protocol.Message
	generation  uint64
	sequence    uint64
	visited     map[string]bool
}
type event struct {
	kind        string
	hello       protocol.Hello
	out         chan protocol.Message
	id          string
	generation  uint64
	request     protocol.Request
	reply       chan joined
	diagnostics chan Diagnostics
}
type joined struct {
	ID         string
	Generation uint64
	Err        error
}
type Coordinator struct {
	performance *perf.Recorder
	events      chan event
	done        chan struct{}
}
type team struct {
	groundLoot     []model.Loot
	performance    *perf.Recorder
	processSampler perf.ProcessSampler
	nextHost       uint64
	spaces         map[string]*scene.FreeSpace
	layerMembers   []string
	runID          string
	started        string
	mapTemplate    model.MapTemplateDef
	mapPlayers     int
	trades         map[string]protocol.Trade
	nextTrade      uint64
	adventureTime  int64
	serverTick     uint64
	actualTPS      timing.Meter
	droppedTicks   uint64
	runtime        *script.Runtime
	config         Config
	catalog        *content.Catalog
	players        map[string]*member
	actors         map[string]*scene.Actor
	views          map[string]protocol.Scene
	nodes          map[string]model.Node
	terrain        []model.Terrain
	phase          string
	floor          int
	shared         int
	initialX       int
	initialY       int
	cursed         bool
	bossRoute      string
	admin          string
	vote           *protocol.Vote
	nextGeneration uint64
}

func New(c *content.Catalog, cfg Config) (*Coordinator, error) {
	if cfg.TickRate == 0 {
		cfg.TickRate = timing.DefaultLogicRate
	}
	if cfg.SnapshotRate == 0 {
		cfg.SnapshotRate = timing.DefaultSnapshotRate
	}
	if !timing.LogicRate(cfg.TickRate) || cfg.SnapshotRate < 1 || cfg.SnapshotRate > 60 {
		return nil, fmt.Errorf("invalid timing: logic 30/60, snapshots 1..60")
	}
	if cfg.MaxPlayers < 1 || cfg.MaxPlayers > 128 {
		return nil, fmt.Errorf("invalid player limits")
	}
	if cfg.Restore != nil {
		if err := validateRestore(c, cfg.Restore); err != nil {
			return nil, err
		}
	}
	if cfg.ContentStore == nil {
		cfg.ContentStore = content.NewStore(c)
	}
	pinned, release := cfg.ContentStore.Acquire()
	if pinned != c {
		release()
		return nil, fmt.Errorf("content catalog is not the current generation")
	}
	success := false
	defer func() {
		if !success {
			release()
		}
	}()
	t := &team{config: cfg, catalog: c, players: map[string]*member{}, actors: map[string]*scene.Actor{}, views: map[string]protocol.Scene{}, nodes: map[string]model.Node{}, phase: "lobby"}
	if cfg.MetaRules != nil {
		rules := *cfg.MetaRules
		t.config.MetaRules = &rules
	}
	if cfg.SpecialMode && (cfg.BackpackCapacity < 1 || cfg.BackpackCapacity > 4096 || cfg.AccessorySlots < -1 || cfg.AccessorySlots > 4096) {
		return nil, fmt.Errorf("invalid special inventory configuration")
	}
	if t.config.ProgressionScope == "" {
		t.config.ProgressionScope = "normal"
		if c.Namespace() != "base" || c.Modified() || cfg.SpecialMode || cfg.MetaRules != nil && *cfg.MetaRules != model.DefaultMetaRules() {
			t.config.ProgressionScope = "custom:" + c.Digest
		}
	}
	t.started = time.Now().UTC().Format(time.RFC3339Nano)
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", cfg.Seed, t.started)))
	t.runID = hex.EncodeToString(hash[:])
	if cfg.VoteRatio == 0 {
		t.config.VoteRatio = c.Run().VoteRatio
	}
	if t.config.VoteRatio <= 0 || t.config.VoteRatio > 1 {
		return nil, fmt.Errorf("invalid vote ratio")
	}
	if cfg.Restore != nil {
		cfg.Seed = cfg.Restore.Seed
	}
	vm, err := script.New(c, cfg.Seed)
	if err != nil {
		return nil, err
	}
	t.runtime = vm
	if cfg.Restore != nil {
		s := protocol.CloneState(cfg.Restore.State)
		t.groundLoot = s.GroundLoot
		t.runID = s.RunID
		t.started = s.Started
		t.config.ProgressionScope = s.ProgressionScope
		rules := s.MetaRules
		t.config.MetaRules = &rules
		t.layerMembers = append([]string(nil), s.LayerMembers...)
		t.config.SpecialMode = s.SpecialMode
		t.config.BackpackCapacity = s.BackpackCapacity
		t.config.AccessorySlots = s.AccessorySlots
		t.floor = s.Floor
		t.adventureTime = s.AdventureTime
		t.phase = "preparation"
		t.cursed = s.Cursed
		t.config.Seed = cfg.Restore.Seed
		t.shared = s.SharedAP
		t.initialX = s.PersonalInitial
		t.initialY = s.SharedInitial
		tokens := map[string]string{}
		for _, i := range cfg.Restore.Identities {
			tokens[i.Player] = i.Token
		}
		for _, p := range s.Players {
			p.Inventory, err = inventory.ReturnCursor(p.Inventory, c.Item)
			if err != nil {
				vm.Close()
				return nil, err
			}
			p.Online = false
			p.Active = false
			p.Scene = ""
			token := tokens[p.ID]
			if token == "" {
				vm.Close()
				return nil, fmt.Errorf("missing identity in checkpoint")
			}
			t.players[p.ID] = &member{player: p, token: token}
		}
		for _, n := range s.Nodes {
			t.nodes[n.ID] = n
		}
		t.mapPlayers = s.MapPlayers
		t.mapTemplate, _ = c.GenerateMap(t.floor, t.config.Seed, t.mapPlayers)
		_, _, special := c.TeamMeta(layerPlayers(s.Players, s.LayerMembers), t.metaRules())
		t.mapTemplate = t.catalog.AddLayerSpecialNodes(t.mapTemplate, special, t.floor)
		t.terrain = t.mapTemplate.Terrain
		for _, v := range cfg.Restore.Visits {
			if m := t.players[v.Player]; m != nil {
				m.visited = map[string]bool{}
				for _, id := range v.Nodes {
					m.visited[id] = true
				}
			}
		}
		for _, id := range t.ids() {
			m := t.players[id]
			if !t.spaceArena(m.player.Node).ContainsDisk(m.player.Position, 12) {
				t.placeInSpace(m, m.player.Node)
			}
		}
	}
	t.performance = &perf.Recorder{}
	a := &Coordinator{performance: t.performance, events: make(chan event, 256), done: make(chan struct{})}
	success = true
	go a.run(t, release)
	return a, nil
}
func (a *Coordinator) Connect(h protocol.Hello, out chan protocol.Message) (string, uint64, error) {
	reply := make(chan joined, 1)
	select {
	case a.events <- event{kind: "connect", hello: h, out: out, reply: reply}:
	case <-a.done:
		return "", 0, protocol.Reject("error.server_stopped")
	}
	select {
	case r := <-reply:
		return r.ID, r.Generation, r.Err
	case <-a.done:
		return "", 0, protocol.Reject("error.server_stopped")
	}
}
func (a *Coordinator) Submit(id string, g uint64, r protocol.Request) {
	select {
	case a.events <- event{kind: "request", id: id, generation: g, request: r}:
	case <-a.done:
	}
}
func (a *Coordinator) Disconnect(id string, g uint64) {
	select {
	case a.events <- event{kind: "disconnect", id: id, generation: g}:
	case <-a.done:
	}
}
func (a *Coordinator) Close() {
	select {
	case a.events <- event{kind: "stop"}:
	case <-a.done:
	}
	<-a.done
}
func (a *Coordinator) run(t *team, release func()) {
	defer close(a.done)
	defer release()
	defer t.stopScenes()
	defer t.runtime.Close()
	stepper := timing.NewStepper(t.config.TickRate, time.Now())
	logic := time.NewTicker(time.Second / time.Duration(t.config.TickRate))
	defer logic.Stop()
	network := time.NewTicker(time.Second / time.Duration(t.config.SnapshotRate))
	defer network.Stop()
	for {
		select {
		case e := <-a.events:
			eventStart := perf.Start()
			switch e.kind {
			case "diagnostics":
				e.diagnostics <- t.diagnostics(time.Now())
				continue
			case "stop":
				return
			case "connect":
				id, g, err := t.connect(e.hello, e.out)
				e.reply <- joined{id, g, err}
			case "disconnect":
				if m := t.players[e.id]; m != nil && m.generation == e.generation {
					_, _ = t.inventoryRequest(m, protocol.Request{Kind: "inventory_return"})
					m.player.Online = false
					m.walk = model.Input{}
					m.out = nil
					if s := t.actors[m.player.Scene]; s != nil {
						s.Online(e.id, false)
					}
				}
			case "request":
				m := t.players[e.id]
				if m == nil || !m.player.Online || m.generation != e.generation || e.request.Seq <= m.sequence {
					continue
				}
				m.sequence = e.request.Seq
				if err := t.request(m, e.request); err != nil {
					t.send(m, protocol.ErrorMessage(err))
				}
			}
			t.performance.Since("server.command", eventStart)
		case <-logic.C:
			now := time.Now()
			batchStart := perf.Start()
			steps := stepper.Advance(now)
			for i := 0; i < steps; i++ {
				t.serverTick++
				t.advanceAdventure()
				t.stepSpaces()
			}
			t.actualTPS.Add(steps, now)
			t.droppedTicks = stepper.Dropped
			if steps > 0 {
				t.refresh()
			}
			t.performance.Since("server.logic.batch", batchStart)
		case <-network.C:
			snapshotStart := perf.Start()
			s := t.snapshot()
			for _, m := range t.players {
				if m.player.Online {
					t.send(m, protocol.Message{Kind: "state", State: &s})
				}
			}
			t.performance.Since("server.snapshot_publish", snapshotStart)
		}
	}
}
func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (t *team) connect(h protocol.Hello, out chan protocol.Message) (string, uint64, error) {
	if h.Version != protocol.Version || h.Digest != t.catalog.Digest {
		return "", 0, protocol.Reject("error.content_mismatch")
	}
	var m *member
	if h.Token != "" {
		for _, p := range t.players {
			if p.token == h.Token {
				m = p
				break
			}
		}
		if m == nil {
			return "", 0, protocol.Reject("error.reconnect_token")
		}
	}
	if m == nil {
		if t.phase != "lobby" && t.phase != "preparation" {
			return "", 0, protocol.Reject("error.join_preparation")
		}
		if t.activeCount() >= t.config.MaxPlayers {
			return "", 0, protocol.Reject("error.room_full")
		}
		if len([]rune(h.Name)) < 1 || len([]rune(h.Name)) > 24 {
			return "", 0, protocol.Reject("error.name_length")
		}
		id := randomToken()[:12]
		p, err := content.Player(t.catalog, id, h.Name, h.Chassis)
		if err != nil {
			return "", 0, protocol.Reject("error.unknown_chassis")
		}
		if t.config.SpecialMode {
			p.Inventory.Capacity = t.config.BackpackCapacity
			p.Inventory.BagSlots = nil // Initial configuration, before the player joins.
			p.Inventory.AccessorySlots = t.config.AccessorySlots
		}
		p, err = t.catalog.ApplyMeta(p, h.MetaRanks, t.metaRules())
		if err != nil {
			return "", 0, err
		}
		if t.floor > 0 {
			p.Level = t.floor
			p, err = t.catalog.RefreshLevelGrowth(p)
			if err != nil {
				return "", 0, err
			}
			p, err = content.ApplyFloorJoinGrowth(p, t.floor)
			if err != nil {
				return "", 0, err
			}
			p.HP = p.MaxHP
			p.AP = t.initialX
		}
		p.Node = t.catalog.Map(t.catalog.Floor(max(1, t.floor)).Map).Start
		p.Statuses.Time = t.adventureTime
		p.ResourceTime = t.adventureTime
		p.Position = t.spawnAt(p.Node, p.ID)
		p.PositionEpoch++
		m = &member{player: p, token: randomToken()}
		t.players[id] = m
	}
	if !m.player.Active {
		if t.phase != "lobby" && t.phase != "preparation" {
			return "", 0, protocol.Reject("error.return_preparation")
		}
		if t.activeCount() >= t.config.MaxPlayers {
			return "", 0, protocol.Reject("error.room_full")
		}
		m.player.Active = true
	}
	if m.out != nil {
		t.send(m, protocol.Message{Kind: "replaced", Code: "notice.replaced"})
	}
	t.nextGeneration++
	m.out = out
	m.generation = t.nextGeneration
	m.sequence = 0
	m.walk = model.Input{}
	m.player.Online = true
	if h.AdminToken != "" && t.config.AdminToken != "" && subtle.ConstantTimeCompare([]byte(h.AdminToken), []byte(t.config.AdminToken)) == 1 {
		t.admin = m.player.ID
	} else if t.admin == "" && t.config.AdminToken == "" {
		t.admin = m.player.ID
	}
	if s := t.actors[m.player.Scene]; s != nil {
		s.Online(m.player.ID, true)
	}
	t.send(m, protocol.Message{Kind: "welcome", Player: m.player.ID, Token: m.token})
	return m.player.ID, m.generation, nil
}
func (t *team) send(m *member, msg protocol.Message) {
	if m.out == nil {
		return
	}
	select {
	case m.out <- msg:
	default:
	}
}
func (t *team) request(m *member, r protocol.Request) error {
	if r.Kind == "ping" {
		t.send(m, protocol.Message{Kind: "pong", Probe: r.Probe})
		return nil
	}
	if r.Kind == "inventory_return" {
		_, err := t.inventoryRequest(m, r)
		return err
	}
	if r.Kind == "debug_grant_item" {
		return t.debugGrantItem(m, r.Item)
	}
	if r.Kind == "export" {
		if m.player.ID != t.admin {
			return protocol.Reject("error.admin_only")
		}
		saved, err := persistence.Load(t.config.Checkpoint, t.catalog.Digest)
		if err != nil {
			return err
		}
		data, err := json.Marshal(saved)
		if err != nil {
			return err
		}
		t.send(m, protocol.Message{Kind: "checkpoint", Export: data})
		return nil
	}
	if r.Kind == "input" {
		if !r.Input.Move.Finite() || !r.Input.Aim.Finite() {
			return protocol.Reject("error.invalid_input")
		}
		if s := t.actors[m.player.Scene]; s != nil {
			s.Input(m.player.ID, r.Input)
		} else if m.player.Scene == "" {
			r.Input.Dodge = r.Input.Dodge || m.walk.Dodge
			m.walk = r.Input
			m.walkUpdated = t.serverTick
		}
		return nil
	}
	if r.Kind == "start" {
		t.refresh()
		if m.player.ID != t.admin {
			return protocol.Reject("error.admin_only")
		}
		if t.phase != "lobby" && t.phase != "preparation" {
			return protocol.Reject("error.not_preparation")
		}
		if t.phase == "lobby" {
			if err := t.initialize(1); err != nil {
				return err
			}
		}
		for _, p := range t.players {
			p.player.Active = p.player.Online
		}
		if err := t.save(); err != nil {
			return err
		}
		t.phase = "exploring"
		return nil
	}
	if (t.phase == "preparation" || t.phase == "lobby") && (r.Kind == "inventory_click" || r.Kind == "inventory_quick" || r.Kind == "equip" || r.Kind == "drop_item" || r.Kind == "pickup") {
		if !m.player.Active || m.player.Life != "alive" {
			return protocol.Reject("error.incapacitated")
		}
		_, err := t.inventoryRequest(m, r)
		return err
	}
	if t.phase == "preparation" && (r.Kind == "weapon_skill" || r.Kind == "use_item") {
		if !m.player.Active || m.player.Life != "alive" {
			return protocol.Reject("error.incapacitated")
		}
		_, err := t.inventoryRequest(m, r)
		return err
	}
	if t.phase != "exploring" && t.phase != "intermission" {
		return protocol.Reject("error.not_exploring")
	}
	// Validate global actions against current scene results, including incapacitation.
	t.refresh()
	if t.phase != "exploring" && t.phase != "intermission" {
		return protocol.Reject("error.not_exploring")
	}
	if r.Kind == "withdraw_vote" {
		if t.vote == nil || t.vote.Node != t.bossNode() || t.vote.Item != "" {
			return protocol.Reject("error.no_vote")
		}
		t.vote.Yes = slices.DeleteFunc(t.vote.Yes, func(id string) bool { return id == m.player.ID })
		t.vote.Entrants = slices.DeleteFunc(t.vote.Entrants, func(id string) bool { return id == m.player.ID })
		delete(t.vote.Tools, m.player.ID)
		if len(t.vote.Yes) == 0 {
			t.vote = nil
		}
		return nil
	}
	if r.Kind == "vote" {
		if r.Accept && t.vote != nil && t.nodes[t.vote.Node].ID == "" {
			if err := t.deviceReady(m, t.vote.Node); err != nil {
				return err
			}
		}
		return t.castVoteEntry(m, r.Accept, r.Join)
	}
	if m.player.Life != "alive" {
		return protocol.Reject("error.incapacitated")
	}
	if handled, err := t.inventoryRequest(m, r); handled {
		return err
	}
	if r.Kind == "device" || r.Kind == "next" {
		return t.useDevice(m, r.Node)
	}
	if t.phase == "intermission" {
		return protocol.Reject("error.not_exploring")
	}
	if t.bossRoute != "" {
		return protocol.Reject("error.boss_committed")
	}
	switch r.Kind {
	case "travel_tool":
		return t.useMovement(m, r.Item, r.Node)
	case "enter":
		return t.enter(m, r.Node)
	case "propose":
		if err := t.deviceReady(m, r.Node); err != nil {
			return err
		}
		return t.propose(m, r.Node)
	case "upgrade":
		if m.player.Scene != "" {
			return protocol.Reject("error.outside_combat")
		}
		n := t.nodes[m.player.Node]
		if n.Kind != "shop" || !n.Open || !t.catalog.Node(n.Definition).WeaponUpgrade {
			return protocol.Reject("error.shop_required")
		}
		if !t.atShop(m) {
			return protocol.Reject("error.door_distance")
		}
		player, err := t.interact(m, n, "upgrade", false)
		if err != nil {
			return err
		}
		m.player = player
		return nil
	case "rescue":
		return t.rescue(m)
	default:
		return protocol.Reject("error.unknown_command")
	}
}
func (t *team) initialize(floor int) error {
	defer t.performance.Since("server.load.floor", perf.Start())
	count := 0
	for _, m := range t.players {
		if m.player.Online || m.player.Active {
			count++
		}
	}
	count = max(1, count)
	template, err := t.catalog.GenerateMap(floor, t.config.Seed, count)
	if err != nil {
		return err
	}
	var members []model.Player
	for _, m := range t.players {
		if m.player.Active || m.player.Online {
			members = append(members, m.player)
		}
	}
	extraX, extraY, special := t.catalog.TeamMeta(members, t.metaRules())
	template = t.catalog.AddLayerSpecialNodes(template, special, floor)
	prepared := make(map[string]model.Player, len(t.players))
	for id, m := range t.players {
		p, err := content.PrepareLayerPlayer(m.player)
		if err != nil {
			return err
		}
		prepared[id] = p
	}
	t.floor = floor
	t.mapTemplate = template
	t.mapPlayers = count
	t.layerMembers = nil
	for _, p := range members {
		t.layerMembers = append(t.layerMembers, p.ID)
	}
	sort.Strings(t.layerMembers)
	t.phase = "preparation"
	t.vote = nil
	t.bossRoute = ""
	t.initialX = t.catalog.Floor(floor).PersonalAP + extraX
	t.initialY = t.catalog.Floor(floor).SharedAP + extraY
	if t.cursed {
		t.initialX--
		t.initialY--
	}
	t.shared = t.initialY
	t.nodes = t.makeMap()
	t.spaces = nil
	t.groundLoot = nil
	t.terrain = template.Terrain
	for _, id := range t.ids() {
		m := t.players[id]
		m.player = prepared[id]
		m.actions = scene.PlayerActions{}
		m.player.Loot = nil
		m.visited = map[string]bool{}
		m.player.AP = t.initialX
		m.player.Node = t.catalog.Map(t.catalog.Floor(floor).Map).Start
		m.player.Scene = ""
		t.placeInSpace(m, m.player.Node)
		m.player.Dodges = 2
	}
	return nil
}
func (t *team) makeMap() map[string]model.Node {
	template := t.mapTemplate
	nodes := map[string]model.Node{}
	for _, entry := range template.Nodes {
		def := t.catalog.Node(entry.Definition)
		free := entry.ID == template.Start || def.Kind == "passage"
		nodes[entry.ID] = model.Node{ID: entry.ID, Definition: def.ID, Name: def.Name, NameKey: def.NameKey, Room: def.Room, Cost: def.Cost, Q: entry.Q, R: entry.R, Kind: def.Kind, Blocking: expedition.Blocking(def), Payment: def.Payment, Hint: def.Hint, HintKey: def.HintKey, Edges: entry.Edges, Open: free, Completed: free}
	}
	return nodes
}
func (t *team) bossNode() string { return t.catalog.Map(t.catalog.Floor(t.floor).Map).Boss }
func (t *team) interact(m *member, n model.Node, command string, first bool) (model.Player, error) {
	def := t.catalog.Node(n.Definition)
	commands, err := t.runtime.Node(def.Handler, sdk.NodeContext{Player: m.player, Node: n, Floor: t.floor, Command: command, FirstVisit: first, Parameters: def.Parameters})
	if err != nil {
		return model.Player{}, err
	}
	next, err := content.ApplyCommands(t.catalog, map[string]model.Player{m.player.ID: m.player}, commands, content.NodeCommands)
	if err != nil {
		return model.Player{}, err
	}
	return next[m.player.ID], nil
}
func (t *team) propose(m *member, id string) error {
	if t.bossRoute != "" {
		return protocol.Reject("error.boss_committed")
	}
	if id == t.bossNode() {
		return t.selectBoss(m, "")
	}
	if t.vote != nil {
		return protocol.Reject("error.vote_open")
	}
	if id == "enhanced" {
		if t.shared != 0 {
			return protocol.Reject("error.enhanced_requires_empty")
		}
	} else if id == "exhaust" {
		if t.shared == 0 {
			return protocol.Reject("error.shared_empty")
		}
	} else {
		n, ok := t.nodes[id]
		if !ok || !t.reachable(m.player.Node, id) || m.player.Scene != "" {
			return protocol.Reject("error.invalid_shared_node")
		}
		if t.shared < 1 {
			return protocol.Reject("error.no_shared_ap")
		}
		if n.Kind == "boss" {
			if err := t.normalBossReady(); err != nil {
				return err
			}
		}
	}
	v := &protocol.Vote{Node: id}
	for _, pid := range t.ids() {
		p := t.players[pid].player
		if p.Active && p.Online && p.Life == "alive" {
			v.Eligible = append(v.Eligible, pid)
		}
	}
	v.Required = int(math.Ceil(float64(len(v.Eligible)) * t.config.VoteRatio))
	if v.Required == 0 {
		return protocol.Reject("error.no_voters")
	}
	t.vote = v
	return t.castVoteEntry(m, true, t.nodes[id].ID != "" && t.nodes[id].Kind != "boss")
}
func (t *team) castVote(m *member, yes bool) error {
	return t.castVoteEntry(m, yes, false)
}
func (t *team) castVoteEntry(m *member, yes, join bool) error {
	v := t.vote
	if v == nil {
		return protocol.Reject("error.no_vote")
	}
	if v.Node == t.bossNode() && v.Item == "" && yes {
		return t.selectBoss(m, "")
	}
	has := func(ids []string) bool {
		for _, id := range ids {
			if id == m.player.ID {
				return true
			}
		}
		return false
	}
	if !has(v.Eligible) || m.player.Life != "alive" {
		return protocol.Reject("error.not_eligible")
	}
	if has(v.Yes) || has(v.No) {
		return protocol.Reject("error.already_voted")
	}
	if yes && join && t.nodes[v.Node].ID != "" {
		if m.player.Scene != "" || !t.reachable(m.player.Node, v.Node) {
			return protocol.Reject("error.unreachable")
		}
		v.Entrants = append(v.Entrants, m.player.ID)
	}
	if yes {
		v.Yes = append(v.Yes, m.player.ID)
	} else {
		v.No = append(v.No, m.player.ID)
	}
	if len(v.Yes) >= v.Required {
		id := v.Node
		t.vote = nil
		if v.Item != "" {
			return t.executeTeamMovement(*v)
		}
		switch id {
		case "exhaust":
			t.shared = 0
			return nil
		case "enhanced":
			return t.startBoss(true)
		case "next":
			if t.phase != "intermission" {
				return protocol.Reject("error.not_preparation")
			}
			return t.advanceLayer()
		case t.bossNode():
			if err := t.normalBossReady(); err != nil {
				return err
			}
			return t.startBoss(false)
		default:
			var entrants []*member
			for _, pid := range v.Entrants {
				if entry := t.players[pid]; entry != nil && entry.player.Online && entry.player.Active && entry.player.Life == "alive" && entry.player.Scene == "" && t.reachable(entry.player.Node, id) {
					entrants = append(entrants, entry)
				}
			}
			return t.enterGroup(entrants, id, true)
		}
	}
	if len(v.No) > len(v.Eligible)-v.Required {
		t.vote = nil
	}
	return nil
}
func (t *team) normalBossReady() error {
	for _, a := range t.actors {
		if a.Snapshot().State != "paused" {
			return protocol.Reject("error.battles_pending")
		}
	}
	return nil
}
func (t *team) startBoss(enhanced bool) error {
	if t.bossRoute != "" {
		return protocol.Reject("error.boss_committed")
	}
	if !enhanced && t.shared < 1 {
		return protocol.Reject("error.no_shared_ap")
	}
	boss := t.bossNode()
	room := t.nodes[boss].Room
	if enhanced {
		room = t.catalog.Floor(t.floor).EnhancedRoom
	}
	a, err := scene.New(t.catalog, scene.Config{ShotOrigin: t.projectileOrigin(boss), ExternalClock: true, AdventureTime: t.adventureTime, TickRate: t.config.TickRate, ID: boss, Room: room, Floor: t.floor, Seed: content.Seed(t.config.Seed, t.floor, boss+":"+room)})
	if err != nil {
		return err
	}
	// Capture each scene before stopping; earned wave rewards survive forced unfinished settlement.
	t.stopScenes()
	for _, id := range t.ids() {
		m := t.players[id]
		if !m.player.Active || m.player.Life == "dead" {
			continue
		}
		p := m.player
		p, err = content.LeaveStatuses(p)
		if err != nil {
			a.Stop()
			return err
		}
		p.Node = boss
		p.Scene = boss
		if err = a.AddWithActions(p, nil, m.actions); err != nil {
			a.Stop()
			return err
		}
		m.actions = scene.PlayerActions{}
		m.player = p
	}
	if enhanced {
		t.cursed = true
		t.bossRoute = "enhanced"
	} else {
		t.shared--
		t.bossRoute = "normal"
	}
	t.actors[boss] = a
	n := t.nodes[boss]
	n.Open = true
	n.Room = room
	t.nodes[boss] = n
	return nil
}
func (t *team) rescue(m *member) error {
	if m.player.Scene != "" {
		return protocol.Reject("error.rescue_clear")
	}
	prepared := map[string]model.Player{}
	for id, p := range t.players {
		if p.player.Node == m.player.Node && p.player.Scene == "" && p.player.Life == "down" {
			next, err := content.RescuePlayer(p.player)
			if err != nil {
				return err
			}
			prepared[id] = next
		}
	}
	if len(prepared) == 0 {
		return protocol.Reject("error.rescue_missing")
	}
	for id, next := range prepared {
		t.players[id].player = next
	}
	return nil
}
func (t *team) merge(s protocol.Scene) {
	for _, p := range s.Players {
		if m := t.players[p.ID]; m != nil {
			p.AP = m.player.AP
			p.Online = m.player.Online
			p.Active = m.player.Active
			m.player = p
		}
	}
}
func (t *team) stopScenes() {
	for _, id := range t.sceneIDs() {
		departure := t.actors[id].StopWithActions()
		s := departure.Scene
		t.merge(s)
		t.keepSceneProjectiles(id, departure.Projectiles)
		for _, p := range s.Players {
			if m := t.players[p.ID]; m != nil {
				m.actions = departure.Actions[p.ID]
				m.player.Scene = ""
			}
		}
	}
	t.actors = map[string]*scene.Actor{}
	t.views = map[string]protocol.Scene{}
}
func (t *team) refresh() {
	defer t.performance.Since("server.logic.scene_sync", perf.Start())
	for _, id := range t.sceneIDs() {
		a := t.actors[id]
		s := a.Snapshot()
		t.merge(s)
		t.views[id] = s
		if s.State == "cleared" {
			departure := a.StopWithActions()
			s = departure.Scene
			t.merge(s)
			delete(t.actors, id)
			delete(t.views, id)
			t.keepSceneProjectiles(id, departure.Projectiles)
			if space, err := t.freeSpace(id); err == nil {
				space.ReceiveEnemyVisuals(s.EnemyActions)
			}
			n := t.nodes[id]
			n.Completed = true
			t.nodes[id] = n
			for _, p := range s.Players {
				m := t.players[p.ID]
				m.actions = departure.Actions[p.ID]
				m.player.Scene = ""
				// Combat completion changes the simulation owner, not the node or
				// its coordinate frame. Stay exactly where the battle ended.
				m.player.Position = t.spaceArena(id).ClampDisk(p.Position, 12)
				m.walk = model.Input{}

				m.player.MaxHP = content.MaxHP(m.player)
			}
			if id == t.bossNode() {
				if t.bossRoute == "enhanced" {
					for _, m := range t.players {
						if m.player.Active {
							m.player.EnhancedWins++
						}
					}
				}
				t.stopScenes()
				if t.floor >= len(t.catalog.Run().Floors) {
					t.phase = "won"
				} else {
					t.phase = "intermission"
					t.vote = nil
				}
				return
			}
		}
	}
	if t.phase == "exploring" {
		alive := false
		for _, m := range t.players {
			if m.player.Active && m.player.Life == "alive" {
				alive = true
			}
		}
		if !alive {
			t.phase = "lost"
			t.stopScenes()
		}
	}
}

func (t *team) activeCount() int {
	n := 0
	for _, m := range t.players {
		if m.player.Active {
			n++
		}
	}
	return n
}
func (t *team) ids() []string {
	ids := make([]string, 0, len(t.players))
	for id := range t.players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (t *team) sceneIDs() []string {
	ids := make([]string, 0, len(t.actors))
	for id := range t.actors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (t *team) snapshot() protocol.State {
	defer t.performance.Since("server.snapshot_build", perf.Start())
	s := protocol.State{AdventureTime: t.adventureTime, ServerTick: t.serverTick, TickRate: t.config.TickRate, SnapshotRate: t.config.SnapshotRate, ActualTPS: t.actualTPS.Rate(), DroppedTicks: t.droppedTicks, Phase: t.phase, Floor: t.floor, Floors: len(t.catalog.Run().Floors), SharedAP: t.shared, PersonalInitial: t.initialX, SharedInitial: t.initialY, Cursed: t.cursed, Admin: t.admin}
	if t.performance != nil {
		s.Performance = t.performance.Snapshot(time.Now())
		s.Process = t.processSampler.Sample(time.Now())
	}
	s.Trades = t.tradeSnapshot()
	s.GroundLoot = slices.Clone(t.groundLoot)
	s.RunID = t.runID
	s.Started = t.started
	s.ProgressionScope = t.config.ProgressionScope
	s.MetaRules = t.metaRules()
	s.SpecialMode = t.config.SpecialMode
	s.BackpackCapacity = t.config.BackpackCapacity
	s.AccessorySlots = t.config.AccessorySlots
	s.LayerMembers = append([]string(nil), t.layerMembers...)
	s.MapPlayers = t.mapPlayers
	s.Terrain = append([]model.Terrain(nil), t.terrain...)
	for _, id := range t.ids() {
		s.Players = append(s.Players, model.ClonePlayer(t.players[id].player))
	}
	keys := make([]string, 0, len(t.nodes))
	for k := range t.nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, id := range keys {
		n := t.nodes[id]
		n.Edges = append([]string(nil), n.Edges...)
		s.Nodes = append(s.Nodes, n)
	}
	for _, id := range t.sceneIDs() {
		s.Scenes = append(s.Scenes, protocol.CloneScene(t.views[id]))
	}
	spaceIDs := make([]string, 0, len(t.spaces))
	for id := range t.spaces {
		spaceIDs = append(spaceIDs, id)
	}
	sort.Strings(spaceIDs)
	for _, id := range spaceIDs {
		view := t.spaces[id].Snapshot()
		for _, p := range t.spacePlayers(id) {
			view.Actions = append(view.Actions, p.Actions.View(p.Player.ID, p.Player.Life))
		}
		if len(view.Projectiles) > 0 || len(view.Actions) > 0 || len(view.EnemyActions) > 0 {
			s.Scenes = append(s.Scenes, view)
		}
	}
	if t.vote != nil {
		v := *t.vote
		v.Eligible = append([]string(nil), v.Eligible...)
		v.Yes = append([]string(nil), v.Yes...)
		v.No = append([]string(nil), v.No...)
		v.Entrants = append([]string(nil), v.Entrants...)
		v.Tools = maps.Clone(v.Tools)
		s.Vote = &v
	}
	return s
}
func (t *team) metaRules() model.MetaRules {
	if t.config.MetaRules != nil {
		return *t.config.MetaRules
	}
	return model.DefaultMetaRules()
}
func (t *team) save() error {
	c := persistence.Checkpoint{Version: persistence.Version, Digest: t.catalog.Digest, Seed: t.config.Seed, State: t.snapshot()}
	c.State.Performance, c.State.Process = perf.Snapshot{}, perf.Process{}
	// Checkpoints contain layer preparation, not transient practice projectiles.
	var battles []protocol.Scene
	for _, view := range c.State.Scenes {
		if view.State != "idle" {
			view.Performance = perf.Snapshot{}
			battles = append(battles, view)
		}
	}
	c.State.Scenes = battles
	for _, id := range t.ids() {
		c.Identities = append(c.Identities, persistence.Identity{Player: id, Token: t.players[id].token})
		visit := persistence.Visit{Player: id}
		for node, seen := range t.players[id].visited {
			if seen {
				visit.Nodes = append(visit.Nodes, node)
			}
		}
		sort.Strings(visit.Nodes)
		c.Visits = append(c.Visits, visit)
	}
	return persistence.Save(t.config.Checkpoint, c)
}
