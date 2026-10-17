package convert

import (
	"math"
	"testing"
)

// Golden values worked out by hand from a representative 4K skin's [4K] section
// (ColumnSize=140, HitPosOffsetY=176, ReceptorPosOffsetY=0, ComboPosY=-120,
// JudgementBurstPosY=-85, ColumnAlignment=0) with a 250x400 receptor.
func TestCoordsGolden(t *testing.T) {
	rec := QuaverReceptor{ColumnSize: 140, TexW: 250, TexH: 400}

	if got := roundI(ColumnWidthF(140)); got != 88 { // 87.5
		t.Errorf("ColumnWidth = %d, want 88", got)
	}
	if got := ComboPosition(-120); got != 165 {
		t.Errorf("ComboPosition = %d, want 165", got)
	}
	if got := ScorePosition(-85); got != 187 {
		t.Errorf("ScorePosition = %d, want 187", got)
	}
	// Note-landing line (note bottom at hit time) = receptor top + HitPosOffsetY:
	// receptor 250x400 scaled to 140 wide -> 224 tall; 768 - 0 - 224 + 176 = 720
	// -> 450 in osu! 480-space.
	if got := HitPosition(rec, 176); got != 450 {
		t.Errorf("HitPosition = %d, want 450", got)
	}
	// HitPosOffsetY=0 (Bar default): note bottom on the receptor top -> 340.
	if got := HitPosition(rec, 0); got != 340 {
		t.Errorf("HitPosition (offset 0) = %d, want 340", got)
	}
	if got := HitPosition(QuaverReceptor{}, 0); got != OsuDefaultHitPosition {
		t.Errorf("HitPosition fallback = %d, want %d", got, OsuDefaultHitPosition)
	}
	if got := HitPosition(rec, 600); got != 480 { // osu! clamps to 240..480
		t.Errorf("HitPosition clamp = %d, want 480", got)
	}
	// Column lighting sits on the receptor top (544), minus ColumnLightingOffsetY.
	if got := LightPosition(rec, 0); got != 340 {
		t.Errorf("LightPosition = %d, want 340", got)
	}
	if got := LightPosition(rec, 32); got != 320 {
		t.Errorf("LightPosition (offset 32) = %d, want 320", got)
	}
	if got := ReceptorBottomPad(24); got != 24 {
		t.Errorf("ReceptorBottomPad(24) = %d, want 24", got)
	}
	if got := ReceptorBottomPad(-8); got != 0 {
		t.Errorf("ReceptorBottomPad(-8) = %d, want 0", got)
	}
	// A centred 4×88 stage on the 853.3-wide screen: 426.7 - 176.
	if got := ColumnStart(0, 88, 0, 4); got != 251 {
		t.Errorf("ColumnStart = %d, want 251", got)
	}
	// Spacing widens the stage symmetrically; alignment shifts it by /1.6.
	if got := ColumnStart(80, 88, 10, 4); got != 286 { // 426.7 + 50 - 191
		t.Errorf("ColumnStart (aligned, spaced) = %d, want 286", got)
	}
	// WidthForNoteHeightScale is in Quaver's 1080 units: 180 -> 80.
	if got := NoteHeightScale(180); got != 80 {
		t.Errorf("NoteHeightScale = %d, want 80", got)
	}
	// 100% lighting: one texture pixel is 0.711 of a 768 unit -> width 21.3.
	if got := LightingWidth(100); math.Abs(got-21.33) > 0.01 {
		t.Errorf("LightingWidth = %.2f, want 21.33", got)
	}
}

// Every reverse mapping must invert its forward counterpart.
func TestReverseCoordsRoundTrip(t *testing.T) {
	if got := QuaverColumnSize(88); got != 140.8 {
		t.Errorf("QuaverColumnSize = %v, want 140.8", got)
	}
	for _, comboPosY := range []float64{-120, -40, 0, 96} {
		if got := QuaverComboPosY(float64(ComboPosition(comboPosY))); math.Abs(got-comboPosY) > R/2 {
			t.Errorf("ComboPosY round-trip: %v -> %v", comboPosY, got)
		}
	}
	for _, align := range []float64{-200, 0, 80} {
		start := ColumnStart(align, 88, 10, 4)
		if got := QuaverColumnAlignment(float64(start), 88, 10, 4); math.Abs(got-align) > R {
			t.Errorf("ColumnAlignment round-trip: %v -> %v", align, got)
		}
	}
	// Receptor drawn 224 tall with the line at 450*1.6=720 from the top:
	// 720 - 768 + 224 = 176, the forward golden HitPosOffsetY.
	if got := QuaverHitPosOffsetY(450, 224); got != 176 {
		t.Errorf("QuaverHitPosOffsetY = %v, want 176", got)
	}
	if got := QuaverColumnLightingOffsetY(320, 224); math.Abs(got-32) > 1e-9 {
		t.Errorf("QuaverColumnLightingOffsetY = %v, want 32", got)
	}
	if got := QuaverNoteHeightScale(80); got != 180 {
		t.Errorf("QuaverNoteHeightScale = %d, want 180", got)
	}
	for _, s := range []int{50, 100, 150} {
		if got := QuaverLightingScale(LightingWidth(s)); got != s {
			t.Errorf("lighting scale round-trip: %d -> %d", s, got)
		}
	}
}

// The exported layout helpers are shared with the preview and tuning code;
// pin their geometry.
func TestQuaverGeometry(t *testing.T) {
	rec := QuaverReceptor{ColumnSize: 140, ReceptorPosOffsetY: 24, TexW: 250, TexH: 400}
	if !rec.Known() || rec.Height() != 224 || rec.Top() != 520 { // 768 - 24 - 224
		t.Errorf("receptor: known %v, height %v, top %v", rec.Known(), rec.Height(), rec.Top())
	}
	// Without a texture the receptor has no height, so its top is its bottom.
	if bare := (QuaverReceptor{ColumnSize: 140, ReceptorPosOffsetY: 24}); bare.Known() || bare.Height() != 0 || bare.Top() != 744 {
		t.Errorf("bare receptor: known %v, height %v, top %v", bare.Known(), bare.Height(), bare.Top())
	}

	// 4 lanes of 160 with 16 between and 20 of padding each side: 728 wide,
	// centred on 682.67 and shifted 32 right.
	s := QuaverStage{Keys: 4, ColumnSize: 160, NotePadding: 16, StageReceptorPadding: 20, ColumnAlignment: 32}
	if s.Width() != 728 {
		t.Errorf("stage width = %v, want 728", s.Width())
	}
	if want := 768*16.0/9/2 + 32 - 364; math.Abs(s.Left()-want) > 1e-9 {
		t.Errorf("stage left = %v, want %v", s.Left(), want)
	}
	// Padding moves the borders, not the lanes.
	unpadded := s
	unpadded.StageReceptorPadding = 0
	for i := range 4 {
		if math.Abs(s.ColumnX(i)-unpadded.ColumnX(i)) > 1e-9 || math.Abs(s.ColumnX(i)-s.ColumnX(0)-float64(i)*176) > 1e-9 {
			t.Errorf("lane %d at %v (unpadded %v)", i, s.ColumnX(i), unpadded.ColumnX(i))
		}
	}
	// The stage the forward conversion centres for osu! is the same stage.
	if got := ColumnStart(32, 100, 10, 4); math.Abs(float64(got)-unpadded.ColumnX(0)/R) > 0.5 {
		t.Errorf("ColumnStart %d vs Quaver lane 1 at %v", got, unpadded.ColumnX(0)/R)
	}

	if got := QuaverPixels(1080); got != 768 {
		t.Errorf("QuaverPixels(1080) = %v, want 768", got)
	}
	if got := ReceptorUpPath(7, 3); got != "7k/Receptors/receptor-up-3.png" {
		t.Errorf("ReceptorUpPath = %q", got)
	}
}

func TestClampHitPosition(t *testing.T) {
	for in, want := range map[int]int{100: 240, 240: 240, 402: 402, 480: 480, 600: 480} {
		if got := ClampHitPosition(in); got != want {
			t.Errorf("ClampHitPosition(%d) = %d, want %d", in, got, want)
		}
	}
	// Tuned values may be fractional; the clamp keeps them so.
	if got := ClampHitPosition(402.5); got != 402.5 {
		t.Errorf("ClampHitPosition(402.5) = %v", got)
	}
	if got := ClampHitPosition(-1.5); got != 240 {
		t.Errorf("ClampHitPosition(-1.5) = %v", got)
	}
}
