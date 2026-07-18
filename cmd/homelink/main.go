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

	"github.com/tyandl/homelink/internal/config"
	"github.com/tyandl/homelink/internal/frigate"
	"github.com/tyandl/homelink/internal/watcher"
	"github.com/tyandl/yolink-api/pkg/controller"
	_ "github.com/tyandl/yolink-api/pkg/devices"
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

	frigateClient := frigate.New(cfg)
	if err := frigateClient.Login(); err != nil {
		slog.Warn("frigate login failed", "error", err)
	}

	home, err := controller.NewHome(
		cfg.YoLinkAPIHost,
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
	if err := home.InitializeMqtt(cfg.MQTTBroker); err != nil {
		slog.Error("yolink mqtt", "error", err)
		os.Exit(1)
	}
	defer home.CloseMqtt()
	slog.Info("yolink connected", "home", home.GetName())

	w := watcher.New(cfg.Mappings, home, frigateClient)
	w.Start()
	defer w.Stop()

	// HTTP server.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, request *http.Request) {
		handleHealth(writer, request, home)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
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

func handleHealth(writer http.ResponseWriter, _ *http.Request, home *controller.Home) {
	if !home.IsMqttConnected() {
		writer.WriteHeader(http.StatusServiceUnavailable)
		writer.Write([]byte("mqtt disconnected"))
		return
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
