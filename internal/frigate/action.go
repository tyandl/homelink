package frigate

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// ActionConfig is the frigate-specific portion of a [[rules.actions]] entry.
type ActionConfig struct {
	// Device is the Frigate camera name.
	Device string `toml:"device"`
	// Action selects what to do. Only "create_event" is currently supported.
	Action string `toml:"action"`

	// Label is the Frigate event label (e.g. "mail", "person").
	Label string `toml:"label"`
	// SubLabel is an optional secondary label.
	SubLabel string `toml:"sub_label"`
	// Duration is how long the Frigate event stays open, in seconds. 0 uses the default (30s).
	Duration int `toml:"duration"`
	// BoundingBox is an optional [x, y, dx, dy] region in the camera frame where
	// x,y is the top-left corner and dx,dy is the width and height, all as
	// fractions of the frame dimensions (0.0-1.0).
	BoundingBox []float64 `toml:"bounding_box"`
	// PreCapture is seconds of footage before the trigger to include in the recording.
	// Omitted when zero, which lets Frigate apply its own default.
	PreCapture int `toml:"pre_capture"`
	// IncludeRecording controls whether a recording clip is attached. Default true.
	IncludeRecording *bool `toml:"include_recording"`
}

// DecodeAction decodes a [[rules.actions]] primitive into an ActionConfig
// and validates it.
func DecodeAction(meta toml.MetaData, prim toml.Primitive) (*ActionConfig, error) {
	var ac ActionConfig
	if err := meta.PrimitiveDecode(prim, &ac); err != nil {
		return nil, fmt.Errorf("frigate action: %w", err)
	}
	if ac.Device == "" {
		return nil, fmt.Errorf("frigate action: device (camera) is required")
	}
	if ac.Action != "create_event" {
		return nil, fmt.Errorf("frigate action: unsupported action %q (only create_event is supported)", ac.Action)
	}
	if ac.Label == "" {
		return nil, fmt.Errorf("frigate action: label is required")
	}
	if len(ac.BoundingBox) != 0 && len(ac.BoundingBox) != 4 {
		return nil, fmt.Errorf("frigate action: bounding_box must have exactly 4 values [x, y, dx, dy]")
	}
	return &ac, nil
}
