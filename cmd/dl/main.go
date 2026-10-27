package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/queone/gkit/help"
	"io"
	"io/fs"
	"net/url"
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
	programVersion = "2.7.0"
)

const (
	// formatBest prefers the highest-quality mp4-container video (often AV1)
	// remuxed without transcoding; some older players cannot decode it.
	formatBest = "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4]/b"

	// formatFloor takes the smallest video plus audio when nothing fits the
	// caps. Split-stream sites such as Reddit have no muxed format, so "w"
	// alone matches nothing there; it stays last for muxed-only sites.
	formatFloor = "wv+wa/w"

	// Default size caps: 360p at 16:9 for small, shareable files.
	defaultHeightCap = 360
	defaultWidthCap  = 640

	// loginMarker appears in every yt-dlp error that asks for browser
	// cookies, such as Reddit's "Account authentication is required".
	loginMarker = "--cookies-from-browser"
)

// dlOptions holds the parsed command-line download options.
type dlOptions struct {
	filename string
	url      string
	browser  string // yt-dlp --cookies-from-browser value; "" for none
	best     bool
	capped   bool // false only for -b without -q/-w
	height   int
	width    int
}

// parseDownloadArgs parses flags and positional arguments for a download.
// Options may come before, between, or after FILENAME and URL. A missing
// dimension pairs with the given one at 16:9, rounded up, so the caps never
// exclude the requested rung.
func parseDownloadArgs(args []string) (dlOptions, error) {
	var o dlOptions
	var pos []string
	h, w := 0, 0
	for len(args) > 0 {
		if !strings.HasPrefix(args[0], "-") {
			pos = append(pos, args[0])
			args = args[1:]
			continue
		}
		switch args[0] {
		case "-b", "--best":
			o.best = true
			args = args[1:]
		case "-c", "--cookies":
			if len(args) < 2 || args[1] == "" || strings.HasPrefix(args[1], "-") {
				return o, fmt.Errorf("%s requires a browser name, such as safari", args[0])
			}
			o.browser = args[1]
			args = args[2:]
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
	if len(pos) != 2 {
		return o, fmt.Errorf("expected FILENAME and URL arguments")
	}
	o.filename, o.url = pos[0], pos[1]
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
// Capped modes filter to the height/width caps, then to the same caps with
// the sides swapped so a portrait video gets the same rung as a landscape
// one, then to the worst-format floor; -b without caps stays unfiltered.
func buildFormat(o dlOptions) string {
	if o.best && !o.capped {
		return formatBest
	}
	video := "bv*[vcodec^=avc1]"
	if o.best {
		video = "bv*[ext=mp4]"
	}
	sel := cappedBranches(video, o.height, o.width)
	if o.height != o.width {
		sel += "/" + cappedBranches(video, o.width, o.height)
	}
	return sel + "/" + formatFloor
}

// cappedBranches returns the capped branches for one orientation: the best
// video under the caps merged with m4a audio, then a muxed mp4 under the
// caps, then any muxed format under the caps.
func cappedBranches(video string, height, width int) string {
	f := fmt.Sprintf("[height<=%d][width<=%d]", height, width)
	return video + f + "+ba[ext=m4a]/b[ext=mp4]" + f + "/b" + f
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
// stderr is piped through a loginWatcher, so yt-dlp is told to keep coloring
// its ERROR and WARNING lines.
func ytDlpArgs(o dlOptions) []string {
	args := []string{"-f", buildFormat(o), "-o", o.filename, "--recode-video", "mp4", "--color", "stderr:always"}
	if o.browser != "" {
		args = append(args, "--cookies-from-browser", o.browser)
	}
	return append(args, o.url)
}

// downloadDir returns the folder for downloads saved under a bare name:
// $XDG_DOWNLOAD_DIR when it holds an absolute path, else Downloads in the
// home folder. It returns "" when that folder is missing.
func downloadDir() string {
	dir := os.Getenv("XDG_DOWNLOAD_DIR")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, "Downloads")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}

// outputPath places a bare name in dir. A name with a directory part, such
// as ./v, sub/v, or /tmp/v, stays as given; so does every name when dir is "".
func outputPath(name, dir string) string {
	if dir == "" || filepath.Base(name) != name {
		return name
	}
	return filepath.Join(dir, name)
}

// cookieSite maps a site to the browser whose login dl uses for it.
type cookieSite struct {
	host    string
	browser string
}

// configDir returns the user's XDG config directory, honoring XDG_CONFIG_HOME
// when set to an absolute path and falling back to $HOME/.config otherwise.
func configDir() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// cookiesPath returns the path of the per-site cookies file.
func cookiesPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, programName, "cookies"), nil
}

// parseCookieSites reads HOST BROWSER lines from r, naming path in errors.
// Blank lines and # comments are skipped; a leading dot on HOST is ignored.
func parseCookieSites(r io.Reader, path string) ([]cookieSite, error) {
	var sites []cookieSite
	scanner := bufio.NewScanner(r)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s line %d: expected HOST BROWSER, such as reddit.com safari, got %d field(s)", path, line, len(fields))
		}
		host := strings.ToLower(strings.TrimPrefix(fields[0], "."))
		sites = append(sites, cookieSite{host: host, browser: fields[1]})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return sites, nil
}

// loadCookieSites reads the per-site cookies file. A missing file means no
// sites; any other failure names the file so the user can fix it.
func loadCookieSites() ([]cookieSite, error) {
	path, err := cookiesPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()
	return parseCookieSites(f, path)
}

// siteBrowser returns the browser of the first site whose host is rawURL's
// host or a parent domain of it, or "" when none matches. A URL without a
// scheme is read as https, as yt-dlp does.
func siteBrowser(rawURL string, sites []cookieSite) string {
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	for _, s := range sites {
		if host == s.host || strings.HasSuffix(host, "."+s.host) {
			return s.browser
		}
	}
	return ""
}

// loginWatcher passes yt-dlp stderr through to w and records whether yt-dlp
// asked for browser cookies. tail keeps a marker-length prefix candidate so a
// marker split across writes is still found.
type loginWatcher struct {
	w     io.Writer
	tail  string
	asked bool
}

func (l *loginWatcher) Write(p []byte) (int, error) {
	if !l.asked {
		s := l.tail + string(p)
		l.asked = strings.Contains(s, loginMarker)
		l.tail = s[max(0, len(s)-len(loginMarker)+1):]
	}
	return l.w.Write(p)
}

// loginHint returns a yellow hint to run again with -c when yt-dlp asked for
// browser cookies and none were given, or an empty string otherwise.
func loginHint(asked bool, browser string) string {
	if !asked || browser != "" {
		return ""
	}
	return fmt.Sprintf("%sHint: this site needs a login; log in to it in your browser, then run again with -c BROWSER, such as -c safari%s\n", Yellow, Reset)
}

// safariCookieFiles lists Safari's cookie file locations under the home
// folder, in the order yt-dlp tries them.
var safariCookieFiles = []string{
	filepath.Join("Library", "Cookies", "Cookies.binarycookies"),
	filepath.Join("Library", "Containers", "com.apple.Safari", "Data", "Library", "Cookies", "Cookies.binarycookies"),
}

// safariCookiesBlocked reports whether the first Safari cookie file found
// under home cannot be opened. macOS blocks it unless the terminal app has
// Full Disk Access. A missing file is left for yt-dlp to report.
func safariCookiesBlocked(home string) bool {
	for _, rel := range safariCookieFiles {
		f, err := os.Open(filepath.Join(home, rel))
		if err == nil {
			f.Close()
			return false
		}
		if errors.Is(err, fs.ErrPermission) {
			return true
		}
	}
	return false
}

// safariHint returns a yellow hint to grant Full Disk Access when browser is
// Safari and macOS blocks its cookie file under home, or an empty string
// otherwise.
func safariHint(browser, home string) string {
	if !strings.EqualFold(browser, "safari") || !safariCookiesBlocked(home) {
		return ""
	}
	return fmt.Sprintf("%sHint: give your terminal app Full Disk Access in System Settings > Privacy & Security, reopen it, and run again; or log in to the site in Firefox and use -c firefox%s\n", Yellow, Reset)
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
	o.filename = outputPath(o.filename, downloadDir())

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

	// Stop before yt-dlp prints a raw permission error for Safari's cookies
	if home, err := os.UserHomeDir(); err == nil {
		if hint := safariHint(o.browser, home); hint != "" {
			fmt.Fprint(os.Stderr, hint)
			return fmt.Errorf("macOS blocked dl from reading Safari's cookies")
		}
	}

	// Download as MP4
	fmt.Printf("==> %sDownloading to: %s%s\n", Green, o.filename, Reset)
	fmt.Print(denoHint(exec.LookPath))
	cmd := exec.Command("yt-dlp", ytDlpArgs(o)...)
	highlighter := &ffmpegHighlighter{w: os.Stdout}
	watcher := &loginWatcher{w: os.Stderr}
	cmd.Stdout = highlighter
	cmd.Stderr = watcher

	err := cmd.Run()
	if flushErr := highlighter.Flush(); flushErr != nil && err == nil {
		err = flushErr
	}
	if err != nil {
		fmt.Fprint(os.Stderr, loginHint(watcher.asked, o.browser))
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("%s✓ Download completed successfully%s\n", Green, Reset)
	return nil
}

// printUsage displays usage information
func helpDoc() help.Doc {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Download an online video as an MP4 with yt-dlp",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + ` [options] FILENAME "URL"`, Meaning: "Save the video at URL as FILENAME.mp4"}},
				Lines: []string{
					"Downloads default to a small 360p (max 640x360, or 360x640 portrait) mp4 for quick sharing.",
					"A bare FILENAME saves to the Downloads folder; a FILENAME with a path, such as ./myvideo, saves there.",
					"Options may come before or after FILENAME and URL.",
				}},
			{Title: "Options", Rows: []help.Row{
				{Form: "-q, --quality N", Meaning: "Cap video height at N pixels, or width for portrait (default 360; other side pairs at 16:9)"},
				{Form: "-w, --width N", Meaning: "Cap video width at N pixels, or height for portrait (default 640; other side pairs at 16:9)"},
				{Form: "-b, --best", Meaning: "Uncapped best-quality mp4 (often AV1); may not play in older players"},
				{Form: "-c, --cookies BROWSER", Meaning: "Use your login from BROWSER, such as safari, chrome, or firefox, for sites that need one"},
				{Form: "-u, --update", Meaning: "Upgrade yt-dlp to the nightly version"},
			},
				Lines: []string{
					"Without -c, ~/.config/dl/cookies ($XDG_CONFIG_HOME/dl/cookies) picks the browser by site.",
					"One site per line as HOST BROWSER, such as: reddit.com safari",
					"A HOST matches itself and its subdomains, such as www.reddit.com.",
				}},
			{Title: "Examples", Rows: []help.Row{
				{Form: programName + ` myvideo "https://youtube.com/watch?v=..."`, Meaning: ""},
				{Form: programName + ` -q 480 myvideo "https://youtube.com/watch?v=..."`, Meaning: ""},
				{Form: programName + ` -b myvideo "https://youtube.com/watch?v=..."`, Meaning: ""},
				{Form: programName + ` ./myvideo "https://youtube.com/watch?v=..."`, Meaning: ""},
				{Form: programName + ` -c safari myvideo "https://www.reddit.com/r/..."`, Meaning: ""},
				{Form: programName + " -u", Meaning: ""},
			}},
		},
	}
}

func usage() string { return helpDoc().String() }

// printUsage prints the help.
func printUsage() {
	fmt.Print(usage())
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		showVersion()
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "-?" || os.Args[1] == "--help") {
		printUsage()
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

		case "-h", "-?", "--help":
			printUsage()
			return
		}
	}

	// Options and the required arguments, in any order
	opts, err := parseDownloadArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n\n", Red, err, Reset)
		printUsage()
		os.Exit(1)
	}

	// Without -c, let the per-site cookies file pick the browser
	if opts.browser == "" {
		sites, err := loadCookieSites()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%sError: %v%s\n", Red, err, Reset)
			os.Exit(1)
		}
		if opts.browser = siteBrowser(opts.url, sites); opts.browser != "" {
			fmt.Printf("==> %sUsing your %s login for this site, from the dl cookies file%s\n", Green, opts.browser, Reset)
		}
	}

	// Download the video
	if err := downloadVideo(opts); err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}
}
