// Package color provides ANSI 256-color terminal helpers for CLI output.
//
// v1.0 redesign: the API is built around 11-step intensity ramps per hue
// (step 0 = darkest, step 5 = canonical, step 10 = brightest) plus a
// red→yellow→green Heat ramp for severity / quality scales. Modifier
// wrappers (Bold, Reverse) compose with any color helper.
//
// All color helpers emit 256-color SGR sequences (38;5;N). Terminals that
// don't support 256-color receive uncolored output. NO_COLOR=1 and TERM=dumb
// also suppress all escapes (https://no-color.org).
package color

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ─── enable detection + SetEnabled ───────────────────────────────────────────

// enabled is true when the terminal supports color output.
var enabled = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}()

// SetEnabled is a test helper that mutates package-level color-enablement
// state. It is NOT safe for concurrent use — tests calling this must NOT
// call t.Parallel(). The returned closure restores the prior enabled value
// and is intended for deferred invocation.
func SetEnabled(b bool) func() {
	prev := enabled
	enabled = b
	return func() { enabled = prev }
}

// color256 is true when the terminal advertises 256-color support. Only used
// by ShowPalette to print a mode label; the color helpers themselves always
// emit 256-color SGR (uncolored when enabled is false).
var color256 = func() bool {
	ct := os.Getenv("COLORTERM")
	if ct == "truecolor" || ct == "24bit" {
		return true
	}
	return strings.Contains(os.Getenv("TERM"), "256color")
}()

// ─── ClearCode (ANSI strip) ──────────────────────────────────────────────────

// ansiSGR matches a CSI SGR escape sequence (ESC[ ... m). Used by ClearCode
// to strip color/formatting escapes from already-rendered strings or input
// piped from a color-emitting source.
var ansiSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// ClearCode strips ANSI/SGR escape sequences from s. Useful for stripping
// color/formatting from previously-rendered strings or from piped/file
// input before further processing.
func ClearCode(s string) string {
	return ansiSGR.ReplaceAllString(s, "")
}

// ─── core wrap ───────────────────────────────────────────────────────────────

// wrap emits "\x1b[<code>m<v>\x1b[0m" when both enabled and color256 are
// true; otherwise returns fmt.Sprint(v) unwrapped. v1.0 dropped the
// basic-ANSI fallback layer, so non-256-color terminals receive plain text
// (matching the no-TTY / NO_COLOR / TERM=dumb suppression).
func wrap(code string, v any) string {
	s := fmt.Sprint(v)
	if !enabled || !color256 {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// bgWrap is the bg-family counterpart to wrap. Like wrap it gates on enabled
// and color256, but additionally rewrites every inner "\x1b[0m" in the
// rendered input to "\x1b[0m\x1b[<code>m" so the bg color re-applies after
// every nested reset — matching the resetCodeRe trick Bold and Reverse use.
// Without this rewrite, BgGra2(Whi9("A") + Yel7("B")) would lose the gray
// background at the first inner reset; with it, the bg persists across the
// whole compound string. Fg helpers stay on wrap: their semantics are
// intentionally per-segment (inner fg overrides outer fg).
func bgWrap(code string, v any) string {
	s := fmt.Sprint(v)
	if !enabled || !color256 {
		return s
	}
	rewrapped := resetCodeRe.ReplaceAllString(s, "\x1b[0m\x1b["+code+"m")
	return "\x1b[" + code + "m" + rewrapped + "\x1b[0m"
}

// ─── modifier wrappers ───────────────────────────────────────────────────────

// resetCodeRe matches the literal "\x1b[0m" inner-reset emitted by every
// color helper. Bold/Reverse rewrite these so the modifier survives across
// nested color blocks (otherwise the inner reset would clear the modifier
// and only the first colored segment would render bold/reverse).
var resetCodeRe = regexp.MustCompile(`\x1b\[0m`)

// Bold wraps s with the ANSI bold attribute. Composable with hue / heat
// helpers: color.Bold(color.Grn5("text")) renders bold-green. Internal
// resets inside s are rewritten so the bold attribute survives across
// multiple colored segments — color.Bold(color.Grn5("a")+color.Red5("b"))
// renders both segments bold.
func Bold(s string) string {
	if !enabled || !color256 {
		return s
	}
	rewrapped := resetCodeRe.ReplaceAllString(s, "\x1b[0m\x1b[1m")
	return "\x1b[1m" + rewrapped + "\x1b[0m"
}

// Reverse wraps s with the ANSI reverse-video attribute (foreground and
// background swapped). Composable with hue / heat helpers; same internal-
// reset rewrite as Bold.
func Reverse(s string) string {
	if !enabled || !color256 {
		return s
	}
	rewrapped := resetCodeRe.ReplaceAllString(s, "\x1b[0m\x1b[7m")
	return "\x1b[7m" + rewrapped + "\x1b[0m"
}

// ─── hue ramps (10 hues × 11 steps = 110 funcs) ──────────────────────────────
//
// Step convention: 0 = darkest, 5 = canonical, 10 = brightest. Step 5 of any
// hue is the "default" shade (e.g., Grn5 is the canonical green). Walk
// toward 0 to dim, toward 10 to brighten. Indices are curated from the
// 256-color cube + grayscale ramp; ramps may share extreme indices across
// hues (e.g., Grn0 == Red0 == 16 = near-black) since the cube has limited
// pure-hue shades and the dim/bright extremes converge to gray/white.

// Gra — gray ramp.
func Gra0(v any) string  { return wrap("38;5;16", v) }
func Gra1(v any) string  { return wrap("38;5;233", v) }
func Gra2(v any) string  { return wrap("38;5;236", v) }
func Gra3(v any) string  { return wrap("38;5;239", v) }
func Gra4(v any) string  { return wrap("38;5;242", v) }
func Gra5(v any) string  { return wrap("38;5;245", v) }
func Gra6(v any) string  { return wrap("38;5;248", v) }
func Gra7(v any) string  { return wrap("38;5;251", v) }
func Gra8(v any) string  { return wrap("38;5;254", v) }
func Gra9(v any) string  { return wrap("38;5;255", v) }
func Gra10(v any) string { return wrap("38;5;231", v) }

// Red — red ramp.
func Red0(v any) string  { return wrap("38;5;16", v) }
func Red1(v any) string  { return wrap("38;5;52", v) }
func Red2(v any) string  { return wrap("38;5;88", v) }
func Red3(v any) string  { return wrap("38;5;124", v) }
func Red4(v any) string  { return wrap("38;5;160", v) }
func Red5(v any) string  { return wrap("38;5;196", v) }
func Red6(v any) string  { return wrap("38;5;203", v) }
func Red7(v any) string  { return wrap("38;5;210", v) }
func Red8(v any) string  { return wrap("38;5;217", v) }
func Red9(v any) string  { return wrap("38;5;224", v) }
func Red10(v any) string { return wrap("38;5;231", v) }

// Org — orange ramp.
func Org0(v any) string  { return wrap("38;5;16", v) }
func Org1(v any) string  { return wrap("38;5;88", v) }
func Org2(v any) string  { return wrap("38;5;130", v) }
func Org3(v any) string  { return wrap("38;5;166", v) }
func Org4(v any) string  { return wrap("38;5;172", v) }
func Org5(v any) string  { return wrap("38;5;208", v) }
func Org6(v any) string  { return wrap("38;5;214", v) }
func Org7(v any) string  { return wrap("38;5;215", v) }
func Org8(v any) string  { return wrap("38;5;222", v) }
func Org9(v any) string  { return wrap("38;5;229", v) }
func Org10(v any) string { return wrap("38;5;230", v) }

// Yel — yellow ramp.
func Yel0(v any) string  { return wrap("38;5;16", v) }
func Yel1(v any) string  { return wrap("38;5;58", v) }
func Yel2(v any) string  { return wrap("38;5;100", v) }
func Yel3(v any) string  { return wrap("38;5;142", v) }
func Yel4(v any) string  { return wrap("38;5;184", v) }
func Yel5(v any) string  { return wrap("38;5;220", v) }
func Yel6(v any) string  { return wrap("38;5;226", v) }
func Yel7(v any) string  { return wrap("38;5;227", v) }
func Yel8(v any) string  { return wrap("38;5;228", v) }
func Yel9(v any) string  { return wrap("38;5;229", v) }
func Yel10(v any) string { return wrap("38;5;230", v) }

// Grn — green ramp.
func Grn0(v any) string  { return wrap("38;5;16", v) }
func Grn1(v any) string  { return wrap("38;5;22", v) }
func Grn2(v any) string  { return wrap("38;5;28", v) }
func Grn3(v any) string  { return wrap("38;5;34", v) }
func Grn4(v any) string  { return wrap("38;5;40", v) }
func Grn5(v any) string  { return wrap("38;5;46", v) }
func Grn6(v any) string  { return wrap("38;5;83", v) }
func Grn7(v any) string  { return wrap("38;5;120", v) }
func Grn8(v any) string  { return wrap("38;5;157", v) }
func Grn9(v any) string  { return wrap("38;5;194", v) }
func Grn10(v any) string { return wrap("38;5;231", v) }

// Cya — cyan ramp.
func Cya0(v any) string  { return wrap("38;5;16", v) }
func Cya1(v any) string  { return wrap("38;5;23", v) }
func Cya2(v any) string  { return wrap("38;5;30", v) }
func Cya3(v any) string  { return wrap("38;5;37", v) }
func Cya4(v any) string  { return wrap("38;5;44", v) }
func Cya5(v any) string  { return wrap("38;5;51", v) }
func Cya6(v any) string  { return wrap("38;5;87", v) }
func Cya7(v any) string  { return wrap("38;5;123", v) }
func Cya8(v any) string  { return wrap("38;5;159", v) }
func Cya9(v any) string  { return wrap("38;5;195", v) }
func Cya10(v any) string { return wrap("38;5;231", v) }

// Blu — blue ramp.
func Blu0(v any) string  { return wrap("38;5;16", v) }
func Blu1(v any) string  { return wrap("38;5;17", v) }
func Blu2(v any) string  { return wrap("38;5;18", v) }
func Blu3(v any) string  { return wrap("38;5;19", v) }
func Blu4(v any) string  { return wrap("38;5;20", v) }
func Blu5(v any) string  { return wrap("38;5;21", v) }
func Blu6(v any) string  { return wrap("38;5;27", v) }
func Blu7(v any) string  { return wrap("38;5;33", v) }
func Blu8(v any) string  { return wrap("38;5;75", v) }
func Blu9(v any) string  { return wrap("38;5;111", v) }
func Blu10(v any) string { return wrap("38;5;195", v) }

// Pur — purple ramp.
func Pur0(v any) string  { return wrap("38;5;16", v) }
func Pur1(v any) string  { return wrap("38;5;53", v) }
func Pur2(v any) string  { return wrap("38;5;54", v) }
func Pur3(v any) string  { return wrap("38;5;55", v) }
func Pur4(v any) string  { return wrap("38;5;91", v) }
func Pur5(v any) string  { return wrap("38;5;92", v) }
func Pur6(v any) string  { return wrap("38;5;99", v) }
func Pur7(v any) string  { return wrap("38;5;105", v) }
func Pur8(v any) string  { return wrap("38;5;141", v) }
func Pur9(v any) string  { return wrap("38;5;177", v) }
func Pur10(v any) string { return wrap("38;5;219", v) }

// Mag — magenta ramp.
func Mag0(v any) string  { return wrap("38;5;16", v) }
func Mag1(v any) string  { return wrap("38;5;53", v) }
func Mag2(v any) string  { return wrap("38;5;90", v) }
func Mag3(v any) string  { return wrap("38;5;127", v) }
func Mag4(v any) string  { return wrap("38;5;164", v) }
func Mag5(v any) string  { return wrap("38;5;201", v) }
func Mag6(v any) string  { return wrap("38;5;207", v) }
func Mag7(v any) string  { return wrap("38;5;213", v) }
func Mag8(v any) string  { return wrap("38;5;219", v) }
func Mag9(v any) string  { return wrap("38;5;225", v) }
func Mag10(v any) string { return wrap("38;5;231", v) }

// Whi — white ramp.
func Whi0(v any) string  { return wrap("38;5;16", v) }
func Whi1(v any) string  { return wrap("38;5;59", v) }
func Whi2(v any) string  { return wrap("38;5;102", v) }
func Whi3(v any) string  { return wrap("38;5;145", v) }
func Whi4(v any) string  { return wrap("38;5;188", v) }
func Whi5(v any) string  { return wrap("38;5;231", v) }
func Whi6(v any) string  { return wrap("38;5;252", v) }
func Whi7(v any) string  { return wrap("38;5;253", v) }
func Whi8(v any) string  { return wrap("38;5;254", v) }
func Whi9(v any) string  { return wrap("38;5;255", v) }
func Whi10(v any) string { return wrap("38;5;15", v) }

// ─── heat ramp (red → yellow → green, 11 steps) ──────────────────────────────
//
// Heat0 = worst (bright red), Heat10 = best (bright green). Heat5 is the
// midpoint (yellow). Designed for severity / quality scales — e.g., skout's
// SHS column. Anchors traverse the cube's red→yellow edge (r=5, g=0..5)
// then the yellow→green edge (g=5, r=5..0).

func Heat0(v any) string  { return wrap("38;5;196", v) }
func Heat1(v any) string  { return wrap("38;5;202", v) }
func Heat2(v any) string  { return wrap("38;5;208", v) }
func Heat3(v any) string  { return wrap("38;5;214", v) }
func Heat4(v any) string  { return wrap("38;5;220", v) }
func Heat5(v any) string  { return wrap("38;5;226", v) }
func Heat6(v any) string  { return wrap("38;5;190", v) }
func Heat7(v any) string  { return wrap("38;5;154", v) }
func Heat8(v any) string  { return wrap("38;5;118", v) }
func Heat9(v any) string  { return wrap("38;5;82", v) }
func Heat10(v any) string { return wrap("38;5;46", v) }

// ─── ShowPalette ─────────────────────────────────────────────────────────────

// rampDef pairs a ramp's name (matching the function-name prefix) with its
// 11 ordered SGR indices (step 0 darkest, step 5 canonical, step 10 brightest).
type rampDef struct {
	name    string
	indices [11]int
}

// hueRamps drives showHueRamps. The indices here mirror the per-hue funcs
// above; keep the two in sync when iterating on the palette.
var hueRamps = []rampDef{
	{"Gra", [11]int{16, 233, 236, 239, 242, 245, 248, 251, 254, 255, 231}},
	{"Red", [11]int{16, 52, 88, 124, 160, 196, 203, 210, 217, 224, 231}},
	{"Org", [11]int{16, 88, 130, 166, 172, 208, 214, 215, 222, 229, 230}},
	{"Yel", [11]int{16, 58, 100, 142, 184, 220, 226, 227, 228, 229, 230}},
	{"Grn", [11]int{16, 22, 28, 34, 40, 46, 83, 120, 157, 194, 231}},
	{"Cya", [11]int{16, 23, 30, 37, 44, 51, 87, 123, 159, 195, 231}},
	{"Blu", [11]int{16, 17, 18, 19, 20, 21, 27, 33, 75, 111, 195}},
	{"Pur", [11]int{16, 53, 54, 55, 91, 92, 99, 105, 141, 177, 219}},
	{"Mag", [11]int{16, 53, 90, 127, 164, 201, 207, 213, 219, 225, 231}},
	{"Whi", [11]int{16, 59, 102, 145, 188, 231, 252, 253, 254, 255, 15}},
}

// heatRamp drives showHeatRamp. Mirrors the Heat0..Heat10 funcs.
var heatRamp = [11]int{196, 202, 208, 214, 220, 226, 190, 154, 118, 82, 46}

// ShowPalette prints a labeled inspection of the v1.0 palette: standard
// 16-color, 256-color cube, grayscale ramp, the 10 per-hue ramps, and the
// heat ramp. Useful for verifying terminal rendering and tuning ramp
// indices. Run from a real terminal so the SGR escapes render.
func ShowPalette() {
	mode := "uncolored (no 256-color terminal)"
	if color256 {
		mode = "256-color"
	}
	fmt.Printf("%s (%s)\n", Bold(Whi5("Color palette v1.0")), mode)
	fmt.Println()
	showAllIndices()
	showHueRamps()
	showHeatRamp()
}

func showAllIndices() {
	fmt.Println(Gra5("Standard 16-color palette (0-15):"))
	for i := range 16 {
		block := wrap(fmt.Sprintf("38;5;%d", i), "███")
		fmt.Printf("  %s %s", block, Gra5(fmt.Sprintf("%-3d", i)))
		if (i+1)%8 == 0 {
			fmt.Println()
		}
	}
	fmt.Println()
	fmt.Println(Gra5("256-color cube (16-231) — index = 16 + 36r + 6g + b:"))
	for r := range 6 {
		for g := range 6 {
			fmt.Printf("  r=%d g=%d  ", r, g)
			for b := range 6 {
				idx := 16 + 36*r + 6*g + b
				block := wrap(fmt.Sprintf("38;5;%d", idx), "███")
				fmt.Printf(" %s %s", block, Gra5(fmt.Sprintf("%-3d", idx)))
			}
			fmt.Println()
		}
	}
	fmt.Println()
	fmt.Println(Gra5("Grayscale ramp (232-255):"))
	for i := 232; i < 256; i++ {
		block := wrap(fmt.Sprintf("38;5;%d", i), "███")
		fmt.Printf("  %s %s", block, Gra5(fmt.Sprintf("%-3d", i)))
		if (i-232+1)%6 == 0 {
			fmt.Println()
		}
	}
	fmt.Println()
}

func showHueRamps() {
	fmt.Println(Gra5("Hue ramps (Hue0 dim → Hue5 canonical → Hue10 bright):"))
	for _, h := range hueRamps {
		fmt.Printf("  %-3s ", h.name)
		for step, idx := range h.indices {
			block := wrap(fmt.Sprintf("38;5;%d", idx), "███")
			fmt.Printf(" %s%s", block, Gra5(fmt.Sprintf("%2d:%-3d", step, idx)))
		}
		fmt.Println()
	}
	fmt.Println()
}

func showHeatRamp() {
	fmt.Println(Gra5("Heat ramp 0-10 (red → yellow → green):"))
	fmt.Print("  ")
	for step, idx := range heatRamp {
		block := wrap(fmt.Sprintf("38;5;%d", idx), "███")
		fmt.Printf(" %s%s", block, Gra5(fmt.Sprintf("%2d:%-3d", step, idx)))
	}
	fmt.Println()
	fmt.Println()
}

// ─── ShowGrid ────────────────────────────────────────────────────────────────

// defaultGridToken is rendered when ShowGrid is called with an empty token.
const defaultGridToken = "TOKEN"

// ShowGrid prints a bordered side-by-side grid: every ramp as a column (hue
// ramps then Heat), every step (0..10) as a row. Column headers use the
// function-name pattern with "X" as the step placeholder (e.g. RedX, BluX,
// HeatX); row labels are the X value, with row 5 suffixed "*" to mark the
// canonical step. Each cell renders token in that ramp/step combo, padded to
// fill the cell so adjacent close shades separate cleanly along the borders.
// If token is empty, "TOKEN" is used.
//
// When reverse is true, cells use background-color SGR (48;5;N) instead of
// foreground (38;5;N), turning each cell into a solid colored rectangle —
// useful for evaluating shades as label backgrounds. fgIndex then selects
// the text color rendered atop those colored cells: pass any 256-color SGR
// index (0..255), or a negative value to keep the terminal's default
// foreground. fgIndex is ignored when reverse is false.
//
// Useful for picking a label color or debugging ramp choices in any consumer
// of this package.
func ShowGrid(token string, reverse bool, fgIndex int) {
	if token == "" {
		token = defaultGridToken
	}
	ramps := append(append([]rampDef{}, hueRamps...), rampDef{"Heat", heatRamp})

	cellW := len(token)
	for _, r := range ramps {
		if l := len(r.name) + 1; l > cellW { // "<name>X"
			cellW = l
		}
	}
	const labelW = 2 // fits "5*" or "10"

	labelDash := strings.Repeat("─", labelW+2)
	cellDash := strings.Repeat("─", cellW+2)
	top := "┌" + labelDash + strings.Repeat("┬"+cellDash, len(ramps)) + "┐"
	mid := "├" + labelDash + strings.Repeat("┼"+cellDash, len(ramps)) + "┤"
	bot := "└" + labelDash + strings.Repeat("┴"+cellDash, len(ramps)) + "┘"

	// Header row: blank label cell, then "<name>X" per column.
	fmt.Println(top)
	fmt.Printf("│%s", strings.Repeat(" ", labelW+2))
	for _, r := range ramps {
		fmt.Printf("│ %-*s ", cellW, r.name+"X")
	}
	fmt.Println("│")
	fmt.Println(mid)

	// Step rows.
	for step := 0; step <= 10; step++ {
		label := fmt.Sprintf("%d", step)
		if step == 5 {
			label += "*"
		}
		fmt.Printf("│ %-*s ", labelW, label)
		for _, r := range ramps {
			padded := fmt.Sprintf(" %-*s ", cellW, token)
			var sgrCode string
			switch {
			case !reverse:
				sgrCode = fmt.Sprintf("38;5;%d", r.indices[step])
			case fgIndex < 0:
				sgrCode = fmt.Sprintf("48;5;%d", r.indices[step])
			default:
				sgrCode = fmt.Sprintf("48;5;%d;38;5;%d", r.indices[step], fgIndex)
			}
			fmt.Printf("│%s", wrap(sgrCode, padded))
		}
		fmt.Println("│")
	}
	fmt.Println(bot)
}

// ─── ShowBgRamps ─────────────────────────────────────────────────────────────

// ShowBgRamps prints one row per Bg<Hue> family (10 hues + Heat = 11 rows),
// each showing the family label followed by 11 step swatches (steps 0..10) as
// solid background-colored blocks with token rendered inside. Distinct from
// ShowGrid(token, true, fgIndex), which is a dense bordered ramp×step matrix;
// ShowBgRamps is a per-family strip optimized for picking a Bg<Hue><Step> at
// a glance, with always-readable labels.
//
// When fgIndex < 0, the text color is auto-contrasted per step from the bg
// luminance (white on dark steps, black on light steps) so the token is
// legible at every step. When fgIndex >= 0 (0..255), that fixed 256-color
// SGR index is used as the text color across all swatches. If token is empty,
// "TOKEN" is used.
func ShowBgRamps(token string, fgIndex int) {
	if token == "" {
		token = defaultGridToken
	}
	ramps := append(append([]rampDef{}, hueRamps...), rampDef{"Heat", heatRamp})

	cellW := len(token)
	nameW := 0
	for _, r := range ramps {
		if l := len("Bg") + len(r.name); l > nameW {
			nameW = l
		}
	}

	fmt.Println(Gra5("Bg<Hue><Step> swatches (steps 0..10):"))
	for _, r := range ramps {
		fmt.Printf("  %-*s ", nameW, "Bg"+r.name)
		for _, idx := range r.indices {
			fg := fgIndex
			if fg < 0 {
				fg = contrastFg(idx)
			}
			padded := fmt.Sprintf(" %-*s ", cellW, token)
			sgrCode := fmt.Sprintf("48;5;%d;38;5;%d", idx, fg)
			fmt.Print(wrap(sgrCode, padded))
		}
		fmt.Println()
	}
}

// contrastFg returns a high-contrast 256-color SGR index to use as foreground
// text on a background of the given index. 15 (bright white) for dark
// backgrounds; 16 (black) for light backgrounds. Threshold is 128 on an
// ITU-R BT.601 luminance approximation; the cube levels [0,95,135,175,215,
// 255] are the xterm-256 RGB anchors.
func contrastFg(idx int) int {
	var r, g, b int
	switch {
	case idx >= 232 && idx <= 255:
		level := 8 + (idx-232)*10
		r, g, b = level, level, level
	case idx >= 16 && idx <= 231:
		n := idx - 16
		levels := [6]int{0, 95, 135, 175, 215, 255}
		r = levels[n/36]
		g = levels[(n%36)/6]
		b = levels[n%6]
	default:
		if idx < 7 {
			return 15
		}
		return 16
	}
	lum := (299*r + 587*g + 114*b) / 1000
	if lum < 128 {
		return 15
	}
	return 16
}

// ─── FormatUsage ─────────────────────────────────────────────────────────────

// UsageLine is a single flag+description pair for FormatUsage.
type UsageLine struct {
	Flag string
	Desc string
}

func formatFlag(flag string) (string, int) {
	rawLen := len(flag)
	idx := strings.LastIndex(flag, " ")
	if idx < 0 {
		return flag, rawLen
	}
	suffix := flag[idx+1:]
	switch suffix {
	case "string", "int", "float", "bool", "duration":
		return flag[:idx+1] + Gra5(suffix), rawLen
	}
	return flag, rawLen
}

// FormatUsage builds a formatted help string with a heading, flag table, and optional footer.
func FormatUsage(heading string, lines []UsageLine, footer string) string {
	var b strings.Builder
	b.WriteString(Bold(Whi5("Usage:")))
	b.WriteString(" ")
	b.WriteString(heading)
	b.WriteString("\n")
	for _, l := range lines {
		flag, flagLen := formatFlag(l.Flag)
		col := 2 + flagLen
		b.WriteString("  ")
		b.WriteString(flag)
		if col < 38 {
			b.WriteString(strings.Repeat(" ", 38-col))
		} else {
			b.WriteString("  ")
		}
		b.WriteString(l.Desc)
		b.WriteString("\n")
	}
	if footer != "" {
		b.WriteString("\n")
		b.WriteString(footer)
		if !strings.HasSuffix(footer, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}
