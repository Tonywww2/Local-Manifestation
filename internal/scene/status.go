package scene

import (
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/status"
)

// Commands and combat steps are serialized by the actor. Expiry is applied
// before the next hit at the accepted adventure timestamp, even while paused.
func (w *world) advanceStatuses(now int64) {
	defer w.performance.Since("scene.logic.statuses", perf.Start())
	if now < w.adventureTime {
		return
	}
	w.adventureTime = now
	if err := w.advanceEnemyStatuses(now); err != nil {
		w.state, w.fault = "faulted", err.Error()
		return
	}
	for _, p := range w.players {
		set, pulses, err := status.Pulses(p.player.Statuses, now)
		if err != nil {
			w.state, w.fault = "faulted", err.Error()
			return
		}
		p.player.Statuses = set
		current, changed, err := status.Advance(p.player.Statuses, now)
		if err != nil {
			w.state, w.fault = "faulted", err.Error()
			return
		}
		p.player.Statuses = current
		if changed {
			p.player, err = content.RefreshPlayer(p.player)
			if err != nil {
				w.state, w.fault = "faulted", err.Error()
				return
			}
		}
		for _, pulse := range pulses {
			if pulse.Kind == "heal" {
				if p.player.Life == "alive" {
					p.player.HP = damage.Round(min(p.player.MaxHP, p.player.HP+pulse.Amount*float64(pulse.Count)))
				}
				continue
			}
			for i := int64(0); i < pulse.Count && p.player.Life == "alive"; i++ {
				_, err = w.resolveHit(damage.Request{Weapon: model.Weapon{ID: pulse.Definition, Type: pulse.Type, Base: pulse.Amount}, FinalMultiplier: 1}, model.Event{Kind: "periodic", Source: pulse.Source.Actor, SourceDefinition: pulse.Definition, Target: p.player.ID, HitID: fmt.Sprintf("pulse:%s:%s:%d:%d", p.player.ID, pulse.Definition, now, i), Tick: w.tick}, nil)
				if err != nil {
					w.state, w.fault = "faulted", err.Error()
					return
				}
			}
		}
		next, err := w.catalog.AdvancePlayerStatuses(p.player, now)
		if err != nil {
			w.state, w.fault = "faulted", err.Error()
			return
		}
		p.player = next
	}
	w.adventureTime = now
}
