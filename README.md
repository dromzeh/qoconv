# qoconv

Convert your Quaver skins to osu!mania — and back. Works as a guided TUI or a one-line command. Supports 4K and 7K.

![last commit](https://shieldcn.dev/github/last-commit/dromzeh/qoconv.svg?variant=outline&size=xs)
![license](https://shieldcn.dev/github/license/dromzeh/qoconv.svg?variant=outline&size=xs)

---

> [!NOTE]
> Not every mapping is 1:1 yet. This handles the core elements needed for a solid skin conversion. Both directions are supported — Quaver → osu!mania and osu!mania → Quaver — and the direction is auto-detected from the input skin's `skin.ini`.

Hit positions are computed from each skin's receptor geometry and `HitPosOffsetY`, so they adapt per skin. A value may still need a small manual tweak in the generated `skin.ini`. osu!stable (what I personally use) is closed-source, so the osu!mania positioning was reverse-engineered from osu! lazer's `Legacy*` mania classes and matched against them.

## What it converts

- **Gameplay:** notes, long notes (head, body, tail), receptors, stage borders, lighting
- **Positioning:** column width and start, hit position, combo and score positions, all computed from the skin
- **HUD and UI:** judgements, health bar (rotated to osu!'s orientation), cursor, combo and score fonts, pause menu
- Auto-detects keymodes (4K and 7K) and writes one `[Mania]` block per mode
- Blanks the osu! defaults Quaver skins don't have, like the judgement line, column glow, hit particles, combo bursts, and kiai stars, so the result matches the original

Quaver-only screens like song select, results, and the scoreboard have no osu!mania equivalent and are skipped. The conversion report prints exactly what was converted, suppressed, and skipped.

The reverse direction (osu!mania → Quaver) maps the same set of elements back: `[Mania]` geometry to `[4K]`/`[7K]` values, per-column note/hold/receptor images (including osu!'s default `mania-*` filenames and `@2x` variants), stage borders, lighting animations recomposed into Quaver spritesheets, judgements, health bars, fonts, grades, and hitsounds. osu!-only elements (standard mode, menus, and so on) are skipped and counted in the report.

## Usage

Download the latest build from the [releases page](https://github.com/dromzeh/qoconv/releases), then run `qoconv.exe`.

With no arguments it runs an interactive walkthrough (input skin, output folder, skin details). Pass flags for a one-liner instead:

```
qoconv.exe --input "MySkin.qs" --output "C:\path\to\Skins" --open
qoconv.exe --input "MySkin.osk" --output "C:\path\to\Skins"
```

Output is written as a ready-to-use skin folder plus an importable archive: `.osk` when converting to osu!, `.qs` when converting to Quaver.

### Flags

| Flag | Default | What it does |
| --- | --- | --- |
| `--input` | (prompts) | Quaver `.qs` / osu! `.osk` file, or an unpacked skin folder. Omit to use the interactive walkthrough. |
| `--output` | `Documents/qoconv/output` | Where to write the skin folder and archive. |
| `--name` | from `skin.ini` | Override the skin name. |
| `--author` | from `skin.ini` | Override the author. |
| `--keymodes` | all detected | Limit output, for example `4k,7k`. |
| `--osk` | `true` | Also write an importable archive (`.osk`, or `.qs` when converting to Quaver). Use `--osk=false` for the folder only. |
| `--open` | `false` | Install the converted skin by opening the archive (makes one if needed). |
| `--hit-position` | auto | Override osu! HitPosition (`0`-`480`; higher sits lower on screen). Quaver → osu! only. |
| `--grades` | `true` | Map letter grades between `ranking-*` and `grade-small-*` images. |
| `--hitsounds` | `true` | Map hitsounds between Quaver SFX and osu! `normal-hit*`. |
| `--health-rotate-cw` | `true` | Health-bar rotation direction. Try `--health-rotate-cw=false` if it looks upside down. |
| `--quiet` | `false` | Hide the conversion report. |
| `--version` | | Print the version and exit. |

## Building from source

```
go build -o qoconv.exe ./cmd/qoconv
go test ./...
```

Requires a recent Go toolchain (see `go.mod`).

## Credits

Thanks to [robby250's gist](https://gist.github.com/robby250/fc1a90db6cc4ed5dd9ccb4f592d8bae7) for documenting most of the appropriate mappings.

## License

[dromzeh/qoconv](https://github.com/dromzeh/qoconv) is licensed under the [MIT License](https://dromzeh.mit-license.org/). Authored by [@dromzeh](https://dromzeh.dev/) <[marcel@dromzeh.dev](mailto:marcel@dromzeh.dev)>
