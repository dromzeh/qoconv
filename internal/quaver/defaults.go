package quaver

import (
	"cmp"
	"fmt"
	"sync"
)

// Quaver reads every keymode value from the skin.ini first and fills the gaps
// from the built-in default skin it is based on (SkinKeys: user, default, user
// passes over Quaver.Resources/Textures/Skins/{DefaultSkin}/skin.ini). So a key
// a skin omits means the default skin's value, not zero.
//
// Only 4K and 7K have a full set of their own. Every other keymode falls back
// to the shared keymode's values (Skin.KeyModeConfig), so the default skins
// give those just their lane fallbacks (laneFallbacks).
//
// These are the values from those built-in skin.ini files for the keys the
// converter reads. ColumnColorN is deliberately absent: Quaver only reads it
// from the user's skin.ini and treats a missing one as transparent.
// HoldLightingScale is absent too: when a skin omits it Quaver uses that skin's
// own HitLightingScale.
const defaultSkinsIni = `
[Bar 4K]
ColumnSize = 85
HitPosOffsetY = 0
ReceptorPosOffsetY = 0
ReceptorsOverHitObjects = True
FlipNoteImagesOnUpscroll = True

[Bar 7K]
ColumnSize = 58
HitPosOffsetY = 0
ReceptorPosOffsetY = 0
ReceptorsOverHitObjects = True
FlipNoteImagesOnUpscroll = True

[Bar SharedK]
ColumnSize = 58
HitPosOffsetY = 0
ReceptorPosOffsetY = 0
ReceptorsOverHitObjects = True
FlipNoteImagesOnUpscroll = True

[Arrow 4K]
ColumnSize = 90
HitPosOffsetY = 90
ReceptorPosOffsetY = 5
ReceptorsOverHitObjects = False
FlipNoteImagesOnUpscroll = False

[Arrow 7K]
ColumnSize = 58
HitPosOffsetY = 58
ReceptorPosOffsetY = 5
ReceptorsOverHitObjects = False
FlipNoteImagesOnUpscroll = False

[Arrow SharedK]
ColumnSize = 58
HitPosOffsetY = 58
ReceptorPosOffsetY = 5
ReceptorsOverHitObjects = False
FlipNoteImagesOnUpscroll = False

[Circle 4K]
ColumnSize = 90
HitPosOffsetY = 90
ReceptorPosOffsetY = 5
ReceptorsOverHitObjects = False
FlipNoteImagesOnUpscroll = True

[Circle 7K]
ColumnSize = 58
HitPosOffsetY = 58
ReceptorPosOffsetY = 5
ReceptorsOverHitObjects = False
FlipNoteImagesOnUpscroll = True
HitLightingY = 28

[Circle SharedK]
ColumnSize = 58
HitPosOffsetY = 58
ReceptorPosOffsetY = 5
ReceptorsOverHitObjects = False
FlipNoteImagesOnUpscroll = True
HitLightingY = 28

; Shared by every default skin and keymode.
[Common]
ColumnAlignment = 0
NotePadding = 0
StageReceptorPadding = 0
WidthForNoteHeightScale = 0
ComboPosY = -40
JudgementBurstPosY = 108
ColumnLightingScale = 1.0
ColumnLightingOffsetY = 0
HitLightingScale = 100
HitLightingX = 0
HitLightingY = 0
JudgementHitBurstScale = 100
ComboDisplayScale = 45
ScoreDisplayScale = 18
HealthBarType = Vertical
HealthBarScale = 100
DrawLongNoteEnd = True
FlipNoteEndImagesOnUpscroll = True
`

var defaultSkins = sync.OnceValue(func() *Skin { return ParseString(defaultSkinsIni) })

// laneLists is a default skin's lane fallbacks for a keymode without textures
// of its own: the shared-keymode lane (1-based) each column borrows its notes
// and receptors from ([nK] HitObjectFallbacks, ReceptorFallbacks), and its
// long-note body and end from (HoldBodyFallbacks, HoldEndFallbacks).
type laneLists struct{ notes, holds string }

var barLanes = map[int]laneLists{
	1:  {"3", "3"},
	2:  {"1,1", "1,1"},
	3:  {"1,3,1", "1,3,1"},
	5:  {"2,1,3,1,2", "2,1,3,1,2"},
	6:  {"1,2,1,1,2,1", "1,2,1,1,2,1"},
	8:  {"2,1,2,1,1,2,1,2", "2,1,2,1,1,2,1,2"},
	9:  {"2,1,2,1,3,1,2,1,2", "2,1,2,1,3,1,2,1,2"},
	10: {"2,1,3,1,2,2,1,3,1,2", "2,1,3,1,2,2,1,3,1,2"},
}

var laneFallbacks = map[string]map[int]laneLists{
	"Bar":    barLanes,
	"Circle": barLanes, // the same as Bar's
	"Arrow": {
		1:  {"2", "1"},
		2:  {"1,1", "1,1"},
		3:  {"1,2,1", "1,1,1"},
		5:  {"1,1,2,1,1", "1,1,1,1,1"},
		6:  {"1,1,1,1,1,1", "1,1,1,1,1,1"},
		8:  {"1,1,1,1,1,1,1,1", "1,1,1,1,1,1,1,1"},
		9:  {"1,1,1,1,2,1,1,1,1", "1,1,1,1,1,1,1,1,1"},
		10: {"1,1,2,1,1,1,1,2,1,1", "1,1,1,1,1,1,1,1,1,1"},
	},
}

// defaultSkinName returns the built-in default skin s names with DefaultSkin,
// or "" when it names none. Quaver ignores anything that isn't exactly one of
// them (Enum.IsDefined is case-sensitive, so "arrow" doesn't count).
func defaultSkinName(s *Section) string {
	switch v, _ := s.Explicit("DefaultSkin"); v {
	case "Bar", "Arrow", "Circle":
		return v
	}
	return ""
}

// defaultKeyMode returns a built-in default skin's values for a keymode,
// layered over the values common to all of them; keys = 0 is the shared
// keymode. skin is a defaultSkinName, "" meaning Bar. A keymode without values
// of its own gets just its lane fallbacks, with nothing behind them: Quaver
// goes on to the shared keymode for the rest.
func defaultKeyMode(skin string, keys int) *Section {
	name := cmp.Or(skin, "Bar")
	if lists, ok := laneFallbacks[name][keys]; ok {
		s := newSection(fmt.Sprintf("%dK", keys))
		s.set("HitObjectFallbacks", lists.notes)
		s.set("ReceptorFallbacks", lists.notes)
		s.set("HoldBodyFallbacks", lists.holds)
		s.set("HoldEndFallbacks", lists.holds)
		return s
	}
	header := fmt.Sprintf("%s %dK", name, keys)
	if keys == 0 {
		header = name + " SharedK"
	}
	d := defaultSkins()
	common := d.Section("Common")
	s := d.Section(header)
	if s == nil {
		return common
	}
	cp := *s
	cp.fallback = common
	return &cp
}
