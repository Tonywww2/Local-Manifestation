package client

import (
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"hsmultiplayer/pkg/model"
	"slices"
	"strings"
	"testing"
)

func TestRealStatusUsesHoverOnlyAndSharedCalculations(t *testing.T) {
	g := interfaceGame(t)
	g.catalog = testcontent.Load(t, func(d *content.Definitions, _ *model.AdventureDef) {
		d.Statuses = []model.StatusDef{{ID: "base:test-focus", Name: "Focus", Description: "Raises strength", Level: 2, MaxLayers: 2, DurationMillis: 2000, Modifiers: []model.StatModifier{{ID: "base:focus-str", Stat: "strength", Operation: "add", Value: 5}}}}
	}, "")
	p, _ := content.Player(g.catalog, "me", "Me", "base:scout")
	var err error
	p, err = content.ApplyStatus(g.catalog, p, "base:test-focus", model.SourceRef{Kind: "weapon", Actor: p.ID, Instance: "weapon-1", Definition: p.Weapon})
	if err != nil {
		t.Fatal(err)
	}
	g.state.Players = []model.Player{p}
	g.buildInterface()
	var found uiControl
	for _, c := range g.u.frame.Controls {
		if strings.HasPrefix(c.ID, "status:") {
			found = c
		}
	}
	if found.ID == "" || found.Action != "" || found.Key != "" || found.Badge == "" || len(found.Details) == 0 {
		t.Fatal("status is not a timed hover icon")
	}
	windows := slices.Clone(g.u.windows)
	g.performControl(found)
	if !slices.Equal(windows, g.u.windows) {
		t.Fatal("status click opened a page")
	}
	g.u.hover, g.u.focus = found.ID, "hover"
	g.buildInterface()
	compact := labelsFor(g, "hover")
	g.u.expanded = true
	g.buildInterface()
	expanded := labelsFor(g, "hover")
	if strings.Contains(compact, "=") || !strings.Contains(expanded, "=") || strings.Contains(expanded, "ui.status.") {
		t.Fatal("status expansion or localization", compact, expanded)
	}
}

func TestOverflowStatusesCanAllBeScrolledWithoutOpeningPages(t *testing.T) {
	g := interfaceGame(t)
	g.catalog = testcontent.Load(t, func(d *content.Definitions, _ *model.AdventureDef) {
		d.Statuses = []model.StatusDef{{ID: "base:test-many", Name: "Many", Description: "Test", Level: 1, MaxLayers: 1, DurationMillis: 10000}}
	}, "")
	p, _ := content.Player(g.catalog, "me", "Me", "base:scout")
	for i := 0; i < 16; i++ {
		var err error
		p, err = content.ApplyStatus(g.catalog, p, "base:test-many", model.SourceRef{Kind: "weapon", Actor: p.ID, Instance: fmt.Sprint(i), Definition: p.Weapon})
		if err != nil {
			t.Fatal(err)
		}
	}
	g.state.Players = []model.Player{p}
	g.buildInterface()
	if !slices.ContainsFunc(g.u.frame.Controls, func(c uiControl) bool { return c.ID == "statuses:more" && c.Action == "" }) {
		t.Fatal("status overflow is invisible")
	}
	last := fmt.Sprintf("status:%d", p.Statuses.Instances[15].ID)
	g.u.scroll["statuses"] = 100
	g.buildInterface()
	if !slices.ContainsFunc(g.u.frame.Controls, func(c uiControl) bool { return c.ID == last }) {
		t.Fatal("last status cannot be reached by scrolling")
	}
}
