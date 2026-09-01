package main

import (
	"bytes"
	"fmt"
	"io"
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
	programVersion = "2.1.0"
)

const (
	// formatCompat prefers H.264 video with AAC audio so the resulting mp4
	// plays in strict players (QuickTime, older devices) without transcoding.
	formatCompat = "bv*[vcodec^=avc1]+ba[ext=m4a]/b[ext=mp4]/b"
	// formatBest prefers the highest-quality mp4-container video (often AV1)
	// remuxed without transcoding; some older players cannot decode it.
	formatBest = "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4]/b"
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

// ffmpegPrefixes marks yt-dlp output lines emitted while ffmpeg is running.
var ffmpegPrefixes = []string{"[Merger]", "[VideoConvertor]", "[VideoRemuxer]"}

// colorizeFFmpegLine wraps an ffmpeg post-processing line in yellow, keeping
// the trailing line delimiter outside the color escape.
func colorizeFFmpegLine(line []byte) []byte {
	ffmpeg := false
	for _, prefix := range ffmpegPrefixes {
		if bytes.HasPrefix(line, []byte(prefix)) {
			ffmpeg = true
			break
		}
	}
	if !ffmpeg {
		return line
	}
	body, delim := line, []byte(nil)
	if n := len(line); n > 0 && (line[n-1] == '\n' || line[n-1] == '\r') {
		body, delim = line[:n-1], line[n-1:]
	}
	out := make([]byte, 0, len(line)+len(Yellow)+len(Reset))
	out = append(out, Yellow...)
	out = append(out, body...)
	out = append(out, Reset...)
	out = append(out, delim...)
	return out
}

// ffmpegHighlighter passes yt-dlp output through to w, coloring ffmpeg
// post-processing lines yellow so the merge/convert phase is visible.
type ffmpegHighlighter struct {
	w   io.Writer
	buf bytes.Buffer
}

func (h *ffmpegHighlighter) Write(p []byte) (int, error) {
	h.buf.Write(p)
	for {
		b := h.buf.Bytes()
		i := bytes.IndexAny(b, "\r\n")
		if i < 0 {
			return len(p), nil
		}
		if _, err := h.w.Write(colorizeFFmpegLine(b[:i+1])); err != nil {
			return len(p), err
		}
		h.buf.Next(i + 1)
	}
}

// Flush writes any buffered partial line.
func (h *ffmpegHighlighter) Flush() error {
	if h.buf.Len() == 0 {
		return nil
	}
	_, err := h.w.Write(colorizeFFmpegLine(h.buf.Bytes()))
	h.buf.Reset()
	return err
}

// ytDlpArgs builds the yt-dlp invocation for a download. The compat selector
// avoids the full re-encode that --recode-video alone forces on non-mp4
// merges; --recode-video stays as a fallback for the rare non-mp4 result.
func ytDlpArgs(filename, url string, best bool) []string {
	format := formatCompat
	if best {
		format = formatBest
	}
	return []string{"-f", format, "-o", filename, "--recode-video", "mp4", url}
}

// downloadVideo downloads a video as mp4 using yt-dlp
func downloadVideo(filename, url string, best bool) error {
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
	cmd := exec.Command("yt-dlp", ytDlpArgs(filename, url, best)...)
	highlighter := &ffmpegHighlighter{w: os.Stdout}
	cmd.Stdout = highlighter
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if flushErr := highlighter.Flush(); flushErr != nil && err == nil {
		err = flushErr
	}
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("%s✓ Download completed successfully%s\n", Green, Reset)
	return nil
}

// printUsage displays usage information
func printUsage() {
	fmt.Printf("Usage: %s [OPTIONS] FILENAME \"URL\"\n\n", programName)
	fmt.Println("Options:")
	fmt.Println("  -b, --best       Prefer highest-quality mp4 streams (often AV1); may not play in older players")
	fmt.Println("  -u, --update     Upgrade yt-dlp to nightly version")
	fmt.Printf("  -v, --version    Print %s v%s and exit\n", programName, programVersion)
	fmt.Println("\nExamples:")
	fmt.Printf("  %s myvideo \"https://youtube.com/watch?v=...\"\n", programName)
	fmt.Printf("  %s -b myvideo \"https://youtube.com/watch?v=...\"\n", programName)
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

	// Optional quality flag, then required arguments
	args := os.Args[1:]
	best := false
	if len(args) > 0 && (args[0] == "-b" || args[0] == "--best") {
		best = true
		args = args[1:]
	}
	if len(args) != 2 {
		printUsage()
		os.Exit(1)
	}

	filename := args[0]
	url := args[1]

	// Download the video
	if err := downloadVideo(filename, url, best); err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}
}
