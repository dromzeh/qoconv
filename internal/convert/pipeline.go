package convert

import (
	"fmt"
	"strings"

	"github.com/dromzeh/qoconv/internal/imageops"
	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
)

// Options configures a conversion.
type Options struct {
	Name      string // override skin name (empty = use Quaver's)
	Author    string // override author (empty = use Quaver's)
	KeyModes  []int  // nil = all detected
	Grades    bool   // opportunistic: map letter grades -> ranking-*
	Hitsounds bool   // opportunistic: map SFX -> normal-hit*
	// RotateHealthCW chooses the health-bar rotation direction (calibration).
	RotateHealthCW bool
	// HitPosition overrides the computed osu! HitPosition; -1 = auto.
	HitPosition int
}

// DefaultOptions returns sensible defaults (opportunistic extras on).
func DefaultOptions() Options {
	return Options{Grades: true, Hitsounds: true, RotateHealthCW: true, HitPosition: -1}
}

// Result is a completed conversion: the osu! skin model, the flat output files
// (including the serialized skin.ini), and the report.
type Result struct {
	Skin   *osu.Skin
	Output *Output
	Report *Report
	Modes  []int
}

// Convert turns a parsed Quaver skin into an osu!mania skin per opts.
func Convert(sk *quaver.Skin, opts Options) (*Result, error) {
	modes := opts.KeyModes
	if modes == nil {
		modes = sk.PresentKeyModes()
	}
	if len(modes) == 0 {
		return nil, fmt.Errorf("no convertible keymodes (4K/7K) found in this skin")
	}

	out := newOutput()
	rep := &Report{}
	skin := osu.NewSkin()
	skin.General = buildGeneral(sk, opts.Name, opts.Author)

	for _, mode := range modes {
		cfg := sk.KeyModeOrEmpty(mode)
		columnSize := cfg.Int("ColumnSize", 0)
		receptorPos := cfg.Int("ReceptorPosOffsetY", 0)
		rW, rH := imgDims(sk.Source, fmt.Sprintf("%dk/Receptors/receptor-up-1.png", mode))

		hitPos := opts.HitPosition
		if hitPos < 0 {
			hitPos = HitPosition(receptorPos, cfg.Int("HitPosOffsetY", 0), columnSize, rW, rH)
		}

		mania := buildMania(cfg, mode, hitPos)
		mapKeymodeAssets(sk.Source, mode, columnSize, receptorPos, rW, rH, out, mania.KV, rep)
		skin.Mania = append(skin.Mania, mania)
		rep.position(maniaSummary(mode, mania))
	}
	rep.suppress("judgement line")
	rep.suppress("column dividers")

	mapUniversal(sk.Source, out, skin.Fonts, rep, opts)
	computeDropped(sk.Source, modes, opts, rep)

	out.add("skin.ini", []byte(skin.Serialize()))
	rep.warn("Health-bar rotation and exact hit/combo/score positions are approximate — verify in-game.")

	return &Result{Skin: skin, Output: out, Report: rep, Modes: modes}, nil
}

// maniaSummary is a one-line summary of a keymode's computed positions.
func maniaSummary(mode int, m *osu.Mania) string {
	g := func(k string) string { v, _ := m.KV.Get(k); return v }
	cw := g("ColumnWidth")
	if i := strings.IndexByte(cw, ','); i >= 0 {
		cw = cw[:i] // all columns share a width; show one
	}
	return fmt.Sprintf("%dK  HitPosition %s · ColumnStart %s · ColumnWidth %s · Combo %s · Score %s",
		mode, g("HitPosition"), g("ColumnStart"), cw, g("ComboPosition"), g("ScorePosition"))
}

// imgDims returns a source image's pixel dimensions, or 0,0 if absent/unreadable.
func imgDims(src quaver.Source, path string) (w, h int) {
	data, err := src.ReadFile(path)
	if err != nil {
		return 0, 0
	}
	w, h, err = imageops.Size(data)
	if err != nil {
		return 0, 0
	}
	return w, h
}
