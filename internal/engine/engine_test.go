package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeSource struct {
	events  chan Event
	stopped bool
}

func (f *fakeSource) Start(ctx context.Context) (<-chan Event, error) {
	go func() {
		<-ctx.Done()
		close(f.events)
	}()
	return f.events, nil
}

func (f *fakeSource) Stop() {
	f.stopped = true
}

type recordingActuator struct {
	mu   sync.Mutex
	got  []any
	fail bool
}

func (r *recordingActuator) Execute(_ context.Context, action any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("boom")
	}
	r.got = append(r.got, action)
	return nil
}

func (r *recordingActuator) calls() []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]any(nil), r.got...)
}

func TestEngineDispatchesMatchingRuleToAllActions(t *testing.T) {
	source := &fakeSource{events: make(chan Event)}
	frigateActuator := &recordingActuator{}
	kasaActuator := &recordingActuator{}

	rules := []Rule{{
		Name:    "mailbox opened",
		Service: "yolink",
		Device:  "Mailbox Sensor",
		Event:   "door.opened",
		Actions: []RuleAction{
			{Service: "frigate", Config: "frigate-action"},
			{Service: "kasa", Config: "kasa-action"},
		},
	}}

	eng := New(rules, map[string]Source{"yolink": source}, map[string]Actuator{
		"frigate": frigateActuator,
		"kasa":    kasaActuator,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	source.events <- Event{Service: "yolink", Device: "Mailbox Sensor", Name: "door.opened", Time: time.Now()}
	// Non-matching event (wrong device) should not trigger anything.
	source.events <- Event{Service: "yolink", Device: "Other Sensor", Name: "door.opened", Time: time.Now()}

	deadline := time.After(2 * time.Second)
	for {
		if len(frigateActuator.calls()) == 1 && len(kasaActuator.calls()) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for dispatch: frigate=%v kasa=%v", frigateActuator.calls(), kasaActuator.calls())
		case <-time.After(10 * time.Millisecond):
		}
	}

	if got := frigateActuator.calls(); len(got) != 1 || got[0] != "frigate-action" {
		t.Errorf("frigate actuator calls = %v", got)
	}
	if got := kasaActuator.calls(); len(got) != 1 || got[0] != "kasa-action" {
		t.Errorf("kasa actuator calls = %v", got)
	}

	eng.Stop()
	if !source.stopped {
		t.Errorf("expected source.Stop to have been called")
	}
}

func TestEngineActionFailureDoesNotBlockOtherActions(t *testing.T) {
	source := &fakeSource{events: make(chan Event)}
	failing := &recordingActuator{fail: true}
	succeeding := &recordingActuator{}

	rules := []Rule{{
		Service: "yolink",
		Device:  "D",
		Event:   "leak.detected",
		Actions: []RuleAction{
			{Service: "failing", Config: 1},
			{Service: "succeeding", Config: 2},
		},
	}}

	eng := New(rules, map[string]Source{"yolink": source}, map[string]Actuator{
		"failing":    failing,
		"succeeding": succeeding,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	source.events <- Event{Service: "yolink", Device: "D", Name: "leak.detected", Time: time.Now()}

	deadline := time.After(2 * time.Second)
	for len(succeeding.calls()) == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for succeeding actuator to run despite failing actuator")
		case <-time.After(10 * time.Millisecond):
		}
	}

	eng.Stop()
}
