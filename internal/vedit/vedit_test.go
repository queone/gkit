// vedit_test.go

package vedit

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// AT1 — parseOffset syntax, duration-independent.
func TestParseOffset(t *testing.T) {
	ok := []struct {
		in    string
		secs  int
		comps int
	}{
		{"8:31", 511, 2},
		{"0", 0, 1},
		{"90", 90, 1},
		{"1:08:31", 4111, 3},
		{"0:00", 0, 2},
	}
	for _, c := range ok {
		secs, comps, err := parseOffset(c.in)
		if err != nil {
			t.Errorf("parseOffset(%q) unexpected error: %v", c.in, err)
			continue
		}
		if secs != c.secs || comps != c.comps {
			t.Errorf("parseOffset(%q) = (%d, %d), want (%d, %d)", c.in, secs, comps, c.secs, c.comps)
		}
	}

	bad := []string{"8:61", "1:2:3:4", "abc", "-5", "1:-1", "", "1:60:00", "end"}
	for _, c := range bad {
		if _, _, err := parseOffset(c); err == nil {
			t.Errorf("parseOffset(%q) expected error, got nil", c)
		}
	}
}

// AT2 — validateTimestamp duration gate for 3-component forms.
func TestValidateTimestampDurationGate(t *testing.T) {
	if err := validateTimestamp(3, 3000); err == nil {
		t.Error("expected rejection of HH:MM:SS when duration <= 3600")
	}
	if err := validateTimestamp(3, 3600); err == nil {
		t.Error("expected rejection of HH:MM:SS when duration == 3600")
	}
	if err := validateTimestamp(3, 4000); err != nil {
		t.Errorf("expected HH:MM:SS accepted when duration > 3600, got %v", err)
	}
	if err := validateTimestamp(2, 100); err != nil {
		t.Errorf("MM:SS should never be duration-gated, got %v", err)
	}
}

// AT3 — resolveEnd: literal "end" yields the whole-second duration; a timestamp
// is parsed; an out-of-syntax token errors.
func TestResolveEnd(t *testing.T) {
	if got, err := resolveEnd("end", 600.9); err != nil || got != 600 {
		t.Errorf(`resolveEnd("end", 600.9) = (%d, %v), want (600, nil)`, got, err)
	}
	if got, err := resolveEnd("8:31", 600); err != nil || got != 511 {
		t.Errorf(`resolveEnd("8:31", 600) = (%d, %v), want (511, nil)`, got, err)
	}
	if _, err := resolveEnd("nope", 600); err == nil {
		t.Error(`resolveEnd("nope", 600) expected error, got nil`)
	}
}

// AT4 — clip/cut bounds, including the cut whole-video guard.
func TestValidateClipCut(t *testing.T) {
	const d = 600.0
	clip := []struct {
		name       string
		start, end int
		wantErr    bool
	}{
		{"interior", 60, 300, false},
		{"left to end", 60, 600, false},
		{"right from 0", 0, 300, false},
		{"whole file", 0, 600, false},
		{"start==end", 300, 300, true},
		{"start>end", 300, 60, true},
		{"end>dur", 60, 700, true},
		{"start<0", -1, 300, true},
	}
	for _, c := range clip {
		if err := validateClip(c.start, c.end, d); (err != nil) != c.wantErr {
			t.Errorf("clip %s: validateClip(%d,%d) err=%v wantErr=%v", c.name, c.start, c.end, err, c.wantErr)
		}
	}

	cut := []struct {
		name       string
		start, end int
		wantErr    bool
	}{
		{"interior", 60, 300, false},
		{"from 0", 0, 300, false},
		{"to end", 300, 600, false},
		{"remove all", 0, 600, true},
		{"start==end", 300, 300, true},
		{"end>dur", 60, 700, true},
	}
	for _, c := range cut {
		err := validateCut(c.start, c.end, d)
		if (err != nil) != c.wantErr {
			t.Errorf("cut %s: validateCut(%d,%d) err=%v wantErr=%v", c.name, c.start, c.end, err, c.wantErr)
		}
	}
	if err := validateCut(0, 600, d); err == nil || !contains(err.Error(), "entire video") {
		t.Errorf("validateCut(0,600) = %v, want 'entire video' error", err)
	}
}

// AT5 — keep produces exact argv per resolved bounds, fast and accurate.
func TestKeepPlans(t *testing.T) {
	const in, out = "in.mp4", "in_1.mp4"
	const d = 600.0

	t.Run("whole file is byte copy", func(t *testing.T) {
		if p := keep(0, 600, false, in, out, d); len(p.Cmds) != 0 {
			t.Errorf("whole-file keep should be empty Plan, got %v", p.Cmds)
		}
		if p := keep(0, 600, true, in, out, d); len(p.Cmds) != 0 {
			t.Errorf("whole-file keep (accurate) should be empty Plan, got %v", p.Cmds)
		}
	})
	t.Run("right copy", func(t *testing.T) {
		p := keep(0, 300, false, in, out, d)
		assertCmds(t, p.Cmds, [][]string{{"-i", in, "-to", "300", "-c", "copy", out}})
	})
	t.Run("right accurate", func(t *testing.T) {
		p := keep(0, 300, true, in, out, d)
		assertCmds(t, p.Cmds, [][]string{{"-i", in, "-to", "300", "-c:v", "libx264", "-c:a", "aac", out}})
	})
	t.Run("left copy", func(t *testing.T) {
		p := keep(60, 600, false, in, out, d)
		assertCmds(t, p.Cmds, [][]string{{"-ss", "60", "-i", in, "-c", "copy", out}})
	})
	t.Run("left accurate", func(t *testing.T) {
		p := keep(60, 600, true, in, out, d)
		assertCmds(t, p.Cmds, [][]string{{"-i", in, "-ss", "60", "-c:v", "libx264", "-c:a", "aac", out}})
	})
	t.Run("middle copy", func(t *testing.T) {
		p := keep(60, 511, false, in, out, d)
		assertCmds(t, p.Cmds, [][]string{{"-ss", "60", "-i", in, "-t", "451", "-c", "copy", out}})
	})
	t.Run("middle accurate", func(t *testing.T) {
		p := keep(60, 511, true, in, out, d)
		assertCmds(t, p.Cmds, [][]string{{"-i", in, "-ss", "60", "-t", "451", "-c:v", "libx264", "-c:a", "aac", out}})
	})
}

// AT6 — cut reductions and interior concat/filter shapes.
func TestCutPlans(t *testing.T) {
	const in, out, tmp = "in.mp4", "in_1.mp4", "/tmp/vt"
	const d = 600.0

	t.Run("from 0 reduces to left trim", func(t *testing.T) {
		p := cut(0, 300, false, in, out, "", d)
		assertCmds(t, p.Cmds, [][]string{{"-ss", "300", "-i", in, "-c", "copy", out}})
	})
	t.Run("to end reduces to right trim", func(t *testing.T) {
		p := cut(300, 600, false, in, out, "", d)
		assertCmds(t, p.Cmds, [][]string{{"-i", in, "-to", "300", "-c", "copy", out}})
	})
	t.Run("interior copy concat", func(t *testing.T) {
		p := cut(60, 300, false, in, out, tmp, d)
		seg1 := filepath.Join(tmp, "seg0.mp4")
		seg2 := filepath.Join(tmp, "seg1.mp4")
		list := filepath.Join(tmp, "concat.txt")
		assertCmds(t, p.Cmds, [][]string{
			{"-i", in, "-to", "60", "-c", "copy", seg1},
			{"-ss", "300", "-i", in, "-c", "copy", seg2},
			{"-f", "concat", "-safe", "0", "-i", list, "-c", "copy", out},
		})
		if p.ConcatList != list {
			t.Errorf("ConcatList = %q, want %q", p.ConcatList, list)
		}
		wantBody := "file '" + seg1 + "'\nfile '" + seg2 + "'\n"
		if p.ConcatBody != wantBody {
			t.Errorf("ConcatBody = %q, want %q", p.ConcatBody, wantBody)
		}
	})
	t.Run("interior accurate filter", func(t *testing.T) {
		p := cut(60, 300, true, in, out, tmp, d)
		if len(p.Cmds) != 1 {
			t.Fatalf("interior accurate: want single command, got %d", len(p.Cmds))
		}
		got := p.Cmds[0]
		if got[0] != "-i" || got[1] != in || got[2] != "-filter_complex" {
			t.Errorf("interior accurate argv prefix = %v", got[:3])
		}
		if got[len(got)-1] != out {
			t.Errorf("interior accurate output = %q, want %q", got[len(got)-1], out)
		}
		if p.ConcatList != "" {
			t.Errorf("interior accurate should not use a concat list, got %q", p.ConcatList)
		}
	})
}

// keep/drop argv is pinned byte-for-byte for each command form, so a future
// engine change cannot silently alter the ffmpeg invocations.
func TestKeepDropArgvPinned(t *testing.T) {
	const in, out, tmp = "in.mp4", "in_1.mp4", "/tmp/vt"
	const d = 600.0

	// vkeep START (END omitted) -> left trim, START=511
	assertCmds(t, keep(511, int(d), false, in, out, d).Cmds,
		[][]string{{"-ss", "511", "-i", in, "-c", "copy", out}})
	assertCmds(t, keep(511, int(d), true, in, out, d).Cmds,
		[][]string{{"-i", in, "-ss", "511", "-c:v", "libx264", "-c:a", "aac", out}})

	// vkeep 0 END -> right trim, END=300
	assertCmds(t, keep(0, 300, false, in, out, d).Cmds,
		[][]string{{"-i", in, "-to", "300", "-c", "copy", out}})

	// vkeep START END -> middle keep
	assertCmds(t, keep(60, 511, false, in, out, d).Cmds,
		[][]string{{"-ss", "60", "-i", in, "-t", "451", "-c", "copy", out}})

	// vkeep 0 (END omitted) -> whole-file byte copy
	if p := keep(0, int(d), false, in, out, d); len(p.Cmds) != 0 {
		t.Errorf("vkeep 0 should be byte copy, got %v", p.Cmds)
	}

	// vdrop START END (interior) -> concat of the two surrounding segments
	p := cut(60, 300, false, in, out, tmp, d)
	seg1 := filepath.Join(tmp, "seg0.mp4")
	seg2 := filepath.Join(tmp, "seg1.mp4")
	list := filepath.Join(tmp, "concat.txt")
	assertCmds(t, p.Cmds, [][]string{
		{"-i", in, "-to", "60", "-c", "copy", seg1},
		{"-ss", "300", "-i", in, "-c", "copy", seg2},
		{"-f", "concat", "-safe", "0", "-i", list, "-c", "copy", out},
	})
}

// AT8 — output name derivation.
func TestDeriveOutputName(t *testing.T) {
	cases := map[string]string{
		"SOURCE1.mp4": "SOURCE_1.mp4",
		"april12.mp4": "april_12.mp4",
		"clip.mp4":    "clip_1.mp4",
		"a.b.mkv":     "a.b_1.mkv",
	}
	for in, want := range cases {
		if got := deriveOutputName(in); got != want {
			t.Errorf("deriveOutputName(%q) = %q, want %q", in, got, want)
		}
	}
}

// AT9 — overwrite protection errors without invoking ffmpeg.
func TestOverwriteProtection(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "clip0.mp4")
	writeFile(t, input, "data")
	writeFile(t, filepath.Join(dir, "clip_0.mp4"), "existing") // derived output already present

	var calls int
	withFakes(t, 600, func([]string) error { calls++; return nil })

	err := Keep(false, "8:31", "end", input)
	if err == nil || !contains(err.Error(), "already exists") {
		t.Fatalf("expected overwrite error, got %v", err)
	}
	if calls != 0 {
		t.Errorf("ffmpeg invoked %d times, want 0", calls)
	}
}

// AT10 — whole-file clip byte-copies without invoking ffmpeg.
func TestWholeFileByteCopy(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "clip0.mp4")
	writeFile(t, input, "payload")

	var calls int
	withFakes(t, 600, func([]string) error { calls++; return nil })

	if err := Keep(false, "0", "end", input); err != nil {
		t.Fatalf("Keep whole-file failed: %v", err)
	}
	if calls != 0 {
		t.Errorf("ffmpeg invoked %d times for a byte copy, want 0", calls)
	}
	got, err := os.ReadFile(filepath.Join(dir, "clip_0.mp4"))
	if err != nil || string(got) != "payload" {
		t.Errorf("byte copy output = %q (err %v), want %q", got, err, "payload")
	}
}

// AT11 — accurate vs copy routing reaches ffmpeg.
func TestAccurateVsCopyRouting(t *testing.T) {
	check := func(accurate bool, wantFlag, notFlag string) {
		dir := t.TempDir()
		input := filepath.Join(dir, "clip3.mp4")
		writeFile(t, input, "data")

		var got []string
		withFakes(t, 600, func(args []string) error {
			got = args
			writeFile(t, args[len(args)-1], "out")
			return nil
		})
		if err := Keep(accurate, "8:31", "end", input); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if !sliceHas(got, wantFlag) {
			t.Errorf("accurate=%v argv %v missing %q", accurate, got, wantFlag)
		}
		if sliceHas(got, notFlag) {
			t.Errorf("accurate=%v argv %v should not contain %q", accurate, got, notFlag)
		}
	}
	check(false, "copy", "libx264")
	check(true, "libx264", "copy")
}

// AT12 — tools missing prints the brew hint and fails before any work, for both
// Keep and Drop.
func TestToolMissing(t *testing.T) {
	restore := lookPath
	defer func() { lookPath = restore }()
	lookPath = func(string) (string, error) { return "", errors.New("not found") }

	for _, fn := range []func() error{
		func() error { return Keep(false, "8:31", "end", "whatever.mp4") },
		func() error { return Drop(false, 0, "1:00", "8:31", "whatever.mp4") },
	} {
		err := fn()
		if err == nil || !contains(err.Error(), "brew install ffmpeg") {
			t.Errorf("expected brew hint, got %v", err)
		}
	}
}

// AT13 — interior cut temp dir cleaned up on success and on failure.
func TestCutTempCleanup(t *testing.T) {
	run13 := func(fail bool) string {
		dir := t.TempDir()
		input := filepath.Join(dir, "clip5.mp4")
		writeFile(t, input, "data")
		tmp := filepath.Join(dir, "scratch")

		restoreMk := makeTempDir
		makeTempDir = func() (string, error) { return tmp, os.Mkdir(tmp, 0o755) }
		defer func() { makeTempDir = restoreMk }()

		withFakes(t, 600, func(args []string) error {
			if fail {
				return errors.New("boom")
			}
			writeFile(t, args[len(args)-1], "out")
			return nil
		})

		err := Drop(false, 0, "1:00", "5:00", input)
		if fail && err == nil {
			t.Error("expected failure to propagate")
		}
		if !fail && err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		return tmp
	}

	for _, fail := range []bool{false, true} {
		tmp := run13(fail)
		if _, err := os.Stat(tmp); !os.IsNotExist(err) {
			t.Errorf("fail=%v: temp dir %q not cleaned up (stat err=%v)", fail, tmp, err)
		}
	}
}

// AT18 — END is optional: the two-positional form (END omitted) runs to the
// source end, matching the explicit literal "end".
func TestOptionalEndEquivalence(t *testing.T) {
	dir := t.TempDir()

	capture := func(endTok string) []string {
		input := filepath.Join(dir, "src1.mp4")
		out := filepath.Join(dir, "src_1.mp4")
		writeFile(t, input, "data")
		defer func() { os.Remove(input); os.Remove(out) }()

		var got []string
		withFakes(t, 600, func(args []string) error {
			got = args
			writeFile(t, args[len(args)-1], "out")
			return nil
		})
		if err := Keep(false, "4:13", endTok, input); err != nil {
			t.Fatalf("Keep(%q) failed: %v", endTok, err)
		}
		return got
	}

	omitted := capture("end") // wrappers pass "end" when END is omitted
	if !sliceHas(omitted, "-ss") {
		t.Errorf("omitted-END keep argv %v should be a left trim (-ss)", omitted)
	}
}

// Usage renders one shared screen for both tools: it contains every cheatsheet
// command form and the whole-file note, and the two tools' screens are identical
// apart from the header line and which command column is highlighted.
func TestUsage(t *testing.T) {
	keepScreen := Usage("vkeep", "0.1.0")
	for _, want := range []string{
		"vkeep 0 FILE",
		"vkeep 0 1:00 FILE",
		"vdrop 0 1:00 FILE",
		"vkeep 1:00 FILE",
		"vdrop 1:00 FILE",
		"vkeep 1:00 8:31 FILE",
		"vdrop 1:00 8:31 FILE",
		"vdrop has no whole-file form",
		"--crossfade",
		"Print vkeep v0.1.0 and exit",
	} {
		if !contains(keepScreen, want) {
			t.Errorf("vkeep usage missing %q", want)
		}
	}
	goals := []string{
		"Copy the whole file",
		"Keep from beginning to 1:00",
		"Drop from beginning to 1:00",
		"Keep from 1:00 to the end",
	}
	last := -1
	for _, goal := range goals {
		pos := strings.Index(keepScreen, goal)
		if pos < 0 {
			t.Fatalf("vkeep usage missing goal %q", goal)
		}
		if pos <= last {
			t.Fatalf("vkeep usage goal %q is out of order", goal)
		}
		last = pos
	}

	dropScreen := Usage("vdrop", "0.1.0")
	if !contains(dropScreen, "Print vdrop v0.1.0 and exit") {
		t.Error("vdrop usage describes version output incorrectly")
	}
	if stripExceptHeader(keepScreen) != stripExceptHeader(dropScreen) {
		t.Error("vkeep and vdrop usage bodies differ beyond the header line and highlight")
	}
}

func TestUsageREADMECheatsheets(t *testing.T) {
	want := "  Copy the whole file             vkeep 0 FILE\n" +
		"  Keep from beginning to 1:00       vkeep 0 1:00 FILE\n" +
		"  Drop from beginning to 1:00       vdrop 0 1:00 FILE\n" +
		"  Keep from 1:00 to the end       vkeep 1:00 FILE"
	for _, path := range []string{
		filepath.Join("..", "..", "cmd", "vkeep", "README.md"),
		filepath.Join("..", "..", "cmd", "vdrop", "README.md"),
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(body), want) {
			t.Errorf("%s cheatsheet missing ordered beginning-of-clip rows", path)
		}
	}
}

// AT1 — crossfadeFilter emits the xfade/acrossfade graph with the transition
// beginning dur seconds before the cut (offset = start - dur).
func TestCrossfadeFilter(t *testing.T) {
	got := crossfadeFilter(60, 300, 1)
	for _, want := range []string{
		"[0:v]trim=0:60",
		"[0:v]trim=start=300",
		"xfade=transition=fade",
		"duration=1",
		"offset=59",
		"acrossfade=d=1",
	} {
		if !contains(got, want) {
			t.Errorf("crossfadeFilter missing %q in %q", want, got)
		}
	}
	// Fractional duration yields a fractional offset.
	if f := crossfadeFilter(60, 300, 0.5); !contains(f, "duration=0.5") || !contains(f, "offset=59.5") {
		t.Errorf("fractional crossfade filter = %q", f)
	}
}

// AT2 — Drop with crossfade > 0 routes to the single filter-complex re-encode
// (no concat list); crossfade == 0 keeps the existing hard-cut plan.
func TestCrossfadeRouting(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "clip7.mp4")
	writeFile(t, input, "data")

	var got []string
	withFakes(t, 600, func(args []string) error {
		got = args
		writeFile(t, args[len(args)-1], "out")
		return nil
	})
	if err := Drop(false, 1, "1:00", "5:00", input); err != nil {
		t.Fatalf("crossfade Drop failed: %v", err)
	}
	joined := strings.Join(got, " ")
	if !sliceHas(got, "-filter_complex") || !contains(joined, "xfade=transition=fade") {
		t.Errorf("crossfade Drop argv %v should use the xfade filter", got)
	}
	if sliceHas(got, "concat") {
		t.Errorf("crossfade Drop should not concat, argv %v", got)
	}
	// Hard-cut routing is exercised by TestCutPlans/TestKeepDropArgvPinned.
}

// AT3/AT4 — crossfade is interior-only and its duration must fit both kept
// segments.
func TestCrossfadeValidation(t *testing.T) {
	const d = 600.0
	if err := validateCrossfade(0, 300, d, 1); err == nil {
		t.Error("crossfade should be rejected for a left-edge drop")
	}
	if err := validateCrossfade(300, 600, d, 1); err == nil {
		t.Error("crossfade should be rejected for a right-edge drop")
	}
	if err := validateCrossfade(60, 300, d, 0); err == nil {
		t.Error("zero crossfade duration should error")
	}
	if err := validateCrossfade(60, 300, d, -1); err == nil {
		t.Error("negative crossfade duration should error")
	}
	if err := validateCrossfade(60, 300, d, 61); err == nil {
		t.Error("crossfade longer than the kept-before segment should error")
	}
	if err := validateCrossfade(60, 300, d, 301); err == nil {
		t.Error("crossfade longer than the kept-after segment should error")
	}
	if err := validateCrossfade(60, 300, d, 0.5); err != nil {
		t.Errorf("in-bounds crossfade should be accepted, got %v", err)
	}
}

// --- helpers ---

// withFakes installs lookPath, runFFprobe (fixed duration), and runFFmpeg fakes,
// restoring them when the test ends.
func withFakes(t *testing.T, durationSecs int, ffmpeg func([]string) error) {
	t.Helper()
	rl, rp, rf := lookPath, runFFprobe, runFFmpeg
	t.Cleanup(func() { lookPath, runFFprobe, runFFmpeg = rl, rp, rf })
	lookPath = func(string) (string, error) { return "/fake/bin", nil }
	runFFprobe = func([]string) ([]byte, error) {
		return []byte(strconv.Itoa(durationSecs) + ".000000\n"), nil
	}
	runFFmpeg = ffmpeg
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %q: %v", path, err)
	}
}

func assertCmds(t *testing.T, got, want [][]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("argv mismatch:\n got  %v\n want %v", got, want)
	}
}

func sliceHas(s []string, v string) bool {
	return slices.Contains(s, v)
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripExceptHeader removes ANSI color codes and normalizes the two per-tool
// details, leaving the shared body for cross-tool comparison.
func stripExceptHeader(s string) string {
	plain := ansiRE.ReplaceAllString(s, "")
	if _, body, found := strings.Cut(plain, "\n"); found {
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "  -v, --version") {
				lines[i] = "  -v, --version <per-tool version line>"
			}
		}
		return strings.Join(lines, "\n")
	}
	return plain
}
