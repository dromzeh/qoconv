// Package convert converts between Quaver and osu!mania skins — Convert for
// Quaver -> osu!, Reverse for osu! -> Quaver: coordinate math, config-key
// mapping, asset mapping, and the conversion report.
package convert

import (
	"fmt"
	"math"
)

// The three coordinate spaces involved:
//
//   - osu! skin.ini positions are in a 480-tall space; osu! multiplies them by
//     1.6 into its 768-tall drawing space (LegacyManiaSkinDecoder), and draws
//     skin textures 1 pixel = 1 unit in that 768 space.
//   - Quaver's [FixedScale] float keys (ColumnSize, HitPosOffsetY, ComboPosY,
//     JudgementHitBurstScale, …) are written in a 1366×768 space: Quaver
//     multiplies them by 1920/1366 into its 1920×1080 virtual screen. So they
//     share osu!'s 768 units.
//   - Quaver draws raw texture pixels, and int keys that FixedScale skips
//     (WidthForNoteHeightScale, HitLightingScale), in 1080 units: one of them
//     is 768/1080 of a 768 unit.
const (
	// R converts osu!'s 480 space to the shared 768 space.
	R = 768.0 / 480.0
	// quaverPixel is the size of one Quaver 1080-space unit in 768 units.
	quaverPixel = 768.0 / 1080.0
	// osuWidth is the width of osu!'s 480-tall space at 16:9.
	osuWidth = 16.0 / 9.0 * 480.0
	// quaverWidth is the width of the shared 768-tall space at 16:9.
	quaverWidth = 16.0 / 9.0 * 768.0
)

func roundI(f float64) int { return int(math.Round(f)) }

// --- forward (Quaver -> osu!) ---

// ColumnWidthF: Quaver draws each lane ColumnSize wide (768 units).
func ColumnWidthF(columnSize float64) float64 { return columnSize / R }

// ColumnSpacingF: Quaver's gap between lanes is NotePadding (GameplayPlayfieldKeys
// ReceptorPadding); StageReceptorPadding is the stage's outer margin instead.
func ColumnSpacingF(notePadding float64) float64 { return notePadding / R }

// ColumnStart is the left edge of the first column. Quaver centres the stage
// on the screen and shifts it by ColumnAlignment (Alignment TopCenter, X =
// ColumnAlignment); StageReceptorPadding pads both sides equally, so it moves
// the borders but not the columns. columnWidth and spacing are the (rounded)
// osu! values, so the stage osu! actually draws is the one that gets centred.
func ColumnStart(columnAlignment float64, columnWidth, spacing, keys int) int {
	stage := float64(columnWidth*keys + spacing*(keys-1))
	return roundI(osuWidth/2 + columnAlignment/R - stage/2)
}

// Quaver anchors both the combo display and the judgement burst centre-on-
// screen-centre plus a Y offset (Alignment MidCenter, Y = ComboPosY /
// JudgementBurstPosY); osu! draws both centred at N×1.6 from the top of the
// 768-tall space (ManiaLegacySkinTransformer, LegacyManiaJudgementPiece). So
// N = (offset + 384) / R.

// ComboPosition maps Quaver ComboPosY to osu! ComboPosition.
func ComboPosition(comboPosY float64) int { return roundI((comboPosY + 384) / R) }

// ScorePosition maps Quaver JudgementBurstPosY to osu! ScorePosition (osu!
// draws judgements there).
func ScorePosition(judgementBurstPosY float64) int { return roundI((judgementBurstPosY + 384) / R) }

// ClampHitPosition applies osu!'s own clamp of the skin.ini HitPosition value
// (LegacyManiaSkinDecoder).
func ClampHitPosition[T ~int | ~float64](v T) T { return min(max(v, 240), 480) }

// osu!mania's stock values, used when the receptor size is unknown.
const (
	OsuDefaultHitPosition   = 402
	OsuDefaultLightPosition = 413
)

// osu!mania's defaults for the [Mania] layout keys (LegacyManiaSkinConfiguration).
const (
	OsuDefaultColumnWidth = 30
	OsuDefaultColumnStart = 136
)

// ReceptorUpPath is the Quaver path of a keymode's released receptor for col
// (1-based). Quaver sizes each lane's receptor sprite from this texture, and
// the first lane's places the hit line.
func ReceptorUpPath(keys, col int) string {
	return fmt.Sprintf("%dk/Receptors/receptor-up-%d.png", keys, col)
}

// QuaverReceptor is one keymode's Quaver receptor placement (768 units, except
// the texture size). Quaver draws receptor-up ColumnSize wide, aspect-scaled
// tall, with its bottom edge ReceptorPosOffsetY above the screen bottom
// (GameplayPlayfieldKeys.SetReferencePositions).
type QuaverReceptor struct {
	ColumnSize, ReceptorPosOffsetY float64
	TexW, TexH                     int // receptor-up pixel size; 0 when unknown
}

// Known reports whether the receptor's drawn size can be worked out.
func (r QuaverReceptor) Known() bool { return r.ColumnSize > 0 && r.TexW > 0 && r.TexH > 0 }

// Height is the receptor's drawn height, or 0 without a texture.
func (r QuaverReceptor) Height() float64 {
	if r.TexW <= 0 {
		return 0
	}
	return float64(r.TexH) * r.ColumnSize / float64(r.TexW)
}

// Top is the receptor's top edge, measured from the top of the screen. Quaver
// hangs both the hit line (HitPosOffsetY) and the column light
// (ColumnLightingOffsetY) off it.
func (r QuaverReceptor) Top() float64 { return 768 - r.ReceptorPosOffsetY - r.Height() }

// QuaverStage is one keymode's Quaver lane layout (768 units). Quaver centres
// the stage on the screen and shifts it by ColumnAlignment (Alignment
// TopCenter); lanes are ColumnSize wide with NotePadding between them, and
// StageReceptorPadding widens the stage on both sides (GameplayPlayfieldKeys).
type QuaverStage struct {
	Keys                 int
	ColumnSize           float64
	NotePadding          float64
	StageReceptorPadding float64
	ColumnAlignment      float64
}

// Width is the whole stage's width, padding included.
func (s QuaverStage) Width() float64 {
	return (s.ColumnSize+s.NotePadding)*float64(s.Keys) - s.NotePadding + 2*s.StageReceptorPadding
}

// Left is the stage's left edge, padding included.
func (s QuaverStage) Left() float64 { return quaverWidth/2 + s.ColumnAlignment - s.Width()/2 }

// ColumnX is the left edge of lane i (0-based).
func (s QuaverStage) ColumnX(i int) float64 {
	return s.Left() + s.StageReceptorPadding + float64(i)*(s.ColumnSize+s.NotePadding)
}

// HitPosition is the osu! judgement line, as a distance from the TOP of the
// 480-tall playfield (LegacyManiaSkinDecoder stores (480 − clamp(N,240,480)) ×
// 1.6 as bottom padding). At hit time osu! lands the note's BOTTOM edge on it.
// Quaver lands a note's bottom edge on receptor top + HitPosOffsetY.
// e.g. ColumnSize 140, 250x400 receptor, HitPosOffsetY 176 -> 450.
func HitPosition(r QuaverReceptor, hitPosOffsetY float64) int {
	if !r.Known() {
		return OsuDefaultHitPosition
	}
	return ClampHitPosition(roundI((r.Top() + hitPosOffsetY) / R))
}

// LightPosition is where osu! puts the bottom of the column light (480 space,
// from the top; unclamped). Quaver draws its column lighting with the bottom
// ColumnLightingOffsetY above the receptor top, not on the hit line.
func LightPosition(r QuaverReceptor, lightingOffsetY float64) int {
	if !r.Known() {
		return OsuDefaultLightPosition
	}
	return roundI((r.Top() - lightingOffsetY) / R)
}

// ReceptorBottomPad returns the transparent padding (in output pixels) to add
// BELOW a receptor image. osu! bottom-anchors key images to the screen and
// draws them at native pixel height, ignoring HitPosition; Quaver lifts the
// receptor ReceptorPosOffsetY off the screen bottom. Never negative.
func ReceptorBottomPad(receptorPosOffsetY float64) int { return max(roundI(receptorPosOffsetY), 0) }

// NoteHeightScale maps Quaver WidthForNoteHeightScale to osu!'s. Quaver's is
// an int, which FixScale leaves in 1080 units; osu! multiplies its own by 1.6.
func NoteHeightScale(widthForNoteHeightScale int) int {
	return roundI(QuaverPixels(float64(widthForNoteHeightScale)) / R)
}

// QuaverPixels converts a length in Quaver's 1080 units (raw texture pixels,
// and the int keys FixedScale skips, such as WidthForNoteHeightScale) to 768
// units.
func QuaverPixels(v float64) float64 { return v * quaverPixel }

// LightingWidth maps Quaver Hit/HoldLightingScale (a percentage of the texture's
// 1080-space size) to osu! LightingNWidth/LightingLWidth, which scale the
// texture by width/30 (LegacySkin ExplosionScale).
func LightingWidth(lightingScale int) float64 {
	return 30 * lightingPixel(lightingScale)
}

// lightingPixel is the drawn size, in 768 units, of one hit-lighting texture
// pixel in Quaver.
func lightingPixel(lightingScale int) float64 { return QuaverPixels(float64(lightingScale)) / 100 }

// --- reverse (osu! -> Quaver) ---

// QuaverColumnSize maps osu! ColumnWidth to Quaver ColumnSize.
func QuaverColumnSize(columnWidth float64) float64 { return columnWidth * R }

// QuaverNotePadding maps osu! ColumnSpacing to Quaver NotePadding.
func QuaverNotePadding(columnSpacing float64) float64 { return columnSpacing * R }

// QuaverColumnAlignment inverts ColumnStart: the offset of the osu! stage's
// centre from the screen centre.
func QuaverColumnAlignment(columnStart, columnWidth, spacing float64, keys int) float64 {
	stage := columnWidth*float64(keys) + spacing*float64(keys-1)
	return (columnStart + stage/2 - osuWidth/2) * R
}

// QuaverComboPosY inverts ComboPosition.
func QuaverComboPosY(comboPosition float64) float64 { return comboPosition*R - 384 }

// QuaverJudgementBurstPosY inverts ScorePosition.
func QuaverJudgementBurstPosY(scorePosition float64) float64 { return scorePosition*R - 384 }

// QuaverHitPosOffsetY inverts HitPosition. The converted receptor is written
// with ReceptorPosOffsetY 0 and is receptorH tall as Quaver draws it, so its
// top sits at 768 - receptorH; the hit line must land at HitPosition*R.
func QuaverHitPosOffsetY(hitPosition int, receptorH float64) float64 {
	return float64(hitPosition)*R - 768 + receptorH
}

// QuaverColumnLightingOffsetY inverts LightPosition for a receptor written at
// ReceptorPosOffsetY 0 that is receptorH tall.
func QuaverColumnLightingOffsetY(lightPosition int, receptorH float64) float64 {
	return 768 - receptorH - float64(lightPosition)*R
}

// QuaverNoteHeightScale inverts NoteHeightScale.
func QuaverNoteHeightScale(widthForNoteHeightScale float64) int {
	return roundI(widthForNoteHeightScale * R / quaverPixel)
}

// QuaverLightingScale inverts LightingWidth.
func QuaverLightingScale(lightingWidth float64) int {
	return roundI(lightingWidth / 30 / quaverPixel * 100)
}

// QuaverHitLightingY places Quaver's hit lighting (centred on the receptor's
// centre plus HitLightingY) on osu!'s hit line (where osu! centres it).
func QuaverHitLightingY(hitPosOffsetY, receptorH float64) float64 {
	return hitPosOffsetY - receptorH/2
}
