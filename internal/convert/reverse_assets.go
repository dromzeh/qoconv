package convert

import (
	"fmt"
	"image"
	"strings"

	"github.com/dromzeh/qoconv/internal/imageops"
	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
)

// revCtx carries the state of a reverse (osu! -> Quaver) conversion: the osu!
// source, the Quaver output being built, the report, and which source files
// were consumed (for the skipped-files count).
type revCtx struct {
	src      quaver.Source
	out      *Output
	rep      *Report
	consumed map[string]bool
}

func (rc *revCtx) markConsumed(path string) {
	rc.consumed[strings.ToLower(strings.ReplaceAll(path, "\\", "/"))] = true
}

// unconsumed counts source files no mapping touched.
func (rc *revCtx) unconsumed() int {
	n := 0
	for _, p := range rc.src.List() {
		if !rc.consumed[strings.ToLower(p)] {
			n++
		}
	}
	return n
}

// resolved is an osu! image located by name: its bytes, the path used, and
// whether it is an @2x variant (stable draws those at half scale, so metrics
// and pixels must be halved to match the @1x space Quaver expects).
type resolved struct {
	data []byte
	path string
	x2   bool
}

// resolve finds an osu! image by its skin.ini base name (no extension), trying
// the plain file, the first animation frame, then their @2x variants.
func (rc *revCtx) resolve(base string) (resolved, bool) {
	base = strings.ReplaceAll(base, "\\", "/")
	for _, c := range []struct {
		suffix string
		x2     bool
	}{{".png", false}, {"-0.png", false}, {"@2x.png", true}, {"-0@2x.png", true}} {
		p := base + c.suffix
		if data, err := rc.src.ReadFile(p); err == nil {
			rc.markConsumed(p)
			return resolved{data: data, path: p, x2: c.x2}, true
		}
	}
	return resolved{}, false
}

// copyImage resolves base and writes it to dst, decoded through fn (which may
// be nil). @2x sources are halved first. Falls back to copying the bytes
// unchanged when the image can't be decoded/re-encoded. Returns false only
// when no source image was found.
func (rc *revCtx) copyImage(base, dst, note string, fn func(image.Image) image.Image) bool {
	r, ok := rc.resolve(base)
	if !ok {
		return false
	}
	if fn == nil && !r.x2 {
		return rawCopy(rc.out, r.path, dst, r.data, rc.rep)
	}
	img, err := imageops.Decode(r.data)
	if err != nil {
		return rawCopy(rc.out, r.path, dst, r.data, rc.rep)
	}
	if r.x2 {
		b := img.Bounds()
		img = imageops.Scale(img, b.Dx()/2, b.Dy()/2)
	}
	if fn != nil {
		img = fn(img)
	}
	enc, err := imageops.Encode(img)
	if err != nil {
		return rawCopy(rc.out, r.path, dst, r.data, rc.rep)
	}
	rc.out.add(dst, enc)
	rc.rep.converted("%s -> %s%s", r.path, dst, note)
	return true
}

// columnStyles returns osu!'s default per-column image styles — the "1"/"2"/
// "S" suffixes of mania-note1/mania-note2/mania-noteS — used when a [Mania]
// block doesn't override NoteImage*/KeyImage* paths.
func columnStyles(keys int) []string {
	switch keys {
	case 4:
		return []string{"1", "2", "2", "1"}
	case 7:
		return []string{"1", "2", "1", "S", "1", "2", "1"}
	}
	out := make([]string, keys)
	for i := range out {
		out[i] = "1"
	}
	return out
}

// mapReverseKeymodeAssets writes one keymode's lane/stage/lighting assets and
// returns the drawn height of the first receptor (0 if none was found), which
// buildQuaverMode needs for HitPosOffsetY.
func mapReverseKeymodeAssets(rc *revCtx, cfg *osu.Section, keys, columnSize int) (recH int) {
	m := fmt.Sprintf("%dk", keys)
	styles := columnStyles(keys)

	for c := 1; c <= keys; c++ {
		style := styles[c-1]
		col := c - 1 // osu! columns are 0-based

		lane := []struct {
			key, def, dst string
			receptor      bool // resized for Quaver's receptor box (see mapReverseReceptor)
			flipV         bool // Quaver's LN tail is upside-down vs osu!'s NoteImage#T
			core          bool // absence is worth reporting
		}{
			{fmt.Sprintf("NoteImage%d", col), "mania-note" + style, fmt.Sprintf("%s/HitObjects/note-hitobject-%d.png", m, c), false, false, true},
			{fmt.Sprintf("NoteImage%dH", col), "mania-note" + style + "H", fmt.Sprintf("%s/HitObjects/note-holdhitobject-%d.png", m, c), false, false, false},
			{fmt.Sprintf("NoteImage%dL", col), "mania-note" + style + "L", fmt.Sprintf("%s/HitObjects/note-holdbody-%d.png", m, c), false, false, false},
			{fmt.Sprintf("NoteImage%dT", col), "mania-note" + style + "T", fmt.Sprintf("%s/HitObjects/note-holdend-%d.png", m, c), false, true, false},
			{fmt.Sprintf("KeyImage%d", col), "mania-key" + style, fmt.Sprintf("%s/Receptors/receptor-up-%d.png", m, c), true, false, true},
			{fmt.Sprintf("KeyImage%dD", col), "mania-key" + style + "D", fmt.Sprintf("%s/Receptors/receptor-down-%d.png", m, c), true, false, false},
		}
		for _, el := range lane {
			base := cfg.Str(el.key, el.def)
			var ok bool
			switch {
			case el.receptor:
				var h int
				h, ok = mapReverseReceptor(rc, base, el.dst, columnSize)
				if ok && c == 1 && !strings.HasSuffix(el.key, "D") {
					recH = h
				}
			case el.flipV:
				ok = rc.copyImage(base, el.dst, " (flipped vertically)", imageops.FlipVertical)
			default:
				ok = rc.copyImage(base, el.dst, "", nil)
			}
			if !ok && el.core {
				rc.rep.missing("%s (%s, column %d core image)", base, el.key, c)
			}
		}
	}

	mapReverseStage(rc, cfg, m)
	mapReverseLighting(rc, cfg, m)
	return recH
}

// mapReverseReceptor writes an osu! key image the way Quaver will read it.
// osu! forces the key image's width to ColumnWidth and draws its pixels 1:1
// tall; Quaver draws receptor-up ColumnSize wide, aspect-scaled tall, bottom
// on the screen edge (ReceptorPosOffsetY 0). Writing the image resized to
// ColumnSize × its own pixel height makes Quaver's aspect scaling reproduce
// osu!'s drawn size exactly. Returns the written height for HitPosOffsetY.
func mapReverseReceptor(rc *revCtx, base, dst string, columnSize int) (h int, ok bool) {
	r, found := rc.resolve(base)
	if !found {
		return 0, false
	}
	img, err := imageops.Decode(r.data)
	if err != nil || columnSize <= 0 {
		return 0, rawCopy(rc.out, r.path, dst, r.data, rc.rep)
	}
	b := img.Bounds()
	h = b.Dy()
	if r.x2 {
		h /= 2
	}
	enc, err := imageops.Encode(imageops.Scale(img, columnSize, h))
	if err != nil {
		return 0, rawCopy(rc.out, r.path, dst, r.data, rc.rep)
	}
	rc.out.add(dst, enc)
	rc.rep.converted("%s -> %s (resized to %dx%d, osu! receptor placement)", r.path, dst, columnSize, h)
	return h, true
}

func mapReverseStage(rc *revCtx, cfg *osu.Section, m string) {
	rc.copyImage(cfg.Str("StageLeft", "mania-stage-left"), m+"/Stage/stage-left-border.png", "", nil)
	rc.copyImage(cfg.Str("StageRight", "mania-stage-right"), m+"/Stage/stage-right-border.png", "", nil)
	// osu! centres the stage hint on the hit line; Quaver draws its bottom edge
	// there. Content below the line can't be represented, so copy unchanged and
	// flag the potential half-height shift.
	if rc.copyImage(cfg.Str("StageHint", "mania-stage-hint"), m+"/Stage/stage-hitposition-overlay.png", "", nil) {
		rc.rep.warn("%s: stage hint sits fully above Quaver's hit line (osu! centres it); it may look shifted by half its height.", strings.ToUpper(m))
	}
}

func mapReverseLighting(rc *revCtx, cfg *osu.Section, m string) {
	rc.copyImage(cfg.Str("StageLight", "mania-stage-light"), m+"/Lighting/column-lighting.png", "", nil)
	mapReverseLightingSheet(rc, cfg.Str("LightingN", "lightingN"), m+"/Lighting/hitlighting")
	mapReverseLightingSheet(rc, cfg.Str("LightingL", "lightingL"), m+"/Lighting/holdlighting")
}

// mapReverseLightingSheet collects an osu! lighting animation ("base-0.png",
// "base-1.png", … or a single "base.png") into Quaver's one-row spritesheet
// convention "{dst}@1x{frames}.png".
func mapReverseLightingSheet(rc *revCtx, base, dstBase string) {
	base = strings.ReplaceAll(base, "\\", "/")
	var frames []image.Image
	var srcPath string
	for i := 0; ; i++ {
		p := fmt.Sprintf("%s-%d.png", base, i)
		data, err := rc.src.ReadFile(p)
		if err != nil {
			break
		}
		img, derr := imageops.Decode(data)
		if derr != nil {
			break
		}
		rc.markConsumed(p)
		frames = append(frames, img)
		if srcPath == "" {
			srcPath = p
		}
	}
	if len(frames) == 0 {
		p := base + ".png"
		data, err := rc.src.ReadFile(p)
		if err != nil {
			return
		}
		img, derr := imageops.Decode(data)
		if derr != nil {
			return
		}
		rc.markConsumed(p)
		frames, srcPath = []image.Image{img}, p
	}
	enc, err := imageops.Encode(imageops.JoinRow(frames))
	if err != nil {
		return
	}
	dst := fmt.Sprintf("%s@1x%d.png", dstBase, len(frames))
	rc.out.add(dst, enc)
	rc.rep.converted("%s (+%d frames) -> %s", srcPath, len(frames)-1, dst)
}

// --- universal assets (judgements, health, cursor, pause, numbers, extras) ---

// mapReverseUniversal maps the keymode-independent assets. primary is the
// first converted [Mania] block, consulted for Hit* judgement path overrides
// (Quaver judgements are universal, so one block has to win).
func mapReverseUniversal(rc *revCtx, sf *osu.SkinFile, primary *osu.Section, opts Options) {
	judge := []struct{ key, def, q string }{
		{"Hit300g", "mania-hit300g", "judge-marv"},
		{"Hit300", "mania-hit300", "judge-perf"},
		{"Hit200", "mania-hit200", "judge-great"},
		{"Hit100", "mania-hit100", "judge-good"},
		{"Hit50", "mania-hit50", "judge-okay"},
		{"Hit0", "mania-hit0", "judge-miss"},
	}
	for _, j := range judge {
		rc.copyImage(primary.Str(j.key, j.def), "Judgements/"+j.q+".png", "", nil)
	}

	// Health bars: rotate back to vertical (inverse of the forward rotation).
	rot := func(img image.Image) image.Image { return imageops.Rotate90(img, !opts.RotateHealthCW) }
	rc.copyImage("scorebar-bg", "Health/health-background.png", " (rotated 90°)", rot)
	rc.copyImage("scorebar-colour", "Health/health-foreground.png", " (rotated 90°)", rot)

	rc.copyImage("cursor", "Cursor/main-cursor.png", "", nil)

	rc.copyImage("pause-continue", "Pause/pause-continue.png", "", nil)
	rc.copyImage("pause-retry", "Pause/pause-retry.png", "", nil)
	rc.copyImage("pause-back", "Pause/pause-back.png", "", nil)
	rc.copyImage("pause-overlay", "Pause/pause-background.png", "", nil)

	// Score/combo fonts -> Quaver's Numbers folder.
	scorePrefix := sf.Fonts.Str("ScorePrefix", "score")
	comboPrefix := sf.Fonts.Str("ComboPrefix", "combo")
	for i := 0; i <= 9; i++ {
		rc.copyImage(fmt.Sprintf("%s-%d", scorePrefix, i), fmt.Sprintf("Numbers/score-%d.png", i), "", nil)
		rc.copyImage(fmt.Sprintf("%s-%d", comboPrefix, i), fmt.Sprintf("Numbers/combo-%d.png", i), "", nil)
	}
	rc.copyImage(scorePrefix+"-percent", "Numbers/score-percent.png", "", nil)
	rc.copyImage(scorePrefix+"-dot", "Numbers/score-decimal.png", "", nil)

	if opts.Grades {
		grades := map[string]string{
			"ranking-X-small":  "grade-small-x",
			"ranking-SS-small": "grade-small-ss",
			"ranking-S-small":  "grade-small-s",
			"ranking-A-small":  "grade-small-a",
			"ranking-B-small":  "grade-small-b",
			"ranking-C-small":  "grade-small-c",
			"ranking-D-small":  "grade-small-d",
		}
		for o, q := range grades {
			rc.copyImage(o, "Grades/"+q+".png", "", nil)
		}
	}
	if opts.Hitsounds {
		sounds := map[string]string{
			"normal-hitnormal.wav":  "sound-hit.wav",
			"normal-hitclap.wav":    "sound-hitclap.wav",
			"normal-hitwhistle.wav": "sound-hitwhistle.wav",
			"normal-hitfinish.wav":  "sound-hitfinish.wav",
			"combobreak.wav":        "sound-combobreak.wav",
		}
		for o, q := range sounds {
			if data, err := rc.src.ReadFile(o); err == nil {
				rc.markConsumed(o)
				rawCopy(rc.out, o, "SFX/"+q, data, rc.rep)
			}
		}
	}
}
