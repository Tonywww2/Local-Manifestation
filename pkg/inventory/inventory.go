// Package inventory precomputes detached transactions without I/O or world access.
package inventory

import (
	"errors"
	"fmt"
	"hsmultiplayer/pkg/model"
	"math"
	"slices"
)

var ErrFull = errors.New("backpack full")

type Resolve func(string) (model.ItemDef, bool)

var Slots = [9]string{"weapon", "head", "body", "legs", "feet", "consumable", "consumable", "consumable", "consumable"}

func Equipped(v model.Inventory, id string) bool {
	return id != "" && (slices.Contains(v.Equipment[:], id) || slices.Contains(v.Accessories, id))
}
func Bag(v model.Inventory) []model.ItemInstance {
	var out []model.ItemInstance
	if v.BagSlots != nil {
		for _, id := range v.BagSlots {
			if it, ok := Find(v, id); ok {
				out = append(out, it)
			}
		}
		return out
	}
	for _, it := range v.Items {
		if !Equipped(v, it.ID) && it.ID != v.Cursor {
			out = append(out, it)
		}
	}
	return out
}
func Find(v model.Inventory, id string) (model.ItemInstance, bool) {
	for _, it := range v.Items {
		if it.ID == id {
			return it, true
		}
	}
	return model.ItemInstance{}, false
}
func Validate(v model.Inventory, resolve Resolve) error {
	if err := validate(v, resolve, true); err != nil {
		return err
	}
	if v.Cursor != "" {
		trial := layout(v)
		id := trial.Cursor
		trial.Cursor = ""
		if _, err := placeBag(&trial, id, resolve); err != nil {
			return err
		}
	}
	return nil
}
func validate(v model.Inventory, resolve Resolve, capacity bool) error {
	maximumCapacity := 4096
	if !capacity {
		maximumCapacity++
	}
	if v.Capacity < 1 || v.Capacity > maximumCapacity || v.AccessorySlots < -1 || v.AccessorySlots > 4096 || len(v.Items) > 8192 || v.AccessorySlots >= 0 && len(v.Accessories) > v.AccessorySlots {
		return fmt.Errorf("invalid inventory limits")
	}
	ids := map[string]model.ItemDef{}
	for _, it := range v.Items {
		d, ok := resolve(it.Definition)
		if !ok || it.ID == "" || ids[it.ID].ID != "" || it.Upgrade < 0 || it.Upgrade > 100000 || math.IsNaN(it.WeaponBonus) || math.IsInf(it.WeaponBonus, 0) || it.WeaponBonus < 0 || it.WeaponBonus > 1e9 || it.SkillReady < 0 {
			return fmt.Errorf("invalid item instance")
		}
		if Count(it) < 1 || Count(it) > StackLimit(d) {
			return fmt.Errorf("invalid stack count")
		}
		if d.Kind == "movement" || d.Kind == "passive" {
			if d.Durability == -1 && it.Durability != -1 || d.Durability > 0 && (it.Durability < 1 || it.Durability > d.Durability) {
				return fmt.Errorf("invalid item durability")
			}
		} else if it.Durability != 0 {
			return fmt.Errorf("unexpected durability")
		}
		if d.Kind != "weapon" && (it.Upgrade != 0 || it.WeaponBonus != 0 || it.SkillReady != 0) {
			return fmt.Errorf("non-weapon upgrade")
		}
		if it.Affix != "" && !slices.Contains(d.Affixes, it.Affix) {
			return fmt.Errorf("invalid item affix")
		}
		ids[it.ID] = d
	}
	seen := map[string]bool{}
	for i, id := range v.Equipment {
		if id == "" {
			continue
		}
		d := ids[id]
		slot := d.Kind
		if d.Kind == "armor" {
			slot = d.Slot
		}
		if seen[id] || d.ID == "" || slot != Slots[i] {
			return fmt.Errorf("invalid equipment slot")
		}
		seen[id] = true
	}
	groups := map[string]bool{}
	for _, id := range v.Accessories {
		d := ids[id]
		if seen[id] || d.Kind != "accessory" || d.Group != "" && groups[d.Group] {
			return fmt.Errorf("conflicting accessory")
		}
		seen[id] = true
		if d.Group != "" {
			groups[d.Group] = true
		}
	}
	if v.BagSlots != nil {
		if len(v.BagSlots) != v.Capacity {
			return fmt.Errorf("invalid backpack layout")
		}
		for _, id := range v.BagSlots {
			if id == "" {
				continue
			}
			if seen[id] || ids[id].ID == "" {
				return fmt.Errorf("invalid backpack slot")
			}
			seen[id] = true
		}
		if v.Cursor != "" {
			if seen[v.Cursor] || ids[v.Cursor].ID == "" {
				return fmt.Errorf("invalid cursor item")
			}
			seen[v.Cursor] = true
		}
		if len(seen) != len(v.Items) {
			return fmt.Errorf("unplaced item")
		}
	} else if capacity && len(v.Items)-len(seen) > v.Capacity {
		return ErrFull
	}
	return nil
}
func Grant(v model.Inventory, owner, definition string, resolve Resolve) (model.Inventory, model.ItemInstance, error) {
	n := layout(v)
	n.NextID++
	d, _ := resolve(definition)
	it := NewInstance(fmt.Sprintf("%s:%d", owner, n.NextID), d)
	return Insert(n, it, resolve)
}

// Equip swaps the slot atomically, including the bag capacity of the displaced item.
func Equip(v model.Inventory, id string, slot int, resolve Resolve) (model.Inventory, error) {
	return equip(v, id, slot, resolve, true)
}

// PreviewEquip validates definitions, slots and conflicts exactly like Equip,
// but inventory capacity cannot affect a hypothetical comparison.
func PreviewEquip(v model.Inventory, item model.ItemInstance, slot int, resolve Resolve) (model.Inventory, error) {
	v = layout(v)
	// Hypothetical capacity must not affect stat comparisons.
	v.Capacity++
	v.BagSlots = append(v.BagSlots, "")
	if _, owned := Find(v, item.ID); !owned {
		v.Items = append(v.Items, item)
		v.BagSlots[len(v.BagSlots)-1] = item.ID
	}
	return equip(v, item.ID, slot, resolve, false)
}

func equip(v model.Inventory, id string, slot int, resolve Resolve, capacity bool) (model.Inventory, error) {
	if slot < 0 || slot > 4096 {
		return model.Inventory{}, fmt.Errorf("invalid slot")
	}
	if slot < 9 && v.Equipment[slot] == id {
		return model.CloneInventory(v), nil
	}
	if slot >= 9 && slot-9 < len(v.Accessories) && v.Accessories[slot-9] == id {
		return model.CloneInventory(v), nil
	}
	if id != "" {
		if _, ok := Find(v, id); !ok {
			return model.Inventory{}, fmt.Errorf("item not owned")
		}
	}
	n := layout(v)
	sourceSlot := slices.Index(n.BagSlots, id)
	oldAccessory := slices.Index(n.Accessories, id)
	detach(&n, id)
	displaced := ""
	if slot < 9 {
		displaced = n.Equipment[slot]
		n.Equipment[slot] = id
	} else {
		i := slot - 9
		if id != "" && oldAccessory >= 0 && oldAccessory < i {
			i--
		}
		if i > len(n.Accessories) {
			return model.Inventory{}, fmt.Errorf("invalid accessory slot")
		}
		if i == len(n.Accessories) {
			if id != "" {
				n.Accessories = append(n.Accessories, id)
			}
		} else if id == "" {
			displaced = n.Accessories[i]
			n.Accessories = slices.Delete(n.Accessories, i, i+1)
		} else {
			displaced = n.Accessories[i]
			n.Accessories[i] = id
		}
	}
	if displaced != "" {
		if sourceSlot >= 0 && n.BagSlots[sourceSlot] == "" {
			n.BagSlots[sourceSlot] = displaced
		} else if _, err := placeBag(&n, displaced, resolve); err != nil {
			return model.Inventory{}, err
		}
	}
	if err := validate(n, resolve, capacity); err != nil {
		return model.Inventory{}, err
	}
	if !capacity {
		n.Revision++
		return n, nil
	}
	if err := finish(&n, resolve); err != nil {
		return model.Inventory{}, err
	}
	return n, nil
}
func Remove(v model.Inventory, id string, resolve Resolve) (model.Inventory, model.ItemInstance, error) {
	it, ok := Find(v, id)
	if !ok {
		return model.Inventory{}, it, fmt.Errorf("item not owned")
	}
	n := layout(v)
	detach(&n, id)
	erase(&n, id)
	if err := finish(&n, resolve); err != nil {
		return model.Inventory{}, it, err
	}
	return n, it, nil
}
func Exchange(a, b model.Inventory, offerA, offerB []string, resolve Resolve) (model.Inventory, model.Inventory, error) {
	x, y := model.CloneInventory(a), model.CloneInventory(b)
	var incomingA, incomingB []model.ItemInstance
	seen := map[string]bool{}
	for _, side := range []struct {
		v   *model.Inventory
		ids []string
		out *[]model.ItemInstance
	}{{&x, offerA, &incomingB}, {&y, offerB, &incomingA}} {
		for _, id := range side.ids {
			if seen[id] || Equipped(*side.v, id) || side.v.Cursor == id {
				return model.Inventory{}, model.Inventory{}, fmt.Errorf("trade requires distinct unequipped items")
			}
			seen[id] = true
			n, it, err := Remove(*side.v, id, resolve)
			if err != nil {
				return model.Inventory{}, model.Inventory{}, err
			}
			*side.v = n
			*side.out = append(*side.out, it)
		}
	}
	for _, side := range []struct {
		v     *model.Inventory
		items []model.ItemInstance
	}{{&x, incomingA}, {&y, incomingB}} {
		for _, it := range side.items {
			n, _, err := Insert(*side.v, it, resolve)
			if err != nil {
				return model.Inventory{}, model.Inventory{}, err
			}
			*side.v = n
		}
	}
	return x, y, nil
}
