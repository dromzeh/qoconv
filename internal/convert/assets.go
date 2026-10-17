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

// Output is the flat set of output skin files (forward-slash path -> bytes).
type Output struct {
	Files map[string][]byte
}

func newOutput() *Output { return &Output{Files: map[string][]byte{}} }

func (o *Output) add(name string, data []byte) { o.Files[name] = data }

// blankPNG returns a 1×1 transparent PNG, used to override a default element
// the source skin doesn't have, so the target game's default doesn't leak in.
func blankPNG() []byte {
	b, _ := imageops.Encode(imageops.Blank(1, 1))
	return b
}

// rawCopy writes data to out as dst unchanged and records the conversion. It is
// also the fallback when an image can't be decoded/encoded for a transform.
func rawCopy(out *Output, srcPath, dst string, data []byte, rep *Report) bool {
	out.add(dst, data)
	rep.converted("%s -> %s", srcPath, dst)
	return true
}

// fwdCtx carries a forward (Quaver -> osu!) conversion: the Quaver source,
// the osu! output being built, and the report.
type fwdCtx struct {
	src   quaver.Source
	out   *Output
	rep   *Report
	fonts *osu.KV // the osu! [Fonts] section, set when number fonts map
}

// blank writes a transparent placeholder unless the file was already produced.
func (c *fwdCtx) blank(name string) {
	if _, ok := c.out.Files[name]; !ok {
		c.out.add(name, blankPNG())
	}
}

// copy copies src[p] to dst unchanged. It returns false (recording nothing)
// when the file is absent; callers decide whether absence is noteworthy.
func (c *fwdCtx) copy(p, dst string) bool {
	data, err := c.src.ReadFile(p)
	if err != nil {
		return false
	}
	return rawCopy(c.out, p, dst, data, c.rep)
}

// load decodes the image at p. Quaver also accepts a "{name}@{rows}x{cols}.png"
// spritesheet wherever it takes a single image, so failing an exact match the
// sheet's first frame is used.
func (c *fwdCtx) load(p string) (img image.Image, used string, ok bool) {
	if data, err := c.src.ReadFile(p); err == nil {
		if img, err := imageops.Decode(data); err == nil {
			return img, p, true
		}
		return nil, "", false
	}
	sheet, rows, cols, found := findSheet(c.src, path.Dir(p), strings.TrimSuffix(path.Base(p), ".png"))
	if !found {
		return nil, "", false
	}
	data, err := c.src.ReadFile(sheet)
	if err != nil {
		return nil, "", false
	}
	sheetImg, err := imageops.Decode(data)
	if err != nil {
		return nil, "", false
	}
	return imageops.SplitGrid(sheetImg, rows, cols)[0], sheet, true
}

// image writes src[p] transformed by fn (nil = unchanged) to dst. A file that
// exists but can't be decoded is copied raw rather than lost. Returns false
// only when no source image was found.
func (c *fwdCtx) image(p, dst, note string, fn func(image.Image) image.Image) bool {
	img, used, ok := c.load(p)
	if !ok {
		return c.copy(p, dst) // absent (false), or undecodable: keep the bytes
	}
	if fn == nil && used == p {
		return c.copy(p, dst)
	}
	if fn != nil {
		img = fn(img)
	}
	enc, err := imageops.Encode(img)
	if err != nil {
		return c.copy(used, dst)
	}
	c.out.add(dst, enc)
	c.rep.converted("%s -> %s%s", used, dst, note)
	return true
}

// size returns an image's pixel size (first frame for spritesheets), or 0,0.
func (c *fwdCtx) size(p string) (w, h int) {
	if data, err := c.src.ReadFile(p); err == nil {
		if w, h, err := imageops.Size(data); err == nil {
			return w, h
		}
		return 0, 0
	}
	if img, _, ok := c.load(p); ok {
		return img.Bounds().Dx(), img.Bounds().Dy()
	}
	return 0, 0
}

// --- per-keymode assets (notes, holds, receptors, stage, lighting) ---

// keymode maps one keymode's lane, stage and lighting images, setting the
// matching image keys on the osu! [Mania] block.
func (c *fwdCtx) keymode(mode int, cfg *quaver.Section, rec QuaverReceptor, kv *osu.KV) {
	m := fmt.Sprintf("%dk", mode)
	for col := 1; col <= mode; col++ {
		c.lane(mode, col, cfg, rec, kv)
	}
	c.stage(m, mode, cfg, kv)
	c.columnLight(m, mode, cfg, rec, kv)
	c.lighting(m, "hitlighting", "qo-lightingN", "LightingN", cfg.Int("HitLightingScale", 100), cfg, rec, kv)
	c.lighting(m, "holdlighting", "qo-lightingL", "LightingL", cfg.Int("HoldLightingScale", cfg.Int("HitLightingScale", 100)), cfg, rec, kv)
}

// lane maps one column's notes, long-note parts and receptors.
//
// osu! hold-note suffixes are H=head, L=body, T=tail (the community gist had H
// and L swapped). When the Quaver skin lacks an optional part, osu! would fall
// back to its own default skin's texture, so point the key at the closest
// Quaver image instead: the note for a missing LN head, the released receptor
// for a missing pressed one.
func (c *fwdCtx) lane(mode, col int, cfg *quaver.Section, rec QuaverReceptor, kv *osu.KV) {
	m := fmt.Sprintf("%dk", mode)
	idx := col - 1 // osu! columns are 0-based
	obj := func(name string) string { return fmt.Sprintf("%s/HitObjects/%s-%d.png", m, name, col) }
	set := func(key, dst string) { kv.Set(fmt.Sprintf(key, idx), strings.TrimSuffix(dst, ".png")) }
	dst := func(prefix string) string { return fmt.Sprintf("%s-%d.png", prefix, col) }

	note := c.image(obj("note-hitobject"), dst("qo-note"), "", nil)
	if note {
		set("NoteImage%d", dst("qo-note"))
	} else {
		c.rep.missing("%s (column %d core image)", obj("note-hitobject"), col)
	}

	switch {
	case c.image(obj("note-holdhitobject"), dst("qo-head"), "", nil):
		set("NoteImage%dH", dst("qo-head"))
	case note:
		set("NoteImage%dH", dst("qo-note"))
	}
	if c.image(obj("note-holdbody"), dst("qo-body"), "", nil) {
		set("NoteImage%dL", dst("qo-body"))
	}
	// Quaver's LN end is upside-down relative to osu!'s NoteImage#T, which osu!
	// flips in downscroll (LegacyHoldNoteTailPiece). DrawLongNoteEnd = False
	// hides it entirely.
	if !cfg.Bool("DrawLongNoteEnd", true) {
		c.blank(dst("qo-tail"))
		set("NoteImage%dT", dst("qo-tail"))
	} else if c.image(obj("note-holdend"), dst("qo-tail"), " (flipped vertically)", imageops.FlipVertical) {
		set("NoteImage%dT", dst("qo-tail"))
	}

	// Each lane sizes its receptor from its own receptor-up texture.
	up := ReceptorUpPath(mode, col)
	laneRec := rec
	laneRec.TexW, laneRec.TexH = c.size(up)
	if c.receptor(up, dst("qo-key"), laneRec) {
		set("KeyImage%d", dst("qo-key"))
	} else {
		// osu! only uses a skin's mania elements at all when it has key images
		// (ManiaLegacySkinTransformer), so never leave one unset.
		c.blank(dst("qo-key"))
		set("KeyImage%d", dst("qo-key"))
		c.rep.missing("%s (column %d core image)", up, col)
	}
	down := fmt.Sprintf("%s/Receptors/receptor-down-%d.png", m, col)
	if c.receptor(down, dst("qo-keyD"), laneRec) {
		set("KeyImage%dD", dst("qo-keyD"))
	} else {
		set("KeyImage%dD", dst("qo-key"))
	}
}

// receptor re-renders a receptor the way Quaver draws it. Quaver sizes the
// sprite from the lane's receptor-up texture: ColumnSize wide, aspect-scaled
// tall, bottom edge ReceptorPosOffsetY above the screen bottom; the pressed
// texture is stretched into the same box. osu! forces a key image's width to
// ColumnWidth, draws its pixels 1:1 into the 768-tall space, and bottom-anchors
// it (ignoring HitPosition). So: resample the whole canvas (the art's placement
// inside it matters) to that box, then pad transparent rows below it.
func (c *fwdCtx) receptor(p, dst string, rec QuaverReceptor) bool {
	if !rec.Known() {
		return c.image(p, dst, "", nil)
	}
	w, h, pad := roundI(rec.ColumnSize), roundI(rec.Height()), ReceptorBottomPad(rec.ReceptorPosOffsetY)
	note := fmt.Sprintf(" (scaled to %dx%d + %dpx bottom pad, Quaver receptor placement)", w, h, pad)
	return c.image(p, dst, note, func(img image.Image) image.Image {
		return imageops.Pad(imageops.Scale(img, w, h), 0, 0, pad, 0)
	})
}

func (c *fwdCtx) stage(m string, keys int, cfg *quaver.Section, kv *osu.KV) {
	// Quaver stretches the borders to full screen height keeping their aspect
	// (Stage.cs); osu! stretches only the height and keeps the native width. So
	// bake Quaver's width in. StageReceptorPadding pushes Quaver's borders that
	// far out from the columns; osu! draws them flush, so pad the inner side.
	pad := roundI(cfg.Float("StageReceptorPadding", 0))
	borders := []struct {
		name, dst, key string
		left           bool
	}{
		{"stage-left-border", "qo-stage-left.png", "StageLeft", true},
		{"stage-right-border", "qo-stage-right.png", "StageRight", false},
	}
	for _, b := range borders {
		ok := c.image(fmt.Sprintf("%s/Stage/%s.png", m, b.name), b.dst, " (resized to Quaver's full-height width)", func(img image.Image) image.Image {
			s := img.Bounds().Size()
			img = imageops.Scale(img, max(1, roundI(float64(s.X)*768/float64(s.Y))), s.Y)
			if b.left {
				return imageops.Pad(img, 0, pad, 0, 0)
			}
			return imageops.Pad(img, 0, 0, 0, pad)
		})
		if !ok {
			c.blank(b.dst)
			c.rep.suppress("stage borders")
		}
		kv.Set(b.key, strings.TrimSuffix(b.dst, ".png"))
	}

	// Quaver draws the hit-position overlay across the whole stage, aspect-
	// scaled, with its BOTTOM edge on the hit line. osu! stretches StageHint to
	// the stage width but draws it 0.9×1.6025 its native height, CENTRED on the
	// line (LegacyHitTarget). So size it so osu!'s stretch lands on Quaver's
	// height, then pad below by that height to move the bottom edge to centre.
	stageW := QuaverStage{
		Keys:                 keys,
		ColumnSize:           cfg.Float("ColumnSize", 0),
		NotePadding:          cfg.Float("NotePadding", 0),
		StageReceptorPadding: cfg.Float("StageReceptorPadding", 0),
	}.Width()
	hint := c.image(m+"/Stage/stage-hitposition-overlay.png", "qo-stage-hint.png",
		" (resized for osu!'s hint scaling, bottom edge on the hit line)", func(img image.Image) image.Image {
			s := img.Bounds().Size()
			w := max(1, roundI(stageW))
			h := max(1, roundI(float64(s.Y)*float64(w)/float64(s.X)/(0.9*1.6025)))
			return imageops.Pad(imageops.Scale(img, w, h), 0, 0, h, 0)
		})
	if !hint {
		// Blank osu!'s default stage-hint line by filename and by key.
		c.blank("qo-stage-hint.png")
		c.blank("mania-stage-hint.png")
		c.rep.suppress("stage-hint line")
	}
	kv.Set("StageHint", "qo-stage-hint")
	// stage-bgmask / stage-distant-overlay have no osu! equivalent (dropped).
}

// columnLight maps Quaver's column lighting (the glow over a pressed lane).
// Quaver draws it ColumnSize wide and ColumnLightingScale×aspect tall, tinted
// by ColumnColorN; osu! draws StageLight at the column width and native
// height, tinted by ColourLightN (set in buildMania), bottom at LightPosition.
func (c *fwdCtx) columnLight(m string, keys int, cfg *quaver.Section, rec QuaverReceptor, kv *osu.KV) {
	_, tinted := columnColors(cfg, keys)
	p := m + "/Lighting/column-lighting.png"
	ok := tinted && c.image(p, "qo-stage-light.png", " (resized to Quaver's lighting size)", func(img image.Image) image.Image {
		s := img.Bounds().Size()
		w := max(1, roundI(rec.ColumnSize))
		h := max(1, roundI(cfg.Float("ColumnLightingScale", 1)*float64(w)*float64(s.Y)/float64(s.X)))
		return imageops.Scale(img, w, h)
	})
	if ok {
		kv.Set("StageLight", "qo-stage-light")
		return
	}
	if c.src.Exists(p) {
		c.rep.warn("%s: column lighting has no ColumnColor set, so Quaver never shows it; left off.", strings.ToUpper(m))
	}
	c.blank("mania-stage-light.png")
	c.rep.suppress("column glow")
}

// lighting maps a hit/hold lighting animation ("{base}@RxC.png" spritesheet or
// a single "{base}.png") into osu! frames "{dstBase}-0.png", "-1.png", ….
//
// Quaver centres the lighting on the receptor's centre plus HitLightingX/Y;
// osu! centres it on the hit line. Padding each frame on the opposite side by
// twice the offset moves the art by exactly that offset. The frames' size is
// set by LightingNWidth/LightingLWidth in buildMania.
func (c *fwdCtx) lighting(m, base, dstBase, key string, scale int, cfg *quaver.Section, rec QuaverReceptor, kv *osu.KV) {
	frames, used := c.frames(m+"/Lighting", base)
	if len(frames) == 0 || roundI(LightingWidth(scale)) <= 0 {
		c.blank(dstBase + "-0.png")
		kv.Set(key, dstBase)
		return
	}
	var dx, dy int
	if px := lightingPixel(scale); rec.Known() && px > 0 {
		dx = roundI(cfg.Float("HitLightingX", 0) / px)
		dy = roundI((rec.Height()/2 + cfg.Float("HitLightingY", 0) - cfg.Float("HitPosOffsetY", 0)) / px)
	}
	for i, fr := range frames {
		fr = imageops.Pad(fr, 2*dy, -2*dx, -2*dy, 2*dx)
		name := fmt.Sprintf("%s-%d.png", dstBase, i)
		enc, err := imageops.Encode(fr)
		if err != nil {
			c.rep.warn("could not encode %s (from %s): %v", name, used, err)
			continue
		}
		c.out.add(name, enc)
	}
	kv.Set(key, dstBase)
	c.rep.converted("%s -> %s-{0..%d}.png [%s]", used, dstBase, len(frames)-1, key)
}

// frames loads an animation from dir: a "{base}@RxC.png" spritesheet split into
// frames, or a single "{base}.png" as one frame.
func (c *fwdCtx) frames(dir, base string) ([]image.Image, string) {
	if sheet, rows, cols, ok := findSheet(c.src, dir, base); ok {
		if data, err := c.src.ReadFile(sheet); err == nil {
			if img, err := imageops.Decode(data); err == nil {
				return imageops.SplitGrid(img, rows, cols), sheet
			}
			c.rep.warn("could not decode %s", sheet)
		}
	}
	p := dir + "/" + base + ".png"
	if data, err := c.src.ReadFile(p); err == nil {
		if img, err := imageops.Decode(data); err == nil {
			return []image.Image{img}, p
		}
		c.rep.warn("could not decode %s", p)
	}
	return nil, ""
}

func findSheet(src quaver.Source, dir, base string) (sheetPath string, rows, cols int, ok bool) {
	prefix := strings.ToLower(dir + "/" + base + "@")
	for _, p := range src.List() {
		if strings.HasPrefix(strings.ToLower(p), prefix) {
			if _, r, cl, ok := imageops.ParseSheetName(path.Base(p)); ok {
				return p, r, cl, true
			}
		}
	}
	return "", 0, 0, false
}

// --- universal assets (judgements, health, cursor, pause, numbers, extras) ---

// Quaver's cursor is drawn 40×40 on its 1080-tall screen whatever the
// texture's size or shape (Wobble Cursor); osu! draws cursor.png at native
// size in its 768 space, @2x at half. These are that size in each file.
const (
	cursorSize   = 28 // 40 × 768/1080 = 28.4
	cursorSize2x = 57
)

// universal maps the keymode-independent assets. primary is the first
// converted keymode's config: Quaver keeps the judgement/number/health scales
// per keymode, but osu! has one set of these images.
func (c *fwdCtx) universal(primary *quaver.Section, centerCursor bool, opts Options) {
	c.judgements(primary.Float("JudgementHitBurstScale", 100))
	c.health(primary, opts.RotateHealthCW)
	c.cursor(centerCursor)

	c.copy("Pause/pause-continue.png", "pause-continue.png")
	c.copy("Pause/pause-retry.png", "pause-retry.png")
	c.copy("Pause/pause-back.png", "pause-back.png")
	c.copy("Pause/pause-background.png", "pause-overlay.png")

	c.numbers(primary)

	// Quaver has no hit-burst particles, combo bursts, or kiai stars; blank osu!'s
	// defaults so they don't spawn on the converted skin.
	for _, name := range []string{
		"particle50.png", "particle100.png", "particle300.png",
		"comboburst.png", "comboburst-mania.png", "star.png", "star2.png",
		"mania-stage-bottom.png", // Quaver has no bottom stage border
	} {
		c.blank(name)
	}
	c.rep.suppress("hit particles")
	c.rep.suppress("combo bursts")
	c.rep.suppress("kiai stars")

	if opts.Grades {
		c.grades()
	}
	if opts.Hitsounds {
		c.sounds()
	}
}

// judgements maps Quaver's judgement bursts to osu!'s default mania-hit*
// filenames (auto-loaded; no skin.ini key needed). Quaver draws them
// JudgementHitBurstScale tall (aspect kept). osu! draws them at native size
// but animates the scale (LegacyManiaJudgementPiece): hits pop and settle at
// 0.7× for most of their life, misses settle at 1×. Size each so the settled
// size matches Quaver's. Quaver spritesheets become osu! frame animations.
func (c *fwdCtx) judgements(height float64) {
	judge := []struct {
		q, o   string
		settle float64
	}{
		{"judge-marv", "mania-hit300g", 0.7},
		{"judge-perf", "mania-hit300", 0.7},
		{"judge-great", "mania-hit200", 0.7},
		{"judge-good", "mania-hit100", 0.7},
		{"judge-okay", "mania-hit50", 0.7},
		{"judge-miss", "mania-hit0", 1},
	}
	for _, j := range judge {
		frames, used := c.frames("Judgements", j.q)
		if len(frames) == 0 {
			continue
		}
		target := height / j.settle
		for i, fr := range frames {
			s := fr.Bounds().Size()
			fr = imageops.Scale(fr, max(1, roundI(target*float64(s.X)/float64(s.Y))), max(1, roundI(target)))
			name := j.o + ".png"
			if len(frames) > 1 {
				name = fmt.Sprintf("%s-%d.png", j.o, i)
			}
			enc, err := imageops.Encode(fr)
			if err != nil {
				c.rep.warn("could not encode %s (from %s): %v", name, used, err)
				continue
			}
			c.out.add(name, enc)
		}
		c.rep.converted("%s -> %s (%d frame(s), scaled to Quaver's judgement size)", used, j.o, len(frames))
	}
}

// health maps Quaver's health bar to osu!'s scorebar. Quaver's default bar is
// vertical (HealthBarType Vertical); osu!'s is always horizontal, so a vertical
// bar is rotated. Quaver draws it at HealthBarScale% of the texture.
func (c *fwdCtx) health(cfg *quaver.Section, cw bool) {
	vertical := !strings.EqualFold(cfg.Str("HealthBarType", "Vertical"), "Horizontal")
	scale := cfg.Float("HealthBarScale", 100) / 100
	fn := func(img image.Image) image.Image {
		img = imageops.ScaleBy(img, scale)
		if vertical {
			img = imageops.Rotate90(img, cw)
		}
		return img
	}
	note := ""
	if vertical {
		note = " (rotated 90°)"
	}
	if !c.image("Health/health-background.png", "scorebar-bg.png", note, fn) {
		c.blank("scorebar-bg.png") // otherwise osu!'s whole default bar shows
	}
	c.image("Health/health-foreground.png", "scorebar-colour.png", note, fn)
	// Quaver has no health marker; blank osu!'s old-style ones.
	for _, name := range []string{"scorebar-ki.png", "scorebar-kidanger.png", "scorebar-kidanger2.png"} {
		c.blank(name)
	}
}

// cursor maps Quaver's cursor to osu!'s: forced square like Quaver draws it
// (padded, so the art keeps its shape and a non-centred hotspot stays at the
// top-left), at Quaver's on-screen size. Quaver has no trail, middle piece or
// smoke, so osu!'s defaults for those are blanked.
func (c *fwdCtx) cursor(centered bool) {
	if img, used, ok := c.load("Cursor/main-cursor.png"); ok {
		sq := imageops.Square(img, centered)
		for _, v := range []struct {
			name string
			size int
		}{{"cursor.png", cursorSize}, {"cursor@2x.png", cursorSize2x}} {
			enc, err := imageops.Encode(imageops.Scale(sq, v.size, v.size))
			if err != nil {
				c.rep.warn("could not encode %s (from %s): %v", v.name, used, err)
				continue
			}
			c.out.add(v.name, enc)
		}
		c.rep.converted("%s -> cursor.png, cursor@2x.png (Quaver's 40px cursor size)", used)
	}
	for _, name := range []string{"cursortrail.png", "cursormiddle.png", "cursor-smoke.png"} {
		c.blank(name)
	}
	c.rep.suppress("cursor trail")
}

// numbers maps Quaver's number fonts to osu!'s score/combo fonts. Quaver draws
// combo digits at ComboDisplayScale% and score digits at ScoreDisplayScale%
// (both 768 units), while osu! draws the combo at native size and the score
// counter at 0.96× (LegacyScoreCounter); bake the difference into the images.
func (c *fwdCtx) numbers(cfg *quaver.Section) {
	comboScale := cfg.Float("ComboDisplayScale", 45) / 100
	scoreScale := cfg.Float("ScoreDisplayScale", 18) / 100 / 0.96
	scaled := func(p, dst string, f float64) bool {
		return c.image(p, dst, "", func(img image.Image) image.Image { return imageops.ScaleBy(img, f) })
	}

	score, combo := 0, 0
	for i := 0; i <= 9; i++ {
		if scaled(fmt.Sprintf("Numbers/score-%d.png", i), fmt.Sprintf("score-%d.png", i), scoreScale) {
			score++
		}
		if scaled(fmt.Sprintf("Numbers/combo-%d.png", i), fmt.Sprintf("combo-%d.png", i), comboScale) {
			combo++
		}
	}
	scaled("Numbers/score-percent.png", "score-percent.png", scoreScale)
	scaled("Numbers/score-decimal.png", "score-dot.png", scoreScale)
	if score > 0 {
		c.fonts.Set("ScorePrefix", "score")
		c.fonts.Set("ScoreOverlap", "0")
	}
	if combo > 0 {
		c.fonts.Set("ComboPrefix", "combo")
		c.fonts.Set("ComboOverlap", "0")
	}
}

// grades is an opportunistic extra: Quaver letter grades -> osu! ranking-*.
func (c *fwdCtx) grades() {
	for _, g := range []struct{ q, o string }{
		{"grade-small-x", "ranking-X-small"}, // perfect
		{"grade-small-ss", "ranking-SS-small"},
		{"grade-small-s", "ranking-S-small"},
		{"grade-small-a", "ranking-A-small"},
		{"grade-small-b", "ranking-B-small"},
		{"grade-small-c", "ranking-C-small"},
		{"grade-small-d", "ranking-D-small"},
	} {
		c.copy("Grades/"+g.q+".png", g.o+".png")
	}
	// osu! has no F grade; use it for D only when the skin has no D.
	if _, ok := c.out.Files["ranking-D-small.png"]; !ok {
		c.copy("Grades/grade-small-f.png", "ranking-D-small.png")
	}
}

// sounds is an opportunistic extra: Quaver SFX -> osu! hitsounds.
func (c *fwdCtx) sounds() {
	for _, s := range []struct{ q, o string }{
		{"sound-hit", "normal-hitnormal.wav"},
		{"sound-hitclap", "normal-hitclap.wav"},
		{"sound-hitwhistle", "normal-hitwhistle.wav"},
		{"sound-hitfinish", "normal-hitfinish.wav"},
		{"sound-combobreak", "combobreak.wav"},
	} {
		c.copy("SFX/"+s.q+".wav", s.o)
	}
}

// dropped records top-level Quaver folders that have no osu! equivalent.
func (c *fwdCtx) dropped(modes []int, opts Options) {
	consumed := map[string]bool{
		"cursor": true, "judgements": true, "health": true,
		"numbers": true, "pause": true,
	}
	for _, n := range modes {
		consumed[fmt.Sprintf("%dk", n)] = true
	}
	if opts.Grades {
		consumed["grades"] = true
	}
	if opts.Hitsounds {
		consumed["sfx"] = true
	}
	seen := map[string]bool{}
	for _, p := range c.src.List() {
		top, _, nested := strings.Cut(p, "/")
		if !nested {
			continue // root files (skin.ini, credits) — not "dropped folders"
		}
		lt := strings.ToLower(top)
		if !consumed[lt] && !seen[lt] {
			seen[lt] = true
			c.rep.Dropped = append(c.rep.Dropped, top)
		}
	}
}
