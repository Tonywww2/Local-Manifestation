package inventory

import (
	"fmt"
	"hsmultiplayer/pkg/model"
)

func NewInstance(id string, d model.ItemDef) model.ItemInstance {
	return model.ItemInstance{ID: id, Definition: d.ID, Count: 1, Durability: d.Durability}
}

func InBag(v model.Inventory, id string) bool {
	if id == "" || Equipped(v, id) || v.Cursor == id {
		return false
	}
	if _, ok := Find(v, id); !ok {
		return false
	}
	if v.BagSlots == nil {
		return true
	}
	for _, slot := range v.BagSlots {
		if slot == id {
			return true
		}
	}
	return false
}

// SpendDurability is a detached transaction. The final use removes the item;
// unlimited passive consumables keep their identity and inventory revision.
func SpendDurability(v model.Inventory, id string, resolve Resolve) (model.Inventory, error) {
	if err := Validate(v, resolve); err != nil {
		return model.Inventory{}, err
	}
	it, ok := Find(v, id)
	d, known := resolve(it.Definition)
	if !ok || !known || (d.Kind != "movement" && d.Kind != "passive") || it.Durability == 0 {
		return model.Inventory{}, fmt.Errorf("invalid durable consumable")
	}
	if it.Durability == -1 {
		return model.CloneInventory(v), nil
	}
	if it.Durability == 1 {
		n, _, err := Remove(v, id, resolve)
		return n, err
	}
	n := model.CloneInventory(v)
	for i := range n.Items {
		if n.Items[i].ID == id {
			n.Items[i].Durability--
		}
	}
	if err := finish(&n, resolve); err != nil {
		return model.Inventory{}, err
	}
	return n, nil
}
