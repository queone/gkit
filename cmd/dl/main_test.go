package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDenoHintShownOnlyWhenDenoMissing(t *testing.T) {
	missing := func(string) (string, error) { return "", exec.ErrNotFound }
	found := func(string) (string, error) { return "/opt/homebrew/bin/deno", nil }

	hint := denoHint(missing)
	if !strings.Contains(hint, "brew install deno") {
		t.Errorf("missing-deno hint = %q, want it to name 'brew install deno'", hint)
	}
	if !strings.HasPrefix(hint, Yellow) || !strings.Contains(hint, Reset) {
		t.Errorf("missing-deno hint = %q, want yellow-wrapped", hint)
	}
	if hint := denoHint(found); hint != "" {
		t.Errorf("installed-deno hint = %q, want empty", hint)
	}
}

func mustParse(t *testing.T, args ...string) dlOptions {
	t.Helper()
	o, err := parseDownloadArgs(append(args, "out.mp4", "https://example.com/v"))
	if err != nil {
		t.Fatalf("parse %q: %v", args, err)
	}
	return o
}

// The default selector tries the 360p rung in landscape, then the same rung
// in portrait, then falls back to the smallest split or muxed format.
func TestDefaultDownloadCapsAt360pInEitherOrientation(t *testing.T) {
	got := ytDlpArgs(mustParse(t))
	want := []string{
		"-f", "bv*[vcodec^=avc1][height<=360][width<=640]+ba[ext=m4a]/b[ext=mp4][height<=360][width<=640]/b[height<=360][width<=640]" +
			"/bv*[vcodec^=avc1][height<=640][width<=360]+ba[ext=m4a]/b[ext=mp4][height<=640][width<=360]/b[height<=640][width<=360]" +
			"/wv+wa/w",
		"-o", "out.mp4", "--recode-video", "mp4", "--color", "stderr:always", "https://example.com/v",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("default args = %q, want %q", got, want)
	}
}

func TestCookiesFlagPassesBrowserToYtDlp(t *testing.T) {
	for _, flag := range []string{"-c", "--cookies"} {
		got := ytDlpArgs(mustParse(t, flag, "safari"))
		tail := []string{"--cookies-from-browser", "safari", "https://example.com/v"}
		if len(got) < len(tail) || !reflect.DeepEqual(got[len(got)-len(tail):], tail) {
			t.Errorf("%s safari args = %q, want them to end with %q", flag, got, tail)
		}
	}
	if got := ytDlpArgs(mustParse(t)); strings.Contains(strings.Join(got, " "), loginMarker) {
		t.Errorf("args without -c = %q, want no browser cookies", got)
	}
}

func TestCookiesFileParsesHostBrowserLines(t *testing.T) {
	in := "# sites that need a login\n\nreddit.com safari\n  .Example.ORG  firefox \n"
	got, err := parseCookieSites(strings.NewReader(in), "cookies")
	if err != nil {
		t.Fatal(err)
	}
	want := []cookieSite{{host: "reddit.com", browser: "safari"}, {host: "example.org", browser: "firefox"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsed sites = %+v, want %+v", got, want)
	}
	for _, bad := range []string{"reddit.com", "reddit.com safari extra"} {
		_, err := parseCookieSites(strings.NewReader(bad+"\n"), "cookies")
		if err == nil || !strings.Contains(err.Error(), "cookies line 1") {
			t.Errorf("parse %q error = %v, want one naming cookies line 1", bad, err)
		}
	}
}

func TestSiteBrowserMatchesHostAndSubdomains(t *testing.T) {
	sites := []cookieSite{{host: "reddit.com", browser: "safari"}, {host: "example.org", browser: "firefox"}}
	cases := map[string]string{
		"https://www.reddit.com/r/videos/comments/abc": "safari",
		"https://old.REDDIT.com/r/x":                   "safari",
		"reddit.com/r/x":                               "safari",
		"https://example.org/v":                        "firefox",
		"https://notreddit.com/r/x":                    "",
		"https://reddit.com.evil.net/x":                "",
		"https://youtube.com/watch?v=1":                "",
	}
	for rawURL, want := range cases {
		if got := siteBrowser(rawURL, sites); got != want {
			t.Errorf("siteBrowser(%q) = %q, want %q", rawURL, got, want)
		}
	}
	if got := siteBrowser("https://www.reddit.com/r/x", nil); got != "" {
		t.Errorf("siteBrowser with no sites = %q, want empty", got)
	}
}

func TestLoadCookieSitesReadsXDGFileOrNothing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	sites, err := loadCookieSites()
	if err != nil || sites != nil {
		t.Errorf("missing file: sites = %v, err = %v, want nil, nil", sites, err)
	}
	path := filepath.Join(dir, "dl", "cookies")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("reddit.com safari\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, err = loadCookieSites()
	if err != nil {
		t.Fatal(err)
	}
	if got := siteBrowser("https://www.reddit.com/r/x", sites); got != "safari" {
		t.Errorf("browser from %s = %q, want safari", path, got)
	}
	if err := os.WriteFile(path, []byte("reddit.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCookieSites(); err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("malformed file error = %v, want one naming %s", err, path)
	}
}

func TestOutputPathPlacesOnlyBareNamesInDownloadDir(t *testing.T) {
	dir := filepath.Join("/home", "u", "Downloads")
	cases := []struct{ name, dir, want string }{
		{"v", dir, filepath.Join(dir, "v")},
		{"v.mp4", dir, filepath.Join(dir, "v.mp4")},
		{"./v", dir, "./v"},
		{"sub/v", dir, "sub/v"},
		{"/tmp/v", dir, "/tmp/v"},
		{"v", "", "v"},
	}
	for _, c := range cases {
		if got := outputPath(c.name, c.dir); got != c.want {
			t.Errorf("outputPath(%q, %q) = %q, want %q", c.name, c.dir, got, c.want)
		}
	}
}

func TestDownloadDirPrefersXDGThenHomeDownloads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DOWNLOAD_DIR", "")
	if got := downloadDir(); got != "" {
		t.Errorf("missing ~/Downloads = %q, want empty", got)
	}

	downloads := filepath.Join(home, "Downloads")
	if err := os.Mkdir(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := downloadDir(); got != downloads {
		t.Errorf("default = %q, want %q", got, downloads)
	}

	t.Setenv("XDG_DOWNLOAD_DIR", "relative/dir")
	if got := downloadDir(); got != downloads {
		t.Errorf("relative XDG_DOWNLOAD_DIR = %q, want fallback %q", got, downloads)
	}

	xdg := t.TempDir()
	t.Setenv("XDG_DOWNLOAD_DIR", xdg)
	if got := downloadDir(); got != xdg {
		t.Errorf("absolute XDG_DOWNLOAD_DIR = %q, want %q", got, xdg)
	}
}

func TestLoginWatcherFindsCookiesRequestSplitAcrossWrites(t *testing.T) {
	var out bytes.Buffer
	l := &loginWatcher{w: &out}
	chunks := []string{
		"ERROR: [Reddit] 1x58kxn: Account authentication is required. Use --cookies-fr",
		"om-browser or --cookies for the authentication.\n",
	}
	for _, c := range chunks {
		if n, err := l.Write([]byte(c)); err != nil || n != len(c) {
			t.Fatalf("write %q = %d, %v", c, n, err)
		}
	}
	if !l.asked {
		t.Error("split marker not found")
	}
	if got, want := out.String(), strings.Join(chunks, ""); got != want {
		t.Errorf("passed through %q, want %q", got, want)
	}

	quiet := &loginWatcher{w: &bytes.Buffer{}}
	quiet.Write([]byte("ERROR: Unsupported URL: https://example.com/v\n"))
	if quiet.asked {
		t.Error("unrelated error flagged as a login request")
	}
}

func TestLoginHintShownOnlyWhenLoginNeededWithoutCookies(t *testing.T) {
	hint := loginHint(true, "")
	if !strings.Contains(hint, "-c BROWSER") {
		t.Errorf("login hint = %q, want it to name -c BROWSER", hint)
	}
	if !strings.HasPrefix(hint, Yellow) || !strings.Contains(hint, Reset) {
		t.Errorf("login hint = %q, want yellow-wrapped", hint)
	}
	if hint := loginHint(true, "safari"); hint != "" {
		t.Errorf("hint with -c safari = %q, want empty", hint)
	}
	if hint := loginHint(false, ""); hint != "" {
		t.Errorf("hint without a login request = %q, want empty", hint)
	}
}

func TestSafariHintShownOnlyWhenCookieFileBlocked(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens files regardless of permissions")
	}
	home := t.TempDir()
	if hint := safariHint("safari", home); hint != "" {
		t.Errorf("missing cookie file hint = %q, want empty so yt-dlp reports it", hint)
	}

	writeCookieFile := func(rel string, perm os.FileMode) string {
		t.Helper()
		path := filepath.Join(home, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, perm); err != nil {
			t.Fatal(err)
		}
		return path
	}
	container := writeCookieFile(safariCookieFiles[1], 0o000)
	for _, browser := range []string{"safari", "Safari"} {
		hint := safariHint(browser, home)
		if !strings.Contains(hint, "Full Disk Access") {
			t.Errorf("%s blocked hint = %q, want it to name Full Disk Access", browser, hint)
		}
		if !strings.HasPrefix(hint, Yellow) || !strings.Contains(hint, Reset) {
			t.Errorf("%s blocked hint = %q, want yellow-wrapped", browser, hint)
		}
	}
	if hint := safariHint("firefox", home); hint != "" {
		t.Errorf("firefox hint = %q, want empty", hint)
	}

	// yt-dlp reads the first cookie file it finds, so a readable one wins.
	legacy := writeCookieFile(safariCookieFiles[0], 0o600)
	if hint := safariHint("safari", home); hint != "" {
		t.Errorf("readable first cookie file hint = %q, want empty", hint)
	}
	if err := os.Remove(legacy); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(container, 0o600); err != nil {
		t.Fatal(err)
	}
	if hint := safariHint("safari", home); hint != "" {
		t.Errorf("readable cookie file hint = %q, want empty", hint)
	}
}

// Equal caps describe a square, so the swapped orientation adds nothing.
func TestSquareCapsEmitOneOrientation(t *testing.T) {
	want := "bv*[vcodec^=avc1][height<=480][width<=480]+ba[ext=m4a]/b[ext=mp4][height<=480][width<=480]/b[height<=480][width<=480]/wv+wa/w"
	if got := buildFormat(mustParse(t, "-q", "480", "-w", "480")); got != want {
		t.Errorf("-q 480 -w 480 format = %q, want %q", got, want)
	}
}

func TestSingleDimensionDerivesOtherCapAt16x9(t *testing.T) {
	cases := []struct {
		args []string
		h, w int
	}{
		{[]string{"-q", "480"}, 480, 854},
		{[]string{"-q", "720"}, 720, 1280},
		{[]string{"-w", "640"}, 360, 640},
		{[]string{"--quality", "1080"}, 1080, 1920},
		{[]string{"-q", "360", "-w", "800"}, 360, 800},
	}
	for _, c := range cases {
		o := mustParse(t, c.args...)
		if o.height != c.h || o.width != c.w {
			t.Errorf("%q caps = %dx%d, want %dx%d", c.args, o.height, o.width, c.h, c.w)
		}
	}
}

func TestBestModeUncappedUnlessSized(t *testing.T) {
	if got := buildFormat(mustParse(t, "-b")); got != formatBest {
		t.Errorf("-b format = %q, want %q", got, formatBest)
	}
	want := "bv*[ext=mp4][height<=480][width<=854]+ba[ext=m4a]/b[ext=mp4][height<=480][width<=854]/b[height<=480][width<=854]" +
		"/bv*[ext=mp4][height<=854][width<=480]+ba[ext=m4a]/b[ext=mp4][height<=854][width<=480]/b[height<=854][width<=480]" +
		"/wv+wa/w"
	if got := buildFormat(mustParse(t, "-b", "-q", "480")); got != want {
		t.Errorf("-b -q 480 format = %q, want %q", got, want)
	}
}

func TestParseDownloadArgsOrderingAndErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-q", "480", "-b"},
		{"-b", "-q", "480"},
		{"-w", "640", "-q", "360"},
	} {
		o := mustParse(t, args...)
		if o.filename != "out.mp4" || o.url != "https://example.com/v" {
			t.Errorf("%q positionals = %q %q", args, o.filename, o.url)
		}
	}
	trailing, err := parseDownloadArgs([]string{"out.mp4", "https://example.com/v", "-c", "safari"})
	if err != nil || trailing.filename != "out.mp4" || trailing.url != "https://example.com/v" || trailing.browser != "safari" {
		t.Errorf("trailing -c safari = %+v, %v", trailing, err)
	}
	between, err := parseDownloadArgs([]string{"out.mp4", "-q", "480", "https://example.com/v"})
	if err != nil || between.filename != "out.mp4" || between.url != "https://example.com/v" || between.height != 480 {
		t.Errorf("-q 480 between positionals = %+v, %v", between, err)
	}
	for _, args := range [][]string{
		{"-q", "out.mp4", "https://example.com/v"},
		{"-q", "0", "out.mp4", "https://example.com/v"},
		{"-q", "-5", "out.mp4", "https://example.com/v"},
		{"-w", "abc", "out.mp4", "https://example.com/v"},
		{"-q"},
		{"-x", "out.mp4", "https://example.com/v"},
		{"-c"},
		{"-c", "", "out.mp4", "https://example.com/v"},
		{"-c", "-q", "480", "out.mp4", "https://example.com/v"},
		{"out.mp4"},
		{"out.mp4", "https://example.com/v", "-c"},
		{"out.mp4", "https://example.com/v", "-q"},
		{"out.mp4", "https://example.com/v", "extra"},
	} {
		if _, err := parseDownloadArgs(args); err == nil {
			t.Errorf("parse %q: expected error, got none", args)
		}
	}
}

func TestFFmpegHighlighterColorsPostProcessingLines(t *testing.T) {
	var out bytes.Buffer
	h := &ffmpegHighlighter{w: &out}

	// Chunks split mid-line and mixing \r progress updates with \n lines.
	chunks := []string{
		"[download]  50.0% of 1GiB\r",
		"[download] 100% of 1GiB\n[Mer",
		"ger] Merging formats into \"v.mp4\"\n",
		"[VideoConvertor] Converting video from webm to mp4\n",
		"Deleting original file v.f401.mp4",
	}
	for _, c := range chunks {
		n, err := h.Write([]byte(c))
		if err != nil {
			t.Fatalf("write %q: %v", c, err)
		}
		if n != len(c) {
			t.Fatalf("write %q returned %d, want %d", c, n, len(c))
		}
	}
	if err := h.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	want := "[download]  50.0% of 1GiB\r" +
		"[download] 100% of 1GiB\n" +
		Yellow + "[Merger] (ffmpeg) Merging formats into \"v.mp4\"" + Reset + "\n" +
		Yellow + "[VideoConvertor] (ffmpeg) Converting video from webm to mp4" + Reset + "\n" +
		"Deleting original file v.f401.mp4"
	if got := out.String(); got != want {
		t.Errorf("highlighted output = %q, want %q", got, want)
	}
}

func TestFFmpegHighlighterFlushOnEmptyBuffer(t *testing.T) {
	var out bytes.Buffer
	h := &ffmpegHighlighter{w: &out}
	if err := h.Flush(); err != nil {
		t.Fatalf("flush empty: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("flush empty wrote %q, want nothing", out.String())
	}
}

func TestVersionAliases(t *testing.T) {
	const envName = "GKIT_DL_VERSION_FLAG"
	if flag := os.Getenv(envName); flag != "" {
		os.Args = []string{programName, flag}
		main()
		os.Exit(0)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			cmd := exec.Command(exe, "-test.run=^TestVersionAliases$")
			cmd.Env = append(os.Environ(), "PATH=", envName+"="+flag)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("run %s without yt-dlp on PATH: %v", flag, err)
			}
			if got, want := stdout.String(), programName+" v"+programVersion+"\n"; got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

// The help screen follows the shared standard: three header lines, then the
// sections the renderer accepts, with the standard rows left to the renderer.
func TestHelpDocFollowsTheStandard(t *testing.T) {
	if err := helpDoc().Check(); err != nil {
		t.Fatal(err)
	}
}
