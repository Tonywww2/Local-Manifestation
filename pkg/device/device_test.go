package device

import (
	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestEntranceSpecializationKeepsTypeAfterOpening(t *testing.T) {
	a := geometry.NewArena(2000, 1250)
	from := model.Node{ID: "from", Edges: []string{"to"}}
	for _, test := range []struct{ kind, room, want string }{{"boss", "boss-room", "boss"}, {"combat", "battle-room", "battle"}, {"event", "", "event"}, {"passage", "", "passage"}} {
		to := model.Node{ID: "to", Q: 1, Kind: test.kind, Room: test.room, Payment: "shared", Cost: 1, Name: "Door"}
		e, ok := RoomEntrance(from, to, a)
		if !ok || e.Kind != "entrance" || e.RoomKind != test.want || e.Action != "vote" || !e.Near(a.DoorPoint(0, 24)) {
			t.Fatal(e)
		}
		to.Open = true
		to.Completed = true
		e, _ = RoomEntrance(from, to, a)
		if e.RoomKind != test.want {
			t.Fatal("completion changed entrance type", e)
		}
		if e.Near(a.Center) {
			t.Fatal("remote interaction")
		}
	}
}
func TestWorldShopAndExitUseDetachedCommonDevice(t *testing.T) {
	a := geometry.NewArena(2000, 1250)
	shop := model.Node{ID: "shop", Kind: "shop", Open: true, NameKey: "shop.name"}
	values := World(Context{Node: shop, Arena: a, Phase: "exploring", SharedAP: 3})
	d, ok := Find(values, "shop:shop")
	if !ok || d.Action != "shop" || !d.Near(a.Center) {
		t.Fatal(values)
	}
	values[0].Position = model.Vec{}
	if next := World(Context{Node: shop, Arena: a, Phase: "exploring", SharedAP: 3}); next[0].Position != a.Center {
		t.Fatal("caller mutated source")
	}
	boss := model.Node{ID: "boss", Kind: "boss"}
	d, ok = Find(World(Context{Node: boss, Arena: a, Phase: "intermission"}), "next")
	if !ok || !d.Near(a.ExitDevice()) || d.Target != "next" {
		t.Fatal(d)
	}
}
