package content

import (
	"fmt"
	"hsmultiplayer/pkg/model"
)

func (c *Catalog) validateSpecialConsumable(d model.ItemDef) error {
	special := d.Kind == "movement" || d.Kind == "passive"
	if !special {
		if d.Durability != 0 || d.Movement != nil || d.Passive != nil {
			return fmt.Errorf("unexpected special consumable components")
		}
		return nil
	}
	if d.Durability != -1 && (d.Durability < 1 || d.Durability > 1000000) || d.StackLimit > 1 || len(d.Use) > 0 {
		return fmt.Errorf("invalid special consumable durability or stack")
	}
	if d.Kind == "movement" {
		m := d.Movement
		if m == nil || d.Passive != nil || d.Durability < 1 || m.Mode != "line" && m.Mode != "ring" && m.Mode != "anywhere" || m.Mode == "anywhere" && m.Range != 0 || m.Mode != "anywhere" && (m.Range < 1 || m.Range > 1000) {
			return fmt.Errorf("invalid movement component")
		}
	} else {
		p := d.Passive
		if p == nil || d.Movement != nil || p.Trigger != "lethal_damage" && p.Trigger != "movement" && p.Trigger != "movement_combat" || p.Status == "" && !p.Accessory {
			return fmt.Errorf("invalid passive component")
		}
		if p.Status != "" {
			if _, ok := c.statuses[p.Status]; !ok {
				return fmt.Errorf("unknown passive status")
			}
		}
	}
	return nil
}
