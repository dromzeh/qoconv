package osu

import "testing"

const sampleOsuIni = `[General]
Name: Test Skin
// full-line comment
Author: Someone

[Fonts]
ScorePrefix: fonts\score

[Mania]
Keys: 4
ColumnWidth: 88,88,88,88
KeysUnderNotes: 1
NoteImage0: qo-note-1

[Mania]
Keys: 7
HitPosition: 402
`

func TestParseSkinIni(t *testing.T) {
	sf := ParseSkinIni([]byte(sampleOsuIni))

	if got := sf.General.Str("name", ""); got != "Test Skin" { // keys are case-insensitive
		t.Errorf("Name = %q, want Test Skin", got)
	}
	if got := sf.Fonts.Str("ScorePrefix", "score"); got != `fonts\score` {
		t.Errorf("ScorePrefix = %q", got)
	}

	m4 := sf.ManiaFor(4)
	if m4 == nil {
		t.Fatal("ManiaFor(4) = nil")
	}
	if got := m4.FirstCSVInt("ColumnWidth", 30); got != 88 {
		t.Errorf("ColumnWidth first = %d, want 88", got)
	}
	if !m4.Bool("KeysUnderNotes", false) {
		t.Error("KeysUnderNotes should parse as true")
	}
	if got := m4.Str("NoteImage0", ""); got != "qo-note-1" {
		t.Errorf("NoteImage0 = %q", got)
	}

	if m7 := sf.ManiaFor(7); m7 == nil || m7.Int("HitPosition", 0) != 402 {
		t.Errorf("ManiaFor(7) HitPosition wrong: %v", m7)
	}
	if sf.ManiaFor(5) != nil {
		t.Error("ManiaFor(5) should be nil")
	}
	if got := sf.ManiaKeyCounts(); len(got) != 2 || got[0] != 4 || got[1] != 7 {
		t.Errorf("ManiaKeyCounts = %v, want [4 7]", got)
	}

	// A nil section must fall back to defaults instead of panicking.
	var nilSec *Section
	if got := nilSec.Int("HitPosition", 402); got != 402 {
		t.Errorf("nil Section Int = %d, want default", got)
	}
	if got := nilSec.FirstCSVInt("ColumnWidth", 30); got != 30 {
		t.Errorf("nil Section FirstCSVInt = %d, want default", got)
	}
}
