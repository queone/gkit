// main.go

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/help"
	"github.com/queone/gkit/internal/numfmt"
)

const (
	programName    = "ishrink"
	programVersion = "1.1.0"
	usageLine      = "usage: ishrink FILE.[heic|jpeg|jpg]"
)

// Injectable seams, replaced in tests so no real sips runs, the platform check
// is controllable, and the output name is deterministic.
var (
	lookPath = exec.LookPath
	runSips  = func(args []string) error {
		cmd := exec.Command("sips", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	now      = time.Now
	isDarwin = runtime.GOOS == "darwin"
)

// supported lists the input extensions, lower-cased, that sips re-encodes here.
var supported = map[string]bool{".heic": true, ".jpeg": true, ".jpg": true}

// env carries the output streams so tests can capture them.
type env struct {
	stdout io.Writer
	stderr io.Writer
}

// helpDoc describes the help screen.
func helpDoc() help.Doc {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Shrink a HEIC, JPEG, or JPG image into a small JPEG via macOS sips",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " INPUT", Meaning: "Write INPUT's stem plus today's date as a small .jpg next to it"}},
				Lines: []string{
					"INPUT is a .heic, .jpeg, or .jpg file. ishrink refuses to overwrite an",
					"existing file. sips ships with macOS, so ishrink runs only there.",
				}},
			{Title: "Examples", Rows: []help.Row{{Form: programName + " photo.heic", Meaning: "writes photo_20260908a.jpg"}}},
		},
	}
}

// usage returns the help screen.
func usage() string { return helpDoc().String() }

// fail prints err with the program name and returns the failure exit code.
func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s: %v\n", programName, err)
	return 1
}

// outputPath returns input's stem with a date suffix and a .jpg extension,
// matching the retired script: photo.heic -> photo_20260908a.jpg.
func outputPath(input string, t time.Time) string {
	ext := filepath.Ext(input)
	return input[:len(input)-len(ext)] + "_" + t.Format("20060102") + "a.jpg"
}

// sipsArgs builds the complete sips argument list for one re-encode.
func sipsArgs(input, output string) []string {
	return []string{"-s", "format", "jpeg", "-s", "formatOptions", "10", input, "-o", output}
}

// fileSize returns the size of path in bytes.
func fileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// run executes the utility with the given arguments and returns its exit code.
func run(args []string, e env) int {
	var input string
	for _, a := range args {
		switch {
		case a == "-v" || a == "--version":
			fmt.Fprintf(e.stdout, "%s v%s\n", programName, programVersion)
			return 0
		case a == "-h" || a == "-?" || a == "--help":
			fmt.Fprint(e.stdout, usage())
			return 0
		case strings.HasPrefix(a, "-") && a != "-":
			return fail(e.stderr, fmt.Errorf("unknown flag %q (see %s --help)", a, programName))
		case input != "":
			return fail(e.stderr, fmt.Errorf("expected one INPUT (see %s --help)", programName))
		default:
			input = a
		}
	}
	if input == "" {
		fmt.Fprint(e.stdout, usage())
		return 0
	}
	if !supported[strings.ToLower(filepath.Ext(input))] {
		fmt.Fprintln(e.stderr, usageLine)
		return 1
	}
	if fi, err := os.Stat(input); err != nil || fi.IsDir() {
		return fail(e.stderr, fmt.Errorf("input %q: not a readable file", input))
	}
	if !isDarwin {
		return fail(e.stderr, fmt.Errorf("sips is a macOS tool; %s runs only on macOS", programName))
	}
	if _, err := lookPath("sips"); err != nil {
		return fail(e.stderr, fmt.Errorf("sips not found on PATH; it ships with macOS at /usr/bin/sips"))
	}
	output := outputPath(input, now())
	if _, err := os.Stat(output); err == nil {
		return fail(e.stderr, fmt.Errorf("output %q already exists; refusing to overwrite", output))
	}
	fmt.Fprintf(e.stdout, "==> Shrinking %s\n", color.Yel5(input))
	if err := runSips(sipsArgs(input, output)); err != nil {
		return fail(e.stderr, fmt.Errorf("sips failed: %w", err))
	}
	inSize, err := fileSize(input)
	if err != nil {
		return fail(e.stderr, err)
	}
	outSize, err := fileSize(output)
	if err != nil {
		return fail(e.stderr, fmt.Errorf("sips reported success but %q is missing: %w", output, err))
	}
	fmt.Fprintf(e.stdout, "%s\n", color.Whi10(fmt.Sprintf("%-8s%-*s  %12s", "FILE", len(filepath.Base(output)), "NAME", "SIZE")))
	fmt.Fprintf(e.stdout, "%-8s%-*s  %12s\n", "input", len(filepath.Base(output)), filepath.Base(input), numfmt.Int(inSize))
	fmt.Fprintf(e.stdout, "%-8s%-*s  %12s\n", "output", len(filepath.Base(output)), filepath.Base(output), numfmt.Int(outSize))
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], env{stdout: os.Stdout, stderr: os.Stderr}))
}
