package main

import (
	"bytes"
	"os"
	"os/exec"
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

func TestDefaultDownloadCapsAt360p(t *testing.T) {
	got := ytDlpArgs(mustParse(t))
	want := []string{
		"-f", "bv*[vcodec^=avc1][height<=360][width<=640]+ba[ext=m4a]/b[ext=mp4][height<=360][width<=640]/b[height<=360][width<=640]/w",
		"-o", "out.mp4", "--recode-video", "mp4", "https://example.com/v",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("default args = %q, want %q", got, want)
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
	want := "bv*[ext=mp4][height<=480][width<=854]+ba[ext=m4a]/b[ext=mp4][height<=480][width<=854]/b[height<=480][width<=854]/w"
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
	for _, args := range [][]string{
		{"-q", "out.mp4", "https://example.com/v"},
		{"-q", "0", "out.mp4", "https://example.com/v"},
		{"-q", "-5", "out.mp4", "https://example.com/v"},
		{"-w", "abc", "out.mp4", "https://example.com/v"},
		{"-q"},
		{"-x", "out.mp4", "https://example.com/v"},
		{"out.mp4"},
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
