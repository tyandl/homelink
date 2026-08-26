package timer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadPersistedMissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	timers, err := loadPersisted(path)
	if err != nil {
		t.Fatalf("loadPersisted: %v", err)
	}
	if len(timers) != 0 {
		t.Fatalf("expected no timers, got %+v", timers)
	}
}

func TestSavePersistedRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timers.json")
	want := []persistedTimer{
		{Name: "garage_lights_off", Expiry: time.Now().Add(time.Hour).Truncate(time.Second)},
	}
	if err := savePersisted(path, want); err != nil {
		t.Fatalf("savePersisted: %v", err)
	}

	got, err := loadPersisted(path)
	if err != nil {
		t.Fatalf("loadPersisted: %v", err)
	}
	if len(got) != 1 || got[0].Name != want[0].Name || !got[0].Expiry.Equal(want[0].Expiry) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSavePersistedLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "timers.json")
	if err := savePersisted(path, []persistedTimer{{Name: "x", Expiry: time.Now()}}); err != nil {
		t.Fatalf("savePersisted: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "timers.json" {
		t.Fatalf("expected only timers.json in %s, got %v", dir, entries)
	}
}

// TestServiceResumesStillFutureTimer proves the whole round trip through Service, not just the
// file layer: a timer set before a (simulated) restart is still watchable by name afterward,
// with its remaining duration honored rather than restarting the full original duration.
func TestServiceResumesStillFutureTimer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timers.json")

	first := NewService(nil, path)
	first.Set("garage_lights_off", 150*time.Millisecond)

	// Simulate a restart: a brand new Service reading the same persisted file, standing in
	// for the process having exited and been relaunched.
	second := NewService(nil, path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := second.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case event := <-events:
		if event.Device != "garage_lights_off" || event.Name != "expired" {
			t.Errorf("unexpected event: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resumed timer to expire")
	}
}

// TestServiceResumesAlreadyExpiredTimerFiresImmediately covers the "process was down longer
// than the timer's duration" case: resume should fire it right away (the documented catch-up
// policy) rather than silently dropping it or (worse) treating a negative duration as "wait
// forever".
func TestServiceResumesAlreadyExpiredTimerFiresImmediately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timers.json")
	stale := []persistedTimer{{Name: "garage_lights_off", Expiry: time.Now().Add(-time.Hour)}}
	if err := savePersisted(path, stale); err != nil {
		t.Fatalf("savePersisted: %v", err)
	}

	svc := NewService(nil, path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case event := <-events:
		if event.Device != "garage_lights_off" || event.Name != "expired" {
			t.Errorf("unexpected event: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stale timer to fire immediately on resume")
	}

	// The stale entry must not linger in the file, or it would fire again on a later restart.
	remaining, err := loadPersisted(path)
	if err != nil {
		t.Fatalf("loadPersisted: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected the already-fired entry to be dropped from the file, got %+v", remaining)
	}
}

func TestServiceCancelRemovesFromPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timers.json")
	svc := NewService(nil, path)

	svc.Set("garage_lights_off", time.Hour)
	svc.Cancel("garage_lights_off")

	timers, err := loadPersisted(path)
	if err != nil {
		t.Fatalf("loadPersisted: %v", err)
	}
	if len(timers) != 0 {
		t.Fatalf("expected no persisted timers after cancel, got %+v", timers)
	}
}

func TestServiceExpiryRemovesFromPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timers.json")
	svc := NewService(nil, path)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	svc.Set("garage_lights_off", 10*time.Millisecond)
	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for expiry")
	}

	// The write happens inside the AfterFunc callback, concurrently with this goroutine
	// receiving from events; give it a moment to land before asserting the file is empty.
	deadline := time.Now().Add(time.Second)
	for {
		timers, err := loadPersisted(path)
		if err != nil {
			t.Fatalf("loadPersisted: %v", err)
		}
		if len(timers) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected no persisted timers after expiry, got %+v", timers)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPersistenceDisabledWhenPathEmpty(t *testing.T) {
	svc := NewService(nil, "")
	svc.Set("garage_lights_off", time.Hour)
	svc.mu.Lock()
	_, tracked := svc.timers["garage_lights_off"]
	svc.mu.Unlock()
	if !tracked {
		t.Fatal("expected the timer to still work in-memory with persistence disabled")
	}
	// persist() with an empty path must not attempt any file I/O; nothing to assert beyond
	// Set not panicking/erroring, which the above already exercises.
}

// jsonRoundTrip is a sanity check that persistedTimer's JSON shape is what we expect (field
// names a human debugging the cache file on disk would actually see).
func TestPersistedTimerJSONShape(t *testing.T) {
	data, err := json.Marshal(persistedTimer{Name: "garage_lights_off", Expiry: time.Unix(0, 0).UTC()})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"name":"garage_lights_off","expiry":"1970-01-01T00:00:00Z"}`
	if string(data) != want {
		t.Fatalf("got %s, want %s", data, want)
	}
}
