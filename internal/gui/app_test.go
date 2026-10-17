package gui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func quaverSkin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel string, data []byte) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var img bytes.Buffer
	png.Encode(&img, image.NewNRGBA(image.Rect(0, 0, 100, 100)))
	write("skin.ini", []byte("[General]\nName = GUI Skin\nAuthor = Tester\n\n[4K]\nColumnSize = 140\n"))
	for c := 1; c <= 4; c++ {
		write(fmt.Sprintf("4k/HitObjects/note-hitobject-%d.png", c), img.Bytes())
		write(fmt.Sprintf("4k/Receptors/receptor-up-%d.png", c), img.Bytes())
	}
	write("MainMenu/menu-background.png", img.Bytes())
	return dir
}

func get(t *testing.T, a *App, url string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	a.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec.Code
}

func TestAppFlow(t *testing.T) {
	out := t.TempDir()
	a := NewApp(Config{Version: "test", OutputDir: out})

	if _, err := a.Convert(Options{}); err == nil {
		t.Error("Convert before Load should fail")
	}
	if _, err := a.Load(t.TempDir()); err == nil || !strings.Contains(err.Error(), "no skin.ini") {
		t.Errorf("Load(empty dir) error = %v, want a friendly no-skin.ini error", err)
	}

	info, err := a.Load(quaverSkin(t))
	if err != nil {
		t.Fatal(err)
	}
	if info.From != "Quaver" || info.To != "osu!" || info.Ext != ".osk" || info.Output != out {
		t.Errorf("SkinInfo = %+v", info)
	}
	if info.Banner != "/skin/MainMenu/menu-background.png" || get(t, a, info.Banner) != http.StatusOK {
		t.Errorf("banner %q not served", info.Banner)
	}

	p, err := a.Convert(Options{Grades: true, Hitsounds: true, HealthCW: true,
		Overrides: []Override{{Mode: 4, Key: "HitPosition", Value: "420"}, {Mode: 7, Key: "HitPosition", Value: "1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Modes) != 1 || p.Modes[0].Fields[0].Value != "420" {
		t.Fatalf("override not applied: %+v", p.Modes)
	}

	note := fmt.Sprintf("/out/%d/qo-4k-note-1.png", p.Gen)
	if get(t, a, note) != http.StatusOK {
		t.Errorf("%s not served", note)
	}
	if get(t, a, fmt.Sprintf("/out/%d/qo-4k-note-1.png", p.Gen-1)) != http.StatusNotFound {
		t.Error("stale preview generation still served")
	}
	if get(t, a, fmt.Sprintf("/out/%d/skin.ini", p.Gen)) != http.StatusNotFound {
		t.Error("non-image file served")
	}

	view, err := a.Tune(4, "HitPosition", "300")
	if err != nil || view.Playfield.HitY != 300.0/480 {
		t.Errorf("Tune = %+v, %v", view.Playfield.HitY, err)
	}
	if _, err := a.Tune(4, "HitPosition", "x"); err == nil {
		t.Error("Tune accepted a non-number")
	}

	saved, err := a.Save(SaveRequest{Name: "Saved Skin", Output: out, Archive: true})
	if err != nil {
		t.Fatal(err)
	}
	ini, err := os.ReadFile(filepath.Join(saved.Folder, "skin.ini"))
	if err != nil || !strings.Contains(string(ini), "HitPosition: 300") || !strings.Contains(string(ini), "Name: Saved Skin") {
		t.Errorf("saved skin.ini (err %v):\n%s", err, ini)
	}
	if saved.Archive != filepath.Join(out, "Saved Skin.osk") || saved.Installed || saved.Game != "osu!" {
		t.Errorf("Saved = %+v", saved)
	}
}

// Conversions run unlocked, so a result must not be installed over a skin
// loaded meanwhile, nor over a newer conversion's.
func TestConvertDropsStaleResult(t *testing.T) {
	a := NewApp(Config{})
	if _, err := a.Load(quaverSkin(t)); err != nil {
		t.Fatal(err)
	}

	skin, req, err := a.beginConvert()
	if err != nil {
		t.Fatal(err)
	}
	res, err := convertSkin(skin, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Load(quaverSkin(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.install(skin, req, res); err == nil || !strings.Contains(err.Error(), "skin changed") {
		t.Errorf("install after Load = %v, want a skin-changed error", err)
	}
	if a.res != nil || a.gen != 0 {
		t.Errorf("stale result installed: res %v, gen %d", a.res != nil, a.gen)
	}

	skin, older, _ := a.beginConvert()
	_, newer, _ := a.beginConvert()
	res, err = convertSkin(skin, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.install(skin, older, res); err == nil {
		t.Error("an older conversion replaced a newer one")
	}
	p, err := a.install(skin, newer, res)
	if err != nil || p.Gen != 1 || a.res != res {
		t.Errorf("install = gen %d, %v", p.Gen, err)
	}
	if get(t, a, fmt.Sprintf("/out/%d/qo-4k-note-1.png", p.Gen)) != http.StatusOK {
		t.Error("installed result not served")
	}
}
