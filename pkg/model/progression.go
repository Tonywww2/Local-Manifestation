package model

type PerkDef struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Cost          int            `json:"cost"`
	MaxRank       int            `json:"max_rank"`
	Requires      []string       `json:"requires,omitempty"`
	Modifiers     []StatModifier `json:"modifiers,omitempty"`
	Coins         int            `json:"coins,omitempty"`
	PersonalAP    int            `json:"personal_ap,omitempty"`
	SharedAP      int            `json:"shared_ap,omitempty"`
	StartingItems []string       `json:"starting_items,omitempty"`
	SpecialNodes  []string       `json:"special_nodes,omitempty"`
}
type MetaRules struct {
	Stats     bool `json:"stats"`
	Coins     bool `json:"coins"`
	Equipment bool `json:"equipment"`
	AP        bool `json:"ap"`
	Nodes     bool `json:"nodes"`
	Unlocks   bool `json:"unlocks"`
}
type CompletionRewardDef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Condition string `json:"condition"`
	Points    int    `json:"points"`
}

func DefaultMetaRules() MetaRules { return MetaRules{true, true, true, true, true, true} }

type StoredItem struct {
	Item  ItemInstance `json:"item"`
	RunID string       `json:"run_id"`
}
type Progression struct {
	Points       int            `json:"points"`
	Earned       int            `json:"earned"`
	Ranks        map[string]int `json:"ranks"`
	Warehouse    []StoredItem   `json:"warehouse,omitempty"`
	Settlements  []string       `json:"settlements,omitempty"`
	Achievements []string       `json:"achievements,omitempty"`
}

func CloneProgression(p Progression) Progression {
	m := map[string]int{}
	for k, v := range p.Ranks {
		m[k] = v
	}
	p.Ranks = m
	p.Warehouse = append([]StoredItem(nil), p.Warehouse...)
	p.Settlements = append([]string(nil), p.Settlements...)
	p.Achievements = append([]string(nil), p.Achievements...)
	return p
}
