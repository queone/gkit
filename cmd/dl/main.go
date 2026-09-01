package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	programVersion = "2.3.0"
)

const (
	// formatBest prefers the highest-quality mp4-container video (often AV1)
	// remuxed without transcoding; some older players cannot decode it.
	formatBest = "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4]/b"

	// Default size caps: 360p at 16:9 for small, shareable files.
	defaultHeightCap = 360
	defaultWidthCap  = 640
)

// dlOptions holds the parsed command-line download options.
type dlOptions struct {
	filename string
	url      string
	best     bool
	capped   bool // false only for -b without -q/-w
	height   int
	width    int
}

// parseDownloadArgs parses flags and positional arguments for a download.
// A missing dimension pairs with the given one at 16:9, rounded up, so the
// caps never exclude the requested rung.
func parseDownloadArgs(args []string) (dlOptions, error) {
	var o dlOptions
	h, w := 0, 0
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "-b", "--best":
			o.best = true
			args = args[1:]
		case "-q", "--quality", "-w", "--width":
			flag := args[0]
			if len(args) < 2 {
				return o, fmt.Errorf("%s requires a value", flag)
			}
			n, err := strconv.Atoi(args[1])
			if err != nil || n <= 0 {
				return o, fmt.Errorf("%s requires a positive integer, got %q", flag, args[1])
			}
			if flag == "-q" || flag == "--quality" {
				h = n
			} else {
				w = n
			}
			args = args[2:]
		default:
			return o, fmt.Errorf("unknown option %s", args[0])
		}
	}
	if len(args) != 2 {
		return o, fmt.Errorf("expected FILENAME and URL arguments")
	}
	o.filename, o.url = args[0], args[1]
	o.capped = !o.best || h > 0 || w > 0
	switch {
	case h > 0 && w == 0:
		w = (h*16 + 8) / 9
	case w > 0 && h == 0:
		h = (w*9 + 15) / 16
	case h == 0 && w == 0:
		h, w = defaultHeightCap, defaultWidthCap
	}
	o.height, o.width = h, w
	return o, nil
}

// buildFormat returns the yt-dlp format selector for the parsed options.
// Capped modes filter to the height/width caps with a worst-format floor;
// -b without caps stays unfiltered.
func buildFormat(o dlOptions) string {
	if o.best && !o.capped {
		return formatBest
	}
	video := "bv*[vcodec^=avc1]"
	if o.best {
		video = "bv*[ext=mp4]"
	}
	f := fmt.Sprintf("[height<=%d][width<=%d]", o.height, o.width)
	return video + f + "+ba[ext=m4a]/b[ext=mp4]" + f + "/b" + f + "/w"
}

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

// colorizeFFmpegLine wraps an ffmpeg post-processing line in yellow with an
// explicit (ffmpeg) marker after the phase tag, keeping the trailing line
// delimiter outside the color escape.
func colorizeFFmpegLine(line []byte) []byte {
	tag := ""
	for _, prefix := range ffmpegPrefixes {
		if bytes.HasPrefix(line, []byte(prefix)) {
			tag = prefix
			break
		}
	}
	if tag == "" {
		return line
	}
	body, delim := line, []byte(nil)
	if n := len(line); n > 0 && (line[n-1] == '\n' || line[n-1] == '\r') {
		body, delim = line[:n-1], line[n-1:]
	}
	const marker = " (ffmpeg)"
	out := make([]byte, 0, len(line)+len(Yellow)+len(marker)+len(Reset))
	out = append(out, Yellow...)
	out = append(out, tag...)
	out = append(out, marker...)
	out = append(out, body[len(tag):]...)
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

// ytDlpArgs builds the yt-dlp invocation for a download. The selector
// avoids the full re-encode that --recode-video alone forces on non-mp4
// merges; --recode-video stays as a fallback for the rare non-mp4 result.
func ytDlpArgs(o dlOptions) []string {
	return []string{"-f", buildFormat(o), "-o", o.filename, "--recode-video", "mp4", o.url}
}

// denoHint returns a yellow hint to install deno when yt-dlp has no
// JavaScript runtime available, or an empty string when deno is installed.
// yt-dlp needs a JS runtime for reliable YouTube extraction; without one it
// falls back to deprecated clients with missing formats and more bot checks.
func denoHint(lookPath func(string) (string, error)) string {
	if _, err := lookPath("deno"); err == nil {
		return ""
	}
	return fmt.Sprintf("%sHint: yt-dlp extracts YouTube best with a JavaScript runtime; run: brew install deno%s\n", Yellow, Reset)
}

// downloadVideo downloads a video as mp4 using yt-dlp
func downloadVideo(o dlOptions) error {
	// Get file extension in lowercase
	ext := strings.ToLower(filepath.Ext(o.filename))

	// Add .mp4 extension if missing
	if ext != ".mp4" {
		o.filename = o.filename + ".mp4"
	}

	// Check if file already exists
	if _, err := os.Stat(o.filename); err == nil {
		fmt.Printf("%sFile already exists: %s%s\n", Red, o.filename, Reset)
		return fmt.Errorf("file already exists")
	}

	// Download as MP4
	fmt.Printf("==> %sDownloading to: %s%s\n", Green, o.filename, Reset)
	fmt.Print(denoHint(exec.LookPath))
	cmd := exec.Command("yt-dlp", ytDlpArgs(o)...)
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
	fmt.Println("Downloads default to a small 360p (max 640x360) mp4 for quick sharing.")
	fmt.Println("\nOptions:")
	fmt.Println("  -q, --quality N  Cap video height at N pixels (default 360; width pairs at 16:9)")
	fmt.Println("  -w, --width N    Cap video width at N pixels (default 640; height pairs at 16:9)")
	fmt.Println("  -b, --best       Uncapped best-quality mp4 (often AV1); may not play in older players")
	fmt.Println("  -u, --update     Upgrade yt-dlp to nightly version")
	fmt.Printf("  -v, --version    Print %s v%s and exit\n", programName, programVersion)
	fmt.Println("\nExamples:")
	fmt.Printf("  %s myvideo \"https://youtube.com/watch?v=...\"\n", programName)
	fmt.Printf("  %s -q 480 myvideo \"https://youtube.com/watch?v=...\"\n", programName)
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

	// Optional size/quality flags, then required arguments
	opts, err := parseDownloadArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n\n", Red, err, Reset)
		printUsage()
		os.Exit(1)
	}

	// Download the video
	if err := downloadVideo(opts); err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}
}
