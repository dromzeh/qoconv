// Package gui is qoconv's desktop app: a Wails window whose web front end
// (frontend/) drives the conversion through the methods of App.
//
// The window needs Wails' "production" build tag (and CGO + WebKitGTK on
// Linux); without it Run reports ErrUnavailable and the caller falls back to
// the terminal UI. Everything else here builds and tests without Wails.
package gui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/dromzeh/qoconv/internal/convert"
	"github.com/dromzeh/qoconv/internal/job"
)

// ErrUnavailable means this build or machine can't show the GUI.
var ErrUnavailable = errors.New("gui unavailable")

// Config configures the app.
type Config struct {
	Version   string
	OutputDir string // pre-filled output directory
	Open      string // skin to load on start, e.g. one dropped on the executable
}

// App is bound to the front end: each exported method is callable from
// JavaScript as window.go.gui.App.<Method>, returning a promise.
type App struct {
	cfg Config
	ctx context.Context // the Wails context, set on startup

	mu   sync.Mutex
	skin *job.Skin
	res  *job.Result
	gen  int // bumped per conversion, so preview image URLs never go stale
	// req numbers Convert calls. Conversions run unlocked and may finish out
	// of order; only the latest one installs its result.
	req int
}

// NewApp returns an App with nothing loaded.
func NewApp(cfg Config) *App { return &App{cfg: cfg} }

// StartupInfo is what the front end needs before anything is loaded.
type StartupInfo struct {
	Version string `json:"version"`
	Open    string `json:"open"` // skin to load right away, if any
}

// Startup returns the version and any skin to open on launch.
func (a *App) Startup() StartupInfo { return StartupInfo{Version: a.cfg.Version, Open: a.cfg.Open} }

// InstalledSkins lists the convertible skins in osu!'s and Quaver's own
// skins folders, for the skin switcher.
func (a *App) InstalledSkins() []job.Installed {
	if s := job.InstalledSkins(); s != nil {
		return s
	}
	return []job.Installed{}
}

// SkinInfo describes a loaded input skin.
type SkinInfo struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Author string `json:"author"`
	From   string `json:"from"` // "Quaver" / "osu!"
	To     string `json:"to"`
	Ext    string `json:"ext"`    // output archive extension
	Modes  []int  `json:"modes"`  // keymodes converted unless deselected
	Extra  []int  `json:"extra"`  // keymodes offered but off until selected (job.Skin.Borrowed)
	Output string `json:"output"` // suggested output directory
	Banner string `json:"banner"` // URL of a background image from the skin, or ""
}

// Load opens the skin at path (an archive or folder) as the one to convert.
func (a *App) Load(skinPath string) (SkinInfo, error) {
	s, err := job.Open(strings.TrimSpace(skinPath))
	if err != nil {
		return SkinInfo{}, friendly(err)
	}
	if len(s.Modes) == 0 {
		return SkinInfo{}, fmt.Errorf("this %s skin has no 1K-10K setup to convert", s.Direction.From())
	}
	a.mu.Lock()
	a.skin, a.res = s, nil
	a.mu.Unlock()
	return SkinInfo{
		Path: s.Path, Name: s.Name, Author: s.Author,
		From: s.Direction.From(), To: s.Direction.To(), Ext: s.Direction.Ext(),
		Modes: s.Modes, Extra: s.Borrowed, Output: a.cfg.OutputDir, Banner: bannerURL(s),
	}, nil
}

// Options are the conversion choices made in the front end.
type Options struct {
	Modes     []int      `json:"modes"`
	Grades    bool       `json:"grades"`
	Hitsounds bool       `json:"hitsounds"`
	HealthCW  bool       `json:"healthCW"`
	Overrides []Override `json:"overrides"` // hand-tuned values, re-applied after converting
}

// Override is one hand-tuned skin.ini value.
type Override struct {
	Mode  int    `json:"mode"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Preview is a converted (not yet written) skin.
type Preview struct {
	Modes  []ModeView `json:"modes"`
	Report ReportView `json:"report"`
	Files  int        `json:"files"`
	Gen    int        `json:"gen"`
}

// ModeView is one keymode's tunable values and playfield sketch.
type ModeView struct {
	Mode      int           `json:"mode"`
	Fields    []job.Field   `json:"fields"`
	Playfield job.Playfield `json:"playfield"`
}

// ReportView summarizes the conversion report.
type ReportView struct {
	Converted  int      `json:"converted"`
	Missing    []string `json:"missing"`
	Dropped    []string `json:"dropped"`
	Suppressed []string `json:"suppressed"`
	Warnings   []string `json:"warnings"`
}

// Convert converts the loaded skin in memory with o, ready to tune and save.
// The conversion itself runs without the lock, so preview images and Tune
// stay responsive meanwhile.
func (a *App) Convert(o Options) (Preview, error) {
	skin, req, err := a.beginConvert()
	if err != nil {
		return Preview{}, err
	}
	res, err := convertSkin(skin, o)
	if err != nil {
		return Preview{}, err
	}
	return a.install(skin, req, res)
}

// beginConvert snapshots the loaded skin and numbers the conversion.
func (a *App) beginConvert() (*job.Skin, int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.skin == nil {
		return nil, 0, errors.New("no skin loaded")
	}
	a.req++
	return a.skin, a.req, nil
}

// convertSkin converts skin with o and re-applies its overrides. It touches
// no App state, so it needs no lock.
func convertSkin(skin *job.Skin, o Options) (*job.Result, error) {
	opts := convert.DefaultOptions()
	opts.Grades, opts.Hitsounds, opts.RotateHealthCW = o.Grades, o.Hitsounds, o.HealthCW
	if len(o.Modes) > 0 {
		opts.KeyModes = o.Modes
	}
	res, err := skin.Convert(opts)
	if err != nil {
		return nil, err
	}
	for _, ov := range o.Overrides {
		if slices.Contains(res.Modes, ov.Mode) {
			if err := res.Set(ov.Mode, ov.Key, ov.Value); err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

// install makes res, converted from skin as conversion req, the current
// result. It refuses a stale one: another skin loaded since, or a newer
// conversion started, would otherwise be overwritten by an outdated result.
func (a *App) install(skin *job.Skin, req int, res *job.Result) (Preview, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.skin != skin:
		return Preview{}, errors.New("the skin changed while converting")
	case a.req != req:
		return Preview{}, errors.New("a newer conversion replaced this one")
	}
	a.res = res
	a.gen++
	return a.preview(), nil
}

// Tune sets one value on the converted skin and returns the keymode's
// refreshed view (no re-conversion; it only touches skin.ini).
func (a *App) Tune(mode int, key, value string) (ModeView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.res == nil {
		return ModeView{}, errors.New("nothing converted yet")
	}
	if err := a.res.Set(mode, key, value); err != nil {
		return ModeView{}, err
	}
	return a.modeView(mode), nil
}

// SaveRequest says where and how to write the converted skin.
type SaveRequest struct {
	Name    string `json:"name"`
	Author  string `json:"author"`
	Output  string `json:"output"`
	Archive bool   `json:"archive"`
	Install bool   `json:"install"`
}

// Saved reports a written skin.
type Saved struct {
	Folder    string `json:"folder"`
	Archive   string `json:"archive"`
	Installed bool   `json:"installed"`
	Game      string `json:"game"`
}

// Save writes the converted skin, and opens the archive to install it when
// asked (which needs one, so Install implies Archive).
func (a *App) Save(r SaveRequest) (Saved, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.res == nil {
		return Saved{}, errors.New("nothing converted yet")
	}
	out := strings.TrimSpace(r.Output)
	if out == "" {
		return Saved{}, errors.New("choose where to save the skin")
	}
	a.res.Rename(r.Name, r.Author)
	w, err := a.res.Write(out, r.Archive || r.Install)
	if err != nil {
		return Saved{}, friendly(err)
	}
	s := Saved{Folder: w.Folder, Archive: w.Archive, Game: a.res.Direction.To()}
	if r.Install {
		if err := job.OpenPath(w.Archive); err != nil {
			return s, fmt.Errorf("saved, but couldn't open it in %s: %w", s.Game, err)
		}
		s.Installed = true
	}
	return s, nil
}

// Reveal opens a written folder (or an archive's folder) in the file manager.
func (a *App) Reveal(path string) error { return job.OpenPath(path) }

func (a *App) preview() Preview {
	r := a.res.Report
	p := Preview{
		Files: len(a.res.Files),
		Gen:   a.gen,
		Report: ReportView{
			Converted:  len(r.Converted),
			Missing:    nonNil(r.Missing),
			Dropped:    nonNil(r.Dropped),
			Suppressed: nonNil(r.Suppressed),
			Warnings:   nonNil(r.Warnings),
		},
	}
	for _, m := range a.res.Modes {
		p.Modes = append(p.Modes, a.modeView(m))
	}
	return p
}

func (a *App) modeView(mode int) ModeView {
	pf, _ := a.res.Playfield(mode)
	return ModeView{Mode: mode, Fields: a.res.Fields(mode), Playfield: pf}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ServeHTTP serves images to the front end, for routes its embedded assets
// don't cover (Wails sends those here):
//
//	/out/{gen}/{path}  a file of the converted skin (gen = conversion number)
//	/skin/{path}       a file of the loaded input skin (its banner)
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, name := a.lookup(r.URL.Path)
	ctype := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg"}[strings.ToLower(path.Ext(name))]
	if data == nil || ctype == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}

func (a *App) lookup(urlPath string) (data []byte, name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if rest, ok := strings.CutPrefix(urlPath, "/out/"); ok {
		gen, name, _ := strings.Cut(rest, "/")
		if a.res != nil && gen == strconv.Itoa(a.gen) {
			return a.res.Files[name], name
		}
		return nil, ""
	}
	if name, ok := strings.CutPrefix(urlPath, "/skin/"); ok && a.skin != nil {
		data, _ := a.skin.ReadFile(name)
		return data, name
	}
	return nil, ""
}

func bannerURL(s *job.Skin) string {
	b := s.Banner()
	if b == "" {
		return ""
	}
	parts := strings.Split(b, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "/skin/" + strings.Join(parts, "/")
}

// friendly rewords the errors a player is most likely to hit.
func friendly(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "read skin.ini"):
		return errors.New("that doesn't look like a skin: there's no skin.ini in it")
	case strings.Contains(msg, "read zip"):
		return errors.New("that file isn't a skin archive (.qs, .osk or .zip)")
	}
	return err
}
