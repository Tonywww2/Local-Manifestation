package expedition

import "hsmultiplayer/pkg/model"

// Visibility stops at unfinished blocking nodes. Passages and public landmarks
// remain known without revealing neighbours or bypassing a blocker.
func Visibility(nodes []model.Node, from string) map[string]bool {
	visible := map[string]bool{from: true}
	for id := range Routes(nodes, from) {
		visible[id] = true
	}
	for _, n := range nodes {
		if n.Kind == "passage" || n.Kind == "shop" || n.Kind == "boss" {
			visible[n.ID] = true
		}
	}
	return visible
}
