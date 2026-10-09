package expedition

import "hsmultiplayer/pkg/model"

// FreeTransit identifies rooms which can hand off movement without an admission
// transaction or combat actor. Paid entrances remain explicit confirmations.
func FreeTransit(n model.Node) bool {
	return n.Kind != "boss" && (n.Payment == "none" || n.Completed) && (n.Room == "" || n.Completed)
}
