package quaver

import (
	"slices"
	"testing"
)

func TestKeyModeConfigSharedK(t *testing.T) {
	sk := ParseString(`
[5K]
ColumnSize = 100

[SharedK]
DefaultSkin = Arrow
ColumnSize = 70
HitPosOffsetY = 33

[8K]
UseFallback = True

[9K]
UseFallback = False

[10K]
DefaultSkin = arrow
`)
	// 5K: its own value, then the skin's shared values. Arrow (named only by
	// [SharedK]) supplies the 5K lane fallbacks, but Quaver loaded the shared
	// defaults from Bar before it read [SharedK].
	five := sk.KeyModeConfig(5)
	for key, want := range map[string]string{
		"ColumnSize":               "100",
		"HitPosOffsetY":            "33",
		"ReceptorPosOffsetY":       "0", // Bar's shared default
		"FlipNoteImagesOnUpscroll": "True",
		"HitObjectFallbacks":       "1,1,2,1,1", // Arrow's
		"HoldBodyFallbacks":        "1,1,1,1,1",
		"ComboPosY":                "-40", // common to every default skin
	} {
		if got := five.Str(key, ""); got != want {
			t.Errorf("5K %s = %q, want %q", key, got, want)
		}
	}
	// 8K asks for the fallback itself, so [SharedK]'s Arrow is read first.
	if got := sk.KeyModeConfig(8).Str("ReceptorPosOffsetY", ""); got != "5" {
		t.Errorf("8K ReceptorPosOffsetY = %q, want Arrow's shared 5", got)
	}
	// "arrow" isn't a default skin name to Quaver; [SharedK]'s Arrow stands.
	if got := sk.KeyModeConfig(10).Str("HitObjectFallbacks", ""); got != "1,1,2,1,1,1,1,2,1,1" {
		t.Errorf("10K HitObjectFallbacks = %q, want Arrow's", got)
	}
	if _, ok := five.Explicit("HitPosOffsetY"); ok {
		t.Error("5K Explicit(HitPosOffsetY) should only see [5K]")
	}

	// 6K has no section of its own: all shared.
	if got := sk.KeyModeConfig(6).Int("ColumnSize", 0); got != 70 {
		t.Errorf("6K ColumnSize = %d, want 70 from [SharedK]", got)
	}
	// 4K doesn't use SharedK: Bar's own 4K defaults, not the skin's [SharedK].
	four := sk.KeyModeConfig(4)
	if four.Int("ColumnSize", 0) != 85 || four.Int("HitPosOffsetY", -1) != 0 {
		t.Errorf("4K ColumnSize/HitPosOffsetY = %d/%d, want Bar's 85/0", four.Int("ColumnSize", 0), four.Int("HitPosOffsetY", -1))
	}
	// 9K turns the fallback off, so the skin's [SharedK] doesn't apply.
	if sk.UsesSharedK(9) || sk.KeyModeConfig(9).Int("ColumnSize", 0) == 70 {
		t.Error("9K with UseFallback = False should ignore [SharedK]")
	}
	// The default tables are shared; resolving must not have changed them.
	if got := defaultKeyMode("Bar", 4).Int("ColumnSize", 0); got != 85 {
		t.Errorf("Bar 4K default ColumnSize = %d after resolving, want 85", got)
	}
}

func TestKeyModes(t *testing.T) {
	sk, err := Load(writeQS(t, map[string][]byte{
		"skin.ini":                                []byte("[7K]\nColumnSize = 60\n[3K]\nUseFallback = False\n"),
		"4k/HitObjects/note-hitobject-1.png":      []byte("x"),
		"SharedK/HitObjects/note-hitobject-1.png": []byte("x"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := sk.PresentKeyModes(); !slices.Equal(got, []int{3, 4, 7}) {
		t.Errorf("PresentKeyModes = %v, want [3 4 7]", got)
	}
	if got := sk.SharedKeyModes(); !slices.Equal(got, []int{1, 2, 5, 6, 8, 9, 10}) {
		t.Errorf("SharedKeyModes = %v, want [1 2 5 6 8 9 10]", got)
	}
	if got := sk.DefaultKeyModes(); !slices.Equal(got, []int{3, 4, 7}) {
		t.Errorf("DefaultKeyModes = %v, want the skin's own", got)
	}

	sharedOnly, err := Load(writeQS(t, map[string][]byte{
		"skin.ini":                            []byte("[General]\nName = x\n"),
		"sharedk/Receptors/receptor-up-1.png": []byte("x"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := sharedOnly.DefaultKeyModes(); !slices.Equal(got, []int{1, 2, 3, 5, 6, 8, 9, 10}) {
		t.Errorf("shared-only DefaultKeyModes = %v, want every keymode but 4K and 7K", got)
	}
}

func TestResolvedSource(t *testing.T) {
	sk, err := Load(writeQS(t, map[string][]byte{
		"skin.ini": []byte("[General]\nName = x\n"),
		"SharedK/HitObjects/note-hitobject-1.png": []byte("shared1"),
		"SharedK/HitObjects/note-hitobject-2.png": []byte("shared2"),
		"SharedK/HitObjects/note-hitobject-3.png": []byte("shared3"),
		"SharedK/Lighting/hitlighting@1x4.png":    []byte("light"),
		"SharedK/Stage/stage-left-border.png":     []byte("border"),
		"5k/HitObjects/note-hitobject-3.png":      []byte("own3"),
		"5k/Stage/stage-left-border@1x2.png":      []byte("own border sheet"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	src := sk.ResolvedSource([]int{4, 5})

	// Bar's 5K lanes borrow shared lanes 2,1,3,1,2; the skin's own lane 3 wins.
	for p, want := range map[string]string{
		"5k/HitObjects/note-hitobject-1.png": "shared2",
		"5k/HitObjects/note-hitobject-2.png": "shared1",
		"5K/hitobjects/NOTE-HITOBJECT-3.png": "own3",
		"5k/HitObjects/note-hitobject-5.png": "shared2",
		"5k/Lighting/hitlighting@1x4.png":    "light",
	} {
		data, err := src.ReadFile(p)
		if err != nil || string(data) != want {
			t.Errorf("ReadFile(%s) = %q, %v; want %q", p, data, err, want)
		}
	}
	if got := Origin(src, "5k/HitObjects/note-hitobject-1.png"); got != "SharedK/HitObjects/note-hitobject-2.png" {
		t.Errorf("Origin = %q", got)
	}
	// The skin's own border (a spritesheet) isn't shadowed by the shared one.
	if src.Exists("5k/Stage/stage-left-border.png") {
		t.Error("5K borrowed a border it already has as a spritesheet")
	}
	// 4K doesn't fall back to SharedK.
	if src.Exists("4k/HitObjects/note-hitobject-1.png") {
		t.Error("4K borrowed from sharedk/")
	}
	if !slices.Contains(src.List(), "5k/Lighting/hitlighting@1x4.png") {
		t.Error("List is missing the borrowed lighting sheet")
	}
}
