package content_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/sdk"
)

func edited(t testing.TB, change func(*model.AdventureDef)) fstest.MapFS {
	t.Helper()
	files := testcontent.Files(t)
	var a model.AdventureDef
	if err := content.Decode(files["adventure.json"].Data, &a); err != nil {
		t.Fatal(err)
	}
	change(&a)
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	files["adventure.json"].Data = data
	return files
}
func TestInvalidAdventureAndRegistration(t *testing.T) {
	cases := []struct {
		name, diagnostic string
		change           func(*model.AdventureDef)
	}{
		{"version", "format", func(a *model.AdventureDef) { a.Format = 1 }},
		{"duplicate", "base:skirmish", func(a *model.AdventureDef) { a.Rooms = append(a.Rooms, a.Rooms[0]) }},
		{"enemy", "waves[0] enemy reference", func(a *model.AdventureDef) { a.Rooms[0].Waves[0].Enemies[0].Enemy = "base:missing" }},
		{"room", "room reference", func(a *model.AdventureDef) { a.Nodes[1].Room = "base:missing" }},
		{"edge", "edge", func(a *model.AdventureDef) { a.Maps[0].Nodes[0].Edges = append(a.Maps[0].Nodes[0].Edges, "missing") }},
		{"distant-door", "nonadjacent", func(a *model.AdventureDef) { a.Maps[0].Nodes[1].Q += 20 }},
		{"terrain-room-overlap", "terrain", func(a *model.AdventureDef) {
			a.Maps[0].Terrain = []model.Terrain{{Q: a.Maps[0].Nodes[0].Q, R: a.Maps[0].Nodes[0].R, Name: "Cliff"}}
		}},
		{"paid-passage", "passage", func(a *model.AdventureDef) {
			a.Nodes[len(a.Nodes)-1].Payment = "personal"
			a.Nodes[len(a.Nodes)-1].Cost = 1
		}},
		{"waves", "room parameters", func(a *model.AdventureDef) { a.Rooms[0].Waves = nil }},
		{"overflow", "enemy count", func(a *model.AdventureDef) { a.Rooms[0].Waves[0].Enemies[0].Count.Base = int(^uint(0) >> 1) }},
		{"handler", "unregistered handler", func(a *model.AdventureDef) { a.Nodes[0].Handler = "base:missing" }},
		{"parameter", "unknown parameter", func(a *model.AdventureDef) { a.Nodes[0].Parameters = map[string]any{"misspelled": 1} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := content.Load(edited(t, tc.change))
			if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatal(err)
			}
		})
	}
	files := testcontent.Files(t)
	files["adventure.json"].Data = []byte(`{"format":2,"roms":[]}`)
	if _, err := content.Load(files); err == nil || !strings.Contains(err.Error(), "adventure.json") {
		t.Fatal(err)
	}
}
func TestCatalogQueriesAreDetached(t *testing.T) {
	c := testcontent.Load(t, func(d *content.Definitions, _ *model.AdventureDef) {
		d.Weapons[0].Modifiers = []model.Modifier{{Group: "test", Source: "test", Percent: .2}}
	}, "")
	d := c.Definitions()
	d.Weapons[0].Modifiers[0].Percent = 999
	d.Run.Floors[0] = "missing"
	m := c.Map(c.Floor(1).Map)
	m.Nodes[0].Edges[0] = "missing"
	r, _ := c.Room("base:skirmish")
	r.Waves[0].Enemies[0].Enemy = "missing"
	n := c.Node("base:forge")
	n.Parameters["cost_per_level"] = 999
	w := c.Weapon(d.Weapons[0].ID)
	w.Modifiers[0].Percent = 999
	if c.Run().Floors[0] == "missing" || c.Map(c.Floor(1).Map).Nodes[0].Edges[0] == "missing" || c.Node("base:forge").Parameters["cost_per_level"] == 999 || c.Weapon(w.ID).Modifiers[0].Percent == 999 {
		t.Fatal("mutable catalog escaped")
	}
	actual, _ := c.Room("base:skirmish")
	if actual.Waves[0].Enemies[0].Enemy == "missing" {
		t.Fatal("mutable wave escaped")
	}
}
func TestCommandBatchIsAtomicAndScoped(t *testing.T) {
	p := model.Player{ID: "p", Coins: 10, HP: 10, MaxHP: 20, Life: "alive"}
	original := map[string]model.Player{"p": p}
	for _, cmds := range [][]sdk.Command{
		{{Kind: "grant_coins", Target: "p", Amount: 2}, {Kind: "spend_coins", Target: "p", Amount: 50}},
		{{Kind: "grant_coins", Target: "p", Amount: 2}, {Kind: "heal", Target: "other", Amount: 1}},
		{{Kind: "grant_coins", Target: "p", Amount: -1}},
		{{Kind: "grant_xp", Target: "p", Amount: .5}},
	} {
		if next, err := content.ApplyCommands(nil, original, cmds, content.NodeCommands); err == nil || next != nil || !reflect.DeepEqual(original["p"], p) {
			t.Fatal("partial batch committed", next, err)
		}
	}
	next, err := content.ApplyCommands(nil, original, []sdk.Command{{Kind: "spend_coins", Target: "p", Amount: 10}, {Kind: "upgrade_weapon", Target: "p", Amount: 1}, {Kind: "weapon_bonus", Target: "p", Amount: 3}}, content.NodeCommands)
	if err != nil || next["p"].Coins != 0 || next["p"].Upgrade != 1 || next["p"].WeaponBonus != 3 || !reflect.DeepEqual(original["p"], p) {
		t.Fatal(next, err)
	}
	if _, err = content.ApplyCommands(nil, original, []sdk.Command{{Kind: "upgrade_weapon", Target: "p", Amount: 1}}, content.EventCommands); err == nil {
		t.Fatal("event exceeded permission")
	}
}
func TestStorePublicationAndAdventureLease(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	s := content.NewStore(c)
	pinned, release := s.Acquire()
	if pinned != c {
		t.Fatal("wrong generation")
	}
	if err := s.Reload(base.Files); err == nil {
		t.Fatal("reloaded an adventure")
	}
	release()
	release()
	bad := testcontent.Files(t)
	bad["manifest.json"].Data = []byte(`{}`)
	if err := s.Reload(bad); err == nil || s.Current() != c {
		t.Fatal("published failed reload")
	}
	native := testcontent.Files(t)
	native["native.go"] = &fstest.MapFile{Data: []byte("package mod")}
	if err := s.Reload(native); err == nil || s.Current() != c {
		t.Fatal("hot reloaded Go content")
	}
	changed := edited(t, func(a *model.AdventureDef) { a.Rooms[0].Width = 2500 })
	if err := s.Reload(changed); err != nil || s.Current() == c || s.Current().Digest == c.Digest {
		t.Fatal(err)
	}
	old, _ := c.Room("base:skirmish")
	if old.Width != 2000 {
		t.Fatal("rewrote pinned generation")
	}
}
func BenchmarkCatalogLoad(b *testing.B) {
	for _, size := range []int{3, 100} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			files := edited(b, func(a *model.AdventureDef) {
				for len(a.Rooms) < size {
					room := a.Rooms[0]
					room.ID = fmt.Sprintf("base:benchmark-room-%d", len(a.Rooms))
					a.Rooms = append(a.Rooms, room)
				}
			})
			b.ReportAllocs()
			var total content.Timings
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				catalog, err := content.Load(files)
				if err != nil {
					b.Fatal(err)
				}
				v := catalog.Timings()
				total.Parse += v.Parse
				total.Validate += v.Validate
				total.Compile += v.Compile
				total.Register += v.Register
				total.Hash += v.Hash
			}
			b.StopTimer()
			b.ReportMetric(float64(total.Parse.Nanoseconds())/float64(b.N), "parse-ns/op")
			b.ReportMetric(float64(total.Validate.Nanoseconds())/float64(b.N), "validate-ns/op")
			b.ReportMetric(float64(total.Compile.Nanoseconds())/float64(b.N), "compile-ns/op")
			b.ReportMetric(float64(total.Register.Nanoseconds())/float64(b.N), "register-ns/op")
			b.ReportMetric(float64(total.Hash.Nanoseconds())/float64(b.N), "hash-ns/op")

		})
	}
}
