package convert

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dromzeh/qoconv/internal/imageops"
	"github.com/dromzeh/qoconv/internal/quaver"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{1, 2, 3, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func buildSampleSkin(t *testing.T) string {
	t.Helper()
	files := map[string][]byte{"skin.ini": []byte(sample4K)}
	add := func(p string, w, h int) { files[p] = pngBytes(t, w, h) }

	for c := 1; c <= 4; c++ {
		add("4k/Hitobjects/note-hitobject-"+strconv.Itoa(c)+".png", 150, 150)
		add("4k/Hitobjects/note-holdbody-"+strconv.Itoa(c)+".png", 128, 32)
		add("4k/Hitobjects/note-holdend-"+strconv.Itoa(c)+".png", 128, 102)
		add("4k/Hitobjects/note-holdhitobject-"+strconv.Itoa(c)+".png", 150, 150)
		add("4k/Receptors/receptor-up-"+strconv.Itoa(c)+".png", 250, 400)
		add("4k/Receptors/receptor-down-"+strconv.Itoa(c)+".png", 250, 400)
	}
	add("4k/Stage/stage-left-border.png", 43, 677)
	add("4k/Stage/stage-right-border.png", 474, 768)
	for _, j := range []string{"Marv", "Perf", "Great", "Good", "Okay", "Miss"} {
		add("Judgements/Judge-"+j+".png", 270, 270)
	}
	add("Health/health-background.png", 37, 600)
	add("Health/health-foreground.png", 32, 600)
	add("Cursor/main-cursor.png", 128, 128)
	for i := 0; i <= 9; i++ {
		add("Numbers/score-"+strconv.Itoa(i)+".png", 15, 58)
		add("Numbers/combo-"+strconv.Itoa(i)+".png", 44, 70)
	}
	add("Pause/Pause-Continue.png", 562, 168)
	add("Pause/Pause-Background.png", 1920, 1080)
	// junk that must be ignored / reported as dropped
	add("Assets/Bars/junk.png", 10, 10)
	add("MainMenu/menu-background.png", 100, 100)
	files["i dont take credit.txt"] = []byte("nope")

	path := filepath.Join(t.TempDir(), "sample.qs")
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

func TestConvertEndToEnd(t *testing.T) {
	sk, err := quaver.Load(buildSampleSkin(t))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Convert(sk, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Modes) != 1 || res.Modes[0] != 4 {
		t.Fatalf("Modes = %v, want [4]", res.Modes)
	}

	ini := string(res.Output.Files["skin.ini"])
	for _, want := range []string{
		"Keys: 4",
		"ColumnWidth: 88,88,88,88",
		"ColumnStart: 252",
		"HitPosition: 410",
		"ComboPosition: 165",
		"NoteImage0: qo-note-1",
		"NoteImage0H: qo-head-1", // hold head
		"NoteImage0L: qo-body-1", // hold body
		"NoteImage3T: qo-tail-4", // hold tail
		"KeyImage3D: qo-keyD-4",
		"StageLeft: qo-stage-left",
		"[Fonts]",
		"ScorePrefix: score",
	} {
		if !strings.Contains(ini, want) {
			t.Errorf("skin.ini missing %q\n---\n%s", want, ini)
		}
	}

	wantFiles := []string{
		"qo-note-1.png", "qo-note-4.png", "qo-head-1.png", "qo-body-1.png", "qo-tail-1.png",
		"qo-key-1.png", "qo-keyD-4.png",
		"qo-stage-left.png", "mania-hit300g.png", "mania-hit0.png",
		"scorebar-bg.png", "scorebar-colour.png", "cursor.png",
		"score-0.png", "combo-9.png", "pause-overlay.png", "skin.ini",
	}
	for _, f := range wantFiles {
		if _, ok := res.Output.Files[f]; !ok {
			t.Errorf("output missing file %q", f)
		}
	}

	// Health bar rotated: 37x600 -> 600x37.
	if w, h, err := imageops.Size(res.Output.Files["scorebar-bg.png"]); err != nil || w != 600 || h != 37 {
		t.Errorf("scorebar-bg size = %dx%d (err %v), want 600x37 (rotated)", w, h, err)
	}

	// Receptor: tall 250x400 source squared to ColumnSize (140) then bottom-padded
	// (42px for HitPosition 410) so the bottom-anchored circle reaches the line.
	if w, h, err := imageops.Size(res.Output.Files["qo-key-1.png"]); err != nil || w != 140 || h != 182 {
		t.Errorf("qo-key-1 size = %dx%d (err %v), want 140x182 (squared + padded)", w, h, err)
	}

	// Dropped folders reported.
	dropped := strings.Join(res.Report.Dropped, ",")
	if !strings.Contains(dropped, "MainMenu") || !strings.Contains(dropped, "Assets") {
		t.Errorf("Dropped = %v, want MainMenu and Assets", res.Report.Dropped)
	}
	if len(res.Report.Converted) == 0 {
		t.Error("expected non-empty Converted list")
	}
}
