package content

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// Seed derives independent streams from stable identity, not scene creation order.
func Seed(seed int64, floor int, identity string) int64 {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%d:%s", seed, floor, len(identity), identity)))
	return int64(binary.LittleEndian.Uint64(digest[:8]))
}
