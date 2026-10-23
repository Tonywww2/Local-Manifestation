package client

import (
	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
	"math"
)

func (g *Game) weaponAnimationClass(id, kind string) string {
	if clip := g.resources.skeletons.Library.WeaponClips[id]; clip != "" {
		return clip
	}
	switch kind {
	case "blade", "pry-bar":
		return "melee"
	case "focus":
		return "cast"
	default:
		return "rifle"
	}
}

func skeletonPartSize(r skeletonRig, p animation.SkeletonPose, index int, height float64) (float64, float64) {
	img := r.Parts[p.Direction][index]
	if img == nil {
		return 0, 0
	}
	b := img.Bounds()
	size := animation.PartSize(r.Definition.Bones[index], p.Bones[index], model.Vec{X: float64(b.Dx()), Y: float64(b.Dy())}).Scale(height)
	return size.X, size.Y
}

func skeletonHand(r skeletonRig, p animation.SkeletonPose, ground model.Vec, height float64) model.Vec {
	b := p.Bones[animation.ArmRight]
	_, h := skeletonPartSize(r, p, animation.ArmRight, height)
	pivot := r.Definition.Bones[animation.ArmRight].Pivot
	return ground.Add(b.Position.Scale(height)).Add(rotatePoint(model.Vec{Y: h * (.95 - pivot.Y)}, b.Angle))
}

func drawSkeletonPart(dst *ebiten.Image, r skeletonRig, p animation.SkeletonPose, index int, ground model.Vec, height, alpha float64) {
	img := r.Parts[p.Direction][index]
	if img == nil {
		return
	}
	w, h := skeletonPartSize(r, p, index, height)
	if w <= 0 || h <= 0 {
		return
	}
	bone := p.Bones[index]
	definition := r.Definition.Bones[index]
	bounds := img.Bounds()
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-float64(bounds.Dx())*definition.Pivot.X, -float64(bounds.Dy())*definition.Pivot.Y)
	op.GeoM.Scale(w/float64(bounds.Dx()), h/float64(bounds.Dy()))
	op.GeoM.Rotate(bone.Angle)
	v := ground.Add(bone.Position.Scale(height))
	op.GeoM.Translate(v.X, v.Y)
	op.ColorScale.Scale(float32(1+.4*p.Hurt), float32(1+.25*p.Hurt), float32(1+.25*p.Hurt), float32(max(0, min(1, p.Alpha*alpha))))
	dst.DrawImage(img, op)
}
func drawSkeleton(dst *ebiten.Image, r skeletonRig, p animation.SkeletonPose, ground model.Vec, height, alpha float64) {
	for _, i := range p.Order {
		drawSkeletonPart(dst, r, p, i, ground, height, alpha)
	}
}

func (g *Game) drawSkeletonPlayer(dst *ebiten.Image, r skeletonRig, p model.Player, a model.PlayerActionState, c Camera, ground model.Vec, radius float64, weapon, kind string, weaponPose playerVisualPose) float64 {
	in := animation.SkeletonInput{Facing: rotatePoint(a.Facing, c.Angle), Move: rotatePoint(a.Move, c.Angle), Walk: a.Walk, Time: a.Walk, WeaponClass: g.weaponAnimationClass(weapon, kind), Life: p.Life, Attack: a.Attack, Skill: a.Skill, Dodge: a.Dodge, Hit: a.Hit, LifeAction: a.Life}
	in.Dodge.Direction = rotatePoint(in.Dodge.Direction, c.Angle)
	in.Hit.Direction = rotatePoint(in.Hit.Direction, c.Angle)
	pose := animation.SampleSkeleton(r.Definition, in)
	height := radius * r.Definition.Height
	if pose.Alpha <= 0 {
		return height
	}
	drawActorTexture(dst, g.actorTexture("shadow"), ground.Add(model.Vec{Y: radius * .3}), radius*2.7, radius*.65, 0, pose.Alpha, false)
	if weaponPose.Rolling && p.Life == "alive" {
		for i := 2; i > 0; i-- {
			drawSkeleton(dst, r, pose, ground.Sub(weaponPose.RollDirection.Scale(radius*.8*float64(i))), height, .12/float64(i))
		}
	}
	if weaponPose.Pulse > 0 && p.Life == "alive" {
		drawActorTexture(dst, g.actorTexture("pulse"), ground, radius*(2.4+weaponPose.Pulse), radius*(2.4+weaponPose.Pulse), 0, weaponPose.Pulse*.65, false)
	}
	hand := skeletonHand(r, pose, ground, height)
	back := math.Sin(weaponPose.Angle) < -.15
	if back && p.Life == "alive" && weapon != "" {
		g.drawHeldWeaponAt(dst, weapon, kind, hand, radius, weaponPose)
	}
	drawSkeleton(dst, r, pose, ground, height, 1)
	if !back && p.Life == "alive" && weapon != "" {
		g.drawHeldWeaponAt(dst, weapon, kind, hand, radius, weaponPose)
	}
	return height
}

func (g *Game) drawSkeletonEnemy(dst *ebiten.Image, c Camera, e model.Enemy, a model.EnemyActionState) (float64, bool) {
	r, ok := g.resources.skeletons.Rigs[e.Definition]
	if !ok {
		return 0, false
	}
	life := "alive"
	if a.Death.Duration > 0 {
		life = "dead"
	}
	class := "melee"
	if a.Attack.Kind == "shoot" {
		class = "cast"
	}
	in := animation.SkeletonInput{Facing: rotatePoint(a.Facing, c.Angle), Move: rotatePoint(a.Move, c.Angle), Walk: a.Walk, Time: a.Walk, Life: life, WeaponClass: class, Attack: a.Attack, Hit: a.Hit, LifeAction: a.Death}
	in.Hit.Direction = rotatePoint(a.Hit.Direction, c.Angle)
	pose := animation.SampleSkeleton(r.Definition, in)
	height := g.catalog.EnemyVisualRadius(e.Definition, e.Radius) * c.Scale * r.Definition.Height
	bounds := r.Rest[pose.Direction]
	if pose.Alpha > 0 {
		// The world position is the full body's center. The rig keeps its
		// original feet-based bone coordinates and its independent art scale.
		ground := c.Project(e.Position).Sub(bounds.Center().Scale(height))
		shadow := ground.Add(model.Vec{Y: bounds.Max.Y * height})
		drawActorTexture(dst, g.actorTexture("shadow"), shadow, height*.7, height*.16, 0, pose.Alpha, false)
		drawSkeleton(dst, r, pose, ground, height, 1)
	}
	return (bounds.Max.Y - bounds.Min.Y) * height / 2, true
}
