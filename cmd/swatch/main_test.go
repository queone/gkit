package main

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// captureRun invokes run(args) in-process, capturing stdout/stderr by
// swapping the package-level os.Stdout/os.Stderr vars around the call.
// internal/color's Show* functions write directly to os.Stdout, so this
// swap captures their output too, not just swatch's own text.
func captureRun(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	ro, wo, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	re, we, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wo, we
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	code = run(args)

	wo.Close()
	we.Close()
	outBytes, _ := io.ReadAll(ro)
	errBytes, _ := io.ReadAll(re)
	return code, string(outBytes), string(errBytes)
}

func TestSwatchAliasesRenderIdentically(t *testing.T) {
	pairs := [][2]string{{"p", "palette"}, {"g", "grid"}, {"b", "backgrounds"}}
	for _, pair := range pairs {
		shortCode, shortOut, shortErr := captureRun(t, []string{pair[0]})
		longCode, longOut, longErr := captureRun(t, []string{pair[1]})
		if shortCode != 0 || longCode != 0 {
			t.Errorf("%v: codes = %d, %d, want 0, 0", pair, shortCode, longCode)
		}
		if shortOut != longOut {
			t.Errorf("%v: stdout mismatch between alias and full name", pair)
		}
		if shortErr != longErr {
			t.Errorf("%v: stderr mismatch between alias and full name", pair)
		}
	}
}

func TestSwatchGridForegroundRequiresReverse(t *testing.T) {
	code, out, errOut := captureRun(t, []string{"g", "-f", "15"})
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "--foreground requires --reverse") {
		t.Errorf("stderr = %q, missing requirement message", errOut)
	}
}

func TestSwatchForegroundIndexValidation(t *testing.T) {
	cases := [][]string{
		{"g", "-r", "-f", "256"},
		{"g", "-r", "--foreground", "-1"},
		{"g", "-r", "-f", "abc"},
		{"b", "-f", "256"},
		{"b", "--foreground", "xyz"},
	}
	for _, args := range cases {
		code, out, errOut := captureRun(t, args)
		if code != 2 {
			t.Errorf("%v: code = %d, want 2", args, code)
		}
		if out != "" {
			t.Errorf("%v: stdout = %q, want empty", args, out)
		}
		if !strings.HasPrefix(errOut, "swatch: ") {
			t.Errorf("%v: stderr = %q, missing prefix", args, errOut)
		}
	}
}

func TestSwatchInvalidInput(t *testing.T) {
	cases := [][]string{
		{"unknown"},
		{"p", "extra"},
		{"g", "--bad"},
		{"g", "-f"},
		{"b", "extra1", "extra2"},
	}
	for _, args := range cases {
		code, out, errOut := captureRun(t, args)
		if code != 2 {
			t.Errorf("%v: code = %d, want 2", args, code)
		}
		if out != "" {
			t.Errorf("%v: stdout not empty: %q", args, out)
		}
		if !strings.HasPrefix(errOut, "swatch: ") || !strings.Contains(errOut, "swatch --help") {
			t.Errorf("%v: stderr = %q, missing diagnostic shape", args, errOut)
		}
	}
}

func TestSwatchDefaultToken(t *testing.T) {
	code, gridOut, _ := captureRun(t, []string{"g", ""})
	if code != 0 {
		t.Fatalf("grid: code = %d, want 0", code)
	}
	if !strings.Contains(gridOut, "TOKEN") {
		t.Errorf("grid with empty token missing default TOKEN: %q", gridOut)
	}

	code, bgOut, _ := captureRun(t, []string{"b"})
	if code != 0 {
		t.Fatalf("backgrounds: code = %d, want 0", code)
	}
	if !strings.Contains(bgOut, "TOKEN") {
		t.Errorf("backgrounds with omitted token missing default TOKEN: %q", bgOut)
	}
}

func TestSwatchVersionOutput(t *testing.T) {
	for _, args := range [][]string{{"-v"}, {"--version"}} {
		code, out, errOut := captureRun(t, args)
		if code != 0 {
			t.Errorf("%v: code = %d, want 0", args, code)
		}
		want := "swatch v" + programVersion + "\n"
		if out != want {
			t.Errorf("%v: stdout = %q, want %q", args, out, want)
		}
		if errOut != "" {
			t.Errorf("%v: stderr not empty: %q", args, errOut)
		}
	}
}

func TestSwatchVersionCombinationRejected(t *testing.T) {
	cases := [][]string{{"-v", "p"}, {"p", "--version"}, {"--version", "extra"}}
	for _, args := range cases {
		code, out, errOut := captureRun(t, args)
		if code != 2 {
			t.Errorf("%v: code = %d, want 2", args, code)
		}
		if out != "" {
			t.Errorf("%v: stdout not empty: %q", args, out)
		}
		if !strings.Contains(errOut, "version flag cannot be combined with a subcommand or operand") {
			t.Errorf("%v: stderr = %q, missing message", args, errOut)
		}
	}
}

func TestSwatchHelpOutput(t *testing.T) {
	for _, args := range [][]string{{}, {"-h"}, {"--help"}, {"-?"}} {
		code, out, errOut := captureRun(t, args)
		if code != 0 {
			t.Errorf("%v: code = %d, want 0", args, code)
		}
		for _, want := range []string{"Usage", "Options", "Examples", "p, palette", "g, grid", "b, backgrounds"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: help output missing %q", args, want)
			}
		}
		if errOut != "" {
			t.Errorf("%v: stderr not empty: %q", args, errOut)
		}
	}
}

func TestSwatchHelpCombinationRejected(t *testing.T) {
	cases := [][]string{{"-h", "p"}, {"p", "--help"}, {"-?", "extra"}}
	for _, args := range cases {
		code, out, errOut := captureRun(t, args)
		if code != 2 {
			t.Errorf("%v: code = %d, want 2", args, code)
		}
		if out != "" {
			t.Errorf("%v: stdout not empty: %q", args, out)
		}
		if !strings.Contains(errOut, "help flag cannot be combined with a subcommand or operand") {
			t.Errorf("%v: stderr = %q, missing message", args, errOut)
		}
	}
}

// TestSwatchHelperProcess is invoked as a subprocess by runSwatchSubprocess
// to exercise swatch's real os.Getenv-based color detection, which is fixed
// at process-init time and can't be re-triggered by t.Setenv in-process.
func TestSwatchHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_SWATCH") != "1" {
		return
	}

	args := os.Args
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep == -1 {
		os.Exit(2)
	}

	os.Args = append([]string{"swatch"}, args[sep+1:]...)
	main()
}

func runSwatchSubprocess(t *testing.T, env []string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-test.run=TestSwatchHelperProcess", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(env, "GO_WANT_HELPER_PROCESS_SWATCH=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("subprocess failed: %v", err)
		}
	}
	return string(out)
}

// TestSwatchColorPolicySuppressesSGR mirrors rkit's
// color_policy_suppresses_sgr_for_nonterminal_and_unsupported_environments:
// it spawns the real swatch binary under several NO_COLOR/TERM/COLORTERM
// combinations and confirms no SGR escapes leak into non-interactive
// output. This exercises internal/color's real os.Getenv-based detection
// path (AC67's ported test suite only unit-tests wrap/bgWrap gating via
// pre-injected bools, never the actual env-var reads).
func TestSwatchColorPolicySuppressesSGR(t *testing.T) {
	var baseEnv []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "NO_COLOR=") || strings.HasPrefix(e, "TERM=") || strings.HasPrefix(e, "COLORTERM=") {
			continue
		}
		baseEnv = append(baseEnv, e)
	}

	cases := []struct {
		name      string
		noColor   string
		term      string
		colorterm string
	}{
		{"no_color_set", "1", "xterm-256color", "truecolor"},
		{"term_dumb", "", "dumb", "truecolor"},
		{"no_256_support", "", "xterm", ""},
		{"nothing_set", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := append([]string{}, baseEnv...)
			if tc.noColor != "" {
				env = append(env, "NO_COLOR="+tc.noColor)
			}
			if tc.term != "" {
				env = append(env, "TERM="+tc.term)
			}
			if tc.colorterm != "" {
				env = append(env, "COLORTERM="+tc.colorterm)
			}
			out := runSwatchSubprocess(t, env, "p")
			if strings.Contains(out, "\x1b[") {
				t.Errorf("case %s: expected plain output, got escapes: %q", tc.name, out)
			}
			if !strings.Contains(out, "Standard 16-color palette") {
				t.Errorf("case %s: missing expected palette content: %q", tc.name, out)
			}
		})
	}
}
