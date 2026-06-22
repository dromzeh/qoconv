// Package quaver reads Quaver skins (a .qs ZIP archive or an unpacked skin
// folder) into an in-memory model: the parsed skin.ini plus case-insensitive
// access to the asset files.
package quaver

import (
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

// Get returns the raw value for key (case-insensitive) and whether it was set.
func (s *Section) Get(key string) (string, bool) {
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

// KeyMode returns the [4K]/[7K]-style config section for n keys, or nil.
func (sk *Skin) KeyMode(n int) *Section {
	return sk.Section(fmt.Sprintf("%dK", n))
}

// KeyModeOrEmpty is like KeyMode but returns an empty section (never nil) when
// the skin provides mode assets but no [nK] config block.
func (sk *Skin) KeyModeOrEmpty(n int) *Section {
	if s := sk.KeyMode(n); s != nil {
		return s
	}
	return newSection(fmt.Sprintf("%dK", n))
}
