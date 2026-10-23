package client

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
	"image"
	"strings"
)

type skeletonRig struct {
	Definition animation.RigDefinition
	Parts      [animation.DirectionCount][animation.BoneCount]*ebiten.Image
	Rest       [animation.DirectionCount]animation.Bounds
}
type skeletonResources struct {
	Library animation.RigLibrary
	Rigs    map[string]skeletonRig
	Actions map[int]model.EnemyActionState
}

// Runs once before publication. GPU views refer to their one owned atlas;
// neither animation sampling nor rendering uploads or decodes any images.
func prepareSkeletons(decoded map[string]image.Image, uploaded map[string]*ebiten.Image, library animation.RigLibrary) (skeletonResources, error) {
	if library.Version == 0 {
		library.Version = 1
	}
	for id, r := range library.Rigs {
		library.Rigs[id] = animation.NormalizeRig(r)
	}
	if err := animation.ValidateRigLibrary(library); err != nil {
		return skeletonResources{}, err
	}
	out := skeletonResources{Library: library, Rigs: map[string]skeletonRig{}, Actions: map[int]model.EnemyActionState{}}
	for key, value := range decoded {
		if !strings.HasPrefix(key, "rig:") {
			continue
		}
		id := strings.TrimPrefix(key, "rig:")
		b := value.Bounds()
		if b.Dx()%animation.BoneCount != 0 || b.Dy()%animation.DirectionCount != 0 || b.Dx() < 96 || b.Dy() < 64 {
			return skeletonResources{}, fmt.Errorf("skeleton atlas must have 6 columns and 4 rows: %s", id)
		}
		r, ok := library.Rigs[id]
		if !ok {
			return skeletonResources{}, fmt.Errorf("missing skeleton definition: %s", id)
		}
		rig := skeletonRig{Definition: r}
		cw, ch := b.Dx()/animation.BoneCount, b.Dy()/animation.DirectionCount
		for row := 0; row < animation.DirectionCount; row++ {
			var pixels [animation.BoneCount]model.Vec
			for col := 0; col < animation.BoneCount; col++ {
				cell := image.Rect(b.Min.X+col*cw, b.Min.Y+row*ch, b.Min.X+(col+1)*cw, b.Min.Y+(row+1)*ch)
				region := animation.PartRegion(value, cell)
				if !region.Empty() {
					rig.Parts[row][col] = uploaded[key].SubImage(region).(*ebiten.Image)
					pixels[col] = model.Vec{X: float64(region.Dx()), Y: float64(region.Dy())}
				}
			}
			if rig.Parts[row][animation.Body] == nil {
				return skeletonResources{}, fmt.Errorf("empty skeleton body: %s/%d", id, row)
			}
			facing := [animation.DirectionCount]model.Vec{{Y: 1}, {Y: -1}, {X: 1}, {X: -1}}
			rig.Rest[row] = animation.PoseBounds(r, animation.SampleSkeleton(r, animation.SkeletonInput{Facing: facing[row]}), pixels)
		}
		out.Rigs[id] = rig
	}
	return out, nil
}
