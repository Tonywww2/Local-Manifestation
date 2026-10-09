package persistence

import (
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"path/filepath"
	"testing"
)

func TestCheckpointReplaceAndMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	c := Checkpoint{Version: Version, Digest: "hash", State: protocol.State{Floor: 1, Players: []model.Player{{ID: "a"}}}}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	c.State.Floor = 2
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, "hash")
	if err != nil || got.State.Floor != 2 {
		t.Fatal(got, err)
	}
	if _, err = Load(path, "other"); err == nil {
		t.Fatal("accepted incompatible content")
	}
}
