package main

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/queone/gkit/internal/color"
)

const (
	programName    = "swatch"
	programVersion = "1.0.0"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches swatch's subcommands and returns the process exit code.
// internal/color's ShowPalette/ShowGrid/ShowBgRamps write directly to
// os.Stdout, so tests capture output by swapping os.Stdout/os.Stderr around
// this call rather than threading injected writers through it.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Print(helpText())
		return 0
	}
	if len(args) == 1 && isVersionFlag(args[0]) {
		fmt.Printf("%s v%s\n", programName, programVersion)
		return 0
	}
	if len(args) == 1 && isHelpFlag(args[0]) {
		fmt.Print(helpText())
		return 0
	}
	if slices.ContainsFunc(args, isVersionFlag) {
		return diagnostic("version flag cannot be combined with a subcommand or operand")
	}
	if slices.ContainsFunc(args, isHelpFlag) {
		return diagnostic("help flag cannot be combined with a subcommand or operand")
	}

	name := args[0]
	switch name {
	case "p", "palette":
		if len(args) != 1 {
			return diagnostic(fmt.Sprintf("%s: does not accept options or operands", name))
		}
		color.ShowPalette()
		return 0
	case "g", "grid":
		token, reverse, foreground, errMsg := parseGridArgs(name, args[1:])
		if errMsg != "" {
			return diagnostic(errMsg)
		}
		color.ShowGrid(token, reverse, foreground)
		return 0
	case "b", "backgrounds":
		token, foreground, errMsg := parseBackgroundArgs(name, args[1:])
		if errMsg != "" {
			return diagnostic(errMsg)
		}
		color.ShowBgRamps(token, foreground)
		return 0
	default:
		return diagnostic(fmt.Sprintf("unknown subcommand %q; use p, g, or b", name))
	}
}

// parseGridArgs parses grid's -r/--reverse and -f/--foreground INDEX flags
// plus an optional token operand. foreground is -1 when unset. --foreground
// is only valid combined with --reverse.
func parseGridArgs(name string, args []string) (token string, reverse bool, foreground int, errMsg string) {
	foreground = -1
	haveForeground := false
	haveToken := false
	for i := 0; i < len(args); i++ {
		v := args[i]
		switch {
		case v == "-r" || v == "--reverse":
			reverse = true
		case v == "-f" || v == "--foreground":
			if haveForeground {
				errMsg = fmt.Sprintf("%s: foreground option may be used only once", name)
				return
			}
			i++
			if i >= len(args) {
				errMsg = fmt.Sprintf("%s: %s requires an xterm index from 0 through 255", name, v)
				return
			}
			idx, ierr := parseIndex(name, args[i])
			if ierr != "" {
				errMsg = ierr
				return
			}
			foreground = idx
			haveForeground = true
		case strings.HasPrefix(v, "-"):
			errMsg = fmt.Sprintf("%s: unknown option %q", name, v)
			return
		case !haveToken:
			token = v
			haveToken = true
		default:
			errMsg = fmt.Sprintf("%s: extra operand %q; provide at most one token", name, v)
			return
		}
	}
	if haveForeground && !reverse {
		errMsg = fmt.Sprintf("%s: --foreground requires --reverse", name)
	}
	return
}

// parseBackgroundArgs parses backgrounds' -f/--foreground INDEX flag plus an
// optional token operand. foreground is -1 when unset (auto-contrast).
func parseBackgroundArgs(name string, args []string) (token string, foreground int, errMsg string) {
	foreground = -1
	haveForeground := false
	haveToken := false
	for i := 0; i < len(args); i++ {
		v := args[i]
		switch {
		case v == "-f" || v == "--foreground":
			if haveForeground {
				errMsg = fmt.Sprintf("%s: foreground option may be used only once", name)
				return
			}
			i++
			if i >= len(args) {
				errMsg = fmt.Sprintf("%s: %s requires an xterm index from 0 through 255", name, v)
				return
			}
			idx, ierr := parseIndex(name, args[i])
			if ierr != "" {
				errMsg = ierr
				return
			}
			foreground = idx
			haveForeground = true
		case strings.HasPrefix(v, "-"):
			errMsg = fmt.Sprintf("%s: unknown option %q", name, v)
			return
		case !haveToken:
			token = v
			haveToken = true
		default:
			errMsg = fmt.Sprintf("%s: extra operand %q; provide at most one token", name, v)
			return
		}
	}
	return
}

func parseIndex(name, value string) (int, string) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || n > 255 {
		return 0, fmt.Sprintf("%s: foreground index %q must be an integer from 0 through 255", name, value)
	}
	return n, ""
}

func isVersionFlag(a string) bool { return a == "-v" || a == "--version" }
func isHelpFlag(a string) bool    { return a == "-?" || a == "-h" || a == "--help" }

func diagnostic(message string) int {
	fmt.Fprintf(os.Stderr, "%s: %s; run '%s --help' for usage\n", programName, message, programName)
	return 2
}

func helpText() string {
	n := color.Whi10(programName)
	return fmt.Sprintf(
		"%s v%s\nXterm palette and ramp inspector\n"+
			"%s\n  %s <subcommand> [options] [TOKEN]\n"+
			"\n"+
			"Subcommands\n"+
			"  p, palette       Print the complete xterm palette and color ramps\n"+
			"  g, grid          Print the bordered ramp-by-step grid\n"+
			"  b, backgrounds   Print background-ramp swatch rows\n"+
			"\n"+
			"%s\n"+
			"  -r, --reverse          Use ramp colors as grid cell backgrounds\n"+
			"  -f, --foreground INDEX Set grid or swatch text to xterm INDEX (0-255)\n"+
			"  -v, --version          Print version and exit\n"+
			"  -?, -h, --help         Print this usage page\n"+
			"\n"+
			"  TOKEN defaults to TOKEN when omitted or empty. Grid --foreground requires --reverse.\n"+
			"\n"+
			"%s\n"+
			"  %s p\n"+
			"  %s palette\n"+
			"  %s g --reverse HEADER\n"+
			"  %s b --foreground 15 LABEL\n",
		n, programVersion,
		color.Whi10("Usage"), n,
		color.Whi10("Options"),
		color.Whi10("Examples"),
		n, n, n, n,
	)
}
