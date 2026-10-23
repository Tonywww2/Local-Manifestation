package client

import (
	"fmt"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"slices"
	"strconv"
	"strings"
)

func (g *Game) itemControl(owner, id string, it model.ItemInstance, r ui.Rect, action string) uiControl {
	d, ok := g.catalog.Item(it.Definition)
	c := uiControl{ID: id, Owner: owner, Icon: "empty", Action: action, Value: it.ID, Bounds: r, Title: g.tr("ui.slot.empty")}
	if !ok {
		return c
	}
	c.Item = it
	c.ItemKind = d.Kind
	c.Count = inventory.Count(it)
	key := g.itemViewKey(it)
	if info, ok := g.itemViews[key]; ok {
		c.Icon = info.Icon
		c.Title = info.Title
		c.Summary = info.Summary
		c.Details = info.Details
		c.Badge = info.Badge
		return c
	}
	c.Icon = d.Kind
	if d.Kind == "armor" {
		c.Icon = d.Slot
	}
	if d.Kind == "weapon" {
		c.Icon = weaponIcon(g.catalog.Weapon(d.Weapon))
	}
	if d.Kind == "accessory" {
		c.Icon = "status"
	}
	c.Title = g.contentName(d.ID, d.Name)
	c.Summary = []string{g.tr("ui.item.type", "type", g.localized("ui.item.kind."+d.Kind, d.Kind)), g.localized("content."+d.ID+".description", d.Description)}
	if d.Kind == "movement" || d.Kind == "passive" {
		c.Icon = d.Kind
		c.Summary = append(c.Summary, g.tr("ui.item.bag_only"))
		if d.Durability == -1 {
			c.Summary = append(c.Summary, g.tr("ui.item.unlimited"))
			c.Badge = "∞"
		} else {
			remaining := it.Durability
			if remaining == 0 {
				remaining = d.Durability
			}
			c.Summary = append(c.Summary, g.tr("ui.item.durability", "remaining", remaining, "maximum", d.Durability))
			c.Badge = fmt.Sprint(remaining)
		}
	}
	if d.Kind == "weapon" {
		if p := g.self(); p != nil {
			if q, err := g.catalog.PreviewWeaponAttack(*p, it); err == nil {
				if result, err := content.EvaluatePreview(q); err == nil {
					c.Summary = append(c.Summary, g.weaponDamageSummary(result, p.Inventory.Equipment[0] == it.ID))
				}
			}
		}
		if skill, ok := g.catalog.WeaponSkill(g.catalog.Weapon(d.Weapon).Skill); ok {
			c.Summary = append(c.Summary, g.tr("ui.weapon_skill.named", "name", g.contentName(skill.ID, skill.Name)))
			c.Details = append(c.Details, g.weaponSkillLines(skill)...)
		}
	}
	c.Details = append(c.Details, g.tr("ui.item.value", "value", g.catalog.ItemValue(it)))
	if d.Tier > 0 {
		c.Summary = append(c.Summary, g.tr("ui.item.tier", "tier", d.Tier))
	}
	if d.Lore != "" {
		c.Details = append(c.Details, g.localized("content."+d.ID+".lore", d.Lore))
	}
	if p := g.self(); p != nil {
		for _, e := range d.Combat {
			switch e.Kind {
			case "critical_damage":
				c.Details = append(c.Details, g.tr("ui.item.crit_formula", "factor", fmt.Sprintf("%.2f", 1+e.Amount), "total", fmt.Sprintf("%.2f", 150*(1+e.Amount))))
			case "healing":
				c.Details = append(c.Details, g.tr("ui.item.heal_factor", "factor", fmt.Sprintf("%.2f", 1+e.Amount)))
			case "append_damage":
				if q, err := content.Attack(g.catalog, *p); err == nil {
					if base, err := content.PreMultiplierBase(q); err == nil {
						c.Details = append(c.Details, g.tr("ui.item.feedback_formula", "base", fmt.Sprintf("%.2f", base), "total", fmt.Sprintf("%.2f", base*e.Amount)))
					}
				}
			case "battle_shield":
				c.Details = append(c.Details, g.tr("ui.item.shield_formula", "hp", fmt.Sprintf("%.2f", p.MaxHP), "shield", fmt.Sprintf("%.2f", damage.Round(p.MaxHP*.4))))
			case "hit_multiplier", "low_health_reduction":
				c.Details = append(c.Details, g.tr("ui.item.conditional"))
			}
		}
	}
	if inventory.StackLimit(d) > 1 {
		c.Summary = append(c.Summary, g.tr("ui.item.stack", "count", inventory.Count(it), "limit", inventory.StackLimit(d)))
	}
	for _, effect := range d.Use {
		if effect.Kind == "heal" && g.self() != nil {
			p := g.self()
			c.Details = append(c.Details, g.tr("ui.item.heal_formula", "factor", fmt.Sprintf("%.2f", g.catalog.HealingFactor(*p)), "base", fmt.Sprintf("%.2f", effect.Amount), "bonus", fmt.Sprintf("%.2f", effect.Scaling.Dot(p.Attributes)), "total", fmt.Sprintf("%.2f", g.catalog.HealingAmount(*p, effect.Amount+effect.Scaling.Dot(p.Attributes), false))))
		}
	}
	if a, ok := g.catalog.Affix(it.Affix); ok {
		c.Title = g.contentName(a.ID, a.Name) + " " + c.Title
		for _, m := range a.Modifiers {
			c.Details = append(c.Details, g.statModifierText(m))
		}
	}
	if it.Upgrade > 0 {
		c.Badge = fmt.Sprintf("+%d", it.Upgrade)
	}
	for _, m := range d.Modifiers {
		c.Details = append(c.Details, g.statModifierText(m))
	}
	if g.self() != nil {
		g.equipmentComparison(&c, it, d, *g.self())
	}
	if g.resources.images["icon:"+d.ID] != nil {
		c.Icon = d.ID
	}
	if g.itemViews == nil || len(g.itemViews) > 512 {
		g.itemViews = map[itemViewKey]itemView{}
	}
	g.itemViews[key] = itemView{c.Icon, c.Title, c.Badge, c.Summary, c.Details}
	return c
}
func (g *Game) sendItem(r protocol.Request) bool {
	if g.link != nil {
		return g.link.Send(r)
	}
	return false
}
func (g *Game) equipItem(id string) {
	p := g.self()
	if p == nil {
		return
	}
	it, ok := inventory.Find(p.Inventory, id)
	if !ok {
		return
	}
	d, _ := g.catalog.Item(it.Definition)
	slot := 0
	if inventory.Equipped(p.Inventory, id) {
		for i, x := range p.Inventory.Equipment {
			if x == id {
				slot = i
			}
		}
		for i, x := range p.Inventory.Accessories {
			if x == id {
				slot = i + 9
			}
		}
		id = ""
	} else {
		switch d.Kind {
		case "armor":
			slot = slices.Index(inventory.Slots[:], d.Slot)
		case "consumable":
			slot = 5
			for i := 5; i < 9; i++ {
				if p.Inventory.Equipment[i] == "" {
					slot = i
					break
				}
			}
		case "accessory":
			slot = 9 + len(p.Inventory.Accessories)
		}
	}
	g.sendItem(protocol.Request{Kind: "equip", Item: id, Slot: slot})
}
func (g *Game) buildInventory(f *uiFrame, layout ui.Layout, id string) bool {
	if id == "armor" {
		g.buildArmor(f, layout)
		return true
	}
	if id != "backpack" && id != "trade" && !strings.HasPrefix(id, "peer:") {
		if id == "revival" {
			g.buildRevival(f, layout)
			return true
		}
		if id == "accessories" {
			g.buildAccessories(f, layout)
			return true
		}
		if id == "loot" {
			g.buildLoot(f, layout)
			return true
		}
		if id == "shop" {
			g.buildShop(f, layout)
			return true
		}
		return false
	}
	p := g.self()
	if p == nil {
		return false
	}
	if strings.HasPrefix(id, "peer:") {
		title, short, _ := g.windowInfo(id)
		r := g.windowPanel(f, layout, id, title, ui.Size{W: 360, H: 240})
		g.addWindowLines(f, id, r, short, 130)
		addControl(f, id, id+":trade", "person", "", "trade-start", strings.TrimPrefix(id, "peer:"), g.tr("ui.trade.start"), ui.Rect{X: r.X + 14, Y: r.Y + r.H - 42, W: 38, H: 30})
		return true
	}
	if id == "trade" {
		g.buildTrade(f, layout)
		return true
	}
	g.buildCharacterInventory(f, layout)
	return true
}
func (g *Game) currentTrade() (protocol.Trade, int, bool) {
	if g.state != nil {
		for _, v := range g.state.Trades {
			if i := slices.Index(v.Players[:], g.id); i >= 0 {
				return v, i, true
			}
		}
	}
	return protocol.Trade{}, 0, false
}
func (g *Game) buildTrade(f *uiFrame, layout ui.Layout) {
	r := g.windowPanel(f, layout, "trade", g.tr("ui.trade"), ui.Size{W: 590, H: 480})
	v, side, ok := g.currentTrade()
	if !ok {
		g.addWindowLines(f, "trade", r, []string{g.tr("ui.trade.closed")}, 120)
		return
	}
	g.addLabel(f, "trade", ui.Rect{X: r.X + 14, Y: r.Y + 40, W: r.W - 28, H: 22}, g.tr("ui.trade.help"))
	for i := 0; i < 2; i++ {
		g.addLabel(f, "trade", ui.Rect{X: r.X + 14 + float64(i)*280, Y: r.Y + 70, W: 260, H: 22}, g.tr("ui.trade.offer", "player", v.Players[i], "accepted", v.Accepted[i]))
		var inv model.Inventory
		for _, p := range g.state.Players {
			if p.ID == v.Players[i] {
				inv = p.Inventory
			}
		}
		start := min(g.u.scroll["trade"]*4, max(0, len(v.Offers[i])-12))
		for j, itID := range v.Offers[i][start:min(start+12, len(v.Offers[i]))] {
			it, _ := inventory.Find(inv, itID)
			f.Controls = append(f.Controls, g.itemControl("trade", fmt.Sprintf("offer:%d:%d", i, j), it, ui.Rect{X: r.X + 14 + float64(i)*280 + float64(j%4)*62, Y: r.Y + 100 + float64(j/4)*50, W: 54, H: 44}, ""))
		}
	}
	if p := g.self(); p != nil {
		bag := inventory.Bag(p.Inventory)
		start := min(g.u.scroll["trade"]*9, max(0, len(bag)-18))
		for i, it := range bag[start:min(start+18, len(bag))] {
			c := g.itemControl("trade", fmt.Sprintf("trade-bag:%d", i), it, ui.Rect{X: r.X + 14 + float64(i%9)*61, Y: r.Y + 270 + float64(i/9)*50, W: 54, H: 44}, "trade-toggle")
			if slices.Contains(g.u.tradeItems, it.ID) {
				c.Badge = "✓"
			}
			f.Controls = append(f.Controls, c)
		}
	}
	_ = side
	for i, a := range []string{"trade-update", "trade-accept", "trade-cancel"} {
		addControl(f, "trade", a, "approve", "", a, v.ID, g.tr("ui."+a), ui.Rect{X: r.X + 14 + float64(i)*50, Y: r.Y + r.H - 44, W: 38, H: 30})
	}
}
func (g *Game) itemAction(c uiControl) bool {
	switch c.Action {
	case "item-details":
		g.openItemDetails(c.Item)
		g.u.itemMenu = nil
	case "formula-pin":
		if c.Inspection != nil {
			g.openInspection(*c.Inspection)
		}
	case "formula-left", "formula-right":
		if g.u.formulaOffsets == nil {
			g.u.formulaOffsets = map[string]float64{}
		}
		delta := 180.0
		if c.Action == "formula-left" {
			delta = -delta
		}
		g.u.formulaOffsets[c.Value] = max(0, min(float64(c.Count), g.u.formulaOffsets[c.Value]+delta))
	case "inventory-bag", "inventory-equipment", "inventory-accessory":
		g.inventoryClick(c, false)
	case "equip-slot":
		slot, err := strconv.Atoi(c.Value)
		if err == nil && g.self() != nil {
			if g.u.selectedItem != "" && g.u.selectedItem != g.self().Inventory.Equipment[slot] {
				g.sendItem(protocol.Request{Kind: "equip", Item: g.u.selectedItem, Slot: slot})
			} else {
				g.u.selectedItem = g.self().Inventory.Equipment[slot]
			}
		}
	case "item-select":
		g.u.selectedItem = c.Value
	case "item-equip":
		g.equipItem(c.Value)
	case "item-sell":
		g.sendItem(protocol.Request{Kind: "sell", Item: c.Value})
	case "item-buy":
		g.sendItem(protocol.Request{Kind: "buy", Item: c.Value})
	case "item-pickup":
		g.sendItem(protocol.Request{Kind: "pickup", Item: c.Value})
	case "item-drop":
		v := g.displayInventory()
		g.sendItem(protocol.Request{Kind: "drop_item", Item: c.Value, Revision: v.Revision})
		g.u.itemMenu = nil
	case "trade-start":
		if _, _, ok := g.currentTrade(); ok {
			g.openWindow("trade")
			return true
		}
		g.u.tradeItems = nil
		g.sendItem(protocol.Request{Kind: "trade_offer", Peer: c.Value})
		g.openWindow("trade")
	case "trade-toggle":
		if i := slices.Index(g.u.tradeItems, c.Value); i >= 0 {
			g.u.tradeItems = slices.Delete(g.u.tradeItems, i, i+1)
		} else {
			g.u.tradeItems = append(g.u.tradeItems, c.Value)
		}
	case "trade-update":
		g.sendItem(protocol.Request{Kind: "trade_offer", Node: c.Value, Items: slices.Clone(g.u.tradeItems)})
	case "trade-accept":
		if v, _, ok := g.currentTrade(); ok {
			g.sendItem(protocol.Request{Kind: "trade_accept", Node: v.ID, Revision: v.Revision})
		}
	case "trade-cancel":
		g.sendItem(protocol.Request{Kind: "trade_cancel", Node: c.Value})
		g.closeWindow("trade")
	case "item-use":
		slot, _ := strconv.Atoi(c.Value)
		g.sendItem(protocol.Request{Kind: "use_item", Slot: slot, Node: g.u.selectedNode})
	case "weapon-skill-use":
		g.sendItem(protocol.Request{Kind: "weapon_skill", Input: model.Input{Aim: g.localInput.Aim}})
	case "revive":
		g.sendItem(protocol.Request{Kind: "revive", Peer: c.Value})
	default:
		return false
	}
	return true
}

func (g *Game) buildLoot(f *uiFrame, layout ui.Layout) {
	r := g.windowPanel(f, layout, "loot", g.tr("ui.loot"), ui.Size{W: 510, H: 320})
	p := g.self()
	if p == nil {
		return
	}
	var drops []model.Loot
	for _, list := range g.lootLists() {
		for _, it := range list {
			if it.Node == p.Node {
				drops = append(drops, it)
			}
		}
	}
	g.addLabel(f, "loot", ui.Rect{X: r.X + 14, Y: r.Y + 36, W: r.W - 28, H: 40}, g.tr("ui.loot.help"))
	start := min(g.u.scroll["loot"]*8, max(0, len(drops)-32))
	for i, it := range drops[start:min(start+32, len(drops))] {
		f.Controls = append(f.Controls, g.itemControl("loot", "loot:"+it.Item.ID, it.Item, ui.Rect{X: r.X + 14 + float64(i%8)*60, Y: r.Y + 82 + float64(i/8)*50, W: 52, H: 42}, "item-pickup"))
	}
}
func (g *Game) buildShop(f *uiFrame, layout ui.Layout) {
	r := g.windowPanel(f, layout, "shop", g.tr("ui.shop"), ui.Size{W: 510, H: 340})
	p := g.self()
	if p == nil {
		return
	}
	d := g.catalog.Node(g.currentNode().Definition)
	g.addLabel(f, "shop", ui.Rect{X: r.X + 14, Y: r.Y + 38, W: r.W - 28, H: 25}, g.tr("ui.shop.balance", "coins", p.Coins))
	start := min(g.u.scroll["shop"]*8, max(0, len(d.Shop)-24))
	for i, offer := range d.Shop[start:min(start+24, len(d.Shop))] {
		c := g.itemControl("shop", "buy:"+offer.Item, model.ItemInstance{ID: offer.Item, Definition: offer.Item}, ui.Rect{X: r.X + 14 + float64(i%8)*60, Y: r.Y + 74 + float64(i/8)*50, W: 52, H: 42}, "item-buy")
		c.Badge = fmt.Sprint(offer.Price)
		f.Controls = append(f.Controls, c)
	}
	addControl(f, "shop", "shop:upgrade", "weapon", "", "shop-upgrade", "", g.tr("ui.upgrade"), ui.Rect{X: r.X + 14, Y: r.Y + r.H - 44, W: 38, H: 32})
	g.addLabel(f, "shop", ui.Rect{X: r.X + 60, Y: r.Y + r.H - 44, W: r.W - 75, H: 30}, g.tr("ui.shop.upgrade"))
}

func (g *Game) actionKeys(aim model.Vec, captured bool) {
	key := g.bound("weapon_skill")
	if bindingAvailable(key, captured) && bindingPressed(key) {
		if p := g.self(); p != nil {
			if d, it, ok := g.catalog.EquippedWeaponSkill(*p); ok && it.SkillReady <= p.Statuses.Time && p.Energy >= d.Cost {
				g.sendItem(protocol.Request{Kind: "weapon_skill", Input: model.Input{Aim: aim}})
			}
		}
	}
	for i := 0; i < 4; i++ {
		key := g.bound(fmt.Sprintf("item%d", i+1))
		if bindingAvailable(key, captured) && bindingPressed(key) {
			g.sendItem(protocol.Request{Kind: "use_item", Slot: i, Node: g.u.selectedNode, Input: model.Input{Aim: aim}})
		}
	}
}
