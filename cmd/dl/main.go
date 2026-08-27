package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	programName    = "dl"
	programVersion = "2.0.0"
)

// checkYtDlpInstalled checks if yt-dlp is installed and accessible
func checkYtDlpInstalled() error {
	_, err := exec.LookPath("yt-dlp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: yt-dlp is not installed or not in PATH%s\n\n", Red, Reset)
		fmt.Println("To install yt-dlp, run:")
		fmt.Println("  curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp \\")
		fmt.Println("    -o /usr/local/bin/yt-dlp && chmod a+rx /usr/local/bin/yt-dlp")
		fmt.Println("\nOr install via package manager:")
		fmt.Println("  brew install yt-dlp        # macOS")
		fmt.Println("  pip install yt-dlp         # pip")
		return fmt.Errorf("yt-dlp not found")
	}
	return nil
}

// getYtDlpVersion returns the version of yt-dlp
func getYtDlpVersion() (string, error) {
	cmd := exec.Command("yt-dlp", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get yt-dlp version: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// upgradeYtDlp upgrades yt-dlp to the nightly version
func upgradeYtDlp() error {
	fmt.Printf("==> %sUpgrading yt-dlp to nightly%s\n", Green, Reset)
	cmd := exec.Command("yt-dlp", "--update-to", "nightly")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// showVersion displays the exact dl version line.
func showVersion() {
	fmt.Printf("%s v%s\n", programName, programVersion)
}

// showYtDlpVersion displays the installed yt-dlp version after an upgrade.
func showYtDlpVersion() error {
	ytdlpVersion, err := getYtDlpVersion()
	if err != nil {
		return err
	}
	fmt.Printf("yt-dlp %s\n", ytdlpVersion)

	return nil
}

// downloadVideo downloads a video using yt-dlp
func downloadVideo(filename, url string) error {
	// Get file extension in lowercase
	ext := strings.ToLower(filepath.Ext(filename))

	// Add .mp4 extension if missing
	if ext != ".mp4" {
		filename = filename + ".mp4"
	}

	// Check if file already exists
	if _, err := os.Stat(filename); err == nil {
		fmt.Printf("%sFile already exists: %s%s\n", Red, filename, Reset)
		return fmt.Errorf("file already exists")
	}

	// Download as MP4
	fmt.Printf("==> %sDownloading to: %s%s\n", Green, filename, Reset)
	cmd := exec.Command("yt-dlp", "-o", filename, "--recode-video", "mp4", url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("%s✓ Download completed successfully%s\n", Green, Reset)
	return nil
}

// printUsage displays usage information
func printUsage() {
	fmt.Printf("Usage: %s [OPTIONS] FILENAME \"URL\"\n\n", programName)
	fmt.Println("Options:")
	fmt.Println("  -u    Upgrade yt-dlp to nightly version")
	fmt.Printf("  -v, --version    Print %s v%s and exit\n", programName, programVersion)
	fmt.Println("\nExamples:")
	fmt.Printf("  %s myvideo \"https://youtube.com/watch?v=...\"\n", programName)
	fmt.Printf("  %s -u\n", programName)
	fmt.Printf("  %s -v\n", programName)
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		showVersion()
		return
	}

	// Check if yt-dlp is installed before doing anything
	if err := checkYtDlpInstalled(); err != nil {
		os.Exit(1)
	}

	// Handle flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-u", "--update":
			if err := upgradeYtDlp(); err != nil {
				fmt.Fprintf(os.Stderr, "%sError: %v%s\n", Red, err, Reset)
				os.Exit(1)
			}
			// Show new version after upgrade
			showVersion()
			if err := showYtDlpVersion(); err != nil {
				fmt.Fprintf(os.Stderr, "%sError: %v%s\n", Red, err, Reset)
				os.Exit(1)
			}
			return

		case "-h", "--help":
			printUsage()
			return
		}
	}

	// Check for required arguments
	if len(os.Args) != 3 {
		printUsage()
		os.Exit(1)
	}

	filename := os.Args[1]
	url := os.Args[2]

	// Download the video
	if err := downloadVideo(filename, url); err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}
}
