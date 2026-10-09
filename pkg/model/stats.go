package model

// SourceRef is identity, not a display label or an implicit stacking policy.
type SourceRef struct {
	Kind       string `json:"kind"`
	Actor      string `json:"actor,omitempty"`
	Instance   string `json:"instance,omitempty"`
	Definition string `json:"definition"`
}

type StatValues struct {
	AttackSpeed float64    `json:"attack_speed"`
	Attributes  Attributes `json:"attributes"`
	MaxHP       float64    `json:"max_hp"`
	Armor       float64    `json:"armor"`
	Speed       float64    `json:"speed"`
}

// StatModifier is content data; StatContribution binds it to a runtime source.
type StatModifier struct {
	ID        string  `json:"id"`
	Stat      string  `json:"stat"`
	Operation string  `json:"operation"`
	Group     string  `json:"group,omitempty"`
	Value     float64 `json:"value"`
}
type StatContribution struct {
	StatModifier
	Source SourceRef `json:"source"`
}

func (a Attributes) Value(key string) (float64, bool) {
	switch key {
	case "strength":
		return a.Strength, true
	case "agility":
		return a.Agility, true
	case "constitution":
		return a.Constitution, true
	case "intelligence":
		return a.Intelligence, true
	case "sense":
		return a.Sense, true
	case "mana":
		return a.Mana, true
	}
	return 0, false
}
func (a Attributes) With(key string, value float64) (Attributes, bool) {
	switch key {
	case "strength":
		a.Strength = value
	case "agility":
		a.Agility = value
	case "constitution":
		a.Constitution = value
	case "intelligence":
		a.Intelligence = value
	case "sense":
		a.Sense = value
	case "mana":
		a.Mana = value
	default:
		return a, false
	}
	return a, true
}

func ClonePlayer(p Player) Player {
	if p.MetaRanks != nil {
		m := map[string]int{}
		for k, v := range p.MetaRanks {
			m[k] = v
		}
		p.MetaRanks = m
	}
	p.Loot = append([]Loot(nil), p.Loot...)
	p.Inventory = CloneInventory(p.Inventory)
	p.Statuses = CloneStatuses(p.Statuses)
	p.StatContributions = append([]StatContribution(nil), p.StatContributions...)
	return p
}

func CloneWeapon(w Weapon) Weapon {
	w.OnHitStatuses = append([]string(nil), w.OnHitStatuses...)
	w.Modifiers = append([]Modifier(nil), w.Modifiers...)
	w.AdditionalDamage = append([]DamagePart(nil), w.AdditionalDamage...)
	for i := range w.AdditionalDamage {
		w.AdditionalDamage[i].Modifiers = append([]Modifier(nil), w.AdditionalDamage[i].Modifiers...)
		w.AdditionalDamage[i].Bonuses = append([]DamageBonus(nil), w.AdditionalDamage[i].Bonuses...)
	}
	return w
}

func CloneEnemy(e Enemy) Enemy { e.Statuses = CloneStatuses(e.Statuses); return e }
