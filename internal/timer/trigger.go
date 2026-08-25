// Package timer implements engine.Source and engine.Actuator for named
// countdown timers and fixed daily schedules. Unlike the other services,
// timer has no external connection — it is purely an in-process registry,
// so its Source and Actuator share one Service instance rather than being
// independent.
package timer

import (
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
)

// timeOfDayLayout is the accepted format for TriggerConfig.At: 24-hour
// "HH:MM", interpreted in the process's local timezone (set TZ).
const timeOfDayLayout = "15:04"

// TriggerConfig is the timer-specific portion of a [rules.trigger] table.
type TriggerConfig struct {
	// Device names the trigger: for event = "expired" it is the timer name
	// set via a timer "set" action; for event = "time_of_day" it is a
	// label identifying that schedule. Rules that share the same Device
	// (and, for time_of_day, the same At) fire together off one underlying
	// timer rather than each getting a redundant one.
	Device string `toml:"device"`
	// Event is "expired" or "time_of_day".
	Event string `toml:"event"`
	// At is the local wall-clock time "HH:MM" (24-hour), required for and
	// only valid with event = "time_of_day".
	At string `toml:"at"`
}

// DecodeTrigger decodes a [rules.trigger] primitive into a TriggerConfig
// and validates it.
func DecodeTrigger(meta toml.MetaData, prim toml.Primitive) (TriggerConfig, error) {
	var tc TriggerConfig
	if err := meta.PrimitiveDecode(prim, &tc); err != nil {
		return TriggerConfig{}, fmt.Errorf("timer trigger: %w", err)
	}
	if tc.Device == "" {
		return TriggerConfig{}, fmt.Errorf("timer trigger: device is required")
	}

	switch tc.Event {
	case "expired":
		if tc.At != "" {
			return TriggerConfig{}, fmt.Errorf("timer trigger: at is not valid with event %q", tc.Event)
		}
	case "time_of_day":
		if tc.At == "" {
			return TriggerConfig{}, fmt.Errorf("timer trigger: at is required with event %q (e.g. \"16:00\")", tc.Event)
		}
		if _, err := time.Parse(timeOfDayLayout, tc.At); err != nil {
			return TriggerConfig{}, fmt.Errorf("timer trigger: at must be in HH:MM 24-hour format, got %q", tc.At)
		}
	default:
		return TriggerConfig{}, fmt.Errorf("timer trigger: unknown event %q (want expired or time_of_day)", tc.Event)
	}

	return tc, nil
}
