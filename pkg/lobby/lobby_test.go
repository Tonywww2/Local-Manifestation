package lobby

import (
	"hsmultiplayer/pkg/model"
	"testing"
)

func TestHallDevicesReachableAndMovementRateIndependent(t *testing.T) {
	a := Footprint()
	seen := map[string]bool{}
	for _, d := range Devices() {
		if seen[d.ID] || !a.ContainsDisk(d.Position, 12) || d.NameKey == "" || d.DescriptionKey == "" {
			t.Fatal(d)
		}
		seen[d.ID] = true
	}
	for _, rate := range []int{30, 60} {
		p := Spawn()
		for i := 0; i < rate; i++ {
			p = Move(p, model.Vec{X: 1}, 100, 1.0/float64(rate))
		}
		if p.Sub(Spawn()).Length() < 99.999 || p.Sub(Spawn()).Length() > 100.001 {
			t.Fatal(p)
		}
	}
}
func TestRoomConfigBounds(t *testing.T) {
	for _, players := range []int{1, 8, 128} {
		c := DefaultConfig()
		c.Players = players
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, players := range []int{0, 129} {
		c := DefaultConfig()
		c.Players = players
		if c.Validate() == nil {
			t.Fatal("unbounded capacity")
		}
	}
}
