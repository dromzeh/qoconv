package quaver

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// A keymode that uses SharedK (Skin.UsesSharedK) takes every texture its nk/
// folder lacks from sharedk/ (SkinKeys.LoadTexture's fallback). Lane textures
// change number on the way: column c borrows the shared lane its [nK] fallback
// list names, e.g. HitObjectFallbacks = 2,1,3,1,2 gives 5K column 1 the shared
// note-hitobject-2.

// laneFallbackKeys maps a lane texture to the [nK] list choosing its lane.
var laneFallbackKeys = map[string]string{
	"note-hitobject":     "HitObjectFallbacks",
	"note-holdhitobject": "HitObjectFallbacks",
	"note-holdbody":      "HoldBodyFallbacks",
	"note-holdend":       "HoldEndFallbacks",
	"receptor-up":        "ReceptorFallbacks",
	"receptor-down":      "ReceptorFallbacks",
}

var (
	laneTexture = regexp.MustCompile(`^(.+?)-(\d+)(.*)$`) // name, lane, any suffix ("-2nd")
	sheetSuffix = regexp.MustCompile(`@\d+x\d+$`)
)

// ResolvedSource returns the skin's files with what Quaver fills in for the
// given keymodes: for each that uses SharedK, every sharedk/ texture missing
// from its nk/ folder also appears there, lane textures renumbered through the
// [nK] fallback lists. The converter can then read nk/ alone. Origin names the
// real file behind a filled-in path.
func (sk *Skin) ResolvedSource(modes []int) Source {
	type file struct{ path, rel string }
	var shared []file
	for _, p := range sk.Source.List() {
		if rel, ok := cutFold(p, "sharedk/"); ok {
			shared = append(shared, file{p, rel})
		}
	}
	if len(shared) == 0 {
		return sk.Source
	}

	o := &resolvedSource{Source: sk.Source, extra: map[string]string{}}
	for _, n := range modes {
		if !sk.UsesSharedK(n) {
			continue
		}
		cfg := sk.KeyModeConfig(n)
		dir := fmt.Sprintf("%dk/", n)
		own := map[string]bool{}
		for _, p := range sk.Source.List() {
			if rel, ok := cutFold(p, dir); ok {
				own[textureKey(rel)] = true
			}
		}
		for _, f := range shared {
			for _, rel := range laneCopies(f.rel, n, cfg) {
				p := dir + rel
				key := strings.ToLower(p)
				if own[textureKey(rel)] || o.extra[key] != "" {
					continue
				}
				o.extra[key] = f.path
				o.added = append(o.added, p)
			}
		}
	}
	return o
}

// Origin returns the skin file behind p: p itself, unless src filled it in
// from sharedk/ (ResolvedSource).
func Origin(src Source, p string) string {
	if o, ok := src.(*resolvedSource); ok {
		return o.real(p)
	}
	return p
}

type resolvedSource struct {
	Source
	extra map[string]string // lowercased filled-in path -> the sharedk/ file
	added []string          // the filled-in paths, as listed
}

func (o *resolvedSource) real(p string) string {
	if r, ok := o.extra[strings.ToLower(normPath(p))]; ok {
		return r
	}
	return p
}

func (o *resolvedSource) ReadFile(p string) ([]byte, error) { return o.Source.ReadFile(o.real(p)) }
func (o *resolvedSource) Exists(p string) bool              { return o.Source.Exists(o.real(p)) }
func (o *resolvedSource) List() []string                    { return slices.Concat(o.Source.List(), o.added) }

// laneCopies returns the nk/-relative names a sharedk/-relative file stands in
// for in a keymode of the given keys: its own name, or for a lane texture, the
// name for each column whose fallback list in cfg points at its lane.
func laneCopies(rel string, keys int, cfg *Section) []string {
	dir, file := path.Split(rel)
	ext := path.Ext(file)
	stem := strings.TrimSuffix(file, ext)
	sheet := sheetSuffix.FindString(stem)
	stem = strings.TrimSuffix(stem, sheet)

	m := laneTexture.FindStringSubmatch(stem)
	if m == nil {
		return []string{rel}
	}
	key, ok := laneFallbackKeys[strings.ToLower(m[1])]
	if !ok {
		return []string{rel}
	}
	lane, _ := strconv.Atoi(m[2])
	var out []string
	for c, l := range fallbackLanes(cfg.Str(key, ""), keys) {
		if l == lane {
			out = append(out, fmt.Sprintf("%s%s-%d%s%s%s", dir, m[1], c+1, m[3], sheet, ext))
		}
	}
	return out
}

// fallbackLanes parses an [nK] fallback list ("2,1,3,1,2") into each column's
// shared lane. Columns it doesn't cover borrow lane 1, as in Quaver.
func fallbackLanes(v string, keys int) []int {
	parts := strings.Split(v, ",")
	lanes := make([]int, keys)
	for i := range lanes {
		lanes[i] = 1
		if i < len(parts) {
			if n, err := strconv.Atoi(strings.TrimSpace(parts[i])); err == nil {
				lanes[i] = n
			}
		}
	}
	return lanes
}

// textureKey identifies a texture whatever its form: lower case, without the
// extension or a spritesheet's "@RxC".
func textureKey(rel string) string {
	stem := strings.TrimSuffix(rel, path.Ext(rel))
	return strings.ToLower(sheetSuffix.ReplaceAllString(stem, ""))
}

// cutFold is strings.CutPrefix ignoring case.
func cutFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return "", false
	}
	return s[len(prefix):], true
}
