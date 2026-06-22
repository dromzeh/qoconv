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
	// Judgement line at Quaver's receptor centre: ReceptorPosOffsetY=0,
	// receptor 250x400 -> centre 656/768 -> 410 in osu! 480-space.
	if got := HitPosition(0, columnSize, 250, 400); got != 410 {
		t.Errorf("HitPosition = %d, want 410", got)
	}
	// Unknown receptor size -> osu! default.
	if got := HitPosition(0, 0, 0, 0); got != OsuDefaultHitPosition {
		t.Errorf("HitPosition fallback = %d, want %d", got, OsuDefaultHitPosition)
	}
	// Receptor bottom-pad lifts the squared circle onto the line:
	// (480-410)*1.6 - 140/2 = 112 - 70 = 42.
	if got := ReceptorBottomPad(410, columnSize); got != 42 {
		t.Errorf("ReceptorBottomPad = %d, want 42", got)
	}
	if got := ColumnStart(0, columnSize, keys, colW); got != 252 {
		t.Errorf("ColumnStart = %d, want 252", got)
	}
	if got := roundI(ColumnSpacingF(0)); got != 0 {
		t.Errorf("ColumnSpacing = %d, want 0", got)
	}
}
