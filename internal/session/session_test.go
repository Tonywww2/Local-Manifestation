package session

import (
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/persistence"
	"hsmultiplayer/internal/scene"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func fixture(t *testing.T) *team {
	t.Helper()
	c, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	tm := &team{config: Config{MaxPlayers: 8, Seed: 1, VoteRatio: .7}, catalog: c, players: map[string]*member{}, actors: map[string]*scene.Actor{}, views: map[string]protocol.Scene{}, nodes: map[string]model.Node{}, phase: "lobby"}
	for _, id := range []string{"a", "b"} {
		p, err := content.Player(c, id, id, "base:scout")
		if err != nil {
			t.Fatal(err)
		}
		p.Active = true
		tm.players[id] = &member{player: p, token: id, out: make(chan protocol.Message, 32)}
	}
	tm.runtime, err = script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tm.runtime.Close)
	tm.initialize(1)
	tm.phase = "exploring"
	t.Cleanup(tm.stopScenes)
	return tm
}
func TestSharedZeroKeepsPersonalExploration(t *testing.T) {
	tm := fixture(t)
	tm.shared = 0
	if err := tm.enter(tm.players["a"], "a"); err != nil {
		t.Fatal(err)
	}
	if tm.players["a"].player.AP != tm.initialX-1 || tm.bossRoute != "" {
		t.Fatal("exhaustion forced routing")
	}
	if err := tm.enter(tm.players["b"], "b"); err != nil {
		t.Fatal(err)
	}
	if len(tm.actors) != 2 {
		t.Fatal("cannot split battles")
	}
}
func TestLastSharedPointNormalBossCannotBecomeEnhanced(t *testing.T) {
	tm := fixture(t)
	tm.shared = 1
	for _, m := range tm.players {
		m.player.Node = "c"
	}
	if err := tm.propose(tm.players["a"], "boss"); err != nil {
		t.Fatal(err)
	}
	if tm.shared != 1 {
		t.Fatal("spent before vote")
	}
	if err := tm.castVote(tm.players["b"], true); err != nil {
		t.Fatal(err)
	}
	if tm.shared != 0 || tm.bossRoute != "normal" || tm.cursed {
		t.Fatal(tm.shared, tm.bossRoute, tm.cursed)
	}
	if err := tm.propose(tm.players["a"], "enhanced"); err == nil {
		t.Fatal("normal boss converted")
	}
}
func TestOpeningIsIndependentFromParticipation(t *testing.T) {
	tm := fixture(t)
	tm.players["a"].player.Node = "b"
	tm.players["b"].player.Node = "b"
	before := tm.shared
	if err := tm.propose(tm.players["a"], "shop"); err != nil {
		t.Fatal(err)
	}
	if err := tm.castVote(tm.players["b"], true); err != nil {
		t.Fatal(err)
	}
	if tm.shared != before-1 || tm.players["a"].player.Node != "b" || tm.players["b"].player.Node != "b" {
		t.Fatal("vote moved players")
	}
	if err := tm.enter(tm.players["a"], "shop"); err != nil {
		t.Fatal(err)
	}
	if tm.players["b"].player.Node != "b" {
		t.Fatal("forced optional entry")
	}
	if err := tm.enter(tm.players["b"], "shop"); err != nil || tm.shared != before-1 {
		t.Fatal("charged twice", err)
	}
}
func TestFailedAdmissionDoesNotSpend(t *testing.T) {
	tm := fixture(t)
	ap := tm.players["a"].player.AP
	if err := tm.enter(tm.players["a"], "boss"); err == nil {
		t.Fatal("boss entered individually")
	}
	if tm.players["a"].player.AP != ap {
		t.Fatal("charged rejected entry")
	}
}
func TestCheckpointRestoresWithoutOriginalRoster(t *testing.T) {
	tm := fixture(t)
	saved := persistence.Checkpoint{Version: persistence.Version, Digest: tm.catalog.Digest, Seed: 1, State: tm.snapshot(), Identities: []persistence.Identity{{Player: "a", Token: "a"}, {Player: "b", Token: "b"}}}
	co, err := New(tm.catalog, Config{MaxPlayers: 2, Restore: &saved})
	if err != nil {
		t.Fatal(err)
	}
	defer co.Close()
	hello := protocol.Hello{Version: protocol.Version, Name: "new", Chassis: "base:scout", Digest: tm.catalog.Digest}
	if _, _, err := co.Connect(hello, make(chan protocol.Message, 32)); err != nil {
		t.Fatal("restore waited for roster", err)
	}
}
func TestEnhancedSettlementAndCurseDoNotStack(t *testing.T) {
	tm := fixture(t)
	if err := tm.enter(tm.players["a"], "a"); err != nil {
		t.Fatal(err)
	}
	tm.shared = 0
	if err := tm.propose(tm.players["b"], "enhanced"); err != nil {
		t.Fatal(err)
	}
	if err := tm.castVote(tm.players["a"], true); err != nil {
		t.Fatal(err)
	}
	if len(tm.actors) != 1 || tm.actors["boss"] == nil || tm.nodes["a"].Completed {
		t.Fatal("unfinished scene credited")
	}
	tm.stopScenes()
	tm.initialize(2)
	if tm.initialX != tm.catalog.Floor(tm.floor).PersonalAP-1 || tm.initialY != tm.catalog.Floor(tm.floor).SharedAP-1 {
		t.Fatal("missing curse")
	}
	tm.cursed = true
	tm.initialize(3)
	if tm.initialX != tm.catalog.Floor(tm.floor).PersonalAP-1 {
		t.Fatal("stacking curse")
	}
}
func TestRescueCannotHappenDuringCombat(t *testing.T) {
	tm := fixture(t)
	tm.players["a"].player.Node = "a"
	tm.players["b"].player.Node = "a"
	tm.players["b"].player.Life = "down"
	tm.players["b"].player.Knockdowns = 1
	tm.players["b"].player.HP = 0
	tm.players["b"].player.Scene = "a"
	if err := tm.rescue(tm.players["a"]); err == nil {
		t.Fatal("rescued active battle")
	}
	tm.players["b"].player.Scene = ""
	if err := tm.rescue(tm.players["a"]); err != nil {
		t.Fatal(err)
	}
	p := tm.players["b"].player
	if p.Downs != 1 || p.HP != 90 || p.Life != "alive" {
		t.Fatal(p)
	}
}

func installContent(t *testing.T, tm *team, c *content.Catalog) {
	t.Helper()
	tm.stopScenes()
	tm.runtime.Close()
	vm, err := script.New(c, tm.config.Seed)
	if err != nil {
		t.Fatal(err)
	}
	tm.catalog = c
	tm.runtime = vm
	t.Cleanup(vm.Close)
	tm.initialize(1)
	tm.phase = "exploring"
}
func TestRegisteredNodesAndVisitCheckpoint(t *testing.T) {
	tm := fixture(t)
	c := testcontent.Load(t, nil, `sdk.on("base:entry-reward","session","node_entered",function(ctx){return [{kind:"grant_coins",target:ctx.event.source,amount:7}];});`)
	installContent(t, tm, c)
	tm.nodes["shop"] = func() model.Node { n := tm.nodes["shop"]; n.Open = true; return n }()
	m := tm.players["a"]
	m.player.Node = "b"
	m.player.Coins = 3
	if err := tm.enter(m, "shop"); err != nil {
		t.Fatal(err)
	}
	if m.player.Coins != 10 {
		t.Fatal("entry event not committed")
	}
	if err := tm.request(m, protocol.Request{Kind: "upgrade"}); err != nil || m.player.Coins != 0 || m.player.WeaponBonus != 3 || m.player.Upgrade != 1 {
		t.Fatal(m.player, err)
	}
	before := m.player
	if err := tm.request(m, protocol.Request{Kind: "upgrade"}); err == nil || !reflect.DeepEqual(m.player, before) {
		t.Fatal("failed purchase committed")
	}
	tm.config.Checkpoint = filepath.Join(t.TempDir(), "checkpoint.json")
	if err := tm.save(); err != nil {
		t.Fatal(err)
	}
	saved, err := persistence.Load(tm.config.Checkpoint, c.Digest)
	if err != nil {
		t.Fatal(err)
	}
	co, err := New(c, Config{MaxPlayers: 2, Restore: &saved})
	if err != nil {
		t.Fatal(err)
	}
	defer co.Close()
	out := make(chan protocol.Message, 32)
	id, g, err := co.Connect(protocol.Hello{Version: protocol.Version, Digest: c.Digest, Token: "a"}, out)
	if err != nil {
		t.Fatal(err)
	}
	co.Submit(id, g, protocol.Request{Kind: "start", Seq: 1})
	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg := <-out:
			if msg.Kind == "error" {
				t.Fatal(msg)
			}
			if msg.State != nil && msg.State.Phase == "exploring" {
				for _, p := range msg.State.Players {
					if p.ID == id && p.Node == "shop" {
						if p.Coins != 0 || p.Upgrade != 1 {
							t.Fatal("replayed entry reward on restore", p)
						}
						return
					}
				}
			}
		case <-deadline:
			t.Fatal("restored interaction timeout")
		}
	}
}
func TestRejectedJoinDoesNotBroadcastOrSpend(t *testing.T) {
	tm := fixture(t)
	c := testcontent.Load(t, func(_ *content.Definitions, a *model.AdventureDef) { a.Rooms[0].InitialDelay = 0 }, `sdk.on("base:entry-counter","session","node_entered",function(ctx){return [{kind:"grant_coins",target:ctx.event.source,amount:7}];});`)
	installContent(t, tm, c)
	if err := tm.enter(tm.players["a"], "a"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for tm.actors["a"].Snapshot().State != "fighting" {
		if time.Now().After(deadline) {
			t.Fatal("battle did not start")
		}
		time.Sleep(time.Millisecond)
	}
	m := tm.players["b"]
	before := m.player
	if err := tm.enter(m, "a"); err == nil || !reflect.DeepEqual(m.player, before) || m.visited["a"] {
		t.Fatal("rejected admission committed")
	}
}
func TestCheckpointRejectsInvalidFloorOrDefinition(t *testing.T) {
	tm := fixture(t)
	for _, change := range []func(*persistence.Checkpoint){func(c *persistence.Checkpoint) { c.State.Floor = 999 }, func(c *persistence.Checkpoint) { c.State.Nodes[0].Definition = "base:missing" }} {
		saved := persistence.Checkpoint{Version: persistence.Version, Digest: tm.catalog.Digest, State: tm.snapshot(), Identities: []persistence.Identity{{Player: "a", Token: "a"}, {Player: "b", Token: "b"}}}
		change(&saved)
		if co, err := New(tm.catalog, Config{MaxPlayers: 2, Restore: &saved}); err == nil {
			co.Close()
			t.Fatal("invalid restore accepted")
		}
	}
}

func TestStatAllocationSurvivesRestoreAndInvalidSourcesAreRejected(t *testing.T) {
	tm := fixture(t)
	p := &tm.players["a"].player
	p.Level = 2
	p.Points = 1
	if err := tm.invest(tm.players["a"], "agility"); err != nil {
		t.Fatal(err)
	}
	saved := persistence.Checkpoint{Version: persistence.Version, Digest: tm.catalog.Digest, State: tm.snapshot(), Identities: []persistence.Identity{{Player: "a", Token: "a"}, {Player: "b", Token: "b"}}}
	co, err := New(tm.catalog, Config{MaxPlayers: 2, Restore: &saved})
	if err != nil {
		t.Fatal(err)
	}
	co.Close()
	for _, change := range []func(*model.Player){
		func(p *model.Player) { p.Attributes.Agility++ },
		func(p *model.Player) { p.BaseStats.Attributes.Agility++ },
		func(p *model.Player) { p.StatContributions[0].Source.Definition = "missing" },
		func(p *model.Player) { p.StatContributions[0].Source.Actor = "other" },
		func(p *model.Player) { p.StatContributions = append(p.StatContributions, p.StatContributions[0]) },
	} {
		bad := saved
		bad.State = protocol.CloneState(saved.State)
		change(&bad.State.Players[0])
		if co, err := New(tm.catalog, Config{MaxPlayers: 2, Restore: &bad}); err == nil {
			co.Close()
			t.Fatal("invalid stats accepted")
		}
	}
}
func TestCoordinatorPinsContentThroughPreparation(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	store := content.NewStore(c)
	co, err := New(c, Config{MaxPlayers: 1, ContentStore: store})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Reload(base.Files); err == nil {
		t.Fatal("coordinator failed to pin content")
	}
	co.Close()
	if err = store.Reload(base.Files); err != nil {
		t.Fatal("lease leaked", err)
	}
}

func TestPausedBattleClearedBeforeRescue(t *testing.T) {
	tm := fixture(t)
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		for i := range d.Weapons {
			d.Weapons[i].Base = 10000
			d.Weapons[i].Cooldown = .02
		}
		for i := range d.Enemies {
			d.Enemies[i].HP = 1
			d.Enemies[i].Damage = 1000
		}
		a.Rooms[0].InitialDelay = 0
		a.Rooms[0].SpawnRadius = model.Vec{}
		a.Rooms[0].Waves = a.Rooms[0].Waves[:1]
	}, "")
	installContent(t, tm, c)
	if err := tm.enter(tm.players["a"], "a"); err != nil {
		t.Fatal(err)
	}
	wait := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for !check() {
			if time.Now().After(deadline) {
				t.Fatal("scene transition timeout")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait(func() bool { tm.refresh(); return tm.views["a"].State == "paused" })
	if tm.players["a"].player.Life != "down" || len(tm.actors) != 1 || tm.nodes["a"].Completed {
		t.Fatal("discarded paused battle")
	}
	b := tm.players["b"]
	b.player.Armor = 1000000
	before := b.player.AP
	if err := tm.enter(b, "a"); err != nil {
		t.Fatal(err)
	}
	if b.player.AP != before-1 {
		t.Fatal("rescue admission did not spend personal AP")
	}
	if err := tm.rescue(b); err == nil {
		t.Fatal("rescued before clearing enemies")
	}
	actor := tm.actors["a"]
	wait(func() bool {
		actor.Input("b", model.Input{Aim: model.Vec{X: 400, Y: 250}, Attack: true})
		tm.refresh()
		return len(tm.actors) == 0
	})
	if err := tm.rescue(b); err != nil {
		t.Fatal(err)
	}
	a := tm.players["a"].player
	if a.Life != "alive" || a.Downs != 1 || a.HP != 90 || a.AP != tm.initialX-1 {
		t.Fatal(a)
	}
}
func TestWholeTeamDownLosesAndStopsScenes(t *testing.T) {
	tm := fixture(t)
	c := testcontent.Load(t, func(d *content.Definitions, a *model.AdventureDef) {
		for i := range d.Enemies {
			d.Enemies[i].Damage = 1000
		}
		a.Rooms[0].InitialDelay = 0
		a.Rooms[0].SpawnRadius = model.Vec{}
	}, "")
	installContent(t, tm, c)
	if err := tm.enter(tm.players["a"], "a"); err != nil {
		t.Fatal(err)
	}
	if err := tm.enter(tm.players["b"], "b"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for tm.phase != "lost" {
		tm.refresh()
		if time.Now().After(deadline) {
			t.Fatal("whole-team defeat timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(tm.actors) != 0 || tm.players["a"].player.Scene != "" || tm.players["b"].player.Scene != "" {
		t.Fatal("defeat left scenes running")
	}
}
