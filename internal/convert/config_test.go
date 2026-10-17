package convert

import (
	"testing"

	"github.com/dromzeh/qoconv/internal/quaver"
)

// sample4K is a synthetic 4K skin.ini with representative values (no real skin).
const sample4K = `[General]
Name = Sample Skin
Author = Sample Author
CenterCursor = True

[4K]
ColumnSize = 140
ColumnAlignment = 0
HitPosOffsetY = 176
ComboPosY = -120
JudgementBurstPosY = -85
StageReceptorPadding = 0
ReceptorsOverHitObjects = false
FlipNoteImagesOnUpscroll = false
TimingLineColor = 190,190,190
ColumnColor1 = 0,0,0
`

func TestBuildMania(t *testing.T) {
	sk := quaver.ParseString(sample4K)
	m := buildMania(sk.KeyModeConfig(4), 4, QuaverReceptor{ColumnSize: 140, TexW: 250, TexH: 400}, 450)

	want := map[string]string{
		"ColumnStart":             "251",
		"ColumnWidth":             "88,88,88,88",
		"WidthForNoteHeightScale": "88",
		"HitPosition":             "450",
		"LightPosition":           "340", // column lighting sits on the receptor top
		"NoteBodyStyle":           "0",   // stretch LN bodies like Quaver
		"ScorePosition":           "187",
		"ComboPosition":           "165",
		"KeysUnderNotes":          "1", // inverse of ReceptorsOverHitObjects=false
		"NoteFlipWhenUpsideDown":  "0",
		"JudgementLine":           "0", // hide osu!'s default hit line
		"ColourBarline":           "190,190,190",
		"ColourLight1":            "0,0,0",
		"ColourLight2":            "0,0,0,0",         // no ColumnColor2: Quaver shows no light
		"ColourHold":              "255,255,255,255", // combo stays static during holds, as in Quaver
		"ColumnLineWidth":         "0,0,0,0,0",
		"LightingNWidth":          "21,21,21,21", // 100% of a 1080-space texture
	}
	for k, v := range want {
		got, ok := m.KV.Get(k)
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	// ColumnSpacing should be omitted when zero.
	if _, ok := m.KV.Get("ColumnSpacing"); ok {
		t.Error("ColumnSpacing should be omitted when 0")
	}
}

// A skin.ini that omits ComboPosY/JudgementBurstPosY inherits Quaver's default
// skin values (-40 / 108), not 0 — the converted positions must reflect that.
func TestBuildManiaPositionDefaults(t *testing.T) {
	sk := quaver.ParseString("[4K]\nColumnSize = 140\n")
	m := buildMania(sk.KeyModeConfig(4), 4, QuaverReceptor{ColumnSize: 140}, 450)

	if got, _ := m.KV.Get("ComboPosition"); got != "215" { // (-40+384)/1.6
		t.Errorf("ComboPosition = %q, want 215", got)
	}
	if got, _ := m.KV.Get("ScorePosition"); got != "308" { // (108+384)/1.6
		t.Errorf("ScorePosition = %q, want 308", got)
	}
}

func TestBuildGeneralOverride(t *testing.T) {
	sk := quaver.ParseString(sample4K)
	g := buildGeneral(sk, "Custom Name", "")
	if v, _ := g.Get("Name"); v != "Custom Name" {
		t.Errorf("Name override = %q", v)
	}
	if v, _ := g.Get("Author"); v != "Sample Author" { // empty override falls back to the skin's
		t.Errorf("Author = %q, want fallback Sample Author", v)
	}
	if v, _ := g.Get("Version"); v != "latest" {
		t.Errorf("Version = %q, want latest", v)
	}
	if v, _ := g.Get("CursorCentre"); v != "1" {
		t.Errorf("CursorCentre = %q, want 1", v)
	}
	if v, _ := g.Get("CursorRotate"); v != "0" { // Quaver's cursor never spins
		t.Errorf("CursorRotate = %q, want 0", v)
	}
}

// Quaver defaults CenterCursor to false; osu! defaults CursorCentre to true.
func TestBuildGeneralCursorDefault(t *testing.T) {
	g := buildGeneral(quaver.ParseString("[General]\nName = X\n"), "", "")
	if v, _ := g.Get("CursorCentre"); v != "0" {
		t.Errorf("CursorCentre = %q, want 0", v)
	}
}

// Keys a skin omits come from Quaver's default skin (Bar unless DefaultSkin
// says otherwise), not zero.
func TestBuildManiaDefaultSkinFallback(t *testing.T) {
	bar := buildMania(quaver.ParseString("[4K]\n").KeyModeConfig(4), 4, QuaverReceptor{ColumnSize: 85}, 402)
	if got, _ := bar.KV.Get("ColumnWidth"); got != "53,53,53,53" { // Bar ColumnSize 85
		t.Errorf("Bar ColumnWidth = %q, want 53,...", got)
	}
	if got, _ := bar.KV.Get("KeysUnderNotes"); got != "0" { // Bar: receptors over notes
		t.Errorf("Bar KeysUnderNotes = %q, want 0", got)
	}
	arrow := quaver.ParseString("[4K]\nDefaultSkin = Arrow\n").KeyModeConfig(4)
	if got := arrow.Int("ColumnSize", 0); got != 90 {
		t.Errorf("Arrow ColumnSize = %d, want 90", got)
	}
	if got := arrow.Int("HitPosOffsetY", 0); got != 90 {
		t.Errorf("Arrow HitPosOffsetY = %d, want 90", got)
	}
}
