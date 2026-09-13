package main

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/queone/gkit/internal/color"
)

const mp4Format = "mov,mp4,m4a,3gp,3g2,mj2"

var fixedDay = time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)

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

// stubSeams fixes the clock, answers every probe with format, and records each
// ffmpeg argument list while creating the output file.
func stubSeams(t *testing.T, format string) *[][]string {
	t.Helper()
	rp, rt, rn := probeFormat, transcode, now
	t.Cleanup(func() { probeFormat, transcode, now = rp, rt, rn })
	now = func() time.Time { return fixedDay }
	probeFormat = func(string) (string, error) { return format, nil }
	var calls [][]string
	transcode = func(input, output string, argv []string) error {
		calls = append(calls, append([]string(nil), argv...))
		return os.WriteFile(output, []byte("mp4"), 0o644)
	}
	return &calls
}

// touch creates a small file in the current directory.
func touch(t *testing.T, name string) {
	t.Helper()
	if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestShrinkBuildsFfmpegArgumentsForMp4(t *testing.T) {
	calls := stubSeams(t, mp4Format)
	t.Chdir(t.TempDir())
	touch(t, "clip.mp4")
	r := runWith(t, "clip.mp4")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	want := []string{"-i", "clip.mp4", "-vcodec", "libx264", "-crf", "35", "clip_20260908a.mp4"}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Errorf("ffmpeg calls = %v, want exactly %v", *calls, want)
	}
	if !strings.Contains(r.stdout, "Shrinking clip.mp4") {
		t.Errorf("stdout = %q, want the shrinking line", r.stdout)
	}
}

func TestShrinkRejectsNonMp4Container(t *testing.T) {
	calls := stubSeams(t, "matroska,webm")
	t.Chdir(t.TempDir())
	touch(t, "clip.mp4")
	r := runWith(t, "clip.mp4")
	if r.code != 1 || !strings.Contains(r.stderr, "clip.mp4 is not a valid MP4 file") {
		t.Errorf("exit %d, stderr %q; want exit 1 naming clip.mp4", r.code, r.stderr)
	}
	if len(*calls) != 0 {
		t.Errorf("ffmpeg ran %d times, want 0", len(*calls))
	}
}

func TestShrinkReportsProbeFailure(t *testing.T) {
	stubSeams(t, mp4Format)
	probeFormat = func(string) (string, error) { return "", errors.New("probing \"clip.mp4\": exit status 1") }
	t.Chdir(t.TempDir())
	touch(t, "clip.mp4")
	r := runWith(t, "clip.mp4")
	if r.code != 1 || !strings.Contains(r.stderr, "probing") {
		t.Errorf("exit %d, stderr %q; want exit 1 with the probe error", r.code, r.stderr)
	}
}

func TestShrinkRefusesExistingOutput(t *testing.T) {
	calls := stubSeams(t, mp4Format)
	t.Chdir(t.TempDir())
	touch(t, "clip.mp4")
	touch(t, "clip_20260908a.mp4")
	r := runWith(t, "clip.mp4")
	if r.code != 1 || !strings.Contains(r.stderr, `"clip_20260908a.mp4" already exists`) || len(*calls) != 0 {
		t.Errorf("exit %d, stderr %q, calls %d; want exit 1 and no ffmpeg run", r.code, r.stderr, len(*calls))
	}
}

func TestShrinkRejectsMissingInput(t *testing.T) {
	stubSeams(t, mp4Format)
	t.Chdir(t.TempDir())
	r := runWith(t, "missing.mp4")
	if r.code != 1 || !strings.Contains(r.stderr, "not a readable file") {
		t.Errorf("exit %d, stderr %q; want exit 1 not-readable", r.code, r.stderr)
	}
}

func TestVersionHelpAndBadFlags(t *testing.T) {
	if r := runWith(t, "-v"); r.code != 0 || r.stdout != "vshrink v"+programVersion+"\n" {
		t.Errorf("-v: exit %d stdout %q", r.code, r.stdout)
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
}

func TestOutputPathAppendsDateSuffix(t *testing.T) {
	cases := map[string]string{
		"clip.mp4":      "clip_20260908a.mp4",
		"dir/movie.MP4": "dir/movie_20260908a.mp4",
		"/abs/x.y.mp4":  "/abs/x.y_20260908a.mp4",
		"noext":         "noext_20260908a.mp4",
	}
	for in, want := range cases {
		if got := outputPath(in, fixedDay); got != want {
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
