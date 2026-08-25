package timer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tyandl/homelink/internal/engine"
)

// eventBuffer is how many pending expiry/schedule events Service will hold
// before a slow consumer causes it to start dropping them (with a warning)
// rather than blocking the goroutine that produced them. A production
// config with dozens of simultaneously-firing timers is already an unusual
// shape; this just bounds the worst case instead of risking a deadlock.
const eventBuffer = 16

// Service implements both engine.Source and engine.Actuator for the timer
// service. It owns an in-process registry of named countdowns — armed and
// cancelled by "set"/"cancel" actions, watched by "expired" triggers — plus
// a set of fixed daily schedules for "time_of_day" triggers, established
// once at construction. Source and Actuator share this one instance because
// actions need to reach the same countdown state the source watches.
type Service struct {
	events chan engine.Event

	mu     sync.Mutex
	timers map[string]*time.Timer

	schedules []TriggerConfig
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewService constructs a Service. schedules should be the time_of_day
// TriggerConfig entries actually referenced by rules (expired triggers need
// no upfront registration — any timer armed by a "set" action can be
// watched by name). Entries with the same Device and At are deduplicated to
// a single underlying schedule, so N rules sharing one schedule fire once
// each rather than N times each.
func NewService(schedules []TriggerConfig) *Service {
	return &Service{
		events:    make(chan engine.Event, eventBuffer),
		timers:    make(map[string]*time.Timer),
		schedules: dedupeSchedules(schedules),
	}
}

func dedupeSchedules(schedules []TriggerConfig) []TriggerConfig {
	seen := make(map[string]bool, len(schedules))
	var out []TriggerConfig
	for _, s := range schedules {
		key := s.Device + "\x00" + s.At
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// Start begins one goroutine per deduplicated daily schedule. Set and
// Cancel work regardless of whether Start has been called; expiry events
// are only useful once something is consuming the returned channel.
func (s *Service) Start(ctx context.Context) (<-chan engine.Event, error) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	for _, schedule := range s.schedules {
		s.wg.Add(1)
		go s.runSchedule(ctx, schedule)
	}

	return s.events, nil
}

// Stop halts the daily schedule goroutines. Pending named countdowns are
// left running — they're action-armed, independent of this source's
// lifecycle, and the process exiting is what actually ends them.
func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Service) runSchedule(ctx context.Context, schedule TriggerConfig) {
	defer s.wg.Done()

	at, err := time.Parse(timeOfDayLayout, schedule.At)
	if err != nil {
		// Unreachable: validated in DecodeTrigger.
		slog.Error("timer: invalid schedule, not starting", "device", schedule.Device, "at", schedule.At, "error", err)
		return
	}

	slog.Info("timer: watching schedule", "device", schedule.Device, "at", schedule.At)

	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), at.Hour(), at.Minute(), 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}

		timer := time.NewTimer(next.Sub(now))
		select {
		case <-timer.C:
			s.emit(engine.Event{Service: "timer", Device: schedule.Device, Name: "time_of_day", Time: time.Now()})
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

// Set (re)arms the named timer for duration, replacing any existing
// countdown under the same name.
func (s *Service) Set(name string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.timers[name]; ok {
		existing.Stop()
	}
	s.timers[name] = time.AfterFunc(duration, func() {
		s.mu.Lock()
		delete(s.timers, name)
		s.mu.Unlock()
		s.emit(engine.Event{Service: "timer", Device: name, Name: "expired", Time: time.Now()})
	})
}

// Cancel stops the named timer if one is running. Cancelling an unknown or
// already-fired timer is a no-op.
func (s *Service) Cancel(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.timers[name]; ok {
		existing.Stop()
		delete(s.timers, name)
	}
}

func (s *Service) emit(event engine.Event) {
	select {
	case s.events <- event:
	default:
		slog.Warn("timer: event dropped, no rule is consuming timer events fast enough", "device", event.Device, "event", event.Name)
	}
}

// Execute performs the action described by action, which must be the
// *ActionConfig produced by DecodeAction.
func (s *Service) Execute(_ context.Context, action any) error {
	cfg, ok := action.(*ActionConfig)
	if !ok {
		return fmt.Errorf("timer actuator: unexpected config type %T", action)
	}

	switch cfg.Action {
	case "set":
		duration, err := time.ParseDuration(cfg.Duration)
		if err != nil {
			// Unreachable: validated in DecodeAction.
			return fmt.Errorf("timer actuator: invalid duration %q: %w", cfg.Duration, err)
		}
		s.Set(cfg.Device, duration)
		slog.Info("timer: set", "device", cfg.Device, "duration", duration)
	case "cancel":
		s.Cancel(cfg.Device)
		slog.Info("timer: cancelled", "device", cfg.Device)
	default:
		// Unreachable: validated in DecodeAction.
		return fmt.Errorf("timer actuator: unsupported action %q", cfg.Action)
	}
	return nil
}
