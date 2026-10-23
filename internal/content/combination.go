package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/mods"
	"hsmultiplayer/sdk"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

type InstalledPackage struct {
	mods.Package
	Type       string `json:"type"`
	ContainsGo bool   `json:"contains_go"`
	Path       string `json:"-"`
}

func PackageManifest(files fs.FS) (Manifest, error) {
	var m Manifest
	data, err := fs.ReadFile(files, "manifest.json")
	if err != nil {
		return m, err
	}
	err = Decode(data, &m)
	if err == nil && (m.ID == "" || strings.ContainsAny(m.ID, ":/\\.") || !mods.Satisfies(m.Version, "="+m.Version) || m.SDK != sdk.Version || m.Type != "" && m.Type != "gameplay" && m.Type != "texture") {
		err = fmt.Errorf("invalid package manifest")
	}
	if err == nil && m.Go != nil && (m.Go.Package == "" || m.Go.Shared == "" && m.Go.Server == "" && m.Go.Client == "") {
		err = fmt.Errorf("native entrypoints required")
	}
	return m, err
}
func CodeDigest(files fs.FS) (string, error) {
	var paths []string
	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(path, ".go") || path == "go.mod" || path == "go.sum") {
			paths = append(paths, path)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		data, err := fs.ReadFile(files, path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%d:%s:%d:", len(path), path, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func InstallPackage(root, path, expected string) (InstalledPackage, error) {
	archive, info, err := ReadPackage(path, expected)
	if err != nil {
		return InstalledPackage{}, err
	}
	m, err := PackageManifest(archive)
	if err != nil {
		return InstalledPackage{}, err
	}
	if m.Type == "texture" {
		if info.ContainsGo || m.Go != nil || len(m.Scripts) > 0 || m.Definitions != "" || m.Adventure != "" {
			return InstalledPackage{}, fmt.Errorf("texture packages may only contain assets and JSON metadata")
		}
	}
	if info.ContainsGo && m.Go == nil {
		return InstalledPackage{}, fmt.Errorf("Go source requires declared native entrypoints")
	}
	record := InstalledPackage{Package: mods.Package{ID: m.ID, Version: m.Version, SHA256: info.SHA256, Dependencies: m.Dependencies}, Type: m.Type, ContainsGo: info.ContainsGo}
	dir := filepath.Join(root, info.SHA256)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return record, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return record, err
	}
	if _, _, err = Verify(data, expected); err != nil {
		return record, err
	}
	if err = atomicFile(filepath.Join(dir, "package.hsmod"), data); err != nil {
		return record, err
	}
	data, err = json.MarshalIndent(record, "", "  ")
	if err != nil {
		return record, err
	}
	err = atomicFile(filepath.Join(dir, "record.json"), data)
	record.Path = filepath.Join(dir, "package.hsmod")
	return record, err
}
func atomicFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".publish-*")
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
func Installed(root string) ([]InstalledPackage, error) {
	dirs, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []InstalledPackage
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, dir.Name(), "record.json"))
		if err != nil {
			continue
		}
		var record InstalledPackage
		if err = json.Unmarshal(data, &record); err != nil {
			return nil, err
		}
		if record.SHA256 != dir.Name() {
			return nil, fmt.Errorf("installed package identity mismatch")
		}
		record.Path = filepath.Join(root, dir.Name(), "package.hsmod")
		archive, info, err := ReadPackage(record.Path, record.SHA256)
		if err != nil {
			return nil, err
		}
		m, err := PackageManifest(archive)
		if err != nil || record.ID != m.ID || record.Version != m.Version || record.Type != m.Type || record.ContainsGo != info.ContainsGo || !reflect.DeepEqual(record.Dependencies, m.Dependencies) {
			return nil, fmt.Errorf("installed package metadata mismatch")
		}
		out = append(out, record)
	}
	return out, nil
}
func SaveLock(path string, lock mods.Lock) error {
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return atomicFile(path, data)
}
func LoadLock(path string) (mods.Lock, error) {
	var lock mods.Lock
	data, err := os.ReadFile(path)
	if err != nil {
		return lock, err
	}
	err = json.Unmarshal(data, &lock)
	if err == nil && lock.Version != 1 {
		err = fmt.Errorf("unsupported combination lock")
	}
	return lock, err
}

type lockedSource struct {
	manifest Manifest
	files    fs.FS
	hash     string
}

func sources(root string, lock mods.Lock) ([]lockedSource, error) {
	if lock.Version != 1 || len(lock.Packages) > 128 {
		return nil, fmt.Errorf("invalid combination lock")
	}
	seen := map[string]string{}
	var out []lockedSource
	for _, p := range lock.Packages {
		decoded, err := hex.DecodeString(p.SHA256)
		if err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("invalid locked SHA-256")
		}
		archive, _, err := ReadPackage(filepath.Join(root, p.SHA256, "package.hsmod"), p.SHA256)
		if err != nil {
			return nil, err
		}
		m, err := PackageManifest(archive)
		if err != nil || m.ID != p.ID || m.Version != p.Version {
			return nil, fmt.Errorf("locked package identity mismatch")
		}
		for id, requirement := range m.Dependencies {
			if !mods.Satisfies(seen[id], requirement) {
				return nil, fmt.Errorf("locked dependency missing, incompatible or out of order")
			}
		}
		if seen[m.ID] != "" {
			return nil, fmt.Errorf("duplicate locked module")
		}
		seen[m.ID] = m.Version
		out = append(out, lockedSource{m, archive, p.SHA256})
	}
	return out, nil
}

// Compose produces an immutable virtual directory from exact verified packages.
// Explicit overrides are required for definitions and for resources in gameplay packs.
func Compose(base fs.FS, root string, lock mods.Lock, client bool) (*Catalog, error) {
	return ComposeCached(base, root, lock, client, nil)
}
func ComposeCached(base fs.FS, root string, lock mods.Lock, client bool, previous *Catalog) (*Catalog, error) {
	layers, err := sources(root, lock)
	if err != nil {
		return nil, err
	}
	m, err := PackageManifest(base)
	if err != nil {
		return nil, err
	}
	var d Definitions
	gameplay := false
	for _, layer := range layers {
		if layer.manifest.Type != "texture" {
			gameplay = true
		}
	}
	if !gameplay {
		catalog, err := LoadWithOptions(base, LoadOptions{Client: client, Previous: previous})
		if err != nil {
			return nil, err
		}
		if client {
			for _, layer := range layers {
				catalog, err = OverlayTexture(catalog, layer.files)
				if err != nil {
					return nil, err
				}
			}
		}
		return catalog, nil
	}
	data, err := fs.ReadFile(base, m.Definitions)
	if err != nil {
		return nil, err
	}
	if err = Decode(data, &d); err != nil {
		return nil, err
	}
	adventureData, err := fs.ReadFile(base, m.Adventure)
	if err != nil {
		return nil, err
	}
	var adventure model.AdventureDef
	if err = Decode(adventureData, &adventure); err != nil {
		return nil, err
	}
	files := memoryFS{}
	put := func(path string, b []byte) {
		files[path] = append([]byte(nil), b...)
	}
	copyFiles := func(source fs.FS, prefix string) error {
		return fs.WalkDir(source, ".", func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				return nil
			}
			b, err := fs.ReadFile(source, path)
			if err != nil {
				return err
			}
			put(prefix+path, b)
			return nil
		})
	}
	if err = copyFiles(base, "base/"); err != nil {
		return nil, err
	}
	for i, path := range m.Scripts {
		m.Scripts[i] = "base/" + path
	}
	for id, path := range m.Assets {
		m.Assets[id] = "base/" + path
	}
	options := LoadOptions{Namespaces: []string{m.ID}, Client: client, Previous: previous}
	var textures []lockedSource
	for _, layer := range layers {
		if layer.manifest.Type == "texture" {
			if client {
				textures = append(textures, layer)
			}
			continue
		}
		lm := layer.manifest
		options.Modified = true
		options.Namespaces = append(options.Namespaces, lm.ID)
		prefix := "modules/" + lm.ID + "/"
		if err = copyFiles(layer.files, prefix); err != nil {
			return nil, err
		}
		var patch Definitions
		if lm.Definitions != "" {
			b, err := fs.ReadFile(layer.files, lm.Definitions)
			if err != nil {
				return nil, err
			}
			if err = Decode(b, &patch); err != nil {
				return nil, err
			}
			if err = mergeDefinitions(&d, patch, lm.Overrides); err != nil {
				return nil, err
			}
		}
		if lm.Adventure != "" {
			b, err := fs.ReadFile(layer.files, lm.Adventure)
			if err != nil {
				return nil, err
			}
			var patch model.AdventureDef
			if err = Decode(b, &patch); err != nil {
				return nil, err
			}
			if err = mergeAdventure(&adventure, patch, lm.Overrides); err != nil {
				return nil, err
			}
		}
		for _, path := range lm.Scripts {
			m.Scripts = append(m.Scripts, prefix+path)
		}
		if m.Assets == nil {
			m.Assets = map[string]string{}
		}
		for id, path := range lm.Assets {
			if m.Assets[id] != "" && !contains(lm.Overrides, id) {
				return nil, fmt.Errorf("asset override requires declaration: %s", id)
			}
			m.Assets[id] = prefix + path
		}
		if lm.Go != nil {
			hash, err := CodeDigest(layer.files)
			if err != nil {
				return nil, err
			}
			options.Native = append(options.Native, NativeSpec{lm.ID, lm.Version, hash})
		}
		identity, _ := json.Marshal(mods.Package{ID: lm.ID, Version: lm.Version, SHA256: layer.hash})
		put("identities/"+lm.ID+".json", identity)
	}
	m.Definitions = "definitions.json"
	m.Adventure = "adventure.json"
	b, _ := json.Marshal(m)
	put("manifest.json", b)
	b, _ = json.Marshal(d)
	put(m.Definitions, b)
	b, _ = json.Marshal(adventure)
	put(m.Adventure, b)
	catalog, err := LoadWithOptions(files, options)
	if err != nil {
		return nil, err
	}
	for _, texture := range textures {
		catalog, err = OverlayTexture(catalog, texture.files)
		if err != nil {
			return nil, err
		}
	}
	return catalog, nil
}
func contains(values []string, id string) bool {
	for _, v := range values {
		if v == id {
			return true
		}
	}
	return false
}
func mergeValues[T any](base []T, patch []T, overrides []string) ([]T, error) {
	out := append([]T(nil), base...)
	identity := func(v T) string { return reflect.ValueOf(v).FieldByName("ID").String() }
	for _, v := range patch {
		id := identity(v)
		index := -1
		for i, old := range out {
			if identity(old) == id {
				index = i
				break
			}
		}
		if index < 0 {
			out = append(out, v)
		} else if contains(overrides, id) {
			out[index] = v
		} else {
			return nil, fmt.Errorf("definition override requires declaration: %s", id)
		}
	}
	return out, nil
}
func mergeDefinitions(d *Definitions, p Definitions, overrides []string) error {
	var err error
	d.CompletionRewards, err = mergeValues(d.CompletionRewards, p.CompletionRewards, overrides)
	if err != nil {
		return err
	}
	d.Weapons, err = mergeValues(d.Weapons, p.Weapons, overrides)
	if err != nil {
		return err
	}
	d.Chassis, err = mergeValues(d.Chassis, p.Chassis, overrides)
	if err != nil {
		return err
	}
	d.Enemies, err = mergeValues(d.Enemies, p.Enemies, overrides)
	if err != nil {
		return err
	}
	d.Statuses, err = mergeValues(d.Statuses, p.Statuses, overrides)
	if err != nil {
		return err
	}
	d.Items, err = mergeValues(d.Items, p.Items, overrides)
	if err != nil {
		return err
	}
	d.Affixes, err = mergeValues(d.Affixes, p.Affixes, overrides)
	if err != nil {
		return err
	}
	d.WeaponSkills, err = mergeValues(d.WeaponSkills, p.WeaponSkills, overrides)
	if err != nil {
		return err
	}
	d.Perks, err = mergeValues(d.Perks, p.Perks, overrides)
	if err != nil {
		return err
	}
	if len(p.Run.Floors) > 0 {
		if !contains(overrides, "engine:run") {
			return fmt.Errorf("run override required")
		}
		d.Run = p.Run
	}
	if p.Resource.Maximum > 0 {
		if !contains(overrides, "engine:resource") {
			return fmt.Errorf("resource override required")
		}
		d.Resource = p.Resource
	}
	d.StartingItems = append(d.StartingItems, p.StartingItems...)
	if p.Inventory.Capacity > 0 {
		if !contains(overrides, "engine:inventory") {
			return fmt.Errorf("inventory override required")
		}
		d.Inventory = p.Inventory
	}
	if contains(overrides, "engine:clear_points") {
		d.ClearPoints = p.ClearPoints
	}
	return nil
}
func mergeAdventure(d *model.AdventureDef, p model.AdventureDef, overrides []string) error {
	var err error
	d.Floors, err = mergeValues(d.Floors, p.Floors, overrides)
	if err != nil {
		return err
	}
	d.Maps, err = mergeValues(d.Maps, p.Maps, overrides)
	if err != nil {
		return err
	}
	d.Nodes, err = mergeValues(d.Nodes, p.Nodes, overrides)
	if err != nil {
		return err
	}
	d.Rooms, err = mergeValues(d.Rooms, p.Rooms, overrides)
	return err
}
func OverlayTexture(c *Catalog, files fs.FS) (*Catalog, error) {
	immutable, err := snapshotFiles(files)
	if err != nil {
		return nil, err
	}
	files = immutable
	m, err := PackageManifest(files)
	if err != nil {
		return nil, err
	}
	if m.Type != "texture" || m.Go != nil || len(m.Scripts) > 0 || m.Definitions != "" || m.Adventure != "" {
		return nil, fmt.Errorf("invalid texture package")
	}
	for name := range immutable {
		if strings.HasSuffix(strings.ToLower(name), ".go") || strings.HasSuffix(strings.ToLower(name), ".js") {
			return nil, fmt.Errorf("texture packages cannot contain executable source")
		}
	}
	next := *c
	next.assets = map[string][]byte{}
	for id, data := range c.assets {
		next.assets[id] = data
	}
	for id, path := range m.Assets {
		data, err := fs.ReadFile(files, path)
		if err != nil || len(data) > maxFile {
			return nil, fmt.Errorf("invalid texture asset")
		}
		next.assets[id] = append([]byte(nil), data...)
	}
	return &next, nil
}
