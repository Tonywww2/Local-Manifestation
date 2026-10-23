package animation

import (
	"fmt"
	"hsmultiplayer/pkg/model"
	"math"
)

const BoneCount = 6
const DirectionCount = 4

// Atlas columns and direction rows are stable content contracts.
const (
	Body = iota
	Head
	ArmLeft
	ArmRight
	LegLeft
	LegRight
)
const (
	Front = iota
	Back
	Right
	Left
)

type BoneDefinition struct {
	Parent int       `json:"parent"`
	Offset model.Vec `json:"offset"`
	Size   model.Vec `json:"size"`
	Pivot  model.Vec `json:"pivot"`
}
type ClipTuning struct {
	Amplitude float64 `json:"amplitude"`
	Speed     float64 `json:"speed"`
}
type RigDefinition struct {
	Profile   string                         `json:"profile"`
	Height    float64                        `json:"height"`
	Bones     [BoneCount]BoneDefinition      `json:"bones"`
	DrawOrder [DirectionCount][BoneCount]int `json:"draw_order"`
	Clips     map[string]ClipTuning          `json:"clips,omitempty"`
}
type RigLibrary struct {
	Version     int                      `json:"version"`
	Rigs        map[string]RigDefinition `json:"rigs"`
	WeaponClips map[string]string        `json:"weapon_clips,omitempty"`
}

// Coordinates are fractions of actor height, relative to the feet. All bone
// parents precede their children; the owner validates and publishes once.
func DefaultRig(profile string) RigDefinition {
	r := RigDefinition{Profile: profile, Height: 4}
	r.Bones = [BoneCount]BoneDefinition{
		{Parent: -1, Offset: model.Vec{Y: -.5}, Size: model.Vec{X: .38, Y: .42}, Pivot: model.Vec{X: .5, Y: .5}},
		{Parent: 0, Offset: model.Vec{Y: -.31}, Size: model.Vec{X: .27, Y: .27}, Pivot: model.Vec{X: .5, Y: .5}},
		{Parent: 0, Offset: model.Vec{X: -.2, Y: -.16}, Size: model.Vec{X: .13, Y: .4}, Pivot: model.Vec{X: .5, Y: .04}},
		{Parent: 0, Offset: model.Vec{X: .2, Y: -.16}, Size: model.Vec{X: .13, Y: .4}, Pivot: model.Vec{X: .5, Y: .04}},
		{Parent: 0, Offset: model.Vec{X: -.1, Y: .18}, Size: model.Vec{X: .15, Y: .33}, Pivot: model.Vec{X: .5, Y: .04}},
		{Parent: 0, Offset: model.Vec{X: .1, Y: .18}, Size: model.Vec{X: .15, Y: .33}, Pivot: model.Vec{X: .5, Y: .04}},
	}
	r.DrawOrder = [DirectionCount][BoneCount]int{{4, 5, 0, 2, 3, 1}, {4, 5, 2, 3, 0, 1}, {4, 2, 0, 5, 1, 3}, {5, 3, 0, 4, 1, 2}}
	switch profile {
	case "core", "lens", "bag", "load", "encased", "leaker", "spitter", "array", "carrier":
		r.Height = 3.1
		r.Bones[0].Offset.Y = -.47
		r.Bones[0].Size = model.Vec{X: .55, Y: .59}
		r.Bones[1].Offset.Y = -.28
		r.Bones[1].Size = model.Vec{X: .29, Y: .23}
		for _, i := range []int{ArmLeft, ArmRight} {
			r.Bones[i].Size = model.Vec{X: .16, Y: .32}
			r.Bones[i].Offset.Y = -.08
			r.Bones[i].Offset.X *= 1.35
		}
		for _, i := range []int{LegLeft, LegRight} {
			r.Bones[i].Size = model.Vec{X: .18, Y: .23}
			r.Bones[i].Offset.Y = .24
		}
		if profile == "lens" || profile == "array" {
			r.Bones[0].Size = model.Vec{X: .62, Y: .51}
			r.Bones[0].Offset.Y = -.43
			// Optical front feature overlays its empty mounting ring.
			r.Bones[1].Offset = model.Vec{}
			r.Bones[1].Size = model.Vec{X: .51, Y: .46}
		}
		if profile == "leaker" || profile == "encased" {
			r.Bones[1].Offset.Y = -.34
			r.Bones[1].Size = model.Vec{X: .48, Y: .16}
		}
		if profile == "carrier" {
			r.Bones[0].Size = model.Vec{X: .62, Y: .64}
			r.Bones[1].Size = model.Vec{X: .42, Y: .25}
		}
	}
	return r
}

func ValidateRigLibrary(l RigLibrary) error {
	if l.Version != 1 || len(l.Rigs) > 512 || len(l.WeaponClips) > 4096 {
		return fmt.Errorf("invalid skeleton library budget or version")
	}
	for id, r := range l.Rigs {
		if id == "" || !finite(r.Height) || r.Height < .5 || r.Height > 8 || len(r.Clips) > 32 {
			return fmt.Errorf("invalid skeleton rig: %s", id)
		}
		for i, b := range r.Bones {
			if b.Parent < -1 || b.Parent >= i || !b.Offset.Finite() || !b.Size.Finite() || !b.Pivot.Finite() || math.Abs(b.Offset.X) > 2 || math.Abs(b.Offset.Y) > 2 || b.Size.X <= 0 || b.Size.Y <= 0 || b.Size.X > 2 || b.Size.Y > 2 || b.Pivot.X < 0 || b.Pivot.X > 1 || b.Pivot.Y < 0 || b.Pivot.Y > 1 {
				return fmt.Errorf("invalid skeleton bone %s/%d", id, i)
			}
		}
		for _, order := range r.DrawOrder {
			seen := [BoneCount]bool{}
			for _, i := range order {
				if i < 0 || i >= BoneCount || seen[i] {
					return fmt.Errorf("invalid skeleton draw order: %s", id)
				}
				seen[i] = true
			}
		}
		for _, c := range r.Clips {
			if !finite(c.Amplitude) || !finite(c.Speed) || c.Amplitude < 0 || c.Amplitude > 3 || c.Speed <= 0 || c.Speed > 4 {
				return fmt.Errorf("invalid skeleton clip: %s", id)
			}
		}
	}
	for _, clip := range l.WeaponClips {
		if clip != "melee" && clip != "pistol" && clip != "rifle" && clip != "cast" {
			return fmt.Errorf("invalid weapon animation class")
		}
	}
	return nil
}

// NormalizeRig supplies the standard anatomy when a content file only names a
// profile. Explicit bones and draw orders are complete fixed-size overrides.
func NormalizeRig(r RigDefinition) RigDefinition {
	base := DefaultRig(r.Profile)
	if r.Height > 0 {
		base.Height = r.Height
	}
	if r.Bones[0].Size.X > 0 {
		base.Bones = r.Bones
	}
	if r.DrawOrder[0][1] != 0 {
		base.DrawOrder = r.DrawOrder
	}
	base.Clips = r.Clips
	return base
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Direction is selected after projection, so a rotated camera changes the
// visible row without changing world movement or authoritative aim.
func ScreenDirection(v model.Vec) int {
	if math.Abs(v.Y) >= math.Abs(v.X) {
		if v.Y < 0 {
			return Back
		}
		return Front
	}
	if v.X < 0 {
		return Left
	}
	return Right
}

type SkeletonInput struct {
	Facing, Move                          model.Vec
	Walk, Time                            float64
	WeaponClass, Life                     string
	Attack, Skill, Dodge, Hit, LifeAction model.ActionAnimation
}
type BonePose struct {
	Position model.Vec
	Angle    float64
	Scale    model.Vec
}
type SkeletonPose struct {
	Bones       [BoneCount]BonePose
	Order       [BoneCount]int
	Direction   int
	Alpha, Hurt float64
}

func rotate(v model.Vec, a float64) model.Vec {
	s, c := math.Sincos(a)
	return model.Vec{X: v.X*c - v.Y*s, Y: v.X*s + v.Y*c}
}
func tuning(r RigDefinition, id string) ClipTuning {
	if c, ok := r.Clips[id]; ok {
		return c
	}
	return ClipTuning{Amplitude: 1, Speed: 1}
}

// SampleSkeleton is a fixed-size, allocation-free hierarchy evaluation. It
// never reads a clock, a mutable world, collision state or image pixels.
func SampleSkeleton(r RigDefinition, in SkeletonInput) SkeletonPose {
	dir := ScreenDirection(in.Facing)
	out := SkeletonPose{Direction: dir, Order: r.DrawOrder[dir], Alpha: 1}
	var angles [BoneCount]float64
	var offsets [BoneCount]model.Vec
	var scales [BoneCount]model.Vec
	for i := range scales {
		scales[i] = model.Vec{X: 1, Y: 1}
		offsets[i] = r.Bones[i].Offset
	}
	// Character-left is screen-right when facing the viewer.
	if dir == Front {
		for i := 1; i < BoneCount; i++ {
			offsets[i].X = -offsets[i].X
		}
	}
	if dir == Right || dir == Left {
		for i := 1; i < BoneCount; i++ {
			offsets[i].X *= .48
		}
		scales[Body].X = .83
	}
	move := tuning(r, "move")
	phase := in.Walk * 2 * math.Pi * move.Speed
	stride := min(1, in.Move.Length()/120) * move.Amplitude
	swing := math.Sin(phase) * stride
	angles[LegLeft], angles[LegRight] = .43*swing, -.43*swing
	angles[ArmLeft], angles[ArmRight] = -.25*swing, .25*swing
	offsets[Body].Y -= .018 * math.Abs(math.Sin(phase)) * stride
	if stride < .05 {
		offsets[Body].Y += .004 * math.Sin(in.Time*2.4)
	}
	if Active(in.Attack) {
		a := in.Attack
		t := max(0, min(1, a.Elapsed/max(.01, a.Duration)))
		clip := tuning(r, "attack:"+in.WeaponClass)
		pulse := math.Sin(math.Pi*t) * clip.Amplitude
		sign := 1.0
		if dir == Left {
			sign = -1
		}
		switch in.WeaponClass {
		case "melee":
			angles[ArmRight] = sign * (-.85 + 1.8*ease(max(0, min(1, (a.Elapsed-a.Windup)/max(.04, a.Active))))) * clip.Amplitude
			angles[Body] = sign * .07 * pulse
			angles[ArmLeft] = -sign * .3 * pulse
		case "cast":
			angles[ArmRight] = -sign * .95 * pulse
			angles[ArmLeft] = sign * .6 * pulse
		case "pistol":
			angles[ArmRight] = -sign * 1.15
			offsets[ArmRight].Y -= .025 * pulse
		default:
			angles[ArmRight] = -sign * .95
			angles[ArmLeft] = sign * .75
			offsets[Body].X -= sign * .025 * pulse
		}
	}
	if Active(in.Skill) {
		c := tuning(r, "skill")
		p := math.Sin(math.Pi*in.Skill.Elapsed/in.Skill.Duration) * c.Amplitude
		angles[ArmLeft] += .8 * p
		angles[ArmRight] -= .95 * p
		offsets[Body].Y -= .025 * p
	}
	if Active(in.Hit) {
		p := (1 - in.Hit.Elapsed/in.Hit.Duration) * tuning(r, "hit").Amplitude
		out.Hurt = p
		offsets[Body].X -= in.Hit.Direction.X * .035 * p
		angles[Head] = -.12 * p
	}
	rootAngle := 0.0
	if Active(in.Dodge) {
		t := in.Dodge.Elapsed / in.Dodge.Duration
		sign := 1.0
		if in.Dodge.Direction.X < 0 {
			sign = -1
		}
		rootAngle = sign * 2 * math.Pi * ease(t)
		offsets[Body].Y = -.36 - .1*math.Sin(math.Pi*t)
		angles[LegLeft], angles[LegRight] = -.75, .75
		angles[ArmLeft], angles[ArmRight] = .7, -.7
	}
	if in.Life == "down" || in.Life == "dead" {
		t := 1.0
		if Active(in.LifeAction) {
			t = ease(in.LifeAction.Elapsed / in.LifeAction.Duration)
		}
		rootAngle = 1.35 * t
		offsets[Body].Y = -.5 + .3*t
		angles[LegLeft], angles[LegRight] = -.35, .2
		angles[ArmLeft], angles[ArmRight] = .6, -.4
		if in.Life == "dead" {
			out.Alpha = 1 - ease(max(0, min(1, (in.LifeAction.Elapsed/max(.01, in.LifeAction.Duration)-.25)/.75)))
			if in.LifeAction.Duration <= 0 {
				out.Alpha = 0
			}
		}
	}
	for i, b := range r.Bones {
		pos, angle, scale := offsets[i], angles[i], scales[i]
		if b.Parent >= 0 {
			p := out.Bones[b.Parent]
			pos = p.Position.Add(rotate(model.Vec{X: pos.X * p.Scale.X, Y: pos.Y * p.Scale.Y}, p.Angle))
			angle += p.Angle
			scale.X *= p.Scale.X
			scale.Y *= p.Scale.Y
		}
		out.Bones[i] = BonePose{Position: pos, Angle: angle, Scale: scale}
	}
	if rootAngle != 0 {
		pivot := out.Bones[Body].Position
		for i := range out.Bones {
			out.Bones[i].Position = pivot.Add(rotate(out.Bones[i].Position.Sub(pivot), rootAngle))
			out.Bones[i].Angle += rootAngle
		}
	}
	return out
}
func ease(t float64) float64 { t = max(0, min(1, t)); return t * t * (3 - 2*t) }
