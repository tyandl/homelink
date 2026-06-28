package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tyandl/homelink/internal/config"
	"github.com/tyandl/homelink/internal/frigate"
	"github.com/tyandl/yolink-api/pkg/controller"
	_ "github.com/tyandl/yolink-api/pkg/devices"
	"github.com/tyandl/yolink-api/pkg/devices"
	"github.com/tyandl/yolink-api/pkg/types"
)

const (
	yolinkAPIHost = "https://api.yosmart.com"
	mqttBroker    = "tcp://mqtt.api.yosmart.com:8003"

	// frigateCamera and frigateLabel are the Frigate event target.
	// TODO: make these configurable via env vars.
	frigateCamera   = "Street Camera"
	frigateLabel    = "mail"
	frigateSubLabel = "mailbox"
	frigateDuration = 30 // seconds
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

	frigateClient := frigate.New(cfg)
	if err := frigateClient.Login(); err != nil {
		slog.Warn("frigate login failed", "error", err)
	}

	home, err := controller.NewHome(
		yolinkAPIHost,
		func() string { return cfg.YoLinkClientID },
		func() string { return cfg.YoLinkClientSecret },
	)
	if err != nil {
		slog.Error("yolink connect", "error", err)
		os.Exit(1)
	}
	if _, err := home.GetDeviceList(); err != nil {
		slog.Error("yolink device list", "error", err)
		os.Exit(1)
	}
	if err := home.InitializeMqtt(mqttBroker); err != nil {
		slog.Error("yolink mqtt", "error", err)
		os.Exit(1)
	}
	defer home.CloseMqtt()
	slog.Info("yolink connected", "home", home.GetName())

	// Watch the mailbox door sensor.
	// TODO: make the device name configurable via env var.
	mailbox, err := home.GetDeviceByName("Mailbox Sensor")
	if err != nil {
		slog.Error("yolink device lookup", "error", err)
		os.Exit(1)
	}
	reports, stopReports, err := mailbox.Subscribe()
	if err != nil {
		slog.Error("yolink subscribe", "error", err)
		os.Exit(1)
	}
	defer stopReports()
	slog.Info("watching device", "name", mailbox.GetName(), "id", mailbox.GetId())

	// HTTP server for health checks.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
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

	// Shut down cleanly on SIGINT / SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	duration := frigateDuration
	eventParams := frigate.CreateEventParams{
		SourceType: "api",
		SubLabel:   frigateSubLabel,
		Duration:   &duration,
	}

	for {
		select {
		case <-stop:
			slog.Info("shutting down")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server.Shutdown(ctx)
			return

		case report, ok := <-reports:
			if !ok {
				slog.Error("yolink report channel closed")
				return
			}
			slog.Debug("yolink report", "device", mailbox.GetName(), "event", report.Event)

			if report.Event != "DoorSensor.Alert" {
				continue
			}
			alert, ok := report.Data.(devices.DoorSensorAlertResponse)
			if !ok || alert.State != types.DoorStateOpen {
				continue
			}
			slog.Info("door opened — firing frigate event", "camera", frigateCamera, "label", frigateLabel)
			if err := frigateClient.CreateEvent(frigateCamera, frigateLabel, eventParams); err != nil {
				slog.Error("frigate create event", "error", err)
			}
		}
	}
}

func handleHealth(writer http.ResponseWriter, _ *http.Request) {
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
