package persistence

import (
	"encoding/json"
	"fmt"
	"hsmultiplayer/pkg/protocol"
	"os"
	"path/filepath"
)

const Version = 7

type Visit struct {
	Player string   `json:"player"`
	Nodes  []string `json:"nodes"`
}

type Identity struct {
	Player string `json:"player"`
	Token  string `json:"token"`
}
type Checkpoint struct {
	Visits     []Visit        `json:"visits,omitempty"`
	Version    int            `json:"version"`
	Digest     string         `json:"digest"`
	Seed       int64          `json:"seed"`
	State      protocol.State `json:"state"`
	Identities []Identity     `json:"identities"`
}

func Save(path string, c Checkpoint) error {
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".checkpoint-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func Load(path, digest string) (Checkpoint, error) {
	var c Checkpoint
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Version != Version || c.Digest != digest || c.State.Floor < 1 || len(c.State.Players) == 0 {
		return c, fmt.Errorf("incompatible checkpoint")
	}
	return c, nil
}
