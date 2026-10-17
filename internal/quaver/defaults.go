package quaver

import (
	"fmt"
	"strings"
	"sync"
)

// Quaver reads every keymode value from the skin.ini first and fills the gaps
// from the built-in default skin it is based on (SkinKeys: user, default, user
// passes over Quaver.Resources/Textures/Skins/{DefaultSkin}/skin.ini). So a key
// a skin omits means the default skin's value, not zero.
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

// defaultKeyMode returns the built-in default skin's [nK] values for the
// named skin (Bar when the name is empty or unknown), layered over the values
// common to all of them.
func defaultKeyMode(skin string, keys int) *Section {
	d := defaultSkins()
	name := "Bar"
	for _, n := range []string{"Bar", "Arrow", "Circle"} {
		if strings.EqualFold(strings.TrimSpace(skin), n) {
			name = n
		}
	}
	common := d.Section("Common")
	s := d.Section(fmt.Sprintf("%s %dK", name, keys))
	if s == nil {
		return common
	}
	cp := *s
	cp.fallback = common
	return &cp
}
