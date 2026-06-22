package convert

import (
	"fmt"
	"strings"

	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
)

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// csv returns "v,v,...,v" repeated n times (empty if n <= 0).
func csv(v, n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	s := fmt.Sprint(v)
	for i := range parts {
		parts[i] = s
	}
	return strings.Join(parts, ",")
}

// buildGeneral maps the Quaver [General] section to osu! [General]. Name/author
// may be overridden (TUI/flags); empty overrides fall back to the Quaver values.
func buildGeneral(sk *quaver.Skin, nameOverride, authorOverride string) *osu.KV {
	g := sk.General()
	name := firstNonEmpty(nameOverride, g.Str("Name", "Converted Skin"))
	author := firstNonEmpty(authorOverride, g.Str("Author", "Unknown"))

	out := osu.NewKV()
	out.Set("Name", name)
	out.Set("Author", author)
	out.Set("Version", "latest") // never copy Quaver's Version (gist); set a valid osu! value
	out.SetInt("CursorCentre", b2i(g.Bool("CenterCursor", true)))
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// buildMania maps a Quaver [nK] config section to an osu! [Mania] block's
// geometry, colors, and flags. Image-path keys are appended later (asset stage).
// receptorHeight is the pixel height of receptor-up-1 (couples HitPosition/LightPosition).
func buildMania(cfg *quaver.Section, keys, hitPosition int) *osu.Mania {
	m := osu.NewMania(keys)
	kv := m.KV

	columnSize := cfg.Int("ColumnSize", 0)
	colWidthF := ColumnWidthF(columnSize)
	colWidth := roundI(colWidthF)

	kv.SetInt("ColumnStart", ColumnStart(cfg.Int("ColumnAlignment", 0), columnSize, keys, colWidthF))
	kv.Set("ColumnWidth", csv(colWidth, keys))
	// Quaver notes/receptors are square (e.g. 150×150); pin the note height scale
	// to the column width so osu! renders them square instead of stretched tall.
	kv.SetInt("WidthForNoteHeightScale", colWidth)
	if sp := roundI(ColumnSpacingF(cfg.Int("StageReceptorPadding", 0))); sp != 0 {
		kv.Set("ColumnSpacing", csv(sp, keys-1))
	}
	kv.SetInt("HitPosition", hitPosition)
	kv.SetInt("LightPosition", hitPosition) // the stage light flashes at the hit line
	kv.SetInt("ScorePosition", ScorePosition(cfg.Int("JudgementBurstPosY", 0)))
	kv.SetInt("ComboPosition", ComboPosition(cfg.Int("ComboPosY", 0)))

	// ReceptorsOverHitObjects is the inverse of osu! KeysUnderNotes.
	kv.SetInt("KeysUnderNotes", b2i(!cfg.Bool("ReceptorsOverHitObjects", false)))
	kv.SetInt("NoteFlipWhenUpsideDown", b2i(cfg.Bool("FlipNoteImagesOnUpscroll", false)))
	// Quaver has no separate judgement line (the receptors mark the hit area);
	// hide osu!'s default one. A Quaver stage-hint overlay, if present, still maps
	// to StageHint and shows independently.
	kv.SetInt("JudgementLine", 0)

	if c, ok := cfg.Color("TimingLineColor"); ok {
		kv.Set("ColourBarline", c.String())
	}
	// Quaver has no vertical column-divider lines; hide osu!'s (default white).
	kv.Set("ColourColumnLine", "0,0,0,0")
	for i := 1; i <= keys; i++ {
		if c, ok := cfg.Color(fmt.Sprintf("ColumnColor%d", i)); ok {
			kv.Set(fmt.Sprintf("ColourLight%d", i), c.String())
		}
	}
	return m
}
