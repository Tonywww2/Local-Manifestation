package client

import (
	"math"
	"testing"

	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

func TestProjectileMeshCoverageAndReuse(t *testing.T) {
	shape := projectileAppearance(model.Projectile{Kind: "enemy", DamageType: "fire"}, 2)
	center := model.Vec{X: 100, Y: 80}
	m := newProjectileMesh(projectileBatchQuads)
	allocations := testing.AllocsPerRun(3, func() {
		m.vertices = m.vertices[:0]
		for range projectileBatchQuads {
			m.append(center, shape)
		}
	})
	if allocations != 0 {
		t.Fatalf("staging a full batch allocated %v times", allocations)
	}
	if len(m.vertices) != projectileBatchQuads*4 || len(m.indices) != projectileBatchQuads*6 {
		t.Fatal("full batch lost vertices or triangle indices")
	}
	for _, index := range m.indices {
		if int(index) >= len(m.vertices) {
			t.Fatal("triangle references a vertex outside the transfer")
		}
	}
	for _, v := range m.vertices[:4] {
		// One quad must contain both the disk and the outer ring, including
		// antialias coverage. All fragments receive the same shape and tint.
		if math.Abs(float64(v.SrcX)) < shape.ring+shape.width/2+.5 || v.Custom0 != 8 || v.Custom1 != 12 || v.Custom2 != 3 || v.Custom3 != 1 || v.ColorR != float32(241)/255 {
			t.Fatal("compound projectile shape or tint was truncated", v)
		}
		if v.DstX-v.SrcX != float32(center.X) || v.DstY-v.SrcY != float32(center.Y) {
			t.Fatal("local shader coordinates do not match screen projection")
		}
	}
	edge := model.Vec{X: -shape.extent(), Y: 40}
	if !projectileVisible(edge, shape.extent(), 1280, 800) || projectileVisible(edge.Sub(model.Vec{X: .01}), shape.extent(), 1280, 800) {
		t.Fatal("screen culling clips a partially visible ring or keeps an outside ring")
	}
}

func TestActorDepthOrderAfterRotation(t *testing.T) {
	s := protocol.Scene{
		Enemies: []model.Enemy{{Position: model.Vec{X: 10}}, {Position: model.Vec{X: -10}}},
		Players: []model.Player{{Position: model.Vec{X: 10}}},
	}
	c := Camera{Scale: 2, Angle: math.Pi / 2, Origin: model.Vec{X: 50, Y: 50}}
	scratch := make([]actorSprite, 0, 3)
	entries := sortedActorSprites(scratch, s, c)
	if entries[0].index != 1 || entries[0].player || entries[1].index != 0 || entries[1].player || !entries[2].player {
		t.Fatal("rotation or stable depth ordering changed", entries)
	}
	if entries[0].position != c.Project(s.Enemies[1].Position) || s.Enemies[1].Position.X != -10 {
		t.Fatal("projection changed the authoritative snapshot")
	}
	again := sortedActorSprites(entries, s, c)
	if &again[0] != &entries[0] || len(again) != 3 {
		t.Fatal("actor staging buffer was not reused")
	}
}
