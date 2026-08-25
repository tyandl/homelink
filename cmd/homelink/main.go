package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // embed the IANA zoneinfo database so TZ resolves correctly in the scratch runtime image

	"github.com/tyandl/homelink/internal/config"
	"github.com/tyandl/homelink/internal/engine"
	"github.com/tyandl/homelink/internal/frigate"
	"github.com/tyandl/homelink/internal/kasa"
	"github.com/tyandl/homelink/internal/timer"
	"github.com/tyandl/homelink/internal/yolink"
	"github.com/tyandl/yolink-api/v2/pkg/controller"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "homelink: %v\n", err)
		os.Exit(1)
	}
	if err := configureLogging(cfg.LogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "homelink: %v\n", err)
		os.Exit(1)
	}

	logBuildInfo()
	logConfigFileInfo(cfg)

	rules, usedTriggers, usedActions, err := buildRules(cfg)
	if err != nil {
		slog.Error("homelink: config", "error", err)
		os.Exit(1)
	}

	actuators := map[string]engine.Actuator{}
	var frigateClient *frigate.Client
	var kasaActuator *kasa.Actuator

	if usedActions["frigate"] {
		frigateSettings, err := frigate.LoadSettings(cfg.Meta, cfg.Frigate, cfg.FrigateDefined)
		if err != nil {
			slog.Error("frigate settings", "error", err)
			os.Exit(1)
		}
		if frigateSettings.BaseURL == "" {
			slog.Error("frigate settings: FRIGATE_BASE_URL (env var or [frigate] base_url in config file) is required because a rule uses a frigate action")
			os.Exit(1)
		}

		frigateClient = frigate.New(frigateSettings)
		if err := frigateClient.Login(); err != nil {
			slog.Warn("frigate login failed", "error", err)
		}
		if err := frigateClient.IsHealthy(); err != nil {
			slog.Warn("frigate health check failed", "error", err)
		}
		actuators["frigate"] = frigate.NewActuator(frigateClient)
	}

	if usedActions["kasa"] {
		kasaSettings, err := kasa.LoadSettings(cfg.Meta, cfg.Kasa, cfg.KasaDefined)
		if err != nil {
			slog.Error("kasa settings", "error", err)
			os.Exit(1)
		}
		if kasaSettings.AgentBaseURL == "" {
			slog.Error("kasa settings: KASA_AGENT_BASE_URL (env var or [kasa] agent_base_url in config file) is required because a rule uses a kasa action")
			os.Exit(1)
		}

		kasaActuator = kasa.NewActuator(kasaSettings)
		actuators["kasa"] = kasaActuator
	}

	sources := map[string]engine.Source{}
	var home *controller.Home

	if usedTriggers["timer"] || usedActions["timer"] {
		var schedules []timer.TriggerConfig
		for _, rule := range rules {
			if rule.Service != "timer" {
				continue
			}
			if trigger, ok := rule.TriggerConfig.(*timer.TriggerConfig); ok && trigger.Event == "time_of_day" {
				schedules = append(schedules, *trigger)
			}
		}

		timerService := timer.NewService(schedules)
		if usedTriggers["timer"] {
			sources["timer"] = timerService
		}
		if usedActions["timer"] {
			actuators["timer"] = timerService
		}
	}

	if usedTriggers["yolink"] {
		yolinkSettings, err := yolink.LoadSettings(cfg.Meta, cfg.YoLink, cfg.YoLinkDefined)
		if err != nil {
			slog.Error("yolink settings", "error", err)
			os.Exit(1)
		}
		if yolinkSettings.ClientID == "" || yolinkSettings.ClientSecret == "" {
			slog.Error("yolink settings: YOLINK_CLIENT_ID and YOLINK_CLIENT_SECRET are required because a rule uses a yolink trigger")
			os.Exit(1)
		}

		home, err = controller.NewHome(
			yolinkSettings.Options,
			func() string { return yolinkSettings.ClientID },
			func() string { return yolinkSettings.ClientSecret },
		)
		if err != nil {
			slog.Error("yolink connect", "error", err)
			os.Exit(1)
		}
		if _, err := home.GetDeviceList(); err != nil {
			slog.Error("yolink device list", "error", err)
			os.Exit(1)
		}
		if err := home.InitializeMqtt(); err != nil {
			slog.Error("yolink mqtt", "error", err)
			os.Exit(1)
		}
		defer home.CloseMqtt()
		slog.Info("yolink connected", "home", home.GetName())

		var triggers []yolink.TriggerConfig
		for _, rule := range rules {
			if rule.Service == "yolink" {
				triggers = append(triggers, yolink.TriggerConfig{Device: rule.Device, Event: rule.Event})
			}
		}
		sources["yolink"] = yolink.NewSource(home, triggers)
	}

	eng := engine.New(rules, sources, actuators)
	engineCtx, cancelEngine := context.WithCancel(context.Background())
	if err := eng.Start(engineCtx); err != nil {
		slog.Error("engine start", "error", err)
		os.Exit(1)
	}
	defer eng.Stop()
	defer cancelEngine()

	// HTTP server.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, request *http.Request) {
		handleHealth(writer, request, home, frigateClient, kasaActuator)
	})
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}
	go func() {
		slog.Info("http server listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", "error", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	server.Shutdown(shutdownCtx)
}

// serviceHeader reads just the "service" discriminator from a trigger or
// action primitive, so buildRules knows which service package should decode
// the rest of its fields.
type serviceHeader struct {
	Service string `toml:"service"`
}

// buildRules decodes every [[rules]] entry's trigger and actions by
// dispatching to the named service's own Decode function, and reports which
// trigger and action services were actually referenced so main only
// constructs the services it needs.
func buildRules(cfg *config.Config) (rules []engine.Rule, usedTriggers map[string]bool, usedActions map[string]bool, err error) {
	usedTriggers = map[string]bool{}
	usedActions = map[string]bool{}

	for _, rc := range cfg.Rules {
		var triggerHeader serviceHeader
		if err := cfg.Meta.PrimitiveDecode(rc.Trigger, &triggerHeader); err != nil {
			return nil, nil, nil, fmt.Errorf("rule %q: trigger: %w", rc.Name, err)
		}

		rule := engine.Rule{Name: rc.Name, Service: triggerHeader.Service}

		switch triggerHeader.Service {
		case "yolink":
			trigger, err := yolink.DecodeTrigger(cfg.Meta, rc.Trigger)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("rule %q: %w", rc.Name, err)
			}
			rule.Device = trigger.Device
			rule.Event = trigger.Event
			rule.TriggerConfig = &trigger
		case "timer":
			trigger, err := timer.DecodeTrigger(cfg.Meta, rc.Trigger)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("rule %q: %w", rc.Name, err)
			}
			rule.Device = trigger.Device
			rule.Event = trigger.Event
			rule.TriggerConfig = &trigger
		default:
			return nil, nil, nil, fmt.Errorf("rule %q: unknown trigger service %q", rc.Name, triggerHeader.Service)
		}
		usedTriggers[triggerHeader.Service] = true

		for i, ap := range rc.Actions {
			var actionHeader serviceHeader
			if err := cfg.Meta.PrimitiveDecode(ap, &actionHeader); err != nil {
				return nil, nil, nil, fmt.Errorf("rule %q: action %d: %w", rc.Name, i, err)
			}

			var actionConfig any
			switch actionHeader.Service {
			case "frigate":
				actionConfig, err = frigate.DecodeAction(cfg.Meta, ap)
			case "kasa":
				actionConfig, err = kasa.DecodeAction(cfg.Meta, ap)
			case "timer":
				actionConfig, err = timer.DecodeAction(cfg.Meta, ap)
			default:
				return nil, nil, nil, fmt.Errorf("rule %q: action %d: unknown service %q", rc.Name, i, actionHeader.Service)
			}
			if err != nil {
				return nil, nil, nil, fmt.Errorf("rule %q: action %d: %w", rc.Name, i, err)
			}

			rule.Actions = append(rule.Actions, engine.RuleAction{Service: actionHeader.Service, Config: actionConfig})
			usedActions[actionHeader.Service] = true
		}

		if len(rule.Actions) == 0 {
			return nil, nil, nil, fmt.Errorf("rule %q: at least one action is required", rc.Name)
		}

		rules = append(rules, rule)
	}

	return rules, usedTriggers, usedActions, nil
}

// logBuildInfo logs the module version and VCS revision embedded in the
// binary by the Go toolchain, so a running container can be traced back to
// the exact build that produced it.
func logBuildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		slog.Info("starting homelink", "version", "unknown")
		return
	}

	var revision string
	var dirty bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	slog.Info("starting homelink",
		"version", info.Main.Version,
		"revision", revision,
		"dirty", dirty,
		"go", info.GoVersion,
	)
}

// logConfigFileInfo logs which config file was loaded, along with its last
// modified time and checksum, so config drift is visible in the logs.
func logConfigFileInfo(cfg *config.Config) {
	if cfg.ConfigFilePath == "" {
		slog.Info("no config file found, using defaults and environment variables")
		return
	}
	slog.Info("config file loaded",
		"path", cfg.ConfigFilePath,
		"modified", cfg.ConfigFileModTime,
		"sha256", cfg.ConfigFileChecksum,
	)
}

func handleHealth(writer http.ResponseWriter, request *http.Request, home *controller.Home, frigateClient *frigate.Client, kasaActuator *kasa.Actuator) {
	if home != nil && !home.IsMqttConnected() {
		writer.WriteHeader(http.StatusServiceUnavailable)
		writer.Write([]byte("mqtt disconnected"))
		return
	}
	if frigateClient != nil {
		if err := frigateClient.IsHealthy(); err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			writer.Write([]byte("frigate unhealthy: " + err.Error()))
			return
		}
	}
	if kasaActuator != nil {
		if err := kasaActuator.Ping(request.Context()); err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			writer.Write([]byte("kasa agent unhealthy: " + err.Error()))
			return
		}
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("ok"))
}

func configureLogging(levelName string) error {
	var level slog.Level
	switch strings.ToLower(levelName) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return fmt.Errorf("invalid LOG_LEVEL %q (want debug, info, warn, or error)", levelName)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	return nil
}
