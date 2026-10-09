package client

import (
	"fmt"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/persistence"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/progression"
	"path/filepath"
	"slices"
)

func (g *Game) progressionPath() string { return filepath.Clean(g.cfg.Profile + ".progress.json") }
func (g *Game) initializeProgression() {
	p, err := persistence.LoadProfiles(g.progressionPath())
	if err != nil || p.Version != 1 {
		g.progressionError = fmt.Errorf("progression profile could not be loaded: %v", err)
		g.notifyError(g.progressionError)
		p = persistence.Profiles{Version: 1, Scopes: map[string]model.Progression{}}
	}
	g.profiles = p
}
func (g *Game) localScope() string {
	if g.catalog.Namespace() != "base" || g.catalog.Modified() || g.front.Config.SpecialMode || g.front.Config.MetaRules != nil && *g.front.Config.MetaRules != model.DefaultMetaRules() {
		return "custom:" + g.catalog.Digest
	}
	return "normal"
}
func (g *Game) joinRanks() map[string]int {
	ranks := map[string]int{}
	for _, scope := range []string{"normal", g.localScope()} {
		for id, rank := range g.profiles.Scopes[scope].Ranks {
			if _, ok := g.catalog.Perk(id); ok {
				ranks[id] = max(rank, ranks[id])
			}
		}
	}
	return ranks
}
func (g *Game) storeProgression(scope string, p model.Progression) error {
	if g.progressionError != nil {
		return g.progressionError
	}
	if err := progression.Validate(p, g.catalog.Perk); err != nil {
		return err
	}
	n := persistence.Profiles{Version: 1, Scopes: map[string]model.Progression{}}
	for id, v := range g.profiles.Scopes {
		n.Scopes[id] = model.CloneProgression(v)
	}
	n.Scopes[scope] = p
	if err := persistence.SaveProfiles(g.progressionPath(), n); err != nil {
		return err
	}
	g.profiles = n
	return nil
}
func (g *Game) settleProgression() {
	if g.state == nil || g.state.Phase != "won" && g.state.Phase != "lost" || g.previewOnly {
		return
	}
	p := g.self()
	if p == nil {
		return
	}
	scope := g.state.ProgressionScope
	points, rewards := g.catalog.CompletionRewards(*p, g.profiles.Scopes[scope])
	n, changed, err := progression.Settle(g.profiles.Scopes[scope], g.state.RunID, *p, g.state.Phase == "won", points)
	if err == nil && changed {
		if g.state.Phase == "won" {
			n.Achievements = append(n.Achievements, rewards...)
		}
		err = g.storeProgression(scope, n)
	}
	if err != nil {
		g.notifyError(err)
	}
}
func (g *Game) progressionAction(c uiControl) bool {
	if c.Action != "learn" {
		return false
	}
	if g.state != nil || g.frontBusy() {
		return true
	}
	scope := g.localScope()
	n, err := progression.Learn(g.profiles.Scopes[scope], c.Value, g.catalog.Perk)
	if err == nil {
		err = g.storeProgression(scope, n)
	}
	if err != nil {
		g.notifyError(err)
	}
	return true
}
func (g *Game) buildProgression(f *uiFrame, id string, r ui.Rect) {
	scope := g.localScope()
	p := g.profiles.Scopes[scope]
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 38, W: r.W - 28, H: 42}, g.tr("ui.meta.scope", "scope", scope, "points", p.Points))
	if id == "warehouse" {
		start := min(g.u.scroll[id], max(0, len(p.Warehouse)-12))
		for i, it := range p.Warehouse[start:min(start+12, len(p.Warehouse))] {
			d, _ := g.catalog.Item(it.Item.Definition)
			g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 84 + float64(i)*23, W: r.W - 28, H: 23}, fmt.Sprintf("%s +%d · %s", g.contentName(d.ID, d.Name), it.Item.Upgrade, it.RunID[:min(8, len(it.RunID))]))
		}
		return
	}
	if id == "achievements" {
		start := min(g.u.scroll[id], max(0, len(g.catalog.Definitions().CompletionRewards)-10))
		values := g.catalog.Definitions().CompletionRewards
		for i, d := range values[start:min(start+10, len(values))] {
			g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 85 + float64(i)*28, W: r.W - 28, H: 28}, g.tr("ui.meta.reward", "name", g.contentName(d.ID, d.Name), "done", slices.Contains(p.Achievements, d.ID), "points", d.Points))
		}
		return
	}
	addControl(f, id, "meta:achievements", "status", "", "window", "achievements", g.tr("hall.achievements"), ui.Rect{X: r.X + r.W - 80, Y: r.Y + 4, W: 38, H: 24})
	perks := g.catalog.Perks()
	start := min(g.u.scroll[id], max(0, len(perks)-5))
	for i, d := range perks[start:min(start+5, len(perks))] {
		y := r.Y + 88 + float64(i)*54
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: y, W: r.W - 85, H: 45}, g.tr("ui.meta.perk", "name", g.contentName(d.ID, d.Name), "rank", p.Ranks[d.ID], "max", d.MaxRank, "cost", d.Cost))
		c := uiControl{Owner: id, ID: "learn:" + d.ID, Icon: "plus", Action: "learn", Value: d.ID, Title: g.contentName(d.ID, d.Name), Summary: []string{g.localized("content."+d.ID+".description", d.Description)}, Bounds: ui.Rect{X: r.X + r.W - 54, Y: y, W: 38, H: 32}}
		f.Controls = append(f.Controls, c)
	}
}
func (g *Game) toggleMetaRule(key string) {
	if g.front.Draft == nil {
		return
	}
	if g.front.Draft.MetaRules == nil {
		r := model.DefaultMetaRules()
		g.front.Draft.MetaRules = &r
	}
	r := g.front.Draft.MetaRules
	switch key {
	case "stats":
		r.Stats = !r.Stats
	case "coins":
		r.Coins = !r.Coins
	case "equipment":
		r.Equipment = !r.Equipment
	case "ap":
		r.AP = !r.AP
	case "nodes":
		r.Nodes = !r.Nodes
	case "unlocks":
		r.Unlocks = !r.Unlocks
	}
}
func (g *Game) buildRevival(f *uiFrame, layout ui.Layout) {
	r := g.windowPanel(f, layout, "revival", g.tr("ui.revival"), ui.Size{W: 440, H: 310})
	d := g.catalog.Node(g.currentNode().Definition)
	g.addLabel(f, "revival", ui.Rect{X: r.X + 14, Y: r.Y + 38, W: r.W - 28, H: 48}, g.tr("ui.revival.help", "cost", d.FacilityCost))
	row := 0
	for _, p := range g.state.Players {
		if p.Life != "dead" {
			continue
		}
		y := r.Y + 100 + float64(row)*40
		g.addLabel(f, "revival", ui.Rect{X: r.X + 14, Y: y, W: r.W - 85, H: 30}, p.Name)
		addControl(f, "revival", "revive:"+p.ID, "heart", "", "revive", p.ID, g.tr("ui.revival"), ui.Rect{X: r.X + r.W - 56, Y: y, W: 38, H: 30})
		row++
	}
}
func (g *Game) buildAccessories(f *uiFrame, layout ui.Layout) {
	r := g.windowPanel(f, layout, "accessories", g.tr("ui.inventory.accessories.title"), ui.Size{W: 510, H: 330})
	p := g.self()
	if p == nil {
		return
	}
	v := g.displayInventory()
	start := min(g.u.scroll["accessories"]*8, max(0, accessoryDisplaySlots(v)-32))
	for i := 0; i < min(32, accessoryDisplaySlots(v)-start); i++ {
		it := model.ItemInstance{}
		if start+i < len(v.Accessories) {
			it, _ = inventory.Find(v, v.Accessories[start+i])
		}
		c := g.itemControl("accessories", fmt.Sprintf("accessory-all:%d", start+i), it, ui.Rect{X: r.X + 14 + float64(i%8)*60, Y: r.Y + 45 + float64(i/8)*52, W: 52, H: 44}, "inventory-accessory")
		c.Value = fmt.Sprint(start + i)
		f.Controls = append(f.Controls, c)
	}
}
