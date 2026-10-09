// Package session serializes team transactions and owns each player's scene assignment.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/persistence"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/progression"
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
	kind       string
	hello      protocol.Hello
	out        chan protocol.Message
	id         string
	generation uint64
	request    protocol.Request
	reply      chan joined
}
type joined struct {
	ID         string
	Generation uint64
	Err        error
}
type Coordinator struct {
	events chan event
	done   chan struct{}
}
type team struct {
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
		t.mapTemplate = content.AddSpecialNodes(t.mapTemplate, special)
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
	a := &Coordinator{events: make(chan event, 256), done: make(chan struct{})}
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
			switch e.kind {
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
		case <-logic.C:
			now := time.Now()
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
		case <-network.C:
			s := t.snapshot()
			for _, m := range t.players {
				if m.player.Online {
					t.send(m, protocol.Message{Kind: "state", State: &s})
				}
			}
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
	if r.Kind == "inventory_return" {
		_, err := t.inventoryRequest(m, r)
		return err
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
	if (t.phase == "preparation" || t.phase == "lobby") && (r.Kind == "inventory_click" || r.Kind == "equip") {
		if !m.player.Active || m.player.Life != "alive" {
			return protocol.Reject("error.incapacitated")
		}
		_, err := t.inventoryRequest(m, r)
		return err
	}
	if t.phase == "preparation" && (r.Kind == "skill" || r.Kind == "use_item") {
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
	if r.Kind == "vote" {
		if r.Accept && t.vote != nil {
			if err := t.deviceReady(m, t.vote.Node); err != nil {
				return err
			}
		}
		return t.castVote(m, r.Accept)
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
		if r.Kind == "attribute" {
			return t.invest(m, r.Node)
		}
		return protocol.Reject("error.boss_committed")
	}
	switch r.Kind {
	case "enter":
		return t.enterDoor(m, r.Node)
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
		if n.Kind != "shop" || !n.Open {
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
	case "attribute":
		return t.invest(m, r.Node)
	default:
		return protocol.Reject("error.unknown_command")
	}
}
func (t *team) invest(m *member, key string) error {
	if a := t.actors[m.player.Scene]; a != nil {
		return a.Invest(m.player.ID, key)
	}
	if m.player.Points < 1 {
		return protocol.Reject("error.no_points")
	}
	next, err := content.Invest(m.player, key)
	if err != nil {
		return err
	}
	m.player = next
	return nil
}
func (t *team) initialize(floor int) error {
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
	template = content.AddSpecialNodes(template, special)
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
		nodes[entry.ID] = model.Node{ID: entry.ID, Definition: def.ID, Name: def.Name, NameKey: def.NameKey, Room: def.Room, Cost: def.Cost, Q: entry.Q, R: entry.R, Kind: def.Kind, Payment: def.Payment, Hint: def.Hint, HintKey: def.HintKey, Edges: entry.Edges, Open: free, Completed: free}
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
func (t *team) reachable(from, to string) bool {
	if from == to {
		return true
	}
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range t.nodes[id].Edges {
			if next == to {
				return true
			}
			if !seen[next] && t.nodes[next].Completed {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}
func (t *team) enter(m *member, id string) error {
	return t.enterAt(m, id, nil)
}
func (t *team) enterAt(m *member, id string, arrival *model.Vec) error {
	if m.player.Scene != "" {
		return protocol.Reject("error.already_battle")
	}
	n, ok := t.nodes[id]
	if !ok || !t.reachable(m.player.Node, id) {
		return protocol.Reject("error.unreachable")
	}
	if id == t.bossNode() {
		return protocol.Reject("error.boss_vote")
	}
	entry := *m
	if m.player.Node != id {
		var err error
		entry.player, err = content.LeaveStatuses(m.player)
		if err != nil {
			return err
		}
	}
	if n.Completed && (n.Room != "" || n.Kind == "passage" || n.Kind == "start") {
		m.player = entry.player
		t.placeInSpace(m, id)
		if arrival != nil {
			m.player.Position = *arrival
		}
		return nil
	}
	if n.Payment == "shared" && !n.Open {
		return protocol.Reject("error.node_vote")
	}
	cost := 0
	if n.Payment == "personal" && !n.Completed {
		cost = n.Cost
	}
	if m.player.AP < cost {
		return protocol.Reject("error.no_personal_ap")
	}
	first := !m.visited[id]
	player, err := t.interact(&entry, n, "enter", first)
	if err != nil {
		return err
	}
	player.Node = id
	player.AP -= cost
	if n.Room != "" {
		player.Scene = id
	}
	// Evaluate all scripts and validate their commands before charging or adding
	// the actor. A failed hook leaves the original space and resources intact.
	if first {
		commands, hookErr := t.runtime.Dispatch(sdk.EventContext{Scope: "session", Scene: player.Scene, Floor: t.floor, Event: model.Event{Kind: "node_entered", Scene: player.Scene, Source: player.ID, Target: n.ID}, Players: []model.Player{player}})
		if hookErr != nil {
			return hookErr
		}
		next, hookErr := content.ApplyCommands(t.catalog, map[string]model.Player{player.ID: player}, commands, content.EventCommands)
		if hookErr != nil {
			return hookErr
		}
		player = next[player.ID]
	}
	if n.Room != "" {
		a := t.actors[id]
		created := a == nil
		if created {
			a, err = scene.New(t.catalog, scene.Config{ShotOrigin: t.projectileOrigin(id), ExternalClock: true, AdventureTime: t.adventureTime, TickRate: t.config.TickRate, ID: id, Room: n.Room, Floor: t.floor, Seed: content.Seed(t.config.Seed, t.floor, id+":"+n.Room)})
			if err != nil {
				return err
			}
			a.EnableProjectileTransit()
		}
		player.Scene = id
		addErr := a.AddWithActions(player, arrival, m.actions)
		if addErr != nil {
			if created {
				a.Stop()
			}
			return addErr
		}
		m.actions = scene.PlayerActions{}
		t.actors[id] = a
		if space := t.spaces[id]; space != nil {
			a.ReceiveProjectiles(space.ReleaseProjectiles())
			delete(t.spaces, id)
		}
	} else {
		n.Completed = true
		t.nodes[id] = n
	}
	if m.visited == nil {
		m.visited = map[string]bool{}
	}
	m.visited[id] = true
	m.player = player
	m.walk = model.Input{}
	if player.Scene == "" {
		t.placeInSpace(m, id)
		if arrival != nil {
			m.player.Position = *arrival
		}
	}
	if a := t.actors[id]; a != nil {
		t.merge(a.Snapshot())
		player = m.player
	}
	return nil
}
func (t *team) propose(m *member, id string) error {
	if t.bossRoute != "" {
		return protocol.Reject("error.boss_committed")
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
		if !ok || n.Payment != "shared" || n.Open || !t.reachable(m.player.Node, id) || m.player.Scene != "" {
			return protocol.Reject("error.invalid_shared_node")
		}
		if t.shared < n.Cost {
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
	return t.castVote(m, true)
}
func (t *team) castVote(m *member, yes bool) error {
	v := t.vote
	if v == nil {
		return protocol.Reject("error.no_vote")
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
	if yes {
		v.Yes = append(v.Yes, m.player.ID)
	} else {
		v.No = append(v.No, m.player.ID)
	}
	if len(v.Yes) >= v.Required {
		id := v.Node
		t.vote = nil
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
			n := t.nodes[id]
			if t.shared < n.Cost {
				return protocol.Reject("error.no_shared_ap")
			}
			t.shared -= n.Cost
			n.Open = true
			t.nodes[id] = n
			return nil
		}
	}
	if len(v.No) > len(v.Eligible)-v.Required {
		t.vote = nil
	}
	return nil
}
func (t *team) normalBossReady() error {
	if len(t.actors) > 0 {
		return protocol.Reject("error.battles_pending")
	}
	for _, m := range t.players {
		p := m.player
		if p.Active && p.Life == "alive" && !t.reachable(p.Node, t.bossNode()) {
			return protocol.Reject("error.boss_route")
		}
	}
	return nil
}
func (t *team) startBoss(enhanced bool) error {
	if t.bossRoute != "" {
		return protocol.Reject("error.boss_committed")
	}
	if !enhanced && t.shared < t.nodes[t.bossNode()].Cost {
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
	a.EnableProjectileTransit()
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
		t.shared -= t.nodes[boss].Cost
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
			n := t.nodes[id]
			n.Completed = true
			t.nodes[id] = n
			for _, p := range s.Players {
				m := t.players[p.ID]
				m.actions = departure.Actions[p.ID]
				m.player.Scene = ""
				oldArena := geometry.NewArena(s.Width, s.Height)
				newArena := t.spaceArena(id)
				m.player.Position = newArena.ClampDisk(newArena.Center.Add(p.Position.Sub(oldArena.Center).Scale(newArena.Radius/oldArena.Radius)), 12)
				m.walk = model.Input{}
				for m.player.XP >= progression.NextLevelXP(m.player.Level) {
					m.player.XP -= progression.NextLevelXP(m.player.Level)
					m.player.Level++
					m.player.Points++
				}
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
	s := protocol.State{AdventureTime: t.adventureTime, ServerTick: t.serverTick, TickRate: t.config.TickRate, SnapshotRate: t.config.SnapshotRate, ActualTPS: t.actualTPS.Rate(), DroppedTicks: t.droppedTicks, Phase: t.phase, Floor: t.floor, Floors: len(t.catalog.Run().Floors), SharedAP: t.shared, PersonalInitial: t.initialX, SharedInitial: t.initialY, Cursed: t.cursed, Admin: t.admin}
	s.Trades = t.tradeSnapshot()
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
		if len(view.Projectiles) > 0 {
			s.Scenes = append(s.Scenes, view)
		}
	}
	if t.vote != nil {
		v := *t.vote
		v.Eligible = append([]string(nil), v.Eligible...)
		v.Yes = append([]string(nil), v.Yes...)
		v.No = append([]string(nil), v.No...)
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
	// Checkpoints contain layer preparation, not transient practice projectiles.
	var battles []protocol.Scene
	for _, view := range c.State.Scenes {
		if view.State != "idle" {
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
