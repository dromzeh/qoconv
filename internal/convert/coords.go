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

// Quaver anchors both the combo display and the judgement burst centre-on-
// screen-centre plus a Y offset (GameplayPlayfieldKeysStage: Alignment
// MidCenter, Y = ComboPosY / JudgementBurstPosY); osu! draws both centre-
// anchored at N×1.6 from the top of the 768-tall space (confirmed from
// ManiaLegacySkinTransformer and LegacyManiaJudgementPiece). So the exact
// mapping for both is N = (offset + 384) / R, which is what the gist-inverse
// formulas below reduce to.
//
// When a skin.ini omits these keys Quaver falls back to the selected default
// skin's values — Bar, Arrow, and Circle all ship ComboPosY -40 and
// JudgementBurstPosY 108 — so absence means these values, not 0.
const (
	quaverDefaultComboPosY          = -40
	quaverDefaultJudgementBurstPosY = 108
)

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

// HitPosition is the osu! judgement line, as a distance from the TOP of the
// 480-tall playfield — confirmed from osu! source (LegacyManiaSkinDecoder:
// stored as (480 − clamp(N,240,480)) × 1.6, applied as bottom-padding). At hit
// time osu! lands the note's BOTTOM edge on this line (DrawableManiaHitObject /
// LegacyNotePiece anchor BottomCentre in downscroll); the same holds for LN
// heads and tails at their start/end times.
//
// Quaver's equivalent line (GameplayPlayfieldKeys.SetReferencePositions) is
// where the note's bottom edge sits at hit time:
//
//	HitLineY = ReceptorTopY + HitPosOffsetY
//	         = 768 − ReceptorPosOffsetY − receptorH×ColumnSize/receptorW + HitPosOffsetY
//
// (the receptor is drawn ColumnSize wide, aspect-scaled tall, with its bottom
// ReceptorPosOffsetY up from the screen bottom). ÷1.6 maps it into 480 space.
// e.g. ColumnSize 140, 250x400 receptor, HitPosOffsetY 176 -> 450. Falls back
// to osu!'s default when the receptor size is unknown.
func HitPosition(receptorPosOffsetY, hitPosOffsetY, columnSize, receptorW, receptorH int) int {
	if columnSize <= 0 || receptorW <= 0 {
		return OsuDefaultHitPosition
	}
	receptorScaledH := float64(receptorH) * float64(columnSize) / float64(receptorW)
	lineFromTop := 768.0 - float64(receptorPosOffsetY) - receptorScaledH + float64(hitPosOffsetY)
	return clampHitPosition(roundI(lineFromTop / R))
}

// ReceptorBottomPad returns the transparent padding (in source pixels) to add
// BELOW a receptor image. osu! bottom-anchors key images to the screen and
// draws them at native pixel height (px map 1:1 into its 768-tall space),
// ignoring HitPosition entirely — which matches Quaver, where the receptor's
// bottom edge sits ReceptorPosOffsetY above the screen bottom. So the pad is
// simply ReceptorPosOffsetY (never negative).
func ReceptorBottomPad(receptorPosOffsetY int) int {
	if receptorPosOffsetY < 0 {
		return 0
	}
	return receptorPosOffsetY
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
