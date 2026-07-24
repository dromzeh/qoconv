package quaver

import "testing"

func TestSkinIniSerializeRoundTrip(t *testing.T) {
	ini := NewSkinIni()
	gen := ini.Section("General")
	gen.Set("Name", "My Skin")
	gen.SetBool("CenterCursor", true)
	k4 := ini.Section("4K")
	k4.SetInt("ColumnSize", 141)
	k4.SetInt("ComboPosY", -40)
	k4.SetBool("ReceptorsOverHitObjects", false)

	if ini.Section("4K") != k4 {
		t.Error("Section should return the existing section on second call")
	}
	if v, ok := k4.Lookup("ComboPosY"); !ok || v != "-40" {
		t.Errorf("Lookup ComboPosY = %q, %v", v, ok)
	}

	// The output must parse back through Quaver's own skin.ini parser.
	sk := ParseString(ini.Serialize())
	if got := sk.General().Str("Name", ""); got != "My Skin" {
		t.Errorf("round-trip Name = %q", got)
	}
	if !sk.General().Bool("CenterCursor", false) {
		t.Error("round-trip CenterCursor should be true")
	}
	cfg := sk.KeyMode(4)
	if cfg == nil {
		t.Fatal("round-trip lost [4K]")
	}
	if got := cfg.Int("ComboPosY", 0); got != -40 {
		t.Errorf("round-trip ComboPosY = %d", got)
	}
	if cfg.Bool("ReceptorsOverHitObjects", true) {
		t.Error("round-trip ReceptorsOverHitObjects should be False")
	}
}
