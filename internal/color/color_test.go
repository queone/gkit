package color

import (
	"io"
	"os"
	"strings"
	"testing"
)

// In test environments stdout is not a TTY, so enabled == false and all
// color functions return the input string unchanged. The tests below verify
// both the no-color path (input preserved) and the wrap helper directly.
//
// No test in this file uses t.Parallel() because several tests mutate the
// package-level enabled and color256 variables. Running any test concurrently
// with those mutations would be a data race.

// allHelpers returns a labeled list of every named color helper in v1.0.
// Used by tests to drive uniform property checks.
func allHelpers() []struct {
	name string
	fn   func(any) string
} {
	return []struct {
		name string
		fn   func(any) string
	}{
		{"Gra0", Gra0}, {"Gra1", Gra1}, {"Gra2", Gra2}, {"Gra3", Gra3}, {"Gra4", Gra4},
		{"Gra5", Gra5}, {"Gra6", Gra6}, {"Gra7", Gra7}, {"Gra8", Gra8}, {"Gra9", Gra9}, {"Gra10", Gra10},
		{"Red0", Red0}, {"Red1", Red1}, {"Red2", Red2}, {"Red3", Red3}, {"Red4", Red4},
		{"Red5", Red5}, {"Red6", Red6}, {"Red7", Red7}, {"Red8", Red8}, {"Red9", Red9}, {"Red10", Red10},
		{"Org0", Org0}, {"Org1", Org1}, {"Org2", Org2}, {"Org3", Org3}, {"Org4", Org4},
		{"Org5", Org5}, {"Org6", Org6}, {"Org7", Org7}, {"Org8", Org8}, {"Org9", Org9}, {"Org10", Org10},
		{"Yel0", Yel0}, {"Yel1", Yel1}, {"Yel2", Yel2}, {"Yel3", Yel3}, {"Yel4", Yel4},
		{"Yel5", Yel5}, {"Yel6", Yel6}, {"Yel7", Yel7}, {"Yel8", Yel8}, {"Yel9", Yel9}, {"Yel10", Yel10},
		{"Grn0", Grn0}, {"Grn1", Grn1}, {"Grn2", Grn2}, {"Grn3", Grn3}, {"Grn4", Grn4},
		{"Grn5", Grn5}, {"Grn6", Grn6}, {"Grn7", Grn7}, {"Grn8", Grn8}, {"Grn9", Grn9}, {"Grn10", Grn10},
		{"Cya0", Cya0}, {"Cya1", Cya1}, {"Cya2", Cya2}, {"Cya3", Cya3}, {"Cya4", Cya4},
		{"Cya5", Cya5}, {"Cya6", Cya6}, {"Cya7", Cya7}, {"Cya8", Cya8}, {"Cya9", Cya9}, {"Cya10", Cya10},
		{"Blu0", Blu0}, {"Blu1", Blu1}, {"Blu2", Blu2}, {"Blu3", Blu3}, {"Blu4", Blu4},
		{"Blu5", Blu5}, {"Blu6", Blu6}, {"Blu7", Blu7}, {"Blu8", Blu8}, {"Blu9", Blu9}, {"Blu10", Blu10},
		{"Pur0", Pur0}, {"Pur1", Pur1}, {"Pur2", Pur2}, {"Pur3", Pur3}, {"Pur4", Pur4},
		{"Pur5", Pur5}, {"Pur6", Pur6}, {"Pur7", Pur7}, {"Pur8", Pur8}, {"Pur9", Pur9}, {"Pur10", Pur10},
		{"Mag0", Mag0}, {"Mag1", Mag1}, {"Mag2", Mag2}, {"Mag3", Mag3}, {"Mag4", Mag4},
		{"Mag5", Mag5}, {"Mag6", Mag6}, {"Mag7", Mag7}, {"Mag8", Mag8}, {"Mag9", Mag9}, {"Mag10", Mag10},
		{"Whi0", Whi0}, {"Whi1", Whi1}, {"Whi2", Whi2}, {"Whi3", Whi3}, {"Whi4", Whi4},
		{"Whi5", Whi5}, {"Whi6", Whi6}, {"Whi7", Whi7}, {"Whi8", Whi8}, {"Whi9", Whi9}, {"Whi10", Whi10},
		{"Heat0", Heat0}, {"Heat1", Heat1}, {"Heat2", Heat2}, {"Heat3", Heat3}, {"Heat4", Heat4},
		{"Heat5", Heat5}, {"Heat6", Heat6}, {"Heat7", Heat7}, {"Heat8", Heat8}, {"Heat9", Heat9}, {"Heat10", Heat10},
		{"BgGra0", BgGra0}, {"BgGra1", BgGra1}, {"BgGra2", BgGra2}, {"BgGra3", BgGra3}, {"BgGra4", BgGra4},
		{"BgGra5", BgGra5}, {"BgGra6", BgGra6}, {"BgGra7", BgGra7}, {"BgGra8", BgGra8}, {"BgGra9", BgGra9}, {"BgGra10", BgGra10},
		{"BgRed0", BgRed0}, {"BgRed1", BgRed1}, {"BgRed2", BgRed2}, {"BgRed3", BgRed3}, {"BgRed4", BgRed4},
		{"BgRed5", BgRed5}, {"BgRed6", BgRed6}, {"BgRed7", BgRed7}, {"BgRed8", BgRed8}, {"BgRed9", BgRed9}, {"BgRed10", BgRed10},
		{"BgOrg0", BgOrg0}, {"BgOrg1", BgOrg1}, {"BgOrg2", BgOrg2}, {"BgOrg3", BgOrg3}, {"BgOrg4", BgOrg4},
		{"BgOrg5", BgOrg5}, {"BgOrg6", BgOrg6}, {"BgOrg7", BgOrg7}, {"BgOrg8", BgOrg8}, {"BgOrg9", BgOrg9}, {"BgOrg10", BgOrg10},
		{"BgYel0", BgYel0}, {"BgYel1", BgYel1}, {"BgYel2", BgYel2}, {"BgYel3", BgYel3}, {"BgYel4", BgYel4},
		{"BgYel5", BgYel5}, {"BgYel6", BgYel6}, {"BgYel7", BgYel7}, {"BgYel8", BgYel8}, {"BgYel9", BgYel9}, {"BgYel10", BgYel10},
		{"BgGrn0", BgGrn0}, {"BgGrn1", BgGrn1}, {"BgGrn2", BgGrn2}, {"BgGrn3", BgGrn3}, {"BgGrn4", BgGrn4},
		{"BgGrn5", BgGrn5}, {"BgGrn6", BgGrn6}, {"BgGrn7", BgGrn7}, {"BgGrn8", BgGrn8}, {"BgGrn9", BgGrn9}, {"BgGrn10", BgGrn10},
		{"BgCya0", BgCya0}, {"BgCya1", BgCya1}, {"BgCya2", BgCya2}, {"BgCya3", BgCya3}, {"BgCya4", BgCya4},
		{"BgCya5", BgCya5}, {"BgCya6", BgCya6}, {"BgCya7", BgCya7}, {"BgCya8", BgCya8}, {"BgCya9", BgCya9}, {"BgCya10", BgCya10},
		{"BgBlu0", BgBlu0}, {"BgBlu1", BgBlu1}, {"BgBlu2", BgBlu2}, {"BgBlu3", BgBlu3}, {"BgBlu4", BgBlu4},
		{"BgBlu5", BgBlu5}, {"BgBlu6", BgBlu6}, {"BgBlu7", BgBlu7}, {"BgBlu8", BgBlu8}, {"BgBlu9", BgBlu9}, {"BgBlu10", BgBlu10},
		{"BgPur0", BgPur0}, {"BgPur1", BgPur1}, {"BgPur2", BgPur2}, {"BgPur3", BgPur3}, {"BgPur4", BgPur4},
		{"BgPur5", BgPur5}, {"BgPur6", BgPur6}, {"BgPur7", BgPur7}, {"BgPur8", BgPur8}, {"BgPur9", BgPur9}, {"BgPur10", BgPur10},
		{"BgMag0", BgMag0}, {"BgMag1", BgMag1}, {"BgMag2", BgMag2}, {"BgMag3", BgMag3}, {"BgMag4", BgMag4},
		{"BgMag5", BgMag5}, {"BgMag6", BgMag6}, {"BgMag7", BgMag7}, {"BgMag8", BgMag8}, {"BgMag9", BgMag9}, {"BgMag10", BgMag10},
		{"BgWhi0", BgWhi0}, {"BgWhi1", BgWhi1}, {"BgWhi2", BgWhi2}, {"BgWhi3", BgWhi3}, {"BgWhi4", BgWhi4},
		{"BgWhi5", BgWhi5}, {"BgWhi6", BgWhi6}, {"BgWhi7", BgWhi7}, {"BgWhi8", BgWhi8}, {"BgWhi9", BgWhi9}, {"BgWhi10", BgWhi10},
		{"BgHeat0", BgHeat0}, {"BgHeat1", BgHeat1}, {"BgHeat2", BgHeat2}, {"BgHeat3", BgHeat3}, {"BgHeat4", BgHeat4},
		{"BgHeat5", BgHeat5}, {"BgHeat6", BgHeat6}, {"BgHeat7", BgHeat7}, {"BgHeat8", BgHeat8}, {"BgHeat9", BgHeat9}, {"BgHeat10", BgHeat10},
	}
}

// TestColorFunctionsContainInput verifies every named helper preserves the
// input string. With color enabled, the input is wrapped in escape codes;
// with color disabled, the input is returned bare. Either way, "hello" must
// appear somewhere in the output.
func TestColorFunctionsContainInput(t *testing.T) {
	for _, tc := range allHelpers() {
		got := tc.fn("hello")
		if !strings.Contains(got, "hello") {
			t.Errorf("%s(%q) = %q, does not contain input", tc.name, "hello", got)
		}
		if got == "" {
			t.Errorf("%s(%q) returned empty string", tc.name, "hello")
		}
	}
}

// TestColorFunctionsNoTTY confirms the no-color path returns bare input.
func TestColorFunctionsNoTTY(t *testing.T) {
	if enabled {
		t.Skip("TTY detected — skipping no-color path test")
	}
	for _, tc := range allHelpers() {
		got := tc.fn("test")
		if got != "test" {
			t.Errorf("%s(%q) = %q, want %q (no-TTY path)", tc.name, "test", got, "test")
		}
	}
}

func TestWrapEmptyString(t *testing.T) {
	_ = wrap("32", "")
}

// TestWrapProducesEscapes verifies wrap() emits the documented ANSI shape.
func TestWrapProducesEscapes(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	got := wrap("38;5;46", "ok")
	want := "\x1b[38;5;46mok\x1b[0m"
	if got != want {
		t.Fatalf("wrap(\"38;5;46\", \"ok\") = %q, want %q", got, want)
	}
}

// TestWrapPlainTextOnNon256TTY verifies v1.0.1's fix: when enabled=true but
// color256=false (a 16-color TTY), wrap returns plain text instead of
// emitting 256-color SGR escapes the terminal can't render properly.
func TestWrapPlainTextOnNon256TTY(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = false
	defer func() { color256 = prev }()

	if got := wrap("38;5;46", "ok"); got != "ok" {
		t.Errorf("wrap on non-256 TTY: got %q, want %q", got, "ok")
	}
	if got := Bold("ok"); got != "ok" {
		t.Errorf("Bold on non-256 TTY: got %q, want %q", got, "ok")
	}
	if got := Reverse("ok"); got != "ok" {
		t.Errorf("Reverse on non-256 TTY: got %q, want %q", got, "ok")
	}
}

// TestColorFunctions256Codes spot-checks the documented 256-color escape
// for canonical (step 5) hues and a few endpoints.
func TestColorFunctions256Codes(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	cases := []struct {
		name string
		fn   func(any) string
		code string
	}{
		{"Gra5", Gra5, "38;5;245"},
		{"Red5", Red5, "38;5;196"},
		{"Org5", Org5, "38;5;208"},
		{"Yel5", Yel5, "38;5;220"},
		{"Grn5", Grn5, "38;5;46"},
		{"Cya5", Cya5, "38;5;51"},
		{"Blu5", Blu5, "38;5;21"},
		{"Pur5", Pur5, "38;5;92"},
		{"Mag5", Mag5, "38;5;201"},
		{"Whi5", Whi5, "38;5;231"},
		{"Heat0", Heat0, "38;5;196"},
		{"Heat5", Heat5, "38;5;226"},
		{"Heat10", Heat10, "38;5;46"},
		{"Grn0", Grn0, "38;5;16"},
		{"Grn10", Grn10, "38;5;231"},
		{"BgGra5", BgGra5, "48;5;245"},
		{"BgRed5", BgRed5, "48;5;196"},
		{"BgGrn5", BgGrn5, "48;5;46"},
		{"BgBlu5", BgBlu5, "48;5;21"},
		{"BgHeat0", BgHeat0, "48;5;196"},
		{"BgHeat10", BgHeat10, "48;5;46"},
		{"BgGrn0", BgGrn0, "48;5;16"},
		{"BgGrn10", BgGrn10, "48;5;231"},
	}
	for _, tc := range cases {
		got := tc.fn("x")
		wantPrefix := "\x1b[" + tc.code + "m"
		if !strings.HasPrefix(got, wantPrefix) {
			t.Errorf("%s: got %q, want prefix %q", tc.name, got, wantPrefix)
		}
		if !strings.HasSuffix(got, "\x1b[0m") {
			t.Errorf("%s: got %q, want suffix %q", tc.name, got, "\x1b[0m")
		}
	}
}

// TestBoldComposesWithSingleColor verifies Bold(Grn5("x")) renders bold
// green: bold attribute prefix, color escape, content, single-reset suffix.
func TestBoldComposesWithSingleColor(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	got := Bold(Grn5("x"))
	want := "\x1b[1m\x1b[38;5;46mx\x1b[0m\x1b[1m\x1b[0m"
	if got != want {
		t.Errorf("Bold(Grn5(\"x\")) = %q, want %q", got, want)
	}
}

// TestBoldComposesWithMultipleColors verifies Bold's reset-rewrite makes the
// modifier survive across nested color blocks.
func TestBoldComposesWithMultipleColors(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	got := Bold(Grn5("a") + Red5("b"))
	// The inner reset between segments is rewritten so bold re-applies for "b".
	if !strings.Contains(got, "\x1b[1m\x1b[38;5;46ma\x1b[0m\x1b[1m\x1b[38;5;196mb\x1b[0m") {
		t.Errorf("Bold composition didn't rewrap inner resets: got %q", got)
	}
}

// TestReverseComposesWithColor — Reverse mirrors Bold's behavior.
func TestReverseComposesWithColor(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	got := Reverse(Grn5("x"))
	want := "\x1b[7m\x1b[38;5;46mx\x1b[0m\x1b[7m\x1b[0m"
	if got != want {
		t.Errorf("Reverse(Grn5(\"x\")) = %q, want %q", got, want)
	}
}

// TestBgComposesWithForegroundHelper verifies a Bg<Hue> wrapping a fg helper
// emits the documented two-prefix shape (48;5;BG then 38;5;FG, both inner
// then outer reset), round-trips clean through ClearCode, returns bare
// input on the no-TTY path, and that Bold(Bg(Fg(...))) keeps the bold
// attribute alive across the nested resets (same resetCodeRe trick as the
// fg-only compose tests).
func TestBgComposesWithForegroundHelper(t *testing.T) {
	t.Run("two_prefix_shape", func(t *testing.T) {
		defer SetEnabled(true)()
		prev := color256
		color256 = true
		defer func() { color256 = prev }()

		// bgWrap rewrites the single inner reset to "\x1b[0m\x1b[48;5;236m",
		// so the bg re-applies before the outer reset clears it.
		got := BgGra2(Whi9("x"))
		want := "\x1b[48;5;236m\x1b[38;5;255mx\x1b[0m\x1b[48;5;236m\x1b[0m"
		if got != want {
			t.Errorf("BgGra2(Whi9(\"x\")) = %q, want %q", got, want)
		}
		if stripped := ClearCode(got); stripped != "x" {
			t.Errorf("ClearCode(BgGra2(Whi9(\"x\"))) = %q, want %q", stripped, "x")
		}
	})

	t.Run("no_tty", func(t *testing.T) {
		if enabled {
			t.Skip("TTY detected — skipping no-color path test")
		}
		if got := BgGra2(Whi9("x")); got != "x" {
			t.Errorf("no-TTY path: BgGra2(Whi9(\"x\")) = %q, want %q", got, "x")
		}
	})

	t.Run("bold_survives_bg_fg_nest", func(t *testing.T) {
		defer SetEnabled(true)()
		prev := color256
		color256 = true
		defer func() { color256 = prev }()

		got := Bold(BgGra2(Whi9("x")))
		// bgWrap inserts an inner bg re-apply; Bold's outer rewrite then sees
		// two inner resets and re-applies bold after each, keeping bold alive
		// across the full nest.
		want := "\x1b[1m\x1b[48;5;236m\x1b[38;5;255mx\x1b[0m\x1b[1m\x1b[48;5;236m\x1b[0m\x1b[1m\x1b[0m"
		if got != want {
			t.Errorf("Bold(BgGra2(Whi9(\"x\"))) = %q, want %q", got, want)
		}
	})
}

// TestBgSurvivesAcrossInnerResets verifies the bgWrap rewrite makes a Bg
// helper preserve its background across an arbitrarily long compound input
// that itself contains multiple inner fg helpers (each ending in \x1b[0m).
// Before the bgWrap fix, only the first segment got the bg; everything after
// the first inner reset rendered against the terminal's default background.
func TestBgSurvivesAcrossInnerResets(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	got := BgGra2(Whi9("A") + " " + Yel7("B") + " " + Whi9("C"))

	// 1 leading bg-set + 3 re-applies after each inner reset = 4 occurrences.
	if c := strings.Count(got, "\x1b[48;5;236m"); c != 4 {
		t.Errorf("BgGra2 across 3 inner resets: got %d bg-set occurrences, want 4\noutput: %q", c, got)
	}
	if stripped := ClearCode(got); stripped != "A B C" {
		t.Errorf("ClearCode round-trip: got %q, want %q", stripped, "A B C")
	}
}

// TestFgCompositionUnchanged confirms Part C's bgWrap fix did not accidentally
// change foreground compose behavior. Fg helpers route through plain wrap
// (no resetCodeRe rewrite), so an outer fg helper does NOT re-apply after
// inner resets — inner fg helpers override the outer per segment, which is
// the documented v1.0+ semantics. Test: Red5(Whi9("A") + Yel7("B")) must
// contain "\x1b[38;5;196m" exactly once (the outer wrap only).
func TestFgCompositionUnchanged(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	got := Red5(Whi9("A") + Yel7("B"))
	if c := strings.Count(got, "\x1b[38;5;196m"); c != 1 {
		t.Errorf("fg compose changed: Red5 outer wrap appears %d times, want 1\noutput: %q", c, got)
	}
}

// TestBoldNoTTY: when color is disabled, Bold returns input unchanged.
func TestBoldNoTTY(t *testing.T) {
	if enabled {
		t.Skip("TTY detected — skipping no-color path test")
	}
	if got := Bold("hello"); got != "hello" {
		t.Errorf("Bold(%q) = %q, want %q", "hello", got, "hello")
	}
	if got := Reverse("hello"); got != "hello" {
		t.Errorf("Reverse(%q) = %q, want %q", "hello", got, "hello")
	}
}

// TestShowPaletteCoversAllSections captures ShowPalette output and checks
// every documented section header is present.
func TestShowPaletteCoversAllSections(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	ShowPalette()
	w.Close()
	os.Stdout = oldStdout

	buf, _ := io.ReadAll(r)
	output := string(buf)

	for _, header := range []string{
		"Color palette v1.0",
		"Standard 16-color palette",
		"256-color cube",
		"Grayscale ramp",
		"Hue ramps",
		"Heat ramp",
	} {
		if !strings.Contains(output, header) {
			t.Errorf("ShowPalette() output missing section %q", header)
		}
	}

	// Spot-check hue labels and heat-ramp indices appear.
	for _, label := range []string{"Gra ", "Red ", "Org ", "Yel ", "Grn ", "Cya ", "Blu ", "Pur ", "Mag ", "Whi "} {
		if !strings.Contains(output, label) {
			t.Errorf("ShowPalette() output missing hue label %q", label)
		}
	}
	for _, idx := range []string{"196", "226", "46"} {
		if !strings.Contains(output, idx) {
			t.Errorf("ShowPalette() output missing heat-ramp index %q", idx)
		}
	}
}

// captureShowGrid runs ShowGrid with the given args and returns its stdout.
func captureShowGrid(t *testing.T, token string, reverse bool, fgIndex int) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	ShowGrid(token, reverse, fgIndex)
	w.Close()
	os.Stdout = oldStdout
	buf, _ := io.ReadAll(r)
	return string(buf)
}

// TestShowGrid_layout captures ShowGrid output and verifies the bordered
// grid: top / header / divider / 11 step rows / bottom; every ramp's
// "<name>X" column heading; default token "TOKEN" when empty; row 5 flagged
// with "*"; and one token instance per (ramp, step) cell.
func TestShowGrid_layout(t *testing.T) {
	output := captureShowGrid(t, "", false, -1) // empty → defaults to "TOKEN"
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// top + header + mid + 11 step rows + bottom = 15 lines.
	if len(lines) != 15 {
		t.Fatalf("line count: got %d, want 15\noutput:\n%s", len(lines), output)
	}
	if !strings.HasPrefix(lines[0], "┌") || !strings.HasSuffix(lines[0], "┐") {
		t.Errorf("top border malformed: %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "├") || !strings.HasSuffix(lines[2], "┤") {
		t.Errorf("mid divider malformed: %q", lines[2])
	}
	if !strings.HasPrefix(lines[14], "└") || !strings.HasSuffix(lines[14], "┘") {
		t.Errorf("bottom border malformed: %q", lines[14])
	}

	// Every hue plus Heat appears as "<name>X" in the header line.
	wantHeaders := []string{"GraX", "RedX", "OrgX", "YelX", "GrnX", "CyaX", "BluX", "PurX", "MagX", "WhiX", "HeatX"}
	for _, h := range wantHeaders {
		if !strings.Contains(lines[1], h) {
			t.Errorf("header missing column %q\nheader: %q", h, lines[1])
		}
	}

	// Default token rendered once per (ramp, step) cell = 11 ramps × 11 steps.
	if got, want := strings.Count(output, "TOKEN"), 11*11; got != want {
		t.Errorf("token count: got %d, want %d", got, want)
	}

	// Row 5's first cell carries the canonical "*"; other rows do not.
	for step := 0; step <= 10; step++ {
		row := lines[3+step]
		first, _, _ := strings.Cut(strings.TrimPrefix(row, "│ "), " ")
		hasStar := strings.HasSuffix(first, "*")
		if step == 5 && !hasStar {
			t.Errorf("step 5 label missing canonical *: %q", first)
		}
		if step != 5 && hasStar {
			t.Errorf("step %d label unexpectedly has *: %q", step, first)
		}
	}
}

// TestShowGrid_customToken verifies a non-empty token is rendered verbatim
// and the default does not leak in.
func TestShowGrid_customToken(t *testing.T) {
	output := captureShowGrid(t, "HEADER", false, -1)
	if got, want := strings.Count(output, "HEADER"), 11*11; got != want {
		t.Errorf("custom-token count: got %d, want %d", got, want)
	}
	if strings.Contains(output, "TOKEN") {
		t.Errorf("default token leaked into custom-token output")
	}
}

// TestShowGrid_reverseEmitsBackgroundSGR forces 256-color mode so wrap emits
// real SGR escapes, then verifies reverse=true with default fg uses only the
// background-color prefix (48;5;), reverse=true with a fgIndex emits both
// (48;5;BG;38;5;FG), and reverse=false uses only the foreground prefix
// (38;5;).
func TestShowGrid_reverseEmitsBackgroundSGR(t *testing.T) {
	prevEnabled := SetEnabled(true)
	defer prevEnabled()
	prev256 := color256
	color256 = true
	defer func() { color256 = prev256 }()

	fg := captureShowGrid(t, "", false, -1)
	if !strings.Contains(fg, "\x1b[38;5;") {
		t.Errorf("forward mode missing foreground SGR (38;5;...)\noutput:\n%q", fg)
	}
	if strings.Contains(fg, "\x1b[48;5;") {
		t.Errorf("forward mode unexpectedly emitted background SGR (48;5;...)")
	}

	rev := captureShowGrid(t, "", true, -1)
	if !strings.Contains(rev, "\x1b[48;5;") {
		t.Errorf("reverse mode missing background SGR (48;5;...)\noutput:\n%q", rev)
	}
	if strings.Contains(rev, "\x1b[38;5;") {
		t.Errorf("reverse mode (default fg) unexpectedly emitted foreground SGR (38;5;...)")
	}

	revFg := captureShowGrid(t, "", true, 15)
	if !strings.Contains(revFg, "\x1b[48;5;") {
		t.Errorf("reverse+fg missing background SGR (48;5;...)\noutput:\n%q", revFg)
	}
	if !strings.Contains(revFg, ";38;5;15") {
		t.Errorf("reverse+fg missing combined fg SGR (;38;5;15)\noutput:\n%q", revFg)
	}
}

func TestFormatUsage(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		got := FormatUsage("prog [flags]", []UsageLine{
			{"-v", "verbose output"},
			{"-o string", "output file"},
		}, "")
		if !strings.HasPrefix(got, "Usage: prog [flags]\n") {
			t.Errorf("heading mismatch: %q", got)
		}
		if !strings.Contains(got, "-v") || !strings.Contains(got, "verbose output") {
			t.Errorf("missing flag line: %q", got)
		}
		if !strings.Contains(got, "-o string") || !strings.Contains(got, "output file") {
			t.Errorf("missing type-suffix flag line: %q", got)
		}
		if strings.HasSuffix(got, "\n\n") {
			t.Errorf("unexpected trailing blank line with empty footer: %q", got)
		}
	})

	t.Run("footer_no_newline", func(t *testing.T) {
		got := FormatUsage("prog", nil, "See docs.")
		if !strings.Contains(got, "\nSee docs.\n") {
			t.Errorf("footer missing or not newline-terminated: %q", got)
		}
	})

	t.Run("footer_with_newline", func(t *testing.T) {
		got := FormatUsage("prog", nil, "See docs.\n")
		if strings.HasSuffix(got, "docs.\n\n") {
			t.Errorf("footer double-newlined: %q", got)
		}
	})

	t.Run("long_flag", func(t *testing.T) {
		got := FormatUsage("prog", []UsageLine{
			{"--very-long-flag-name-that-exceeds-column string", "desc"},
		}, "")
		if !strings.Contains(got, "string  desc") {
			t.Errorf("long flag alignment wrong: %q", got)
		}
	})
}

func TestClearCodeStripsAnsiSGR(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"\x1b[32mhello\x1b[0m", "hello"},
		{"plain text", "plain text"},
		{"\x1b[38;5;46mok\x1b[0m end", "ok end"},
		{"", ""},
		{"\x1b[1;38;5;15mbold\x1b[0m and \x1b[31mred\x1b[0m", "bold and red"},
	}
	for _, tc := range cases {
		if got := ClearCode(tc.in); got != tc.want {
			t.Errorf("ClearCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestClearCodeRoundTripWithColorHelpers renders a sample through each
// canonical (step 5) hue helper plus Bold/Reverse, then verifies ClearCode
// strips back to the original.
func TestClearCodeRoundTripWithColorHelpers(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	sample := "the quick brown fox"
	helpers := []struct {
		name string
		fn   func(any) string
	}{
		{"Gra5", Gra5}, {"Red5", Red5}, {"Org5", Org5}, {"Yel5", Yel5},
		{"Grn5", Grn5}, {"Cya5", Cya5}, {"Blu5", Blu5}, {"Pur5", Pur5},
		{"Mag5", Mag5}, {"Whi5", Whi5}, {"Heat5", Heat5},
	}
	for _, h := range helpers {
		rendered := h.fn(sample)
		if got := ClearCode(rendered); got != sample {
			t.Errorf("ClearCode(%s(%q)) = %q, want %q", h.name, sample, got, sample)
		}
	}

	// Composed Bold + color also strips clean.
	rendered := Bold(Grn5(sample))
	if got := ClearCode(rendered); got != sample {
		t.Errorf("ClearCode(Bold(Grn5(%q))) = %q, want %q", sample, got, sample)
	}
}

// TestColorSetEnabledTogglesAndRestores: SetEnabled flips the package-level
// enabled var and the returned closure restores it.
func TestColorSetEnabledTogglesAndRestores(t *testing.T) {
	before := enabled

	restore := SetEnabled(true)
	if !enabled {
		t.Errorf("after SetEnabled(true): enabled = false, want true")
	}
	restore()
	if enabled != before {
		t.Errorf("after restore: enabled = %v, want %v", enabled, before)
	}

	restore = SetEnabled(false)
	if enabled {
		t.Errorf("after SetEnabled(false): enabled = true, want false")
	}
	restore()
	if enabled != before {
		t.Errorf("after second restore: enabled = %v, want %v", enabled, before)
	}
}

// captureShowBgRamps runs ShowBgRamps with the given args and returns stdout.
func captureShowBgRamps(t *testing.T, token string, fgIndex int) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	ShowBgRamps(token, fgIndex)
	w.Close()
	os.Stdout = oldStdout
	buf, _ := io.ReadAll(r)
	return string(buf)
}

// TestShowBgRampsCoversAllBgFamilies verifies every Bg family label appears in
// the output, the default token "TOKEN" renders once per (family, step) when
// the token argument is empty, and a custom token does not leak the default.
func TestShowBgRampsCoversAllBgFamilies(t *testing.T) {
	output := captureShowBgRamps(t, "", -1)

	wantFamilies := []string{"BgGra", "BgRed", "BgOrg", "BgYel", "BgGrn", "BgCya", "BgBlu", "BgPur", "BgMag", "BgWhi", "BgHeat"}
	// Each Bg family name embeds an existing fg family name (e.g. "BgGra"
	// contains "Gra"), so check the full "Bg<X>" prefix appears in the
	// row label which is rendered as "Bg<X> " (label is left-aligned, with
	// a trailing space before the swatches). We accept any occurrence.
	for _, fam := range wantFamilies {
		if !strings.Contains(output, fam) {
			t.Errorf("ShowBgRamps output missing family label %q\noutput:\n%s", fam, output)
		}
	}

	if got, want := strings.Count(output, "TOKEN"), 11*11; got != want {
		t.Errorf("default token count: got %d, want %d", got, want)
	}

	custom := captureShowBgRamps(t, "HEADER", -1)
	if got, want := strings.Count(custom, "HEADER"), 11*11; got != want {
		t.Errorf("custom token count: got %d, want %d", got, want)
	}
	if strings.Contains(custom, "TOKEN") {
		t.Errorf("default token leaked into custom-token output")
	}
}

// TestShowBgRampsAutoContrast forces color mode and verifies the auto-contrast
// path produces both white text (on dark bg steps) and black text (on light
// bg steps). Confirms contrastFg is wired up correctly and the SGR composition
// emits the combined 48;5;<bg>;38;5;<fg>m sequence.
func TestShowBgRampsAutoContrast(t *testing.T) {
	defer SetEnabled(true)()
	prev := color256
	color256 = true
	defer func() { color256 = prev }()

	output := captureShowBgRamps(t, "x", -1)

	if !strings.Contains(output, ";38;5;15m") {
		t.Errorf("auto-contrast missing white-on-dark (;38;5;15m)\noutput:\n%q", output)
	}
	if !strings.Contains(output, ";38;5;16m") {
		t.Errorf("auto-contrast missing black-on-light (;38;5;16m)\noutput:\n%q", output)
	}
}

// TestContrastFgThreshold spot-checks the readability threshold across the
// 256-color space: cube black/white endpoints, cube primaries red/green,
// grayscale endpoints, and a grayscale midpoint.
func TestContrastFgThreshold(t *testing.T) {
	cases := []struct {
		idx  int
		want int
		note string
	}{
		{16, 15, "cube black → white text"},
		{231, 16, "cube white → black text"},
		{196, 15, "Red5 → white text"},
		{245, 16, "gray midpoint (level 138) → black text"},
		{232, 15, "darkest gray (level 8) → white text"},
		{255, 16, "lightest gray (level 238) → black text"},
		{46, 16, "Grn5 (bright green) → black text"},
	}
	for _, tc := range cases {
		if got := contrastFg(tc.idx); got != tc.want {
			t.Errorf("contrastFg(%d) = %d, want %d (%s)", tc.idx, got, tc.want, tc.note)
		}
	}
}
