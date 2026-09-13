package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"
	icolor "github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "decolor"
	programVersion = "1.2.0"
)

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Strip shell color escape codes from a file or piped text",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{
				{Form: programName + " FILE", Meaning: "Print FILE without its color escape codes"},
				{Form: "... | " + programName, Meaning: "Print piped text without its color escape codes"},
			}},
			{Title: "Examples", Rows: []help.Row{
				{Form: "cat file | " + programName, Meaning: ""},
				{Form: programName + " /path/to/file", Meaning: ""},
			}},
		},
	}.String()
}

// printUsage prints the help and exits 0; it also answers a run with no input.
func printUsage() {
	fmt.Print(usage())
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
