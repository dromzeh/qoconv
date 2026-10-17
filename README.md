<div align="center">

# qoconv

Convert your Quaver skins to osu!mania and back. Drop a skin into the app, tune the layout against a to-scale preview and install it, or do the whole thing with a one-line command. Supports 4K and 7K.

![last commit](https://shieldcn.dev/github/last-commit/dromzeh/qoconv.svg?variant=outline&size=xs)
![license](https://shieldcn.dev/github/license/dromzeh/qoconv.svg?variant=outline&size=xs)

<a href="https://github.com/dromzeh/qoconv/releases/latest"><img src="https://shieldcn.dev/badge/Download-Windows%20%C2%B7%20macOS%20%C2%B7%20Linux-blue.svg?size=lg&logo=ri:FiDownload" alt="Download for Windows, macOS and Linux"></a>

<img src="https://x.dromzeh.dev/ShareX/2026/10/qoconv_vIPtcaooEn.png" alt="qoconv converting an osu!mania skin to Quaver, with a to-scale 7K stage preview and layout sliders" width="640">

</div>

---

It works in both directions, Quaver to osu!mania and osu!mania to Quaver, and figures out which way to go from the input skin's `skin.ini`.

Every position and size is computed from the skin's own geometry (receptor sizes, `HitPosOffsetY`, column sizes and so on) using the same maths each game uses to draw it. The osu!mania side is matched against osu!lazer's `Legacy*` skin classes (osu!stable, which I use, is closed-source), and the Quaver side against Quaver's own `SkinKeys` and playfield code. If something still needs a nudge, you can tune it live in the app.

> [!NOTE]
> The two games don't draw everything the same way, so a few things can only be matched as closely as the other game allows, like where the health bar sits. The conversion report lists anything that differs.

## What it converts

- **Gameplay:** notes, long notes (head, body, tail), receptors, stage borders, column lighting, hit and hold lighting (spritesheets or single images)
- **Positioning:** column width, spacing and start, hit position, light position, combo and judgement positions, all computed from the skin
- **Sizing:** judgements, combo and score digits, health bar, hit lighting and the column light are resized to the size the other game draws them at, so a skin doesn't come out 2-5× too big
- **HUD and UI:** judgements (including animated ones), health bar, cursor, combo and score fonts, pause menu
- **Cursor:** drawn square at Quaver's size, with osu!'s spin, trail and smoke turned off, since Quaver's cursor has none of those
- Keys a Quaver skin leaves out fall back to Quaver's built-in default skin values (Bar, or whichever `DefaultSkin` names), the same as in-game
- Auto-detects keymodes (4K and 7K) and writes one `[Mania]` block per mode
- Blanks the osu! defaults Quaver skins don't have, like the judgement line, column glow, cursor trail, hit particles, combo bursts, and kiai stars, so the result matches the original

Quaver-only screens like song select, results, and the scoreboard have no osu!mania equivalent and are skipped. The conversion report prints exactly what was converted, suppressed, and skipped.

Going the other way (osu!mania to Quaver) maps the same elements back: `[Mania]` geometry becomes `[4K]`/`[7K]` values, per-column note/hold/receptor images come through (including osu!'s default `mania-*` filenames and `@2x` variants), and lighting animations get rebuilt into Quaver spritesheets, along with stage borders, judgements, health bars, fonts, grades, and hitsounds. Every layout key is written out explicitly so Quaver's own defaults don't creep in. Anything osu!-only, like standard mode or menu elements, is skipped and counted in the report.

## Download

Grab the latest build from the [releases page](https://github.com/dromzeh/qoconv/releases):

| OS             | File                              | Notes                                                                                                                                |
| -------------- | --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| Windows        | `qoconv_*_windows_amd64.exe`      | Download and run it. Uses the WebView2 runtime that ships with Windows 10/11.                                                        |
| macOS          | `qoconv_*_macos_universal.zip`    | Contains `qoconv.app` (Apple Silicon and Intel). The app isn't notarized yet, so the first time, right-click it and choose **Open**. |
| Linux          | `qoconv_*_linux_amd64.tar.gz`     | Needs GTK 3 and WebKitGTK 4.1 (most desktops have them).                                                                             |
| Linux (no GUI) | `qoconv_*_linux_amd64_cli.tar.gz` | Command line and terminal walkthrough only; runs anywhere.                                                                           |

ARM64 builds are there too for Windows and Linux.

## Using the app

1. **Pick a skin.** Drop a Quaver `.qs`, an osu! `.osk`, or a skin folder onto the window. Or click the banner (Ctrl+O) to choose from the skins already installed in osu! and Quaver, or to browse for a file or folder. qoconv works out which way to convert. You can also drop a skin onto `qoconv.exe` itself.
2. **Check the preview.** It draws the converted stage to scale the way the target game will: receptors, notes falling onto the hit line, and the judgement and combo where they'll sit.
3. **Tune the layout.** The main positions and sizes each have a slider under **Layout**, and the preview updates as you drag. Changed values light up. Click ↺ or double-click a slider to reset one, or **Reset all** to reset the whole keymode. The tabs on the right switch between 4K and 7K. When converting to Quaver, hit and light positions are shown as places on screen, so resizing columns doesn't move them.
4. **Check the output.** The **Output** tab sets the skin's name and author, where it's saved, whether to make an archive and install it, the extras to include (grades, hitsounds) and the keymodes to convert. It also lists anything that was skipped.
5. **Convert** (Ctrl+Enter). You get a ready-to-use skin folder plus an importable archive (`.osk` for osu!, `.qs` for Quaver), and it can open straight in the game. The status line links to the folder afterwards.

## Command line

Pass flags for a one-liner instead:

```
qoconv --input "MySkin.qs" --output "C:\path\to\Skins" --open
qoconv --input "MySkin.osk" --output "C:\path\to\Skins"
```

`--tui` (or running with no arguments where there's no display) gives the interactive terminal walkthrough instead of the app.

### Flags

| Flag                 | Default                   | What it does                                                                                                          |
| -------------------- | ------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `--input`            | (opens the app)           | Quaver `.qs` / osu! `.osk` file, or an unpacked skin folder. Omit to convert interactively.                           |
| `--output`           | `Documents/qoconv/output` | Where to write the skin folder and archive.                                                                           |
| `--name`             | from `skin.ini`           | Override the skin name.                                                                                               |
| `--author`           | from `skin.ini`           | Override the author.                                                                                                  |
| `--keymodes`         | all detected              | Limit output, for example `4k,7k`.                                                                                    |
| `--osk`              | `true`                    | Also write an importable archive (`.osk`, or `.qs` when converting to Quaver). Use `--osk=false` for the folder only. |
| `--open`             | `false`                   | Install the converted skin by opening the archive (makes one if needed).                                              |
| `--hit-position`     | auto                      | Override osu! HitPosition (`0`-`480`; higher sits lower on screen). Only applies when converting to osu!.             |
| `--grades`           | `true`                    | Map letter grades between `ranking-*` and `grade-small-*` images.                                                     |
| `--hitsounds`        | `true`                    | Map hitsounds between Quaver SFX and osu! `normal-hit*`.                                                              |
| `--health-rotate-cw` | `true`                    | Health-bar rotation direction. Try `--health-rotate-cw=false` if it looks upside down.                                |
| `--quiet`            | `false`                   | Hide the conversion report.                                                                                           |
| `--tui`              | `false`                   | Use the terminal walkthrough instead of the app when no `--input` is given.                                           |
| `--version`          |                           | Print the version and exit.                                                                                           |

## Building from source

```
go build -o qoconv.exe ./cmd/qoconv                     # CLI + terminal walkthrough
go build -tags production -o qoconv.exe ./cmd/qoconv    # with the app (Wails)
go test ./...
```

The app is a [Wails](https://wails.io) window, compiled in with Wails' `production` build tag. Without the tag you get the same binary minus the window (it falls back to the terminal walkthrough).

- **Windows:** no extra setup; it uses the WebView2 runtime that ships with Windows 10/11. Release builds add `-ldflags "-H windowsgui"` so double-clicking doesn't flash a console window; command-line use then attaches to the terminal it's run from, and `--tui` opens its own console window.
- **macOS:** needs CGO (Xcode command line tools): `CGO_LDFLAGS="-framework UniformTypeIdentifiers" go build -tags production ./cmd/qoconv`.
- **Linux:** needs CGO plus GTK 3 and WebKitGTK (`libgtk-3-dev libwebkit2gtk-4.1-dev` on Ubuntu 24.04): `go build -tags production,webkit2_41 ./cmd/qoconv`. Release builds also ship a `_cli` archive that runs anywhere, without WebKitGTK.

The app's front end is plain HTML/CSS/JS in `internal/gui/frontend` (no build step). The Windows icon and manifest live in `cmd/qoconv/winres`; regenerate the `.syso` files after changing them with `go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --arch amd64,arm64` from `cmd/qoconv`.

Requires a recent Go toolchain (see `go.mod`).

## Credits

Thanks to [robby250's gist](https://gist.github.com/robby250/fc1a90db6cc4ed5dd9ccb4f592d8bae7) for documenting most of the appropriate mappings.

The osu! and Quaver logos belong to their respective owners and are used only to show which way a conversion goes; the osu! logo follows [osu! brand identity guidelines](https://osu.ppy.sh/wiki/en/Brand_identity_guidelines). qoconv isn't affiliated with either game.

## License

[dromzeh/qoconv](https://github.com/dromzeh/qoconv) is licensed under the [MIT License](https://dromzeh.mit-license.org/). Authored by [@dromzeh](https://dromzeh.dev/) <[marcel@dromzeh.dev](mailto:marcel@dromzeh.dev)>
