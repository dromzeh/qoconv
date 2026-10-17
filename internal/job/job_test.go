package job

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dromzeh/qoconv/internal/convert"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, b.Bytes())
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// quaverSkin writes a minimal 4K Quaver skin folder: ColumnSize 140 with a
// 250x400 receptor and HitPosOffsetY 176 puts the osu! HitPosition at 450.
func quaverSkin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skin.ini"), []byte("[General]\nName = Test Skin\nAuthor = Tester\n\n[4K]\nColumnSize = 140\nHitPosOffsetY = 176\n"))
	for c := 1; c <= 4; c++ {
		writePNG(t, filepath.Join(dir, "4k", "HitObjects", fmt.Sprintf("note-hitobject-%d.png", c)), 150, 75)
		writePNG(t, filepath.Join(dir, "4k", "Receptors", fmt.Sprintf("receptor-up-%d.png", c)), 250, 400)
	}
	writePNG(t, filepath.Join(dir, "Judgements", "judge-marv.png"), 200, 100)
	writePNG(t, filepath.Join(dir, "Numbers", "combo-0.png"), 40, 60)
	writePNG(t, filepath.Join(dir, "steam_workshop_preview.png"), 64, 36)
	return dir
}

// osuSkin writes a minimal 4K osu! skin folder using default mania-* names.
func osuSkin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skin.ini"), []byte("[General]\nName: Osu Skin\nAuthor: Someone\n\n[Mania]\nKeys: 4\nColumnWidth: 88,88,88,88\nHitPosition: 450\n"))
	for _, style := range []string{"1", "2"} {
		writePNG(t, filepath.Join(dir, "mania-note"+style+".png"), 150, 75)
		writePNG(t, filepath.Join(dir, "mania-key"+style+".png"), 140, 224)
	}
	return dir
}

func TestOpenDetectsDirection(t *testing.T) {
	q, err := Open(quaverSkin(t))
	if err != nil {
		t.Fatal(err)
	}
	if q.Direction != ToOsu || q.Name != "Test Skin" || q.Author != "Tester" || fmt.Sprint(q.Modes) != "[4]" {
		t.Errorf("Quaver skin opened as %+v", q)
	}
	if q.Banner() != "steam_workshop_preview.png" {
		t.Errorf("Banner = %q", q.Banner())
	}

	o, err := Open(osuSkin(t))
	if err != nil {
		t.Fatal(err)
	}
	if o.Direction != ToQuaver || o.Name != "Osu Skin" || fmt.Sprint(o.Modes) != "[4]" {
		t.Errorf("osu! skin opened as %+v", o)
	}
	if o.Direction.Ext() != ".qs" || q.Direction.Ext() != ".osk" {
		t.Error("wrong archive extensions")
	}
}

func TestTuneRenameWrite(t *testing.T) {
	s, err := Open(quaverSkin(t))
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Convert(convert.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]Field{}
	for _, f := range res.Fields(4) {
		got[f.Key] = f
	}
	if got["HitPosition"].Value != "450" || got["ColumnWidth"].Per != 4 || got["ColumnSpacing"].Per != 3 {
		t.Errorf("Fields = %+v", got)
	}
	if res.Fields(7) != nil {
		t.Error("Fields(7) should be nil for a 4K-only conversion")
	}

	if err := res.Set(4, "ColumnWidth", "70"); err != nil {
		t.Fatal(err)
	}
	if err := res.Set(4, "HitPosition", "lots"); err == nil {
		t.Error("Set accepted a non-number")
	}
	if err := res.Set(4, "NoteImage0", "1"); err == nil {
		t.Error("Set accepted an untunable key")
	}
	if err := res.Set(7, "HitPosition", "400"); err == nil {
		t.Error("Set accepted a keymode that wasn't converted")
	}
	res.Rename("Renamed / Skin", "")

	ini := string(res.Files["skin.ini"])
	for _, want := range []string{"ColumnWidth: 70,70,70,70", "Name: Renamed / Skin", "Author: Tester"} {
		if !strings.Contains(ini, want) {
			t.Errorf("skin.ini missing %q\n%s", want, ini)
		}
	}

	out := t.TempDir()
	w, err := res.Write(out, true)
	if err != nil {
		t.Fatal(err)
	}
	if w.Folder != filepath.Join(out, "Renamed - Skin") || w.Archive != filepath.Join(out, "Renamed - Skin.osk") {
		t.Errorf("Written = %+v", w)
	}
	disk, err := os.ReadFile(filepath.Join(w.Folder, "skin.ini"))
	if err != nil || !bytes.Equal(disk, res.Files["skin.ini"]) {
		t.Errorf("skin.ini on disk differs (err %v)", err)
	}
	if _, err := os.Stat(w.Archive); err != nil {
		t.Error(err)
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestPlayfieldOsu(t *testing.T) {
	s, _ := Open(quaverSkin(t))
	res, err := s.Convert(convert.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	pf, ok := res.Playfield(4)
	if !ok || len(pf.Columns) != 4 {
		t.Fatalf("Playfield = %+v, %v", pf, ok)
	}
	if !near(pf.HitY, 450.0/480) {
		t.Errorf("HitY = %v, want 450/480", pf.HitY)
	}
	c := pf.Columns[1]
	// ColumnStart 251 + one 88-wide column; key image 140x224 is drawn 224/768 tall.
	if !near(c.X, (251.0+88)/480) || !near(c.W, 88.0/480) || !near(c.ReceptorH, 224.0/768) || !near(c.ReceptorY, 1-224.0/768) {
		t.Errorf("column 2 = %+v", c)
	}
	// 150x75 note at WidthForNoteHeightScale 88 -> 44 tall.
	if !near(c.NoteH, 44.0/480) {
		t.Errorf("NoteH = %v, want 44/480", c.NoteH)
	}
	if pf.Judgement.Image == "" || pf.Combo.Image == "" {
		t.Errorf("judgement/combo sprites missing: %+v %+v", pf.Judgement, pf.Combo)
	}

	// Tuning moves the playfield.
	if err := res.Set(4, "HitPosition", "400"); err != nil {
		t.Fatal(err)
	}
	if pf, _ := res.Playfield(4); !near(pf.HitY, 400.0/480) {
		t.Errorf("HitY after tune = %v", pf.HitY)
	}
}

func TestPlayfieldQuaver(t *testing.T) {
	s, _ := Open(osuSkin(t))
	res, err := s.Convert(convert.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	pf, ok := res.Playfield(4)
	if !ok || len(pf.Columns) != 4 {
		t.Fatalf("Playfield = %+v, %v", pf, ok)
	}
	// The receptor round-trips to 224 tall at the bottom, and the hit line to
	// osu!'s HitPosition 450 (720 of 768).
	c := pf.Columns[0]
	if math.Abs(c.ReceptorH-224.0/768) > 0.002 || math.Abs(pf.HitY-720.0/768) > 0.002 {
		t.Errorf("column 1 = %+v, HitY %v", c, pf.HitY)
	}
	// ColumnSize 141 lanes, centred where osu!'s default ColumnStart 136 puts
	// the 4×88 stage: 136 + 176 = 312 of 480.
	mid := (pf.Columns[0].X + pf.Columns[3].X + pf.Columns[3].W) / 2
	if math.Abs(mid-312.0/480) > 0.005 || math.Abs(c.W-141.0/768) > 1e-6 {
		t.Errorf("stage centre %v, lane width %v", mid, c.W)
	}
}

func TestSanitize(t *testing.T) {
	for in, want := range map[string]string{
		`a/b\c:d`: "a-b-c-d",
		"  ":      "Converted Skin",
		`"x"?<y>`: "'x'(y)",
	} {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScanSkins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b mania", "skin.ini"), []byte("[Mania]\nKeys: 7\n"))
	writeFile(t, filepath.Join(dir, "A Mania", "Skin.ini"), []byte("[Mania]\nKeys: 4\n"))
	writeFile(t, filepath.Join(dir, "std only", "skin.ini"), []byte("[General]\nName: x\n"))
	writeFile(t, filepath.Join(dir, "five key", "skin.ini"), []byte("[Mania]\nKeys: 5\n"))
	writeFile(t, filepath.Join(dir, "loose.osk"), []byte("zip"))

	got := scanSkins(dir, "osu!", osuConvertible)
	if len(got) != 2 || got[0].Name != "A Mania" || got[1].Name != "b mania" || got[0].Game != "osu!" {
		t.Errorf("osu! skins = %+v", got)
	}

	q := t.TempDir()
	writeFile(t, filepath.Join(q, "by section", "skin.ini"), []byte("[7K]\nColumnSize = 60\n"))
	writePNG(t, filepath.Join(q, "by folder", "4k", "Receptors", "receptor-up-1.png"), 2, 2)
	writeFile(t, filepath.Join(q, "by folder", "skin.ini"), []byte("[General]\nName = x\n"))
	writeFile(t, filepath.Join(q, "nothing", "skin.ini"), []byte("[General]\nName = x\n"))
	if got := scanSkins(q, "Quaver", quaverConvertible); len(got) != 2 {
		t.Errorf("Quaver skins = %+v", got)
	}
	if scanSkins(filepath.Join(q, "missing"), "Quaver", quaverConvertible) != nil {
		t.Error("a missing folder should give no skins")
	}
}

// In Quaver the hit line hangs off the receptor, whose height follows
// ColumnSize; tuning ColumnSize must leave the hit line and light where they
// were, and overrides must give the same skin in any order.
func TestQuaverTuneKeepsHitLine(t *testing.T) {
	conv := func(t *testing.T, overrides [][2]string) *Result {
		t.Helper()
		s, _ := Open(osuSkin(t))
		res, err := s.Convert(convert.DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range overrides {
			if err := res.Set(4, o[0], o[1]); err != nil {
				t.Fatal(err)
			}
		}
		return res
	}
	field := func(res *Result, key string) string {
		for _, f := range res.Fields(4) {
			if f.Key == key {
				return f.Value
			}
		}
		return ""
	}

	res := conv(t, nil)
	hit, light := field(res, "HitPosition"), field(res, "LightPosition")
	if hit != "720" { // osu! HitPosition 450 × 1.6
		t.Fatalf("HitPosition = %s, want 720", hit)
	}
	if err := res.Set(4, "ColumnSize", "232"); err != nil {
		t.Fatal(err)
	}
	if field(res, "HitPosition") != hit || field(res, "LightPosition") != light {
		t.Errorf("ColumnSize moved the hit line %s -> %s or light %s -> %s",
			hit, field(res, "HitPosition"), light, field(res, "LightPosition"))
	}
	if pf, _ := res.Playfield(4); math.Abs(pf.HitY-720.0/768) > 0.002 {
		t.Errorf("preview hit line = %v, want 720/768", pf.HitY)
	}

	a := conv(t, [][2]string{{"ColumnSize", "200"}, {"HitPosition", "700"}})
	b := conv(t, [][2]string{{"HitPosition", "700"}, {"ColumnSize", "200"}})
	if !bytes.Equal(a.Files["skin.ini"], b.Files["skin.ini"]) {
		t.Errorf("override order changed the skin:\n%s\n---\n%s", a.Files["skin.ini"], b.Files["skin.ini"])
	}
	if field(a, "HitPosition") != "700" {
		t.Errorf("HitPosition = %s, want 700", field(a, "HitPosition"))
	}
}
