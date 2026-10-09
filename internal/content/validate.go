package content

import (
	"fmt"
	"hsmultiplayer/pkg/damage"
	"math"
	"strings"

	"hsmultiplayer/pkg/geometry"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/stats"
	"hsmultiplayer/pkg/status"
)

func (c *Catalog) validate() error {
	definitionError := func(id, field string) error {
		return fmt.Errorf("%s: %s: invalid %s", c.manifest.Definitions, id, field)
	}
	adventureError := func(id, field string) error { return fmt.Errorf("%s: %s: invalid %s", c.manifest.Adventure, id, field) }
	ids := map[string]bool{}
	claim := func(id string) bool {
		allowed := false
		for _, namespace := range c.namespaces {
			if strings.HasPrefix(id, namespace+":") && len(id) > len(namespace)+1 {
				allowed = true
			}
		}
		if !allowed || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	valid := func(n, min, max float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= min && n <= max }
	countValid := func(n model.CountDef) bool {
		return n.Base >= 0 && n.Base <= 1000000 && n.PerFloor >= 0 && n.PerFloor <= 1000000 && n.PerPlayer >= 0 && n.PerPlayer <= 1000000 && n.Resolve(100, 128) <= 1000000
	}
	r := c.definitions.Run
	c.statuses = map[string]model.StatusDef{}
	for _, d := range c.definitions.Statuses {
		if !claim(d.ID) {
			return definitionError(d.ID, "status ID")
		}
		if err := status.ValidateDefinition(d); err != nil {
			return definitionError(d.ID, err.Error())
		}
		for _, m := range d.Modifiers {
			if !strings.Contains(m.ID, ":") {
				return definitionError(d.ID, "modifier namespace")
			}
		}
		c.statuses[d.ID] = d
	}
	for _, d := range c.definitions.Statuses {
		if err := c.validateCombat(d.Combat, true); err != nil {
			return definitionError(d.ID, err.Error())
		}
		for _, id := range d.Conflicts {
			if _, ok := c.statuses[id]; !ok {
				return definitionError(d.ID, "conflict reference")
			}
		}
	}
	if len(r.Floors) < 1 || len(r.Floors) > 100 || !valid(r.VoteRatio, .01, 1) {
		return definitionError("run", "floors or vote_ratio")
	}
	for _, w := range c.definitions.Weapons {
		if len(w.OnHitStatuses) > 8 {
			return definitionError(w.ID, "on-hit status budget")
		}
		for _, id := range w.OnHitStatuses {
			if _, ok := c.statuses[id]; !ok {
				return definitionError(w.ID, "on-hit status reference")
			}
		}
		if w.Mode != "" && w.Mode != "projectile" && w.Mode != "melee" || !valid(w.Windup, 0, 60) || !valid(w.Recovery, 0, 60) || !valid(w.Tracking, 0, 100) || w.Pierce < 0 || w.Pierce > 128 || w.Mode == "melee" && (!valid(w.Range, 1, 1000) || !valid(w.ActiveDuration, .02, 2)) {
			return definitionError(w.ID, "attack behavior")
		}
		if !claim(w.ID) {
			return definitionError(w.ID, "ID (namespace or duplicate)")
		}
		if !valid(w.Cooldown, .02, 60) || !(w.Mode == "melee" && valid(w.ProjectileSpeed, 0, 2000) || w.Mode != "melee" && valid(w.ProjectileSpeed, .01, 2000)) || !valid(w.Base, 0, 1e6) || !valid(w.Flat, -1e6, 1e6) || !valid(w.FinalFlat, -1e6, 1e6) {
			return definitionError(w.ID, "weapon parameters")
		}
		for _, score := range []float64{w.Scaling.Strength, w.Scaling.Agility, w.Scaling.Constitution, w.Scaling.Intelligence, w.Scaling.Sense, w.Scaling.Mana} {
			if !valid(score, 0, 1e6) {
				return definitionError(w.ID, "scaling")
			}
		}
		for _, m := range w.Modifiers {
			if m.Group == "" || m.Source == "" || !valid(m.Percent, -1, 1e6) {
				return definitionError(w.ID, "modifiers")
			}
		}
		if w.Type != "physical" && w.Type != "magic" && w.Type != "fire" && w.Type != "cold" && w.Type != "electromagnetic" && w.Type != "chemical" {
			return definitionError(w.ID, "damage type")
		}
		c.weapons[w.ID] = w
		if len(w.AdditionalDamage) > 15 {
			return definitionError(w.ID, "too many damage segments")
		}
		parts := map[string]bool{"base": true}
		for _, p := range w.AdditionalDamage {
			if p.ID == "" || strings.HasPrefix(p.ID, "hit:") || parts[p.ID] || !damage.ValidType(p.Type) || !valid(p.Base, 0, 1e6) || len(p.Bonuses) != 0 {
				return definitionError(w.ID, "damage segment")
			}
			parts[p.ID] = true
			for _, v := range []float64{p.Scaling.Strength, p.Scaling.Agility, p.Scaling.Constitution, p.Scaling.Intelligence, p.Scaling.Sense, p.Scaling.Mana} {
				if !valid(v, 0, 1e6) {
					return definitionError(w.ID, "segment scaling")
				}
			}
			for _, m := range p.Modifiers {
				if m.Group == "" || m.Source == "" || !valid(m.Percent, -1, 1e6) {
					return definitionError(w.ID, "segment modifier")
				}
			}
		}
	}
	if err := c.validateItems(claim); err != nil {
		return definitionError("items", err.Error())
	}
	if err := c.validateSkills(claim); err != nil {
		return definitionError("skills", err.Error())
	}
	if err := c.validatePerks(claim); err != nil {
		return definitionError("perks", err.Error())
	}
	for _, v := range c.definitions.Chassis {
		if v.RequiredPerk != "" {
			if _, ok := c.perks[v.RequiredPerk]; !ok {
				return definitionError(v.ID, "required perk")
			}
		}
		for _, id := range v.Skills {
			if id != "" {
				if _, ok := c.skills[id]; !ok {
					return definitionError(v.ID, "unknown skill")
				}
			}
		}
		if !claim(v.ID) || !valid(v.HP, 1, 1e6) || !valid(v.Speed, 1, 1000) || !valid(v.Armor, 0, 1e6) {
			return definitionError(v.ID, "chassis parameters or ID")
		}
		if _, ok := c.weapons[v.Weapon]; !ok {
			return definitionError(v.ID, "weapon reference "+v.Weapon)
		}
		for _, score := range []float64{v.Attributes.Strength, v.Attributes.Agility, v.Attributes.Constitution, v.Attributes.Intelligence, v.Attributes.Sense, v.Attributes.Mana} {
			if !valid(score, 0, 1e6) {
				return definitionError(v.ID, "attributes")
			}
		}
		contributions := make([]model.StatContribution, 0, len(v.Modifiers))
		for _, m := range v.Modifiers {
			validNamespace := false
			for _, ns := range c.namespaces {
				if strings.HasPrefix(m.ID, ns+":") {
					validNamespace = true
				}
			}
			if !validNamespace {
				return definitionError(v.ID, "modifier ID namespace")
			}
			contributions = append(contributions, model.StatContribution{StatModifier: m, Source: model.SourceRef{Kind: "chassis", Definition: v.ID}})
		}
		if _, err := stats.Evaluate(stats.Input{Base: model.StatValues{Attributes: v.Attributes, MaxHP: v.HP, Armor: v.Armor, Speed: v.Speed}, Contributions: contributions}); err != nil {
			return definitionError(v.ID, "modifiers: "+err.Error())
		}
		c.chassis[v.ID] = v
	}
	for _, v := range c.definitions.Enemies {
		if !valid(v.ArmorPerFloor, 0, 1e6) || v.EnrageProjectiles < 0 || v.EnrageProjectiles+v.ProjectileCount > 128 {
			return definitionError(v.ID, "enemy growth")
		}
		for k, n := range v.ResistancePerFloor {
			if !damage.ValidType(k) || !valid(n, 0, 1) {
				return definitionError(v.ID, "type resistance growth")
			}
		}
		if !valid(v.AttackRange, 0, 4096) || !valid(v.FireInterval, 0, 60) || !valid(v.ProjectileSpeed, 0, 2000) || v.ProjectileCount < 0 || v.ProjectileCount > 64 || v.DamageType != "" && !damage.ValidType(v.DamageType) || !valid(v.SpeedPerFloor, 0, 10) || !valid(v.FirePerFloor, 0, 10) || !valid(v.ThreatDistance, 0, 1e6) || !valid(v.ThreatDamage, 0, 1e6) || !valid(v.EnrageThreshold, 0, 1) || !valid(v.EnrageFactor, 0, 10) {
			return definitionError(v.ID, "attack or threat configuration")
		}
		if !valid(v.ControlResistance, 0, 1) {
			return definitionError(v.ID, "control resistance")
		}
		for k, n := range v.Resistance {
			if !damage.ValidType(k) || !valid(n, 0, 1) {
				return definitionError(v.ID, "type resistance")
			}
		}
		if !claim(v.ID) || !valid(v.HP, 1, 1e6) || !valid(v.Speed, 1, 1000) || !valid(v.Radius, 1, 80) || !valid(v.Damage, 0, 1e6) || !valid(v.Armor, 0, 1e6) || v.Behavior == "" {
			return definitionError(v.ID, "enemy parameters or ID")
		}
		if len(v.Attacks) > 8 {
			return definitionError(v.ID, "attack pattern budget")
		}
		for _, a := range v.Attacks {
			if a.Kind != "shoot" && a.Kind != "rush" || !valid(a.Windup, .1, 10) || !valid(a.Recovery, .1, 10) || !valid(a.Cooldown, 0, 60) || !valid(a.Range, 1, 4096) || !valid(a.Active, 0, 5) || !valid(a.Speed, 0, 1500) || a.Kind == "rush" && (a.Active <= 0 || a.Speed <= 0) || a.Kind == "shoot" && (v.ProjectileSpeed <= 0 || v.ProjectileCount < 1) {
				return definitionError(v.ID, "attack pattern")
			}
		}
		c.enemies[v.ID] = v
	}
	if len(c.chassis) == 0 || len(c.enemies) == 0 {
		return definitionError("catalog", "empty chassis or enemies")
	}
	if c.adventure.Format != model.ContentFormat {
		return adventureError("format", fmt.Sprintf("version (expected %d)", model.ContentFormat))
	}
	for _, room := range c.adventure.Rooms {
		if len(room.Obstacles) > 128 {
			return adventureError(room.ID, "obstacle budget")
		}
		for _, o := range room.Obstacles {
			if !o.Position.Finite() || !valid(o.Radius, 1, 200) || !geometry.NewArena(room.Width, room.Height).ContainsDisk(o.Position, o.Radius) {
				return adventureError(room.ID, "obstacle")
			}
		}
		if !claim(room.ID) || !valid(room.Width, 200, 4096) || !valid(room.Height, 200, 4096) || (room.Objective != "clear" && room.Objective != "survive") || room.Objective == "survive" && !valid(room.ObjectiveSeconds, 1, 3600) || (room.Join != "between_waves" && room.Join != "never") || !valid(room.InitialDelay, 0, 60) || len(room.Waves) < 1 || len(room.Waves) > 100 {
			return adventureError(room.ID, "room parameters or ID")
		}
		if !room.SpawnRadius.Finite() || room.SpawnRadius.X < 0 || room.SpawnRadius.X > room.Width/2 || room.SpawnRadius.Y < 0 || room.SpawnRadius.Y > room.Height/2 {
			return adventureError(room.ID, "spawn_radius")
		}
		if !valid(room.Scaling.HPVariation, 0, .3) || !valid(room.Scaling.HPFactor, .01, 100) || !valid(room.Scaling.HPPerFloor, 0, 100) || !valid(room.Scaling.HPPerExtraPlayer, 0, 100) || !valid(room.Scaling.DamagePerFloor, 0, 100) {
			return adventureError(room.ID, "scaling")
		}
		for i, w := range room.Waves {
			if w.Reward.XPPolicy != "" && w.Reward.XPPolicy != "individual" && w.Reward.XPPolicy != "total" || w.Reward.CoinsPolicy != "" && w.Reward.CoinsPolicy != "individual" && w.Reward.CoinsPolicy != "total" {
				return adventureError(room.ID, "reward policy")
			}
			if err := c.validateDraws(w.Reward.Draws); err != nil {
				return adventureError(room.ID, err.Error())
			}
			for _, drop := range w.Reward.Items {
				if _, ok := c.items[drop.Item]; !ok || !valid(drop.Chance, 0, 1) {
					return adventureError(room.ID, "drop definition")
				}
			}
			field := fmt.Sprintf("waves[%d]", i)
			if len(w.Enemies) == 0 || !valid(w.BreakSeconds, 0, 60) || !countValid(w.Reward.XP) || !countValid(w.Reward.Coins) {
				return adventureError(room.ID, field+" parameters")
			}
			maxCount, minCount := 0, 0
			for _, g := range w.Enemies {
				if _, ok := c.enemies[g.Enemy]; !ok {
					return adventureError(room.ID, field+" enemy reference "+g.Enemy)
				}
				if !countValid(g.Count) {
					return adventureError(room.ID, field+" enemy count")
				}
				maxCount += g.Count.Resolve(len(r.Floors), 128)
				minCount += g.Count.Resolve(1, 1)
			}
			if minCount < 1 || maxCount > 2048 {
				return adventureError(room.ID, field+" total enemy count (1..2048)")
			}
		}
		c.rooms[room.ID] = room
	}
	for _, n := range c.adventure.Nodes {
		if n.Facility != "" && n.Facility != "revive" || n.FacilityCost < 0 || n.FacilityCost > 1e6 {
			return adventureError(n.ID, "facility")
		}
		for _, offer := range n.Shop {
			if _, ok := c.items[offer.Item]; !ok || offer.Price < 0 || offer.Price > 1e6 {
				return adventureError(n.ID, "shop offer")
			}
		}
		if !claim(n.ID) || n.Kind == "" || n.Name == "" || n.Handler == "" {
			return adventureError(n.ID, "node ID, kind, name or handler")
		}
		if (n.Payment != "none" && n.Payment != "personal" && n.Payment != "shared") || n.Cost < 0 || n.Cost > 1000 || (n.Payment == "none") != (n.Cost == 0) {
			return adventureError(n.ID, "payment or cost")
		}
		if n.Room != "" {
			if _, ok := c.rooms[n.Room]; !ok {
				return adventureError(n.ID, "room reference "+n.Room)
			}
		}
		if n.Kind == "boss" && (n.Payment != "shared" || n.Room == "") {
			return adventureError(n.ID, "boss payment or room")
		}
		if n.Kind == "passage" && (n.Payment != "none" || n.Room != "" || n.Handler != "engine:noop") {
			return adventureError(n.ID, "passage must be free and have no combat or reward handler")
		}
		c.nodes[n.ID] = n
	}
	for _, d := range c.definitions.Perks {
		for _, id := range d.SpecialNodes {
			node := c.nodes[id]
			if node.ID == "" || node.Kind == "boss" || node.Kind == "start" {
				return adventureError(d.ID, "unknown or reserved special node")
			}
		}
	}
	for _, m := range c.adventure.Maps {
		if g := m.Generator; g != nil {
			if !countValid(g.Count) || g.Count.Resolve(1, 1) < 2 || g.Count.Resolve(len(r.Floors), 128) > 512 || len(g.Pool) == 0 || !valid(g.ExtraEdgeChance, 0, 1) {
				return adventureError(m.ID, "map generator")
			}
			for _, id := range g.Pool {
				n := c.nodes[id]
				if n.ID == "" || n.Kind == "boss" || n.Kind == "start" {
					return adventureError(m.ID, "generator pool")
				}
			}
		}
		if !claim(m.ID) || len(m.Nodes) < 2 || len(m.Nodes) > 512 {
			return adventureError(m.ID, "map ID or size")
		}
		entries := map[string]model.MapNodeDef{}
		positions := map[[2]int]bool{}
		for _, n := range m.Nodes {
			if c.nodes[n.Definition].Kind == "boss" && n.ID != m.Boss {
				return adventureError(m.ID, "additional boss node "+n.ID)
			}
			if n.ID == "" || n.ID == "enhanced" || n.ID == "exhaust" {
				return adventureError(m.ID, "reserved or empty node ID")
			}
			if _, ok := entries[n.ID]; ok {
				return adventureError(m.ID, "duplicate node "+n.ID)
			}
			if _, ok := c.nodes[n.Definition]; !ok {
				return adventureError(m.ID, "node reference "+n.Definition)
			}
			pos := [2]int{n.Q, n.R}
			if positions[pos] || n.Q < -1000 || n.Q > 1000 || n.R < -1000 || n.R > 1000 {
				return adventureError(m.ID, "node position")
			}
			positions[pos] = true
			entries[n.ID] = n
		}
		start, ok := entries[m.Start]
		if !ok || c.nodes[start.Definition].Kind != "start" || c.nodes[start.Definition].Payment != "none" || c.nodes[start.Definition].Room != "" {
			return adventureError(m.ID, "start reference")
		}
		boss, ok := entries[m.Boss]
		if !ok || c.nodes[boss.Definition].Kind != "boss" {
			return adventureError(m.ID, "boss reference")
		}
		for _, terrain := range m.Terrain {
			pos := [2]int{terrain.Q, terrain.R}
			if positions[pos] || terrain.Name == "" || terrain.Q < -1000 || terrain.Q > 1000 || terrain.R < -1000 || terrain.R > 1000 {
				return adventureError(m.ID, "terrain position or name")
			}
			positions[pos] = true
		}
		for _, n := range m.Nodes {
			seen := map[string]bool{}
			for _, edge := range n.Edges {
				other, ok := entries[edge]
				if !ok || edge == n.ID || seen[edge] {
					return adventureError(m.ID, "edge "+n.ID+" -> "+edge)
				}
				seen[edge] = true
				if _, adjacent := geometry.DoorSide(model.Node{Q: n.Q, R: n.R}, model.Node{Q: other.Q, R: other.R}); !adjacent {
					return adventureError(m.ID, "nonadjacent room edge "+n.ID+" -> "+edge)
				}
				reverse := false
				for _, back := range other.Edges {
					if back == n.ID {
						reverse = true
					}
				}
				if !reverse {
					return adventureError(m.ID, "nonreciprocal edge "+n.ID+" -> "+edge)
				}
			}
		}
		seen := map[string]bool{m.Start: true}
		queue := []string{m.Start}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			for _, next := range entries[id].Edges {
				if !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
		if len(seen) != len(entries) {
			return adventureError(m.ID, "disconnected nodes")
		}
		c.maps[m.ID] = m
	}
	for _, f := range c.adventure.Floors {
		if !claim(f.ID) || f.PersonalAP < 1 || f.SharedAP < 1 || f.PersonalAP > 1000 || f.SharedAP > 1000 {
			return adventureError(f.ID, "floor parameters or ID")
		}
		if _, ok := c.maps[f.Map]; !ok {
			return adventureError(f.ID, "map reference "+f.Map)
		}
		if _, ok := c.rooms[f.EnhancedRoom]; !ok {
			return adventureError(f.ID, "enhanced_room reference "+f.EnhancedRoom)
		}
		c.floors[f.ID] = f
	}
	for _, id := range r.Floors {
		if _, ok := c.floors[id]; !ok {
			return definitionError("run.floors", "floor reference "+id)
		}
	}
	return nil
}
