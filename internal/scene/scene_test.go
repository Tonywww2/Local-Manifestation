package scene

import (
	"fmt"
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"math/rand"
	"sync"
	"testing"
)

func TestPauseAndClearRules(t *testing.T) {
	c, _ := content.Load(base.Files)
	vm, err := script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vm.Close)
	room, _ := c.Room("base:skirmish")
	p, _ := content.Player(c, "p", "p", "base:scout")
	p.Life = "down"
	w := &world{room: room, id: "a", catalog: c, runtime: vm, players: map[string]*fighter{"p": {player: p, contact: map[int]float64{}}}, state: "fighting", waves: 2, wave: 1, enemies: []model.Enemy{{ID: 1, HP: 10}}}
	w.step(1.0 / 30)
	if w.state != "paused" || len(w.enemies) != 1 {
		t.Fatal("did not preserve battle")
	}
	clock := w.clock
	w.step(1.0 / 30)
	if w.clock != clock {
		t.Fatal("paused scene advanced script time")
	}
	s := w.snapshot()
	s.Players[0].HP = 99
	if w.players["p"].player.HP == 99 {
		t.Fatal("snapshot mutated world")
	}
}
func TestSegmentCollisionPreventsProjectileTunneling(t *testing.T) {
	d := segmentDistance(model.Vec{X: 0}, model.Vec{X: 100}, model.Vec{X: 50, Y: 5})
	if d != 5 {
		t.Fatal(d)
	}
}

func TestLastShotDoesNotOverrideWholePartyDefeat(t *testing.T) {
	c, _ := content.Load(base.Files)
	vm, err := script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vm.Close)
	room, _ := c.Room("base:skirmish")
	p, _ := content.Player(c, "p", "p", "base:scout")
	p.HP = 1
	p.Position = model.Vec{X: 400, Y: 250}
	w := &world{room: room, id: "boss", catalog: c, runtime: vm, players: map[string]*fighter{"p": {player: p, contact: map[int]float64{}}}, state: "fighting", waves: 1, wave: 1, floor: 1,
		enemies: []model.Enemy{{ID: 1, Definition: "base:stalker", HP: 1, Radius: 13, Position: p.Position}},
		shots:   []shot{{view: model.Projectile{Owner: "p", Position: p.Position, TTL: 1}, weapon: c.Weapon(p.Weapon), attributes: p.Attributes, multiplier: 1}}}
	w.step(1.0 / 30)
	if w.players["p"].player.Life != "down" || w.state != "paused" || w.players["p"].player.XP != 0 {
		t.Fatal("credited victory after party incapacitated")
	}
}

func templateWorld(t testing.TB, c *content.Catalog, roomID string) *world {
	t.Helper()
	room, ok := c.Room(roomID)
	if !ok {
		t.Fatal("missing room")
	}
	vm, err := script.New(c, 3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vm.Close)
	w := &world{id: "custom", room: room, catalog: c, runtime: vm, rng: rand.New(rand.NewSource(3)), players: map[string]*fighter{}, state: "waiting", floor: 2, waves: len(room.Waves)}
	for _, id := range []string{"a", "b"} {
		p, _ := content.Player(c, id, id, "base:scout")
		p.Position = model.Vec{X: room.Width / 2, Y: room.Height / 2}
		w.players[id] = &fighter{player: p, contact: map[int]float64{}}
	}
	return w
}
func TestConfiguredWavesStableReferencesAndRewards(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		d.Enemies[0], d.Enemies[1] = d.Enemies[1], d.Enemies[0]
		room := a.Rooms[0]
		room.ID = "base:custom"
		room.Width = 1000
		room.Height = 300
		room.SpawnRadius = model.Vec{X: 200, Y: 100}
		room.InitialDelay = 0
		wave := model.WaveDef{Enemies: []model.SpawnGroupDef{{Enemy: "base:warden", Count: model.CountDef{Base: 1}}, {Enemy: "base:stalker", Count: model.CountDef{Base: 1, PerPlayer: 1}}}, BreakSeconds: .25, Reward: model.RewardDef{XP: model.CountDef{Base: 7, PerPlayer: 3, PerFloor: 2}, Coins: model.CountDef{Base: 4}}}
		room.Waves = []model.WaveDef{wave, wave}
		a.Rooms = append(a.Rooms, room)
	}, `sdk.on("base:bonus","scene","wave_completed",function(ctx){return [{kind:"grant_coins",target:ctx.players[0].id,amount:2}];});`)
	w := templateWorld(t, c, "base:custom")
	w.step(.1)
	if w.state != "fighting" || w.wave != 1 || len(w.enemies) != 4 || w.enemies[0].Definition != "base:warden" {
		t.Fatal(w.snapshot())
	}
	for _, e := range w.enemies {
		if e.Position.X < 0 || e.Position.X > 1000 || e.Position.Y < 0 || e.Position.Y > 300 {
			t.Fatal("spawn outside room")
		}
	}
	for i := range w.enemies {
		w.enemies[i].HP = 0
	}
	w.step(.1)
	if w.state != "between_waves" || w.players["a"].player.XP != 17 || w.players["a"].player.Coins != 6 || w.players["b"].player.Coins != 4 {
		t.Fatal(w.snapshot())
	}
	w.step(.1)
	if w.players["a"].player.XP != 17 {
		t.Fatal("duplicate wave reward")
	}
	w.step(.2)
	if w.wave != 2 || w.state != "fighting" {
		t.Fatal("did not advance configured waves")
	}
	for i := range w.enemies {
		w.enemies[i].HP = 0
	}
	w.step(.1)
	w.step(.1)
	if w.state != "cleared" || w.players["a"].player.XP != 34 || w.players["a"].player.Coins != 12 {
		t.Fatal(w.snapshot())
	}
}
func TestInvalidEventBatchDoesNotCommit(t *testing.T) {
	c := testcontent.Load(t, nil, `sdk.on("base:bad-batch","scene","hit",function(ctx){return [{kind:"grant_coins",target:ctx.players[0].id,amount:3},{kind:"grant_coins",target:"other-scene-player",amount:3}];});`)
	w := templateWorld(t, c, "base:skirmish")
	if !w.emit(model.Event{Kind: "hit"}) {
		t.Fatal(w.fault)
	}
	if w.commitEvents() || w.state != "faulted" || w.players["a"].player.Coins != 0 {
		t.Fatal("partial invalid commands committed")
	}
}
func TestJoinWindowsAndPausedRescue(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	for _, roomID := range []string{"base:skirmish", "base:guardian"} {
		for _, state := range []string{"waiting", "fighting", "between_waves", "paused", "cleared"} {
			t.Run(roomID+"/"+state, func(t *testing.T) {
				w := templateWorld(t, c, roomID)
				w.players = map[string]*fighter{}
				w.state = state
				actor := &Actor{commands: make(chan command, 1), done: make(chan struct{})}
				go actor.run(w)
				p, _ := content.Player(c, "new", "new", "base:scout")
				err := actor.Add(p)
				allowed := state == "waiting" || state == "paused" || state == "between_waves" && roomID == "base:skirmish"
				if (err == nil) != allowed {
					t.Fatal(state, err)
				}
				result := actor.Stop()
				if allowed && len(result.Players) != 1 {
					t.Fatal("lost final stop snapshot")
				}
			})
		}
	}
}
func TestDamageFactsUseActualLossAndStableIdentity(t *testing.T) {
	c := testcontent.Load(t, nil, `sdk.on("base:actual-loss","scene","after_damage",function(ctx){if(!ctx.event.attack_id||!ctx.event.hit_id||!ctx.event.segment_id||ctx.event.damage_type!=="physical")throw Error("identity missing");return [{kind:"grant_coins",target:ctx.event.source,amount:ctx.event.amount}];});`)
	w := templateWorld(t, c, "base:skirmish")
	w.state = "fighting"
	w.wave = 1
	p := w.players["a"].player
	w.enemies = []model.Enemy{{ID: 10, Definition: "base:stalker", Position: p.Position.Add(model.Vec{X: 100}), HP: 1, MaxHP: 1, Radius: 13}}
	w.shots = []shot{{view: model.Projectile{ID: 20, Owner: "a", Position: w.enemies[0].Position, TTL: 1}, weapon: c.Weapon(p.Weapon), attributes: p.Attributes, multiplier: 1}}
	w.step(.01)
	if w.state == "faulted" || w.players["a"].player.Coins != 11 {
		t.Fatal("after_damage credited overkill or lost identity", w.snapshot())
	}
}
func BenchmarkConcurrentSceneStep(b *testing.B) {
	c := testcontent.Load(b, nil, "")
	for _, count := range []int{1, 8} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			worlds := make([]*world, count)
			for i := range worlds {
				w := templateWorld(b, c, "base:skirmish")
				w.id = fmt.Sprint(i)
				w.spawn()
				w.state = "fighting"
				for _, p := range w.players {
					p.player.HP = 1e9
					p.player.MaxHP = 1e9
				}
				worlds[i] = w
			}
			starts := make([]chan struct{}, count)
			finished := make(chan struct{}, count)
			stop := make(chan struct{})
			var wg sync.WaitGroup
			for index, w := range worlds {
				starts[index] = make(chan struct{})
				start := starts[index]
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-start:
							w.step(1.0 / 30)
							finished <- struct{}{}
						case <-stop:
							return
						}
					}
				}()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, start := range starts {
					start <- struct{}{}
				}
				for range worlds {
					<-finished
				}
			}
			b.StopTimer()
			close(stop)
			wg.Wait()
		})
	}
}
