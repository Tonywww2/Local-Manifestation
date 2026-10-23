package client

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/model"
)

// Optional value metadata keeps a drawn weapon's real grip at the hand even
// when its stock or emitter makes the silhouette asymmetric.
type artLayout struct {
	WeaponGrips map[string]model.Vec `json:"weapon_grips"`
}

func validateArtLayout(layout artLayout) error {
	if len(layout.WeaponGrips) > 4096 {
		return fmt.Errorf("artwork grip budget exceeded")
	}
	for id, p := range layout.WeaponGrips {
		if id == "" || !p.Finite() || p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
			return fmt.Errorf("invalid artwork grip: %s", id)
		}
	}
	return nil
}

func (g *Game) weaponGrip(id string) model.Vec {
	if p, ok := g.resources.art.WeaponGrips[id]; ok {
		return p
	}
	return model.Vec{X: .22, Y: .5}
}

// Flat environment cutouts follow world rotation and scale. Furniture is baked
// into cached surfaces; interactive facilities use the same projection live.
func drawWorldArt(dst, texture *ebiten.Image, c Camera, p model.Vec, width, depth float64) bool {
	if texture == nil {
		return false
	}
	b := texture.Bounds()
	scale := min(width/float64(b.Dx()), depth/float64(b.Dy())) * c.Scale
	drawActorTexture(dst, texture, c.Project(p), float64(b.Dx())*scale, float64(b.Dy())*scale, c.Angle, 1, false)
	return true
}

func (g *Game) drawWorldProp(dst *ebiten.Image, id string, c Camera, p model.Vec, width, depth float64) bool {
	return drawWorldArt(dst, g.resources.sprites[id], c, p, width, depth)
}
