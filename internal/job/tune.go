package job

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dromzeh/qoconv/internal/convert"
)

// Field is one hand-tunable number in a keymode's output skin.ini: the
// layout values the converter computes and players most often nudge.
type Field struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Hint  string `json:"hint"`
	Value string `json:"value"`
	// Writes names the skin.ini key the field ends up in, when that isn't Key
	// (a field shown as an on-screen position but stored as an offset).
	Writes string `json:"writes,omitempty"`
	// Min and Max bound a sensible slider; typed values may go beyond them.
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	// Per is how many comma-separated copies the key holds (osu! repeats
	// per-column values, e.g. "ColumnWidth: 88,88,88,88"); 0 for a single value.
	// Fields show and take one number and fan it out on Set.
	Per int `json:"per"`
}

type fieldSpec struct {
	key, label, hint string
	min, max         float64
	per              func(keys int) int // nil = single value

	// Derived fields aren't a skin.ini key themselves: get computes the shown
	// value and set writes the keys behind it. nil = plain key.
	get    func(q quaverLines) float64
	set    func(q quaverLines, v float64)
	writes string
}

func perColumn(keys int) int { return keys }
func perGap(keys int) int    { return keys - 1 }

var osuFields = []fieldSpec{
	{key: "HitPosition", label: "Hit position", hint: "Where notes land, from the top (higher is lower on screen)", min: 240, max: 480},
	{key: "ColumnStart", label: "Column start", hint: "Left edge of the first column", min: 0, max: 640},
	{key: "ColumnWidth", label: "Column width", hint: "Width of every column", min: 10, max: 160, per: perColumn},
	{key: "ColumnSpacing", label: "Column spacing", hint: "Gap between columns", min: 0, max: 60, per: perGap},
	{key: "ComboPosition", label: "Combo position", hint: "Combo counter height, from the top", min: 0, max: 480},
	{key: "ScorePosition", label: "Judgement position", hint: "Judgement height, from the top", min: 0, max: 480},
	{key: "LightPosition", label: "Light position", hint: "Bottom of the column light, from the top", min: 0, max: 480},
}

// Quaver measures the hit line and column light from the receptor's top, and a
// receptor's height follows ColumnSize (it keeps its aspect ratio). So these
// are shown as positions on screen, and changing ColumnSize keeps them put.
var quaverFields = []fieldSpec{
	{key: "HitPosition", label: "Hit position", hint: "Where notes land, from the top of the screen", min: 384, max: 768,
		get: quaverLines.hitLine, set: quaverLines.setHitLine, writes: "HitPosOffsetY"},
	{key: "ColumnSize", label: "Column size", hint: "Width of every lane; the hit line stays put", min: 20, max: 250,
		set: quaverLines.setColumnSize},
	{key: "ColumnAlignment", label: "Column alignment", hint: "Stage offset from the screen centre", min: -600, max: 600},
	{key: "NotePadding", label: "Note padding", hint: "Gap between lanes", min: 0, max: 100},
	{key: "ComboPosY", label: "Combo position", hint: "Combo offset from the screen centre", min: -384, max: 384},
	{key: "JudgementBurstPosY", label: "Judgement position", hint: "Judgement offset from the screen centre", min: -384, max: 384},
	{key: "LightPosition", label: "Light position", hint: "Bottom of the column light, from the top of the screen", min: 0, max: 768,
		get: quaverLines.lightLine, set: quaverLines.setLightLine, writes: "ColumnLightingOffsetY"},
}

// quaverLines is a converted Quaver keymode's hit line and column light (768
// units), which both hang off the first lane's receptor (convert.QuaverReceptor).
type quaverLines struct {
	sec        section
	texW, texH int // receptor-up-1 as written; 0 when there is none
}

func (r *Result) quaverLines(mode int, sec section) quaverLines {
	w, h := r.size(convert.ReceptorUpPath(mode, 1))
	return quaverLines{sec: sec, texW: w, texH: h}
}

// receptor reads the first receptor's placement from the section as it is
// now: setColumnSize changes it between reads.
func (q quaverLines) receptor() convert.QuaverReceptor {
	return convert.QuaverReceptor{
		ColumnSize:         num(q.sec, "ColumnSize", 0),
		ReceptorPosOffsetY: num(q.sec, "ReceptorPosOffsetY", 0),
		TexW:               q.texW,
		TexH:               q.texH,
	}
}

func (q quaverLines) hitLine() float64 { return q.receptor().Top() + num(q.sec, "HitPosOffsetY", 0) }

func (q quaverLines) lightLine() float64 {
	return q.receptor().Top() - num(q.sec, "ColumnLightingOffsetY", 0)
}

// setHitLine moves the hit line to y, keeping the hit lighting centred on it.
func (q quaverLines) setHitLine(y float64) {
	rec := q.receptor()
	offset := y - rec.Top()
	q.sec.Set("HitPosOffsetY", formatNum(offset))
	q.sec.Set("HitLightingY", formatNum(convert.QuaverHitLightingY(offset, rec.Height())))
}

func (q quaverLines) setLightLine(y float64) {
	q.sec.Set("ColumnLightingOffsetY", formatNum(q.receptor().Top()-y))
}

// setColumnSize resizes the lanes (and so the receptors) without moving the
// hit line or column light on screen.
func (q quaverLines) setColumnSize(v float64) {
	hit, light := q.hitLine(), q.lightLine()
	q.sec.Set("ColumnSize", formatNum(v))
	q.setHitLine(hit)
	q.setLightLine(light)
}

func formatNum(v float64) string { return strconv.FormatFloat(math.Round(v), 'f', -1, 64) }

func (r *Result) specs() []fieldSpec {
	if r.Direction == ToQuaver {
		return quaverFields
	}
	return osuFields
}

// Fields returns a keymode's tunable values as currently set.
func (r *Result) Fields(mode int) []Field {
	sec := r.section(mode)
	if sec == nil {
		return nil
	}
	q := r.quaverLines(mode, sec)
	out := make([]Field, 0, len(r.specs()))
	for _, s := range r.specs() {
		f := Field{Key: s.key, Label: s.label, Hint: s.hint, Writes: s.writes, Min: s.min, Max: s.max}
		if s.get != nil {
			f.Value = formatNum(s.get(q))
		} else {
			v, _ := sec.Get(s.key)
			first, _, _ := strings.Cut(v, ",")
			if f.Value = strings.TrimSpace(first); f.Value == "" {
				f.Value = "0"
			}
		}
		if s.per != nil {
			f.Per = s.per(mode)
		}
		out = append(out, f)
	}
	return out
}

// Set overrides one tunable value in a keymode's skin.ini and re-renders it.
// value must be a number; per-column keys get it in every column. Derived
// fields are order-independent: setting A then B gives the same skin as B
// then A, so a list of overrides can be replayed in any order.
func (r *Result) Set(mode int, key, value string) error {
	sec := r.section(mode)
	if sec == nil {
		return fmt.Errorf("%dK isn't part of this conversion", mode)
	}
	var spec *fieldSpec
	for i, s := range r.specs() {
		if s.key == key {
			spec = &r.specs()[i]
		}
	}
	if spec == nil {
		return fmt.Errorf("%s can't be tuned", key)
	}
	value = strings.TrimSpace(value)
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("%s: %q isn't a number", spec.label, value)
	}
	switch {
	case spec.set != nil:
		spec.set(r.quaverLines(mode, sec), v)
	case spec.per != nil:
		if n := spec.per(mode); n > 0 {
			sec.Set(key, strings.TrimSuffix(strings.Repeat(value+",", n), ","))
		}
	default:
		sec.Set(key, value)
	}
	r.Files["skin.ini"] = []byte(r.render())
	return nil
}
