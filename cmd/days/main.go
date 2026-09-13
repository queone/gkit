// main.go

// TODO: Check back on 03:14:08 UTC on 19 January 2038, to confirm we're good ;-)
//       https://en.wikipedia.org/wiki/Year_2038_problem

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/queone/gkit/internal/help"
)

const (
	// Global constants
	programName    = "days"
	programVersion = "1.2.0"
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

func helpDoc() help.Doc {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Count calendar days between dates, or find the date N days away",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{
				{Form: "days -N", Meaning: "Print the date N days ago"},
				{Form: "days +N", Meaning: "Print the date N days ahead; a bare N means +N"},
				{Form: "days DATE", Meaning: "Print the days from today to DATE, negative when DATE is past"},
				{Form: "days DATE DATE", Meaning: "Print the days between the two dates"},
			}, Lines: []string{"DATE is YYYY-MM-DD or YYYY-MMM-DD."}},
			{Title: "Examples", Rows: []help.Row{
				{Form: "days -11", Meaning: "The date eleven days ago"},
				{Form: "days 6", Meaning: "The date six days ahead"},
				{Form: "days 2026-12-25", Meaning: "Days until that date"},
			}},
		},
	}
}

func usage() string { return helpDoc().String() }

// printUsage prints the help and exits 0; it also answers a bad command line.
func printUsage() {
	fmt.Print(usage())
	os.Exit(0)
}

func main() {
	numberOfArguments := len(os.Args[1:]) // Not including the program itself
	if numberOfArguments < 1 || numberOfArguments > 2 {
		// Don't accept less than 1 or more than 2 arguments
		printUsage()
	}

	// Process given arguments
	switch numberOfArguments {
	case 1:
		arg1 := os.Args[1]
		if arg1 == "-v" || arg1 == "--version" {
			fmt.Printf("%s v%s\n", programName, programVersion)
		} else if arg1 == "-h" || arg1 == "-?" || arg1 == "--help" {
			printUsage()
		} else if validDate(arg1, "2006-01-02") {
			days, err := getDaysSinceOrTo(arg1)
			if err != nil {
				die("days: bad date %q: %v\n", arg1, err)
			}
			printDays(days)
		} else if arg1[0:1] == "+" || arg1[0:1] == "-" {
			dateStr, err := getDateInDays(arg1)
			if err != nil {
				die("days: bad offset %q: %v\n", arg1, err)
			}
			fmt.Println(dateStr.Format("2006-01-02"))
		} else if _, err := strconv.Atoi(arg1); err == nil { // Check if arg1 is a valid number
			arg1 = "+" + arg1
			dateStr, err := getDateInDays(arg1)
			if err != nil {
				die("days: bad offset %q: %v\n", arg1, err)
			}
			fmt.Println(dateStr.Format("2006-01-02"))
		}
	case 2:
		arg1 := os.Args[1]
		arg2 := os.Args[2]
		if validDate(arg1, "2006-01-02") && validDate(arg2, "2006-01-02") {
			days, err := getDaysBetween(arg1, arg2)
			if err != nil {
				die("days: bad date pair %q %q: %v\n", arg1, arg2, err)
			}
			printDays(days)
		}
	default:
		printUsage()
	}
}
