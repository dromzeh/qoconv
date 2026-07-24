// Command qoconv converts a Quaver skin (.qs or folder) into an osu!mania
// skin, or an osu!mania skin (.osk or folder) into a Quaver skin. The
// direction is auto-detected from the input's skin.ini.
//
// Non-interactive:  qoconv --input skin.qs --output ./skins
// Interactive TUI:  qoconv            (prompts for input/output/name/author)
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/dromzeh/qoconv/internal/convert"
	"github.com/dromzeh/qoconv/internal/osu"
	"github.com/dromzeh/qoconv/internal/quaver"
	"github.com/dromzeh/qoconv/internal/tui"
)

// version is set by the release build via -ldflags "-X main.version=...".
var version = "dev"

type config struct {
	input, output    string
	name, author     string
	keymodes         string
	osk, grades      bool
	hitsounds, quiet bool
	rotateCW         bool
	open             bool
	hitPosition      int
	interactive      bool
}

func main() {
	if err := run(parseFlags()); err != nil {
		fmt.Fprintf(os.Stderr, "qoconv: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	var c config
	flag.StringVar(&c.input, "input", "", "Skin to convert: Quaver .qs or osu! .osk file, or an unpacked skin folder (direction is auto-detected). Omit to use the interactive TUI.")
	flag.StringVar(&c.output, "output", defaultOutputDir(), "Output directory (parent of the generated skin).")
	flag.StringVar(&c.name, "name", "", "Override skin name (default: from the input skin.ini).")
	flag.StringVar(&c.author, "author", "", "Override author (default: from the input skin.ini).")
	flag.StringVar(&c.keymodes, "keymodes", "", "Comma list e.g. 4k,7k (default: all detected).")
	flag.BoolVar(&c.osk, "osk", true, "Also produce an importable archive (.osk, or .qs when converting to Quaver).")
	flag.BoolVar(&c.grades, "grades", true, "Map letter grades between ranking-* and grade-small-* images.")
	flag.BoolVar(&c.hitsounds, "hitsounds", true, "Map hitsounds between Quaver SFX and osu! normal-hit*.")
	flag.BoolVar(&c.quiet, "quiet", false, "Suppress the conversion report.")
	flag.BoolVar(&c.rotateCW, "health-rotate-cw", true, "Health-bar rotation direction (try false if upside-down).")
	flag.BoolVar(&c.open, "open", false, "Install the converted skin when done by opening the archive (implies one is made).")
	flag.IntVar(&c.hitPosition, "hit-position", -1, "Override osu! HitPosition 0-480 (Quaver -> osu! only; default: auto).")
	showVersion := flag.Bool("version", false, "Print version and exit.")
	flag.Parse()
	if *showVersion {
		fmt.Printf("qoconv %s\n", version)
		os.Exit(0)
	}
	c.interactive = c.input == ""
	return c
}

func run(c config) error {
	if c.interactive {
		p, err := tui.Gather(c.output, probeSkin)
		if err != nil {
			return err
		}
		c.input, c.output = p.Input, p.Output
		c.name, c.author, c.osk, c.open = p.Name, p.Author, p.Archive, p.Install
	}
	return execute(c)
}

// probeSkin opens the input just far enough to pre-fill the TUI: the skin's
// name/author and which direction the conversion will run.
func probeSkin(input string) (tui.SkinInfo, error) {
	src, err := quaver.OpenSource(input)
	if err != nil {
		return tui.SkinInfo{}, err
	}
	if convert.IsOsuSkin(src) {
		data, err := src.ReadFile("skin.ini")
		if err != nil {
			return tui.SkinInfo{}, err
		}
		g := osu.ParseSkinIni(data).General
		return tui.SkinInfo{Name: g.Str("Name", "Converted Skin"), Author: g.Str("Author", "Unknown"), ToQuaver: true}, nil
	}
	sk, err := quaver.FromSource(src)
	if err != nil {
		return tui.SkinInfo{}, err
	}
	g := sk.General()
	return tui.SkinInfo{Name: g.Str("Name", "Converted Skin"), Author: g.Str("Author", "Unknown")}, nil
}

func execute(c config) error {
	src, err := quaver.OpenSource(c.input)
	if err != nil {
		return err
	}

	opts := convert.DefaultOptions()
	opts.Name, opts.Author = c.name, c.author
	opts.Grades, opts.Hitsounds, opts.RotateHealthCW = c.grades, c.hitsounds, c.rotateCW
	opts.HitPosition = c.hitPosition
	if c.keymodes != "" {
		modes, err := parseKeymodes(c.keymodes)
		if err != nil {
			return err
		}
		opts.KeyModes = modes
	}

	if convert.IsOsuSkin(src) {
		res, err := convert.Reverse(src, opts)
		if err != nil {
			return err
		}
		return writeResult(c, res.Name, res.Modes, res.Output.Files, res.Report, ".qs", "Quaver")
	}

	sk, err := quaver.FromSource(src)
	if err != nil {
		return err
	}
	res, err := convert.Convert(sk, opts)
	if err != nil {
		return err
	}
	return writeResult(c, skinName(res), res.Modes, res.Output.Files, res.Report, ".osk", "osu!")
}

// writeResult writes the converted skin folder and (optionally) its archive,
// prints the report, and opens the archive to install it when requested.
func writeResult(c config, name string, modes []int, files map[string][]byte, rep *convert.Report, ext, game string) error {
	// Installing works by opening the archive, so make one when --open is set.
	if c.open {
		c.osk = true
	}

	folderName := sanitize(name)
	if err := os.MkdirAll(c.output, 0o755); err != nil {
		return err
	}
	folder := filepath.Join(c.output, folderName)
	if err := osu.WriteFolder(folder, files); err != nil {
		return fmt.Errorf("write skin folder: %w", err)
	}
	fmt.Printf("Wrote skin folder: %s\n", folder)

	archivePath := ""
	if c.osk {
		archivePath = filepath.Join(c.output, folderName+ext)
		if err := osu.WriteOSK(archivePath, files); err != nil {
			return fmt.Errorf("write %s: %w", ext, err)
		}
		fmt.Printf("Wrote %s file:    %s\n", ext, archivePath)
	}
	fmt.Printf("Keymodes: %s, files: %d\n", joinModes(modes), len(files))

	if !c.quiet {
		fmt.Println()
		fmt.Print(rep.String())
	}

	if c.open && archivePath != "" {
		if err := openWithOS(archivePath); err != nil {
			fmt.Fprintf(os.Stderr, "could not install in %s: %v\n", game, err)
		} else {
			fmt.Printf("\nOpening in %s to install...\n", game)
		}
	}
	return nil
}

// defaultOutputDir is <home>/Documents/qoconv/output (created on write).
func defaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("qoconv", "output")
	}
	return filepath.Join(home, "Documents", "qoconv", "output")
}

// openWithOS opens path with the OS default handler. For a .osk that is osu!,
// which imports the skin.
func openWithOS(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("cmd", "/c", "start", "", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

func skinName(res *convert.Result) string {
	if v, ok := res.Skin.General.Get("Name"); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return "Converted Skin"
}

func parseKeymodes(s string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSuffix(strings.TrimSpace(strings.ToLower(part)), "k")
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid keymode %q (use e.g. 4k,7k)", part)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no keymodes parsed from %q", s)
	}
	return out, nil
}

func joinModes(m []int) string {
	parts := make([]string, len(m))
	for i, n := range m {
		parts[i] = fmt.Sprintf("%dK", n)
	}
	return strings.Join(parts, ", ")
}

// sanitize strips characters illegal in skin folder/file names.
func sanitize(name string) string {
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
