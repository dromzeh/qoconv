package convert

import (
	"fmt"

	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
)

// IsOsuSkin reports whether the source's skin.ini looks like an osu! skin —
// it has at least one [Mania] block — rather than a Quaver one (Quaver uses
// [4K]/[7K] sections and has no [Mania]).
func IsOsuSkin(src quaver.Source) bool {
	data, err := src.ReadFile("skin.ini")
	if err != nil {
		return false
	}
	return len(osu.ParseSkinIni(data).ManiaKeyCounts()) > 0
}

// ReverseResult is a completed osu!mania -> Quaver conversion: the built
// Quaver skin.ini, the output files, and the report.
type ReverseResult struct {
	Ini    *quaver.SkinIni
	Output *Output
	Report *Report
	Modes  []int
	Name   string
}

// Reverse turns an osu!mania skin into a Quaver skin per opts. It is the
// inverse of Convert; opts.HitPosition is osu!-specific and ignored here.
func Reverse(src quaver.Source, opts Options) (*ReverseResult, error) {
	data, err := src.ReadFile("skin.ini")
	if err != nil {
		return nil, fmt.Errorf("read skin.ini (is this an osu! skin?): %w", err)
	}
	sf := osu.ParseSkinIni(data)

	modes := opts.KeyModes
	if modes == nil {
		for _, n := range quaver.SupportedKeyModes {
			if sf.ManiaFor(n) != nil {
				modes = append(modes, n)
			}
		}
	}
	if len(modes) == 0 {
		return nil, fmt.Errorf("no convertible keymodes (4K/7K [Mania] blocks) found in this skin")
	}

	ini := quaver.NewSkinIni()
	name := firstNonEmpty(opts.Name, sf.General.Str("Name", "Converted Skin"))
	gen := ini.Section("General")
	gen.Set("Name", name)
	gen.Set("Author", firstNonEmpty(opts.Author, sf.General.Str("Author", "Unknown")))
	gen.SetBool("CenterCursor", sf.General.Bool("CursorCentre", true))

	rc := &revCtx{src: src, out: newOutput(), rep: &Report{ToQuaver: true}, consumed: map[string]bool{"skin.ini": true}}

	for _, mode := range modes {
		cfg := sf.ManiaFor(mode) // nil is fine: osu.Section accessors fall back to defaults
		buildQuaverMode(ini, rc, cfg, mode)
	}
	mapReverseUniversal(rc, sf, sf.ManiaFor(modes[0]), opts)

	if n := rc.unconsumed(); n > 0 {
		rc.rep.warn("%d file(s) in the osu! skin have no Quaver equivalent and were skipped.", n)
	}
	rc.rep.warn("Health-bar rotation and exact hit/combo/score positions are approximate — verify in-game.")

	rc.out.add("skin.ini", []byte(ini.Serialize()))
	return &ReverseResult{Ini: ini, Output: rc.out, Report: rc.rep, Modes: modes, Name: name}, nil
}

// buildQuaverMode maps one osu! [Mania] block to a Quaver [nK] section and its
// keymode assets. cfg may be nil (no [Mania] block for a forced keymode).
func buildQuaverMode(ini *quaver.SkinIni, rc *revCtx, cfg *osu.Section, keys int) {
	columnWidth := cfg.FirstCSVInt("ColumnWidth", 30) // osu! default width
	columnSize := QuaverColumnSize(columnWidth)

	recH := mapReverseKeymodeAssets(rc, cfg, keys, columnSize)

	sec := ini.Section(fmt.Sprintf("%dK", keys))
	sec.SetInt("ColumnSize", columnSize)
	sec.SetInt("ColumnAlignment", QuaverColumnAlignment(cfg.Int("ColumnStart", 136), columnWidth, columnSize, keys))
	if sp := QuaverStageReceptorPadding(cfg.FirstCSVInt("ColumnSpacing", 0)); sp != 0 {
		sec.SetInt("StageReceptorPadding", sp)
	}

	hitPos := clampHitPosition(cfg.Int("HitPosition", OsuDefaultHitPosition))
	if recH > 0 {
		// Receptors are written for ReceptorPosOffsetY 0 (see mapReverseReceptor).
		sec.SetInt("ReceptorPosOffsetY", 0)
		sec.SetInt("HitPosOffsetY", QuaverHitPosOffsetY(hitPos, recH))
	} else {
		rc.rep.warn("%dK: no receptor image found; HitPosOffsetY left at Quaver's default.", keys)
	}

	// Only translate positions the osu! skin actually sets; when absent, Quaver's
	// own defaults are the closest match for osu!'s default layout.
	if _, ok := cfg.Get("ComboPosition"); ok {
		sec.SetInt("ComboPosY", QuaverComboPosY(cfg.Int("ComboPosition", 0)))
	}
	if _, ok := cfg.Get("ScorePosition"); ok {
		sec.SetInt("JudgementBurstPosY", QuaverJudgementBurstPosY(cfg.Int("ScorePosition", 0)))
	}
	if _, ok := cfg.Get("WidthForNoteHeightScale"); ok {
		sec.SetInt("WidthForNoteHeightScale", QuaverColumnSize(cfg.Int("WidthForNoteHeightScale", 0)))
	}

	// KeysUnderNotes is the inverse of Quaver's ReceptorsOverHitObjects.
	sec.SetBool("ReceptorsOverHitObjects", !cfg.Bool("KeysUnderNotes", false))
	if _, ok := cfg.Get("NoteFlipWhenUpsideDown"); ok {
		sec.SetBool("FlipNoteImagesOnUpscroll", cfg.Bool("NoteFlipWhenUpsideDown", true))
	}

	if c, ok := colorOf(cfg, "ColourBarline"); ok {
		sec.Set("TimingLineColor", c)
	}
	for i := 1; i <= keys; i++ {
		if c, ok := colorOf(cfg, fmt.Sprintf("ColourLight%d", i)); ok {
			sec.Set(fmt.Sprintf("ColumnColor%d", i), c)
		}
	}

	rc.rep.position(fmt.Sprintf("%dK  ColumnSize %d · ColumnAlignment %s · HitPosOffsetY %s · ComboPosY %s · JudgementBurstPosY %s",
		keys, columnSize, secStr(sec, "ColumnAlignment"), secStr(sec, "HitPosOffsetY"),
		secStr(sec, "ComboPosY"), secStr(sec, "JudgementBurstPosY")))
}

// colorOf re-renders an osu! colour value through Quaver's parser so malformed
// values are dropped instead of copied verbatim.
func colorOf(cfg *osu.Section, key string) (string, bool) {
	v, ok := cfg.Get(key)
	if !ok {
		return "", false
	}
	c, ok := quaver.ParseColor(v)
	if !ok {
		return "", false
	}
	return c.String(), true
}

// secStr reads a built section key for the position summary ("-" if unset).
func secStr(sec *quaver.IniSection, key string) string {
	if v, ok := sec.Lookup(key); ok {
		return v
	}
	return "-"
}
