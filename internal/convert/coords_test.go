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
