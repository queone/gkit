package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/queone/gkit/internal/help"
)

// ANSI color codes
const (
	Green   = "\033[1;32m"
	Red     = "\033[1;31m"
	Magenta = "\033[1;35m"
	Yellow  = "\033[1;33m"
	Blue    = "\033[1;34m"
	Reset   = "\033[0m"
)

const (
	programName    = "brew-update"
	programVersion = "1.4.0"
)

// runCommand executes a command and streams its output
func runCommand(cmdStr string, color string) error {
	fmt.Printf("==> %s%s%s\n", color, cmdStr, Reset)

	parts := strings.Fields(cmdStr)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// upgradeCasks gets the list of installed casks and upgrades them
func upgradeCasks() error {
	// Get list of installed casks
	cmd := exec.Command("brew", "list", "--cask")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list casks: %w", err)
	}

	// Parse cask names
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var casks []string
	for scanner.Scan() {
		cask := strings.TrimSpace(scanner.Text())
		if cask != "" {
			casks = append(casks, cask)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to parse cask list: %w", err)
	}

	// Upgrade each cask
	if len(casks) > 0 {
		args := append([]string{"upgrade"}, casks...)
		cmdStr := "brew " + strings.Join(args, " ")
		return runCommand(cmdStr, Green)
	}

	return nil
}

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Update, upgrade, and clean up Homebrew formulae and casks",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName, Meaning: "Run brew update, brew upgrade, cask upgrades, and brew cleanup -s"}}},
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

	fmt.Printf("%s %s\n\n", programName, programVersion)

	// Step 1: brew update
	if err := runCommand("brew update", Green); err != nil {
		fmt.Fprintf(os.Stderr, "%sError during brew update: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}

	// Step 2: brew upgrade (formulae)
	if err := runCommand("brew upgrade", Green); err != nil {
		fmt.Fprintf(os.Stderr, "%sError during brew upgrade: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}

	// Step 3: upgrade casks
	if err := upgradeCasks(); err != nil {
		fmt.Fprintf(os.Stderr, "%sError during cask upgrade: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}

	// Step 4: brew cleanup -s
	if err := runCommand("brew cleanup -s", Green); err != nil {
		fmt.Fprintf(os.Stderr, "%sError during brew cleanup -s: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}

	fmt.Printf("\n%s✓ All updates completed successfully%s\n", Green, Reset)
}
