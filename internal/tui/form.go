// Package tui implements qoconv's interactive flow using charmbracelet/huh.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// Params are the values collected interactively.
type Params struct {
	Input   string
	Output  string
	Name    string
	Author  string
	Archive bool // also write a .osk/.qs archive
	Install bool // open the archive so the target game imports it
}

// SkinInfo is what Probe learns from the input skin: its name/author for
// pre-filling the form, and the conversion direction.
type SkinInfo struct {
	Name     string
	Author   string
	ToQuaver bool // input is an osu! skin; converting to Quaver
}

// Probe loads just enough of the input skin to pre-fill the form after the
// input path is known.
type Probe func(input string) (SkinInfo, error)

// Gather runs the interactive flow: it asks for the input skin and output
// directory (pre-filled with defaultOutput), probes the skin to pre-fill
// name/author and detect the direction, then asks for those plus whether to
// emit an archive and install it.
func Gather(defaultOutput string, probe Probe) (Params, error) {
	p := Params{Output: defaultOutput, Archive: true, Install: true}

	form1 := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Skin to convert").
			Description("Path to a Quaver .qs / osu! .osk file, or an unpacked skin folder").
			Value(&p.Input).
			Validate(required),
		huh.NewInput().
			Title("Output directory").
			Description("Where the converted skin folder and archive will be written (created if needed)").
			Value(&p.Output).
			Validate(required),
	))
	if err := form1.Run(); err != nil {
		return p, err
	}
	p.Input = strings.TrimSpace(p.Input)
	p.Output = strings.TrimSpace(p.Output)

	info, err := probe(p.Input)
	if err != nil {
		return p, err
	}
	p.Name, p.Author = info.Name, info.Author

	game, ext := "osu!", ".osk"
	if info.ToQuaver {
		game, ext = "Quaver", ".qs"
	}

	form2 := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Skin name").Value(&p.Name),
		huh.NewInput().Title("Author").Value(&p.Author),
		huh.NewConfirm().
			Title(fmt.Sprintf("Make a %s file?", ext)).
			Description(fmt.Sprintf("A single file that installs the skin in %s when opened.\nYou'll also get the plain skin folder either way.", game)).
			Value(&p.Archive),
	))
	if err := form2.Run(); err != nil {
		return p, err
	}

	// Only worth asking to install if there's an archive to open.
	if p.Archive {
		installForm := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Install it in %s now?", game)).
				Description(fmt.Sprintf("Opens the %s so %s imports the skin right away.", ext, game)).
				Value(&p.Install),
		))
		if err := installForm.Run(); err != nil {
			return p, err
		}
	} else {
		p.Install = false
	}

	p.Name = strings.TrimSpace(p.Name)
	p.Author = strings.TrimSpace(p.Author)
	return p, nil
}

func required(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("required")
	}
	return nil
}
