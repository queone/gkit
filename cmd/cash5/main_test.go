package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/queone/gkit/internal/color"
)

func TestVersionAliases(t *testing.T) {
	const envName = "GKIT_CASH5_VERSION_FLAG"
	if flag := os.Getenv(envName); flag != "" {
		os.Args = []string{programName, flag}
		runCLI()
		os.Exit(0)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			cmd := exec.Command(exe, "-test.run=^TestVersionAliases$")
			cmd.Env = append(os.Environ(), envName+"="+flag)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("run %s: %v", flag, err)
			}
			if got, want := stdout.String(), programName+" v"+programVersion+"\n"; got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

// The help screen follows the shared standard: three header lines, then the
// sections the renderer accepts, with the standard rows left to the renderer.
func TestHelpDocFollowsTheStandard(t *testing.T) {
	if err := helpDoc().Check(); err != nil {
		t.Fatal(err)
	}
}

// The bare run ends with the official website, URL in dark gray.
func TestWebsiteLineEndsTheBareRun(t *testing.T) {
	restore := color.SetEnabled(false)
	if got, want := websiteLine(), "  Website: https://www.njlottery.com/en-us/drawgames/jerseycash.html"; got != want {
		t.Errorf("plain = %q, want %q", got, want)
	}
	restore()
	defer color.SetEnabled(true)()
	defer color.Set256(true)()
	got := websiteLine()
	if !strings.HasPrefix(got, "  Website: \x1b[38;5;242mhttps://www.njlottery.com/en-us/drawgames/jerseycash.html\x1b[0m") {
		t.Errorf("colored = %q", got)
	}
}
