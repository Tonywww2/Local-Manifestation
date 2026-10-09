package content_test

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"hsmultiplayer/sdk"
	"reflect"
	"testing"
)

func statusCatalog(t *testing.T) *content.Catalog {
	return testcontent.Load(t, func(d *content.Definitions, _ *model.AdventureDef) {
		d.Statuses = []model.StatusDef{{ID: "base:vigor", Name: "Vigor", Description: "Test health effect", Level: 2, MaxLayers: 2, DurationMillis: 2000, Modifiers: []model.StatModifier{{ID: "base:vigor-hp", Stat: "max_hp", Operation: "percent", Group: "status", Value: .5}}}}
	}, `sdk.registerNode("base:vigor-node",{},function(ctx){return [{kind:"apply_status",target:ctx.player.id,status:"base:vigor",source:{kind:"node",actor:ctx.player.id,instance:ctx.node.id,definition:ctx.node.definition}}];});`)
}

func TestJSStatusCommandHealthRatioExpiryAndAtomicFailure(t *testing.T) {
	c := statusCatalog(t)
	p, _ := content.Player(c, "p", "p", "base:scout")
	p.HP = p.MaxHP / 2
	vm, err := script.New(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer vm.Close()
	commands, err := vm.Node("base:vigor-node", sdk.NodeContext{Player: p, Node: model.Node{ID: "shrine", Definition: "base:shrine"}})
	if err != nil {
		t.Fatal(err)
	}
	next, err := content.ApplyCommands(c, map[string]model.Player{"p": p}, commands, content.NodeCommands)
	if err != nil || next["p"].HP != 90 || next["p"].MaxHP != 180 || p.HP != 60 {
		t.Fatal("status not in derived values or health ratio", next, err)
	}
	if err := content.ValidatePlayerStats(c, next["p"], 1); err != nil {
		t.Fatal("cannot persist genuine status", err)
	}
	expired, err := content.AdvancePlayerStatuses(next["p"], 2*status.UnitsPerSecond)
	if err != nil || expired.HP != 60 || expired.MaxHP != 120 || len(expired.Statuses.Instances) != 0 {
		t.Fatal("expiry did not restore proportion", expired, err)
	}
	failed := append(sdk.CloneCommands(commands), sdk.Command{Kind: "spend_coins", Target: "p", Amount: 1000})
	if replacement, err := content.ApplyCommands(c, map[string]model.Player{"p": p}, failed, content.NodeCommands); err == nil || replacement != nil || p.HP != 60 || len(p.Statuses.Instances) != 0 {
		t.Fatal("partial status batch")
	}
	copy, _ := c.Status("base:vigor")
	copy.Modifiers[0].Value = 77
	actual, _ := c.Status("base:vigor")
	if reflect.DeepEqual(copy, actual) {
		t.Fatal("catalog definition escaped")
	}
}
