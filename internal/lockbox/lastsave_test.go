package lockbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordLastSaveWritesAPrivateRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "prog", "last-save-abc")
	at := time.Date(2026, 9, 23, 21, 55, 2, 0, time.UTC)
	r := SaveRecord{ID: strings.Repeat("a", 32), Generation: 7, Host: "np10", At: at, Action: "push ~/.bashrc"}
	if err := RecordLastSave(path, r); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("record file: %v %v", info, err)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("record directory: %v %v", info, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"id":"` + r.ID + `","generation":7,"at":"2026-09-23T21:55:02Z","action":"push ~/.bashrc"}` + "\n"; string(b) != want {
		t.Fatalf("record %q, want %q", b, want)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RecordLastSave(path, r); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("rewritten record mode %o, want 600", info.Mode().Perm())
	}
}

func TestCheckLastSaveOutcomes(t *testing.T) {
	dir := t.TempDir()
	st, _ := newTestStore(t, dir)
	defer st.Close()
	path := filepath.Join(dir, "state", "last-save")

	if _, state, err := st.CheckLastSave(path); err != nil || state != SaveNone {
		t.Fatalf("no record: state %d err %v", state, err)
	}

	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	orphan := SaveRecord{ID: strings.Repeat("0", 32), Generation: 1, At: time.Now(), Action: "init"}
	if err := RecordLastSave(path, orphan); err != nil {
		t.Fatal(err)
	}
	if _, state, err := st.CheckLastSave(path); err != nil || state != SaveTooOld {
		t.Fatalf("empty log: state %d err %v", state, err)
	}

	first, err := st.SaveLogged("np10", "init", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordLastSave(path, first); err != nil {
		t.Fatal(err)
	}
	rec, state, err := st.CheckLastSave(path)
	if err != nil || state != SaveInStore || rec.ID != first.ID || rec.Generation != first.Generation || rec.Action != "init" {
		t.Fatalf("in store: %+v state %d err %v", rec, state, err)
	}

	lost := SaveRecord{ID: strings.Repeat("1", 32), Generation: first.Generation, At: time.Now(), Action: "push ~/.bashrc"}
	if err := RecordLastSave(path, lost); err != nil {
		t.Fatal(err)
	}
	if _, state, err := st.CheckLastSave(path); err != nil || state != SaveLost {
		t.Fatalf("lost: state %d err %v", state, err)
	}

	older := SaveRecord{ID: strings.Repeat("2", 32), Generation: first.Generation - 1, At: time.Now(), Action: "add ~/.profile"}
	if err := RecordLastSave(path, older); err != nil {
		t.Fatal(err)
	}
	if _, state, err := st.CheckLastSave(path); err != nil || state != SaveTooOld {
		t.Fatalf("older than the log: state %d err %v", state, err)
	}

	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.CheckLastSave(path); err == nil || !strings.Contains(err.Error(), "unreadable record "+path) {
		t.Fatalf("unreadable record: %v", err)
	}
}
