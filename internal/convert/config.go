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
func csv(v any, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat(fmt.Sprint(v)+",", n), ",")
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
	out.Set("Version", "latest") // never copy Quaver's Version; "latest" is osu!'s 2.7
	// Quaver's cursor is a plain sprite: it never spins, has no trail, and only
	// grows slightly while clicking (Wobble Cursor, ExpandScale 1.2). osu!
	// defaults to a spinning cursor with a trail, so turn those off.
	// CenterCursor defaults to false in Quaver (SkinStore).
	out.SetInt("CursorCentre", b2i(g.Bool("CenterCursor", false)))
	out.SetInt("CursorRotate", 0)
	out.SetInt("CursorTrailRotate", 0)
	out.SetInt("CursorExpand", 1)
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// columnColors returns the explicit ColumnColorN values. Quaver reads these
// only from the skin's own skin.ini and draws a lane whose colour is missing
// with a transparent column light, so absent entries stay nil.
func columnColors(cfg *quaver.Section, keys int) (colors []*quaver.Color, any bool) {
	colors = make([]*quaver.Color, keys)
	for i := range colors {
		v, ok := cfg.Explicit(fmt.Sprintf("ColumnColor%d", i+1))
		if !ok {
			continue
		}
		if c, ok := quaver.ParseColor(v); ok {
			colors[i], any = &c, true
		}
	}
	return colors, any
}

// buildMania maps a Quaver [nK] config section (with default-skin fallback) to
// an osu! [Mania] block's geometry, colours, and flags. Image keys are added
// later by the asset stage. hitPosition may be a user override.
func buildMania(cfg *quaver.Section, keys int, rec QuaverReceptor, hitPosition int) *osu.Mania {
	m := osu.NewMania(keys)
	kv := m.KV

	colWidth := roundI(ColumnWidthF(rec.ColumnSize))
	spacing := roundI(ColumnSpacingF(cfg.Float("NotePadding", 0)))

	kv.SetInt("ColumnStart", ColumnStart(cfg.Float("ColumnAlignment", 0), colWidth, spacing, keys))
	kv.Set("ColumnWidth", csv(colWidth, keys))
	if spacing != 0 && keys > 1 {
		kv.Set("ColumnSpacing", csv(spacing, keys-1))
	}
	// Quaver sizes note heights from WidthForNoteHeightScale when set, else from
	// the lane width; osu!'s key means the same in its own units.
	if q := cfg.Int("WidthForNoteHeightScale", 0); q > 0 {
		kv.SetInt("WidthForNoteHeightScale", NoteHeightScale(q))
	} else {
		kv.SetInt("WidthForNoteHeightScale", colWidth)
	}
	kv.SetInt("HitPosition", hitPosition)
	kv.SetInt("LightPosition", LightPosition(rec, cfg.Float("ColumnLightingOffsetY", 0)))
	kv.SetInt("ScorePosition", ScorePosition(cfg.Float("JudgementBurstPosY", 0)))
	kv.SetInt("ComboPosition", ComboPosition(cfg.Float("ComboPosY", 0)))

	// Quaver stretches LN bodies between head and tail; osu!'s default on modern
	// skin versions tiles the texture instead. 0 = stretch.
	kv.SetInt("NoteBodyStyle", 0)

	// ReceptorsOverHitObjects is the inverse of osu! KeysUnderNotes.
	kv.SetInt("KeysUnderNotes", b2i(!cfg.Bool("ReceptorsOverHitObjects", true)))
	kv.SetInt("NoteFlipWhenUpsideDown", b2i(cfg.Bool("FlipNoteImagesOnUpscroll", true)))
	kv.SetInt("NoteFlipWhenUpsideDownT", b2i(cfg.Bool("FlipNoteEndImagesOnUpscroll", true)))
	// Quaver has no separate judgement line (the receptors mark the hit area);
	// hide osu!'s default one. A Quaver stage-hint overlay, if present, still maps
	// to StageHint and shows independently.
	kv.SetInt("JudgementLine", 0)

	// Quaver draws timing lines 2px tall in its 1080-space screen.
	kv.Set("BarlineHeight", "1.4")
	if c, ok := cfg.Color("TimingLineColor"); ok {
		kv.Set("ColourBarline", c.String())
	}
	// Quaver has no vertical column-divider lines; hide osu!'s. Zero widths do
	// it on every client; the transparent colour covers clients that ignore them.
	kv.Set("ColumnLineWidth", csv(0, keys+1))
	kv.Set("ColourColumnLine", "0,0,0,0")
	// osu! tints the combo counter with ColourHold (default orange 255,191,51)
	// while a hold note is pressed, and lazer pops a red ColourBreak copy on a
	// combo break; Quaver's combo display does neither.
	kv.Set("ColourHold", "255,255,255,255")
	kv.Set("ColourBreak", "0,0,0,0")

	colors, _ := columnColors(cfg, keys)
	for i, c := range colors {
		v := "0,0,0,0" // Quaver: no colour means no visible light
		if c != nil {
			v = c.String()
		}
		kv.Set(fmt.Sprintf("ColourLight%d", i+1), v)
	}

	// Integers: osu!stable doesn't accept fractional lighting widths. osu!
	// reads 0 as "unset" and falls back to the column width, so a Quaver scale
	// of 0 (lighting off) is handled by blanking the frames instead.
	hit := cfg.Int("HitLightingScale", 100)
	for _, l := range []struct {
		key   string
		scale int
	}{{"LightingNWidth", hit}, {"LightingLWidth", cfg.Int("HoldLightingScale", hit)}} {
		if w := roundI(LightingWidth(l.scale)); w > 0 {
			kv.Set(l.key, csv(w, keys))
		}
	}
	return m
}
