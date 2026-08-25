package yolink

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/tyandl/homelink/internal/engine"
	"github.com/tyandl/yolink-api/v2/pkg/controller"
)

// Source implements engine.Source for YoLink. It only ever subscribes to
// the devices it was constructed with — devices referenced by no
// [rules.trigger] never get a subscription at all.
type Source struct {
	home     *controller.Home
	triggers []TriggerConfig

	mu    sync.Mutex
	stops []func()
}

// NewSource constructs a Source that will watch exactly the given triggers
// once Start is called. home must already be connected (device list loaded,
// MQTT initialized).
func NewSource(home *controller.Home, triggers []TriggerConfig) *Source {
	return &Source{home: home, triggers: triggers}
}

// Start subscribes to every configured device and begins translating their
// reports into engine.Events. Devices that cannot be found or subscribed
// are logged and skipped, matching the previous watcher's behavior.
func (s *Source) Start(ctx context.Context) (<-chan engine.Event, error) {
	out := make(chan engine.Event)
	var wg sync.WaitGroup

	for _, trigger := range s.triggers {
		device, err := s.home.GetDeviceByName(trigger.Device)
		if err != nil {
			slog.Error("yolink: device not found", "device", trigger.Device)
			continue
		}

		reports, stop, err := device.Subscribe()
		if err != nil {
			slog.Error("yolink: subscribe failed", "device", trigger.Device, "error", err)
			continue
		}

		s.mu.Lock()
		s.stops = append(s.stops, stop)
		s.mu.Unlock()

		slog.Info("yolink: watching device", "device", trigger.Device, "event", trigger.Event)

		predicate := eventPredicates[trigger.Event]
		wg.Add(1)
		go func(trigger TriggerConfig, reports <-chan controller.Report) {
			defer wg.Done()
			for report := range reports {
				slog.Debug("yolink: device report", "device", trigger.Device, "event", report.Event)
				if !predicate(report) {
					continue
				}
				select {
				case out <- engine.Event{Service: "yolink", Device: trigger.Device, Name: trigger.Event, Time: time.Now()}:
				case <-ctx.Done():
					return
				}
			}
		}(trigger, reports)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out, nil
}

// Stop unsubscribes every watched device.
func (s *Source) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stop := range s.stops {
		stop()
	}
}
