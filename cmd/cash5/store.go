package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const programDataName = "cash5"

// cash5EraStartMillis is the UnixMilli of 2020-06-29 00:00 Eastern (04:00
// UTC), the first Cash 5 draw under the 1-45 pool. Draws before it predate the
// 1-45 pool; they are pruned at load and not retained.
const cash5EraStartMillis int64 = 1593403200000

// cash5EraStartDate is the Eastern date of cash5EraStartMillis, used in every
// message and help line that names the cutoff.
const cash5EraStartDate = "2020-06-29"

// xdgStateDir returns the user's XDG state directory, honoring
// XDG_STATE_HOME when set to an absolute path and falling back to
// $HOME/.local/state otherwise.
func xdgStateDir() (string, error) {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" && filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}

// xdgConfigDir returns the user's XDG config directory, used here only to
// resolve the legacy draws.json path that lived under $HOME/.config/cash5/.
func xdgConfigDir() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// configPath returns the canonical state-file path for cash5's draws cache,
// performing a one-shot lazy migration from the legacy
// $HOME/.config/cash5/draws.json location on first call. The returned path's
// parent directory is created with MkdirAll so callers can write directly.
func configPath() (string, error) {
	stateDir, err := xdgStateDir()
	if err != nil {
		return "", err
	}
	newDir := filepath.Join(stateDir, programDataName)
	newPath := filepath.Join(newDir, "draws.json")

	cfgDir, err := xdgConfigDir()
	if err != nil {
		return "", err
	}
	oldPath := filepath.Join(cfgDir, programDataName, "draws.json")

	if err := migrateIfNeeded(oldPath, newPath); err != nil {
		fmt.Fprintf(os.Stderr, "%s: migration warning: %v\n", programDataName, err)
	}

	if err := os.MkdirAll(newDir, 0755); err != nil {
		return "", err
	}
	return newPath, nil
}

// migrateIfNeeded moves oldPath to newPath when oldPath is a regular file and
// newPath does not exist. Symlinks at oldPath are preserved with a warning.
// When both paths exist, the new path is preferred and a warning is emitted.
func migrateIfNeeded(oldPath, newPath string) error {
	oldInfo, oldErr := os.Lstat(oldPath)
	_, newErr := os.Stat(newPath)
	newExists := newErr == nil

	if oldErr != nil {
		return nil // nothing to migrate
	}

	if oldInfo.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintf(os.Stderr, "%s: %s is a symlink; skipping auto-migration. Move it to %s manually.\n", programDataName, oldPath, newPath)
		return nil
	}

	if newExists {
		fmt.Fprintf(os.Stderr, "%s: both %s and %s exist; using %s. Delete the old file when ready.\n", programDataName, oldPath, newPath, newPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		// Cross-device rename (EXDEV) — fall back to copy + delete.
		if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err == syscall.EXDEV {
			if err := copyFile(oldPath, newPath); err != nil {
				return fmt.Errorf("cross-device migration copy: %w", err)
			}
			if err := os.Remove(oldPath); err != nil {
				return fmt.Errorf("cross-device migration cleanup: %w", err)
			}
		} else {
			return fmt.Errorf("rename: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "%s: migrated %s -> %s\n", programDataName, oldPath, newPath)
	return nil
}

// copyFile copies src to dst (used as EXDEV fallback for os.Rename).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func loadDraws() ([]Draw, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Draw{}, nil
		}
		return nil, err
	}
	var draws []Draw
	decodeErr := json.NewDecoder(file).Decode(&draws)
	_ = file.Close()
	if decodeErr != nil {
		return draws, decodeErr
	}

	pruned, removed := pruneLegacyEra(draws)
	if removed > 0 {
		if err := atomicWriteDraws(path, pruned); err != nil {
			fmt.Fprintf(os.Stderr, "%s: prune rewrite failed: %v\n", programDataName, err)
			return pruned, nil
		}
		fmt.Fprintf(os.Stderr, "%s: pruned %d pre-%s rows from %s\n", programDataName, removed, cash5EraStartDate, path)
	}
	return pruned, nil
}

// pruneLegacyEra filters out draws with DrawTime before cash5EraStartMillis,
// which predate the 1-45 pool and are not retained. It returns the
// post-cutoff slice and the count removed. Input order is preserved.
func pruneLegacyEra(draws []Draw) ([]Draw, int) {
	if len(draws) == 0 {
		return draws, 0
	}
	kept := make([]Draw, 0, len(draws))
	removed := 0
	for _, d := range draws {
		if d.DrawTime >= cash5EraStartMillis {
			kept = append(kept, d)
		} else {
			removed++
		}
	}
	return kept, removed
}

// atomicWriteDraws writes draws to path via temp file + rename. The temp file
// is created in the same directory as path so the rename is atomic on the same
// filesystem; on failure the temp file is removed.
func atomicWriteDraws(path string, draws []Draw) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "draws-*.json.tmp")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp.Name())
		}
	}()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(draws); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
