package persistence

import (
	"encoding/json"
	"hsmultiplayer/pkg/model"
	"os"
	"path/filepath"
)

type Profiles struct {
	Version int                          `json:"version"`
	Scopes  map[string]model.Progression `json:"scopes"`
}

func LoadProfiles(path string) (Profiles, error) {
	p := Profiles{Version: 1, Scopes: map[string]model.Progression{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(data, &p)
	if p.Scopes == nil {
		p.Scopes = map[string]model.Progression{}
	}
	return p, err
}
func SaveProfiles(path string, p Profiles) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".progress-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
