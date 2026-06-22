// Package tui implements qoconv's interactive flow using charmbracelet/huh.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// Params are the values collected interactively.
type Params struct {
	Input     string
	Output    string
	Name      string
	Author    string
	OSK       bool
	OpenInOsu bool
}

// LoadDefaults fetches the Quaver skin's Name/Author to pre-fill the form after
// the input path is known.
type LoadDefaults func(input string) (name, author string, err error)

// Gather runs the interactive flow: it asks for the input skin and output
// directory (pre-filled with defaultOutput), loads the skin to pre-fill
// name/author, then asks for those plus whether to emit a .osk and open osu!.
func Gather(defaultOutput string, load LoadDefaults) (Params, error) {
	p := Params{Output: defaultOutput, OSK: true, OpenInOsu: true}

	form1 := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Quaver skin").
			Description("Path to a .qs file or an unpacked skin folder").
			Value(&p.Input).
			Validate(required),
		huh.NewInput().
			Title("Output directory").
			Description("Where the osu! skin folder and .osk will be written (created if needed)").
			Value(&p.Output).
			Validate(required),
	))
	if err := form1.Run(); err != nil {
		return p, err
	}
	p.Input = strings.TrimSpace(p.Input)
	p.Output = strings.TrimSpace(p.Output)

	name, author, err := load(p.Input)
	if err != nil {
		return p, err
	}
	p.Name, p.Author = name, author

	form2 := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Skin name").Value(&p.Name),
		huh.NewInput().Title("Author").Value(&p.Author),
		huh.NewConfirm().
			Title("Make a .osk file?").
			Description("A single file that installs the skin in osu! when opened.\nYou'll also get the plain skin folder either way.").
			Value(&p.OSK),
	))
	if err := form2.Run(); err != nil {
		return p, err
	}

	// Only worth asking to install if there's a .osk to open.
	if p.OSK {
		installForm := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title("Install it in osu! now?").
				Description("Opens the .osk so osu! imports the skin right away.").
				Value(&p.OpenInOsu),
		))
		if err := installForm.Run(); err != nil {
			return p, err
		}
	} else {
		p.OpenInOsu = false
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
