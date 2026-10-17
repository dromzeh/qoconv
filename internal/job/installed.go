package job

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
)

// Installed is a convertible skin found in a game's own skins folder.
type Installed struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Game string `json:"game"` // "osu!" or "Quaver"
}

// InstalledSkins lists the 1K-10K skins in osu!stable's and Quaver's skins
// folders, osu! first, each sorted by name. Folders that can't be read are
// skipped: this is a convenience list, not an inventory.
func InstalledSkins() []Installed {
	var out []Installed
	for _, dir := range osuSkinDirs() {
		out = append(out, scanSkins(dir, "osu!", osuConvertible)...)
	}
	for _, dir := range quaverSkinDirs() {
		out = append(out, scanSkins(dir, "Quaver", quaverConvertible)...)
	}
	return out
}

func scanSkins(dir, game string, convertible func(skinDir string, ini []byte) bool) []Installed {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Installed
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if ini, ok := readSkinIni(p); ok && convertible(p, ini) {
			out = append(out, Installed{Name: e.Name(), Path: p, Game: game})
		}
	}
	slices.SortFunc(out, func(a, b Installed) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out
}

// readSkinIni reads a skin folder's skin.ini, whatever its case.
func readSkinIni(dir string) ([]byte, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), "skin.ini") {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			return data, err == nil
		}
	}
	return nil, false
}

func osuConvertible(_ string, ini []byte) bool {
	return slices.ContainsFunc(osu.ParseSkinIni(ini).ManiaKeyCounts(), func(n int) bool {
		return slices.Contains(quaver.SupportedKeyModes, n)
	})
}

func quaverConvertible(dir string, ini []byte) bool {
	entries, _ := os.ReadDir(dir)
	folders := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			folders[strings.ToLower(e.Name())] = true
		}
	}
	own, shared := quaver.ParseString(string(ini)).KeyModesWith(func(name string) bool { return folders[name] })
	return len(own)+len(shared) > 0
}

// osuSkinDirs returns osu!stable's skins folder. (osu!lazer keeps skins in its
// own database, not as folders.)
func osuSkinDirs() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	return []string{filepath.Join(os.Getenv("LOCALAPPDATA"), "osu!", "Skins")}
}

// quaverSkinDirs returns Quaver's skins folder in every Steam library.
func quaverSkinDirs() []string {
	var dirs []string
	for _, lib := range steamLibraries() {
		dirs = append(dirs, filepath.Join(lib, "steamapps", "common", "Quaver", "Skins"))
	}
	return dirs
}

var vdfPath = regexp.MustCompile(`"path"\s+"([^"]+)"`)

// steamLibraries returns each Steam install's library folders, read from its
// libraryfolders.vdf, deduplicated.
func steamLibraries() []string {
	var libs []string
	add := func(p string) {
		p = filepath.Clean(p)
		if !slices.ContainsFunc(libs, func(l string) bool { return strings.EqualFold(l, p) }) {
			libs = append(libs, p)
		}
	}
	for _, root := range steamRoots() {
		data, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
		if err != nil {
			continue
		}
		add(root)
		for _, m := range vdfPath.FindAllStringSubmatch(string(data), -1) {
			add(strings.ReplaceAll(m[1], `\\`, `\`))
		}
	}
	return libs
}

func steamRoots() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		roots := []string{filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam")}
		if p := steamPathFromRegistry(); p != "" {
			roots = append(roots, p)
		}
		return roots
	case "darwin":
		return []string{filepath.Join(home, "Library", "Application Support", "Steam")}
	default:
		return []string{
			filepath.Join(home, ".steam", "steam"),
			filepath.Join(home, ".local", "share", "Steam"),
			filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		}
	}
}
