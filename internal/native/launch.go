package native

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type LaunchConfig struct {
	ModRoot     string `json:"mod_root"`
	Combination string `json:"combination"`
	Source      string `json:"source"`
}

// Relinked generations carry explicit paths to their verified combination.
// Normal distribution programs keep their ordinary command-line defaults.
func LaunchDefaults() LaunchConfig {
	v := LaunchConfig{ModRoot: "data/mods", Source: "."}
	exe, err := os.Executable()
	if err != nil {
		return v
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "launch.json"))
	if err == nil {
		_ = json.Unmarshal(data, &v)
	}
	return v
}
