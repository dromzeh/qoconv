package convert

import (
	"fmt"
	"image"
	"path"
	"strings"

	"github.com/dromzeh/qoconv/internal/imageops"
	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
)

// revCtx carries the state of a reverse (osu! -> Quaver) conversion: the osu!
// source, the Quaver output being built, the report, which source files were
// consumed (for the skipped-files count), and the skin's Version.
type revCtx struct {
	src      quaver.Source
	out      *Output
	rep      *Report
	consumed map[string]bool
	version  float64
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

// imageStem splits an osu! image name into its stem and extension. skin.ini
// names usually omit the extension ("mania-hit0") but may include it
// ("mania-hit0.png"); osu! accepts both.
func imageStem(name string) (stem, ext string) {
	name = strings.ReplaceAll(name, "\\", "/")
	if e := path.Ext(name); strings.EqualFold(e, ".png") || strings.EqualFold(e, ".jpg") {
		return strings.TrimSuffix(name, e), e
	}
	return name, ".png"
}

// resolve finds an osu! image by its skin.ini name the way osu! does
// (LegacySkin.GetTexture): the @2x file first, then the plain one, each also
// as the first frame of an animation.
func (rc *revCtx) resolve(name string) (resolved, bool) {
	stem, ext := imageStem(name)
	for _, c := range []struct {
		suffix string
		x2     bool
	}{{"@2x", true}, {"", false}, {"-0@2x", true}, {"-0", false}} {
		p := stem + c.suffix + ext
		if data, err := rc.src.ReadFile(p); err == nil {
			// The other resolution of the same image is accounted for too.
			rc.markConsumed(p)
			rc.markConsumed(strings.Replace(p, "@2x"+ext, ext, 1))
			rc.markConsumed(strings.Replace(p, ext, "@2x"+ext, 1))
			return resolved{data: data, path: p, x2: c.x2}, true
		}
	}
	return resolved{}, false
}

// decode resolves base and decodes it at @1x scale.
func (rc *revCtx) decode(base string) (image.Image, resolved, bool) {
	r, ok := rc.resolve(base)
	if !ok {
		return nil, r, false
	}
	img, err := imageops.Decode(r.data)
	if err != nil {
		return nil, r, false
	}
	if r.x2 {
		b := img.Bounds()
		img = imageops.Scale(img, max(1, b.Dx()/2), max(1, b.Dy()/2))
	}
	return img, r, true
}

// copyImage resolves base and writes it to dst at @1x scale, transformed by fn
// (nil = unchanged). An image that can't be decoded/re-encoded is copied
// unchanged. Returns false only when no source image was found.
func (rc *revCtx) copyImage(base, dst, note string, fn func(image.Image) image.Image) bool {
	img, r, ok := rc.decode(base)
	switch {
	case r.path == "":
		return false
	case !ok || (fn == nil && !r.x2):
		return rawCopy(rc.out, r.path, dst, r.data, rc.rep)
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

// keymode writes one keymode's lane/stage/lighting assets and returns the
// drawn height of the first receptor (0 if none was found), which
// buildQuaverMode needs for HitPosOffsetY.
func (rc *revCtx) keymode(cfg *osu.Section, keys, columnSize int) (recH float64) {
	m := fmt.Sprintf("%dk", keys)
	styles := columnStyles(keys)

	for c := 1; c <= keys; c++ {
		style := styles[c-1]
		col := c - 1 // osu! columns are 0-based
		img := func(key, def string) string { return cfg.Str(fmt.Sprintf(key, col), def) }
		obj := func(name string) string { return fmt.Sprintf("%s/HitObjects/%s-%d.png", m, name, c) }

		if !rc.copyImage(img("NoteImage%d", "mania-note"+style), obj("note-hitobject"), "", nil) {
			rc.rep.missing("%s (column %d core image)", img("NoteImage%d", "mania-note"+style), c)
		}
		rc.copyImage(img("NoteImage%dH", "mania-note"+style+"H"), obj("note-holdhitobject"), "", nil)
		rc.copyImage(img("NoteImage%dL", "mania-note"+style+"L"), obj("note-holdbody"), "", nil)
		// Quaver's LN end is upside-down relative to osu!'s NoteImage#T.
		rc.copyImage(img("NoteImage%dT", "mania-note"+style+"T"), obj("note-holdend"), " (flipped vertically)", imageops.FlipVertical)

		up := img("KeyImage%d", "mania-key"+style)
		h, ok := rc.receptor(up, ReceptorUpPath(keys, c), columnSize)
		switch {
		case !ok:
			rc.rep.missing("%s (column %d core image)", up, c)
		case c == 1:
			recH = h
		}
		rc.receptor(img("KeyImage%dD", "mania-key"+style+"D"), fmt.Sprintf("%s/Receptors/receptor-down-%d.png", m, c), columnSize)
	}

	rc.stage(cfg, m, keys, columnSize)
	rc.lighting(cfg, m, columnSize)
	return recH
}

// receptor writes an osu! key image the way Quaver will read it. osu! forces
// the key image's width to ColumnWidth and draws its pixels 1:1 tall; Quaver
// draws a receptor ColumnSize wide, aspect-scaled tall, bottom on the screen
// edge (ReceptorPosOffsetY 0). Writing the image resized to ColumnSize × its
// own @1x height makes Quaver's aspect scaling reproduce osu!'s drawn size
// exactly. Returns that height for HitPosOffsetY.
func (rc *revCtx) receptor(base, dst string, columnSize int) (h float64, ok bool) {
	img, r, ok := rc.decode(base)
	if r.path == "" {
		return 0, false
	}
	if !ok || columnSize <= 0 {
		return 0, rawCopy(rc.out, r.path, dst, r.data, rc.rep)
	}
	h = float64(img.Bounds().Dy())
	enc, err := imageops.Encode(imageops.Scale(img, columnSize, img.Bounds().Dy()))
	if err != nil {
		return 0, rawCopy(rc.out, r.path, dst, r.data, rc.rep)
	}
	rc.out.add(dst, enc)
	rc.rep.converted("%s -> %s (resized to %dx%.0f, osu! receptor placement)", r.path, dst, columnSize, h)
	return h, true
}

func (rc *revCtx) stage(cfg *osu.Section, m string, keys, columnSize int) {
	// osu! stretches the borders to the stage height at native width; Quaver
	// keeps their aspect at full height. A 768-tall canvas makes the two agree.
	border := func(img image.Image) image.Image { return imageops.Scale(img, img.Bounds().Dx(), 768) }
	rc.copyImage(cfg.Str("StageLeft", "mania-stage-left"), m+"/Stage/stage-left-border.png", " (stretched to full height)", border)
	rc.copyImage(cfg.Str("StageRight", "mania-stage-right"), m+"/Stage/stage-right-border.png", " (stretched to full height)", border)

	// osu! stretches the stage hint to the stage width at 0.9×1.6025 its native
	// height, centred on the hit line; Quaver draws it stage-wide, aspect-
	// scaled, with its bottom edge on the line. Match the size; the half below
	// the line can't be represented, so flag the shift.
	stageW := columnSize*keys + roundI(QuaverNotePadding(cfg.FirstCSVFloat("ColumnSpacing", 0)))*(keys-1)
	hint := func(img image.Image) image.Image {
		return imageops.Scale(img, max(1, stageW), max(1, roundI(float64(img.Bounds().Dy())*0.9*1.6025)))
	}
	if rc.copyImage(cfg.Str("StageHint", "mania-stage-hint"), m+"/Stage/stage-hitposition-overlay.png", " (resized to osu!'s hint size)", hint) {
		rc.rep.warn("%s: stage hint sits fully above Quaver's hit line (osu! centres it); it may look shifted by half its height.", strings.ToUpper(m))
	}
}

func (rc *revCtx) lighting(cfg *osu.Section, m string, columnSize int) {
	// osu! draws the stage light column-wide at native height; written
	// ColumnSize wide, Quaver's ColumnLightingScale 1 keeps that height.
	rc.copyImage(cfg.Str("StageLight", "mania-stage-light"), m+"/Lighting/column-lighting.png", "",
		func(img image.Image) image.Image { return imageops.Scale(img, max(1, columnSize), img.Bounds().Dy()) })
	rc.lightingSheet(cfg.Str("LightingN", "lightingN"), m+"/Lighting/hitlighting")
	rc.lightingSheet(cfg.Str("LightingL", "lightingL"), m+"/Lighting/holdlighting")
}

// lightingSheet collects an osu! animation ("base-0.png", "base-1.png", … or a
// single "base.png") into Quaver's one-row spritesheet "{dst}@1x{frames}.png".
func (rc *revCtx) lightingSheet(base, dstBase string) {
	frames, srcPath := rc.frames(base)
	if len(frames) == 0 {
		return
	}
	dst := fmt.Sprintf("%s@1x%d.png", dstBase, len(frames))
	enc, err := imageops.Encode(imageops.JoinRow(frames))
	if err != nil {
		rc.rep.warn("could not encode %s (from %s): %v", dst, srcPath, err)
		return
	}
	rc.out.add(dst, enc)
	rc.rep.converted("%s (+%d frames) -> %s", srcPath, len(frames)-1, dst)
}

// frames loads an osu! animation: "base-0.png", "base-1.png", … (or their @2x
// variants, halved), else a single "base.png".
func (rc *revCtx) frames(base string) ([]image.Image, string) {
	stem, ext := imageStem(base)
	var frames []image.Image
	var first string
	for i := 0; ; i++ {
		img, r, ok := rc.decode(fmt.Sprintf("%s-%d%s", stem, i, ext))
		if !ok {
			break
		}
		frames = append(frames, img)
		if first == "" {
			first = r.path
		}
	}
	if len(frames) > 0 {
		return frames, first
	}
	if img, r, ok := rc.decode(base); ok {
		return []image.Image{img}, r.path
	}
	return nil, ""
}

// --- universal assets (judgements, health, cursor, pause, numbers, extras) ---

// judgements maps osu!'s judgement images and returns the JudgementHitBurstScale
// that reproduces their size (0 if there were none).
//
// osu! draws judgements at native size and settles hits at 0.7× and misses at
// 1× (LegacyManiaJudgementPiece). Quaver draws every judgement at the same
// height, JudgementHitBurstScale. So that height becomes the tallest settled
// judgement, and each image is padded top and bottom so its art lands at its
// own settled size inside that box. primary is the [Mania] block whose Hit*
// path overrides win (Quaver judgements are universal).
func (rc *revCtx) judgements(primary *osu.Section) float64 {
	judge := []struct {
		key, def, q string
		settle      float64
	}{
		{"Hit300g", "mania-hit300g", "judge-marv", 0.7},
		{"Hit300", "mania-hit300", "judge-perf", 0.7},
		{"Hit200", "mania-hit200", "judge-great", 0.7},
		{"Hit100", "mania-hit100", "judge-good", 0.7},
		{"Hit50", "mania-hit50", "judge-okay", 0.7},
		{"Hit0", "mania-hit0", "judge-miss", 1},
	}
	type found struct {
		img    image.Image
		path   string
		q      string
		settle float64
	}
	var all []found
	height := 0.0
	for _, j := range judge {
		img, r, ok := rc.decode(primary.Str(j.key, j.def))
		if !ok {
			continue
		}
		all = append(all, found{img, r.path, j.q, j.settle})
		height = max(height, j.settle*float64(img.Bounds().Dy()))
	}
	for _, f := range all {
		h := f.img.Bounds().Dy()
		pad := max(roundI(height/f.settle)-h, 0)
		dst := "Judgements/" + f.q + ".png"
		enc, err := imageops.Encode(imageops.Pad(f.img, pad/2, 0, pad-pad/2, 0))
		if err != nil {
			rc.rep.warn("could not encode %s (from %s): %v", dst, f.path, err)
			continue
		}
		rc.out.add(dst, enc)
		rc.rep.converted("%s -> %s (padded to keep osu!'s size)", f.path, dst)
	}
	return height
}

// universal maps the keymode-independent assets.
func (rc *revCtx) universal(sf *osu.SkinFile, centred bool, opts Options) {
	// Health bars: rotate back to vertical (inverse of the forward rotation).
	rot := func(img image.Image) image.Image { return imageops.Rotate90(img, !opts.RotateHealthCW) }
	rc.copyImage("scorebar-bg", "Health/health-background.png", " (rotated 90°)", rot)
	rc.copyImage("scorebar-colour", "Health/health-foreground.png", " (rotated 90°)", rot)

	// Quaver squashes its cursor into a square; pad osu!'s instead, around the
	// centre or away from the top-left hotspot.
	rc.copyImage("cursor", "Cursor/main-cursor.png", " (padded square)",
		func(img image.Image) image.Image { return imageops.Square(img, centred) })

	rc.copyImage("pause-continue", "Pause/pause-continue.png", "", nil)
	rc.copyImage("pause-retry", "Pause/pause-retry.png", "", nil)
	rc.copyImage("pause-back", "Pause/pause-back.png", "", nil)
	rc.copyImage("pause-overlay", "Pause/pause-background.png", "", nil)

	// Score/combo fonts -> Quaver's Numbers folder. osu!'s combo prefix
	// defaults to the score font.
	scorePrefix := sf.Fonts.Str("ScorePrefix", "score")
	comboPrefix := sf.Fonts.Str("ComboPrefix", "score")
	for i := 0; i <= 9; i++ {
		rc.copyImage(fmt.Sprintf("%s-%d", scorePrefix, i), fmt.Sprintf("Numbers/score-%d.png", i), "", nil)
		rc.copyImage(fmt.Sprintf("%s-%d", comboPrefix, i), fmt.Sprintf("Numbers/combo-%d.png", i), "", nil)
	}
	rc.copyImage(scorePrefix+"-percent", "Numbers/score-percent.png", "", nil)
	rc.copyImage(scorePrefix+"-dot", "Numbers/score-decimal.png", "", nil)

	if opts.Grades {
		for _, g := range []struct{ o, q string }{
			{"ranking-X-small", "grade-small-x"},
			{"ranking-SS-small", "grade-small-ss"},
			{"ranking-S-small", "grade-small-s"},
			{"ranking-A-small", "grade-small-a"},
			{"ranking-B-small", "grade-small-b"},
			{"ranking-C-small", "grade-small-c"},
			{"ranking-D-small", "grade-small-d"},
		} {
			rc.copyImage(g.o, "Grades/"+g.q+".png", "", nil)
		}
	}
	if opts.Hitsounds {
		for _, s := range []struct{ o, q string }{
			{"normal-hitnormal.wav", "sound-hit.wav"},
			{"normal-hitclap.wav", "sound-hitclap.wav"},
			{"normal-hitwhistle.wav", "sound-hitwhistle.wav"},
			{"normal-hitfinish.wav", "sound-hitfinish.wav"},
			{"combobreak.wav", "sound-combobreak.wav"},
		} {
			if data, err := rc.src.ReadFile(s.o); err == nil {
				rc.markConsumed(s.o)
				rawCopy(rc.out, s.o, "SFX/"+s.q, data, rc.rep)
			}
		}
	}
}
