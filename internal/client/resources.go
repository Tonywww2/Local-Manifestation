package client

import (
	"bytes"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	textv2 "github.com/hajimehoshi/ebiten/v2/text/v2"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"
)

type resourceTheme struct {
	Panel      [4]uint8  `json:"panel"`
	Background [4]uint8  `json:"background"`
	Layout     ui.Layout `json:"layout"`
}
type resourceBundle struct {
	images    map[string]*ebiten.Image
	sprites   map[string]*ebiten.Image
	glyphs    map[string]*ebiten.Image
	boxes     map[boxStyle]*ebiten.Image
	sounds    map[string][]byte
	theme     resourceTheme
	audio     *audio.Context
	music     *audio.Player
	playing   []*audio.Player
	lastShot  int
	lastScene string
}

var defaultFontSource = fontSource

func (g *Game) publishResources(c *content.Catalog) error {
	decoded := map[string]image.Image{}
	sounds := map[string][]byte{}
	theme := resourceTheme{}
	font := defaultFontSource
	decodedBytes := int64(0)
	for _, id := range c.AssetIDs() {
		data := c.Asset(id)
		switch {
		case id == "ui:theme":
			if err := content.Decode(data, &theme); err != nil {
				return err
			}
			if err := ui.Validate(theme.Layout); err != nil {
				return err
			}
		case id == "ui:font":
			var err error
			font, err = textv2.NewGoTextFaceSource(bytes.NewReader(data))
			if err != nil {
				return err
			}
		case strings.HasPrefix(id, "audio:") || strings.HasPrefix(id, "music:"):
			stream, err := wav.DecodeWithSampleRate(48000, bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("%s requires valid WAV: %w", id, err)
			}
			pcm, err := io.ReadAll(io.LimitReader(stream, 64<<20+1))
			if err != nil || len(pcm) > 64<<20 {
				return fmt.Errorf("decoded audio too large")
			}
			sounds[id] = pcm
			decodedBytes += int64(len(pcm))
		default:
			config, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
				return fmt.Errorf("invalid image asset %s", id)
			}
			decodedBytes += int64(config.Width) * int64(config.Height) * 4
			if decodedBytes > 256<<20 {
				return fmt.Errorf("decoded resource budget exceeded")
			}
			value, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				return err
			}
			decoded[id] = value
		}
		if decodedBytes > 256<<20 {
			return fmt.Errorf("decoded resource budget exceeded")
		}
	}
	next := resourceBundle{images: map[string]*ebiten.Image{}, sprites: map[string]*ebiten.Image{}, sounds: sounds, theme: theme, audio: g.resources.audio}
	for id, value := range decoded {
		next.images[id] = ebiten.NewImageFromImage(value)
		if strings.HasPrefix(id, "player:") || strings.HasPrefix(id, "portrait:") {
			r := spriteRegion(value)
			if !r.Empty() {
				next.sprites[id] = next.images[id].SubImage(r).(*ebiten.Image)
			}
		}
	}
	if g.resources.music != nil {
		g.resources.music.Close()
	}
	for _, p := range g.resources.playing {
		p.Close()
	}
	for _, value := range g.resources.images {
		value.Deallocate()
	}
	for _, value := range g.resources.glyphs {
		value.Deallocate()
	}
	for _, value := range g.resources.boxes {
		value.Deallocate()
	}
	if len(sounds) > 0 && next.audio == nil {
		next.audio = audio.NewContext(48000)
	}
	g.resources = next
	for id, surface := range g.roomSurfaces {
		if surface.Image != nil {
			surface.Image.Deallocate()
		}
		delete(g.roomSurfaces, id)
	}
	if g.front.Surface.Image != nil {
		g.front.Surface.Image.Deallocate()
		g.front.Surface = roomSurface{}
	}
	fontSource = font
	return nil
}

// Find the visible cutout once during loading. Original asset files and alpha
// remain intact; rendering uses a cached region with its original aspect ratio.
func spriteRegion(value image.Image) image.Rectangle {
	r := value.Bounds()
	nrgba, packed := value.(*image.NRGBA)
	x0, y0, x1, y1 := r.Max.X, r.Max.Y, r.Min.X, r.Min.Y
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			var a uint32
			if packed {
				a = uint32(nrgba.Pix[nrgba.PixOffset(x, y)+3]) * 257
			} else {
				_, _, _, a = value.At(x, y).RGBA()
			}
			if a > 256 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
	}
	if x1 <= x0 || y1 <= y0 {
		return image.Rectangle{}
	}
	return image.Rect(x0, y0, x1, y1)
}

func (g *Game) drawSprite(dst *ebiten.Image, id string, center model.Vec, width, height float64) bool {
	value := g.resources.sprites[id]
	if value == nil {
		return false
	}
	r := value.Bounds()
	scale := min(width/float64(r.Dx()), height/float64(r.Dy()))
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(center.X-float64(r.Dx())*scale/2, center.Y-float64(r.Dy())*scale/2)
	dst.DrawImage(value, op)
	return true
}
func (g *Game) drawAsset(dst *ebiten.Image, id string, center model.Vec, width, height float64) bool {
	value := g.resources.images[id]
	if value == nil {
		return false
	}
	r := value.Bounds()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(width/float64(r.Dx()), height/float64(r.Dy()))
	op.GeoM.Translate(center.X-width/2, center.Y-height/2)
	dst.DrawImage(value, op)
	return true
}
func (g *Game) panelColor() color.RGBA {
	v := g.resources.theme.Panel
	if v[3] == 0 {
		return panel
	}
	return color.RGBA{v[0], v[1], v[2], v[3]}
}
func (g *Game) playSound(key string) {
	r := &g.resources
	if r.audio == nil || len(r.sounds[key]) == 0 {
		return
	}
	kept := r.playing[:0]
	for _, p := range r.playing {
		if p.IsPlaying() {
			kept = append(kept, p)
		} else {
			p.Close()
		}
	}
	r.playing = kept
	if len(r.playing) >= 32 {
		return
	}
	p := r.audio.NewPlayerFromBytes(r.sounds[key])
	p.Play()
	r.playing = append(r.playing, p)
}
func (g *Game) updateAudio() {
	r := &g.resources
	active := g.state != nil && g.state.Phase != "won" && g.state.Phase != "lost"
	if !active {
		if r.music != nil {
			r.music.Close()
			r.music = nil
		}
		return
	}
	if r.music == nil && r.audio != nil && len(r.sounds["music:adventure"]) > 0 {
		pcm := r.sounds["music:adventure"]
		p, err := r.audio.NewPlayer(audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm))))
		if err == nil {
			r.music = p
			p.Play()
		}
	}
	if scene := g.currentScene(); scene != nil {
		if scene.ID != r.lastScene {
			r.lastScene = scene.ID
			r.lastShot = 0
		}
		newest := r.lastShot
		for _, shot := range scene.Projectiles {
			if shot.Owner == g.id && shot.ID > r.lastShot {
				g.playSound("audio:attack")
				newest = max(newest, shot.ID)
			}
		}
		r.lastShot = newest
	}
}
