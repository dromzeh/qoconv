package convert

import (
	"fmt"
	"slices"
	"strings"

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
		modes = sk.DefaultKeyModes()
	}
	if len(modes) == 0 {
		return nil, fmt.Errorf("no convertible keymodes (1K-10K) found in this skin")
	}

	// Keymodes that fall back to the shared keymode read its textures as their own.
	c := &fwdCtx{src: sk.ResolvedSource(modes), out: newOutput(), rep: &Report{}}
	skin := osu.NewSkin()
	skin.General = buildGeneral(sk, opts.Name, opts.Author)
	c.fonts = skin.Fonts

	for _, mode := range modes {
		cfg := sk.KeyModeConfig(mode)
		rec := QuaverReceptor{ColumnSize: cfg.Float("ColumnSize", 0), ReceptorPosOffsetY: cfg.Float("ReceptorPosOffsetY", 0)}
		rec.TexW, rec.TexH = c.size(ReceptorUpPath(mode, 1))

		hitPos := opts.HitPosition
		if hitPos < 0 {
			hitPos = HitPosition(rec, cfg.Float("HitPosOffsetY", 0))
		}

		mania := buildMania(cfg, mode, rec, hitPos)
		c.keymode(mode, cfg, rec, mania.KV)
		skin.Mania = append(skin.Mania, mania)
		c.rep.position(maniaSummary(mode, mania))
	}
	c.rep.suppress("judgement line")
	c.rep.suppress("column dividers")

	c.universal(sk.KeyModeConfig(modes[0]), sk.General().Bool("CenterCursor", false), opts)
	c.dropped(modes, slices.ContainsFunc(modes, sk.UsesSharedK), opts)

	c.out.add("skin.ini", []byte(skin.Serialize()))
	c.rep.warn("Health-bar placement differs between the games; verify in-game.")

	return &Result{Skin: skin, Output: c.out, Report: c.rep, Modes: modes}, nil
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
