package script_test

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/sdk"
	"strings"
	"testing"
)

func TestBehaviorIsolationTimeAndRandom(t *testing.T) {
	c := testcontent.Load(t, nil, `
 var counter=0;
 sdk.registerBehavior("base:counter",function(ctx){counter++;return {move:{x:counter,y:Date.now()},attack:Math.random()<0.5};});
 `)
	a, err := script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	b, err := script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	ctx := sdk.Context{Self: model.Vec{X: 4, Y: 5}, Target: model.Vec{X: 10, Y: 20}, Distance: 12}
	act, err := a.Decide("base:chase", ctx)
	if err != nil || act.Move.X != 6 || act.Move.Y != 15 || !act.Attack {
		t.Fatal(act, err)
	}
	ctx.Time = 3
	first, _ := a.Decide("base:counter", ctx)
	second, _ := a.Decide("base:counter", ctx)
	isolated, _ := b.Decide("base:counter", ctx)
	if first.Move.X != 1 || second.Move.X != 2 || isolated != first || first.Move.Y != 3000 {
		t.Fatal(first, second, isolated)
	}
	a.Close()
	if _, err = a.Decide("base:chase", ctx); err == nil {
		t.Fatal("closed runtime used")
	}
}
func TestRegistrationFailures(t *testing.T) {
	for _, code := range []string{`while(true){}`, `sdk.registerBehavior("base:chase",function(){return {};});`, `sdk.on("base:invalid","session","hit",function(){});`} {
		files := testcontent.Files(t)
		files["behavior.js"].Data = append(files["behavior.js"].Data, []byte("\n"+code)...)
		if _, err := content.Load(files); err == nil {
			t.Fatal("invalid registration accepted")
		}
	}
}
func TestEventScopeUnsubscribeIsolationAndClose(t *testing.T) {
	c := testcontent.Load(t, nil, `
 var count=0;
 var removeB;
 sdk.on("base:a","scene","hit",function(ctx){if(Date.now()!==ctx.time*1000)throw Error("stale event time");count++;removeB();ctx.players[0].coins=999;return [{kind:"grant_coins",target:ctx.players[0].id,amount:count}];});
 removeB=sdk.on("base:b","scene","hit",function(ctx){return [{kind:"grant_coins",target:ctx.players[0].id,amount:ctx.players[0].coins}];});
 `)
	r, err := script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	ctx := sdk.EventContext{Scope: "scene", Time: 4, Event: model.Event{Kind: "hit"}, Players: []model.Player{{ID: "p", Coins: 7}}}
	cmds, err := r.Dispatch(ctx)
	if err != nil || len(cmds) != 2 || cmds[0].Amount != 1 || cmds[1].Amount != 7 || ctx.Players[0].Coins != 7 {
		t.Fatal(cmds, err, ctx)
	}
	cmds, err = r.Dispatch(ctx)
	if err != nil || len(cmds) != 1 || cmds[0].Amount != 2 {
		t.Fatal(cmds, err)
	}
	ctx.Scope = "session"
	cmds, err = r.Dispatch(ctx)
	if err != nil || len(cmds) != 0 {
		t.Fatal("wrong scope", cmds, err)
	}
	r.Close()
	if _, err = r.Dispatch(ctx); err == nil {
		t.Fatal("closed listener used")
	}
}
func TestCallbackBudgetAndFrozenRegistration(t *testing.T) {
	c := testcontent.Load(t, nil, `
 sdk.registerBehavior("base:slow",function(){while(true){}});
 sdk.registerBehavior("base:late",function(){sdk.registerBehavior("base:later",function(){return {};});return {};});
 `)
	r, err := script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	if _, err = r.Decide("base:slow", sdk.Context{}); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatal(err)
	}
	if _, err = r.Decide("base:chase", sdk.Context{}); err != nil {
		t.Fatal("interrupt poisoned next callback", err)
	}
	if _, err = r.Decide("base:late", sdk.Context{}); err == nil || !strings.Contains(err.Error(), "registration closed") {
		t.Fatal(err)
	}
}

type nativeNode struct{}

func (nativeNode) Execute(ctx sdk.NodeContext) ([]sdk.Command, error) {
	ctx.Node.Edges[0] = "mutated"
	ctx.Parameters["amount"] = 999
	return []sdk.Command{{Kind: "grant_coins", Target: ctx.Player.ID, Amount: 2}}, nil
}

type nativeObserver struct{}

func (nativeObserver) Handle(ctx sdk.EventContext) ([]sdk.Command, error) {
	ctx.Players[0].Coins = 999
	if len(ctx.Players[0].StatContributions) > 0 {
		ctx.Players[0].StatContributions[0].Value = 999
	}
	return []sdk.Command{{Kind: "heal", Target: ctx.Players[0].ID, Amount: 1}}, nil
}
func TestGoRegistrationUsesDetachedContract(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	r, err := script.NewRegistered(c, 1, func(reg sdk.Registrar) error {
		if err := reg.RegisterNode("base:native", map[string]sdk.Parameter{"amount": {Type: "integer", Minimum: 1, Maximum: 10}}, nativeNode{}); err != nil {
			return err
		}
		_, err := reg.RegisterEvent("base:native-event", "scene", "hit", nativeObserver{})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	ctx := sdk.NodeContext{Player: model.Player{ID: "p"}, Node: model.Node{Edges: []string{"original"}}, Parameters: map[string]any{"amount": 2}}
	commands, err := r.Node("base:native", ctx)
	if err != nil || len(commands) != 1 || ctx.Node.Edges[0] != "original" || ctx.Parameters["amount"] != 2 {
		t.Fatal(ctx, commands, err)
	}
	event := sdk.EventContext{Scope: "scene", Event: model.Event{Kind: "hit"}, Players: []model.Player{{ID: "p", Coins: 3, StatContributions: []model.StatContribution{{StatModifier: model.StatModifier{Value: 2}}}}}}
	if _, err = r.Dispatch(event); err != nil || event.Players[0].Coins != 3 || event.Players[0].StatContributions[0].Value != 2 {
		t.Fatal(event, err)
	}
}

func TestJSSeesFlatContributionFieldsAndCannotMutateOwner(t *testing.T) {
	c := testcontent.Load(t, nil, `sdk.on("base:stats-read","scene","hit",function(ctx){var c=ctx.players[0].stat_contributions[0];if(c.stat!=="strength"||c.value!==2||c.source.instance!=="ring-a")throw Error("invalid contribution contract");c.value=999;return [{kind:"grant_coins",target:ctx.players[0].id,amount:1}];});`)
	r, err := script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	ctx := sdk.EventContext{Scope: "scene", Event: model.Event{Kind: "hit"}, Players: []model.Player{{ID: "p", StatContributions: []model.StatContribution{{StatModifier: model.StatModifier{ID: "test", Stat: "strength", Operation: "add", Value: 2}, Source: model.SourceRef{Kind: "item", Actor: "p", Instance: "ring-a", Definition: "test:ring"}}}}}}
	commands, err := r.Dispatch(ctx)
	if err != nil || len(commands) != 1 || ctx.Players[0].StatContributions[0].Value != 2 {
		t.Fatal(commands, err)
	}
}
