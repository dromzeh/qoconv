package convert

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dromzeh/qoconv/internal/imageops"
	"github.com/dromzeh/qoconv/internal/quaver"
)

// sampleOsu4K mirrors the forward golden skin: column 1 uses skin.ini image
// overrides (qo-* names, as qoconv's own forward output would), the other
// columns fall back to osu!'s default mania-* filenames.
const sampleOsu4K = `[General]
Name: Rev Sample
Author: Rev Author
CursorCentre: 1

[Fonts]
ScorePrefix: score
ComboPrefix: combo

[Mania]
Keys: 4
ColumnStart: 252
ColumnWidth: 88,88,88,88
HitPosition: 450
ScorePosition: 308
ComboPosition: 215
KeysUnderNotes: 1
ColourBarline: 190,190,190,255
NoteImage0: qo-note-1
NoteImage0H: qo-head-1
NoteImage0L: qo-body-1
NoteImage0T: qo-tail-1
KeyImage0: qo-key-1
KeyImage0D: qo-keyD-1
`

func buildSampleOsuSkin(t *testing.T) string {
	t.Helper()
	files := map[string][]byte{"skin.ini": []byte(sampleOsu4K)}
	add := func(p string, w, h int) { files[p] = pngBytes(t, w, h) }

	// Column 1 overrides + default-styled images for columns 2-4 (styles 2,2,1).
	add("qo-note-1.png", 150, 150)
	add("qo-head-1.png", 150, 150)
	add("qo-body-1.png", 128, 32)
	add("qo-tail-1.png", 128, 102)
	add("qo-key-1.png", 140, 224)
	add("qo-keyD-1.png", 140, 224)
	for _, style := range []string{"1", "2"} {
		add("mania-note"+style+".png", 150, 150)
		add("mania-note"+style+"H.png", 150, 150)
		add("mania-note"+style+"L.png", 128, 32)
		add("mania-note"+style+"T.png", 128, 102)
		add("mania-key"+style+".png", 140, 224)
		add("mania-key"+style+"D.png", 140, 224)
	}

	add("mania-stage-left.png", 43, 677)
	add("mania-stage-right.png", 43, 677)
	add("mania-stage-hint.png", 566, 30)
	add("mania-stage-light.png", 140, 300)
	add("lightingN-0.png", 50, 50)
	add("lightingN-1.png", 50, 50)
	add("lightingL.png", 60, 60)

	for _, j := range []string{"300g", "300", "200", "100", "50", "0"} {
		add("mania-hit"+j+".png", 270, 270)
	}
	add("scorebar-bg.png", 600, 37)
	add("scorebar-colour.png", 600, 32)
	add("cursor.png", 128, 128)
	add("pause-continue.png", 562, 168)
	add("pause-overlay.png", 1920, 1080)
	for i := 0; i <= 9; i++ {
		add("score-"+strconv.Itoa(i)+".png", 15, 58)
		add("combo-"+strconv.Itoa(i)+".png", 44, 70)
	}
	add("ranking-X-small.png", 68, 68)
	files["normal-hitnormal.wav"] = []byte("RIFFfake")
	// junk with no Quaver equivalent
	add("menu-background.png", 100, 100)
	add("hitcircle.png", 128, 128)

	path := filepath.Join(t.TempDir(), "sample.osk")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, data := range files {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReverseEndToEnd(t *testing.T) {
	src, err := quaver.OpenSource(buildSampleOsuSkin(t))
	if err != nil {
		t.Fatal(err)
	}
	if !IsOsuSkin(src) {
		t.Fatal("IsOsuSkin = false, want true")
	}

	res, err := Reverse(src, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Modes) != 1 || res.Modes[0] != 4 {
		t.Fatalf("Modes = %v, want [4]", res.Modes)
	}
	if res.Name != "Rev Sample" {
		t.Errorf("Name = %q", res.Name)
	}

	ini := string(res.Output.Files["skin.ini"])
	for _, want := range []string{
		"[General]",
		"Name = Rev Sample",
		"CenterCursor = True",
		"[4K]",
		"ColumnSize = 141",                // 88 * 1.6
		"ColumnAlignment = 2",             // ~0 up to rounding drift
		"ReceptorPosOffsetY = 0",          // receptors written bottom-anchored
		"HitPosOffsetY = 176",             // 450*1.6 - 768 + 224
		"ComboPosY = -40",                 // matches the forward golden skin
		"JudgementBurstPosY = 109",        // 1.6*(308-480)+384, rounded
		"ReceptorsOverHitObjects = False", // inverse of KeysUnderNotes 1
		"TimingLineColor = 190,190,190,255",
	} {
		if !strings.Contains(ini, want) {
			t.Errorf("skin.ini missing %q\n---\n%s", want, ini)
		}
	}

	wantFiles := []string{
		"4k/HitObjects/note-hitobject-1.png", // skin.ini override
		"4k/HitObjects/note-hitobject-4.png", // default mania-note1
		"4k/HitObjects/note-holdbody-2.png",  // default mania-note2L
		"4k/HitObjects/note-holdend-1.png",   // flipped tail
		"4k/Receptors/receptor-up-1.png",
		"4k/Receptors/receptor-down-4.png",
		"4k/Stage/stage-left-border.png",
		"4k/Stage/stage-hitposition-overlay.png",
		"4k/Lighting/column-lighting.png",
		"4k/Lighting/hitlighting@1x2.png",  // two lightingN frames
		"4k/Lighting/holdlighting@1x1.png", // single lightingL image
		"Judgements/judge-marv.png",
		"Judgements/judge-miss.png",
		"Health/health-background.png",
		"Cursor/main-cursor.png",
		"Pause/pause-continue.png",
		"Pause/pause-background.png",
		"Numbers/score-0.png",
		"Numbers/combo-9.png",
		"Grades/grade-small-x.png",
		"SFX/sound-hit.wav",
		"skin.ini",
	}
	for _, f := range wantFiles {
		if _, ok := res.Output.Files[f]; !ok {
			t.Errorf("output missing file %q", f)
		}
	}

	// Health bar rotated back to vertical: 600x37 -> 37x600.
	if w, h, err := imageops.Size(res.Output.Files["Health/health-background.png"]); err != nil || w != 37 || h != 600 {
		t.Errorf("health-background size = %dx%d (err %v), want 37x600 (rotated)", w, h, err)
	}
	// Receptor resized so Quaver's aspect scaling reproduces osu!'s drawn size:
	// ColumnSize wide, own pixel height tall.
	if w, h, err := imageops.Size(res.Output.Files["4k/Receptors/receptor-up-1.png"]); err != nil || w != 141 || h != 224 {
		t.Errorf("receptor-up-1 size = %dx%d (err %v), want 141x224", w, h, err)
	}
	// Lighting sheet: two 50x50 frames side by side.
	if w, h, err := imageops.Size(res.Output.Files["4k/Lighting/hitlighting@1x2.png"]); err != nil || w != 100 || h != 50 {
		t.Errorf("hitlighting sheet size = %dx%d (err %v), want 100x50", w, h, err)
	}

	// Unmapped osu! files are counted, not silently dropped.
	warns := strings.Join(res.Report.Warnings, "\n")
	if !strings.Contains(warns, "no Quaver equivalent") {
		t.Errorf("expected skipped-files warning, got:\n%s", warns)
	}
}

// A Quaver skin must not be detected as an osu! one.
func TestIsOsuSkinRejectsQuaver(t *testing.T) {
	src, err := quaver.OpenSource(buildSampleSkin(t))
	if err != nil {
		t.Fatal(err)
	}
	if IsOsuSkin(src) {
		t.Error("IsOsuSkin = true for a Quaver skin")
	}
}
