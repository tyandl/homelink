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

// armedTimer pairs a running countdown with the absolute instant it expires at. time.Timer
// itself exposes no way to ask "when will this fire" or "how much time is left" -- expiry is
// tracked alongside it purely so persist can save something restorable after a restart.
type armedTimer struct {
	timer  *time.Timer
	expiry time.Time
}

// Service implements both engine.Source and engine.Actuator for the timer
// service. It owns an in-process registry of named countdowns — armed and
// cancelled by "set"/"cancel" actions, watched by "expired" triggers — plus
// a set of fixed daily schedules for "time_of_day" triggers, established
// once at construction. Source and Actuator share this one instance because
// actions need to reach the same countdown state the source watches.
type Service struct {
	events chan engine.Event

	mu     sync.Mutex
	timers map[string]*armedTimer

	// persistPath is where armed countdowns are saved so they survive a restart (see
	// persist.go). Empty disables persistence entirely -- Set/Cancel/expiry work exactly as
	// before, just without surviving a restart.
	persistPath string

	schedules []TriggerConfig
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewService constructs a Service, resuming any countdowns found at persistPath (empty
// disables persistence) — see resume for exactly how. schedules should be the time_of_day
// TriggerConfig entries actually referenced by rules (expired triggers need no upfront
// registration — any timer armed by a "set" action can be watched by name). Entries with the
// same Device and At are deduplicated to a single underlying schedule, so N rules sharing one
// schedule fire once each rather than N times each.
func NewService(schedules []TriggerConfig, persistPath string) *Service {
	s := &Service{
		events:      make(chan engine.Event, eventBuffer),
		timers:      make(map[string]*armedTimer),
		persistPath: persistPath,
		schedules:   dedupeSchedules(schedules),
	}
	s.resume()
	return s
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
		existing.timer.Stop()
	}
	s.timers[name] = &armedTimer{
		expiry: time.Now().Add(duration),
		timer: time.AfterFunc(duration, func() {
			s.mu.Lock()
			delete(s.timers, name)
			s.persist()
			s.mu.Unlock()
			s.emit(engine.Event{Service: "timer", Device: name, Name: "expired", Time: time.Now()})
		}),
	}
	s.persist()
}

// Cancel stops the named timer if one is running. Cancelling an unknown or
// already-fired timer is a no-op.
func (s *Service) Cancel(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.timers[name]; ok {
		existing.timer.Stop()
		delete(s.timers, name)
		s.persist()
	}
}

// resume loads any countdowns persisted from a previous run and either re-arms or catches up
// each one. A countdown still in the future is re-armed for its remaining duration --
// indistinguishable from the outside from one that had been running the whole time. A
// countdown whose expiry already passed while the process was down is treated as expired
// immediately: every timer in this codebase represents a bounded "do X after some inactivity"
// window (e.g. garage_lights_off), so firing late is the safer default -- it still turns
// something off, rather than leaving it in whatever the mid-timer state was until some
// unrelated later trigger happens to reset it. A no-op if persistence is disabled or the file
// doesn't exist yet (both handled by loadPersisted returning an empty slice).
func (s *Service) resume() {
	if s.persistPath == "" {
		return
	}
	persisted, err := loadPersisted(s.persistPath)
	if err != nil {
		slog.Error("timer: failed to load persisted state, starting with no resumed timers", "path", s.persistPath, "error", err)
		return
	}

	now := time.Now()
	for _, entry := range persisted {
		if remaining := entry.Expiry.Sub(now); remaining > 0 {
			s.Set(entry.Name, remaining)
			slog.Info("timer: resumed", "device", entry.Name, "remaining", remaining.Round(time.Second))
		} else {
			slog.Info("timer: resumed timer had already expired while stopped, firing now", "device", entry.Name, "expired_at", entry.Expiry)
			s.emit(engine.Event{Service: "timer", Device: entry.Name, Name: "expired", Time: time.Now()})
		}
	}

	// Rewrite the file even if nothing was re-armed, so already-expired entries (handled
	// above by firing immediately, not by re-adding to s.timers) don't linger and get
	// mistaken for still-pending countdowns on some later restart.
	s.mu.Lock()
	s.persist()
	s.mu.Unlock()
}

// persist rewrites the persistence file with the current timer state. Caller must hold s.mu.
// A write failure is logged, not returned: it shouldn't fail whatever Set/Cancel/expiry
// triggered it, and the timer keeps working correctly in-memory for this run regardless --
// it just won't survive a restart until the next successful write.
func (s *Service) persist() {
	if s.persistPath == "" {
		return
	}
	entries := make([]persistedTimer, 0, len(s.timers))
	for name, armed := range s.timers {
		entries = append(entries, persistedTimer{Name: name, Expiry: armed.expiry})
	}
	if err := savePersisted(s.persistPath, entries); err != nil {
		slog.Error("timer: failed to persist state", "path", s.persistPath, "error", err)
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
