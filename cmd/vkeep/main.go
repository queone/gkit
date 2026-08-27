// main.go

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/queone/gkit/internal/vedit"
)

const (
	// Global constants
	programName    = "vkeep"
	programVersion = "0.3.0"
)

// die prints an error message to stderr and exits with status 1.
func die(format string, args ...any) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, format)
	} else {
		fmt.Fprintf(os.Stderr, format, args...)
	}
	os.Exit(1)
}

// printUsage prints the shared help screen and exits successfully.
func printUsage() {
	fmt.Print(vedit.Usage(programName, programVersion))
	os.Exit(0)
}

// runCLI parses os.Args manually and dispatches a keep.
func runCLI() {
	args := os.Args[1:]
	for _, a := range args {
		if a == "-v" || a == "--version" {
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		}
		if a == "-?" || a == "-h" || a == "--help" {
			printUsage()
			return
		}
	}
	if len(args) == 0 {
		printUsage()
		return
	}

	accurate := false
	var pos []string
	for _, a := range args {
		switch {
		case a == "-a" || a == "--accurate":
			accurate = true
		case a == "-x" || a == "--crossfade" || strings.HasPrefix(a, "--crossfade=") || strings.HasPrefix(a, "-x="):
			die("%s: --crossfade applies only to vdrop; vkeep keeps a single section and has no join to smooth\n", programName)
		case strings.HasPrefix(a, "-"):
			die("%s: unknown flag %q (see %s --help)\n", programName, a, programName)
		default:
			pos = append(pos, a)
		}
	}

	var startTok, endTok, input string
	switch len(pos) {
	case 2:
		startTok, endTok, input = pos[0], "end", pos[1]
	case 3:
		startTok, endTok, input = pos[0], pos[1], pos[2]
	default:
		die("%s: expected START [END] FILE (see %s --help)\n", programName, programName)
	}

	if err := vedit.Keep(accurate, startTok, endTok, input); err != nil {
		die("%s: %v\n", programName, err)
	}
}

func main() {
	runCLI()
}
