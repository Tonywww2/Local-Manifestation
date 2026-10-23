package scene

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

type Admission struct {
	Player   model.Player
	Actions  PlayerActions
	Position *model.Vec
}

// AddGroup prepares the entire party before the scene owner commits any entrant.
func (a *Actor) AddGroup(entries []Admission) error {
	values := make([]Admission, len(entries))
	for i, entry := range entries {
		entry.Player = model.ClonePlayer(entry.Player)
		entry.Actions = entry.Actions.Clone()
		if entry.Position != nil {
			position := *entry.Position
			entry.Position = &position
		}
		values[i] = entry
	}
	return a.call(command{kind: "add_group", admissions: values}).err
}

func (w *world) admitGroup(entries []Admission) error {
	if len(entries) == 0 {
		return protocol.Reject("error.no_entrants")
	}
	if w.state == "fighting" || w.state == "between_waves" && w.room.Join != "between_waves" {
		return protocol.Reject("error.join_window")
	}
	if w.state == "cleared" || w.state == "faulted" {
		return protocol.Reject("error.scene_finished")
	}
	now := w.adventureTime
	for _, entry := range entries {
		now = max(now, entry.Player.Statuses.Time)
	}
	prepared := make(map[string]*fighter, len(entries))
	a := w.arenaGeometry()
	for i, entry := range entries {
		p := entry.Player
		if w.players[p.ID] != nil || prepared[p.ID] != nil {
			return protocol.Reject("error.already_scene")
		}
		var err error
		p, err = w.catalog.AdvancePlayerStatuses(p, now)
		if err == nil && !w.barrierClaimed[p.ID] {
			p, err = content.EnterBattle(w.catalog, p)
		}
		if err != nil {
			return err
		}
		p.Scene, p.Position = w.id, a.Spawn(len(w.players)+i, 12)
		p.PositionEpoch++
		if entry.Position != nil {
			if !a.ContainsDisk(*entry.Position, 12) {
				return protocol.Reject("error.invalid_input")
			}
			p.Position = *entry.Position
		}
		f := entry.Actions.state
		f.player, f.updated, f.contact = p, w.tick, map[int]float64{}
		if p.Life != "alive" {
			f.attack, f.dodgeTime, f.invulnerable = nil, 0, 0
			f.visual = animation.Cancel(f.visual)
		}
		prepared[p.ID] = &f
	}
	w.advanceStatuses(now)
	if w.state == "faulted" {
		return protocol.Reject("error.scene_finished")
	}
	if w.barrierClaimed == nil {
		w.barrierClaimed = map[string]bool{}
	}
	for id, f := range prepared {
		w.players[id], w.barrierClaimed[id] = f, true
	}
	if w.state == "paused" {
		w.state = w.pausedFrom
		if w.state == "" {
			w.state = "fighting"
		}
	}
	return nil
}
