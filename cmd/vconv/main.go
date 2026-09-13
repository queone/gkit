// main.go

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/help"
	"github.com/queone/gkit/internal/vedit"
)

const (
	programName    = "vconv"
	programVersion = "1.1.0"
)

// transcode is the ffmpeg seam, replaced in tests so no real encode runs.
var transcode = vedit.Transcode

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
		Description: "Convert a video to an H.264 MP4 by driving ffmpeg",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " INPUT", Meaning: "Write INPUT's name with an .mp4 extension next to it"}},
				Lines: []string{
					"INPUT is any video ffmpeg can read, typically a WebM. vconv refuses to",
					"overwrite an existing file and needs ffmpeg and ffprobe (brew install ffmpeg).",
				}},
			{Title: "Examples", Rows: []help.Row{{Form: programName + " talk.webm", Meaning: "writes talk.mp4"}}},
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

// outputPath returns input's path with its extension replaced by .mp4.
func outputPath(input string) string {
	ext := filepath.Ext(input)
	return input[:len(input)-len(ext)] + ".mp4"
}

// ffmpegArgs builds the complete ffmpeg argument list for one conversion.
func ffmpegArgs(input, output string) []string {
	return []string{"-i", input, "-c:v", "libx264", "-crf", "23", "-preset", "fast", "-c:a", "aac", output}
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
	output := outputPath(input)
	if output == input {
		return fail(e.stderr, fmt.Errorf("input %q is already an .mp4 file", input))
	}
	if _, err := os.Stat(output); err == nil {
		return fail(e.stderr, fmt.Errorf("output %q already exists; refusing to overwrite", output))
	}
	fmt.Fprintf(e.stdout, "==> Converting %s\n", color.Yel5(input))
	if err := transcode(input, output, ffmpegArgs(input, output)); err != nil {
		return fail(e.stderr, err)
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], env{stdout: os.Stdout, stderr: os.Stderr}))
}
