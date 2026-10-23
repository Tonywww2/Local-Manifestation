package animation

import (
	"hsmultiplayer/pkg/model"
	"image"
)

// PartRegion excludes transparent atlas padding. Loading and offline geometry
// calibration use exactly the same visible region; rendering never scans pixels.
func PartRegion(value image.Image, cell image.Rectangle) image.Rectangle {
	x0, y0, x1, y1 := cell.Max.X, cell.Max.Y, cell.Min.X, cell.Min.Y
	packed, ok := value.(*image.NRGBA)
	for y := cell.Min.Y; y < cell.Max.Y; y++ {
		for x := cell.Min.X; x < cell.Max.X; x++ {
			var a uint32
			if ok {
				a = uint32(packed.Pix[packed.PixOffset(x, y)+3]) * 257
			} else {
				_, _, _, a = value.At(x, y).RGBA()
			}
			if a > 32*257 {
				x0, y0 = min(x0, x), min(y0, y)
				x1, y1 = max(x1, x+1), max(y1, y+1)
			}
		}
	}
	if x1 <= x0 || y1 <= y0 {
		return image.Rectangle{}
	}
	return image.Rect(x0, y0, x1, y1)
}

// PartSize preserves the cropped texture's aspect ratio in normalized rig units.
func PartSize(b BoneDefinition, pose BonePose, pixels model.Vec) model.Vec {
	if pixels.X <= 0 || pixels.Y <= 0 {
		return model.Vec{}
	}
	scale := min(b.Size.X/pixels.X, b.Size.Y/pixels.Y)
	return model.Vec{X: pixels.X * scale * pose.Scale.X, Y: pixels.Y * scale * pose.Scale.Y}
}

type Bounds struct{ Min, Max model.Vec }

func (b Bounds) Center() model.Vec { return b.Min.Add(b.Max).Scale(.5) }

// PartCorners matches the render transform, including parent pose and pivot.
func PartCorners(b BoneDefinition, pose BonePose, pixels model.Vec) [4]model.Vec {
	size := PartSize(b, pose, pixels)
	points := [4]model.Vec{{}, {X: size.X}, size, {Y: size.Y}}
	pivot := model.Vec{X: b.Pivot.X * size.X, Y: b.Pivot.Y * size.Y}
	for i := range points {
		points[i] = pose.Position.Add(rotate(points[i].Sub(pivot), pose.Angle))
	}
	return points
}

func PoseBounds(r RigDefinition, p SkeletonPose, pixels [BoneCount]model.Vec) Bounds {
	out, first := Bounds{}, true
	for i, b := range r.Bones {
		if pixels[i].X <= 0 || pixels[i].Y <= 0 {
			continue
		}
		for _, point := range PartCorners(b, p.Bones[i], pixels[i]) {
			if first {
				out.Min, out.Max, first = point, point, false
			}
			out.Min.X, out.Min.Y = min(out.Min.X, point.X), min(out.Min.Y, point.Y)
			out.Max.X, out.Max.Y = max(out.Max.X, point.X), max(out.Max.Y, point.Y)
		}
	}
	return out
}
