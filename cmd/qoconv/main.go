// Command qoconv converts a Quaver skin (.qs or folder) into an osu!mania
// skin, or an osu!mania skin (.osk or folder) into a Quaver skin. The
// direction is auto-detected from the input's skin.ini.
//
// Non-interactive:  qoconv --input skin.qs --output ./skins
// Interactive:      qoconv            (opens the GUI, or the TUI where there is none)
// Open a skin:      qoconv skin.qs    (GUI with the skin loaded; e.g. a file dropped on the exe)
// Terminal UI:      qoconv --tui
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/dromzeh/qoconv/internal/convert"
	"github.com/dromzeh/qoconv/internal/gui"
	"github.com/dromzeh/qoconv/internal/job"
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
	tui              bool
	skinArg          string // positional skin path: load it in the GUI
}

func main() {
	// Before flag parsing, so usage and flag errors reach the terminal too.
	args := os.Args[1:]
	done := setupConsole(slices.ContainsFunc(args, isFlag), slices.ContainsFunc(args, isTUIFlag))

	err := run(parseFlags())
	if err != nil {
		fmt.Fprintf(os.Stderr, "qoconv: %v\n", err)
	}
	done()
	if err != nil {
		os.Exit(1)
	}
}

func isFlag(arg string) bool { return strings.HasPrefix(arg, "-") }

func isTUIFlag(arg string) bool {
	switch strings.TrimLeft(arg, "-") {
	case "tui", "tui=true", "tui=1":
		return isFlag(arg)
	}
	return false
}

func parseFlags() config {
	var c config
	flag.StringVar(&c.input, "input", "", "Skin to convert: Quaver .qs or osu! .osk file, or an unpacked skin folder (direction is auto-detected). Omit to convert interactively.")
	flag.StringVar(&c.output, "output", job.DefaultOutputDir(), "Output directory (parent of the generated skin).")
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
	flag.BoolVar(&c.tui, "tui", false, "Use the terminal UI instead of the GUI when no --input is given.")
	showVersion := flag.Bool("version", false, "Print version and exit.")
	flag.Parse()
	c.skinArg = flag.Arg(0)
	if *showVersion {
		fmt.Printf("qoconv %s\n", version)
		os.Exit(0)
	}
	return c
}

func run(c config) error {
	if c.input != "" {
		return execute(c)
	}
	if !c.tui {
		err := gui.Run(gui.Config{Version: version, OutputDir: c.output, Open: c.skinArg})
		if !errors.Is(err, gui.ErrUnavailable) {
			return err
		}
	}
	if c.skinArg != "" { // no GUI: convert the given skin straight away
		c.input = c.skinArg
		return execute(c)
	}
	p, err := tui.Gather(c.output, probeSkin)
	if err != nil {
		return err
	}
	c.input, c.output = p.Input, p.Output
	c.name, c.author, c.osk, c.open = p.Name, p.Author, p.Archive, p.Install
	return execute(c)
}

// probeSkin opens the input just far enough to pre-fill the TUI: the skin's
// name/author and which direction the conversion will run.
func probeSkin(input string) (tui.SkinInfo, error) {
	s, err := job.Open(input)
	if err != nil {
		return tui.SkinInfo{}, err
	}
	return tui.SkinInfo{Name: s.Name, Author: s.Author, ToQuaver: s.Direction == job.ToQuaver}, nil
}

func execute(c config) error {
	s, err := job.Open(c.input)
	if err != nil {
		return err
	}

	opts := convert.DefaultOptions()
	opts.Name, opts.Author = c.name, c.author
	opts.Grades, opts.Hitsounds, opts.RotateHealthCW = c.grades, c.hitsounds, c.rotateCW
	opts.HitPosition = c.hitPosition
	if c.keymodes != "" {
		if opts.KeyModes, err = parseKeymodes(c.keymodes); err != nil {
			return err
		}
	}

	res, err := s.Convert(opts)
	if err != nil {
		return err
	}
	// Installing works by opening the archive, so make one when --open is set.
	w, err := res.Write(c.output, c.osk || c.open)
	if err != nil {
		return err
	}

	ext := res.Direction.Ext()
	fmt.Printf("Wrote skin folder: %s\n", w.Folder)
	if w.Archive != "" {
		fmt.Printf("Wrote %s file:    %s\n", ext, w.Archive)
	}
	fmt.Printf("Keymodes: %s, files: %d\n", joinModes(res.Modes), len(res.Files))

	if !c.quiet {
		fmt.Println()
		fmt.Print(res.Report.String())
	}

	if c.open {
		game := res.Direction.To()
		if err := job.OpenPath(w.Archive); err != nil {
			fmt.Fprintf(os.Stderr, "could not install in %s: %v\n", game, err)
		} else {
			fmt.Printf("\nOpening in %s to install...\n", game)
		}
	}
	return nil
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
