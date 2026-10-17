package convert

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
)

// IsOsuSkin reports whether the source's skin.ini looks like an osu! skin —
// it has at least one [Mania] block — rather than a Quaver one (Quaver uses
// [nK] sections and has no [Mania]).
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
		return nil, fmt.Errorf("no convertible keymodes (1K-10K [Mania] blocks) found in this skin")
	}

	ini := quaver.NewSkinIni()
	name := firstNonEmpty(opts.Name, sf.General.Str("Name", "Converted Skin"))
	gen := ini.Section("General")
	gen.Set("Name", name)
	gen.Set("Author", firstNonEmpty(opts.Author, sf.General.Str("Author", "Unknown")))
	centred := sf.General.Bool("CursorCentre", true)
	gen.SetBool("CenterCursor", centred)

	rc := &revCtx{
		src:      src,
		out:      newOutput(),
		rep:      &Report{ToQuaver: true},
		consumed: map[string]bool{"skin.ini": true},
		version:  skinVersion(sf.General.Str("Version", "")),
	}

	// Judgements are universal in both games but Quaver sizes them with a
	// per-keymode key, so map them first and write the scale into every mode.
	primary := sf.ManiaFor(modes[0])
	judgeH := rc.judgements(primary)
	for _, mode := range modes {
		cfg := sf.ManiaFor(mode) // nil is fine: osu.Section accessors fall back to defaults
		sec := buildQuaverMode(ini, rc, cfg, mode)
		if judgeH > 0 {
			sec.SetInt("JudgementHitBurstScale", roundI(judgeH))
		}
	}
	rc.universal(sf, centred, opts)

	if n := rc.unconsumed(); n > 0 {
		rc.rep.warn("%d file(s) in the osu! skin have no Quaver equivalent and were skipped.", n)
	}
	rc.rep.warn("Health-bar placement differs between the games; verify in-game.")

	rc.out.add("skin.ini", []byte(ini.Serialize()))
	return &ReverseResult{Ini: ini, Output: rc.out, Report: rc.rep, Modes: modes, Name: name}, nil
}

// skinVersion parses an osu! [General] Version. A missing one means 1.0;
// "latest" is 2.7 (LegacySkinDecoder).
func skinVersion(v string) float64 {
	v = strings.TrimSpace(v)
	if strings.EqualFold(v, "latest") {
		return 2.7
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return 1.0
}

// buildQuaverMode maps one osu! [Mania] block to a Quaver [nK] section and its
// keymode assets. cfg may be nil (no [Mania] block for a forced keymode).
// Every layout key is written explicitly: Quaver would otherwise fill gaps
// from its own default skin, which isn't what the osu! skin looks like.
func buildQuaverMode(ini *quaver.SkinIni, rc *revCtx, cfg *osu.Section, keys int) *quaver.IniSection {
	columnWidth := cfg.FirstCSVFloat("ColumnWidth", OsuDefaultColumnWidth)
	spacing := cfg.FirstCSVFloat("ColumnSpacing", 0)
	columnSize := roundI(QuaverColumnSize(columnWidth))

	recH := rc.keymode(cfg, keys, columnSize)

	sec := ini.Section(fmt.Sprintf("%dK", keys))
	sec.SetInt("ColumnSize", columnSize)
	sec.SetInt("NotePadding", roundI(QuaverNotePadding(spacing)))
	sec.SetInt("StageReceptorPadding", 0)
	sec.SetInt("ColumnAlignment", roundI(QuaverColumnAlignment(cfg.Float("ColumnStart", OsuDefaultColumnStart), columnWidth, spacing, keys)))

	hitPos := ClampHitPosition(cfg.Int("HitPosition", OsuDefaultHitPosition))
	if recH > 0 {
		// Receptors are written for ReceptorPosOffsetY 0 (see revCtx.receptor).
		sec.SetInt("ReceptorPosOffsetY", 0)
		sec.SetInt("HitPosOffsetY", roundI(QuaverHitPosOffsetY(hitPos, recH)))
		sec.SetInt("ColumnLightingOffsetY", roundI(QuaverColumnLightingOffsetY(cfg.Int("LightPosition", OsuDefaultLightPosition), recH)))
		// osu! centres hit lighting on the hit line; Quaver on the receptor centre.
		sec.SetInt("HitLightingY", roundI(QuaverHitLightingY(QuaverHitPosOffsetY(hitPos, recH), recH)))
	} else {
		rc.rep.warnMode(keys, "no receptor image found; HitPosOffsetY left at Quaver's default.")
	}
	sec.Set("ColumnLightingScale", "1.0") // the light is written ColumnSize wide at osu!'s height

	// Only translate positions the osu! skin sets. osu!stable's defaults for
	// these aren't public (lazer uses 111/300, unverified against stable), and
	// Quaver's own defaults (-40/108) sit where players expect them.
	if _, ok := cfg.Get("ComboPosition"); ok {
		sec.SetInt("ComboPosY", roundI(QuaverComboPosY(cfg.Float("ComboPosition", 0))))
	}
	if _, ok := cfg.Get("ScorePosition"); ok {
		sec.SetInt("JudgementBurstPosY", roundI(QuaverJudgementBurstPosY(cfg.Float("ScorePosition", 0))))
	}
	if w := cfg.Float("WidthForNoteHeightScale", 0); w > 0 {
		sec.SetInt("WidthForNoteHeightScale", QuaverNoteHeightScale(w))
	} else {
		sec.SetInt("WidthForNoteHeightScale", 0) // both games then use the column width
	}

	// osu! scales lighting by LightingNWidth/30 (ColumnWidth when unset; 1×
	// before skin version 2.5).
	lightW := func(key string) float64 {
		if rc.version < 2.5 {
			return OsuDefaultColumnWidth
		}
		return cfg.FirstCSVFloat(key, columnWidth)
	}
	sec.SetInt("HitLightingScale", QuaverLightingScale(lightW("LightingNWidth")))
	sec.SetInt("HoldLightingScale", QuaverLightingScale(lightW("LightingLWidth")))
	sec.SetInt("HitLightingX", 0)

	// osu! draws the combo at native size and the score/accuracy counters at
	// 0.96×/0.576× (LegacyScoreCounter, LegacyAccuracyCounter); Quaver scales
	// digits by these percentages.
	sec.SetInt("ComboDisplayScale", 100)
	sec.SetInt("ScoreDisplayScale", 96)
	sec.SetInt("AccuracyDisplayScale", 58)

	// osu!stable draws the mania health bar vertically beside the stage.
	sec.Set("HealthBarType", "Vertical")
	sec.Set("HealthBarKeysAlignment", "RightStage")
	sec.SetInt("HealthBarScale", 100)

	// KeysUnderNotes is the inverse of Quaver's ReceptorsOverHitObjects.
	sec.SetBool("ReceptorsOverHitObjects", !cfg.Bool("KeysUnderNotes", false))
	sec.SetBool("FlipNoteImagesOnUpscroll", cfg.Bool("NoteFlipWhenUpsideDown", true))
	sec.SetBool("FlipNoteEndImagesOnUpscroll", cfg.Bool("NoteFlipWhenUpsideDownT", true))
	sec.SetBool("DrawLongNoteEnd", true)

	if c, ok := colorOf(cfg, "ColourBarline"); ok {
		sec.Set("TimingLineColor", c)
	}
	// Quaver hides the column light of any lane without a ColumnColor, where
	// osu! defaults ColourLight to white; always write one.
	for i := 1; i <= keys; i++ {
		c, ok := colorOf(cfg, fmt.Sprintf("ColourLight%d", i))
		if !ok {
			c = "255,255,255"
		}
		sec.Set(fmt.Sprintf("ColumnColor%d", i), c)
	}

	rc.rep.position(fmt.Sprintf("%dK  ColumnSize %d · ColumnAlignment %s · HitPosOffsetY %s · ComboPosY %s · JudgementBurstPosY %s",
		keys, columnSize, secStr(sec, "ColumnAlignment"), secStr(sec, "HitPosOffsetY"),
		secStr(sec, "ComboPosY"), secStr(sec, "JudgementBurstPosY")))
	return sec
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
