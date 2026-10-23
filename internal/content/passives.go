package content

import (
	"fmt"
	"math/rand"
	"sort"

	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/inventory"
	"hsmultiplayer/pkg/model"
)

// MatchingPassives snapshots bag order before a trigger consumes any instance.
func (c *Catalog) MatchingPassives(p model.Player, trigger string) []model.ItemInstance {
	var out []model.ItemInstance
	for _, it := range inventory.Bag(p.Inventory) {
		d := c.items[it.Definition]
		if d.Passive != nil && (d.Passive.Trigger == trigger || trigger == "movement_combat" && d.Passive.Trigger == "movement") {
			out = append(out, it)
		}
	}
	return out
}

func (c *Catalog) applyPassive(p model.Player, it model.ItemInstance) (model.Player, error) {
	d := c.items[it.Definition]
	n := p
	var err error
	n.Inventory, err = inventory.SpendDurability(p.Inventory, it.ID, c.Item)
	if err != nil {
		return model.Player{}, err
	}
	if d.Passive.Status != "" {
		n, err = ApplyStatus(c, n, d.Passive.Status, model.SourceRef{Kind: "item", Actor: p.ID, Definition: d.ID, Instance: it.ID})
	}
	return n, err
}

// SettlePlayerDamage stages the entire incoming hit. All bag fuses matching the
// same pre-trigger lethal event fire, and immunity preserves prior shields too.
func (c *Catalog) SettlePlayerDamage(r damage.Result, p model.Player) (damage.Result, model.Player, error) {
	settled, hp, shields, err := damage.Settle(r, p.HP, p.Statuses)
	if err != nil {
		return damage.Result{}, model.Player{}, err
	}
	if hp <= 0 && p.Life == "alive" {
		items := c.MatchingPassives(p, "lethal_damage")
		if len(items) > 0 {
			n := p
			for _, it := range items {
				n, err = c.applyPassive(n, it)
				if err != nil {
					return damage.Result{}, model.Player{}, err
				}
			}
			settled.Absorbed, settled.HealthLoss, settled.Overkill = 0, 0, 0
			for i := range settled.Segments {
				settled.Segments[i].Absorbed, settled.Segments[i].HealthLoss, settled.Segments[i].Overkill = 0, 0, 0
			}
			return settled, n, nil
		}
	}
	n := p
	n.HP, n.Statuses = hp, shields
	return settled, n, nil
}

// FloorAccessoryPool derives the authored layer pool once per arrival, never
// from an unordered all-items scan. Duplicate entries in room templates do not
// multiply their authored drop weights.
func (c *Catalog) FloorAccessoryPool(floor int) []model.WeightedDrop {
	f := c.Floor(floor)
	m := c.maps[f.Map]
	ids := map[string]bool{}
	for _, n := range m.Nodes {
		ids[n.Definition] = true
	}
	if m.Generator != nil {
		for _, id := range m.Generator.Pool {
			ids[id] = true
		}
	}
	weights := map[string]float64{}
	for id := range ids {
		room := c.rooms[c.nodes[id].Room]
		for _, wave := range room.Waves {
			for _, draw := range wave.Reward.Draws {
				for _, drop := range draw.Items {
					if c.items[drop.Item].Kind == "accessory" {
						weights[drop.Item] = max(weights[drop.Item], drop.Weight)
					}
				}
			}
		}
	}
	keys := make([]string, 0, len(weights))
	for id := range weights {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	out := make([]model.WeightedDrop, 0, len(keys))
	for _, id := range keys {
		out = append(out, model.WeightedDrop{Item: id, Weight: weights[id]})
	}
	return out
}

func (c *Catalog) MovementPassives(p model.Player, floor int, seed int64, combat bool) (model.Player, error) {
	trigger := "movement"
	if combat {
		trigger = "movement_combat"
	}
	items := c.MatchingPassives(p, trigger)
	if len(items) == 0 {
		return p, nil
	}
	pool := c.FloorAccessoryPool(floor)
	n := p
	for _, it := range items {
		var err error
		n, err = c.applyPassive(n, it)
		if err != nil {
			return model.Player{}, err
		}
		if c.items[it.Definition].Passive.Accessory {
			if len(pool) == 0 {
				return model.Player{}, fmt.Errorf("floor accessory pool empty")
			}
			rng := rand.New(rand.NewSource(Seed(seed, floor, p.ID+":"+it.ID+":"+fmt.Sprint(p.Inventory.NextID))))
			id := SelectLoot(pool, rng.Float64())
			n, err = c.Drop(n, id, n.Node, n.Position, rng.Float64())
			if err != nil {
				return model.Player{}, err
			}
			// Standard pickup semantics: a full bag leaves the reward on the floor.
			if picked, e := c.Pickup(n, n.Loot[len(n.Loot)-1].Item.ID); e == nil {
				n = picked
			}
		}
	}
	return n, nil
}
