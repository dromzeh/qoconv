//go:build production

package gui

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend
var frontend embed.FS

// Run opens the app window and blocks until it is closed.
func Run(cfg Config) error {
	if !hasDisplay() {
		return ErrUnavailable
	}
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		return err
	}
	detachConsole()

	app := NewApp(cfg)
	return wails.Run(&options.App{
		Title: "qoconv",
		// A small fixed-size tool window; the front end is laid out for it.
		Width:         640,
		Height:        512,
		DisableResize: true,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app, // preview and banner images
		},
		// Windows gets the app's own title bar (see the front end); macOS keeps
		// its traffic lights over the content; Linux keeps its native frame.
		Frameless:        runtime.GOOS == "windows",
		BackgroundColour: &options.RGBA{R: 29, G: 27, B: 36, A: 255},
		OnStartup:        func(ctx context.Context) { app.ctx = ctx },
		Bind:             []any{app},
		// Dropped files reach the front end as paths (OnFileDrop). The web view
		// must keep external drags enabled: on Windows that's how Wails sees them.
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		Windows:     &windows.Options{Theme: windows.Dark},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarHiddenInset(),
			Appearance: mac.NSAppearanceNameDarkAqua,
			About: &mac.AboutInfo{
				Title:   "qoconv " + cfg.Version,
				Message: "Quaver ⇄ osu!mania skin converter",
			},
		},
		Linux: &linux.Options{ProgramName: "qoconv"},
	})
}

// hasDisplay reports whether a window can be shown at all (not, say, over SSH).
func hasDisplay() bool {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		return true
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

var skinFilter = []wruntime.FileFilter{{DisplayName: "Skins (*.qs, *.osk, *.zip)", Pattern: "*.qs;*.osk;*.zip"}}

// PickSkin asks for a skin archive; "" if cancelled.
func (a *App) PickSkin() (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Choose a skin to convert", Filters: skinFilter})
}

// PickSkinFolder asks for an unpacked skin folder; "" if cancelled.
func (a *App) PickSkinFolder() (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Choose a skin folder to convert"})
}

// PickOutput asks where to save the converted skin; "" if cancelled.
func (a *App) PickOutput(current string) (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:                "Save the converted skin in…",
		DefaultDirectory:     existingDir(current),
		CanCreateDirectories: true,
	})
}

// existingDir walks up from dir to the nearest folder that exists, since the
// native dialogs refuse a default directory that doesn't.
func existingDir(dir string) string {
	for d := dir; d != ""; {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}
