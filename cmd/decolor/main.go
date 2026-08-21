package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"
	icolor "github.com/queone/governa-color"
)

const (
	programName    = "decolor"
	programVersion = "1.1.1"
)

func printUsage() {
	n := icolor.Whi10(programName)
	v := programVersion
	usage := fmt.Sprintf("%s v%s\n"+
		"Text decolorizer - https://github.com/queone/gkit/blob/main/cmd/decolor/README.md\n"+
		"%s\n"+
		"  %s [options] [file]\n"+
		"\n"+
		"  The file can be piped into the utility, or it can be referenced as an argument.\n"+
		"\n"+
		"%s\n"+
		"  |piped input|       Piped text is decolorized\n"+
		"  FILENAME            Decolorize given file path\n"+
		"  -v, --version       Print version and exit\n"+
		"  -?, --help, -h      Show this help message and exit\n"+
		"\n"+
		"%s\n"+
		"  cat file | %s\n"+
		"  %s /path/to/file\n"+
		"  %s -h\n",
		n, v, icolor.Whi10("Usage"), n, icolor.Whi10("Options"), icolor.Whi10("Examples"), n, n, n)
	fmt.Print(usage)
	os.Exit(0)
}

func isGitBashOnWindows() bool {
	return runtime.GOOS == "windows" && strings.HasPrefix(os.Getenv("MSYSTEM"), "MINGW")
}

func hasPipedInput() bool {
	stat, _ := os.Stdin.Stat() // Check if anything was piped in
	if isGitBashOnWindows() {
		// Git Bash on Windows handles input redirection differently than other shells. When a program
		// is run without any input or arguments, it still treats the input as if it were piped from an
		// empty stream, causing the program to consider it as piped input and hang. This works around that.
		if !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			return true
		}
	} else {
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			return true
		}
	}
	return false
}

func loadAndDecolorize(filename string) {
	// Read content from the given file
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filename, err)
		os.Exit(1)
	}

	// Remove color codes from file content and print
	decolorizedText := icolor.ClearCode(string(fileBytes))
	fmt.Print(decolorizedText)
}

func main() {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-v", "--version":
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		case "-?", "-h", "--help":
			printUsage()
		default:
			loadAndDecolorize(os.Args[1])
		}
	} else if hasPipedInput() {
		// Process piped input
		rawBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading from stdin:", err)
		}

		// Remove color escape codes in piped input, then print
		decolorizedText := icolor.ClearCode(string(rawBytes))
		fmt.Print(decolorizedText)
	} else {
		printUsage()
	}
}
