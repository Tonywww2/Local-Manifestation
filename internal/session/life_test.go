package session

import (
	"hsmultiplayer/internal/persistence"
	"hsmultiplayer/pkg/model"
	"reflect"
	"testing"
)

func TestRescueStagesAllTargetsBeforeCommit(t *testing.T) {
	tm := fixture(t)
	a, b := tm.players["a"], tm.players["b"]
	b.player.Node = a.player.Node
	b.player.Life = "down"
	b.player.Knockdowns = 1
	b.player.HP = 0
	bad := model.ClonePlayer(b.player)
	bad.ID = "bad"
	bad.Knockdowns = 0
	tm.players[bad.ID] = &member{player: bad}
	before := model.ClonePlayer(b.player)
	if err := tm.rescue(a); err == nil || !reflect.DeepEqual(before, b.player) {
		t.Fatal("partial rescue after invalid target")
	}
	delete(tm.players, bad.ID)
	if err := tm.rescue(a); err != nil || b.player.Knockdowns != 1 || b.player.Downs != 1 || b.player.HP != 90 {
		t.Fatal(b.player, err)
	}
}
func TestCheckpointRejectsForgedHistoryAndPreservesValidRecord(t *testing.T) {
	tm := fixture(t)
	saved := persistence.Checkpoint{Version: persistence.Version, Digest: tm.catalog.Digest, State: tm.snapshot(), Identities: []persistence.Identity{{Player: "a", Token: "a"}, {Player: "b", Token: "b"}}}
	saved.State.Players[0].Knockdowns = 3
	if err := validateRestore(tm.catalog, &saved); err != nil {
		t.Fatal(err)
	}
	saved.State.Players[0].Knockdowns = 4
	if err := validateRestore(tm.catalog, &saved); err == nil {
		t.Fatal("fourth-event living checkpoint accepted")
	}
	saved.State.Players[0].Knockdowns = 0
	saved.State.Players[0].Downs = 1
	if err := validateRestore(tm.catalog, &saved); err == nil {
		t.Fatal("penalty without history accepted")
	}
}
