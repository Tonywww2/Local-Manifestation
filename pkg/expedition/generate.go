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
	roomCount := count
	if g.Passage != "" {
		if count < 3 {
			return model.MapTemplateDef{}, fmt.Errorf("passage network needs at least three nodes")
		}
		// Reserve room for connecting passages and several alternate routes.
		roomCount = max(2, (count+2)*2/5)
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
	incoming := []int{0}
	passages := 0
	pool := slices.Clone(g.Pool)
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for len(out.Nodes) < roomCount {
		i := len(out.Nodes)
		distance := 1
		if g.Passage != "" {
			distance = 2
			// Keep some direct room-to-room edges. The last leaf reserves a
			// passage for the fixed BOSS approach, even in very small maps.
			if i < roomCount-1 && (i <= 2 || rng.Float64() < .25) {
				distance = 1
			}
		}
		var candidates []struct {
			from int
			p    axial
		}
		for i, n := range out.Nodes {
			// The second direct edge joins two actual rooms, rather than
			// making the only direct connections spokes of the start node.
			if g.Passage != "" && len(out.Nodes) == 2 && roomCount > 3 && i == 0 {
				continue
			}
			for _, d := range adjacent {
				p := axial{n.Q + d.q*distance, n.R + d.r*distance}
				if distance == 2 {
					if _, occupied := used[axial{n.Q + d.q, n.R + d.r}]; occupied {
						continue
					}
				}
				if _, ok := used[p]; !ok {
					candidates = append(candidates, struct {
						from int
						p    axial
					}{i, p})
				}
			}
		}
		if len(candidates) == 0 {
			return model.MapTemplateDef{}, fmt.Errorf("no free room placement")
		}
		choice := candidates[rng.Intn(len(candidates))]
		definition := pool[(i-1)%len(pool)]
		if (i-1)%len(pool) == 0 && i > 1 {
			rng.Shuffle(len(pool), func(a, b int) { pool[a], pool[b] = pool[b], pool[a] })
			definition = pool[0]
		}
		n := model.MapNodeDef{ID: fmt.Sprintf("node-%03d", i), Definition: definition, Q: choice.p.q, R: choice.p.r, Edges: []string{out.Nodes[choice.from].ID}}
		out.Nodes[choice.from].Edges = append(out.Nodes[choice.from].Edges, n.ID)
		out.Nodes = append(out.Nodes, n)
		used[choice.p] = i
		if distance == 2 {
			parent := out.Nodes[choice.from]
			used[axial{(parent.Q + n.Q) / 2, (parent.R + n.R) / 2}] = -1
			passages++
		}
		depth = append(depth, depth[choice.from]+distance)
		incoming = append(incoming, distance)
	}
	last := -1
	for i := 1; i < roomCount; i++ {
		if len(out.Nodes[i].Edges) != 1 || g.Passage != "" && g.BossApproach != "" && incoming[i] != 2 {
			continue
		}
		if last < 0 || depth[i] > depth[last] {
			last = i
		}
	}
	if last < 0 {
		return model.MapTemplateDef{}, fmt.Errorf("no eligible boss leaf")
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
	type link struct {
		a, b, distance int
		mid            axial
	}
	var links []link
	for i, n := range out.Nodes {
		maxDistance := 1
		if g.Passage != "" {
			maxDistance = 2
		}
		for distance := 1; distance <= maxDistance; distance++ {
			for _, d := range adjacent {
				p := axial{n.Q + d.q*distance, n.R + d.r*distance}
				mid := axial{n.Q + d.q, n.R + d.r}
				if other, ok := used[p]; ok && other > i && !slices.Contains(out.Nodes[i].Edges, out.Nodes[other].ID) {
					if _, occupied := used[mid]; distance == 2 && occupied {
						continue
					}
					links = append(links, link{a: i, b: other, distance: distance, mid: mid})
				}
			}
		}
	}
	rng.Shuffle(len(links), func(i, j int) { links[i], links[j] = links[j], links[i] })
	addedLoop := false
	for _, link := range links {
		if g.BossApproach != "" && (out.Nodes[link.a].ID == boss.ID || out.Nodes[link.b].ID == boss.ID) {
			continue
		}
		if link.distance == 2 {
			if _, occupied := used[link.mid]; occupied || roomCount+passages >= count {
				continue
			}
		}
		// A network with alternate-route generation enabled gets at least one
		// loop whenever its occupied cells offer one, rather than staying a tree.
		if rng.Float64() >= g.ExtraEdgeChance && !(g.Passage != "" && !addedLoop && g.ExtraEdgeChance > 0) {
			continue
		}
		a, b := link.a, link.b
		out.Nodes[a].Edges = append(out.Nodes[a].Edges, out.Nodes[b].ID)
		out.Nodes[b].Edges = append(out.Nodes[b].Edges, out.Nodes[a].ID)
		addedLoop = true
		if link.distance == 2 {
			used[link.mid] = -1
			passages++
		}
	}
	if g.Passage != "" {
		out = passageNetwork(out, g.Passage)
		if g.BossApproach != "" {
			var approach string
			for _, n := range out.Nodes {
				if n.ID == out.Boss && len(n.Edges) == 1 {
					approach = n.Edges[0]
				}
			}
			if approach == "" {
				return model.MapTemplateDef{}, fmt.Errorf("boss approach requires a single entrance")
			}
			for i := range out.Nodes {
				if out.Nodes[i].ID == approach {
					out.Nodes[i].Definition = g.BossApproach
				}
			}
		}
		used = map[axial]int{}
		for i, n := range out.Nodes {
			used[axial{n.Q, n.R}] = i
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

// Direct edges stay adjacent. Only two-cell edges get a real passage node;
// their reserved middle cells prevent overlap with rooms and other routes.
func passageNetwork(base model.MapTemplateDef, definition string) model.MapTemplateDef {
	out := base
	out.Nodes = make([]model.MapNodeDef, len(base.Nodes))
	indices := map[string]int{}
	for i, n := range base.Nodes {
		n.Edges = nil
		out.Nodes[i] = n
		indices[n.ID] = i
	}
	for i, n := range base.Nodes {
		for _, target := range n.Edges {
			j := indices[target]
			if i >= j {
				continue
			}
			a, b := out.Nodes[i], out.Nodes[j]
			if directRoomEdge(a, b) {
				out.Nodes[i].Edges = append(out.Nodes[i].Edges, b.ID)
				out.Nodes[j].Edges = append(out.Nodes[j].Edges, a.ID)
				continue
			}
			id := fmt.Sprintf("passage-%03d", len(out.Nodes))
			out.Nodes = append(out.Nodes, model.MapNodeDef{ID: id, Definition: definition, Q: (a.Q + b.Q) / 2, R: (a.R + b.R) / 2, Edges: []string{a.ID, b.ID}})
			out.Nodes[i].Edges = append(out.Nodes[i].Edges, id)
			out.Nodes[j].Edges = append(out.Nodes[j].Edges, id)
		}
	}
	return out
}

func directRoomEdge(a, b model.MapNodeDef) bool {
	for _, d := range adjacent {
		if b.Q-a.Q == d.q && b.R-a.R == d.r {
			return true
		}
	}
	return false
}
