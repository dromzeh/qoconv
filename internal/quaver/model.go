// Package quaver reads Quaver skins (a .qs ZIP archive or an unpacked skin
// folder) into an in-memory model: the parsed skin.ini plus case-insensitive
// access to the asset files.
package quaver

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// Color is an RGB(a) value as used in both Quaver and osu! skin.ini files.
type Color struct {
	R, G, B, A uint8
	HasAlpha   bool
}

// ParseColor parses "r,g,b" or "r,g,b,a" (whitespace tolerant).
func ParseColor(s string) (Color, bool) {
	parts := strings.Split(s, ",")
	if len(parts) != 3 && len(parts) != 4 {
		return Color{}, false
	}
	vals := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 || n > 255 {
			return Color{}, false
		}
		vals[i] = n
	}
	c := Color{R: uint8(vals[0]), G: uint8(vals[1]), B: uint8(vals[2]), A: 255}
	if len(parts) == 4 {
		c.A = uint8(vals[3])
		c.HasAlpha = true
	}
	return c, true
}

// String renders the color back to skin.ini form, preserving the alpha channel
// only when it was originally present.
func (c Color) String() string {
	if c.HasAlpha {
		return fmt.Sprintf("%d,%d,%d,%d", c.R, c.G, c.B, c.A)
	}
	return fmt.Sprintf("%d,%d,%d", c.R, c.G, c.B)
}

// Section is one [Header] block of a skin.ini with case-insensitive keys.
type Section struct {
	Name  string            // original-case header name
	keys  map[string]string // lowercased key -> trimmed value
	order []string          // lowercased keys in insertion order

	// fallback answers lookups for keys this section doesn't set. Keymode
	// sections fall back to Quaver's built-in default skin (see KeyModeConfig).
	fallback *Section
}

func newSection(name string) *Section {
	return &Section{Name: name, keys: map[string]string{}}
}

func (s *Section) set(key, val string) {
	lk := strings.ToLower(key)
	if _, ok := s.keys[lk]; !ok {
		s.order = append(s.order, lk)
	}
	s.keys[lk] = val
}

// Get returns the raw value for key (case-insensitive) and whether it was
// set, consulting the fallback section for keys this one lacks.
func (s *Section) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	if v, ok := s.Explicit(key); ok {
		return v, true
	}
	return s.fallback.Get(key)
}

// Explicit is Get without the fallback: only what the skin.ini itself says.
func (s *Section) Explicit(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	v, ok := s.keys[strings.ToLower(key)]
	return v, ok
}

// Str returns the string value for key, or def if absent.
func (s *Section) Str(key, def string) string {
	if v, ok := s.Get(key); ok {
		return v
	}
	return def
}

// Int returns the integer value for key, or def if absent/unparseable.
// Quaver writes some ints as floats (e.g. "1.0"); those truncate toward zero.
func (s *Section) Int(key string, def int) int {
	v, ok := s.Get(key)
	if !ok {
		return def
	}
	v = strings.TrimSpace(v)
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return int(f)
	}
	return def
}

// Float returns the numeric value for key, or def if absent/unparseable.
// Most Quaver layout keys are floats (e.g. "ColumnLightingScale = 1.5").
func (s *Section) Float(key string, def float64) float64 {
	v, ok := s.Get(key)
	if !ok {
		return def
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
		return f
	}
	return def
}

// Bool parses Quaver booleans ("true"/"false", case-insensitive; also 1/0).
func (s *Section) Bool(key string, def bool) bool {
	v, ok := s.Get(key)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	return def
}

// Color parses an RGB(a) value, returning ok=false if absent or malformed.
func (s *Section) Color(key string) (Color, bool) {
	v, ok := s.Get(key)
	if !ok {
		return Color{}, false
	}
	return ParseColor(v)
}

// Keys returns the lowercased keys in insertion order.
func (s *Section) Keys() []string { return s.order }

// Skin is a parsed Quaver skin: its skin.ini sections plus an asset Source.
type Skin struct {
	Sections map[string]*Section // lowercased header -> section
	order    []string            // lowercased headers in order
	Source   Source              // case-insensitive asset access
}

// Section returns the named section (case-insensitive), or nil.
func (sk *Skin) Section(name string) *Section {
	return sk.Sections[strings.ToLower(name)]
}

// General returns the [General] section (never nil; empty if absent).
func (sk *Skin) General() *Section {
	if s := sk.Section("General"); s != nil {
		return s
	}
	return newSection("General")
}

// KeyMode returns the [nK] config section for n keys, or nil.
func (sk *Skin) KeyMode(n int) *Section {
	return sk.Section(fmt.Sprintf("%dK", n))
}

// UsesSharedK reports whether n keys fall back to the skin's shared keymode
// ([SharedK] values, sharedk/ textures) for whatever [nK] and nk/ leave out.
// Quaver only has default textures for 4K and 7K, so every other keymode does
// unless the skin turns it off (UseFallback = False).
func (sk *Skin) UsesSharedK(n int) bool {
	return sk.KeyMode(n).Bool("UseFallback", n != 4 && n != 7)
}

// KeyModeConfig returns the [nK] section as Quaver resolves it: the skin's own
// values, then the built-in default skin's (DefaultSkin = Bar/Arrow/Circle,
// Bar when unset), then for a keymode that uses SharedK the skin's [SharedK]
// values, and last the default skin's [SharedK]. Never nil, even when the skin
// has mode assets but no [nK] block. Explicit still answers from [nK] alone.
func (sk *Skin) KeyModeConfig(n int) *Section {
	own := sk.KeyMode(n)
	if own == nil {
		own = newSection(fmt.Sprintf("%dK", n))
	}
	var shared *Section // [SharedK], when n falls back to it
	if sk.UsesSharedK(n) {
		shared = sk.Section("SharedK")
	}
	// Quaver settles the default skin as it reads (SkinKeys.ReadConfig). Its
	// shared values load from the one named before it gets to the skin's
	// [SharedK]: [nK]'s, or [SharedK]'s if [nK] asks for the fallback itself.
	// The keymode's own defaults load from the last one named.
	base := defaultSkinName(own)
	if own.Bool("UseFallback", false) {
		base = defaultSkinName(shared)
	}
	def := cmp.Or(defaultSkinName(own), defaultSkinName(shared), base)
	// The shared defaults come last either way, so a keymode that turns
	// UseFallback off still has defaults behind its own values.
	return chain(own, defaultKeyMode(def, n), shared, defaultKeyMode(base, 0))
}

// chain links copies of secs, and of the fallbacks each already has, so each
// answers what the ones before it don't. nil entries are skipped and the
// inputs are left as they were.
func chain(secs ...*Section) *Section {
	var head *Section
	next := &head
	for _, s := range secs {
		for ; s != nil; s = s.fallback {
			cp := *s
			cp.fallback = nil
			*next = &cp
			next = &cp.fallback
		}
	}
	return head
}
