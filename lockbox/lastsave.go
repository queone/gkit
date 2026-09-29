package lockbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// LastSave is one Mac's record of its most recent logged save to one store.
type LastSave struct {
	ID         string    `json:"id"`
	Generation uint64    `json:"generation"`
	At         time.Time `json:"at"`
	Action     string    `json:"action"`
}

// SaveState classifies a Mac's last save against a loaded store.
type SaveState int

const (
	SaveNone    SaveState = iota // no record for this store
	SaveInStore                  // the save log holds the record's save
	SaveLost                     // the log covers the record's generation but lacks its save
	SaveTooOld                   // the log is empty or starts after the record's generation
)

// RecordLastSave writes r to path as this Mac's last save. The directory is
// created with mode 0700 and the file is written with mode 0600; the record
// holds the save's id, generation, time, and action, never file contents.
func RecordLastSave(path string, r SaveRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(LastSave{ID: r.ID, Generation: r.Generation, At: r.At, Action: r.Action})
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// CheckLastSave reads the last-save record at path and looks its save up in
// the store's save log.
func (s *Store) CheckLastSave(path string) (LastSave, SaveState, error) {
	var rec LastSave
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return rec, SaveNone, nil
	}
	if err != nil {
		return rec, SaveNone, err
	}
	if err := json.Unmarshal(b, &rec); err != nil || rec.ID == "" {
		return rec, SaveNone, fmt.Errorf("unreadable record %s", path)
	}
	found, oldest, err := s.LookupSave(rec.ID)
	switch {
	case err != nil:
		return rec, SaveNone, err
	case found:
		return rec, SaveInStore, nil
	case oldest == 0 || oldest > rec.Generation:
		return rec, SaveTooOld, nil
	}
	return rec, SaveLost, nil
}
