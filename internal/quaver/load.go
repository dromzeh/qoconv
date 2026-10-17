package quaver

import (
	"fmt"
	"path"
	"strings"
)

// SupportedKeyModes lists the keymodes qoconv converts: every one Quaver
// plays (ModeHelper.MaxKeyCount is 10).
var SupportedKeyModes = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

// OpenSource opens a skin (.qs/.osk/.zip archive or an unpacked folder) for
// reading. If the skin is wrapped in a sub-folder, paths are rebased so callers
// always address files relative to the skin root. The source is game-agnostic:
// both Quaver and osu! skins ship as a folder-or-zip with a root skin.ini.
func OpenSource(path string) (Source, error) {
	src, err := openSource(path)
	if err != nil {
		return nil, fmt.Errorf("open skin %q: %w", path, err)
	}
	if prefix, ok := skinIniPrefix(src); ok && prefix != "" {
		src = prefixSource{Source: src, prefix: prefix}
	}
	return src, nil
}

// Load opens a Quaver skin (.qs/.zip archive or an unpacked folder) and parses
// its skin.ini.
func Load(path string) (*Skin, error) {
	src, err := OpenSource(path)
	if err != nil {
		return nil, err
	}
	return FromSource(src)
}

// FromSource parses the skin.ini of an already-opened Quaver skin source.
func FromSource(src Source) (*Skin, error) {
	ini, err := src.ReadFile("skin.ini")
	if err != nil {
		return nil, fmt.Errorf("read skin.ini (is this a Quaver skin?): %w", err)
	}
	sk := parseSkinIni(ini)
	sk.Source = src
	return sk, nil
}

// PresentKeyModes returns the supported keymodes this skin sets up itself,
// with a [nK] config section or an "nk/" asset folder.
func (sk *Skin) PresentKeyModes() []int {
	own, _ := sk.KeyModesWith(sk.hasFolder)
	return own
}

// SharedKeyModes returns the supported keymodes the skin doesn't set up but
// Quaver still skins from its sharedk/ folder (UsesSharedK).
func (sk *Skin) SharedKeyModes() []int {
	_, shared := sk.KeyModesWith(sk.hasFolder)
	return shared
}

// DefaultKeyModes returns the keymodes to convert when none are picked: the
// ones the skin sets up, or failing that the ones it skins from sharedk/.
func (sk *Skin) DefaultKeyModes() []int {
	own, shared := sk.KeyModesWith(sk.hasFolder)
	if len(own) > 0 {
		return own
	}
	return shared
}

// KeyModesWith is PresentKeyModes and SharedKeyModes for a skin whose files
// aren't loaded: hasFolder reports whether the skin has a top-level folder
// (asked in lower case).
func (sk *Skin) KeyModesWith(hasFolder func(name string) bool) (own, shared []int) {
	hasShared := hasFolder("sharedk")
	for _, n := range SupportedKeyModes {
		switch {
		case sk.KeyMode(n) != nil || hasFolder(fmt.Sprintf("%dk", n)):
			own = append(own, n)
		case hasShared && sk.UsesSharedK(n):
			shared = append(shared, n)
		}
	}
	return own, shared
}

func (sk *Skin) hasFolder(name string) bool {
	if sk.Source == nil {
		return false
	}
	prefix := name + "/"
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
