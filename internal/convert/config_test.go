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
	m := buildMania(sk.KeyMode(4), 4, 410)

	want := map[string]string{
		"ColumnStart":             "252",
		"ColumnWidth":             "88,88,88,88",
		"WidthForNoteHeightScale": "88",
		"HitPosition":             "410",
		"LightPosition":           "410",
		"ScorePosition":           "187",
		"ComboPosition":           "165",
		"KeysUnderNotes":          "1", // inverse of ReceptorsOverHitObjects=false
		"NoteFlipWhenUpsideDown":  "0",
		"JudgementLine":           "0", // hide osu!'s default hit line
		"ColourBarline":           "190,190,190",
		"ColourLight1":            "0,0,0",
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
}
