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

// blankPNG returns a 1×1 transparent PNG, used to override an osu! default
// element that the Quaver skin doesn't provide (so the default doesn't leak in).
func blankPNG() []byte {
	b, _ := imageops.Encode(image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	return b
}

// Output is the flat set of osu! skin files (root filename -> bytes).
type Output struct {
	Files map[string][]byte
}

func newOutput() *Output { return &Output{Files: map[string][]byte{}} }

func (o *Output) add(name string, data []byte) { o.Files[name] = data }

// rawCopy writes data to out as dst unchanged and records the conversion. It is
// also the fallback when an image can't be decoded/encoded for a transform.
func rawCopy(out *Output, srcPath, dst string, data []byte, rep *Report) bool {
	out.add(dst, data)
	rep.converted("%s -> %s", srcPath, dst)
	return true
}

// copyAsset copies src[path] to out as dst. Returns false (and records nothing)
// if the source file is absent — callers decide whether absence is noteworthy.
func copyAsset(src quaver.Source, path string, out *Output, dst string, rep *Report) bool {
	data, err := src.ReadFile(path)
	if err != nil {
		return false
	}
	return rawCopy(out, path, dst, data, rep)
}

// mapImage reads src[path], applies fn, and writes the re-encoded result to out
// as dst (appending note — which may be "" — to the report line). If the image
// can't be decoded or re-encoded it falls back to copying the bytes unchanged.
// Returns false only when the source file is absent.
func mapImage(src quaver.Source, path string, out *Output, dst, note string, rep *Report, fn func(image.Image) image.Image) bool {
	data, err := src.ReadFile(path)
	if err != nil {
		return false
	}
	img, err := imageops.Decode(data)
	if err != nil {
		return rawCopy(out, path, dst, data, rep)
	}
	enc, err := imageops.Encode(fn(img))
	if err != nil {
		return rawCopy(out, path, dst, data, rep)
	}
	out.add(dst, enc)
	rep.converted("%s -> %s%s", path, dst, note)
	return true
}

// --- per-keymode assets (notes, holds, receptors, stage, lighting) ---

// laneElement maps a per-column Quaver element to an osu! NoteImage/KeyImage key.
type laneElement struct {
	quaverPrefix string // e.g. "note-hitobject"
	folder       string // e.g. "HitObjects"
	dstPrefix    string // e.g. "qo-note"
	keyFmt       string // e.g. "NoteImage%dH" (gets 0-based column)
	receptor     bool   // receptors need un-stretching (see mapReceptor)
	flipV        bool   // flip vertically (Quaver's LN tail is upside-down vs osu!)
}

// osu! hold-note suffixes are H=Head, L=Body, T=Tail (confirmed against the osu!
// wiki — the community gist had H/L swapped, which tiled the head circle down
// the body). Map Quaver's hold parts to the matching osu! suffix.
var laneElements = []laneElement{
	{"note-hitobject", "HitObjects", "qo-note", "NoteImage%d", false, false},
	{"note-holdhitobject", "HitObjects", "qo-head", "NoteImage%dH", false, false}, // H = head
	{"note-holdbody", "HitObjects", "qo-body", "NoteImage%dL", false, false},      // L = body
	{"note-holdend", "HitObjects", "qo-tail", "NoteImage%dT", false, true},        // T = tail (flipped)
	{"receptor-up", "Receptors", "qo-key", "KeyImage%d", true, false},
	{"receptor-down", "Receptors", "qo-keyD", "KeyImage%dD", true, false},
}

func mapKeymodeAssets(src quaver.Source, mode, columnSize, receptorPosOffsetY, recW, recH int, out *Output, kv *osu.KV, rep *Report) {
	m := fmt.Sprintf("%dk", mode)
	for c := 1; c <= mode; c++ {
		for _, el := range laneElements {
			srcPath := fmt.Sprintf("%s/%s/%s-%d.png", m, el.folder, el.quaverPrefix, c)
			dstBase := fmt.Sprintf("%s-%d", el.dstPrefix, c)
			key := fmt.Sprintf(el.keyFmt, c-1) // osu! columns are 0-based

			var ok bool
			switch {
			case el.receptor:
				ok = mapReceptor(src, srcPath, out, dstBase+".png", columnSize, receptorPosOffsetY, recW, recH, rep)
			case el.flipV:
				ok = mapFlippedV(src, srcPath, out, dstBase+".png", rep)
			default:
				ok = copyAsset(src, srcPath, out, dstBase+".png", rep)
			}
			if ok {
				kv.Set(key, dstBase) // osu! image keys omit the extension
			} else if el.quaverPrefix == "note-hitobject" || el.quaverPrefix == "receptor-up" {
				rep.missing("%s (column %d core image)", srcPath, c)
			}
		}
	}
	mapStage(src, m, out, kv, rep)
	mapLighting(src, m, out, kv, rep)
}

// mapReceptor re-renders a receptor the way Quaver draws it. Quaver sizes the
// receptor sprite from the receptor-up texture: ColumnSize wide, aspect-scaled
// tall (recH×ColumnSize/recW), bottom edge ReceptorPosOffsetY above the screen
// bottom; the pressed texture is stretched into the same box. osu! forces a key
// image's width to ColumnWidth, draws its pixels 1:1 into the 768-tall space,
// and anchors it to the stage bottom (ignoring HitPosition). So: resample the
// full canvas (no trimming — the art's placement inside it matters) to
// ColumnSize × aspect height, then pad transparent rows below it so the
// bottom-anchored image rises by ReceptorPosOffsetY, exactly as in Quaver.
// recW/recH are the receptor-up dimensions, used for BOTH states so the
// pressed image fills the same box as in Quaver.
func mapReceptor(src quaver.Source, path string, out *Output, dst string, columnSize, receptorPosOffsetY, recW, recH int, rep *Report) bool {
	data, err := src.ReadFile(path)
	if err != nil {
		return false
	}
	img, err := imageops.Decode(data)
	if err != nil || columnSize <= 0 || recW <= 0 {
		return rawCopy(out, path, dst, data, rep) // fall back to a raw copy
	}
	h := roundI(float64(recH) * float64(columnSize) / float64(recW))
	pad := ReceptorBottomPad(receptorPosOffsetY)
	img = imageops.PadBottom(imageops.Scale(img, columnSize, h), pad)
	enc, err := imageops.Encode(img)
	if err != nil {
		return rawCopy(out, path, dst, data, rep)
	}
	out.add(dst, enc)
	rep.converted("%s -> %s (scaled to %dx%d + %dpx bottom pad, Quaver receptor placement)", path, dst, columnSize, h, pad)
	return true
}

// mapFlippedV copies an image flipped top-to-bottom (used for the LN tail, which
// Quaver stores upside-down relative to osu!'s NoteImage#T).
func mapFlippedV(src quaver.Source, path string, out *Output, dst string, rep *Report) bool {
	return mapImage(src, path, out, dst, " (flipped vertically)", rep, imageops.FlipVertical)
}

func mapStage(src quaver.Source, m string, out *Output, kv *osu.KV, rep *Report) {
	for _, s := range []struct{ srcName, dstBase, key string }{
		{"stage-left-border", "qo-stage-left", "StageLeft"},
		{"stage-right-border", "qo-stage-right", "StageRight"},
	} {
		if copyAsset(src, fmt.Sprintf("%s/Stage/%s.png", m, s.srcName), out, s.dstBase+".png", rep) {
			kv.Set(s.key, s.dstBase)
		}
	}
	// osu! draws StageHint CENTRED on the hit line ("the judgement line is drawn
	// in the centre of the image" — wiki/LegacyHitTarget); Quaver draws the
	// overlay's BOTTOM edge on the hit line. Pad the canvas below by its own
	// height so the content's bottom edge becomes the canvas centre.
	hintPadded := mapImage(src, m+"/Stage/stage-hitposition-overlay.png", out, "qo-stage-hint.png",
		" (bottom-padded so its bottom edge sits on the hit line)", rep, func(img image.Image) image.Image {
			return imageops.PadBottom(img, img.Bounds().Dy())
		})
	if hintPadded {
		kv.Set("StageHint", "qo-stage-hint")
	} else {
		// No Quaver hit-position overlay -> blank osu!'s default stage-hint line
		// both by the default filename and via an explicit StageHint override.
		blank := blankPNG()
		out.add("qo-stage-hint.png", blank)
		out.add("mania-stage-hint.png", blank)
		kv.Set("StageHint", "qo-stage-hint")
		rep.suppress("stage-hint line")
	}
	// stage-bgmask / stage-distant-overlay have no osu! equivalent (dropped).
}

func mapLighting(src quaver.Source, m string, out *Output, kv *osu.KV, rep *Report) {
	if copyAsset(src, m+"/Lighting/column-lighting.png", out, "qo-stage-light.png", rep) {
		kv.Set("StageLight", "qo-stage-light")
	} else {
		// No Quaver column lighting -> blank osu!'s default column key-press glow.
		out.add("mania-stage-light.png", blankPNG())
		rep.suppress("column glow")
	}
	if !splitLighting(src, m, "hitlighting", out, "qo-lightingN", "LightingN", kv, rep) {
		blankLighting(out, kv, "qo-lightingN", "LightingN")
	}
	if !splitLighting(src, m, "holdlighting", out, "qo-lightingL", "LightingL", kv, rep) {
		blankLighting(out, kv, "qo-lightingL", "LightingL")
	}
}

// blankLighting points an osu! note/hold lighting key at a transparent frame so
// osu!'s default hit explosion doesn't show when Quaver has none.
func blankLighting(out *Output, kv *osu.KV, base, key string) {
	out.add(base+"-0.png", blankPNG())
	kv.Set(key, base)
}

// splitLighting finds a "{base}@RxC.png" spritesheet and explodes it into osu!
// animation frames "{dstBase}-0.png", "-1.png", … referenced by an osu! key.
func splitLighting(src quaver.Source, m, base string, out *Output, dstBase, key string, kv *osu.KV, rep *Report) bool {
	path, rows, cols, ok := findSheet(src, m+"/Lighting", base)
	if !ok {
		return false
	}
	data, err := src.ReadFile(path)
	if err != nil {
		return false
	}
	img, err := imageops.Decode(data)
	if err != nil {
		rep.warn("could not decode lighting sheet %s: %v", path, err)
		return false
	}
	for i, fr := range imageops.SplitGrid(img, rows, cols) {
		enc, err := imageops.Encode(fr)
		if err != nil {
			continue
		}
		out.add(fmt.Sprintf("%s-%d.png", dstBase, i), enc)
	}
	kv.Set(key, dstBase)
	rep.converted("%s -> %s-{0..%d}.png [%s]", path, dstBase, rows*cols-1, key)
	return true
}

func findSheet(src quaver.Source, dir, base string) (sheetPath string, rows, cols int, ok bool) {
	prefix := strings.ToLower(dir + "/" + base + "@")
	for _, p := range src.List() {
		if strings.HasPrefix(strings.ToLower(p), prefix) {
			if _, r, c, ok2 := imageops.ParseSheetName(path.Base(p)); ok2 {
				return p, r, c, true
			}
		}
	}
	return "", 0, 0, false
}

// --- universal assets (judgements, health, cursor, pause, numbers, extras) ---

func mapUniversal(src quaver.Source, out *Output, fonts *osu.KV, rep *Report, opts Options) {
	// Judgements -> default osu! filenames (auto-loaded; no skin.ini key needed).
	judge := []struct{ q, o string }{
		{"judge-marv", "mania-hit300g"},
		{"judge-perf", "mania-hit300"},
		{"judge-great", "mania-hit200"},
		{"judge-good", "mania-hit100"},
		{"judge-okay", "mania-hit50"},
		{"judge-miss", "mania-hit0"},
	}
	for _, j := range judge {
		copyAsset(src, "Judgements/"+j.q+".png", out, j.o+".png", rep)
	}

	// Health bars: rotate 90° (vertical Quaver -> horizontal osu! scorebar).
	rotateAsset(src, "Health/health-background.png", out, "scorebar-bg.png", opts.RotateHealthCW, rep)
	rotateAsset(src, "Health/health-foreground.png", out, "scorebar-colour.png", opts.RotateHealthCW, rep)

	copyAsset(src, "Cursor/main-cursor.png", out, "cursor.png", rep)

	// Pause menu.
	copyAsset(src, "Pause/pause-continue.png", out, "pause-continue.png", rep)
	copyAsset(src, "Pause/pause-retry.png", out, "pause-retry.png", rep)
	copyAsset(src, "Pause/pause-back.png", out, "pause-back.png", rep)
	copyAsset(src, "Pause/pause-background.png", out, "pause-overlay.png", rep)

	// Numbers -> osu! score/combo fonts.
	scoreN := 0
	for i := 0; i <= 9; i++ {
		if copyAsset(src, fmt.Sprintf("Numbers/score-%d.png", i), out, fmt.Sprintf("score-%d.png", i), rep) {
			scoreN++
		}
	}
	copyAsset(src, "Numbers/score-percent.png", out, "score-percent.png", rep)
	copyAsset(src, "Numbers/score-decimal.png", out, "score-dot.png", rep)
	comboN := 0
	for i := 0; i <= 9; i++ {
		if copyAsset(src, fmt.Sprintf("Numbers/combo-%d.png", i), out, fmt.Sprintf("combo-%d.png", i), rep) {
			comboN++
		}
	}
	if scoreN > 0 {
		fonts.Set("ScorePrefix", "score")
		fonts.Set("ScoreOverlap", "0")
	}
	if comboN > 0 {
		fonts.Set("ComboPrefix", "combo")
		fonts.Set("ComboOverlap", "0")
	}

	// Quaver has no hit-burst particles, combo bursts, or kiai stars; blank osu!'s
	// defaults so they don't spawn on the converted skin.
	for _, name := range []string{
		"particle50.png", "particle100.png", "particle300.png",
		"comboburst.png", "star.png", "star2.png",
		"mania-stage-bottom.png", // Quaver has no bottom stage border
	} {
		if _, exists := out.Files[name]; !exists {
			out.add(name, blankPNG())
		}
	}
	rep.suppress("hit particles")
	rep.suppress("combo bursts")
	rep.suppress("kiai stars")

	if opts.Grades {
		mapGrades(src, out, rep)
	}
	if opts.Hitsounds {
		mapSounds(src, out, rep)
	}
}

func rotateAsset(src quaver.Source, path string, out *Output, dst string, cw bool, rep *Report) {
	mapImage(src, path, out, dst, " (rotated 90°)", rep, func(img image.Image) image.Image {
		return imageops.Rotate90(img, cw)
	})
}

// mapGrades is an opportunistic extra: Quaver letter grades -> osu! ranking-*.
func mapGrades(src quaver.Source, out *Output, rep *Report) {
	grades := map[string]string{
		"grade-small-x":  "ranking-X-small", // perfect
		"grade-small-ss": "ranking-SS-small",
		"grade-small-s":  "ranking-S-small",
		"grade-small-a":  "ranking-A-small",
		"grade-small-b":  "ranking-B-small",
		"grade-small-c":  "ranking-C-small",
		"grade-small-d":  "ranking-D-small",
		"grade-small-f":  "ranking-D-small", // osu! has no F; reuse D
	}
	for q, o := range grades {
		copyAsset(src, "Grades/"+q+".png", out, o+".png", rep)
	}
}

// mapSounds is an opportunistic extra: Quaver SFX -> osu! hitsounds.
func mapSounds(src quaver.Source, out *Output, rep *Report) {
	sounds := map[string]string{
		"sound-hit":        "normal-hitnormal.wav",
		"sound-hitclap":    "normal-hitclap.wav",
		"sound-hitwhistle": "normal-hitwhistle.wav",
		"sound-hitfinish":  "normal-hitfinish.wav",
		"sound-combobreak": "combobreak.wav",
	}
	for q, o := range sounds {
		copyAsset(src, "SFX/"+q+".wav", out, o, rep)
	}
}

// computeDropped records top-level Quaver folders that have no osu! equivalent.
func computeDropped(src quaver.Source, modes []int, opts Options, rep *Report) {
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
	for _, p := range src.List() {
		i := strings.IndexByte(p, '/')
		if i < 0 {
			continue // root files (skin.ini, credits) — not "dropped folders"
		}
		top := p[:i]
		lt := strings.ToLower(top)
		if !consumed[lt] && !seen[lt] {
			seen[lt] = true
			rep.Dropped = append(rep.Dropped, top)
		}
	}
}
