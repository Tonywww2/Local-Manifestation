package inventory

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"slices"
)

// QuickTransfer projects Shift-click without a cursor round trip. Fixed bag slots
// stay in place; failure leaves the complete supplied inventory untouched.
func QuickTransfer(v model.Inventory, s Slot, resolve Resolve) (model.Inventory, error) {
	n := layout(v)
	id, err := slotID(n, s)
	if err != nil {
		return model.Inventory{}, err
	}
	if n.Cursor != "" || id == "" {
		return n, nil
	}
	if s.Area != "bag" {
		detach(&n, id)
		if _, err := placeBag(&n, id, resolve); err != nil {
			return model.Inventory{}, err
		}
		if err := finish(&n, resolve); err != nil {
			return model.Inventory{}, err
		}
		return n, nil
	}
	it, _ := Find(n, id)
	d, ok := resolve(it.Definition)
	if !ok {
		return model.Inventory{}, fmt.Errorf("unknown item")
	}
	switch d.Kind {
	case "weapon":
		return Equip(n, id, 0, resolve)
	case "armor":
		return Equip(n, id, slices.Index(Slots[:], d.Slot), resolve)
	case "accessory":
		return Equip(n, id, 9+len(n.Accessories), resolve)
	case "consumable":
		left := Count(it)
		for i := 5; i < 9; i++ {
			other, ok := Find(n, n.Equipment[i])
			if !ok || !CanStack(it, other, resolve) {
				continue
			}
			amount := min(left, StackLimit(d)-Count(other))
			if amount <= 0 {
				continue
			}
			setCount(&n, other.ID, Count(other)+amount)
			left -= amount
			if left == 0 {
				break
			}
		}
		if left == 0 {
			detach(&n, id)
			erase(&n, id)
		} else {
			setCount(&n, id, left)
			for i := 5; i < 9; i++ {
				if n.Equipment[i] == "" {
					n.BagSlots[s.Index] = ""
					n.Equipment[i] = id
					left = 0
					break
				}
			}
			if left == Count(it) {
				return n, nil
			}
		}
		if err := finish(&n, resolve); err != nil {
			return model.Inventory{}, err
		}
		return n, nil
	default:
		return n, nil
	}
}
