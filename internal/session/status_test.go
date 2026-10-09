package session

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"testing"
)

func TestAdventureTimeRunsInMapAndPreparationAtBothRates(t *testing.T) {
	for _, rate := range []int{30, 60} {
		for _, phase := range []string{"preparation", "exploring", "intermission", "won", "lost", "lobby"} {
			tm := fixture(t)
			tm.config.TickRate = rate
			tm.phase = phase
			for i := 0; i < rate; i++ {
				tm.advanceAdventure()
			}
			expected := status.UnitsPerSecond
			if phase == "won" || phase == "lost" || phase == "lobby" {
				expected = 0
			}
			if tm.adventureTime != expected || tm.players["a"].player.Statuses.Time != expected {
				t.Fatal("adventure clock frequency or phase", rate, phase, tm.adventureTime)
			}
		}
	}
}

func TestRoomHandoffCarriesRemainingTimeAndExplicitLeaveClears(t *testing.T) {
	tm := fixture(t)
	c := testcontent.Load(t, func(d *content.Definitions, _ *model.AdventureDef) {
		d.Statuses = []model.StatusDef{{ID: "base:carry", Name: "Carry", Description: "test", Level: 1, MaxLayers: 1, DurationMillis: 10000}, {ID: "base:leave", Name: "Leave", Description: "test", Level: 1, MaxLayers: 1, DurationMillis: 10000, ClearOnLeave: true}}
	}, "")
	installContent(t, tm, c)
	m := tm.players["a"]
	source := model.SourceRef{Kind: "weapon", Actor: "a", Instance: "item-1", Definition: m.player.Weapon}
	var err error
	m.player, err = content.ApplyStatus(c, m.player, "base:carry", source)
	if err != nil {
		t.Fatal(err)
	}
	m.player, err = content.ApplyStatus(c, m.player, "base:leave", source)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		tm.advanceAdventure()
	}
	if err := tm.enter(m, "passage-a-start"); err != nil {
		t.Fatal(err)
	}
	if len(m.player.Statuses.Instances) != 1 || m.player.Statuses.Instances[0].Definition != "base:carry" || m.player.Statuses.Instances[0].Layers[0].Expires-m.player.Statuses.Time != 9*status.UnitsPerSecond {
		t.Fatal("handoff lost remaining time or leave lifetime")
	}
}
