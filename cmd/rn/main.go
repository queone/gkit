package main

import (
	"fmt"
	"github.com/queone/gkit/internal/help"
	"os"
	"strings"

	icolor "github.com/queone/gkit/internal/color"
)

const (
	programName    = "rn"
	programVersion = "1.6.0"
)

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Rename files in the current directory by replacing a substring",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + ` "OLD" "NEW" [-f]`, Meaning: "Show every file name where OLD becomes NEW; rename with -f"}},
				Lines: []string{`An empty NEW ("") removes OLD from the name.`}},
			{Title: "Options", Rows: []help.Row{{Form: "-f", Meaning: "Rename the files instead of only showing the plan"}}},
			{Title: "Examples", Rows: []help.Row{
				{Form: programName + ` "_draft" ""`, Meaning: "Show the files that would be renamed"},
				{Form: programName + ` "_draft" "" -f`, Meaning: "Rename them"},
				{Form: programName + ` "temp" "final" -f`, Meaning: "Replace one substring with another"},
			}},
		},
	}.String()
}

// printUsage prints the help and exits 0; it also answers a bad command line.
func printUsage() {
	fmt.Print(usage())
	os.Exit(0)
}

func main() {
	args := os.Args[1:]
	if len(args) == 1 {
		switch args[0] {
		case "-v", "--version":
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		case "-?", "--help", "-h":
			printUsage()
		}
	}
	if len(args) < 1 || len(args) > 3 {
		printUsage()
	}

	oldStr := args[0]
	newStr := ""
	option := ""

	if len(args) >= 2 {
		newStr = args[1]
	}
	if len(args) == 3 {
		option = args[2]
	}

	doRename := option == "-f"
	if !doRename {
		fmt.Print(icolor.Yel5("DRY RUN: Re-run with '-f' option to execute.\n"))
	}

	files, err := os.ReadDir(".")
	if err != nil {
		fmt.Print(icolor.Red5(fmt.Sprintf("Error reading directory: %v\n", err)))
		os.Exit(1)
	}

	found := false
	for _, entry := range files {
		if entry.IsDir() {
			continue // skip directories
		}

		oldName := entry.Name()
		if !strings.Contains(oldName, oldStr) {
			continue
		}

		found = true
		newName := strings.ReplaceAll(oldName, oldStr, newStr)

		if doRename {
			err := os.Rename(oldName, newName)
			if err != nil {
				fmt.Print(icolor.Red5(fmt.Sprintf("Failed to rename %s -> %s: %v\n", oldName, newName, err)))
				continue
			}
			fmt.Print(icolor.Grn5(fmt.Sprintf("\"%s\" -> \"%s\"\n", oldName, newName)))
		} else {
			fmt.Printf("%-60s  =>  %s\n", fmt.Sprintf("\"%s\"", oldName), fmt.Sprintf("\"%s\"", newName))
		}
	}

	if !found {
		fmt.Print(icolor.Red5(fmt.Sprintf("No filename has string '%s'.\n", oldStr)))
		os.Exit(1)
	}

	os.Exit(0)
}
