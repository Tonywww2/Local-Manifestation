// Package content builds an immutable, validated content catalog.
package content

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"
	"hsmultiplayer/internal/native"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/sdk"
)

type Manifest struct {
	Type         string            `json:"type,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Overrides    []string          `json:"overrides,omitempty"`
	Assets       map[string]string `json:"assets,omitempty"`
	Go           *GoEntrypoints    `json:"go,omitempty"`
	ID           string            `json:"id"`
	Version      string            `json:"version"`
	SDK          int               `json:"sdk"`
	Definitions  string            `json:"definitions"`
	Adventure    string            `json:"adventure"`
	Scripts      []string          `json:"scripts"`
}
type GoEntrypoints struct {
	Package string `json:"package"`
	Shared  string `json:"shared,omitempty"`
	Server  string `json:"server,omitempty"`
	Client  string `json:"client,omitempty"`
}
type NativeSpec struct{ ID, Version, CodeSHA256 string }
type LoadOptions struct {
	Namespaces []string
	Native     []NativeSpec
	Client     bool
	Modified   bool
	Previous   *Catalog
}
type Definitions struct {
	CompletionRewards []model.CompletionRewardDef `json:"completion_rewards,omitempty"`
	Perks             []model.PerkDef             `json:"perks,omitempty"`
	ClearPoints       int                         `json:"clear_points"`
	Affixes           []model.AffixDef            `json:"affixes,omitempty"`
	Skills            []model.SkillDef            `json:"skills,omitempty"`
	Resource          model.ResourceDef           `json:"resource"`
	Items             []model.ItemDef             `json:"items,omitempty"`
	Inventory         model.Inventory             `json:"inventory"`
	StartingItems     []string                    `json:"starting_items,omitempty"`
	Statuses          []model.StatusDef           `json:"statuses,omitempty"`
	Run               model.RunDef                `json:"run"`
	Weapons           []model.Weapon              `json:"weapons"`
	Chassis           []model.Chassis             `json:"chassis"`
	Enemies           []model.EnemyDef            `json:"enemies"`
}
type Timings struct{ Read, Parse, Validate, Compile, Register, Hash time.Duration }
type Catalog struct {
	assets         map[string][]byte
	namespaces     []string
	nativeRegister func(sdk.Registrar) error
	modified       bool
	perks          map[string]model.PerkDef
	affixes        map[string]model.AffixDef
	skills         map[string]model.SkillDef
	items          map[string]model.ItemDef
	Digest         string
	manifest       Manifest
	definitions    Definitions
	adventure      model.AdventureDef
	weapons        map[string]model.Weapon
	chassis        map[string]model.Chassis
	enemies        map[string]model.EnemyDef
	floors         map[string]model.FloorDef
	maps           map[string]model.MapTemplateDef
	nodes          map[string]model.NodeDef
	rooms          map[string]model.RoomDef
	statuses       map[string]model.StatusDef
	programs       []*goja.Program
	programHashes  []string
	timings        Timings
	containsGo     bool
}

func clone[T any](value T) T {
	b, _ := json.Marshal(value)
	var copy T
	if err := json.Unmarshal(b, &copy); err != nil {
		panic(err)
	}
	return copy
}
func (c *Catalog) ContainsGo() bool                           { return c.containsGo }
func (c *Catalog) Modified() bool                             { return c.modified }
func (c *Catalog) Namespaces() []string                       { return append([]string(nil), c.namespaces...) }
func (c *Catalog) NativeRegistrar() func(sdk.Registrar) error { return c.nativeRegister }
func (c *Catalog) Asset(id string) []byte                     { return append([]byte(nil), c.assets[id]...) }
func (c *Catalog) AssetIDs() []string {
	var ids []string
	for id := range c.assets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (c *Catalog) KnownSource(id string) bool { return c.knownStatusSource(id) }
func (c *Catalog) Status(id string) (model.StatusDef, bool) {
	v, ok := c.statuses[id]
	v.Combat = append([]model.CombatEffect(nil), v.Combat...)
	v.Modifiers = append([]model.StatModifier(nil), v.Modifiers...)
	v.LevelModifiers = append([]model.StatModifier(nil), v.LevelModifiers...)
	v.Periodic = append([]model.PeriodicDef(nil), v.Periodic...)
	v.Controls = append([]string(nil), v.Controls...)
	v.Immunities = append([]string(nil), v.Immunities...)
	v.Conflicts = append([]string(nil), v.Conflicts...)
	return v, ok
}
func (c *Catalog) Namespace() string             { return c.manifest.ID }
func (c *Catalog) Manifest() Manifest            { return clone(c.manifest) }
func (c *Catalog) Definitions() Definitions      { return clone(c.definitions) }
func (c *Catalog) Adventure() model.AdventureDef { return clone(c.adventure) }
func (c *Catalog) Timings() Timings              { return c.timings }
func (c *Catalog) Programs() []*goja.Program     { return append([]*goja.Program(nil), c.programs...) }
func (c *Catalog) ChassisList() []model.Chassis {
	values := append([]model.Chassis(nil), c.definitions.Chassis...)
	for i := range values {
		values[i].Modifiers = append([]model.StatModifier(nil), values[i].Modifiers...)
	}
	return values
}
func (c *Catalog) Chassis(id string) (model.Chassis, bool) {
	v, ok := c.chassis[id]
	v.Modifiers = append([]model.StatModifier(nil), v.Modifiers...)
	return v, ok
}
func (c *Catalog) Run() model.RunDef {
	v := c.definitions.Run
	v.Floors = append([]string(nil), v.Floors...)
	return v
}
func (c *Catalog) Floor(index int) model.FloorDef       { return c.floors[c.definitions.Run.Floors[index-1]] }
func (c *Catalog) Map(id string) model.MapTemplateDef   { return clone(c.maps[id]) }
func (c *Catalog) Node(id string) model.NodeDef         { return clone(c.nodes[id]) }
func (c *Catalog) Room(id string) (model.RoomDef, bool) { v, ok := c.rooms[id]; return clone(v), ok }
func (c *Catalog) Weapon(id string) model.Weapon {
	return model.CloneWeapon(c.weapons[id])
}
func (c *Catalog) Enemy(id string) model.EnemyDef {
	v := c.enemies[id]
	v.Attacks = append([]model.EnemyAttackDef(nil), v.Attacks...)
	if v.Resistance != nil {
		m := map[string]float64{}
		for k, n := range v.Resistance {
			m[k] = n
		}
		v.Resistance = m
	}
	if v.ResistancePerFloor != nil {
		m := map[string]float64{}
		for k, n := range v.ResistancePerFloor {
			m[k] = n
		}
		v.ResistancePerFloor = m
	}
	return v
}

// EnemyAppearance returns a scalar so rendering never copies simulation data.
func (c *Catalog) EnemyAppearance(id string) string { return c.enemies[id].Appearance }
func (c *Catalog) NodeDefinitions() []model.NodeDef { return clone(c.adventure.Nodes) }
func (c *Catalog) BehaviorIDs() []string {
	var ids []string
	for _, e := range c.definitions.Enemies {
		ids = append(ids, e.Behavior)
	}
	return ids
}

// Decode allows underscore comments at every depth and rejects unknown fields.
func Decode(data []byte, out any) error {
	var raw any
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&raw); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	var clean func(any)
	clean = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, y := range x {
				if strings.HasPrefix(k, "_") {
					delete(x, k)
				} else {
					clean(y)
				}
			}
		case []any:
			for _, y := range x {
				clean(y)
			}
		}
	}
	clean(raw)
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func Load(files fs.FS) (*Catalog, error) {
	return LoadWithOptions(files, LoadOptions{})
}
func LoadWithOptions(files fs.FS, options LoadOptions) (*Catalog, error) {
	readStart := time.Now()
	immutable, err := snapshotFiles(files)
	if err != nil {
		return nil, err
	}
	files = immutable
	c := &Catalog{weapons: map[string]model.Weapon{}, chassis: map[string]model.Chassis{}, enemies: map[string]model.EnemyDef{}, floors: map[string]model.FloorDef{}, maps: map[string]model.MapTemplateDef{}, nodes: map[string]model.NodeDef{}, rooms: map[string]model.RoomDef{}}
	c.timings.Read = time.Since(readStart)
	start := time.Now()
	var names []string
	if err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			names = append(names, path)
			if strings.HasSuffix(strings.ToLower(path), ".go") {
				c.containsGo = true
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(names)
	cache := map[string][]byte{}
	available := map[string]bool{}
	for _, name := range names {
		available[name] = true
	}
	readBytes := func(name string) ([]byte, error) {
		if !fs.ValidPath(name) || !available[name] {
			return nil, fmt.Errorf("invalid or missing content path %q", name)
		}
		if data, ok := cache[name]; ok {
			return data, nil
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, err
		}
		// Use exactly these bytes for validation, compilation and identity.
		data = append([]byte(nil), data...)
		cache[name] = data
		return data, nil
	}
	read := func(name string, out any) error {
		if !fs.ValidPath(name) {
			return fmt.Errorf("invalid content path %q", name)
		}
		b, err := readBytes(name)
		if err == nil {
			err = Decode(b, out)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}
	if err := read("manifest.json", &c.manifest); err != nil {
		return nil, err
	}
	if c.manifest.ID == "" || strings.ContainsAny(c.manifest.ID, ":/\\") || c.manifest.Version == "" || c.manifest.SDK != sdk.Version {
		return nil, fmt.Errorf("manifest.json: invalid manifest or SDK version (expected %d)", sdk.Version)
	}
	if c.manifest.Type != "" && c.manifest.Type != "gameplay" {
		return nil, fmt.Errorf("use texture overlay loader for texture packages")
	}
	c.namespaces = append([]string(nil), options.Namespaces...)
	if len(c.namespaces) == 0 {
		c.namespaces = []string{c.manifest.ID}
	}
	c.modified = options.Modified
	var factories []native.Factory
	for _, spec := range options.Native {
		f, err := native.Lookup(spec.ID, spec.Version, spec.CodeSHA256, options.Client)
		if err != nil {
			return nil, err
		}
		factories = append(factories, f)
		c.containsGo = true
	}
	if c.manifest.Go != nil && len(options.Native) == 0 {
		hash, err := CodeDigest(files)
		if err != nil {
			return nil, err
		}
		f, err := native.Lookup(c.manifest.ID, c.manifest.Version, hash, options.Client)
		if err != nil {
			return nil, err
		}
		factories = append(factories, f)
	}
	if len(factories) > 0 {
		c.nativeRegister = func(r sdk.Registrar) error {
			for _, f := range factories {
				if err := f(r); err != nil {
					return err
				}
			}
			return nil
		}
	}
	c.assets = map[string][]byte{}
	for id, path := range c.manifest.Assets {
		data, err := readBytes(path)
		if err != nil {
			return nil, err
		}
		if len(data) > maxFile {
			return nil, fmt.Errorf("asset too large")
		}
		c.assets[id] = data
	}
	if err := read(c.manifest.Definitions, &c.definitions); err != nil {
		return nil, err
	}
	if err := read(c.manifest.Adventure, &c.adventure); err != nil {
		return nil, err
	}
	c.timings.Parse = time.Since(start)
	start = time.Now()
	if err := c.validate(); err != nil {
		return nil, err
	}
	c.timings.Validate = time.Since(start)
	start = time.Now()
	for _, name := range c.manifest.Scripts {
		if !fs.ValidPath(name) {
			return nil, fmt.Errorf("invalid script path %s", name)
		}
		b, err := readBytes(name)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		hash := hex.EncodeToString(sum[:])
		var p *goja.Program
		if previous := options.Previous; previous != nil {
			for i, old := range previous.manifest.Scripts {
				if old == name && i < len(previous.programHashes) && previous.programHashes[i] == hash {
					p = previous.programs[i]
					break
				}
			}
		}
		if p == nil {
			p, err = goja.Compile(name, string(b), true)
		}
		if err != nil {
			return nil, err
		}
		c.programs = append(c.programs, p)
		c.programHashes = append(c.programHashes, hash)
	}
	c.timings.Compile = time.Since(start)
	start = time.Now()
	vm, err := script.New(c, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: content registration: %w", c.manifest.Adventure, err)
	}
	vm.Close()
	c.timings.Register = time.Since(start)
	start = time.Now()
	h := sha256.New()
	for _, name := range names {
		// Assets are hashed once; keep only parsed/compiled sources in the cache.
		b, cached := cache[name]
		var err error
		if !cached {
			b, err = fs.ReadFile(files, name)
		}
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(h, "%d:%s:%d:", len(name), name, len(b))
		h.Write(b)
	}
	c.Digest = hex.EncodeToString(h.Sum(nil))
	c.timings.Hash = time.Since(start)
	return c, nil
}
func Player(c *Catalog, id, name, chassis string) (model.Player, error) {
	v, ok := c.chassis[chassis]
	if !ok {
		return model.Player{}, fmt.Errorf("unknown chassis")
	}
	p := model.Player{ID: id, Name: name, Chassis: chassis, Online: true, BaseStats: model.StatValues{AttackSpeed: 1, Attributes: v.Attributes, MaxHP: v.HP, Armor: v.Armor, Speed: v.Speed}, Weapon: v.Weapon, Level: 1, Life: "alive", Dodges: 2}
	for _, m := range v.Modifiers {
		p.StatContributions = append(p.StatContributions, model.StatContribution{StatModifier: m, Source: model.SourceRef{Kind: "chassis", Actor: id, Definition: chassis}})
	}
	p.Inventory = model.CloneInventory(c.definitions.Inventory)
	p.Skills = v.Skills
	p.Energy = c.definitions.Resource.Maximum
	p.MaxEnergy = p.Energy
	p.EnergyRegen = c.definitions.Resource.Regeneration
	return c.InitializeInventory(p)
}
func MaxHP(p model.Player) float64 {
	return math.Round(math.Max(1, p.BaseHP*life.PenaltyFactor(p.Downs))*100) / 100
}
