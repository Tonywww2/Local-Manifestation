package model

// ItemDef is immutable content; ItemInstance is owned by exactly one inventory.
// CombatEffect is declarative content. Conditions are evaluated once from the
// pre-hit snapshot; captured contributions never retain mutable world pointers.
type CombatEffect struct {
	Kind      string  `json:"kind"`
	Condition string  `json:"condition,omitempty"`
	Group     string  `json:"group,omitempty"`
	Type      string  `json:"type,omitempty"`
	Status    string  `json:"status,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
}
type CombatContribution struct {
	CombatEffect
	Source SourceRef
}
type ItemDef struct {
	Tier            int                `json:"tier,omitempty"`
	Lore            string             `json:"lore,omitempty"`
	Combat          []CombatEffect     `json:"combat,omitempty"`
	Affixes         []string           `json:"affixes,omitempty"`
	Resistance      map[string]float64 `json:"resistance,omitempty"`
	Reductions      []float64          `json:"reductions,omitempty"`
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Kind            string             `json:"kind"`
	Weapon          string             `json:"weapon,omitempty"`
	Slot            string             `json:"slot,omitempty"`
	Group           string             `json:"group,omitempty"`
	Value           int                `json:"value"`
	StackLimit      int                `json:"stack_limit,omitempty"` // Omitted: consumables 30, equipment 1.
	Modifiers       []StatModifier     `json:"modifiers,omitempty"`
	DamageModifiers []Modifier         `json:"damage_modifiers,omitempty"`
	Use             []ItemEffect       `json:"use,omitempty"`
}
type ItemEffect struct {
	Scaling Attributes `json:"scaling,omitempty"`
	Kind    string     `json:"kind"`
	Status  string     `json:"status,omitempty"`
	Amount  float64    `json:"amount,omitempty"`
}
type ItemInstance struct {
	ID          string  `json:"id"`
	Definition  string  `json:"definition"`
	Upgrade     int     `json:"upgrade,omitempty"`
	WeaponBonus float64 `json:"weapon_bonus,omitempty"`
	Affix       string  `json:"affix,omitempty"`
	Count       int     `json:"count,omitempty"` // Omitted legacy/single-item values mean one.
}
type Inventory struct {
	Items          []ItemInstance `json:"items,omitempty"`
	BagSlots       []string       `json:"bag_slots,omitempty"`
	Cursor         string         `json:"cursor,omitempty"`
	Equipment      [9]string      `json:"equipment"`
	Accessories    []string       `json:"accessories,omitempty"`
	Capacity       int            `json:"capacity"`
	AccessorySlots int            `json:"accessory_slots"` // -1 explicitly permits dynamic expansion.
	NextID         uint64         `json:"next_id"`
	Revision       uint64         `json:"revision"`
}

func CloneInventory(v Inventory) Inventory {
	v.Items = append([]ItemInstance(nil), v.Items...)
	v.BagSlots = append([]string(nil), v.BagSlots...)
	v.Accessories = append([]string(nil), v.Accessories...)
	return v
}
func CloneItemDef(v ItemDef) ItemDef {
	v.Affixes = append([]string(nil), v.Affixes...)
	if v.Resistance != nil {
		m := map[string]float64{}
		for k, n := range v.Resistance {
			m[k] = n
		}
		v.Resistance = m
	}
	v.Reductions = append([]float64(nil), v.Reductions...)
	v.Modifiers = append([]StatModifier(nil), v.Modifiers...)
	v.DamageModifiers = append([]Modifier(nil), v.DamageModifiers...)
	v.Use = append([]ItemEffect(nil), v.Use...)
	v.Combat = append([]CombatEffect(nil), v.Combat...)
	return v
}

type AffixDef struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Value           int            `json:"value"`
	Modifiers       []StatModifier `json:"modifiers,omitempty"`
	DamageModifiers []Modifier     `json:"damage_modifiers,omitempty"`
}
type Loot struct {
	Item     ItemInstance `json:"item"`
	Node     string       `json:"node"`
	Position Vec          `json:"position"`
}
type DropDef struct {
	Item   string  `json:"item"`
	Chance float64 `json:"chance"`
}
type ShopEntry struct {
	Item  string `json:"item"`
	Price int    `json:"price"`
}
