package job

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dromzeh/qoconv/internal/convert"
	"github.com/dromzeh/qoconv/internal/imageops"
)

// Playfield is a to-scale sketch of one keymode's stage as the target game
// lays out the converted skin, for previews. Lengths are fractions of the
// screen height on a 16:9 screen, so x runs from 0 to 16/9.
type Playfield struct {
	Mode          int      `json:"mode"`
	HitY          float64  `json:"hitY"` // where a note's bottom edge lands
	ReceptorsOver bool     `json:"receptorsOver"`
	Columns       []Column `json:"columns"`
	Judgement     Sprite   `json:"judgement"` // a "marvelous"/300g burst, centred on the stage
	Combo         Sprite   `json:"combo"`     // one combo digit; the preview repeats it
}

// Sprite is an image drawn centred on the stage at height Y (its centre) and
// H tall; Image is "" when the skin has none.
type Sprite struct {
	Image string  `json:"image"`
	Y     float64 `json:"y"`
	H     float64 `json:"h"`
}

// Column is one lane of a Playfield. Images are output file paths ("" when
// the skin has none).
type Column struct {
	X         float64 `json:"x"`
	W         float64 `json:"w"`
	Receptor  string  `json:"receptor"`
	ReceptorY float64 `json:"receptorY"` // top edge
	ReceptorH float64 `json:"receptorH"`
	Note      string  `json:"note"`
	NoteH     float64 `json:"noteH"`
}

// Playfield lays out a keymode of the result; ok is false if it has none.
func (r *Result) Playfield(mode int) (pf Playfield, ok bool) {
	sec := r.section(mode)
	if sec == nil {
		return Playfield{}, false
	}
	if r.Direction == ToQuaver {
		return r.quaverPlayfield(sec, mode), true
	}
	return r.osuPlayfield(sec, mode), true
}

// osuPlayfield follows osu!mania's legacy layout (480-tall units): columns
// from ColumnStart, key images bottom-anchored at native height (pixels are
// 768 units), notes WidthForNoteHeightScale-scaled landing on HitPosition.
func (r *Result) osuPlayfield(sec section, keys int) Playfield {
	widths := list(sec, "ColumnWidth", keys, convert.OsuDefaultColumnWidth)
	gaps := list(sec, "ColumnSpacing", keys-1, 0)
	noteScale := num(sec, "WidthForNoteHeightScale", 0)
	pf := Playfield{
		Mode:          keys,
		HitY:          convert.ClampHitPosition(num(sec, "HitPosition", convert.OsuDefaultHitPosition)) / 480,
		ReceptorsOver: num(sec, "KeysUnderNotes", 0) == 0,
	}
	x := num(sec, "ColumnStart", convert.OsuDefaultColumnStart)
	for i := range keys {
		w := widths[i]
		c := Column{X: x / 480, W: w / 480}
		if img := r.image(sec, fmt.Sprintf("KeyImage%d", i)); img != "" {
			_, h := r.size(img)
			c.Receptor, c.ReceptorH = img, float64(h)/768
			c.ReceptorY = 1 - c.ReceptorH
		}
		if img := r.image(sec, fmt.Sprintf("NoteImage%d", i)); img != "" {
			iw, ih := r.size(img)
			ns := noteScale
			if ns <= 0 {
				ns = w
			}
			if iw > 0 {
				c.Note, c.NoteH = img, ns*float64(ih)/float64(iw)/480
			}
		}
		pf.Columns = append(pf.Columns, c)
		x += w
		if i < len(gaps) {
			x += gaps[i]
		}
	}
	// Judgements are drawn at native size, settling at 0.7× (LegacyManiaJudgementPiece);
	// the combo at native size. Both centred at Position×1.6 in 768 units.
	pf.Judgement = r.sprite("mania-hit300g.png", num(sec, "ScorePosition", 300)/480, func(h int) float64 { return 0.7 * float64(h) / 768 })
	pf.Combo = r.sprite("combo-0.png", num(sec, "ComboPosition", 111)/480, func(h int) float64 { return float64(h) / 768 })
	return pf
}

// sprite sizes an output image for the preview, if it exists.
func (r *Result) sprite(name string, y float64, height func(px int) float64) Sprite {
	_, h := r.size(name)
	if h == 0 {
		return Sprite{}
	}
	return Sprite{Image: name, Y: y, H: height(h)}
}

// quaverPlayfield follows Quaver's layout (768-tall units): a centred stage
// shifted by ColumnAlignment (convert.QuaverStage), receptors ColumnSize wide
// and aspect-scaled with the bottom ReceptorPosOffsetY up
// (convert.QuaverReceptor), notes landing HitPosOffsetY below the first
// receptor's top.
func (r *Result) quaverPlayfield(sec section, keys int) Playfield {
	stage := convert.QuaverStage{
		Keys:                 keys,
		ColumnSize:           num(sec, "ColumnSize", 0),
		NotePadding:          num(sec, "NotePadding", 0),
		StageReceptorPadding: num(sec, "StageReceptorPadding", 0),
		ColumnAlignment:      num(sec, "ColumnAlignment", 0),
	}
	noteW := stage.ColumnSize
	if v := num(sec, "WidthForNoteHeightScale", 0); v > 0 {
		noteW = convert.QuaverPixels(v) // an int key Quaver leaves in 1080 units
	}
	lines := r.quaverLines(keys, sec)
	over, _ := sec.Get("ReceptorsOverHitObjects")
	pf := Playfield{Mode: keys, HitY: lines.hitLine() / 768, ReceptorsOver: strings.EqualFold(over, "true")}

	for i := range keys {
		c := Column{X: stage.ColumnX(i) / 768, W: stage.ColumnSize / 768}
		// Each lane sizes its receptor from its own texture.
		up := convert.ReceptorUpPath(keys, i+1)
		if w, h := r.size(up); w > 0 {
			rec := lines.receptor()
			rec.TexW, rec.TexH = w, h
			c.Receptor, c.ReceptorH, c.ReceptorY = up, rec.Height()/768, rec.Top()/768
		}
		note := fmt.Sprintf("%dk/HitObjects/note-hitobject-%d.png", keys, i+1)
		if w, h := r.size(note); w > 0 {
			c.Note, c.NoteH = note, noteW*float64(h)/float64(w)/768
		}
		pf.Columns = append(pf.Columns, c)
	}
	// Quaver centres both on the screen centre plus their offsets; judgements
	// are JudgementHitBurstScale tall, combo digits ComboDisplayScale% of the texture.
	pf.Judgement = r.sprite("Judgements/judge-marv.png", (384+num(sec, "JudgementBurstPosY", 108))/768,
		func(int) float64 { return num(sec, "JudgementHitBurstScale", 100) / 768 })
	pf.Combo = r.sprite("Numbers/combo-0.png", (384+num(sec, "ComboPosY", -40))/768,
		func(h int) float64 { return float64(h) * num(sec, "ComboDisplayScale", 45) / 100 / 768 })
	return pf
}

// image resolves an osu! image key to the output file it names.
func (r *Result) image(sec section, key string) string {
	v, ok := sec.Get(key)
	if !ok {
		return ""
	}
	if p := v + ".png"; r.Files[p] != nil {
		return p
	}
	return ""
}

func (r *Result) size(name string) (w, h int) {
	w, h, _ = imageops.Size(r.Files[name])
	return w, h
}

// num reads the first number of a skin.ini value, or def.
func num(sec section, key string, def float64) float64 {
	v, ok := sec.Get(key)
	if !ok {
		return def
	}
	first, _, _ := strings.Cut(v, ",")
	if f, err := strconv.ParseFloat(strings.TrimSpace(first), 64); err == nil {
		return f
	}
	return def
}

// list reads n comma-separated numbers, repeating the last (or def) to fill.
func list(sec section, key string, n int, def float64) []float64 {
	out := make([]float64, max(n, 0))
	v, _ := sec.Get(key)
	parts := strings.Split(v, ",")
	for i := range out {
		out[i] = def
		if i < len(parts) {
			if f, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64); err == nil {
				out[i] = f
				continue
			}
		}
		if i > 0 {
			out[i] = out[i-1]
		}
	}
	return out
}
