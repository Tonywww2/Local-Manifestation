package expedition

import "hsmultiplayer/pkg/model"

// MovementRange ignores edges and blockers, but never invents a destination in
// an empty cell. The caller still validates combat admission and resource costs.
func MovementRange(from, to model.Node, d model.MovementItemDef) bool {
	if from.ID == "" || to.ID == "" || from.ID == to.ID {
		return false
	}
	q, r := to.Q-from.Q, to.R-from.R
	abs := func(n int) int {
		if n < 0 {
			return -n
		}
		return n
	}
	distance := max(abs(q), abs(r), abs(q+r))
	switch d.Mode {
	case "anywhere":
		return true
	case "ring":
		return distance <= d.Range
	case "line":
		return distance <= d.Range && (q == 0 || r == 0 || q == -r)
	}
	return false
}
