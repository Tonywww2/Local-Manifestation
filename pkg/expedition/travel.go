package expedition

import (
	"hsmultiplayer/pkg/model"
	"slices"
)

// CanMove describes a single edge; shared votes deliberately retain this rule.
func CanMove(from, to model.Node) bool {
	return from.ID != "" && to.ID != "" && from.ID != to.ID && slices.Contains(from.Edges, to.ID)
}

// FreeTransit identifies rooms which can hand off movement without an admission
// transaction or combat actor. Paid entrances remain explicit confirmations.
func FreeTransit(n model.Node) bool {
	return n.Kind != "boss" && (n.Payment == "none" || n.Completed) && (n.Room == "" || n.Completed)
}

// Blocking is independent of payment. Only destination admission runs a node's
// interaction; intermediate nodes are transit, never a source of skipped rewards.
func Blocking(d model.NodeDef) bool {
	if d.Kind == "boss" {
		return true
	}
	if d.Blocking != nil {
		return *d.Blocking
	}
	return d.Kind != "passage" && d.Kind != "shop" && d.Kind != "start"
}

// Routes returns deterministic shortest paths (excluding origin, including goal).
// A blocking node may be the goal, but is not expanded until completed.
func Routes(nodes []model.Node, from string) map[string][]string {
	byID := make(map[string]model.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	out := map[string][]string{}
	if byID[from].ID == "" {
		return out
	}
	seen := map[string]bool{from: true}
	queue := []string{from}
	for head := 0; head < len(queue); head++ {
		id := queue[head]
		edges := slices.Clone(byID[id].Edges)
		slices.Sort(edges)
		for _, next := range edges {
			n, ok := byID[next]
			if !ok || seen[next] {
				continue
			}
			seen[next] = true
			out[next] = append(slices.Clone(out[id]), next)
			if n.Kind != "boss" && (!n.Blocking || n.Completed) {
				queue = append(queue, next)
			}
		}
	}
	return out
}
