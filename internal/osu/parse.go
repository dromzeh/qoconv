package osu

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// Section is one parsed block of an osu! skin.ini with case-insensitive keys.
type Section struct {
	keys map[string]string // lowercased key -> trimmed value
}

func newParsedSection() *Section { return &Section{keys: map[string]string{}} }

// Get returns the raw value for key (case-insensitive) and whether it was set.
func (s *Section) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	v, ok := s.keys[strings.ToLower(key)]
	return v, ok
}

// Str returns the string value for key, or def if absent.
func (s *Section) Str(key, def string) string {
	if v, ok := s.Get(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// Int returns the integer value for key, or def if absent/unparseable.
// Fractional values (e.g. "402.5") truncate toward zero, matching stable.
func (s *Section) Int(key string, def int) int {
	v, ok := s.Get(key)
	if !ok {
		return def
	}
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
		return int(f)
	}
	return def
}

// Bool parses osu!'s 0/1 booleans, or def if absent/unrecognized.
func (s *Section) Bool(key string, def bool) bool {
	switch strings.TrimSpace(s.Str(key, "")) {
	case "1":
		return true
	case "0":
		return false
	}
	return def
}

// FirstCSVInt returns the first value of a comma-split list key (osu! lists
// like "ColumnWidth: 30,30,30,30"), or def if absent/unparseable.
func (s *Section) FirstCSVInt(key string, def int) int {
	v, ok := s.Get(key)
	if !ok {
		return def
	}
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
		return int(f)
	}
	return def
}

// SkinFile is a parsed osu! skin.ini: [General], [Fonts], and every [Mania]
// block keyed by its Keys value. Other sections (e.g. [Colours]) are ignored.
type SkinFile struct {
	General *Section
	Fonts   *Section
	mania   []*Section // in file order; keycount read from the "Keys" key
}

// ManiaFor returns the [Mania] block for the given keycount, or nil.
func (sf *SkinFile) ManiaFor(keys int) *Section {
	for _, m := range sf.mania {
		if m.Int("Keys", 0) == keys {
			return m
		}
	}
	return nil
}

// ManiaKeyCounts returns the keycounts that have a [Mania] block, in file order.
func (sf *SkinFile) ManiaKeyCounts() []int {
	var out []int
	for _, m := range sf.mania {
		if n := m.Int("Keys", 0); n > 0 {
			out = append(out, n)
		}
	}
	return out
}

// ParseSkinIni parses an osu! skin.ini. osu! uses `Key: Value` with `//`
// full-line comments; [Mania] may repeat, one block per keycount.
func ParseSkinIni(data []byte) *SkinFile {
	sf := &SkinFile{General: newParsedSection(), Fonts: newParsedSection()}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var cur *Section
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			switch strings.ToLower(strings.TrimSpace(line[1 : len(line)-1])) {
			case "general":
				cur = sf.General
			case "fonts":
				cur = sf.Fonts
			case "mania":
				cur = newParsedSection()
				sf.mania = append(sf.mania, cur)
			default:
				cur = nil // section with no Quaver relevance
			}
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 || cur == nil {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		if key == "" {
			continue
		}
		lk := strings.ToLower(key)
		if _, ok := cur.keys[lk]; !ok { // first value wins, like stable
			cur.keys[lk] = val
		}
	}
	return sf
}
