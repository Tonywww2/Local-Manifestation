package content

import (
	"fmt"
	"hsmultiplayer/pkg/expedition"
	"hsmultiplayer/pkg/model"
	"math/rand"
	"slices"
)

func (c *Catalog) GenerateMap(floor int, seed int64, players int) (model.MapTemplateDef, error) {
	f := c.Floor(floor)
	base := c.Map(f.Map)
	if g := base.Generator; g != nil {
		pool := slices.Clone(g.Pool)
		rng := rand.New(rand.NewSource(Seed(seed, floor, "map-templates")))
		rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		seen := map[string]bool{}
		g.Pool = nil
		for _, id := range pool {
			if slices.Contains(f.ExcludedRandomKinds, c.nodes[id].Kind) {
				continue
			}
			key := id
			if node := c.nodes[id]; node.Room != "" {
				key = node.Room
			}
			if !seen[key] {
				seen[key] = true
				g.Pool = append(g.Pool, id)
			}
		}
	}
	return expedition.Generate(base, Seed(seed, floor, "map"), floor, players)
}

func (c *Catalog) AddLayerSpecialNodes(base model.MapTemplateDef, definitions []string, floor int) model.MapTemplateDef {
	excluded := c.Floor(floor).ExcludedRandomKinds
	allowed := make([]string, 0, len(definitions))
	for _, id := range definitions {
		if !slices.Contains(excluded, c.nodes[id].Kind) {
			allowed = append(allowed, id)
		}
	}
	return AddSpecialNodes(base, allowed)
}

// AddSpecialNodes fixes team unlocks into the layer's published map.
func AddSpecialNodes(base model.MapTemplateDef, definitions []string) model.MapTemplateDef {
	out := clone(base)
	directions := [6][2]int{{1, 0}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}, {1, -1}}
	used := map[[2]int]bool{}
	for _, n := range out.Nodes {
		used[[2]int{n.Q, n.R}] = true
	}
	for i, definition := range definitions {
		placed := false
		for j := range out.Nodes {
			if out.Nodes[j].ID == out.Boss {
				continue
			}
			for _, d := range directions {
				q, r := out.Nodes[j].Q+d[0], out.Nodes[j].R+d[1]
				if used[[2]int{q, r}] {
					continue
				}
				id := fmt.Sprintf("special-%03d", i)
				out.Nodes[j].Edges = append(out.Nodes[j].Edges, id)
				out.Nodes = append(out.Nodes, model.MapNodeDef{ID: id, Definition: definition, Q: q, R: r, Edges: []string{out.Nodes[j].ID}})
				used[[2]int{q, r}] = true
				placed = true
				break
			}
			if placed {
				break
			}
		}
	}
	out.Terrain = slices.DeleteFunc(out.Terrain, func(t model.Terrain) bool { return used[[2]int{t.Q, t.R}] })
	return out
}
