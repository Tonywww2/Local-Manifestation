package session

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
	"reflect"
	"testing"
	"time"
)

func grantSpecial(t *testing.T, tm *team, id, def string) string {
	t.Helper()
	p, err := tm.catalog.GrantItem(tm.players[id].player, def)
	if err != nil {
		t.Fatal(err)
	}
	tm.players[id].player = p
	for _, it := range p.Inventory.Items {
		if it.Definition == def {
			return it.ID
		}
	}
	t.Fatal("grant missing")
	return ""
}

func TestSpecialConsumablesTransactions(t *testing.T) {
	t.Run("team transfer extracts paused downed players and skips combat", func(t *testing.T) {
		tm := fixture(t)
		var target, source model.Node
		for _, n := range tm.nodes {
			if n.Kind == "combat" && source.ID == "" {
				source = n
			}
			if n.Kind == "passage" {
				target = n
			}
		}
		b := tm.players["b"]
		b.player.Node, b.player.Scene, b.player.Life, b.player.HP = source.ID, source.ID, "down", 0
		a, err := scene.New(tm.catalog, scene.Config{ID: source.ID, Room: source.Room, Floor: 1, Seed: 42, TickRate: 30, ExternalClock: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = a.AddGroup([]scene.Admission{{Player: b.player}}); err != nil {
			a.Stop()
			t.Fatal(err)
		}
		tm.actors[source.ID] = a
		deadline := time.Now().Add(time.Second)
		for a.Snapshot().State != "paused" && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		tm.views[source.ID] = a.Snapshot()
		tm.merge(tm.views[source.ID])
		if tm.views[source.ID].State != "paused" {
			t.Fatal("downed room did not pause")
		}
		p, err := content.Player(tm.catalog, "c", "c", "base:scout")
		if err != nil {
			t.Fatal(err)
		}
		p.Active, p.Scene = true, "occupied-battle"
		tm.players["c"] = &member{player: p, visited: map[string]bool{}}
		item := grantSpecial(t, tm, "a", "base:team-coordinate-ticket")
		beforeAP, beforeShared := tm.players["a"].player.AP, tm.shared
		if err = tm.useMovement(tm.players["a"], item, target.ID); err != nil {
			t.Fatal(err)
		}
		if tm.vote == nil {
			t.Fatal("team vote missing")
		}
		if err = tm.castVoteEntry(tm.players["c"], true, false); err != nil {
			t.Fatal(err)
		}
		if b.player.Node != target.ID || b.player.Life != "down" || b.player.Scene != "" || len(a.Snapshot().Players) != 0 {
			t.Fatal("downed transfer", b.player)
		}
		if tm.players["c"].player.Scene != p.Scene || tm.shared != beforeShared-1 || tm.players["a"].player.AP != beforeAP {
			t.Fatal("team transfer costs/participants")
		}
		if _, ok := inventory.Find(tm.players["a"].player.Inventory, item); ok {
			t.Fatal("team item not consumed")
		}
	})
	t.Run("locked target preserves inventory and AP", func(t *testing.T) {
		tm := fixture(t)
		m := tm.players["a"]
		item := grantSpecial(t, tm, "a", "base:coordinate-pass")
		var target model.Node
		for _, n := range tm.nodes {
			if n.Kind == "combat" {
				target = n
				break
			}
		}
		tm.players["b"].player.Node = target.Edges[0]
		if err := tm.enterGroup([]*member{tm.players["b"]}, target.ID, true); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(4 * time.Second)
		for tm.actors[target.ID].Snapshot().State != "fighting" && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if tm.actors[target.ID].Snapshot().State != "fighting" {
			t.Fatal("battle did not lock")
		}
		before := model.ClonePlayer(m.player)
		if err := tm.useMovement(m, item, target.ID); err == nil || !reflect.DeepEqual(before, m.player) {
			t.Fatal("locked travel consumed resources", err)
		}
		m.player.AP = 1
		for _, n := range tm.nodes {
			if n.Kind == "passage" {
				target = n
				break
			}
		}
		if err := tm.useMovement(m, item, target.ID); err != nil {
			t.Fatal(err)
		}
		it, _ := inventory.Find(m.player.Inventory, item)
		if m.player.AP != 0 || it.Durability != 2 || m.player.Node != target.ID {
			t.Fatal("movement cost", m.player.AP, it)
		}
	})
	t.Run("remote boss commitments consume once on launch", func(t *testing.T) {
		tm := fixture(t)
		tm.shared = 1
		a, b := tm.players["a"], tm.players["b"]
		a.player.Node = tm.nodes[tm.bossNode()].Edges[0]
		item := grantSpecial(t, tm, "b", "base:free-coordinate-ticket")
		before := b.player.AP
		if err := tm.selectBoss(a, ""); err != nil {
			t.Fatal(err)
		}
		if tm.shared != 1 || tm.bossRoute != "" {
			t.Fatal("spent before assembly")
		}
		if err := tm.useMovement(b, item, tm.bossNode()); err != nil {
			t.Fatal(err)
		}
		if tm.shared != 0 || tm.bossRoute != "normal" || b.player.AP != before || a.player.Scene != b.player.Scene {
			t.Fatal("boss launch")
		}
		if _, ok := inventory.Find(b.player.Inventory, item); ok {
			t.Fatal("tool not consumed")
		}
	})
	t.Run("all passives and room lifetime", func(t *testing.T) {
		tm := fixture(t)
		m := tm.players["a"]
		grantSpecial(t, tm, "a", "base:emergency-fuse")
		grantSpecial(t, tm, "a", "base:emergency-fuse")
		r, err := damage.Evaluate(damage.Request{Weapon: model.Weapon{ID: "test:lethal", Type: "physical", Base: 1e6}, FinalMultiplier: 1})
		if err != nil {
			t.Fatal(err)
		}
		settled, p, err := tm.catalog.SettlePlayerDamage(r, m.player)
		if err != nil {
			t.Fatal(err)
		}
		if p.HP != m.player.HP || settled.HealthLoss != 0 || status.ShieldTotal(p.Statuses) != p.MaxHP {
			t.Fatal("lethal immunity/shields", p.Statuses)
		}
		for _, it := range p.Inventory.Items {
			if it.Definition == "base:emergency-fuse" {
				t.Fatal("did not trigger every matching fuse")
			}
		}
		m.player = p
		grantSpecial(t, tm, "a", "base:entry-kit")
		infinite := grantSpecial(t, tm, "a", "base:entry-metronome")
		n, err := tm.catalog.MovementPassives(m.player, 1, 42, true)
		if err != nil {
			t.Fatal(err)
		}
		it, _ := inventory.Find(n.Inventory, infinite)
		before, _ := stats.Evaluate(content.StatInput(m.player))
		after, _ := stats.Evaluate(content.StatInput(n))
		if it.Durability != -1 || after.Values.AttackSpeed <= before.Values.AttackSpeed {
			t.Fatal("arrival effects")
		}
		n, err = tm.catalog.AdvancePlayerStatuses(n, 25*60*60*status.UnitsPerSecond)
		if err != nil {
			t.Fatal(err)
		}
		after, _ = stats.Evaluate(content.StatInput(n))
		if after.Values.AttackSpeed <= before.Values.AttackSpeed {
			t.Fatal("room status expired by clock")
		}
		n, err = content.LeaveStatuses(n)
		after, _ = stats.Evaluate(content.StatInput(n))
		if err != nil || after.Values.AttackSpeed != before.Values.AttackSpeed {
			t.Fatal("room status not removed", err)
		}
		if err = content.ValidatePlayerStatuses(tm.catalog, n); err != nil {
			t.Fatal(err)
		}
	})
}
