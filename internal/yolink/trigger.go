package yolink

import (
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/tyandl/yolink-api/v2/pkg/controller"
	"github.com/tyandl/yolink-api/v2/pkg/devices"
	"github.com/tyandl/yolink-api/v2/pkg/types"
)

// TriggerConfig is the yolink-specific portion of a [rules.trigger] table.
type TriggerConfig struct {
	// Device is the exact device name as it appears in the YoLink app.
	Device string `toml:"device"`
	// Event is a canonical event name from the table below (e.g. "door.opened").
	Event string `toml:"event"`
}

// eventPredicates maps a canonical event name to a test of whether a given
// device report represents that event firing. This is the single place that
// knows how YoLink's per-device-type alert reports map onto homelink's
// generic event vocabulary.
var eventPredicates = map[string]func(controller.Report) bool{
	"door.opened": func(r controller.Report) bool {
		alert, ok := r.Data.(devices.DoorSensorAlertResponse)
		return ok && r.Event == "DoorSensor.Alert" && alert.State == types.DoorStateOpen
	},
	"door.closed": func(r controller.Report) bool {
		alert, ok := r.Data.(devices.DoorSensorAlertResponse)
		return ok && r.Event == "DoorSensor.Alert" && alert.State == types.DoorStateClosed
	},
	"leak.detected": func(r controller.Report) bool {
		alert, ok := r.Data.(devices.LeakSensorAlertResponse)
		return ok && r.Event == "LeakSensor.Alert" && alert.State == types.AlarmStateAlert
	},
	"leak.dry": func(r controller.Report) bool {
		alert, ok := r.Data.(devices.LeakSensorAlertResponse)
		return ok && r.Event == "LeakSensor.Alert" && alert.State == types.AlarmStateNormal
	},
	"lock.unlocked": func(r controller.Report) bool {
		if alert, ok := r.Data.(devices.LockAlertResponse); ok && r.Event == "Lock.Alert" {
			return alert.State == types.LockStateUnlocked
		}
		if alert, ok := r.Data.(devices.LockV2AlertResponse); ok && r.Event == "LockV2.Alert" {
			return alert.Lock == types.LockStateUnlocked
		}
		return false
	},
	"lock.locked": func(r controller.Report) bool {
		if alert, ok := r.Data.(devices.LockAlertResponse); ok && r.Event == "Lock.Alert" {
			return alert.State == types.LockStateLocked
		}
		if alert, ok := r.Data.(devices.LockV2AlertResponse); ok && r.Event == "LockV2.Alert" {
			return alert.Lock == types.LockStateLocked
		}
		return false
	},
	"motion.detected": func(r controller.Report) bool {
		alert, ok := r.Data.(devices.MotionSensorAlertResponse)
		return ok && r.Event == "MotionSensor.Alert" && alert.State == types.AlertStateAlert
	},
	"motion.clear": func(r controller.Report) bool {
		alert, ok := r.Data.(devices.MotionSensorAlertResponse)
		return ok && r.Event == "MotionSensor.Alert" && alert.State == types.AlertStateNormal
	},
}

// DecodeTrigger decodes a [rules.trigger] primitive into a TriggerConfig and
// validates it.
func DecodeTrigger(meta toml.MetaData, prim toml.Primitive) (TriggerConfig, error) {
	var tc TriggerConfig
	if err := meta.PrimitiveDecode(prim, &tc); err != nil {
		return TriggerConfig{}, fmt.Errorf("yolink trigger: %w", err)
	}
	if tc.Device == "" {
		return TriggerConfig{}, fmt.Errorf("yolink trigger: device is required")
	}
	if _, ok := eventPredicates[tc.Event]; !ok {
		return TriggerConfig{}, fmt.Errorf("yolink trigger: unknown event %q (want one of door.opened, door.closed, leak.detected, leak.dry, lock.unlocked, lock.locked, motion.detected, motion.clear)", tc.Event)
	}
	return tc, nil
}
