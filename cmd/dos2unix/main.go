package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/queone/gkit/internal/help"
)

const (
	blue           = "\033[34m"
	reset          = "\033[0m"
	programName    = "dos2unix"
	programVersion = "2.1.0"
)

func helpText() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Preview or convert CRLF line endings to LF",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{
				{Form: programName + " FILE", Meaning: "Print FILE with each CRLF shown in blue"},
				{Form: programName + " FILE -f", Meaning: "Rewrite FILE with LF line endings"},
			}},
			{Title: "Options", Rows: []help.Row{{Form: "-f", Meaning: "Convert in place instead of previewing"}}},
		},
	}.String()
}

// usage prints the help to stderr and exits 1 for a bad command line.
func usage() {
	fmt.Fprint(os.Stderr, helpText())
	os.Exit(1)
}

func main() {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-v", "--version":
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		case "-h", "-?", "--help":
			fmt.Print(helpText())
			return
		}
	}

	if len(os.Args) < 2 || len(os.Args) > 3 {
		usage()
	}

	path := os.Args[1]
	force := false

	if len(os.Args) == 3 {
		if os.Args[2] != "-f" {
			usage()
		}
		force = true
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open error: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	if force {
		// Read entire file, convert, write back
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			os.Exit(1)
		}

		converted := strings.ReplaceAll(string(data), "\r\n", "\n")

		if err := os.WriteFile(path, []byte(converted), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Preview mode: cat file and highlight CRLF
	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if before, ok := strings.CutSuffix(line, "\r\n"); ok {
				// Show the CRLF explicitly in blue
				fmt.Print(before)
				fmt.Print(blue + "\\r\\n" + reset)
				fmt.Print("\n")
			} else {
				fmt.Print(line)
			}
		}

		if err != nil {
			break
		}
	}
}
