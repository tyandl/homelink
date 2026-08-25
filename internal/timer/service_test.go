package timer

import (
	"context"
	"testing"
	"time"

	"github.com/tyandl/homelink/internal/engine"
)

func TestSetFiresExpiredEvent(t *testing.T) {
	svc := NewService(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	svc.Set("wash cycle", 10*time.Millisecond)

	select {
	case event := <-events:
		if event.Service != "timer" || event.Device != "wash cycle" || event.Name != "expired" {
			t.Errorf("unexpected event: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for expired event")
	}
}

func TestSetResetsExistingTimer(t *testing.T) {
	svc := NewService(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	svc.Set("wash cycle", 50*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	svc.Set("wash cycle", 200*time.Millisecond) // should push the fire time out, not stack

	start := time.Now()
	select {
	case <-events:
		elapsed := time.Since(start)
		if elapsed < 150*time.Millisecond {
			t.Errorf("expired too early (%v), reset did not take effect", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for expired event")
	}
}

func TestCancelPreventsExpiry(t *testing.T) {
	svc := NewService(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	svc.Set("wash cycle", 20*time.Millisecond)
	svc.Cancel("wash cycle")

	select {
	case event := <-events:
		t.Fatalf("expected no event after cancel, got %+v", event)
	case <-time.After(150 * time.Millisecond):
		// success: nothing fired
	}
}

func TestCancelUnknownTimerIsNoop(t *testing.T) {
	svc := NewService(nil)
	svc.Cancel("never set") // must not panic
}

func TestExecuteSetAndCancel(t *testing.T) {
	svc := NewService(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := svc.Execute(context.Background(), &ActionConfig{Device: "reminder", Action: "set", Duration: "10ms"}); err != nil {
		t.Fatalf("Execute set: %v", err)
	}

	select {
	case event := <-events:
		if event.Device != "reminder" || event.Name != "expired" {
			t.Errorf("unexpected event: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for expired event")
	}

	if err := svc.Execute(context.Background(), &ActionConfig{Device: "reminder", Action: "set", Duration: "200ms"}); err != nil {
		t.Fatalf("Execute set: %v", err)
	}
	if err := svc.Execute(context.Background(), &ActionConfig{Device: "reminder", Action: "cancel"}); err != nil {
		t.Fatalf("Execute cancel: %v", err)
	}

	select {
	case event := <-events:
		t.Fatalf("expected no event after cancel, got %+v", event)
	case <-time.After(400 * time.Millisecond):
		// success
	}
}

func TestExecuteRejectsWrongConfigType(t *testing.T) {
	svc := NewService(nil)
	if err := svc.Execute(context.Background(), "not an ActionConfig"); err == nil {
		t.Fatal("expected error for wrong config type, got nil")
	}
}

func TestDedupeSchedulesBySameDeviceAndAt(t *testing.T) {
	schedules := []TriggerConfig{
		{Device: "afternoon", Event: "time_of_day", At: "16:00"},
		{Device: "afternoon", Event: "time_of_day", At: "16:00"}, // duplicate: two rules, same schedule
		{Device: "evening", Event: "time_of_day", At: "20:00"},
	}
	svc := NewService(schedules)
	if len(svc.schedules) != 2 {
		t.Fatalf("got %d deduplicated schedules, want 2: %+v", len(svc.schedules), svc.schedules)
	}
}

func TestStartStopSchedulesCleanShutdown(t *testing.T) {
	// A schedule far in the future should never fire during this test; this
	// just verifies Start/Stop don't hang or panic.
	svc := NewService([]TriggerConfig{{Device: "far", Event: "time_of_day", At: "03:33"}})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := svc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	cancel()

	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return promptly after ctx cancellation")
	}
}

var _ engine.Source = (*Service)(nil)
var _ engine.Actuator = (*Service)(nil)
