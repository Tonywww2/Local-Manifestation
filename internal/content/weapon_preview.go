package content

import (
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
)

// PreviewWeaponAttack projects the candidate's equipment, affix and upgrade onto
// the current player. Both compact damage descriptions and formulas use it.
func (c *Catalog) PreviewWeaponAttack(p model.Player, item model.ItemInstance) (damage.Request, error) {
	next, err := c.PreviewEquipment(p, item, 0)
	if err != nil {
		return damage.Request{}, err
	}
	return Attack(c, next)
}
