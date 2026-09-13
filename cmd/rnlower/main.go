package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "rnlower"
	programVersion = "2.1.0"
)

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Rename every file in the current directory to lowercase",
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
	fmt.Print("Rename ALL filenames in CWD to lowercase? Y/N ")
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
		newName := strings.ToLower(oldName)

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

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
