// Package scene owns combat state in one goroutine per battle scene.
package scene

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
	"hsmultiplayer/pkg/timing"
	"hsmultiplayer/sdk"
)

const tickRate = timing.DefaultLogicRate

type command struct {
	inventory     protocol.Request
	actions       PlayerActions
	transfers     []ProjectileTransfer
	ids           []string
	players       []model.Player
	adventureTime int64
	arrival       *model.Vec
	effects       []sdk.Command
	kind          string
	player        model.Player
	id            string
	input         model.Input
	reply         chan response
}
type response struct {
	departure Departure
	transfers []ProjectileTransfer
	players   []model.Player
	snapshot  protocol.Scene
	err       error
}
type Actor struct {
	commands chan command
	done     chan struct{}
}
type fighter struct {
	attack       *preparedAttack
	player       model.Player
	input        model.Input
	updated      uint64
	cooldown     float64
	invulnerable float64
	dodgeTime    float64
	dodgeDir     model.Vec
	recharge     float64
	contact      map[int]float64
}
type shot struct {
	effects        []model.CombatContribution
	critMultiplier float64
	identity       string
	sweepStart     *model.Vec
	source         model.SourceRef
	hits           map[string]bool
	remaining      int
	attack         bool
	statInput      *stats.Input
	flatBonuses    []damage.Bonus
	finalSource    model.SourceRef
	view           model.Projectile
	weapon         model.Weapon
	attributes     model.Attributes
	multiplier     float64
	critical       bool
}
type Config struct {
	ShotOrigin    string
	ExternalClock bool
	AdventureTime int64
	TickRate      int
	ID            string
	Room          string
	Floor         int
	Seed          int64
}

type world struct {
	enemyDefinitions map[string]model.EnemyDef
	barrierClaimed   map[string]bool
	enemyAttacks     map[int]enemyAttackState
	footprint        *geometry.Footprint
	shotOrigin       string
	transit          bool
	outgoing         []ProjectileTransfer
	waveTime         float64
	paths            map[int]enemyPath
	threat           map[int]map[string]float64
	enemyFire        map[int]float64
	externalClock    bool
	adventureTime    int64
	logicRate        int
	actualTPS        timing.Meter
	droppedTicks     uint64
	room             model.RoomDef
	pending          []sdk.Command
	id               string
	catalog          *content.Catalog
	runtime          *script.Runtime
	rng              *rand.Rand
	players          map[string]*fighter
	enemies          []model.Enemy
	shots            []shot
	state            string
	pausedFrom       string
	wave             int
	waves            int
	floor            int
	tick             uint64
	clock            float64
	breakTime        float64
	nextID           int
	lastDamage       *damage.Result
	fault            string
}

func New(c *content.Catalog, cfg Config) (*Actor, error) {
	if cfg.TickRate == 0 {
		cfg.TickRate = tickRate
	}
	if !timing.LogicRate(cfg.TickRate) {
		return nil, fmt.Errorf("logic rate must be 30 or 60")
	}
	room, ok := c.Room(cfg.Room)
	if !ok || cfg.ID == "" || cfg.Floor < 1 || cfg.Floor > len(c.Run().Floors) {
		return nil, fmt.Errorf("invalid scene or room %s", cfg.Room)
	}
	vm, err := script.New(c, cfg.Seed)
	if err != nil {
		return nil, err
	}
	w := &world{shotOrigin: cfg.ShotOrigin, externalClock: cfg.ExternalClock, adventureTime: cfg.AdventureTime, logicRate: cfg.TickRate, id: cfg.ID, room: room, catalog: c, runtime: vm, rng: rand.New(rand.NewSource(cfg.Seed)), players: map[string]*fighter{}, state: "waiting", waves: len(room.Waves), floor: cfg.Floor}
	a := &Actor{commands: make(chan command, 128), done: make(chan struct{})}
	go a.run(w)
	return a, nil
}
func (a *Actor) call(c command) response {
	c.reply = make(chan response, 1)
	select {
	case a.commands <- c:
	case <-a.done:
		return response{err: protocol.Reject("error.scene_stopped")}
	}
	select {
	case r := <-c.reply:
		return r
	case <-a.done:
		// Stop may close done after placing its final snapshot in the reply buffer.
		select {
		case r := <-c.reply:
			return r
		default:
			return response{err: protocol.Reject("error.scene_stopped")}
		}
	}
}
func (a *Actor) Apply(commands []sdk.Command) error {
	return a.call(command{kind: "effects", effects: sdk.CloneCommands(commands)}).err
}
func (a *Actor) AdvanceStatuses(now int64) { a.call(command{kind: "status_time", adventureTime: now}) }
func (a *Actor) Add(p model.Player) error {
	return a.call(command{kind: "add", player: model.ClonePlayer(p)}).err
}
func (a *Actor) AddAt(p model.Player, position model.Vec) error {
	return a.call(command{kind: "add", player: model.ClonePlayer(p), arrival: &position}).err
}
func (a *Actor) Input(id string, in model.Input) { a.call(command{kind: "input", id: id, input: in}) }
func (a *Actor) Invest(id, key string) error {
	return a.call(command{kind: "attribute", id: id, player: model.Player{Node: key}}).err
}
func (a *Actor) Online(id string, online bool) {
	a.call(command{kind: "online", id: id, input: model.Input{Attack: online}})
}
func (a *Actor) Snapshot() protocol.Scene { return a.call(command{kind: "snapshot"}).snapshot }
func (a *Actor) Stop() protocol.Scene {
	return a.StopWithActions().Scene
}
func (a *Actor) run(w *world) {
	defer close(a.done)
	defer w.runtime.Close()
	stepper := timing.NewStepper(w.rate(), time.Now())
	ticker := time.NewTicker(time.Second / time.Duration(w.rate()))
	defer ticker.Stop()
	for {
		select {
		case c := <-a.commands:
			r := response{}
			switch c.kind {
			case "projectile_transit":
				w.transit = true
			case "take_projectiles":
				r.transfers, w.outgoing = w.outgoing, nil
			case "receive_projectiles", "sweep_projectiles":
				for _, transfer := range c.transfers {
					w.receiveProjectile(transfer)
				}
				if c.kind == "sweep_projectiles" {
					if w.state != "paused" && w.state != "faulted" && w.state != "cleared" {
						r.err = w.updateShots(0)
						if r.err == nil && !w.commitEvents() {
							r.err = fmt.Errorf("%s", w.fault)
						}
						if r.err != nil {
							w.state, w.fault = "faulted", r.err.Error()
						}
					}
					r.transfers, w.outgoing = w.outgoing, nil
				}
			case "action":
				r.err = w.action(c.id, c.player.Node, c.player.Upgrade, c.input.Aim)
			case "pickup":
				if p := w.players[c.id]; p != nil && p.player.Life == "alive" {
					n, err := w.catalog.Pickup(p.player, c.player.Weapon)
					r.err = err
					if err == nil {
						p.player = n
					}
				} else {
					r.err = fmt.Errorf("invalid loot participant")
				}
			case "inventory_mouse":
				if p := w.players[c.id]; p != nil {
					if c.inventory.Kind != "inventory_return" && c.inventory.Area != "bag" && w.catalog.Weapon(p.player.Weapon).RestrictSwap && p.cooldown > 0 {
						r.err = protocol.Reject("error.equipment_cooldown")
						break
					}
					next, err := w.catalog.InventoryMouse(p.player, c.inventory)
					r.err = err
					if err == nil {
						p.player = next
						r.players = []model.Player{model.ClonePlayer(next)}
					}
				} else {
					r.err = protocol.Reject("error.incapacitated")
				}
			case "equip":
				if p := w.players[c.id]; p != nil && p.player.Life == "alive" {
					if w.catalog.Weapon(p.player.Weapon).RestrictSwap && p.cooldown > 0 {
						r.err = protocol.Reject("error.equipment_cooldown")
						break
					}
					next, err := w.catalog.Equip(p.player, c.player.Weapon, c.player.Upgrade)
					r.err = err
					if err == nil {
						p.player = next
					}
				} else {
					r.err = protocol.Reject("error.incapacitated")
				}
			case "prepare_transaction":
				a.transaction(w, c)
				continue
			case "status_time":
				w.advanceStatuses(c.adventureTime)
			case "add", "support":
				if _, ok := w.players[c.player.ID]; ok {
					r.err = protocol.Reject("error.already_scene")
				} else if c.kind != "support" && (w.state == "fighting" || w.state == "between_waves" && w.room.Join != "between_waves") {
					r.err = protocol.Reject("error.join_window")
				} else if w.state == "cleared" || w.state == "faulted" {
					r.err = protocol.Reject("error.scene_finished")
				} else {
					p := model.ClonePlayer(c.player)
					if p.Statuses.Time > w.adventureTime {
						w.advanceStatuses(p.Statuses.Time)
					}
					var statusErr error
					p, statusErr = content.AdvancePlayerStatuses(p, w.adventureTime)
					if statusErr != nil {
						r.err = statusErr
						break
					}
					if w.barrierClaimed == nil {
						w.barrierClaimed = map[string]bool{}
					}
					if !w.barrierClaimed[p.ID] {
						p, statusErr = content.EnterBattle(w.catalog, p)
						if statusErr != nil {
							r.err = statusErr
							break
						}
					}
					p.Scene = w.id
					p.PositionEpoch++
					p.Position = geometry.NewArena(w.room.Width, w.room.Height).Spawn(len(w.players), 12)
					if c.arrival != nil {
						if !geometry.NewArena(w.room.Width, w.room.Height).ContainsDisk(*c.arrival, 12) {
							r.err = protocol.Reject("error.invalid_input")
							break
						}
						p.Position = *c.arrival
					}
					f := c.actions.state
					f.player, f.updated, f.contact = p, w.tick, map[int]float64{}
					if p.Life != "alive" {
						f.attack, f.dodgeTime, f.invulnerable = nil, 0, 0
					}
					w.players[p.ID] = &f
					w.barrierClaimed[p.ID] = true
					if w.state == "paused" {
						w.state = w.pausedFrom
						if w.state == "" {
							w.state = "fighting"
						}
					}
				}
			case "input":
				if p := w.players[c.id]; p != nil && c.input.Move.Finite() && c.input.Aim.Finite() {
					c.input.Dodge = c.input.Dodge || p.input.Dodge
					p.input = c.input
					p.updated = w.tick
				}
			case "online":
				if p := w.players[c.id]; p != nil {
					p.player.Online = c.input.Attack
					p.input = model.Input{}
				}
			case "effects":
				players := map[string]model.Player{}
				for id, p := range w.players {
					players[id] = p.player
				}
				next, err := content.ApplyCommands(w.catalog, players, c.effects, content.EventCommands)
				r.err = err
				if err == nil {
					for id, p := range next {
						w.players[id].player = p
					}
				}
			case "snapshot":
				r.snapshot = w.snapshot()
			case "attribute":
				p := w.players[c.id]
				if p == nil || p.player.Life != "alive" || p.player.Points < 1 {
					r.err = protocol.Reject("error.no_points")
				} else {
					next, err := content.Invest(p.player, c.player.Node)
					r.err = err
					if err == nil {
						p.player = next
					}
				}
			case "stop":
				r.snapshot = w.snapshot()
				r.departure = Departure{Scene: r.snapshot, Actions: map[string]PlayerActions{}, Projectiles: w.outgoing}
				for id, f := range w.players {
					r.departure.Actions[id] = (PlayerActions{state: *f}).Clone()
				}
				for _, s := range w.shots {
					previous := s.view.Position
					if s.sweepStart != nil {
						previous = *s.sweepStart
					}
					r.departure.Projectiles = append(r.departure.Projectiles, transferShot(s, previous))
				}
				c.reply <- r
				return
			}
			c.reply <- r
		case <-ticker.C:
			now := time.Now()
			steps := stepper.Advance(now)
			for i := 0; i < steps; i++ {
				w.step(1.0 / float64(w.rate()))
			}
			w.actualTPS.Add(steps, now)
			w.droppedTicks = stepper.Dropped
		}
	}
}
func (w *world) rate() int {
	if w.logicRate == 0 {
		return tickRate
	}
	return w.logicRate
}

func (w *world) step(dt float64) {
	w.tick++
	if !w.externalClock {
		w.advanceStatuses(w.adventureTime + status.UnitsPerSecond/int64(w.rate()))
	}
	if w.state == "cleared" || w.state == "faulted" {
		return
	}
	if w.state == "paused" {
		return
	}
	if len(w.players) == 0 {
		return
	}
	living := 0
	for _, p := range w.players {
		if p.player.Life == "alive" {
			living++
		}
	}
	if living == 0 {
		w.pausedFrom = w.state
		w.state = "paused"
		return
	}
	w.clock += dt
	if w.state == "waiting" {
		w.breakTime += dt
		if w.breakTime >= w.room.InitialDelay {
			w.spawn()
			w.state = "fighting"
		}
	}
	if w.state == "between_waves" {
		w.breakTime -= dt
		if w.breakTime <= 0 {
			w.spawn()
			w.state = "fighting"
		}
	}
	for _, id := range w.playerIDs() {
		if err := w.stepPlayer(w.players[id], dt, nil); err != nil {
			w.state, w.fault = "faulted", err.Error()
			return
		}
	}
	if err := w.updateEnemies(dt); err != nil {
		w.state, w.fault = "faulted", err.Error()
		return
	}
	if err := w.updateShots(dt); err != nil {
		w.state, w.fault = "faulted", err.Error()
		return
	}
	alive := w.enemies[:0]
	for _, e := range w.enemies {
		if e.HP > 0 {
			alive = append(alive, e)
		}
	}
	w.enemies = alive
	anyLiving := false
	for _, p := range w.players {
		if p.player.Life == "alive" {
			anyLiving = true
			break
		}
	}
	if !anyLiving {
		w.pending = nil
		w.pausedFrom = w.state
		w.state = "paused"
		return
	}
	if !w.commitEvents() {
		return
	}
	if w.state != "fighting" {
		return
	}
	w.waveTime += dt
	if w.room.Objective == "clear" && len(w.enemies) == 0 || w.room.Objective == "survive" && w.waveTime >= w.room.ObjectiveSeconds {
		w.enemies = nil
		w.enemyFire = nil
		w.enemyAttacks = nil
		w.threat = nil
		w.paths = nil
		reward := w.room.Waves[w.wave-1].Reward
		for _, id := range w.playerIDs() {
			p := w.players[id]
			if p.player.Life != "dead" {
				xp, coins := reward.XP.Resolve(w.floor, len(w.players)), reward.Coins.Resolve(w.floor, len(w.players))
				if reward.XPPolicy == "total" {
					xp /= len(w.players)
				}
				if reward.CoinsPolicy == "total" {
					coins /= len(w.players)
				}
				p.player.XP += xp
				p.player.Coins += coins
				drops := append([]model.DropDef(nil), reward.Items...)
				for _, draw := range reward.Draws {
					for roll := 0; roll < draw.Rolls; roll++ {
						if w.rng.Float64() < draw.Chance {
							drops = append(drops, model.DropDef{Item: content.SelectLoot(draw.Items, w.rng.Float64()), Chance: 1})
						}
					}
				}
				for _, drop := range drops {
					if w.rng.Float64() < drop.Chance {
						n, err := w.catalog.Drop(p.player, drop.Item, w.id, p.player.Position, w.rng.Float64())
						if err != nil {
							w.state, w.fault = "faulted", err.Error()
							return
						}
						p.player = n
					}
				}
			}
		}
		if !w.emit(model.Event{Kind: "wave_completed", Amount: float64(w.wave), Tick: w.tick}) || !w.commitEvents() {
			return
		}
		if w.wave >= w.waves {
			w.state = "cleared"
		} else {
			w.state = "between_waves"
			w.breakTime = w.room.Waves[w.wave-1].BreakSeconds
		}
	}
}
func (w *world) spawn() {
	w.waveTime = 0
	w.wave++
	w.breakTime = 0
	groups := w.room.Waves[w.wave-1].Enemies
	count := 0
	for _, group := range groups {
		count += group.Count.Resolve(w.floor, len(w.players))
	}
	index := 0
	for _, group := range groups {
		def := w.catalog.Enemy(group.Enemy)
		for i := 0; i < group.Count.Resolve(w.floor, len(w.players)); i++ {
			hp := def.HP * w.room.Scaling.HPFactor * (1 + w.room.Scaling.HPPerFloor*float64(w.floor-1)) * (1 + w.room.Scaling.HPPerExtraPlayer*float64(len(w.players)-1))
			if w.room.Scaling.HPVariation > 0 {
				hp *= 1 + (w.rng.Float64()*2-1)*w.room.Scaling.HPVariation
			}
			hp = damage.Round(hp)
			w.nextID++
			angle := 2 * math.Pi * float64(index) / float64(count)
			pos := model.Vec{X: w.room.Width/2 + math.Cos(angle)*w.room.SpawnRadius.X, Y: w.room.Height/2 + math.Sin(angle)*w.room.SpawnRadius.Y}
			w.enemies = append(w.enemies, model.Enemy{ID: w.nextID, Definition: def.ID, Position: w.clamp(pos, def.Radius), HP: hp, MaxHP: hp, Radius: def.Radius})
			index++
		}
	}
}
func (w *world) emit(event model.Event) bool {
	event.Scene = w.id
	players := []model.Player{}
	for _, id := range w.playerIDs() {
		players = append(players, w.players[id].player)
	}
	commands, err := w.runtime.Dispatch(sdk.EventContext{Scope: "scene", Scene: w.id, Floor: w.floor, Time: w.clock, Event: event, Players: players})
	if err != nil || len(w.pending)+len(commands) > 128 {
		w.state = "faulted"
		if err == nil {
			err = fmt.Errorf("scene command budget exceeded")
		}
		w.fault = err.Error()
		w.pending = nil
		return false
	}
	w.pending = append(w.pending, commands...)
	return true
}
func (w *world) commitEvents() bool {
	if len(w.pending) == 0 {
		return true
	}
	players := map[string]model.Player{}
	for id, p := range w.players {
		players[id] = p.player
	}
	next, err := content.ApplyCommands(w.catalog, players, w.pending, content.EventCommands)
	w.pending = nil
	if err != nil {
		w.state = "faulted"
		w.fault = err.Error()
		return false
	}
	for id, p := range next {
		w.players[id].player = p
	}
	return true
}
func (w *world) clamp(p model.Vec, r float64) model.Vec {
	if w.footprint != nil {
		return w.footprint.ClampDisk(p, r)
	}
	return geometry.NewArena(w.room.Width, w.room.Height).ClampDisk(p, r)
}
func (w *world) playerIDs() []string {
	ids := make([]string, 0, len(w.players))
	for id := range w.players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (w *world) snapshot() protocol.Scene {
	s := protocol.Scene{AdventureTime: w.adventureTime, TickRate: w.rate(), ActualTPS: w.actualTPS.Rate(), DroppedTicks: w.droppedTicks, ID: w.id, Room: w.room.ID, State: w.state, Width: w.room.Width, Height: w.room.Height, Wave: w.wave, Waves: w.waves, Tick: w.tick, Enemies: append([]model.Enemy(nil), w.enemies...), Error: w.fault}
	s.Obstacles = append([]model.Obstacle(nil), w.room.Obstacles...)
	if w.room.Objective == "survive" {
		s.ObjectiveRemaining = max(0, w.room.ObjectiveSeconds-w.waveTime)
	}
	if w.state == "waiting" {
		s.Countdown = max(0, w.room.InitialDelay-w.breakTime)
	}
	if w.state == "between_waves" {
		s.Countdown = max(0, w.breakTime)
	}
	for i := range s.Enemies {
		s.Enemies[i] = model.CloneEnemy(s.Enemies[i])
	}
	for _, id := range w.playerIDs() {
		s.Players = append(s.Players, model.ClonePlayer(w.players[id].player))
	}
	for _, p := range w.shots {
		view := p.view
		view.Identity = w.shotIdentity(p)
		s.Projectiles = append(s.Projectiles, view)
	}
	if w.lastDamage != nil {
		r := damage.CloneResult(*w.lastDamage)
		s.LastDamage = &r
	}
	return s
}
func segmentDistance(a, b, p model.Vec) float64 {
	ab := b.Sub(a)
	den := ab.X*ab.X + ab.Y*ab.Y
	if den == 0 {
		return p.Sub(a).Length()
	}
	v := p.Sub(a)
	t := math.Max(0, math.Min(1, (v.X*ab.X+v.Y*ab.Y)/den))
	return p.Sub(a.Add(ab.Scale(t))).Length()
}

func (w *world) canMove(from, to model.Vec) bool {
	if !w.clearObstacles(from, to, 12) {
		return false
	}
	for _, e := range w.enemies {
		if e.HP > 0 && !geometry.DiskStepClear(from, to, e.Position, e.Radius+12) {
			return false
		}
	}
	return true
}
