package main

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/queone/gkit/internal/color"
)

type result struct {
	code   int
	stdout string
	stderr string
}

// runWith runs the utility with captured output.
func runWith(t *testing.T, args ...string) result {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := run(args, env{stdout: &out, stderr: &errBuf})
	return result{code: code, stdout: color.ClearCode(out.String()), stderr: errBuf.String()}
}

// stubTranscode records each ffmpeg argument list and creates the output file.
func stubTranscode(t *testing.T) *[][]string {
	t.Helper()
	restore := transcode
	t.Cleanup(func() { transcode = restore })
	var calls [][]string
	transcode = func(input, output string, argv []string) error {
		calls = append(calls, append([]string(nil), argv...))
		return os.WriteFile(output, []byte("mp4"), 0o644)
	}
	return &calls
}

// touch creates an empty file in the current directory.
func touch(t *testing.T, name string) {
	t.Helper()
	if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConvertBuildsFfmpegArgumentsForWebm(t *testing.T) {
	calls := stubTranscode(t)
	t.Chdir(t.TempDir())
	touch(t, "clip.webm")
	r := runWith(t, "clip.webm")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	want := []string{"-i", "clip.webm", "-c:v", "libx264", "-crf", "23", "-preset", "fast", "-c:a", "aac", "clip.mp4"}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Errorf("ffmpeg calls = %v, want exactly %v", *calls, want)
	}
	if !strings.Contains(r.stdout, "Converting clip.webm") {
		t.Errorf("stdout = %q, want the converting line", r.stdout)
	}
}

func TestConvertRefusesExistingOutput(t *testing.T) {
	calls := stubTranscode(t)
	t.Chdir(t.TempDir())
	touch(t, "clip.webm")
	touch(t, "clip.mp4")
	r := runWith(t, "clip.webm")
	if r.code != 1 || !strings.Contains(r.stderr, `"clip.mp4" already exists`) {
		t.Errorf("exit %d, stderr %q; want exit 1 naming clip.mp4", r.code, r.stderr)
	}
	if len(*calls) != 0 {
		t.Errorf("ffmpeg ran %d times, want 0", len(*calls))
	}
}

func TestConvertRejectsMissingInput(t *testing.T) {
	stubTranscode(t)
	t.Chdir(t.TempDir())
	r := runWith(t, "missing.webm")
	if r.code != 1 || !strings.Contains(r.stderr, "not a readable file") {
		t.Errorf("exit %d, stderr %q; want exit 1 not-readable", r.code, r.stderr)
	}
}

func TestConvertRejectsMp4Input(t *testing.T) {
	calls := stubTranscode(t)
	t.Chdir(t.TempDir())
	touch(t, "clip.mp4")
	r := runWith(t, "clip.mp4")
	if r.code != 1 || !strings.Contains(r.stderr, "already an .mp4") || len(*calls) != 0 {
		t.Errorf("exit %d, stderr %q, calls %d; want exit 1 and no ffmpeg run", r.code, r.stderr, len(*calls))
	}
}

func TestConvertReportsTranscodeFailure(t *testing.T) {
	restore := transcode
	t.Cleanup(func() { transcode = restore })
	transcode = func(string, string, []string) error { return os.ErrPermission }
	t.Chdir(t.TempDir())
	touch(t, "clip.webm")
	r := runWith(t, "clip.webm")
	if r.code != 1 || !strings.Contains(r.stderr, "vconv: ") {
		t.Errorf("exit %d, stderr %q; want exit 1 with a prefixed error", r.code, r.stderr)
	}
}

func TestVersionHelpAndBadFlags(t *testing.T) {
	if r := runWith(t, "-v"); r.code != 0 || r.stdout != "vconv v"+programVersion+"\n" {
		t.Errorf("-v: exit %d stdout %q", r.code, r.stdout)
	}
	if r := runWith(t, "--version"); r.stdout != "vconv v"+programVersion+"\n" {
		t.Errorf("--version: stdout %q", r.stdout)
	}
	for _, f := range []string{"-h", "--help", "-?"} {
		if r := runWith(t, f); r.code != 0 || !strings.Contains(r.stdout, "\nUsage\n") {
			t.Errorf("%s: exit %d stdout %q", f, r.code, r.stdout)
		}
	}
	if r := runWith(t); r.code != 0 || !strings.Contains(r.stdout, "\nUsage\n") {
		t.Errorf("no args: exit %d stdout %q", r.code, r.stdout)
	}
	if r := runWith(t, "--bogus"); r.code != 1 || !strings.Contains(r.stderr, "unknown flag") {
		t.Errorf("--bogus: exit %d stderr %q", r.code, r.stderr)
	}
	if r := runWith(t, "a.webm", "b.webm"); r.code != 1 || !strings.Contains(r.stderr, "one INPUT") {
		t.Errorf("two inputs: exit %d stderr %q", r.code, r.stderr)
	}
}

func TestOutputPathReplacesExtension(t *testing.T) {
	cases := map[string]string{
		"clip.webm":     "clip.mp4",
		"dir/talk.mkv":  "dir/talk.mp4",
		"noext":         "noext.mp4",
		"a.b.webm":      "a.b.mp4",
		"/abs/path.mov": "/abs/path.mp4",
	}
	for in, want := range cases {
		if got := outputPath(in); got != want {
			t.Errorf("outputPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// The help screen follows the shared standard: three header lines, then the
// sections the renderer accepts, with the standard rows left to the renderer.
func TestHelpDocFollowsTheStandard(t *testing.T) {
	if err := helpDoc().Check(); err != nil {
		t.Fatal(err)
	}
}
