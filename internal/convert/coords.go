// Package convert turns a parsed Quaver skin into an osu!mania skin: coordinate
// math, config-key mapping, asset mapping, and the conversion report.
package convert

import "math"

// R is the Quaver (768-height) ÷ osu! (480-height) vertical ratio. The reference
// gist documents osu!→Quaver formulas; every function here is the INVERSE
// (Quaver value in, osu! value out). See the plan's coordinate-math table.
const R = 768.0 / 480.0

// osuPlayfieldWidth16x9 is the horizontal extent of the osu! 480-height space at
// 16:9 (= 16/9 * 480), used by the ColumnAlignment/ColumnStart conversion.
const osuPlayfieldWidth16x9 = 16.0 / 9.0 * 480.0

func roundI(f float64) int { return int(math.Round(f)) }

// ColumnWidthF: gist ColumnSize = round(ColumnWidth * R)  =>  ColumnWidth = ColumnSize / R.
func ColumnWidthF(columnSize int) float64 { return float64(columnSize) / R }

// ColumnSpacingF: gist StageReceptorPadding = round(ColumnSpacing * R).
func ColumnSpacingF(stageReceptorPadding int) float64 { return float64(stageReceptorPadding) / R }

// ComboPosition: gist ComboPosY = round(R*(ComboPosition-480) + 768/2).
func ComboPosition(comboPosY int) int { return roundI((float64(comboPosY)-384.0)/R + 480.0) }

// ScorePosition: gist JudgementBurstPosY = round(R*(ScorePosition-480) + 384).
func ScorePosition(judgementBurstPosY int) int {
	return roundI((float64(judgementBurstPosY)-384.0)/R + 480.0)
}

// clampHitPosition matches osu!'s own clamp of the skin.ini HitPosition value.
func clampHitPosition(v int) int {
	switch {
	case v < 240:
		return 240
	case v > 480:
		return 480
	default:
		return v
	}
}

// OsuDefaultHitPosition is osu!mania's stock judgement-line height.
const OsuDefaultHitPosition = 402

// HitPosition is the osu! judgement line (where notes are hit), as a distance
// from the TOP of the 480-tall playfield — confirmed from osu! source
// (LegacyManiaSkinDecoder: stored as (480 − clamp(N,240,480)) × 1.6, applied as
// bottom-padding, so the line lands N px from the top; default 402 ≈ near the
// bottom). osu! does NOT move the receptor with this — the receptor is anchored
// to the stage bottom and is aligned separately by padding its image.
//
// We target Quaver's receptor centre: it sits ReceptorPosOffsetY up from the
// bottom, scaled to ColumnSize wide, so its centre (from the top, 768 space) is
// 768 − ReceptorPosOffsetY − receptorH×ColumnSize/receptorW/2; ÷1.6 → 480 space.
// e.g. ColumnSize 140 with a 250x400 receptor -> 410. Falls back to osu!'s
// default when the receptor size is unknown.
func HitPosition(receptorPosOffsetY, columnSize, receptorW, receptorH int) int {
	if columnSize <= 0 || receptorW <= 0 {
		return OsuDefaultHitPosition
	}
	receptorScaledH := float64(receptorH) * float64(columnSize) / float64(receptorW)
	centerFromTop := 768.0 - float64(receptorPosOffsetY) - receptorScaledH/2.0
	return clampHitPosition(roundI(centerFromTop / R))
}

// ReceptorBottomPad returns the transparent padding (in source pixels) to add
// BELOW a receptor squared to columnSize, so that — once osu! bottom-anchors and
// ÷1.6-scales it — the receptor's circle centre lands on the judgement line at
// hitPosition. The circle centre must sit (480 − hitPosition) × 1.6 px above the
// image bottom; the squared circle's centre is already columnSize/2 up.
func ReceptorBottomPad(hitPosition, columnSize int) int {
	pad := roundI(float64(480-hitPosition)*R) - columnSize/2
	if pad < 0 {
		return 0
	}
	return pad
}

// ColumnStart inverts the gist ColumnAlignment formula:
//
//	ColumnAlignment = round( A * ( (-2*ColumnStart)/D - 1 ) )
//	A = (1366 - SIZES)/2 ; D = WIDTHS - (16/9*480)
//	SIZES = ColumnSize*keys ; WIDTHS = sum(ColumnWidth) = columnWidth*keys
//
// => ColumnStart = -D * (ColumnAlignment/A + 1) / 2.
func ColumnStart(columnAlignment, columnSize, keys int, columnWidth float64) int {
	sizes := float64(columnSize * keys)
	widths := columnWidth * float64(keys)
	a := (1366.0 - sizes) / 2.0
	d := widths - osuPlayfieldWidth16x9
	if a == 0 {
		return 136 // osu! default; alignment is indeterminate when stage fills width
	}
	return roundI(-d * (float64(columnAlignment)/a + 1.0) / 2.0)
}
