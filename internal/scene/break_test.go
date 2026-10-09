package scene

import (
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"math"
	"testing"
)

func TestCountdownKeepsMovementAttacksAndDodgeRechargeAtBothRates(t *testing.T) {
	for _, rate := range []int{30, 60} {
		for _, state := range []string{"waiting", "between_waves"} {
			c := testcontent.Load(t, nil, "")
			w := templateWorld(t, c, "base:skirmish")
			w.logicRate, w.state = rate, state
			w.room.InitialDelay, w.breakTime = 10, 10
			if state == "waiting" {
				w.breakTime = 0
			}
			w.wave = 1
			p := w.players["a"]
			start := p.player.Position
			p.player.Dodges = 1
			for i := 0; i < rate*2; i++ {
				p.updated = w.tick
				p.input = model.Input{Move: model.Vec{X: 1}, Attack: true, Aim: start.Add(model.Vec{X: 1000})}
				w.step(1 / float64(rate))
			}
			if math.Abs(p.player.Position.X-start.X-p.player.Speed*2) > 1e-7 || w.state != state || w.nextID == 0 || p.player.Dodges != 2 {
				t.Fatal("countdown paused player", rate, state, p.player.Position, w.nextID, p.player.Dodges)
			}
		}
	}
}

func TestCountdownPausesOnlyWhenEveryoneIsIncapacitated(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	w := templateWorld(t, c, "base:skirmish")
	w.state, w.wave, w.breakTime = "between_waves", 1, 3
	for _, p := range w.players {
		p.player.Life = "down"
	}
	w.step(1.0 / 30)
	if w.state != "paused" || w.pausedFrom != "between_waves" || w.breakTime != 3 {
		t.Fatal("all-down pause lost countdown phase")
	}
}
