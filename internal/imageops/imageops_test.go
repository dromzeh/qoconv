package imageops

import (
	"image"
	"image/color"
	"testing"
)

func solid(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{255, 0, 0, 255})
		}
	}
	return img
}

func size(img image.Image) image.Point { return img.Bounds().Size() }

func TestSquare(t *testing.T) {
	// Centred: a 4x2 image gets a row above and below.
	sq := Square(solid(4, 2), true)
	if size(sq) != image.Pt(4, 4) {
		t.Fatalf("size = %v", size(sq))
	}
	if _, _, _, a := sq.At(0, 0).RGBA(); a != 0 {
		t.Error("top-left should be padding when centred")
	}
	if _, _, _, a := sq.At(0, 1).RGBA(); a == 0 {
		t.Error("art should start on row 1 when centred")
	}
	// Pinned: the art stays at the top-left (a cursor hotspot).
	sq = Square(solid(2, 4), false)
	if _, _, _, a := sq.At(0, 0).RGBA(); size(sq) != image.Pt(4, 4) || a == 0 {
		t.Errorf("pinned square = %v, top-left alpha %d", size(sq), a)
	}
}

func TestPadScaleRotateFlip(t *testing.T) {
	if got := size(Pad(solid(3, 3), 1, 2, 3, 4)); got != image.Pt(9, 7) {
		t.Errorf("Pad size = %v, want 9x7", got)
	}
	if got := size(Pad(solid(3, 3), -1, -1, 2, 0)); got != image.Pt(3, 5) {
		t.Errorf("Pad with negatives = %v, want 3x5", got)
	}
	if got := size(Scale(solid(433, 392), 28, 28)); got != image.Pt(28, 28) {
		t.Errorf("Scale size = %v", got)
	}
	if got := size(ScaleBy(solid(44, 70), 0.45)); got != image.Pt(20, 32) {
		t.Errorf("ScaleBy size = %v, want 20x32", got)
	}
	if got := size(Rotate90(solid(37, 600), true)); got != image.Pt(600, 37) {
		t.Errorf("Rotate90 size = %v", got)
	}

	src := image.NewNRGBA(image.Rect(0, 0, 1, 2))
	src.Set(0, 0, color.NRGBA{255, 255, 255, 255})
	if _, _, _, a := FlipVertical(src).At(0, 1).RGBA(); a == 0 {
		t.Error("FlipVertical didn't move the top pixel to the bottom")
	}
}

func TestSplitJoinRoundTrip(t *testing.T) {
	frames := SplitGrid(JoinRow([]image.Image{solid(10, 8), solid(6, 8), solid(10, 8)}), 1, 3)
	if len(frames) != 3 || size(frames[1]) != image.Pt(10, 8) {
		t.Fatalf("frames = %d, size %v", len(frames), size(frames[1]))
	}
	if _, _, _, a := frames[1].At(0, 0).RGBA(); a != 0 {
		t.Error("the narrower frame should be centred in its cell")
	}
}
