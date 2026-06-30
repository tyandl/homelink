package watcher

import (
	"log/slog"

	"github.com/tyandl/homelink/internal/config"
	"github.com/tyandl/homelink/internal/frigate"
	"github.com/tyandl/yolink-api/pkg/controller"
	"github.com/tyandl/yolink-api/pkg/devices"
	"github.com/tyandl/yolink-api/pkg/types"
)

const defaultDuration = 30

// Watcher subscribes to YoLink devices and creates Frigate events when alerts fire.
type Watcher struct {
	mappings      []config.DeviceMapping
	home          *controller.Home
	frigateClient *frigate.Client
	stops         []func()
}

func New(mappings []config.DeviceMapping, home *controller.Home, frigateClient *frigate.Client) *Watcher {
	return &Watcher{
		mappings:      mappings,
		home:          home,
		frigateClient: frigateClient,
	}
}

// Start subscribes to every mapped device and begins routing alerts to Frigate.
// Devices that cannot be found or subscribed are logged and skipped.
func (w *Watcher) Start() {
	for _, mapping := range w.mappings {
		device, err := w.home.GetDeviceByName(mapping.YoLinkDevice)
		if err != nil {
			slog.Error("watcher: device not found", "name", mapping.YoLinkDevice)
			continue
		}

		reports, stop, err := device.Subscribe()
		if err != nil {
			slog.Error("watcher: subscribe failed", "name", mapping.YoLinkDevice, "error", err)
			continue
		}
		w.stops = append(w.stops, stop)

		slog.Info("watching device",
			"device", mapping.YoLinkDevice,
			"camera", mapping.Camera,
			"label", mapping.Label,
		)
		go w.handle(mapping, reports)
	}
}

// Stop unsubscribes all watched devices.
func (w *Watcher) Stop() {
	for _, stop := range w.stops {
		stop()
	}
}

func (w *Watcher) handle(mapping config.DeviceMapping, reports <-chan controller.Report) {
	for report := range reports {
		slog.Debug("device report", "device", mapping.YoLinkDevice, "event", report.Event)

		if !isTrigger(report) {
			continue
		}

		slog.Info("device alert — creating frigate event",
			"device", mapping.YoLinkDevice,
			"camera", mapping.Camera,
			"label", mapping.Label,
		)

		duration := mapping.Duration
		if duration == 0 {
			duration = defaultDuration
		}

		params := frigate.CreateEventParams{
			SourceType: "api",
			SubLabel:   mapping.SubLabel,
			Duration:   &duration,
		}
		if len(mapping.BoundingBox) == 4 {
			params.Draw = &frigate.EventDraw{
				Boxes: [][]float64{mapping.BoundingBox},
			}
		}

		if err := w.frigateClient.CreateEvent(mapping.Camera, mapping.Label, params); err != nil {
			slog.Error("watcher: frigate create event failed",
				"error", err,
				"device", mapping.YoLinkDevice,
				"camera", mapping.Camera,
				"label", mapping.Label,
			)
		}
	}
}

// isTrigger returns true when a report represents a device entering an alert state.
func isTrigger(report controller.Report) bool {
	switch report.Event {
	case "DoorSensor.Alert":
		alert, ok := report.Data.(devices.DoorSensorAlertResponse)
		return ok && alert.State == types.DoorStateOpen
	case "LeakSensor.Alert":
		alert, ok := report.Data.(devices.LeakSensorAlertResponse)
		return ok && alert.State == types.AlarmStateAlert
	case "Lock.Alert":
		alert, ok := report.Data.(devices.LockAlertResponse)
		return ok && alert.State == types.LockStateUnlocked
	case "LockV2.Alert":
		alert, ok := report.Data.(devices.LockV2AlertResponse)
		return ok && alert.Lock == types.LockStateUnlocked
	}
	return false
}
