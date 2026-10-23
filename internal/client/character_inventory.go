package client

import (
	"fmt"
	"strconv"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/progression"
)

type inventoryStatusCard struct {
	Coins, Level, Experience string
	Fraction                 float64
}

func (g *Game) buildCharacterInventory(f *uiFrame, layout ui.Layout) {
	const id = "backpack"
	p := g.self()
	if p == nil {
		return
	}
	r := g.windowPanel(f, layout, id, g.tr("ui.backpack_status"), ui.Size{W: 980, H: 620})
	q := ui.CharacterInventoryAt(r)
	v := g.displayInventory()
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 42, W: q.Consumables.W, H: 24}, p.Name+" · "+g.tr("ui.level.short", "level", p.Level))
	chassisName := p.Chassis
	if chassis, ok := g.catalog.Chassis(p.Chassis); ok {
		chassisName = g.contentName(chassis.ID, chassis.Name)
	}
	f.Controls = append(f.Controls, uiControl{Owner: id, ID: "character:portrait", Icon: p.Chassis, Caption: chassisName, Title: chassisName, Bounds: q.Character,
		Summary: []string{g.tr("life." + p.Life), fmt.Sprintf("%.2f / %.2f", p.HP, p.MaxHP)}})
	for i, bounds := range q.Equipment {
		g.addCharacterEquipment(f, v, i, bounds)
	}
	g.addLabel(f, id, ui.Rect{X: q.Consumables.X, Y: q.Consumables.Y - 28, W: q.Consumables.W, H: 22}, g.tr("ui.consumables"))
	for i := 0; i < 4; i++ {
		g.addCharacterEquipment(f, v, 5+i, ui.GridCell(q.Consumables, i, 4, 54))
	}
	g.addLabel(f, id, ui.Rect{X: q.WeaponSkill.X, Y: q.WeaponSkill.Y - 28, W: q.Consumables.W, H: 22}, g.tr("ui.weapon_skill"))
	f.Controls = append(f.Controls, g.weaponSkillControl(*p, id, "configured:weapon_skill", q.WeaponSkill))
	g.buildCharacterAccessories(f, v, q.Accessories)
	g.buildCharacterBag(f, v, q.Bag)
	if it, ok := inventory.Find(v, g.u.selectedItem); ok {
		d, _ := g.catalog.Item(it.Definition)
		g.addLabel(f, id, ui.Rect{X: q.Bag.X, Y: r.Y + 476, W: q.Bag.W - 94, H: 40}, g.contentName(d.ID, d.Name))
		if d.Kind == "movement" {
			addControl(f, id, "item:movement", "movement", "", "movement-select", it.ID, g.tr("ui.movement.select"), ui.Rect{X: q.Bag.X + q.Bag.W - 86, Y: r.Y + 478, W: 34, H: 30})
		} else if d.Kind != "passive" {
			addControl(f, id, "item:equip", "approve", "", "item-equip", it.ID, g.tr("ui.item.equip"), ui.Rect{X: q.Bag.X + q.Bag.W - 86, Y: r.Y + 478, W: 34, H: 30})
		}
		addControl(f, id, "item:sell", "coins", "", "item-sell", it.ID, g.tr("ui.item.sell"), ui.Rect{X: q.Bag.X + q.Bag.W - 44, Y: r.Y + 478, W: 34, H: 30})
	}
	// This window has its own ID and lifetime, independent of the backpack.
	addControl(f, id, "backpack:attributes", "person", "", "window", "self", g.tr("ui.character.attributes"), q.AttributeButton)
	f.Controls[len(f.Controls)-1].Caption = g.tr("ui.character.attributes")
	f.Controls[len(f.Controls)-1].Summary = []string{g.tr("ui.character.attributes.help"), g.tr("ui.growth.base")}
	addControl(f, id, "backpack:loot", "status", "", "window", "loot", g.tr("ui.loot"), q.LootButton)
	g.addLabel(f, id, q.Help, g.tr("ui.inventory.help.compact"))
}

func (g *Game) addCharacterEquipment(f *uiFrame, v model.Inventory, index int, r ui.Rect) {
	it, _ := inventory.Find(v, v.Equipment[index])
	c := g.itemControl("backpack", "equipped:"+strconv.Itoa(index), it, r, "inventory-equipment")
	c.Value = strconv.Itoa(index)
	c.Slot = inventory.Slots[index]
	if index >= 5 {
		c.Key = g.bindingLabel(g.bound(fmt.Sprintf("item%d", index-4)))
	}
	if it.ID == "" {
		c.Title = g.tr("ui.slot." + inventory.Slots[index])
		c.Icon = []string{"weapon", "head", "body", "legs", "feet", "empty", "empty", "empty", "empty"}[index]
		c.Summary = []string{g.tr("ui.slot.empty")}
	}
	f.Controls = append(f.Controls, c)
}

func (g *Game) inventoryPager(f *uiFrame, key string, count, pageSize int, r ui.Rect) int {
	pages := max(1, (count+pageSize-1)/pageSize)
	page := min(max(0, g.u.scroll[key]), pages-1)
	g.u.scroll[key] = page
	if pages > 1 {
		for i, direction := range []string{"-1", "1"} {
			addControl(f, "backpack", key+":"+direction, []string{"scroll-up", "scroll-down"}[i], "", "inventory-page", key+":"+direction, g.tr("ui.inventory.page", "page", page+1, "pages", pages), ui.Rect{X: r.X + r.W - 62 + float64(i)*30, Y: r.Y - 30, W: 24, H: 24})
			f.Controls[len(f.Controls)-1].Count = pages
		}
	}
	return page * pageSize
}

func (g *Game) buildCharacterBag(f *uiFrame, v model.Inventory, r ui.Rect) {
	g.addLabel(f, "backpack", ui.Rect{X: r.X, Y: r.Y - 32, W: r.W - 70, H: 24}, g.tr("ui.inventory.bag", "used", inventory.BagCount(v), "capacity", v.Capacity))
	start := g.inventoryPager(f, "bag-pages", v.Capacity, 27, r)
	bag := inventory.SlotRange(v, start, 27)
	for i := 0; i < min(27, v.Capacity-start); i++ {
		it := model.ItemInstance{}
		if i < len(bag) {
			it = bag[i]
		}
		c := g.itemControl("backpack", fmt.Sprintf("bag:%d", start+i), it, ui.GridCell(r, i, 9, 54), "inventory-bag")
		c.Value = strconv.Itoa(start + i)
		if it.ID == "" {
			c.Title = g.tr("ui.inventory.empty_slot", "slot", start+i+1)
		}
		f.Controls = append(f.Controls, c)
	}
}

func (g *Game) buildCharacterAccessories(f *uiFrame, v model.Inventory, r ui.Rect) {
	capacity := fmt.Sprint(v.AccessorySlots)
	count := v.AccessorySlots
	if count < 0 {
		capacity, count = "∞", len(v.Accessories)+1
	}
	g.addLabel(f, "backpack", ui.Rect{X: r.X, Y: r.Y - 32, W: r.W - 70, H: 24}, g.tr("ui.inventory.accessories.compact", "used", len(v.Accessories), "capacity", capacity))
	start := g.inventoryPager(f, "accessory-pages", count, 12, r)
	for i := 0; i < min(12, count-start); i++ {
		it := model.ItemInstance{}
		if start+i < len(v.Accessories) {
			it, _ = inventory.Find(v, v.Accessories[start+i])
		}
		c := g.itemControl("backpack", fmt.Sprintf("accessory:%d", start+i), it, ui.GridCell(r, i, 6, 50), "inventory-accessory")
		// Accessories already use a compact equipped list. Every unoccupied
		// display slot targets its append position, without changing bag slots.
		c.Value = strconv.Itoa(min(start+i, len(v.Accessories)))
		c.Slot = "accessory"
		if it.ID == "" {
			c.Title = g.tr("ui.slot.accessory")
			c.Summary = []string{g.tr("ui.slot.empty")}
		}
		f.Controls = append(f.Controls, c)
	}
}

func (g *Game) inventoryStatusControl(r ui.Rect, p model.Player) uiControl {
	w := g.catalog.Weapon(p.Weapon)
	next := progression.NextLevelXP(p.Level)
	c := uiControl{Owner: "equipment", ID: "inventory-status", Icon: "weapon", Action: "window", Value: "backpack", Title: g.tr("ui.backpack_status"), Caption: g.contentName(w.ID, w.Name), Bounds: r,
		InventoryStatus: &inventoryStatusCard{Coins: fmt.Sprint(p.Coins), Level: g.tr("ui.level.short", "level", p.Level), Experience: fmt.Sprintf("%d / %d", p.XP, next), Fraction: float64(p.XP) / float64(next)},
		Summary:         []string{g.contentName(w.ID, w.Name), g.tr("ui.shop.balance", "coins", p.Coins), g.tr("ui.level.short", "level", p.Level), g.tr("ui.xp.progress", "xp", p.XP, "next", next)}}
	if it, ok := inventory.Find(p.Inventory, p.Inventory.Equipment[0]); ok {
		item := g.itemControl("equipment", c.ID, it, r, "window")
		c.Icon = item.Icon
		c.Caption = item.Title
	}
	return c
}
