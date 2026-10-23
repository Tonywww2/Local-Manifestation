package session

import (
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"math"
)

func (t *team) movementTool(m *member, item, node string) (model.ItemDef, error) {
	if m == nil || !m.player.Active || m.player.Life != "alive" || m.player.Scene != "" {
		return model.ItemDef{}, protocol.Reject("error.outside_combat")
	}
	it, ok := inventory.Find(m.player.Inventory, item)
	d, known := t.catalog.Item(it.Definition)
	n, exists := t.nodes[node]
	if !ok || !known || !exists || d.Movement == nil || !inventory.InBag(m.player.Inventory, item) || !expedition.MovementRange(t.nodes[m.player.Node], n, *d.Movement) && !(d.Movement.Team && m.player.Node == node) {
		return model.ItemDef{}, protocol.Reject("error.movement_item")
	}
	return d, nil
}

func (t *team) useMovement(m *member, item, id string) error {
	d, err := t.movementTool(m, item, id)
	if err != nil {
		return err
	}
	if t.nodes[id].Kind == "boss" && !d.Movement.Team {
		return t.selectBoss(m, item)
	}
	if !d.Movement.Team {
		return t.arrive([]*member{m}, id, arrivalMode{owner: m.player.ID, item: item})
	}
	if t.vote != nil {
		return protocol.Reject("error.vote_open")
	}
	if t.shared < 1 {
		return protocol.Reject("error.no_shared_ap")
	}
	v := &protocol.Vote{Node: id, Owner: m.player.ID, Item: item}
	for _, pid := range t.ids() {
		p := t.players[pid].player
		if p.Active && p.Online && p.Life == "alive" {
			v.Eligible = append(v.Eligible, pid)
		}
	}
	v.Required = max(1, int(math.Ceil(float64(len(v.Eligible))*t.config.VoteRatio)))
	t.vote = v
	return t.castVoteEntry(m, true, false)
}

func (t *team) outOfCombat(p model.Player) bool {
	return p.Scene == "" || p.Life == "down" && t.views[p.Scene].State == "paused"
}

func (t *team) executeTeamMovement(v protocol.Vote) error {
	owner := t.players[v.Owner]
	if _, err := t.movementTool(owner, v.Item, v.Node); err != nil {
		return err
	}
	if t.nodes[v.Node].Kind == "boss" {
		return t.startToolBoss(v)
	}
	var members []*member
	for _, id := range t.ids() {
		m := t.players[id]
		if m.player.Active && m.player.Life != "dead" && m.player.Node != v.Node && t.outOfCombat(m.player) {
			members = append(members, m)
		}
	}
	return t.arrive(members, v.Node, arrivalMode{shared: true, owner: v.Owner, item: v.Item})
}

func (t *team) reserveTravelSources(members []*member, allowDown bool) (map[string]*scene.Actor, map[string]model.Player, error) {
	owners := map[string][]string{}
	for _, m := range members {
		if m == nil {
			return nil, nil, protocol.Reject("error.incapacitated")
		}
		if m.player.Scene != "" {
			if !allowDown || m.player.Life != "down" || !t.outOfCombat(m.player) {
				return nil, nil, protocol.Reject("error.already_battle")
			}
			owners[m.player.Scene] = append(owners[m.player.Scene], m.player.ID)
		}
	}
	reserved := map[string]*scene.Actor{}
	players := map[string]model.Player{}
	for _, id := range t.sceneIDs() {
		ids := owners[id]
		if len(ids) == 0 {
			continue
		}
		a := t.actors[id]
		values, err := a.Reserve(ids)
		if err != nil {
			for _, r := range reserved {
				r.Abort()
			}
			return nil, nil, err
		}
		reserved[id] = a
		for _, p := range values {
			players[p.ID] = p
		}
	}
	return reserved, players, nil
}
