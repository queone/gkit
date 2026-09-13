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

// stubSeams fixes the clock, treats the platform as macOS with sips installed,
// and records each sips argument list while creating the output file.
func stubSeams(t *testing.T) *[][]string {
	t.Helper()
	rl, rs, rn, rd := lookPath, runSips, now, isDarwin
	t.Cleanup(func() { lookPath, runSips, now, isDarwin = rl, rs, rn, rd })
	now = func() time.Time { return fixedDay }
	isDarwin = true
	lookPath = func(string) (string, error) { return "/usr/bin/sips", nil }
	var calls [][]string
	runSips = func(args []string) error {
		calls = append(calls, append([]string(nil), args...))
		return os.WriteFile(args[len(args)-1], []byte("jpg"), 0o644)
	}
	return &calls
}

// touch creates a file of the given size in the current directory.
func touch(t *testing.T, name string, size int) {
	t.Helper()
	if err := os.WriteFile(name, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestShrinkBuildsSipsArgumentsForHeic(t *testing.T) {
	calls := stubSeams(t)
	t.Chdir(t.TempDir())
	touch(t, "photo.heic", 1234567)
	r := runWith(t, "photo.heic")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	want := []string{"-s", "format", "jpeg", "-s", "formatOptions", "10", "photo.heic", "-o", "photo_20260908a.jpg"}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Errorf("sips calls = %v, want exactly %v", *calls, want)
	}
	for _, s := range []string{"Shrinking photo.heic", "1,234,567", "photo_20260908a.jpg"} {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("stdout = %q, want it to contain %q", r.stdout, s)
		}
	}
}

func TestShrinkAcceptsUppercaseJpgExtension(t *testing.T) {
	calls := stubSeams(t)
	t.Chdir(t.TempDir())
	touch(t, "IMG.JPG", 10)
	if r := runWith(t, "IMG.JPG"); r.code != 0 || len(*calls) != 1 {
		t.Errorf("exit %d, stderr %q, calls %d; want exit 0 and one sips run", r.code, r.stderr, len(*calls))
	}
}

func TestShrinkRejectsUnsupportedExtension(t *testing.T) {
	calls := stubSeams(t)
	t.Chdir(t.TempDir())
	r := runWith(t, "notes.txt")
	if r.code != 1 || !strings.Contains(r.stderr, "usage: ishrink FILE.[heic|jpeg|jpg]") || len(*calls) != 0 {
		t.Errorf("exit %d, stderr %q, calls %d; want exit 1 with the usage line", r.code, r.stderr, len(*calls))
	}
}

func TestShrinkRefusesExistingOutput(t *testing.T) {
	calls := stubSeams(t)
	t.Chdir(t.TempDir())
	touch(t, "photo.jpg", 10)
	touch(t, "photo_20260908a.jpg", 10)
	r := runWith(t, "photo.jpg")
	if r.code != 1 || !strings.Contains(r.stderr, `"photo_20260908a.jpg" already exists`) || len(*calls) != 0 {
		t.Errorf("exit %d, stderr %q, calls %d; want exit 1 and no sips run", r.code, r.stderr, len(*calls))
	}
}

func TestShrinkRequiresMacOS(t *testing.T) {
	calls := stubSeams(t)
	isDarwin = false
	t.Chdir(t.TempDir())
	touch(t, "photo.heic", 10)
	r := runWith(t, "photo.heic")
	if r.code != 1 || !strings.Contains(r.stderr, "macOS") || len(*calls) != 0 {
		t.Errorf("exit %d, stderr %q, calls %d; want exit 1 naming macOS", r.code, r.stderr, len(*calls))
	}
}

func TestShrinkReportsMissingSips(t *testing.T) {
	stubSeams(t)
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Chdir(t.TempDir())
	touch(t, "photo.heic", 10)
	r := runWith(t, "photo.heic")
	if r.code != 1 || !strings.Contains(r.stderr, "sips not found") {
		t.Errorf("exit %d, stderr %q; want exit 1 naming sips", r.code, r.stderr)
	}
}

func TestShrinkReportsSipsFailure(t *testing.T) {
	stubSeams(t)
	runSips = func([]string) error { return errors.New("exit status 1") }
	t.Chdir(t.TempDir())
	touch(t, "photo.heic", 10)
	r := runWith(t, "photo.heic")
	if r.code != 1 || !strings.Contains(r.stderr, "sips failed") {
		t.Errorf("exit %d, stderr %q; want exit 1 with the sips error", r.code, r.stderr)
	}
}

func TestShrinkRejectsMissingInput(t *testing.T) {
	stubSeams(t)
	t.Chdir(t.TempDir())
	r := runWith(t, "missing.heic")
	if r.code != 1 || !strings.Contains(r.stderr, "not a readable file") {
		t.Errorf("exit %d, stderr %q; want exit 1 not-readable", r.code, r.stderr)
	}
}

func TestVersionHelpAndBadFlags(t *testing.T) {
	if r := runWith(t, "-v"); r.code != 0 || r.stdout != "ishrink v"+programVersion+"\n" {
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

func TestOutputPathUsesDateAndJpg(t *testing.T) {
	cases := map[string]string{
		"photo.heic":     "photo_20260908a.jpg",
		"dir/IMG_1.JPEG": "dir/IMG_1_20260908a.jpg",
		"/abs/a.b.jpg":   "/abs/a.b_20260908a.jpg",
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
