package scene

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"testing"
)

func TestPausedRoomStillExpiresAdventureStatusesWithoutMoving(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, _ *model.AdventureDef) {
		d.Statuses = []model.StatusDef{{ID: "base:haste", Name: "Haste", Description: "test", Level: 1, MaxLayers: 1, DurationMillis: 1000, Modifiers: []model.StatModifier{{ID: "base:speed", Stat: "speed", Operation: "add", Value: 20}}}}
	}, "")
	w := templateWorld(t, c, "base:skirmish")
	p := w.players["a"]
	var err error
	p.player, err = content.ApplyStatus(c, p.player, "base:haste", model.SourceRef{Kind: "weapon", Actor: "a", Instance: "weapon-1", Definition: p.player.Weapon})
	if err != nil {
		t.Fatal(err)
	}
	w.state = "paused"
	position, clock := p.player.Position, w.clock
	w.advanceStatuses(status.UnitsPerSecond)
	if len(p.player.Statuses.Instances) != 0 || p.player.Speed != p.player.BaseStats.Speed || p.player.Position != position || w.clock != clock || w.state != "paused" {
		t.Fatal("paused combat blocked expiry or resumed battle")
	}
	s := w.snapshot()
	s.Players[0].Statuses.Time = 0
	if w.players[s.Players[0].ID].player.Statuses.Time == 0 {
		t.Fatal("status snapshot alias")
	}
}
