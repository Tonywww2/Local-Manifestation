package scene

import (
	"fmt"
	"math"
	"math/rand"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/pkg/timing"
)

// PlayerActions contains transient controls, never an authoritative player.
// Its owner passes a detached value when a player changes simulation hosts.
type PlayerActions struct{ state fighter }

// FreeSpace is an action host with no enemies, waves, rewards or script VM.
// The caller's goroutine owns it; it shares weapon/movement code with battles.
type FreeSpace struct{ world world }

func NewShapedFreeSpace(c *content.Catalog, cfg Config, room model.RoomDef, footprint geometry.Footprint) (*FreeSpace, error) {
	if !footprint.Valid() {
		return nil, fmt.Errorf("invalid free-space footprint")
	}
	s, err := NewFreeSpace(c, cfg, room)
	if err != nil {
		return nil, err
	}
	s.world.footprint = &footprint
	return s, nil
}

func NewFreeSpace(c *content.Catalog, cfg Config, room model.RoomDef) (*FreeSpace, error) {
	if cfg.TickRate == 0 {
		cfg.TickRate = timing.DefaultLogicRate
	}
	if c == nil || cfg.ID == "" || !timing.LogicRate(cfg.TickRate) || room.Width <= 0 || room.Height <= 0 || math.IsNaN(room.Width+room.Height) || math.IsInf(room.Width+room.Height, 0) {
		return nil, fmt.Errorf("invalid free space")
	}
	arena, err := geometry.NewRoomArena(room.Width, room.Height, room.Hexes)
	if err != nil {
		return nil, err
	}
	room.Hexes = append([]model.HexCell(nil), room.Hexes...)
	room.Obstacles = append([]model.Obstacle(nil), room.Obstacles...)
	return &FreeSpace{world: world{arena: arena, shotOrigin: cfg.ShotOrigin, adventureTime: cfg.AdventureTime, catalog: c, room: room, id: cfg.ID, floor: cfg.Floor, logicRate: cfg.TickRate, state: "idle", rng: rand.New(rand.NewSource(cfg.Seed))}}, nil
}

func (s *FreeSpace) StepPlayer(controls *PlayerActions, player model.Player, input model.Input, dt float64, open []int) (model.Player, error) {
	f := &controls.state
	f.player, f.input, f.updated = player, input, s.world.tick
	// Do not retain a borrowed player's nested inventory/status references.
	defer func() { f.player, f.input = model.Player{}, model.Input{} }()
	if err := s.world.stepPlayer(f, dt, open); err != nil {
		return player, err
	}
	return f.player, nil
}

func (s *FreeSpace) Launch(player model.Player, aim model.Vec, request damage.Request) error {
	direction := aim.Sub(player.Position).Unit()
	if !aim.Finite() || direction.Length() == 0 {
		return fmt.Errorf("invalid aim")
	}
	s.world.launch(player.ID, player.Position, direction, request, false, false)
	return nil
}

func (s *FreeSpace) ApplyAction(player model.Player, aim model.Vec, plan content.ActionPlan) error {
	return s.world.applyAction(player, aim, plan)
}

func (s *FreeSpace) Advance(dt float64) error {
	s.world.tick++
	s.world.clock += dt
	s.world.advanceEnemyVisuals(dt)
	return s.world.updateShots(dt)
}

// The coordinator and these spaces share one goroutine, so the session VM has
// one owner even when an enemy projectile hits a player outside combat.
func (s *FreeSpace) SetRuntime(vm *script.Runtime) { s.world.runtime = vm }

type SpacePlayer struct {
	Player  model.Player
	Actions *PlayerActions
}

func (s *FreeSpace) AdvancePlayers(players []SpacePlayer, dt float64, now int64) ([]model.Player, error) {
	s.world.adventureTime = now
	s.world.tick++
	s.world.clock += dt
	s.world.advanceEnemyVisuals(dt)
	for _, p := range players {
		if p.Player.Life != "alive" {
			p.Actions.state.player = p.Player
			p.Actions.state.syncVisualLife()
			p.Actions.state.visual.Life = animation.AdvanceAction(p.Actions.state.visual.Life, dt)
			p.Actions.state.player = model.Player{}
		}
	}
	return s.sweepPlayers(players, dt)
}

// Carry final visual facts across a battle-to-free-space ownership transition.
// These records contain no Enemy, collision, reward or script references.
func (s *FreeSpace) ReceiveEnemyVisuals(values []model.EnemyActionState) {
	for _, v := range values {
		if animation.Active(v.Death) {
			s.world.setEnemyVisual(v)
		}
	}
}

// Incoming sweeps settle the already elapsed part of a step without ticking
// TTL, movement, recharge or the simulation clock a second time.
func (s *FreeSpace) SweepProjectiles(shots []ProjectileTransfer, players []SpacePlayer, now int64) ([]model.Player, []ProjectileTransfer, error) {
	s.world.adventureTime = now
	s.ReceiveProjectiles(shots)
	values, err := s.sweepPlayers(players, 0)
	return values, s.TakeProjectiles(), err
}

func (s *FreeSpace) sweepPlayers(players []SpacePlayer, dt float64) ([]model.Player, error) {
	s.world.players = make(map[string]*fighter, len(players))
	for _, p := range players {
		p.Actions.state.player = p.Player
		s.world.players[p.Player.ID] = &p.Actions.state
	}
	defer func() {
		for _, p := range players {
			p.Actions.state.player = model.Player{}
		}
		s.world.players = nil
	}()
	err := s.world.updateShots(dt)
	if err == nil && !s.world.commitEvents() {
		err = fmt.Errorf("%s", s.world.fault)
	}
	result := make([]model.Player, 0, len(players))
	for _, p := range players {
		result = append(result, p.Actions.state.player)
	}
	return result, err
}

// Snapshot copies only display values. Player ownership stays with the caller.
func (s *FreeSpace) Snapshot() protocol.Scene {
	return s.world.snapshot()
}
