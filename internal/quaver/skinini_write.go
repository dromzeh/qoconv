package quaver

import (
	"fmt"
	"strconv"
	"strings"
)

// SkinIni builds a Quaver skin.ini for writing (`Key = Value` syntax). It is
// the write-side counterpart of parseSkinIni: sections and keys keep their
// insertion order so the output is stable and diff-friendly.
type SkinIni struct {
	sections []*IniSection
}

// IniSection is one [Header] block being built.
type IniSection struct {
	Name string
	keys []string
	m    map[string]string
}

// NewSkinIni returns an empty skin.ini builder.
func NewSkinIni() *SkinIni { return &SkinIni{} }

// Section returns the named section, creating it on first use.
func (s *SkinIni) Section(name string) *IniSection {
	for _, sec := range s.sections {
		if strings.EqualFold(sec.Name, name) {
			return sec
		}
	}
	sec := &IniSection{Name: name, m: map[string]string{}}
	s.sections = append(s.sections, sec)
	return sec
}

// Set inserts or updates key, preserving first-insertion order.
func (s *IniSection) Set(key, val string) {
	if _, ok := s.m[key]; !ok {
		s.keys = append(s.keys, key)
	}
	s.m[key] = val
}

// SetInt is Set with an integer value.
func (s *IniSection) SetInt(key string, v int) { s.Set(key, strconv.Itoa(v)) }

// SetBool is Set with Quaver's True/False boolean spelling.
func (s *IniSection) SetBool(key string, v bool) {
	if v {
		s.Set(key, "True")
	} else {
		s.Set(key, "False")
	}
}

// Len returns the number of keys set in the section.
func (s *IniSection) Len() int { return len(s.keys) }

// Lookup returns the value set for key and whether it was set.
func (s *IniSection) Lookup(key string) (string, bool) { v, ok := s.m[key]; return v, ok }

// Serialize renders the built skin.ini in Quaver's `Key = Value` syntax.
func (s *SkinIni) Serialize() string {
	var b strings.Builder
	for i, sec := range s.sections {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "[%s]\n", sec.Name)
		for _, key := range sec.keys {
			fmt.Fprintf(&b, "%s = %s\n", key, sec.m[key])
		}
	}
	return b.String()
}
