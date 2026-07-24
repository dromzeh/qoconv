package convert

import "testing"

// Golden values worked out by hand from a representative 4K skin's [4K] section
// (ColumnSize=140, HitPosOffsetY=176, ReceptorPosOffsetY=0, ComboPosY=-120,
// JudgementBurstPosY=-85, ColumnAlignment=0). See the plan's math table.
func TestCoordsGolden(t *testing.T) {
	const (
		columnSize = 140
		keys       = 4
	)
	colW := ColumnWidthF(columnSize)

	if got := roundI(colW); got != 88 {
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
	if got := HitPosition(0, 176, columnSize, 250, 400); got != 450 {
		t.Errorf("HitPosition = %d, want 450", got)
	}
	// HitPosOffsetY=0 (Bar default): note bottom on the receptor top -> 340.
	if got := HitPosition(0, 0, columnSize, 250, 400); got != 340 {
		t.Errorf("HitPosition (offset 0) = %d, want 340", got)
	}
	// Unknown receptor size -> osu! default.
	if got := HitPosition(0, 0, 0, 0, 0); got != OsuDefaultHitPosition {
		t.Errorf("HitPosition fallback = %d, want %d", got, OsuDefaultHitPosition)
	}
	// osu! clamps HitPosition to 240..480.
	if got := HitPosition(0, 600, columnSize, 250, 400); got != 480 {
		t.Errorf("HitPosition clamp = %d, want 480", got)
	}
	// Receptor bottom-pad replicates Quaver's ReceptorPosOffsetY (never negative).
	if got := ReceptorBottomPad(0); got != 0 {
		t.Errorf("ReceptorBottomPad(0) = %d, want 0", got)
	}
	if got := ReceptorBottomPad(24); got != 24 {
		t.Errorf("ReceptorBottomPad(24) = %d, want 24", got)
	}
	if got := ReceptorBottomPad(-8); got != 0 {
		t.Errorf("ReceptorBottomPad(-8) = %d, want 0", got)
	}
	if got := ColumnStart(0, columnSize, keys, colW); got != 252 {
		t.Errorf("ColumnStart = %d, want 252", got)
	}
	if got := roundI(ColumnSpacingF(0)); got != 0 {
		t.Errorf("ColumnSpacing = %d, want 0", got)
	}
}

// Reverse (osu! -> Quaver) golden values, derived from the same representative
// skin: the forward goldens above fed back through the gist's forward formulas.
func TestReverseCoordsGolden(t *testing.T) {
	if got := QuaverColumnSize(88); got != 141 { // 88 * 1.6 = 140.8
		t.Errorf("QuaverColumnSize = %d, want 141", got)
	}
	if got := QuaverStageReceptorPadding(10); got != 16 {
		t.Errorf("QuaverStageReceptorPadding = %d, want 16", got)
	}
	if got := QuaverComboPosY(215); got != -40 { // 1.6*(215-480)+384
		t.Errorf("QuaverComboPosY = %d, want -40", got)
	}
	if got := QuaverJudgementBurstPosY(308); got != 109 { // 1.6*(308-480)+384 = 108.8
		t.Errorf("QuaverJudgementBurstPosY = %d, want 109", got)
	}
	// Receptor drawn 224 tall with the line at 450*1.6=720 from the top:
	// 720 - 768 + 224 = 176 — the HitPosOffsetY of the forward golden skin.
	if got := QuaverHitPosOffsetY(450, 224); got != 176 {
		t.Errorf("QuaverHitPosOffsetY = %d, want 176", got)
	}
	if got := QuaverColumnAlignment(252, 88, 141, 4); got != 2 { // ~0 up to rounding drift
		t.Errorf("QuaverColumnAlignment = %d, want 2", got)
	}

	// The position mappings must round-trip exactly through the forward inverse.
	for _, comboPosY := range []int{-120, -40, 0, 96} {
		if got := QuaverComboPosY(ComboPosition(comboPosY)); got != comboPosY {
			t.Errorf("ComboPosY round-trip: %d -> %d", comboPosY, got)
		}
	}
}
