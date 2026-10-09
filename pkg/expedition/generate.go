package expedition

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"math/rand"
	"slices"
)

type axial struct{ q, r int }

var adjacent = [6]axial{{1, 0}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}, {1, -1}}

// Generate is deterministic for explicit seed/floor/player inputs; it never
// changes a published layer when players join, leave or acquire AP bonuses.
func Generate(base model.MapTemplateDef, seed int64, floor, players int) (model.MapTemplateDef, error) {
	g := base.Generator
	if g == nil {
		return base, nil
	}
	count := g.Count.Resolve(floor, players)
	if count < 2 || count > 512 || len(g.Pool) == 0 {
		return model.MapTemplateDef{}, fmt.Errorf("invalid generation request")
	}
	rng := rand.New(rand.NewSource(seed))
	var start, boss model.MapNodeDef
	for _, n := range base.Nodes {
		if n.ID == base.Start {
			start = n
		}
		if n.ID == base.Boss {
			boss = n
		}
	}
	if start.ID == "" || boss.ID == "" {
		return model.MapTemplateDef{}, fmt.Errorf("missing generation endpoints")
	}
	out := model.MapTemplateDef{ID: base.ID, Start: base.Start, Boss: base.Boss}
	start.Q, start.R = 0, 0
	start.Edges = nil
	out.Nodes = append(out.Nodes, start)
	used := map[axial]int{{0, 0}: 0}
	depth := []int{0}
	pool := slices.Clone(g.Pool)
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for len(out.Nodes) < count {
		var candidates []struct {
			from int
			p    axial
		}
		for i, n := range out.Nodes {
			for _, d := range adjacent {
				p := axial{n.Q + d.q, n.R + d.r}
				if _, ok := used[p]; !ok {
					candidates = append(candidates, struct {
						from int
						p    axial
					}{i, p})
				}
			}
		}
		choice := candidates[rng.Intn(len(candidates))]
		i := len(out.Nodes)
		definition := pool[(i-1)%len(pool)]
		if (i-1)%len(pool) == 0 && i > 1 {
			rng.Shuffle(len(pool), func(a, b int) { pool[a], pool[b] = pool[b], pool[a] })
			definition = pool[0]
		}
		n := model.MapNodeDef{ID: fmt.Sprintf("node-%03d", i), Definition: definition, Q: choice.p.q, R: choice.p.r, Edges: []string{out.Nodes[choice.from].ID}}
		out.Nodes[choice.from].Edges = append(out.Nodes[choice.from].Edges, n.ID)
		out.Nodes = append(out.Nodes, n)
		used[choice.p] = i
		depth = append(depth, depth[choice.from]+1)
	}
	last := 1
	for i := 2; i < count; i++ {
		if depth[i] > depth[last] {
			last = i
		}
	}
	old := out.Nodes[last].ID
	out.Nodes[last].ID = boss.ID
	out.Nodes[last].Definition = boss.Definition
	for i := range out.Nodes {
		for j, edge := range out.Nodes[i].Edges {
			if edge == old {
				out.Nodes[i].Edges[j] = boss.ID
			}
		}
	}
	for i, n := range out.Nodes {
		for _, d := range adjacent {
			p := axial{n.Q + d.q, n.R + d.r}
			if other, ok := used[p]; ok && other > i && !slices.Contains(out.Nodes[i].Edges, out.Nodes[other].ID) && rng.Float64() < g.ExtraEdgeChance {
				out.Nodes[i].Edges = append(out.Nodes[i].Edges, out.Nodes[other].ID)
				out.Nodes[other].Edges = append(out.Nodes[other].Edges, n.ID)
			}
		}
	}
	terrain := map[axial]bool{}
	for _, n := range out.Nodes {
		for _, d := range adjacent {
			p := axial{n.Q + d.q, n.R + d.r}
			if _, ok := used[p]; !ok {
				terrain[p] = true
			}
		}
	}
	var cells []axial
	for p := range terrain {
		cells = append(cells, p)
	}
	slices.SortFunc(cells, func(a, b axial) int {
		if a.r != b.r {
			return a.r - b.r
		}
		return a.q - b.q
	})
	for _, p := range cells {
		out.Terrain = append(out.Terrain, model.Terrain{Q: p.q, R: p.r, Name: "Cliff", NameKey: "terrain.cliff"})
	}
	return out, nil
}
