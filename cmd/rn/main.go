package main

import (
	"fmt"
	"os"
	"strings"

	icolor "github.com/queone/gkit/internal/color"
)

const (
	programName    = "rn"
	programVersion = "1.5.0"
)

func printUsage() {
	n := icolor.Whi10(programName)
	v := programVersion
	usage := fmt.Sprintf("%s v%s\n"+
		"Bulk file re-namer — https://github.com/queone/gkit/blob/main/cmd/rn/README.md\n"+
		"\n"+
		"%s\n"+
		"  %s \"OldString\" \"NewString\" [-f]\n"+
		"\n"+
		"  Renames all files in the current directory by replacing occurrences of OldString\n"+
		"  in filenames with NewString. If NewString is empty (\"\"), the OldString is removed.\n"+
		"\n"+
		"%s\n"+
		"  -f                     Perform actual renaming (required to make changes).\n"+
		"  -v, --version          Print version and exit.\n"+
		"  -?, --help, -h         Show this help message and exit.\n"+
		"\n"+
		"%s\n"+
		"  %s \"_draft\" \"\"           Show files that would be renamed (dry run).\n"+
		"  %s \"_draft\" \"\" -f       Actually rename files.\n"+
		"  %s \"temp\" \"final\" -f     Replace one substring with another.\n"+
		"  %s -v                   Print version.\n"+
		"  %s -h                   Display this help message.\n",
		n, v,
		icolor.Whi10("Usage"), n,
		icolor.Whi10("Options"),
		icolor.Whi10("Examples"),
		n, n, n, n, n)
	fmt.Print(usage)
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
