package timer

import (
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
)

// ActionConfig is the timer-specific portion of a [[rules.actions]] entry.
type ActionConfig struct {
	// Device is the timer's name.
	Device string `toml:"device"`
	// Action is "set" or "cancel".
	Action string `toml:"action"`
	// Duration is required for "set", in Go duration syntax (e.g. "5m", "1h30m").
	Duration string `toml:"duration"`
}

// DecodeAction decodes a [[rules.actions]] primitive into an ActionConfig
// and validates it.
func DecodeAction(meta toml.MetaData, prim toml.Primitive) (*ActionConfig, error) {
	var ac ActionConfig
	if err := meta.PrimitiveDecode(prim, &ac); err != nil {
		return nil, fmt.Errorf("timer action: %w", err)
	}
	if ac.Device == "" {
		return nil, fmt.Errorf("timer action: device (timer name) is required")
	}

	switch ac.Action {
	case "set":
		if ac.Duration == "" {
			return nil, fmt.Errorf("timer action: set requires duration")
		}
		duration, err := time.ParseDuration(ac.Duration)
		if err != nil {
			return nil, fmt.Errorf("timer action: invalid duration %q: %w", ac.Duration, err)
		}
		if duration <= 0 {
			return nil, fmt.Errorf("timer action: duration must be positive, got %q", ac.Duration)
		}
	case "cancel":
		if ac.Duration != "" {
			return nil, fmt.Errorf("timer action: duration is not valid with action %q", ac.Action)
		}
	default:
		return nil, fmt.Errorf("timer action: unsupported action %q (want set or cancel)", ac.Action)
	}

	return &ac, nil
}
