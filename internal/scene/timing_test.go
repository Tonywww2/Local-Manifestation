package scene

import (
	"math"
	"math/rand"
	"testing"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
)

func TestFixedRatesPreserveMovementAndAttackCadence(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		for i := range d.Weapons {
			d.Weapons[i].Cooldown = .2
		}
	}, "")
	for _, rate := range []int{30, 60} {
		vm, err := script.New(c, 1)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := content.Player(c, "p", "p", "base:scout")
		p.Position = model.Vec{X: 1000, Y: 1000}
		room, _ := c.Room("base:skirmish")
		room.Width = 4096
		room.Height = 4096
		e := c.Definitions().Enemies[0]
		w := &world{logicRate: rate, room: room, id: "a", catalog: c, runtime: vm, rng: rand.New(rand.NewSource(1)), state: "fighting", wave: 1, waves: 2,
			players: map[string]*fighter{"p": {player: p, input: model.Input{Move: model.Vec{X: 1}, Aim: model.Vec{X: 3500, Y: 3500}, Attack: true}, contact: map[int]float64{}}},
			enemies: []model.Enemy{{ID: 100, Definition: e.ID, Position: model.Vec{X: 3500, Y: 3500}, HP: 1e6, MaxHP: 1e6, Radius: e.Radius}}}
		for i := 0; i < rate*2; i++ {
			w.players["p"].updated = w.tick
			w.step(1 / float64(rate))
		}
		if math.Abs(w.players["p"].player.Position.X-(1000+p.Speed*2)) > 1e-7 {
			t.Fatal("movement rate changed", rate, w.players["p"].player.Position)
		}
		if w.nextID != 10 {
			t.Fatal("attack period changed", rate, w.nextID)
		}
		vm.Close()
	}
}
