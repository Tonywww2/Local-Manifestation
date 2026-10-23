package session

import (
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/status"
	"hsmultiplayer/pkg/timing"
)

func (t *team) advanceAdventure() {
	defer t.performance.Since("server.logic.statuses", perf.Start())
	if t.floor == 0 || t.phase == "lobby" || t.phase == "won" || t.phase == "lost" {
		return
	}
	rate := t.config.TickRate
	if !timing.LogicRate(rate) {
		rate = timing.DefaultLogicRate
	}
	t.adventureTime += status.UnitsPerSecond / int64(rate)
	for _, a := range t.actors {
		a.AdvanceStatuses(t.adventureTime)
	}
	for _, m := range t.players {
		if m.player.Scene != "" {
			continue
		}
		p, err := t.catalog.AdvancePlayerStatuses(m.player, t.adventureTime)
		if err != nil {
			t.phase = "lost"
			continue
		}
		m.player = p
	}
}
