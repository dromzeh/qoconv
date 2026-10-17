// Package imageops provides the PNG transforms qoconv needs: decode/encode,
// resampling, padding, 90° rotation (health bars), flipping (LN tails), and
// spritesheet splitting/joining (lighting and judgement animations).
package imageops

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"regexp"
	"strconv"

	xdraw "golang.org/x/image/draw"
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

// Blank returns a w×h fully transparent image.
func Blank(w, h int) image.Image { return image.NewNRGBA(image.Rect(0, 0, w, h)) }

// toNRGBA returns src as a zero-origin *image.NRGBA, copying only if needed.
func toNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Rect, src, b.Min, draw.Src)
	return dst
}

// Rotate90 rotates src by 90°, clockwise when cw is true. Quaver health bars are
// vertical; osu! scorebars are horizontal, so they must be rotated on conversion.
func Rotate90(src image.Image, cw bool) image.Image {
	s := toNRGBA(src)
	w, h := s.Rect.Dx(), s.Rect.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := y, w-1-x
			if cw {
				dx, dy = h-1-y, x
			}
			copy(dst.Pix[dst.PixOffset(dx, dy):][:4], s.Pix[s.PixOffset(x, y):][:4])
		}
	}
	return dst
}

// FlipVertical mirrors src top-to-bottom. Quaver's long-note end image is
// oriented opposite to osu!'s NoteImage#T, so the tail must be flipped.
func FlipVertical(src image.Image) image.Image {
	s := toNRGBA(src)
	h := s.Rect.Dy()
	dst := image.NewNRGBA(s.Rect)
	for y := 0; y < h; y++ {
		copy(dst.Pix[y*dst.Stride:(y+1)*dst.Stride], s.Pix[(h-1-y)*s.Stride:(h-y)*s.Stride])
	}
	return dst
}

// Scale resamples src to w×h with a Catmull-Rom filter, which stays sharp when
// enlarging and anti-aliases properly when shrinking (e.g. a 433px cursor down
// to 28px). Filtering happens in premultiplied alpha so transparent edges don't
// pick up dark fringes. A zero-sized source or target yields a blank image.
func Scale(src image.Image, w, h int) image.Image {
	w, h = max(w, 0), max(h, 0)
	b := src.Bounds()
	if b.Dx() == w && b.Dy() == h {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if b.Empty() || w == 0 || h == 0 {
		return dst
	}
	xdraw.CatmullRom.Scale(dst, dst.Rect, src, b, xdraw.Src, nil)
	return dst
}

// ScaleBy resamples src by factor f, rounding each side to at least 1px.
func ScaleBy(src image.Image, f float64) image.Image {
	b := src.Bounds()
	return Scale(src, max(1, round(float64(b.Dx())*f)), max(1, round(float64(b.Dy())*f)))
}

// Pad returns src surrounded by transparent margins (negative values count as 0).
func Pad(src image.Image, top, right, bottom, left int) image.Image {
	top, right, bottom, left = max(top, 0), max(right, 0), max(bottom, 0), max(left, 0)
	if top|right|bottom|left == 0 {
		return src
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, left+b.Dx()+right, top+b.Dy()+bottom))
	draw.Draw(dst, image.Rect(left, top, left+b.Dx(), top+b.Dy()), src, b.Min, draw.Src)
	return dst
}

// Square pads src with transparency to a square canvas. Centered keeps the
// art in the middle; otherwise it stays pinned to the top-left corner, which
// preserves where a non-centred cursor's hotspot sits.
func Square(src image.Image, centered bool) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dx, dy := max(h-w, 0), max(w-h, 0)
	if centered {
		return Pad(src, dy/2, dx-dx/2, dy-dy/2, dx/2)
	}
	return Pad(src, 0, dx, dy, 0)
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
		cw = max(cw, f.Bounds().Dx())
		ch = max(ch, f.Bounds().Dy())
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
			draw.Draw(fr, fr.Rect, src, image.Pt(b.Min.X+c*fw, b.Min.Y+r*fh), draw.Src)
			frames = append(frames, fr)
		}
	}
	return frames
}

func round(f float64) int {
	if f < 0 {
		return -int(-f + 0.5)
	}
	return int(f + 0.5)
}
