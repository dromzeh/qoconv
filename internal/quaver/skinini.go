package quaver

import (
	"bufio"
	"bytes"
	"strings"
)

// parseSkinIni parses a Quaver skin.ini. Quaver uses `Key = Value` with `;`
// full-line comments and `[Section]` headers. Values and keys are trimmed; real
// skins contain trailing spaces (e.g. "Name = My Skin ") and invalid values
// (e.g. "Version = lastest") which we tolerate by storing verbatim.
func parseSkinIni(data []byte) *Skin {
	sk := &Skin{Sections: map[string]*Section{}}
	// Strip UTF-8 BOM if present.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var cur *Section
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			cur = sk.addSection(name)
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue // not a key=value line
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key == "" {
			continue
		}
		if cur == nil {
			cur = sk.addSection("") // keys before any header
		}
		cur.set(key, val)
	}
	return sk
}

// ParseString parses skin.ini text into a Skin with no asset Source attached.
// Useful for tests and for callers that already hold the raw config.
func ParseString(s string) *Skin { return parseSkinIni([]byte(s)) }

func (sk *Skin) addSection(name string) *Section {
	lk := strings.ToLower(name)
	if s, ok := sk.Sections[lk]; ok {
		return s // merge duplicate headers
	}
	s := newSection(name)
	sk.Sections[lk] = s
	sk.order = append(sk.order, lk)
	return s
}
