package quaver

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// sampleIni is a synthetic skin.ini (no real skin involved). The typo'd Version
// mimics a real-world quirk the parser must tolerate; the numeric values are
// from a representative 4K skin.
const sampleIni = `[General]
Name = Sample Skin
Author = Sample Author
Version = lastest
CenterCursor = True

[4K]
; a comment line
ColumnSize = 140
ComboPosY = -120
HitPosOffsetY = 176
ReceptorsOverHitObjects = false
FlipNoteImagesOnUpscroll = false
TimingLineColor = 190,190,190
ColumnColor1 = 0,0,0
ColumnLightingScale = 1.0
`

func TestParseAndGetters(t *testing.T) {
	sk := parseSkinIni([]byte(sampleIni))

	g := sk.General()
	if got := g.Str("Name", ""); got != "Sample Skin" {
		t.Errorf("Name = %q, want %q", got, "Sample Skin")
	}
	if !g.Bool("CenterCursor", false) {
		t.Error("CenterCursor should parse True as true")
	}

	k := sk.KeyMode(4)
	if k == nil {
		t.Fatal("4K section missing")
	}
	if got := k.Int("ColumnSize", 0); got != 140 {
		t.Errorf("ColumnSize = %d, want 140", got)
	}
	if got := k.Int("ColumnLightingScale", -1); got != 1 { // "1.0" truncates to 1
		t.Errorf("ColumnLightingScale = %d, want 1", got)
	}
	if k.Bool("ReceptorsOverHitObjects", true) {
		t.Error("ReceptorsOverHitObjects should be false")
	}
	c, ok := k.Color("TimingLineColor")
	if !ok || c.R != 190 || c.G != 190 || c.B != 190 || c.HasAlpha {
		t.Errorf("TimingLineColor = %+v ok=%v, want 190,190,190 no-alpha", c, ok)
	}
	if sk.KeyMode(7) != nil {
		t.Error("7K should be absent")
	}
}

func TestParseColor(t *testing.T) {
	if c, ok := ParseColor("42,55,67"); !ok || c.String() != "42,55,67" {
		t.Errorf("rgb round-trip failed: %+v %q", c, c.String())
	}
	if c, ok := ParseColor(" 1, 2, 3, 4 "); !ok || !c.HasAlpha || c.String() != "1,2,3,4" {
		t.Errorf("rgba round-trip failed: %+v", c)
	}
	if _, ok := ParseColor("256,0,0"); ok {
		t.Error("out-of-range component should fail")
	}
	if _, ok := ParseColor("1,2"); ok {
		t.Error("two components should fail")
	}
}

// writeQS builds a minimal .qs (zip) for testing.
func writeQS(t *testing.T, files map[string][]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.qs")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadArchiveCaseInsensitive(t *testing.T) {
	qs := writeQS(t, map[string][]byte{
		"skin.ini":                           []byte(sampleIni),
		"4k/Hitobjects/note-hitobject-1.png": []byte("PNG1"),
		"Cursor/main-cursor.png":             []byte("CUR"),
		"Assets/Bars/junk.png":               []byte("JUNK"),
		"i dont take credit.txt":             []byte("nope"),
	})

	sk, err := Load(qs)
	if err != nil {
		t.Fatal(err)
	}
	if got := sk.General().Str("Author", ""); got != "Sample Author" {
		t.Errorf("Author = %q", got)
	}
	// Case-insensitive asset lookup.
	if !sk.Source.Exists("4K/HITOBJECTS/NOTE-HITOBJECT-1.PNG") {
		t.Error("case-insensitive Exists failed")
	}
	data, err := sk.Source.ReadFile("4k/hitobjects/note-hitobject-1.png")
	if err != nil || string(data) != "PNG1" {
		t.Errorf("ReadFile = %q, %v", data, err)
	}
	// Keymode detection: section + folder => [4], no 7K.
	modes := sk.PresentKeyModes()
	if len(modes) != 1 || modes[0] != 4 {
		t.Errorf("PresentKeyModes = %v, want [4]", modes)
	}
}

func TestLoadNestedSkinIni(t *testing.T) {
	qs := writeQS(t, map[string][]byte{
		"MySkin/skin.ini":                       []byte(sampleIni),
		"MySkin/4k/Stage/stage-left-border.png": []byte("L"),
	})
	sk, err := Load(qs)
	if err != nil {
		t.Fatal(err)
	}
	if sk.General().Str("Name", "") != "Sample Skin" {
		t.Error("nested skin.ini not rebased")
	}
	if !sk.Source.Exists("4k/Stage/stage-left-border.png") {
		t.Error("nested asset path not rebased to root")
	}
}
