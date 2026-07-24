package convert

import (
	"fmt"
	"sort"
	"strings"
)

// Report records what the conversion did, for the user to review and hand-tune.
type Report struct {
	Converted  []string // detailed src -> dst lines (verbose; not shown by default)
	Missing    []string // expected optional elements that were absent
	Dropped    []string // top-level Quaver folders with no osu! equivalent
	Suppressed []string // osu! defaults blanked because Quaver lacks them
	Positions  []string // per-keymode computed-position summary
	Warnings   []string // calibration caveats
	ToQuaver   bool     // reverse (osu! -> Quaver) run; adjusts wording
}

func (r *Report) converted(format string, a ...any) {
	r.Converted = append(r.Converted, fmt.Sprintf(format, a...))
}
func (r *Report) missing(format string, a ...any) {
	r.Missing = append(r.Missing, fmt.Sprintf(format, a...))
}
func (r *Report) warn(format string, a ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, a...))
}
func (r *Report) position(line string) { r.Positions = append(r.Positions, line) }

// suppress records a blanked osu! default (deduped).
func (r *Report) suppress(tag string) {
	for _, t := range r.Suppressed {
		if t == tag {
			return
		}
	}
	r.Suppressed = append(r.Suppressed, tag)
}

// String renders a concise summary.
func (r *Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Converted %d elements.\n", len(r.Converted))

	for _, p := range r.Positions {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	if len(r.Positions) > 0 {
		if r.ToQuaver {
			b.WriteString("  └─ computed from the skin; fine-tune the [nK] values in skin.ini\n")
		} else {
			b.WriteString("  └─ computed from the skin; fine-tune with --hit-position\n")
		}
	}

	if len(r.Suppressed) > 0 {
		fmt.Fprintf(&b, "\nSuppressed osu! defaults Quaver lacks:\n  %s\n", strings.Join(r.Suppressed, ", "))
	}
	if len(r.Dropped) > 0 {
		d := append([]string(nil), r.Dropped...)
		sort.Strings(d)
		fmt.Fprintf(&b, "\nSkipped (no osu!mania equivalent):\n  %s\n", strings.Join(d, ", "))
	}
	if len(r.Missing) > 0 {
		game := "osu!"
		if r.ToQuaver {
			game = "Quaver"
		}
		fmt.Fprintf(&b, "\n%d optional element(s) absent — %s will use its defaults.\n", len(r.Missing), game)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "\n! %s\n", w)
	}
	return b.String()
}
