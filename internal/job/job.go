// Package job runs a whole conversion: open the input skin, work out which way
// it converts, convert it, apply any hand-tuned skin.ini values, and write the
// skin folder and archive. The CLI, TUI and GUI front ends all go through it.
package job

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/dromzeh/qoconv/internal/convert"
	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
)

// Direction is which way a skin converts.
type Direction int

const (
	ToOsu    Direction = iota // Quaver skin in, osu!mania skin out
	ToQuaver                  // osu!mania skin in, Quaver skin out
)

// From is the input skin's game.
func (d Direction) From() string {
	if d == ToQuaver {
		return "osu!"
	}
	return "Quaver"
}

// To is the output skin's game.
func (d Direction) To() string {
	if d == ToQuaver {
		return "Quaver"
	}
	return "osu!"
}

// Ext is the output archive's extension.
func (d Direction) Ext() string {
	if d == ToQuaver {
		return ".qs"
	}
	return ".osk"
}

// Skin is an opened input skin.
type Skin struct {
	Path      string
	Direction Direction
	Name      string
	Author    string
	Modes     []int // convertible keymodes found in the skin, converted by default
	// Borrowed are keymodes a Quaver skin doesn't set up but Quaver still skins
	// from its sharedk/ folder: convertible on request, off by default.
	Borrowed []int

	src    quaver.Source
	quaver *quaver.Skin // set when Direction is ToOsu
}

// Open opens a Quaver .qs / osu! .osk archive or an unpacked skin folder and
// detects which way it converts from its skin.ini.
func Open(path string) (*Skin, error) {
	src, err := quaver.OpenSource(path)
	if err != nil {
		return nil, err
	}
	s := &Skin{Path: path, src: src}
	if convert.IsOsuSkin(src) {
		data, err := src.ReadFile("skin.ini")
		if err != nil {
			return nil, err
		}
		sf := osu.ParseSkinIni(data)
		s.Direction = ToQuaver
		s.Name = sf.General.Str("Name", "Converted Skin")
		s.Author = sf.General.Str("Author", "Unknown")
		for _, n := range quaver.SupportedKeyModes {
			if sf.ManiaFor(n) != nil {
				s.Modes = append(s.Modes, n)
			}
		}
		return s, nil
	}
	sk, err := quaver.FromSource(src)
	if err != nil {
		return nil, err
	}
	g := sk.General()
	s.Direction = ToOsu
	s.quaver = sk
	s.Name = g.Str("Name", "Converted Skin")
	s.Author = g.Str("Author", "Unknown")
	s.Modes = sk.DefaultKeyModes()
	for _, n := range sk.SharedKeyModes() {
		if !slices.Contains(s.Modes, n) {
			s.Borrowed = append(s.Borrowed, n)
		}
	}
	return s, nil
}

// bannerImages are the files most likely to show off a skin, best first.
var bannerImages = map[Direction][]string{
	ToOsu:    {"steam_workshop_preview.png", "MainMenu/menu-background.png", "Pause/pause-background.png"},
	ToQuaver: {"menu-background.jpg", "menu-background.png", "menu-background@2x.jpg", "menu-background@2x.png", "pause-overlay.png", "pause-overlay.jpg"},
}

// Banner returns the path of a background image representing the skin, or ""
// if it has none.
func (s *Skin) Banner() string {
	for _, p := range bannerImages[s.Direction] {
		if s.src.Exists(p) {
			return p
		}
	}
	return ""
}

// ReadFile reads a file of the input skin (case-insensitive path).
func (s *Skin) ReadFile(path string) ([]byte, error) { return s.src.ReadFile(path) }

// Convert converts the skin in memory.
func (s *Skin) Convert(opts convert.Options) (*Result, error) {
	if s.Direction == ToQuaver {
		res, err := convert.Reverse(s.src, opts)
		if err != nil {
			return nil, err
		}
		ini := res.Ini
		return &Result{
			Name: res.Name, Direction: s.Direction, Modes: res.Modes,
			Files: res.Output.Files, Report: res.Report,
			section: func(mode int) section {
				if !slices.Contains(res.Modes, mode) {
					return nil
				}
				return quaverSection{ini.Section(fmt.Sprintf("%dK", mode))}
			},
			general: quaverSection{ini.Section("General")},
			render:  ini.Serialize,
		}, nil
	}

	res, err := convert.Convert(s.quaver, opts)
	if err != nil {
		return nil, err
	}
	name, _ := res.Skin.General.Get("Name")
	if strings.TrimSpace(name) == "" {
		name = "Converted Skin"
	}
	sk := res.Skin
	return &Result{
		Name: name, Direction: s.Direction, Modes: res.Modes,
		Files: res.Output.Files, Report: res.Report,
		section: func(mode int) section {
			for _, m := range sk.Mania {
				if m.Keys == mode {
					return m.KV
				}
			}
			return nil
		},
		general: sk.General,
		render:  sk.Serialize,
	}, nil
}

// Result is a converted skin, ready to be tuned and written.
type Result struct {
	Name      string
	Direction Direction
	Modes     []int
	Files     map[string][]byte // output path -> contents, including skin.ini
	Report    *convert.Report

	section func(mode int) section // a keymode's skin.ini section, nil if absent
	general section                // the skin.ini [General] section
	render  func() string          // serializes the skin.ini
}

// Rename sets the skin's name and author; empty values keep the current ones.
func (r *Result) Rename(name, author string) {
	if name = strings.TrimSpace(name); name != "" {
		r.Name = name
		r.general.Set("Name", name)
	}
	if author = strings.TrimSpace(author); author != "" {
		r.general.Set("Author", author)
	}
	r.Files["skin.ini"] = []byte(r.render())
}

// section is a keymode block of the output skin.ini.
type section interface {
	Get(key string) (string, bool)
	Set(key, val string)
}

type quaverSection struct{ *quaver.IniSection }

func (q quaverSection) Get(key string) (string, bool) { return q.Lookup(key) }

// Written is where a Result was written.
type Written struct {
	Folder  string
	Archive string // empty when no archive was requested
}

// Write writes the skin folder into dir (replacing an existing one) and, when
// archive is set, an importable archive beside it.
func (r *Result) Write(dir string, archive bool) (Written, error) {
	base := Sanitize(r.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Written{}, err
	}
	w := Written{Folder: filepath.Join(dir, base)}
	if err := osu.WriteFolder(w.Folder, r.Files); err != nil {
		return Written{}, fmt.Errorf("write skin folder: %w", err)
	}
	if archive {
		w.Archive = filepath.Join(dir, base+r.Direction.Ext())
		if err := osu.WriteOSK(w.Archive, r.Files); err != nil {
			return Written{}, fmt.Errorf("write %s: %w", r.Direction.Ext(), err)
		}
	}
	return w, nil
}

// DefaultOutputDir is <home>/Documents/qoconv/output (created on write).
func DefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("qoconv", "output")
	}
	return filepath.Join(home, "Documents", "qoconv", "output")
}

// OpenPath opens path with the OS default handler: a folder in the file
// manager, or an .osk/.qs archive in the game, which imports the skin.
func OpenPath(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// Sanitize strips characters illegal in skin folder/file names.
func Sanitize(name string) string {
	name = strings.TrimSpace(name)
	name = strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "", "\"", "'", "<", "(", ">", ")", "|", "-",
	).Replace(name)
	if strings.TrimSpace(name) == "" {
		return "Converted Skin"
	}
	return name
}
