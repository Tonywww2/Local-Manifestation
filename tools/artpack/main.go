// Artpack formats generated source artwork without changing its design or alpha.
// It also creates development contact sheets; sources are never overwritten.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func main() {
	input := flag.String("input", "", "source PNG/JPEG; with -sheet, a directory of runtime PNGs")
	output := flag.String("output", "", "new output PNG")
	width := flag.Int("width", 128, "runtime width")
	height := flag.Int("height", 128, "runtime height")
	mode := flag.String("mode", "cutout", "cutout, tile or cover")
	padding := flag.Float64("padding", .1, "transparent margin fraction for cutouts")
	sheet := flag.Bool("sheet", false, "compose a development preview of PNGs in the input directory")
	flag.Parse()
	if err := run(*input, *output, *width, *height, *mode, *padding, *sheet); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, output string, width, height int, mode string, padding float64, sheet bool) error {
	if input == "" || output == "" || width < 16 || height < 16 || width > 4096 || height > 4096 || padding < 0 || padding >= .45 {
		return fmt.Errorf("invalid artwork formatting arguments")
	}
	inPath, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	outPath, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if strings.EqualFold(inPath, outPath) {
		return fmt.Errorf("source and output must differ")
	}
	var value image.Image
	if sheet {
		value, err = contactSheet(input)
	} else {
		var source image.Image
		source, err = readImage(input)
		if err == nil {
			value, err = format(source, width, height, mode, padding)
		}
	}
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	err = png.Encode(f, value)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	data, err := os.ReadFile(output)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	return json.NewEncoder(os.Stdout).Encode(struct {
		File          string `json:"file"`
		Width, Height int
		SHA256        string `json:"sha256"`
	}{output, value.Bounds().Dx(), value.Bounds().Dy(), hex.EncodeToString(hash[:])})
}

func readImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	v, _, err := image.Decode(f)
	return v, err
}

func visibleBounds(src image.Image) (image.Rectangle, bool) {
	b := src.Bounds()
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	transparent := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			transparent = transparent || a < 0xffff
			if a > 8*257 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
	}
	return image.Rect(x0, y0, x1, y1), transparent
}

func format(src image.Image, width, height int, mode string, padding float64) (image.Image, error) {
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	b := src.Bounds()
	switch mode {
	case "cutout":
		var transparent bool
		b, transparent = visibleBounds(src)
		if b.Empty() || !transparent {
			return nil, fmt.Errorf("cutout requires genuine nonempty transparent source artwork")
		}
		// Preserve faint edge pixels while removing generated empty canvas margins.
		b = b.Inset(-2).Intersect(src.Bounds())
		scale := min(float64(width)*(1-2*padding)/float64(b.Dx()), float64(height)*(1-2*padding)/float64(b.Dy()))
		w, h := max(1, int(math.Round(float64(b.Dx())*scale))), max(1, int(math.Round(float64(b.Dy())*scale)))
		r := image.Rect((width-w)/2, (height-h)/2, (width-w)/2+w, (height-h)/2+h)
		xdraw.CatmullRom.Scale(dst, r, src, b, draw.Src, nil)
	case "tile":
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	case "cover":
		ratio := float64(width) / float64(height)
		if float64(b.Dx())/float64(b.Dy()) > ratio {
			w := int(float64(b.Dy()) * ratio)
			b.Min.X += (b.Dx() - w) / 2
			b.Max.X = b.Min.X + w
		} else {
			h := int(float64(b.Dx()) / ratio)
			b.Min.Y += (b.Dy() - h) / 2
			b.Max.Y = b.Min.Y + h
		}
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	default:
		return nil, fmt.Errorf("unknown format mode %q", mode)
	}
	return dst, nil
}

func contactSheet(root string) (image.Image, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "sources" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".png") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil || len(paths) == 0 {
		return nil, fmt.Errorf("contact sheet has no images: %v", err)
	}
	sort.Strings(paths)
	const cols, cellW, cellH = 8, 160, 184
	dst := image.NewNRGBA(image.Rect(0, 0, cols*cellW, ((len(paths)+cols-1)/cols)*cellH))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.NRGBA{48, 60, 64, 255}), image.Point{}, draw.Src)
	for i, path := range paths {
		src, err := readImage(path)
		if err != nil {
			return nil, err
		}
		x, y := (i%cols)*cellW, (i/cols)*cellH
		scale := min(136.0/float64(src.Bounds().Dx()), 144.0/float64(src.Bounds().Dy()))
		w, h := int(float64(src.Bounds().Dx())*scale), int(float64(src.Bounds().Dy())*scale)
		r := image.Rect(x+(cellW-w)/2, y+8+(144-h)/2, x+(cellW-w)/2+w, y+8+(144-h)/2+h)
		xdraw.CatmullRom.Scale(dst, r, src, src.Bounds(), draw.Over, nil)
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if len(name) > 23 {
			name = name[:23]
		}
		d := font.Drawer{Dst: dst, Src: image.NewUniform(color.NRGBA{227, 231, 225, 255}), Face: basicfont.Face7x13, Dot: fixed.P(x+6, y+172)}
		d.DrawString(name)
	}
	return dst, nil
}
