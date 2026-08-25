package frigate

import (
	"context"
	"fmt"
)

const defaultDuration = 30

// Actuator implements engine.Actuator for Frigate by wrapping a Client.
type Actuator struct {
	client *Client
}

// NewActuator constructs an Actuator around an already-authenticated Client.
func NewActuator(client *Client) *Actuator {
	return &Actuator{client: client}
}

// Execute creates a manual Frigate event as described by action, which must
// be the *ActionConfig produced by DecodeAction.
func (a *Actuator) Execute(_ context.Context, action any) error {
	cfg, ok := action.(*ActionConfig)
	if !ok {
		return fmt.Errorf("frigate actuator: unexpected config type %T", action)
	}

	duration := cfg.Duration
	if duration == 0 {
		duration = defaultDuration
	}

	params := CreateEventParams{
		SourceType:       "api",
		SubLabel:         cfg.SubLabel,
		Score:            1,
		Duration:         &duration,
		IncludeRecording: cfg.IncludeRecording,
	}
	if cfg.PreCapture > 0 {
		params.PreCapture = &cfg.PreCapture
	}
	if len(cfg.BoundingBox) == 4 {
		bb := cfg.BoundingBox
		params.Draw = &EventDraw{
			Boxes: []EventBox{{Box: [4]float64{bb[0], bb[1], bb[2], bb[3]}}},
		}
	}

	return a.client.CreateEvent(cfg.Device, cfg.Label, params)
}
