package client

import (
	"encoding/json"
	"fmt"
	"hsmultiplayer/internal/persistence"
)

func (g *Game) receiveCheckpoint(data json.RawMessage) {
	var checkpoint persistence.Checkpoint
	err := json.Unmarshal(data, &checkpoint)
	if err == nil && (checkpoint.Version != persistence.Version || checkpoint.Digest != g.catalog.Digest) {
		err = fmt.Errorf("incompatible exported checkpoint")
	}
	if err == nil {
		err = persistence.Save(g.cfg.Profile+".checkpoint.json", checkpoint)
	}
	if err != nil {
		g.notifyError(err)
	} else {
		g.notify("notice.checkpoint_export", "path", g.cfg.Profile+".checkpoint.json")
	}
}
