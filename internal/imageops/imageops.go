// Package imageops provides the PNG transforms qoconv needs: decode/encode,
// 90° rotation (health bars), spritesheet splitting (lighting/animation), and
// scaling. Everything is standard-library only — no external image deps.
package imageops

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"regexp"
	"strconv"
)

// Decode parses PNG bytes into an image.
func Decode(data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
}

// Encode writes an image as PNG bytes.
func Encode(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Size returns a PNG's dimensions without decoding the full image.
func Size(data []byte) (w, h int, err error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// Rotate90 rotates src by 90°, clockwise when cw is true. Quaver health bars are
// vertical; osu! scorebars are horizontal, so they must be rotated on conversion.
func Rotate90(src image.Image, cw bool) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			if cw {
				dst.Set(h-1-y, x, c)
			} else {
				dst.Set(y, w-1-x, c)
			}
		}
	}
	return dst
}

// FlipVertical mirrors src top-to-bottom. Quaver's long-note end image is
// oriented opposite to osu!'s NoteImage#T, so the tail must be flipped.
func FlipVertical(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// Scale resizes src to w×h using bilinear sampling.
func Scale(src image.Image, w, h int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	if sw == 0 || sh == 0 || w == 0 || h == 0 {
		return dst
	}
	for y := 0; y < h; y++ {
		fy := (float64(y) + 0.5) * float64(sh) / float64(h)
		sy := int(fy)
		if sy >= sh {
			sy = sh - 1
		}
		for x := 0; x < w; x++ {
			fx := (float64(x) + 0.5) * float64(sw) / float64(w)
			sx := int(fx)
			if sx >= sw {
				sx = sw - 1
			}
			dst.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}

// PadBottom returns a copy of src with px transparent rows added below it
// (same width). Used to position osu!'s bottom-anchored receptors.
func PadBottom(src image.Image, px int) image.Image {
	if px <= 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h+px))
	draw.Draw(dst, image.Rect(0, 0, w, h), src, b.Min, draw.Src)
	return dst
}

var sheetRe = regexp.MustCompile(`(?i)^(.*)@(\d+)x(\d+)\.png$`)

// ParseSheetName extracts the base name and grid (rows × cols) from Quaver's
// spritesheet filename convention "name@{rows}x{cols}.png".
func ParseSheetName(filename string) (base string, rows, cols int, ok bool) {
	m := sheetRe.FindStringSubmatch(filename)
	if m == nil {
		return "", 0, 0, false
	}
	rows, _ = strconv.Atoi(m[2])
	cols, _ = strconv.Atoi(m[3])
	if rows < 1 || cols < 1 {
		return "", 0, 0, false
	}
	return m[1], rows, cols, true
}

// JoinRow composes frames into a single-row spritesheet (Quaver's "@1xN"
// convention). Cells are all sized to the largest frame so SplitGrid can slice
// the sheet back evenly; smaller frames are centred in their cell.
func JoinRow(frames []image.Image) image.Image {
	cw, ch := 0, 0
	for _, f := range frames {
		if w := f.Bounds().Dx(); w > cw {
			cw = w
		}
		if h := f.Bounds().Dy(); h > ch {
			ch = h
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, cw*len(frames), ch))
	for i, f := range frames {
		b := f.Bounds()
		x := i*cw + (cw-b.Dx())/2
		y := (ch - b.Dy()) / 2
		draw.Draw(dst, image.Rect(x, y, x+b.Dx(), y+b.Dy()), f, b.Min, draw.Src)
	}
	return dst
}

// SplitGrid slices src into rows*cols equal frames in row-major order.
func SplitGrid(src image.Image, rows, cols int) []image.Image {
	b := src.Bounds()
	fw, fh := b.Dx()/cols, b.Dy()/rows
	frames := make([]image.Image, 0, rows*cols)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			fr := image.NewNRGBA(image.Rect(0, 0, fw, fh))
			for y := 0; y < fh; y++ {
				for x := 0; x < fw; x++ {
					fr.Set(x, y, src.At(b.Min.X+c*fw+x, b.Min.Y+r*fh+y))
				}
			}
			frames = append(frames, fr)
		}
	}
	return frames
}
