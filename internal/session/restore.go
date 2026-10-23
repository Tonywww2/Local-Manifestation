package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/persistence"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/model"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func validateRestore(c *content.Catalog, saved *persistence.Checkpoint) error {
	s := saved.State
	bad := func(field string) error { return fmt.Errorf("incompatible checkpoint: %s", field) }
	if saved.Version != persistence.Version || saved.Digest != c.Digest || s.Floor < 1 || s.Floor > len(c.Run().Floors) {
		return bad("version, content or floor")
	}
	if _, err := time.Parse(time.RFC3339Nano, s.Started); err != nil {
		return bad("start time")
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", saved.Seed, s.Started)))
	if s.RunID != hex.EncodeToString(hash[:]) {
		return bad("adventure identity")
	}
	if s.SpecialMode && (s.BackpackCapacity < 1 || s.BackpackCapacity > 4096 || s.AccessorySlots < -1 || s.AccessorySlots > 4096) {
		return bad("inventory configuration")
	}
	expectedScope := "normal"
	if c.Namespace() != "base" || c.Modified() || s.SpecialMode || s.MetaRules != model.DefaultMetaRules() {
		expectedScope = "custom:" + c.Digest
	}
	if s.ProgressionScope != expectedScope {
		return bad("progression scope")
	}
	if s.AdventureTime < 0 || len(s.Scenes) != 0 || s.Vote != nil || len(s.Players) == 0 || s.SharedAP < 0 || s.PersonalInitial < 0 || s.SharedInitial < 0 {
		return bad("not a layer checkpoint")
	}
	if s.MapPlayers < 1 || s.MapPlayers > 128 {
		return bad("generation player count")
	}
	template, err := c.GenerateMap(s.Floor, saved.Seed, s.MapPlayers)
	if err != nil {
		return bad(err.Error())
	}
	if len(s.LayerMembers) != s.MapPlayers {
		return bad("layer membership")
	}
	seenMembers := map[string]bool{}
	for _, id := range s.LayerMembers {
		if seenMembers[id] {
			return bad("duplicate layer member")
		}
		found := false
		for _, p := range s.Players {
			if p.ID == id {
				found = true
			}
		}
		if !found {
			return bad("unknown layer member")
		}
		seenMembers[id] = true
	}
	extraX, extraY, special := c.TeamMeta(layerPlayers(s.Players, s.LayerMembers), s.MetaRules)
	x, y := c.Floor(s.Floor).PersonalAP+extraX, c.Floor(s.Floor).SharedAP+extraY
	if s.Cursed {
		x--
		y--
	}
	if s.PersonalInitial != x || s.SharedInitial != y || s.SharedAP != y {
		return bad("layer initial AP")
	}
	template = c.AddLayerSpecialNodes(template, special, s.Floor)
	if !reflect.DeepEqual(s.Terrain, template.Terrain) {
		return bad("terrain")
	}
	expected := map[string]model.MapNodeDef{}
	for _, n := range template.Nodes {
		expected[n.ID] = n
	}
	if len(s.Nodes) != len(expected) {
		return bad("map size")
	}
	nodes := map[string]bool{}
	for _, n := range s.Nodes {
		def, ok := expected[n.ID]
		if !ok || nodes[n.ID] || n.Definition != def.Definition || n.Q != def.Q || n.R != def.R || !reflect.DeepEqual(n.Edges, def.Edges) {
			return bad("node identity " + n.ID)
		}
		nd := c.Node(n.Definition)
		if n.Blocking != expedition.Blocking(nd) || n.Kind != nd.Kind || n.Payment != nd.Payment || n.Cost != nd.Cost || n.Room != nd.Room {
			return bad("node definition " + n.ID)
		}
		nodes[n.ID] = true
	}
	identities := map[string]string{}
	tokens := map[string]bool{}
	for _, identity := range saved.Identities {
		if identity.Player == "" || identity.Token == "" || identities[identity.Player] != "" || tokens[identity.Token] {
			return bad("duplicate or missing identity")
		}
		identities[identity.Player] = identity.Token
		tokens[identity.Token] = true
	}
	if len(identities) != len(s.Players) {
		return bad("identity count")
	}
	players := map[string]bool{}
	items := map[string]bool{}
	counters := map[string]uint64{}
	for _, p := range s.Players {
		counters[p.ID] = p.Inventory.NextID
	}
	checkID := func(id string) error {
		index := strings.LastIndex(id, ":")
		if index < 1 {
			return bad("item identity")
		}
		number, err := strconv.ParseUint(id[index+1:], 10, 64)
		if err != nil || number == 0 || number > counters[id[:index]] || items[id] {
			return bad("item identity or counter")
		}
		items[id] = true
		return nil
	}
	for _, p := range s.Players {
		if err := c.ValidateRanks(p.MetaRanks); err != nil {
			return bad("meta ranks " + p.ID)
		}
		if p.EnhancedWins < 0 || p.EnhancedWins > s.Floor {
			return bad("enhanced boss history")
		}
		if s.SpecialMode && (p.Inventory.Capacity != s.BackpackCapacity || p.Inventory.AccessorySlots != s.AccessorySlots) {
			return bad("inventory limits")
		}
		for _, it := range p.Inventory.Items {
			if err := checkID(it.ID); err != nil {
				return err
			}
		}
		for _, drop := range p.Loot {
			if !nodes[drop.Node] {
				return bad("loot node")
			}
			if err := checkID(drop.Item.ID); err != nil {
				return err
			}
		}
		if err := life.Validate(p); err != nil {
			return bad("life " + p.ID + ": " + err.Error())
		}
		if p.Statuses.Time != s.AdventureTime {
			return bad("status time")
		}
		if p.ID == "" || players[p.ID] || identities[p.ID] == "" || !nodes[p.Node] || p.Scene != "" || p.Weapon != "" && c.Weapon(p.Weapon).ID == "" || p.AP < 0 || p.Level < 1 || p.Downs < 0 || p.Downs > 3 || (p.Life != "alive" && p.Life != "down" && p.Life != "dead") {
			return bad("player " + p.ID)
		}
		if err := content.ValidatePlayerStats(c, p, s.Floor); err != nil {
			return bad("stats " + p.ID + ": " + err.Error())
		}
		players[p.ID] = true
	}
	if len(s.GroundLoot) > 4096 {
		return bad("shared loot budget")
	}
	for _, drop := range s.GroundLoot {
		if !nodes[drop.Node] {
			return bad("shared loot node")
		}
		if err := checkID(drop.Item.ID); err != nil {
			return err
		}
		if err := content.ValidateLoot(c, drop, s.AdventureTime); err != nil {
			return bad("shared loot: " + err.Error())
		}
	}
	visits := map[string]bool{}
	for _, visit := range saved.Visits {
		if !players[visit.Player] || visits[visit.Player] {
			return bad("visit owner " + visit.Player)
		}
		visits[visit.Player] = true
		seen := map[string]bool{}
		for _, node := range visit.Nodes {
			if !nodes[node] || seen[node] {
				return bad("visit node " + node)
			}
			seen[node] = true
		}
	}
	return nil
}
func layerPlayers(players []model.Player, ids []string) []model.Player {
	var out []model.Player
	for _, id := range ids {
		for _, p := range players {
			if p.ID == id {
				out = append(out, p)
				break
			}
		}
	}
	return out
}
