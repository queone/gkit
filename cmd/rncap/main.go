package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "rncap"
	programVersion = "2.1.0"
)

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Capitalize every word of every file name in the current directory",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName, Meaning: "Ask for confirmation, then rename every file in the current directory"}}},
		},
	}.String()
}

func main() {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-v", "--version":
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		case "-h", "-?", "--help":
			fmt.Print(usage())
			return
		}
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Capitalize every file in CWD? Y/N ")
	resp, _ := reader.ReadString('\n')
	resp = strings.TrimSpace(resp)

	if resp != "Y" && resp != "y" {
		fmt.Println("\nAborted.")
		os.Exit(1)
	}
	fmt.Println()

	entries, err := os.ReadDir(".")
	if err != nil {
		fail(err)
	}

	for _, e := range entries {
		oldName := e.Name()

		newName := titleCase(oldName)
		if newName == oldName {
			continue
		}

		if _, err := os.Stat(newName); err == nil {
			fmt.Fprintf(os.Stderr, "skipped (exists): %s\n", newName)
			continue
		}

		if err := os.Rename(oldName, newName); err != nil {
			fmt.Fprintf(os.Stderr, "rename failed: %s -> %s (%v)\n", oldName, newName, err)
			continue
		}

		fmt.Printf("'%s' -> '%s'\n", oldName, newName)
	}

	fmt.Println()
}

func titleCase(s string) string {
	var out []rune
	capNext := true

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if capNext {
				out = append(out, unicode.ToTitle(r))
				capNext = false
			} else {
				out = append(out, unicode.ToLower(r))
			}
		} else {
			out = append(out, r)
			capNext = true
		}
	}
	return string(out)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
