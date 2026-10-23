package protocol

import (
	"encoding/json"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
	"strings"
	"testing"
)

func TestNestedSnapshotAndDamageCopies(t *testing.T) {
	p := model.Player{StatContributions: []model.StatContribution{{StatModifier: model.StatModifier{Value: 2}}}}
	r := damage.Result{Steps: []damage.Step{{Members: []model.Modifier{{Percent: .2}}, AttributeSteps: []stats.Step{{Contributions: p.StatContributions}}}}}
	s := State{Players: []model.Player{p}, Nodes: []model.Node{{Edges: []string{"a"}}}, Scenes: []Scene{{Hexes: []model.HexCell{{Q: 1}}, Players: []model.Player{p}, LastDamage: &r}}, Vote: &Vote{Yes: []string{"p"}}}
	s.Performance = perf.Snapshot{Stages: []perf.Stage{{Name: "server"}}}
	s.Scenes[0].Performance = perf.Snapshot{Stages: []perf.Stage{{Name: "scene"}}}
	c := CloneState(s)
	c.Performance.Stages[0].Name = "changed"
	c.Scenes[0].Performance.Stages[0].Name = "changed"
	c.Players[0].StatContributions[0].Value = 99
	c.Nodes[0].Edges[0] = "changed"
	c.Scenes[0].Players[0].StatContributions[0].Value = 99
	c.Scenes[0].Hexes[0].Q = 99
	c.Scenes[0].LastDamage.Steps[0].Members[0].Percent = 99
	c.Scenes[0].LastDamage.Steps[0].AttributeSteps[0].Contributions[0].Value = 99
	c.Vote.Yes[0] = "changed"
	if p.StatContributions[0].Value != 2 || r.Steps[0].Members[0].Percent != .2 || s.Nodes[0].Edges[0] != "a" || s.Vote.Yes[0] != "p" || s.Scenes[0].Hexes[0].Q != 1 {
		t.Fatal("mutable snapshot escaped")
	}
	if s.Performance.Stages[0].Name != "server" || s.Scenes[0].Performance.Stages[0].Name != "scene" {
		t.Fatal("diagnostics slices alias published snapshots")
	}
}

func TestEightPlayersWithBoundedStatusLayersFitAndCloneIndependently(t *testing.T) {
	d := model.StatusDef{ID: "base:" + strings.Repeat("d", 123), Name: "Test", Level: 1, MaxLayers: status.MaxLayers, DurationMillis: 1000}
	for i := 0; i < 8; i++ {
		d.Modifiers = append(d.Modifiers, model.StatModifier{ID: strings.Repeat(string(rune('a'+i)), 128), Stat: "strength", Operation: "percent", Group: strings.Repeat("g", 128), Value: .01})
	}
	var set model.StatusSet
	var err error
	for i := 0; i < status.MaxLayers; i++ {
		set, err = status.Apply(set, d, model.SourceRef{Kind: strings.Repeat("k", 128), Actor: strings.Repeat("a", 128), Instance: strings.Repeat("i", 128), Definition: d.ID}, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	s := State{Scenes: []Scene{{ID: "a"}}}
	for i := 0; i < 8; i++ {
		p := model.Player{ID: string(rune('a' + i)), Statuses: model.CloneStatuses(set)}
		s.Players = append(s.Players, p)
		s.Scenes[0].Players = append(s.Scenes[0].Players, p)
	}
	data, err := json.Marshal(Message{Kind: "state", State: &s})
	if err != nil || len(data) >= MaxMessage {
		t.Fatal("bounded status snapshot exceeds frame limit", len(data), err)
	}
	copy := CloneState(s)
	copy.Scenes[0].Players[0].Statuses.Instances[0].Layers[0].Modifiers[0].Value = 99
	if s.Players[0].Statuses.Instances[0].Layers[0].Modifiers[0].Value == 99 {
		t.Fatal("nested status snapshot aliases authority")
	}
}
