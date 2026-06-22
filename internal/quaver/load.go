package quaver

import (
	"fmt"
	"path"
	"strings"
)

// SupportedKeyModes lists the keymodes qoconv recognizes for conversion.
var SupportedKeyModes = []int{4, 7}

// Load opens a Quaver skin (.qs/.osk/.zip archive or an unpacked folder) and
// parses its skin.ini. If the skin is wrapped in a sub-folder, paths are
// rebased so callers always address files relative to the skin root.
func Load(path string) (*Skin, error) {
	src, err := openSource(path)
	if err != nil {
		return nil, fmt.Errorf("open skin %q: %w", path, err)
	}
	if prefix, ok := skinIniPrefix(src); ok && prefix != "" {
		src = prefixSource{Source: src, prefix: prefix}
	}
	ini, err := src.ReadFile("skin.ini")
	if err != nil {
		return nil, fmt.Errorf("read skin.ini (is this a Quaver skin?): %w", err)
	}
	sk := parseSkinIni(ini)
	sk.Source = src
	return sk, nil
}

// PresentKeyModes returns the supported keymodes this skin actually provides,
// detected from either a [nK] config section or an "nk/" asset folder.
func (sk *Skin) PresentKeyModes() []int {
	var modes []int
	for _, n := range SupportedKeyModes {
		if sk.KeyMode(n) != nil || sk.hasModeFolder(n) {
			modes = append(modes, n)
		}
	}
	return modes
}

func (sk *Skin) hasModeFolder(n int) bool {
	prefix := strings.ToLower(fmt.Sprintf("%dk/", n))
	for _, p := range sk.Source.List() {
		if strings.HasPrefix(strings.ToLower(p), prefix) {
			return true
		}
	}
	return false
}

// skinIniPrefix finds the shallowest directory containing a skin.ini and
// returns it (without trailing slash). ok is false if none was found.
func skinIniPrefix(src Source) (string, bool) {
	best, bestDepth, found := "", 1<<30, false
	for _, p := range src.List() {
		if !strings.EqualFold(path.Base(p), "skin.ini") {
			continue
		}
		depth := strings.Count(p, "/")
		if depth < bestDepth {
			bestDepth, found = depth, true
			best = strings.TrimSuffix(strings.TrimSuffix(p, path.Base(p)), "/")
		}
	}
	return best, found
}

// prefixSource rebases a Source onto a sub-folder so callers see it as the root.
type prefixSource struct {
	Source
	prefix string
}

func (p prefixSource) join(rel string) string {
	if p.prefix == "" {
		return rel
	}
	return p.prefix + "/" + rel
}

func (p prefixSource) ReadFile(rel string) ([]byte, error) { return p.Source.ReadFile(p.join(rel)) }
func (p prefixSource) Exists(rel string) bool              { return p.Source.Exists(p.join(rel)) }

func (p prefixSource) List() []string {
	if p.prefix == "" {
		return p.Source.List()
	}
	pre := strings.ToLower(p.prefix) + "/"
	var out []string
	for _, x := range p.Source.List() {
		if strings.HasPrefix(strings.ToLower(x), pre) {
			out = append(out, x[len(pre):])
		}
	}
	return out
}
