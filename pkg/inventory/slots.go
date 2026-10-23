package inventory

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"slices"
)

// Count accepts old single-item values without changing their serialized identity.
func Count(it model.ItemInstance) int {
	if it.Count == 0 {
		return 1
	}
	return it.Count
}
func StackLimit(d model.ItemDef) int {
	if d.Kind != "consumable" {
		return 1
	}
	if d.StackLimit == 0 {
		return 30
	}
	return d.StackLimit
}

// CanStack compares every currently supported item component, except identity/count.
func CanStack(a, b model.ItemInstance, resolve Resolve) bool {
	d, ok := resolve(a.Definition)
	return ok && StackLimit(d) > 1 && a.Definition == b.Definition && a.Affix == b.Affix && a.Upgrade == b.Upgrade && a.WeaponBonus == b.WeaponBonus && a.SkillReady == b.SkillReady && a.Durability == b.Durability
}

// layout migrates legacy lists once, in their existing order. Explicit slots are
// never compacted, sorted, or filled by this operation.
func layout(v model.Inventory) model.Inventory {
	v = model.CloneInventory(v)
	if v.BagSlots == nil && v.Capacity > 0 && v.Capacity <= 4096 {
		v.BagSlots = make([]string, v.Capacity)
		i := 0
		for _, it := range v.Items {
			if !Equipped(v, it.ID) && it.ID != v.Cursor && i < len(v.BagSlots) {
				v.BagSlots[i] = it.ID
				i++
			}
		}
	}
	return v
}
func SlotItems(v model.Inventory) []model.ItemInstance {
	return SlotRange(v, 0, v.Capacity)
}

// Visible pages do not copy an expanded inventory on every rendered frame.
func SlotRange(v model.Inventory, start, count int) []model.ItemInstance {
	if start < 0 || start >= v.Capacity || count < 1 {
		return nil
	}
	out := make([]model.ItemInstance, min(count, v.Capacity-start))
	if v.BagSlots != nil {
		for i := range out {
			if start+i < len(v.BagSlots) {
				out[i], _ = Find(v, v.BagSlots[start+i])
			}
		}
		return out
	}
	index := 0
	for _, it := range v.Items {
		if Equipped(v, it.ID) || it.ID == v.Cursor {
			continue
		}
		if index >= start && index < start+len(out) {
			out[index-start] = it
		}
		index++
		if index >= start+len(out) {
			break
		}
	}
	return out
}

func BagCount(v model.Inventory) int {
	count := 0
	if v.BagSlots != nil {
		for _, id := range v.BagSlots {
			if id != "" {
				count++
			}
		}
		return count
	}
	for _, it := range v.Items {
		if !Equipped(v, it.ID) && it.ID != v.Cursor {
			count++
		}
	}
	return count
}
func setCount(v *model.Inventory, id string, count int) {
	for i := range v.Items {
		if v.Items[i].ID == id {
			v.Items[i].Count = count
			return
		}
	}
}
func erase(v *model.Inventory, id string) {
	v.Items = slices.DeleteFunc(v.Items, func(it model.ItemInstance) bool { return it.ID == id })
}
func detach(v *model.Inventory, id string) {
	if id == "" {
		return
	}
	for i, x := range v.BagSlots {
		if x == id {
			v.BagSlots[i] = ""
		}
	}
	for i, x := range v.Equipment {
		if x == id {
			v.Equipment[i] = ""
		}
	}
	v.Accessories = slices.DeleteFunc(v.Accessories, func(x string) bool { return x == id })
	if v.Cursor == id {
		v.Cursor = ""
	}
}

// placeBag merges compatible stacks first, then uses empty slots, preserving all
// other positions. The item is already owned but has no location yet.
func placeBag(v *model.Inventory, id string, resolve Resolve) (string, error) {
	it, ok := Find(*v, id)
	if !ok {
		return "", fmt.Errorf("item not owned")
	}
	d, _ := resolve(it.Definition)
	left, result := Count(it), id
	for _, target := range v.BagSlots {
		other, ok := Find(*v, target)
		if !ok || !CanStack(it, other, resolve) {
			continue
		}
		n := min(left, StackLimit(d)-Count(other))
		if n <= 0 {
			continue
		}
		setCount(v, target, Count(other)+n)
		left -= n
		result = target
		if left == 0 {
			erase(v, id)
			return result, nil
		}
	}
	for i, target := range v.BagSlots {
		if target == "" {
			v.BagSlots[i] = id
			setCount(v, id, left)
			return id, nil
		}
	}
	return "", ErrFull
}

// Insert is the common all-or-nothing pickup, reward and trade receiving rule.
func Insert(v model.Inventory, it model.ItemInstance, resolve Resolve) (model.Inventory, model.ItemInstance, error) {
	d, known := resolve(it.Definition)
	if !known || it.ID == "" || Count(it) < 1 || Count(it) > StackLimit(d) {
		return model.Inventory{}, it, fmt.Errorf("invalid incoming stack")
	}
	n := layout(v)
	if _, exists := Find(n, it.ID); exists {
		return model.Inventory{}, it, fmt.Errorf("duplicate item")
	}
	n.Items = append(n.Items, it)
	id, err := placeBag(&n, it.ID, resolve)
	if err == nil {
		err = finish(&n, resolve)
	}
	if err != nil {
		return model.Inventory{}, it, err
	}
	result, _ := Find(n, id)
	return n, result, nil
}
func finish(v *model.Inventory, resolve Resolve) error {
	if err := Validate(*v, resolve); err != nil {
		return err
	}
	v.Revision++
	return nil
}
func ReturnCursor(v model.Inventory, resolve Resolve) (model.Inventory, error) {
	n := layout(v)
	if n.Cursor == "" {
		return n, nil
	}
	id := n.Cursor
	n.Cursor = ""
	if _, err := placeBag(&n, id, resolve); err != nil {
		return model.Inventory{}, err
	}
	if err := finish(&n, resolve); err != nil {
		return model.Inventory{}, err
	}
	return n, nil
}
func Consume(v model.Inventory, id string, resolve Resolve) (model.Inventory, error) {
	it, ok := Find(v, id)
	if !ok {
		return model.Inventory{}, fmt.Errorf("item not owned")
	}
	if Count(it) == 1 {
		n, _, err := Remove(v, id, resolve)
		return n, err
	}
	n := layout(v)
	setCount(&n, id, Count(it)-1)
	if err := finish(&n, resolve); err != nil {
		return model.Inventory{}, err
	}
	return n, nil
}

type Slot struct {
	Area  string
	Index int
}

func slotID(v model.Inventory, s Slot) (string, error) {
	switch s.Area {
	case "bag":
		if s.Index >= 0 && s.Index < len(v.BagSlots) {
			return v.BagSlots[s.Index], nil
		}
	case "equipment":
		if s.Index >= 0 && s.Index < len(v.Equipment) {
			return v.Equipment[s.Index], nil
		}
	case "accessory":
		if s.Index >= 0 && s.Index <= len(v.Accessories) && (v.AccessorySlots < 0 || s.Index < v.AccessorySlots) {
			if s.Index < len(v.Accessories) {
				return v.Accessories[s.Index], nil
			}
			return "", nil
		}
	}
	return "", fmt.Errorf("invalid inventory slot")
}
func assign(v *model.Inventory, s Slot, id string) {
	switch s.Area {
	case "bag":
		v.BagSlots[s.Index] = id
	case "equipment":
		v.Equipment[s.Index] = id
	case "accessory":
		if s.Index == len(v.Accessories) {
			if id != "" {
				v.Accessories = append(v.Accessories, id)
			}
		} else if id == "" {
			v.Accessories = slices.Delete(v.Accessories, s.Index, s.Index+1)
		} else {
			v.Accessories[s.Index] = id
		}
	}
}
func newSplit(v *model.Inventory, owner string, it model.ItemInstance, count int) model.ItemInstance {
	v.NextID++
	it.ID, it.Count = fmt.Sprintf("%s:%d", owner, v.NextID), count
	v.Items = append(v.Items, it)
	return it
}

// Click models one authoritative mouse operation. No partial changes escape on
// errors, including wrong equipment kinds, full bags and conflicting accessories.
func Click(v model.Inventory, owner string, s Slot, right bool, resolve Resolve) (model.Inventory, error) {
	n := layout(v)
	target, err := slotID(n, s)
	if err != nil {
		return model.Inventory{}, err
	}
	held, hasHeld := Find(n, n.Cursor)
	it, occupied := Find(n, target)
	if !hasHeld {
		// Empty-handed right clicks are inspection intents, never item movement.
		if right || !occupied {
			return n, nil
		}
		assign(&n, s, "")
		n.Cursor = it.ID
	} else if !occupied || CanStack(held, it, resolve) {
		d, _ := resolve(held.Definition)
		amount := Count(held)
		if right {
			amount = 1
		}
		if occupied {
			amount = min(amount, StackLimit(d)-Count(it))
		}
		if amount <= 0 {
			return n, nil
		}
		if occupied {
			setCount(&n, it.ID, Count(it)+amount)
		} else if amount == Count(held) {
			assign(&n, s, held.ID)
		} else {
			assign(&n, s, newSplit(&n, owner, held, amount).ID)
		}
		if amount == Count(held) {
			n.Cursor = ""
			if occupied {
				erase(&n, held.ID)
			}
		} else {
			setCount(&n, held.ID, Count(held)-amount)
		}
	} else if !right {
		assign(&n, s, held.ID)
		n.Cursor = it.ID
	}
	if err := finish(&n, resolve); err != nil {
		return model.Inventory{}, err
	}
	return n, nil
}
