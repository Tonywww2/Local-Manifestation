// enemy-collision calibrates the authored whole-body circles from the same
// cropped parts, pivots and pose transforms used by the client. No GPU is needed.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"

	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/animation"
	"hsmultiplayer/pkg/model"
)

type measurement struct {
	ID           string                                     `json:"id"`
	VisualRadius float64                                    `json:"visual_radius"`
	Radius       float64                                    `json:"radius"`
	Rest         [animation.DirectionCount]animation.Bounds `json:"rest_bounds"`
}

func main() {
	report := flag.String("report", "", "optional geometry report JSON")
	check := flag.Bool("check", false, "fail if authored radii differ from calibration")
	flag.Parse()
	c, err := content.Load(os.DirFS("content/base"))
	must(err)
	var library animation.RigLibrary
	must(json.Unmarshal(c.Asset("animation:rigs"), &library))
	for id, r := range library.Rigs {
		library.Rigs[id] = animation.NormalizeRig(r)
	}
	must(animation.ValidateRigLibrary(library))
	var definitions struct {
		Enemies []model.EnemyDef `json:"enemies"`
	}
	data, err := os.ReadFile("content/base/definitions.json")
	must(err)
	must(json.Unmarshal(data, &definitions))
	var rows []measurement
	for _, enemy := range definitions.Enemies {
		rig, ok := library.Rigs[enemy.ID]
		if !ok {
			panic("missing rig: " + enemy.ID)
		}
		atlas, _, err := image.Decode(bytes.NewReader(c.Asset("rig:" + enemy.ID)))
		must(err)
		b := atlas.Bounds()
		cw, ch := b.Dx()/animation.BoneCount, b.Dy()/animation.DirectionCount
		visual := c.EnemyVisualRadius(enemy.ID, enemy.Radius)
		row := measurement{ID: enemy.ID, VisualRadius: visual}
		facing := [animation.DirectionCount]model.Vec{{Y: 1}, {Y: -1}, {X: 1}, {X: -1}}
		for direction := range facing {
			var pixels [animation.BoneCount]model.Vec
			for i := range pixels {
				cell := image.Rect(b.Min.X+i*cw, b.Min.Y+direction*ch, b.Min.X+(i+1)*cw, b.Min.Y+(direction+1)*ch)
				region := animation.PartRegion(atlas, cell)
				pixels[i] = model.Vec{X: float64(region.Dx()), Y: float64(region.Dy())}
			}
			in := animation.SkeletonInput{Facing: facing[direction]}
			row.Rest[direction] = animation.PoseBounds(rig, animation.SampleSkeleton(rig, in), pixels)
			center := row.Rest[direction].Center()
			// Fixed collision covers idle, a complete walk cycle, both enemy
			// attack classes, and the initial hit recoil in every direction.
			for _, class := range []string{"", "melee", "cast"} {
				for step := 0; step <= 32; step++ {
					t := float64(step) / 32
					in := animation.SkeletonInput{Facing: facing[direction], Walk: t, Time: t, Move: model.Vec{X: 120}, WeaponClass: class}
					if class != "" {
						in.Attack = model.ActionAnimation{Duration: 1, Windup: .3, Active: .2, Elapsed: t}
					}
					for _, recoil := range []model.Vec{{}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
						in.Hit = model.ActionAnimation{}
						if recoil != (model.Vec{}) {
							in.Hit = model.ActionAnimation{Direction: recoil, Duration: .2}
						}
						pose := animation.SampleSkeleton(rig, in)
						for i, bone := range rig.Bones {
							if pixels[i].X <= 0 || pixels[i].Y <= 0 {
								continue
							}
							for _, point := range animation.PartCorners(bone, pose.Bones[i], pixels[i]) {
								row.Radius = max(row.Radius, point.Sub(center).Length()*visual*rig.Height)
							}
						}
					}
				}
			}
		}
		row.Radius = math.Ceil(row.Radius + 1)
		if *check && (enemy.VisualRadius <= 0 || enemy.Radius != row.Radius) {
			panic(fmt.Sprintf("%s: authored %g, calibrated %g", enemy.ID, enemy.Radius, row.Radius))
		}
		fmt.Printf("%s: visual_radius=%g radius=%g\n", row.ID, row.VisualRadius, row.Radius)
		rows = append(rows, row)
	}
	if *report != "" {
		data, err := json.MarshalIndent(rows, "", "  ")
		must(err)
		must(os.WriteFile(*report, append(data, '\n'), 0644))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
