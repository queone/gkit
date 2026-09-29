package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-done
}

func withCleanHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	return home
}

func TestXDGStateDirHonorsAbsoluteEnvVar(t *testing.T) {
	home := withCleanHome(t)
	abs := filepath.Join(home, "custom-state")
	t.Setenv("XDG_STATE_HOME", abs)

	got, err := xdgStateDir()
	if err != nil {
		t.Fatalf("xdgStateDir: %v", err)
	}
	if got != abs {
		t.Errorf("xdgStateDir = %q, want %q", got, abs)
	}
}

func TestXDGStateDirIgnoresRelativeEnvVar(t *testing.T) {
	home := withCleanHome(t)
	t.Setenv("XDG_STATE_HOME", "relative/path")

	got, err := xdgStateDir()
	if err != nil {
		t.Fatalf("xdgStateDir: %v", err)
	}
	want := filepath.Join(home, ".local", "state")
	if got != want {
		t.Errorf("xdgStateDir = %q, want %q", got, want)
	}
}

func TestXDGStateDirFallsBackWhenUnset(t *testing.T) {
	home := withCleanHome(t)

	got, err := xdgStateDir()
	if err != nil {
		t.Fatalf("xdgStateDir: %v", err)
	}
	want := filepath.Join(home, ".local", "state")
	if got != want {
		t.Errorf("xdgStateDir = %q, want %q", got, want)
	}
}

func TestConfigPathResolvesNewLocation(t *testing.T) {
	home := withCleanHome(t)

	got, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	want := filepath.Join(home, ".local", "state", "cash5", "draws.json")
	if got != want {
		t.Errorf("configPath = %q, want %q", got, want)
	}
}

func TestMigrationFromLegacyPath(t *testing.T) {
	home := withCleanHome(t)
	oldPath := filepath.Join(home, ".config", "cash5", "draws.json")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	body := []byte(`[{"id":"draw-1","gameName":"Cash 5","drawTime":1735689600000}]`)
	if err := os.WriteFile(oldPath, body, 0644); err != nil {
		t.Fatalf("seed old: %v", err)
	}

	var newPath string
	stderr := captureStderr(t, func() {
		var err error
		newPath, err = configPath()
		if err != nil {
			t.Fatalf("configPath: %v", err)
		}
	})

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("old path still exists: err=%v", err)
	}

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read new: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("content mismatch; got %q, want %q", got, body)
	}

	if !strings.Contains(stderr, "migrated") || !strings.Contains(stderr, oldPath) || !strings.Contains(stderr, newPath) {
		t.Errorf("stderr missing migration notice: %q", stderr)
	}
}

func TestMigrationSkippedWhenBothExist(t *testing.T) {
	home := withCleanHome(t)
	oldPath := filepath.Join(home, ".config", "cash5", "draws.json")
	newPath := filepath.Join(home, ".local", "state", "cash5", "draws.json")

	if err := os.MkdirAll(filepath.Dir(oldPath), 0755); err != nil {
		t.Fatalf("mkdir old: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	if err := os.WriteFile(oldPath, []byte("OLD"), 0644); err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if err := os.WriteFile(newPath, []byte("NEW"), 0644); err != nil {
		t.Fatalf("seed new: %v", err)
	}

	stderr := captureStderr(t, func() {
		if _, err := configPath(); err != nil {
			t.Fatalf("configPath: %v", err)
		}
	})

	if !strings.Contains(stderr, "both") {
		t.Errorf("stderr missing both-exist warning: %q", stderr)
	}

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read new: %v", err)
	}
	if string(got) != "NEW" {
		t.Errorf("new file overwritten; got %q, want NEW", string(got))
	}
}

func TestMigrationSkippedForSymlink(t *testing.T) {
	home := withCleanHome(t)
	target := filepath.Join(home, "actual-draws.json")
	if err := os.WriteFile(target, []byte("[]"), 0644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	oldPath := filepath.Join(home, ".config", "cash5", "draws.json")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(target, oldPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	stderr := captureStderr(t, func() {
		if _, err := configPath(); err != nil {
			t.Fatalf("configPath: %v", err)
		}
	})

	info, err := os.Lstat(oldPath)
	if err != nil {
		t.Fatalf("old symlink missing: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("old path is no longer a symlink (mode=%v)", info.Mode())
	}
	if !strings.Contains(stderr, "symlink") {
		t.Errorf("stderr missing symlink warning: %q", stderr)
	}
}

func TestColdStartReturnsEmpty(t *testing.T) {
	withCleanHome(t)

	stderr := captureStderr(t, func() {
		got, err := loadDraws()
		if err != nil {
			t.Fatalf("loadDraws: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %d draws", len(got))
		}
	})

	if stderr != "" {
		t.Errorf("expected silent cold start, got stderr: %q", stderr)
	}
}

// The cutoff is the first 1-45 pool draw, 2020-06-29 at midnight Eastern, and
// the date text in messages names the same day.
func TestCutoffIsTheFirst145PoolDraw(t *testing.T) {
	want := time.Date(2020, 6, 29, 0, 0, 0, 0, easternTime()).UnixMilli()
	if cash5EraStartMillis != want || cash5EraStartMillis != 1593403200000 {
		t.Errorf("cash5EraStartMillis = %d, want %d and 1593403200000", cash5EraStartMillis, want)
	}
	if got := easternDate(cash5EraStartMillis); got != cash5EraStartDate {
		t.Errorf("cash5EraStartDate = %q, want %q", cash5EraStartDate, got)
	}
}

func TestPruneLegacyEraFiltersPreCutoff(t *testing.T) {
	draws := []Draw{
		{ID: "pre-1", DrawTime: 718430400000},                       // 1992-10-08
		{ID: "pre-2", DrawTime: cash5EraStartMillis - 86_400_000},   // day before cutoff
		{ID: "boundary", DrawTime: cash5EraStartMillis},             // exact cutoff (kept)
		{ID: "post-1", DrawTime: cash5EraStartMillis + 86_400_000},  // day after
		{ID: "post-2", DrawTime: cash5EraStartMillis + 172_800_000}, // two days after
	}

	pruned, removed := pruneLegacyEra(draws)

	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	if len(pruned) != 3 {
		t.Fatalf("pruned len = %d, want 3", len(pruned))
	}
	wantIDs := []string{"boundary", "post-1", "post-2"}
	for i, want := range wantIDs {
		if pruned[i].ID != want {
			t.Errorf("pruned[%d].ID = %q, want %q", i, pruned[i].ID, want)
		}
	}
}

func TestPruneLegacyEraEmptyInput(t *testing.T) {
	pruned, removed := pruneLegacyEra(nil)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if len(pruned) != 0 {
		t.Errorf("len(pruned) = %d, want 0", len(pruned))
	}
}

func TestPruneLegacyEraIdempotent(t *testing.T) {
	draws := []Draw{
		{ID: "post-1", DrawTime: cash5EraStartMillis},
		{ID: "post-2", DrawTime: cash5EraStartMillis + 86_400_000},
	}
	once, r1 := pruneLegacyEra(draws)
	twice, r2 := pruneLegacyEra(once)
	if r1 != 0 || r2 != 0 {
		t.Errorf("removed counts = (%d, %d), want (0, 0)", r1, r2)
	}
	if len(twice) != len(draws) {
		t.Errorf("len changed across runs: got %d, want %d", len(twice), len(draws))
	}
}

func TestLoadDrawsPrunesAndRewritesOnce(t *testing.T) {
	withCleanHome(t)
	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}

	seed := []Draw{
		{ID: "pre-A", DrawTime: 718430400000, GameName: "Cash 5"},
		{ID: "pre-B", DrawTime: cash5EraStartMillis - 86_400_000, GameName: "Cash 5"},
		{ID: "post-1", DrawTime: cash5EraStartMillis, GameName: "Cash 5"},
		{ID: "post-2", DrawTime: cash5EraStartMillis + 86_400_000, GameName: "Cash 5"},
	}
	if err := atomicWriteDraws(path, seed); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	var firstStderr string
	var got []Draw
	firstStderr = captureStderr(t, func() {
		var err error
		got, err = loadDraws()
		if err != nil {
			t.Fatalf("loadDraws: %v", err)
		}
	})

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, d := range got {
		if d.DrawTime < cash5EraStartMillis {
			t.Errorf("pre-cutoff row leaked: id=%s drawTime=%d", d.ID, d.DrawTime)
		}
	}
	if !strings.Contains(firstStderr, "pruned 2 pre-2020-06-29 rows") {
		t.Errorf("stderr missing prune notice: %q", firstStderr)
	}

	infoAfterFirst, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after first: %v", err)
	}

	secondStderr := captureStderr(t, func() {
		got2, err := loadDraws()
		if err != nil {
			t.Fatalf("loadDraws second: %v", err)
		}
		if len(got2) != 2 {
			t.Errorf("second len = %d, want 2", len(got2))
		}
	})
	if secondStderr != "" {
		t.Errorf("second call should be silent, got stderr: %q", secondStderr)
	}

	infoAfterSecond, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after second: %v", err)
	}
	if !infoAfterFirst.ModTime().Equal(infoAfterSecond.ModTime()) {
		t.Errorf("second load rewrote the file (mtime changed: %v → %v)",
			infoAfterFirst.ModTime(), infoAfterSecond.ModTime())
	}
}

func TestSaveLoadRoundTripThroughResolver(t *testing.T) {
	withCleanHome(t)

	want := []Draw{
		{ID: "draw-1", DrawTime: 1735689600000, GameName: "Cash 5"},
		{ID: "draw-2", DrawTime: 1735776000000, GameName: "Cash 5"},
	}
	if err := saveDrawsCallback(want); err != nil {
		t.Fatalf("saveDrawsCallback: %v", err)
	}

	got, err := loadDraws()
	if err != nil {
		t.Fatalf("loadDraws: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}

	resolvedPath, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if _, err := os.Stat(resolvedPath); err != nil {
		t.Errorf("save did not land at configPath() result %s: %v", resolvedPath, err)
	}

	body, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatalf("read resolved: %v", err)
	}
	var roundTrip []Draw
	if err := json.Unmarshal(body, &roundTrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(roundTrip) != len(want) {
		t.Errorf("round-trip mismatch: got %d, want %d", len(roundTrip), len(want))
	}
}
