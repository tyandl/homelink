// Package engine wires trigger events from source services to actions on
// actuator services. It knows nothing about YoLink, Frigate, or Kasa
// specifically — each service package implements Source and/or Actuator and
// is registered by name.
package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Event is a normalized trigger firing, produced by a Source and matched
// against configured rules by (Service, Device, Name).
type Event struct {
	Service string
	Device  string
	Name    string
	Time    time.Time
}

// Source is implemented by a service package that can produce trigger
// events. Start is called once per Engine.Start and should only watch the
// devices it was constructed with — a service with no configured triggers
// should never have a Source constructed for it at all.
type Source interface {
	// Start begins watching and returns a channel of events. The engine
	// stops reading from it once ctx is cancelled, so closing it on
	// shutdown is optional, not required — useful for a source like
	// timer's, where the channel is long-lived and shared with production
	// from arbitrary goroutines that ctx cancellation doesn't stop, and
	// closing it would risk a send on a closed channel.
	Start(ctx context.Context) (<-chan Event, error)
	// Stop releases any subscriptions or connections held by the source.
	Stop()
}

// Actuator is implemented by a service package that can perform actions.
// action is the concrete, already-decoded *ActionConfig type owned by that
// same service package; Actuator implementations type-assert their own type.
type Actuator interface {
	Execute(ctx context.Context, action any) error
}

// RuleAction is one action to perform when a rule's trigger fires.
type RuleAction struct {
	Service string
	Config  any
}

// Rule binds one trigger (Service, Device, Name) to the actions to run when
// a matching event arrives.
type Rule struct {
	Name    string
	Service string
	Device  string
	Event   string
	// TriggerConfig is the trigger service's own already-decoded config
	// (e.g. *timer.TriggerConfig), for services whose Source construction
	// needs fields beyond Device/Event — engine itself never reads this.
	TriggerConfig any
	Actions       []RuleAction
}

// Engine matches events from registered sources against rules and dispatches
// to registered actuators.
type Engine struct {
	rules     []Rule
	sources   map[string]Source
	actuators map[string]Actuator

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New constructs an Engine. sources and actuators are keyed by service name
// and should only contain entries for services actually referenced by rules.
func New(rules []Rule, sources map[string]Source, actuators map[string]Actuator) *Engine {
	return &Engine{rules: rules, sources: sources, actuators: actuators}
}

// Start begins every registered source and dispatches their events to
// matching rules until ctx is cancelled or Stop is called.
func (e *Engine) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel

	for name, source := range e.sources {
		events, err := source.Start(ctx)
		if err != nil {
			cancel()
			return err
		}

		e.wg.Add(1)
		go func(name string, events <-chan Event) {
			defer e.wg.Done()
			for {
				select {
				case event, ok := <-events:
					if !ok {
						return
					}
					e.dispatch(ctx, event)
				case <-ctx.Done():
					return
				}
			}
		}(name, events)
	}

	return nil
}

// Stop cancels all sources and waits for their dispatch loops to finish.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	for _, source := range e.sources {
		source.Stop()
	}
	e.wg.Wait()
}

func (e *Engine) dispatch(ctx context.Context, event Event) {
	for _, rule := range e.rules {
		if rule.Service != event.Service || rule.Device != event.Device || rule.Event != event.Name {
			continue
		}

		slog.Info("rule triggered", "rule", rule.Name, "service", event.Service, "device", event.Device, "event", event.Name)

		for _, action := range rule.Actions {
			actuator, ok := e.actuators[action.Service]
			if !ok {
				slog.Error("engine: no actuator registered for service", "service", action.Service, "rule", rule.Name)
				continue
			}
			if err := actuator.Execute(ctx, action.Config); err != nil {
				slog.Error("engine: action failed", "service", action.Service, "rule", rule.Name, "error", err)
			}
		}
	}
}
