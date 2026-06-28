package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	// YoLink credentials.
	YoLinkClientID     string
	YoLinkClientSecret string

	// Frigate base URL (e.g. "https://frigate.example.com").
	FrigateBaseURL string

	// Cloudflare Access service-token credentials.
	// Required when Frigate sits behind a Cloudflare Access policy.
	CFAccessClientID     string
	CFAccessClientSecret string

	// Frigate native auth. Optional — only needed when Frigate's own
	// authentication is enabled in addition to (or instead of) CF Access.
	FrigateUser     string
	FrigatePassword string

	// HTTP server port. Defaults to 8080.
	Port int

	// Log verbosity: debug, info, warn, error. Defaults to warn.
	LogLevel string
}

// Load reads configuration from environment variables and returns an error if
// any required variable is missing or invalid.
func Load() (*Config, error) {
	config := &Config{
		YoLinkClientID:       os.Getenv("YOLINK_CLIENT_ID"),
		YoLinkClientSecret:   os.Getenv("YOLINK_CLIENT_SECRET"),
		FrigateBaseURL:       os.Getenv("FRIGATE_BASE_URL"),
		CFAccessClientID:     os.Getenv("CF_ACCESS_CLIENT_ID"),
		CFAccessClientSecret: os.Getenv("CF_ACCESS_CLIENT_SECRET"),
		FrigateUser:          os.Getenv("FRIGATE_USER"),
		FrigatePassword:      os.Getenv("FRIGATE_PASSWORD"),
		LogLevel:             firstNonEmpty(os.Getenv("LOG_LEVEL"), "warn"),
	}

	port := os.Getenv("PORT")
	if port == "" {
		config.Port = 8080
	} else {
		parsed, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("PORT: %w", err)
		}
		config.Port = parsed
	}

	var missing []string
	if config.YoLinkClientID == "" {
		missing = append(missing, "YOLINK_CLIENT_ID")
	}
	if config.YoLinkClientSecret == "" {
		missing = append(missing, "YOLINK_CLIENT_SECRET")
	}
	if config.FrigateBaseURL == "" {
		missing = append(missing, "FRIGATE_BASE_URL")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return config, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
