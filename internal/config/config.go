package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
)

const defaultConfigPath = "/config/homelink.toml"

// Config holds all runtime configuration, merged from the optional config file
// and environment variables. Environment variables always take precedence.
type Config struct {
	// YoLink API endpoints.
	YoLinkAPIHost string
	MQTTBroker    string

	// YoLink credentials (env var only).
	YoLinkClientID     string
	YoLinkClientSecret string

	// Frigate base URL (e.g. "http://frigate:5000").
	FrigateBaseURL string

	// Frigate native auth (optional, env var only).
	FrigateUser     string
	FrigatePassword string

	// FrigateInsecureSkipVerify disables TLS certificate verification for Frigate.
	// Only set this when Frigate uses a self-signed certificate on a trusted network.
	FrigateInsecureSkipVerify bool

	// Mappings associates YoLink devices with Frigate cameras and event labels.
	Mappings []DeviceMapping

	// HTTP server port. Defaults to 8080.
	Port int

	// Log verbosity: debug, info, warn, error. Defaults to warn.
	LogLevel string
}

// DeviceMapping binds a YoLink device to a Frigate camera event.
type DeviceMapping struct {
	// YoLinkDevice is the exact device name as it appears in the YoLink app.
	YoLinkDevice string `toml:"yolink_device"`
	// Camera is the Frigate camera name.
	Camera string `toml:"camera"`
	// Label is the Frigate event label (e.g. "mail", "person").
	Label string `toml:"label"`
	// SubLabel is an optional secondary label.
	SubLabel string `toml:"sub_label"`
	// Duration is how long the Frigate event stays open, in seconds. 0 uses the default (30s).
	Duration int `toml:"duration"`
	// BoundingBox is an optional [x, y, dx, dy] region in the camera frame where
	// x,y is the top-left corner and dx,dy is the width and height, all as
	// fractions of the frame dimensions (0.0–1.0).
	BoundingBox []float64 `toml:"bounding_box"`
	// PreCapture is seconds of footage before the trigger to include in the recording.
	// Omitted when zero, which lets Frigate apply its own default.
	PreCapture int `toml:"pre_capture"`
	// IncludeRecording controls whether a recording clip is attached. Default true.
	IncludeRecording *bool `toml:"include_recording"`
}

// fileConfig is the TOML schema for /config/homelink.toml.
// Only non-sensitive settings belong here; credentials go in env vars.
type fileConfig struct {
	YoLink struct {
		APIHost    string `toml:"api_host"`
		MQTTBroker string `toml:"mqtt_broker"`
	} `toml:"yolink"`

	Frigate struct {
		BaseURL string `toml:"base_url"`
	} `toml:"frigate"`

	Server struct {
		Port     int    `toml:"port"`
		LogLevel string `toml:"log_level"`
	} `toml:"server"`

	Mappings []DeviceMapping `toml:"mappings"`
}

// Load reads configuration from the optional config file and environment
// variables. Environment variables always take precedence over the file.
// The config file path defaults to /config/homelink.toml and can be
// overridden with the CONFIG_FILE environment variable.
func Load() (*Config, error) {
	cfg := &Config{
		YoLinkAPIHost: "https://api.yosmart.com",
		MQTTBroker:    "tcp://mqtt.api.yosmart.com:8003",
		Port:          8080,
		LogLevel:      "warn",
	}

	if err := loadFile(cfg); err != nil {
		return nil, err
	}
	loadEnv(cfg)

	var missing []string
	if cfg.YoLinkClientID == "" {
		missing = append(missing, "YOLINK_CLIENT_ID")
	}
	if cfg.YoLinkClientSecret == "" {
		missing = append(missing, "YOLINK_CLIENT_SECRET")
	}
	if cfg.FrigateBaseURL == "" {
		missing = append(missing, "FRIGATE_BASE_URL (env var or [frigate] base_url in config file)")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %v", missing)
	}

	return cfg, nil
}

// loadFile reads the TOML config file and overlays its values onto cfg.
// A missing file is silently ignored; any other read or parse error is fatal.
func loadFile(cfg *Config) error {
	path := firstNonEmpty(os.Getenv("CONFIG_FILE"), defaultConfigPath)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config file %s: %w", path, err)
	}

	var fc fileConfig
	if _, err := toml.Decode(string(data), &fc); err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}

	if fc.YoLink.APIHost != "" {
		cfg.YoLinkAPIHost = fc.YoLink.APIHost
	}
	if fc.YoLink.MQTTBroker != "" {
		cfg.MQTTBroker = fc.YoLink.MQTTBroker
	}
	if fc.Frigate.BaseURL != "" {
		cfg.FrigateBaseURL = fc.Frigate.BaseURL
	}
	if fc.Server.Port != 0 {
		cfg.Port = fc.Server.Port
	}
	if fc.Server.LogLevel != "" {
		cfg.LogLevel = fc.Server.LogLevel
	}
	cfg.Mappings = fc.Mappings

	return validateMappings(cfg.Mappings)
}

func validateMappings(mappings []DeviceMapping) error {
	for i, m := range mappings {
		if m.YoLinkDevice == "" {
			return fmt.Errorf("mapping %d: yolink_device is required", i)
		}
		if m.Camera == "" {
			return fmt.Errorf("mapping %d (%s): camera is required", i, m.YoLinkDevice)
		}
		if m.Label == "" {
			return fmt.Errorf("mapping %d (%s): label is required", i, m.YoLinkDevice)
		}
		if len(m.BoundingBox) != 0 && len(m.BoundingBox) != 4 {
			return fmt.Errorf("mapping %d (%s): bounding_box must have exactly 4 values [x, y, dx, dy]", i, m.YoLinkDevice)
		}
	}
	return nil
}

// loadEnv overlays environment variables onto cfg. Non-empty env vars win.
func loadEnv(cfg *Config) {
	if v := os.Getenv("YOLINK_API_HOST"); v != "" {
		cfg.YoLinkAPIHost = v
	}
	if v := os.Getenv("MQTT_BROKER"); v != "" {
		cfg.MQTTBroker = v
	}
	if v := os.Getenv("YOLINK_CLIENT_ID"); v != "" {
		cfg.YoLinkClientID = v
	}
	if v := os.Getenv("YOLINK_CLIENT_SECRET"); v != "" {
		cfg.YoLinkClientSecret = v
	}
	if v := os.Getenv("FRIGATE_BASE_URL"); v != "" {
		cfg.FrigateBaseURL = v
	}
	if v := os.Getenv("FRIGATE_USER"); v != "" {
		cfg.FrigateUser = v
	}
	if v := os.Getenv("FRIGATE_PASSWORD"); v != "" {
		cfg.FrigatePassword = v
	}
	if v := os.Getenv("FRIGATE_INSECURE_SKIP_VERIFY"); v == "true" || v == "1" {
		cfg.FrigateInsecureSkipVerify = true
	}
	if v := os.Getenv("PORT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.Port = parsed
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
