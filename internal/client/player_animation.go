package client

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
)

func sceneAction(actions []model.PlayerActionState, player string) (model.PlayerActionState, bool) {
	for _, a := range actions {
		if a.Player == player {
			return a, true
		}
	}
	return model.PlayerActionState{Player: player, Facing: model.Vec{X: 1}}, false
}

type playerVisualPose struct {
	Angle, Roll, Lift, Recoil, Fold, Pulse float64
	Flash                                  bool
	Rolling                                bool
	RollDirection                          model.Vec
}

func easeAnimation(t float64) float64 {
	t = max(0, min(1, t))
	return t * t * (3 - 2*t)
}

// Timelines and camera direction completely determine the pose. No frame
// counters, random effects, damage queries or wall clocks enter this function.
func samplePlayerVisual(a model.PlayerActionState, melee bool, cameraAngle float64) playerVisualPose {
	direction := a.Facing
	if direction.Length() == 0 {
		direction = model.Vec{X: 1}
	}
	v := playerVisualPose{}
	if animation.Active(a.Attack) {
		attack := a.Attack
		direction = attack.Direction
		if melee {
			active := max(.04, min(attack.Active, max(.04, attack.Duration-attack.Windup)))
			switch {
			case attack.Windup > 0 && attack.Elapsed < attack.Windup:
				v.Angle = -.95 * easeAnimation(attack.Elapsed/attack.Windup)
			case attack.Elapsed < attack.Windup+active:
				v.Angle = -.95 + 2.1*easeAnimation((attack.Elapsed-attack.Windup)/active)
			default:
				v.Angle = 1.15 * (1 - easeAnimation((attack.Elapsed-attack.Windup-active)/max(.02, attack.Duration-attack.Windup-active)))
			}
		} else {
			v.Recoil = max(0, 1-(attack.Elapsed-attack.Windup)/.12)
			v.Flash = attack.Elapsed >= attack.Windup && attack.Elapsed < attack.Windup+.07
			if attack.Elapsed < attack.Windup {
				v.Recoil, v.Flash = 0, false
			}
		}
	}
	if animation.Active(a.Skill) {
		skill := a.Skill
		direction = skill.Direction
		t := skill.Elapsed / skill.Duration
		v.Pulse = math.Sin(math.Pi * t)
		if skill.Kind == "clear" {
			v.Angle = -1.2 + 2.7*easeAnimation(min(1, t*2.5))
			if t > .55 {
				v.Angle *= 1 - easeAnimation((t-.55)/.45)
			}
		} else {
			v.Angle, v.Lift = -.25*v.Pulse, .2*v.Pulse
		}
		v.Flash = false
	}
	v.Angle += math.Atan2(direction.Y, direction.X) + cameraAngle
	if animation.Active(a.Dodge) {
		t := a.Dodge.Elapsed / a.Dodge.Duration
		v.RollDirection = rotatePoint(a.Dodge.Direction, cameraAngle).Unit()
		sign := 1.0
		if v.RollDirection.X < 0 {
			sign = -1
		}
		v.Rolling, v.Roll = true, sign*2*math.Pi*easeAnimation(t)
		v.Lift, v.Fold = .3*math.Sin(math.Pi*t), math.Sin(math.Pi*t)
		v.Angle += v.Roll
		v.Flash = false
	}
	return v
}

func (g *Game) heldWeaponKind(id string) string {
	mode, damageType := g.catalog.WeaponAppearance(id)
	if strings.Contains(id, "pry-bar") {
		return "pry-bar"
	}
	if mode == "melee" || strings.Contains(id, "sword") {
		return "blade"
	}
	if damageType == "magic" {
		return "focus"
	}
	return "rifle"
}

// These tiny fallback textures are uploaded once and released with the resource
// bundle. Mods may supply weapon:<weapon ID> to replace their held appearance.
func (g *Game) actorTexture(kind string) *ebiten.Image {
	id := "builtin:actor:" + kind
	if image := g.resources.sprites[id]; image != nil {
		return image
	}
	if g.resources.images == nil {
		g.resources.images = map[string]*ebiten.Image{}
	}
	if g.resources.sprites == nil {
		g.resources.sprites = map[string]*ebiten.Image{}
	}
	bodyBounds := image.Rect(27, 6, 69, 64)
	image := ebiten.NewImage(96, 64)
	metal, dark := color.RGBA{192, 207, 210, 255}, color.RGBA{46, 59, 64, 255}
	switch kind {
	case "body":
		rect(image, 33, 25, 30, 25, color.RGBA{108, 136, 140, 255})
		rect(image, 38, 29, 20, 18, dark)
		rect(image, 28, 27, 7, 22, metal)
		rect(image, 61, 27, 7, 22, metal)
		rect(image, 35, 48, 10, 15, dark)
		rect(image, 51, 48, 10, 15, dark)
		vector.DrawFilledCircle(image, 48, 17, 10, color.RGBA{210, 203, 184, 255}, true)
		rect(image, 37, 8, 22, 8, dark)
	case "blade":
		rect(image, 4, 29, 25, 6, dark)
		rect(image, 26, 21, 5, 22, hudGold)
		rect(image, 31, 27, 55, 10, metal)
		rect(image, 34, 27, 50, 3, uiInk)
		line(image, model.Vec{X: 86, Y: 27}, model.Vec{X: 93, Y: 32}, metal, 4)
		line(image, model.Vec{X: 93, Y: 32}, model.Vec{X: 86, Y: 37}, metal, 4)
	case "pry-bar":
		rect(image, 5, 29, 72, 7, metal)
		rect(image, 6, 28, 23, 9, dark)
		line(image, model.Vec{X: 77, Y: 32}, model.Vec{X: 86, Y: 20}, metal, 7)
		line(image, model.Vec{X: 86, Y: 20}, model.Vec{X: 92, Y: 20}, metal, 7)
	case "focus":
		rect(image, 4, 29, 68, 6, dark)
		rect(image, 12, 30, 50, 2, hudGold)
		vector.DrawFilledCircle(image, 77, 32, 12, metal, true)
		vector.DrawFilledCircle(image, 77, 32, 8, accent, true)
	case "flash":
		vector.DrawFilledCircle(image, 48, 32, 15, color.RGBA{255, 213, 124, 180}, true)
		rect(image, 18, 29, 60, 6, color.RGBA{255, 242, 204, 255})
		rect(image, 45, 8, 6, 48, color.RGBA{255, 242, 204, 255})
	case "pulse":
		vector.StrokeCircle(image, 48, 32, 27, 3, accent, true)
	case "shadow":
		vector.DrawFilledCircle(image, 48, 32, 28, color.RGBA{15, 24, 27, 85}, true)
	default: // contemporary receiver, stock, grip, magazine and barrel
		rect(image, 4, 26, 26, 13, dark)
		rect(image, 26, 23, 38, 14, metal)
		rect(image, 33, 36, 8, 17, dark)
		rect(image, 48, 36, 10, 15, dark)
		rect(image, 63, 27, 28, 6, dark)
		rect(image, 42, 16, 21, 6, dark)
		rect(image, 30, 25, 28, 3, uiInk)
	}
	g.resources.images[id], g.resources.sprites[id] = image, image
	if kind == "body" {
		// The generated cutout has known bounds; avoid a GPU readback on its first frame.
		g.resources.sprites[id] = image.SubImage(bodyBounds).(*ebiten.Image)
	}
	return g.resources.sprites[id]
}

func drawActorTexture(dst, texture *ebiten.Image, center model.Vec, width, height, angle, alpha float64, flip bool) {
	r := texture.Bounds()
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-float64(r.Dx())/2, -float64(r.Dy())/2)
	sx := width / float64(r.Dx())
	if flip {
		sx = -sx
	}
	op.GeoM.Scale(sx, height/float64(r.Dy()))
	op.GeoM.Rotate(angle)
	op.GeoM.Translate(center.X, center.Y)
	op.ColorScale.ScaleAlpha(float32(max(0, min(1, alpha))))
	dst.DrawImage(texture, op)
}

func (g *Game) drawPlayerBodyPose(dst *ebiten.Image, chassis, life string, ground model.Vec, radius float64, fallback color.RGBA, pose playerVisualPose, alpha float64, flip bool) float64 {
	if life != "alive" {
		vector.DrawFilledCircle(dst, float32(ground.X), float32(ground.Y), float32(radius), fallback, true)
		return radius
	}
	image := g.resources.sprites["player:"+chassis]
	if image == nil {
		image = g.actorTexture("body")
	}
	height := radius * 4
	width := min(radius*3.4, height*float64(image.Bounds().Dx())/float64(image.Bounds().Dy()))
	// Preserve the cutout's proportions even when using a wide fallback texture.
	height = min(height, width*float64(image.Bounds().Dy())/float64(image.Bounds().Dx()))
	center := ground.Add(model.Vec{Y: radius*.6 - height/2 - pose.Lift*radius})
	drawActorTexture(dst, image, center, width*(1+.12*pose.Fold), height*(1-.18*pose.Fold), pose.Roll, alpha, flip)
	return max(radius, height-radius*.6)
}

func (g *Game) drawAnimatedPlayer(dst *ebiten.Image, p model.Player, action model.PlayerActionState, c Camera, ground model.Vec, radius float64, col color.RGBA) float64 {
	action.Facing = animation.PlayerFacing(action, model.Vec{}, model.Vec{}, false)
	weapon := p.Weapon
	if animation.Active(action.Attack) && action.Attack.Weapon != "" {
		weapon = action.Attack.Weapon
	}
	if animation.Active(action.Skill) && action.Skill.Weapon != "" {
		weapon = action.Skill.Weapon
	}
	kind := g.heldWeaponKind(weapon)
	pose := samplePlayerVisual(action, kind == "blade" || kind == "pry-bar", c.Angle)
	if rig, ok := g.resources.skeletons.Rigs[p.Chassis]; ok {
		return g.drawSkeletonPlayer(dst, rig, p, action, c, ground, radius, weapon, kind, pose)
	}
	if p.Life != "alive" {
		pose = playerVisualPose{}
	}
	flip := rotatePoint(action.Facing, c.Angle).X < 0
	if pose.Rolling {
		flip = pose.RollDirection.X < 0
	}
	drawActorTexture(dst, g.actorTexture("shadow"), ground.Add(model.Vec{Y: radius * .3}), radius*2.7, radius*.65, 0, 1, false)
	if pose.Rolling {
		for i := 2; i > 0; i-- {
			trail := ground.Sub(pose.RollDirection.Scale(radius * .8 * float64(i)))
			g.drawPlayerBodyPose(dst, p.Chassis, p.Life, trail, radius, col, pose, .12/float64(i), flip)
		}
	}
	if pose.Pulse > 0 && p.Life == "alive" {
		drawActorTexture(dst, g.actorTexture("pulse"), ground, radius*(2.4+pose.Pulse), radius*(2.4+pose.Pulse), 0, pose.Pulse*.65, false)
	}
	direction := model.Vec{X: math.Cos(pose.Angle), Y: math.Sin(pose.Angle)}
	back := direction.Y < -.15
	if back && p.Life == "alive" && weapon != "" {
		g.drawHeldWeapon(dst, weapon, kind, ground, radius, pose)
	}
	height := g.drawPlayerBodyPose(dst, p.Chassis, p.Life, ground, radius, col, pose, 1, flip)
	if !back && p.Life == "alive" && weapon != "" {
		g.drawHeldWeapon(dst, weapon, kind, ground, radius, pose)
	}
	return height
}

func (g *Game) drawHeldWeapon(dst *ebiten.Image, id, kind string, ground model.Vec, radius float64, pose playerVisualPose) {
	direction := model.Vec{X: math.Cos(pose.Angle), Y: math.Sin(pose.Angle)}
	hand := ground.Add(model.Vec{Y: -radius * (1.1 + pose.Lift)}).Add(direction.Scale(radius * (.75 - .25*pose.Recoil - .25*pose.Fold)))
	line(dst, ground.Add(model.Vec{Y: -radius}), hand, color.RGBA{210, 203, 184, 255}, float32(max(2, radius*.24)))
	g.drawHeldWeaponAt(dst, id, kind, hand, radius, pose)
}

func (g *Game) drawHeldWeaponAt(dst *ebiten.Image, id, kind string, hand model.Vec, radius float64, pose playerVisualPose) {
	direction := model.Vec{X: math.Cos(pose.Angle), Y: math.Sin(pose.Angle)}
	image := g.resources.sprites["weapon:"+id]
	if image == nil {
		image = g.actorTexture(kind)
	}
	width := radius * 2.8
	height := width * float64(image.Bounds().Dy()) / float64(image.Bounds().Dx())
	if direction.X < 0 {
		// Preserve the weapon's upper side when aiming left; the grip offset
		// must mirror with it rather than leaving a rifle upside down.
		height = -height
	}
	grip := g.weaponGrip(id)
	center := hand.Add(rotatePoint(model.Vec{X: width * (.5 - grip.X), Y: height * (.5 - grip.Y)}, pose.Angle))
	drawActorTexture(dst, image, center, width, height, pose.Angle, 1, false)
	if pose.Flash && kind == "rifle" {
		muzzle := center.Add(direction.Scale(width * .48))
		drawActorTexture(dst, g.actorTexture("flash"), muzzle, radius*1.2, radius*.8, pose.Angle, .9, false)
	}
}
