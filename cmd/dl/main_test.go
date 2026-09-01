package main

import (
	"bytes"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func TestYtDlpArgsSelectsCompatOrBestFormat(t *testing.T) {
	got := ytDlpArgs("out.mp4", "https://example.com/v", false)
	want := []string{"-f", formatCompat, "-o", "out.mp4", "--recode-video", "mp4", "https://example.com/v"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("default args = %q, want %q", got, want)
	}

	got = ytDlpArgs("out.mp4", "https://example.com/v", true)
	want = []string{"-f", formatBest, "-o", "out.mp4", "--recode-video", "mp4", "https://example.com/v"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("best args = %q, want %q", got, want)
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
		Yellow + "[Merger] Merging formats into \"v.mp4\"" + Reset + "\n" +
		Yellow + "[VideoConvertor] Converting video from webm to mp4" + Reset + "\n" +
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
