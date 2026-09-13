// main.go

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/help"
	"github.com/queone/gkit/internal/vedit"
)

const (
	programName    = "vshrink"
	programVersion = "1.1.0"
)

// Injectable seams, replaced in tests so no real probe or encode runs and the
// output name is deterministic.
var (
	probeFormat = vedit.ProbeFormat
	transcode   = vedit.Transcode
	now         = time.Now
)

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
		Description: "Shrink an MP4 by re-encoding it at a high compression level via ffmpeg",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " INPUT", Meaning: "Write INPUT's stem plus today's date as a smaller MP4 next to it"}},
				Lines: []string{
					"INPUT must be an MP4; vshrink checks the container with ffprobe first. It",
					"refuses to overwrite an existing file and needs ffmpeg and ffprobe (brew install ffmpeg).",
				}},
			{Title: "Examples", Rows: []help.Row{{Form: programName + " clip.mp4", Meaning: "writes clip_20260908a.mp4"}}},
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

// outputPath returns input's stem with a date suffix and an .mp4 extension,
// matching the retired script: clip.mp4 -> clip_20260908a.mp4.
func outputPath(input string, t time.Time) string {
	ext := filepath.Ext(input)
	return input[:len(input)-len(ext)] + "_" + t.Format("20060102") + "a.mp4"
}

// ffmpegArgs builds the complete ffmpeg argument list for one shrink.
func ffmpegArgs(input, output string) []string {
	return []string{"-i", input, "-vcodec", "libx264", "-crf", "35", output}
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
	if fi, err := os.Stat(input); err != nil || fi.IsDir() {
		return fail(e.stderr, fmt.Errorf("input %q: not a readable file", input))
	}
	format, err := probeFormat(input)
	if err != nil {
		return fail(e.stderr, err)
	}
	if !strings.Contains(format, "mp4") {
		return fail(e.stderr, fmt.Errorf("%s is not a valid MP4 file (ffprobe reports %q)", input, format))
	}
	output := outputPath(input, now())
	if _, err := os.Stat(output); err == nil {
		return fail(e.stderr, fmt.Errorf("output %q already exists; refusing to overwrite", output))
	}
	fmt.Fprintf(e.stdout, "==> Shrinking %s\n", color.Yel5(input))
	if err := transcode(input, output, ffmpegArgs(input, output)); err != nil {
		return fail(e.stderr, err)
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], env{stdout: os.Stdout, stderr: os.Stderr}))
}
