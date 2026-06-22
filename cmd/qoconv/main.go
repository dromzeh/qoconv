// Command qoconv converts a Quaver skin (.qs or folder) into an osu!mania skin.
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
	flag.StringVar(&c.input, "input", "", "Quaver skin (.qs file or folder). Omit to use the interactive TUI.")
	flag.StringVar(&c.output, "output", defaultOutputDir(), "Output directory (parent of the generated osu! skin).")
	flag.StringVar(&c.name, "name", "", "Override skin name (default: from the Quaver skin.ini).")
	flag.StringVar(&c.author, "author", "", "Override author (default: from the Quaver skin.ini).")
	flag.StringVar(&c.keymodes, "keymodes", "", "Comma list e.g. 4k,7k (default: all detected).")
	flag.BoolVar(&c.osk, "osk", true, "Also produce an importable .osk archive.")
	flag.BoolVar(&c.grades, "grades", true, "Map letter grades to osu! ranking-* images.")
	flag.BoolVar(&c.hitsounds, "hitsounds", true, "Map Quaver SFX to osu! hitsounds.")
	flag.BoolVar(&c.quiet, "quiet", false, "Suppress the conversion report.")
	flag.BoolVar(&c.rotateCW, "health-rotate-cw", true, "Rotate health bars clockwise (try false if upside-down).")
	flag.BoolVar(&c.open, "open", false, "Install in osu! when done by opening the .osk (implies a .osk is made).")
	flag.IntVar(&c.hitPosition, "hit-position", -1, "Override osu! HitPosition 0-480 (default: auto; higher = lower on screen).")
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
		p, err := tui.Gather(c.output, func(input string) (string, string, error) {
			sk, err := quaver.Load(input)
			if err != nil {
				return "", "", err
			}
			g := sk.General()
			return g.Str("Name", "Converted Skin"), g.Str("Author", "Unknown"), nil
		})
		if err != nil {
			return err
		}
		c.input, c.output = p.Input, p.Output
		c.name, c.author, c.osk, c.open = p.Name, p.Author, p.OSK, p.OpenInOsu
	}
	return execute(c)
}

func execute(c config) error {
	sk, err := quaver.Load(c.input)
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

	res, err := convert.Convert(sk, opts)
	if err != nil {
		return err
	}

	// Installing in osu! works by opening the .osk, so make one when --open is set.
	if c.open {
		c.osk = true
	}

	folderName := sanitize(skinName(res))
	if err := os.MkdirAll(c.output, 0o755); err != nil {
		return err
	}
	folder := filepath.Join(c.output, folderName)
	if err := osu.WriteFolder(folder, res.Output.Files); err != nil {
		return fmt.Errorf("write skin folder: %w", err)
	}
	fmt.Printf("Wrote skin folder: %s\n", folder)

	oskPath := ""
	if c.osk {
		oskPath = filepath.Join(c.output, folderName+".osk")
		if err := osu.WriteOSK(oskPath, res.Output.Files); err != nil {
			return fmt.Errorf("write .osk: %w", err)
		}
		fmt.Printf("Wrote .osk file:    %s\n", oskPath)
	}
	fmt.Printf("Keymodes: %s, files: %d\n", joinModes(res.Modes), len(res.Output.Files))

	if !c.quiet {
		fmt.Println()
		fmt.Print(res.Report.String())
	}

	if c.open && oskPath != "" {
		if err := openWithOS(oskPath); err != nil {
			fmt.Fprintf(os.Stderr, "could not install in osu!: %v\n", err)
		} else {
			fmt.Println("\nOpening in osu! to install...")
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
