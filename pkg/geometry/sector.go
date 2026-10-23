package geometry

import (
	"hsmultiplayer/pkg/model"
	"math"
)

// SectorContainsDisk includes projectiles touching the sector's arc or rays.
func SectorContainsDisk(origin, direction, point model.Vec, radius, reach, arc float64) bool {
	v := point.Sub(origin)
	distance := v.Length()
	if distance > reach+radius {
		return false
	}
	if distance <= radius || arc >= 360 {
		return true
	}
	angle := math.Acos(max(-1, min(1, v.Unit().X*direction.Unit().X+v.Unit().Y*direction.Unit().Y)))
	return angle <= arc*math.Pi/360+math.Asin(min(1, radius/distance))
}
