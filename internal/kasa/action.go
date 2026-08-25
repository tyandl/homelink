package kasa

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// ActionConfig is the kasa-specific portion of a [[rules.actions]] entry.
type ActionConfig struct {
	// Device is the device's alias, resolved against [[kasa.devices]].
	Device string `toml:"device"`
	// Action selects what to do: turn_on, turn_off, toggle, set_brightness,
	// led_on, or led_off.
	Action string `toml:"action"`
	// Brightness is required for set_brightness, 0-100.
	Brightness *int `toml:"brightness"`
}

var validActions = map[string]bool{
	"turn_on":        true,
	"turn_off":       true,
	"toggle":         true,
	"set_brightness": true,
	"led_on":         true,
	"led_off":        true,
}

// DecodeAction decodes a [[rules.actions]] primitive into an ActionConfig
// and validates it.
func DecodeAction(meta toml.MetaData, prim toml.Primitive) (*ActionConfig, error) {
	var ac ActionConfig
	if err := meta.PrimitiveDecode(prim, &ac); err != nil {
		return nil, fmt.Errorf("kasa action: %w", err)
	}
	if ac.Device == "" {
		return nil, fmt.Errorf("kasa action: device (alias) is required")
	}
	if !validActions[ac.Action] {
		return nil, fmt.Errorf("kasa action: unsupported action %q (want turn_on, turn_off, toggle, set_brightness, led_on, or led_off)", ac.Action)
	}
	if ac.Action == "set_brightness" {
		if ac.Brightness == nil {
			return nil, fmt.Errorf("kasa action: set_brightness requires brightness")
		}
		if *ac.Brightness < 0 || *ac.Brightness > 100 {
			return nil, fmt.Errorf("kasa action: brightness must be 0-100, got %d", *ac.Brightness)
		}
	}
	return &ac, nil
}
