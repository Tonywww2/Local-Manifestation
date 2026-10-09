package session

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/device"
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"math"
	"slices"
)

func (t *team) nearDevice(m *member, target string) bool {
	p := m.player
	if !p.Active || !p.Online || p.Life != "alive" {
		return false
	}
	if target != "next" && target != "enhanced" && target != "exhaust" {
		return t.atDoor(m, target)
	}
	if target == "next" && p.Scene != "" {
		return false
	}
	a := t.spaceArena(p.Node)
	if s, ok := t.views[p.Scene]; ok {
		a = geometry.NewArena(s.Width, s.Height)
	}
	emergency := ""
	if target == "enhanced" || target == "exhaust" {
		emergency = target
	}
	for _, d := range device.World(device.Context{Node: t.nodes[p.Node], Arena: a, Phase: t.phase, SharedAP: t.shared, Emergency: emergency}) {
		if d.Target == target {
			return d.Near(p.Position)
		}
	}
	return false
}

func (t *team) atShop(m *member) bool {
	if m.player.Scene != "" {
		return false
	}
	n := t.nodes[m.player.Node]
	values := device.World(device.Context{Node: n, Arena: t.spaceArena(n.ID), Phase: t.phase, SharedAP: t.shared})
	d, ok := device.Find(values, "shop:"+n.ID)
	return ok && d.Near(m.player.Position)
}

func (t *team) advanceLayer() error {
	// Stage a layer transition so a checkpoint write failure does not consume
	// the previous layer, reset AP or strand the team in a new map.
	next := *t
	next.players = make(map[string]*member, len(t.players))
	for id, m := range t.players {
		copy := *m
		copy.player = model.ClonePlayer(m.player)
		var err error
		copy.player, err = content.LeaveStatuses(copy.player)
		if err != nil {
			return err
		}
		next.players[id] = &copy
	}
	if err := next.initialize(t.floor + 1); err != nil {
		return err
	}
	for _, m := range next.players {
		m.player.Active = m.player.Online
	}
	if err := next.save(); err != nil {
		return err
	}
	next.phase = "exploring"
	*t = next
	return nil
}
func (t *team) deviceReady(m *member, target string) error {
	if !t.nearDevice(m, target) {
		return protocol.Reject("error.door_distance")
	}
	if t.nodes[target].ID != "" {
		return t.sharedDoorReady(target)
	}
	return nil
}
func (t *team) useDevice(m *member, target string) error {
	if target == "" {
		target = "next"
	}
	if err := t.deviceReady(m, target); err != nil {
		return err
	}
	if t.vote != nil {
		if t.vote.Node != target {
			return protocol.Reject("error.vote_open")
		}
		if slices.Contains(t.vote.Yes, m.player.ID) {
			t.vote.Yes = slices.DeleteFunc(t.vote.Yes, func(id string) bool { return id == m.player.ID })
			if len(t.vote.Yes) == 0 {
				t.vote = nil
			}
			return nil
		}
		return t.castVote(m, true)
	}
	if target != "next" {
		return t.propose(m, target)
	}
	v := &protocol.Vote{Node: target}
	for _, id := range t.ids() {
		p := t.players[id].player
		if p.Active && p.Online && p.Life == "alive" {
			v.Eligible = append(v.Eligible, id)
		}
	}
	v.Required = int(math.Ceil(float64(len(v.Eligible)) * t.config.VoteRatio))
	if v.Required == 0 {
		return protocol.Reject("error.no_voters")
	}
	t.vote = v
	return t.castVote(m, true)
}
func (t *team) withdrawDepartedVotes() {
	if t.vote == nil {
		return
	}
	t.vote.Yes = slices.DeleteFunc(t.vote.Yes, func(id string) bool { return t.players[id] == nil || !t.nearDevice(t.players[id], t.vote.Node) })
	if len(t.vote.Yes) == 0 {
		t.vote = nil
	}
}
