package timer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// persistedTimer is the on-disk representation of one armed countdown: its name and the
// absolute instant it expires at. Storing an absolute instant rather than a remaining duration
// means resuming after a restart is a plain comparison against the current time -- no need to
// separately track how long the process was down.
type persistedTimer struct {
	Name   string    `json:"name"`
	Expiry time.Time `json:"expiry"`
}

// loadPersisted reads the persisted timer state from path, returning an empty slice (not an
// error) if the file doesn't exist yet -- the normal case on first startup, or whenever
// persistence hasn't written anything yet.
func loadPersisted(path string) ([]persistedTimer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var timers []persistedTimer
	if err := json.Unmarshal(data, &timers); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return timers, nil
}

// savePersisted atomically overwrites path with timers: written to a temp file in the same
// directory, then renamed over the real path, so a crash mid-write never leaves a half-written
// (and therefore unparsable) file behind for the next startup to trip over.
func savePersisted(path string, timers []persistedTimer) error {
	data, err := json.Marshal(timers)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once Rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
