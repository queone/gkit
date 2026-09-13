package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "fr"
	programVersion = "1.1.0"
)

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

func isTextFile(path string) bool {
	cmd := exec.Command("file", "-b", "--mime-type", path)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	mime := strings.TrimSpace(string(out))
	if mime == "application/xml" || mime == "application/json" {
		return true
	}
	return strings.HasPrefix(mime, "text/")
}

func countMatches(data []byte, pattern string) int {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0 // invalid regex
	}
	return len(re.FindAllIndex(data, -1))
}

func replaceAll(data []byte, pattern, to string) []byte {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return data // invalid regex
	}
	return re.ReplaceAll(data, []byte(to))
}

func highlightLine(line, pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return line // invalid regex
	}
	return re.ReplaceAllStringFunc(line, func(m string) string {
		return color.Red5(m)
	})
}

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Find a regular expression in the text files under the current directory, or replace it",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{
				{Form: programName + " REGEX", Meaning: "Print every matching line with its file and line number"},
				{Form: programName + " FROM TO", Meaning: "Print the lines that FROM matches without changing any file"},
				{Form: programName + " FROM TO -f", Meaning: "Replace FROM with TO in every matching file"},
			}, Lines: []string{"Hidden directories are skipped. Only files the file command reports as text are read."}},
			{Title: "Options", Rows: []help.Row{{Form: "-f", Meaning: "Write the replacement instead of showing matches"}}},
		},
	}.String()
}

// ---------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------

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

	var from, to string
	var replaceMode, singleMode bool

	switch len(os.Args) {
	case 2:
		// single-argument search
		from = os.Args[1]
		singleMode = true
	case 3:
		// show-only mode
		from = os.Args[1]
		to = os.Args[2]
	case 4:
		// replace-and-write mode
		from = os.Args[1]
		to = os.Args[2]
		if os.Args[3] != "-f" {
			fmt.Fprintf(os.Stderr, "Unrecognised flag %q. Only -f is supported.\n", os.Args[3])
			os.Exit(1)
		}
		replaceMode = true
	default:
		fmt.Fprint(os.Stderr, usage())
		os.Exit(1)
	}

	// -------------------- walk the tree --------------------
	err := filepath.Walk(".", func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != "." {
			return filepath.SkipDir
		}
		if info.IsDir() || !info.Mode().IsRegular() || !isTextFile(path) {
			return nil
		}

		orig, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		occ := countMatches(orig, from)
		if occ == 0 {
			return nil
		}

		if replaceMode {
			// ---------------- replacement mode ----------------
			newContent := replaceAll(orig, from, to)
			tmp := path + ".tmp"
			if err := os.WriteFile(tmp, newContent, info.Mode()); err != nil {
				return err
			}
			if err := os.Rename(tmp, path); err != nil {
				return err
			}
			fmt.Printf("%s: %d occurrence(s) replaced\n", color.Yel5(path), occ)
		} else if singleMode || (!replaceMode && !singleMode) {
			// ---------------- show-only or single-arg search ----------------
			scanner := bufio.NewScanner(strings.NewReader(string(orig)))
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()
				if countMatches([]byte(line), from) > 0 {
					hl := highlightLine(line, from)
					fmt.Printf("%s:%d: %s\n", color.Yel5(path), lineNum, hl)
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("walk error: %v", err)
	}
}
