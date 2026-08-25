// Package config parses homelink.toml down to the generic rule structure
// (name + trigger + actions, each left as an undecoded toml.Primitive) plus
// the small amount of infra-level settings (server port, log level) that
// don't belong to any one service. It deliberately knows nothing about what
// a "yolink" or "frigate" trigger/action looks like — each service package
// decodes its own primitives via its own Settings/TriggerConfig/ActionConfig
// types, keeping services independent of each other and of this package's
// internals beyond the shared toml.MetaData/toml.Primitive plumbing.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

const defaultConfigPath = "/config/homelink.toml"

// Config holds the generic (service-agnostic) parts of homelink's
// configuration, plus the raw material each service package needs to decode
// its own settings and rule fields.
type Config struct {
	// Meta is the TOML decode metadata, required to later decode any
	// Primitive below via Meta.PrimitiveDecode.
	Meta toml.MetaData

	// YoLink, Frigate, and Kasa are each service's optional settings
	// section ([yolink], [frigate], [kasa]), left undecoded. The Defined
	// flags distinguish an absent section (zero Primitive, must not be
	// decoded) from an empty-but-present one.
	YoLink        toml.Primitive
	YoLinkDefined bool

	Frigate        toml.Primitive
	FrigateDefined bool

	Kasa        toml.Primitive
	KasaDefined bool

	// Rules are the [[rules]] entries, each with its trigger and actions
	// left undecoded until a service package's Decode function is called
	// with the service name read from the "service" field.
	Rules []RuleConfig

	// Port is the HTTP server port. Defaults to 8080.
	Port int
	// LogLevel is one of debug, info, warn, error. Defaults to warn.
	LogLevel string

	// ConfigFilePath is the path the config file was loaded from, empty if
	// no config file was found.
	ConfigFilePath string
	// ConfigFileModTime is the config file's last-modified time.
	ConfigFileModTime time.Time
	// ConfigFileChecksum is the hex-encoded SHA-256 checksum of the config
	// file's contents.
	ConfigFileChecksum string
}

// RuleConfig is one [[rules]] entry: a name plus its trigger and actions,
// still undecoded. Trigger and each entry of Actions carry at least a
// "service" field identifying which service package owns the rest of their
// fields.
type RuleConfig struct {
	Name    string
	Trigger toml.Primitive
	Actions []toml.Primitive
}

// fileConfig is the TOML schema for /config/homelink.toml.
type fileConfig struct {
	YoLink  toml.Primitive `toml:"yolink"`
	Frigate toml.Primitive `toml:"frigate"`
	Kasa    toml.Primitive `toml:"kasa"`

	Server struct {
		Port     int    `toml:"port"`
		LogLevel string `toml:"log_level"`
	} `toml:"server"`

	Rules []struct {
		Name    string           `toml:"name"`
		Trigger toml.Primitive   `toml:"trigger"`
		Actions []toml.Primitive `toml:"actions"`
	} `toml:"rules"`
}

// Load reads configuration from the optional config file and the PORT /
// LOG_LEVEL environment variables. Service-specific settings and
// credentials are loaded separately by each service package, only when that
// service is actually referenced by a rule.
func Load() (*Config, error) {
	cfg := &Config{
		Port:     8080,
		LogLevel: "warn",
	}

	if err := loadFile(cfg); err != nil {
		return nil, err
	}

	if v := os.Getenv("PORT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.Port = parsed
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	if len(cfg.Rules) == 0 {
		return nil, fmt.Errorf("no [[rules]] configured in %s", firstNonEmpty(os.Getenv("CONFIG_FILE"), defaultConfigPath))
	}

	return cfg, nil
}

// loadFile reads the TOML config file and populates cfg. A missing file is
// silently ignored (Load then fails on the empty-rules check above); any
// other read or parse error is fatal.
func loadFile(cfg *Config) error {
	path := firstNonEmpty(os.Getenv("CONFIG_FILE"), defaultConfigPath)

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config file %s: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}

	var fc fileConfig
	meta, err := toml.Decode(string(data), &fc)
	if err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}

	checksum := sha256.Sum256(data)
	cfg.ConfigFilePath = path
	cfg.ConfigFileModTime = info.ModTime()
	cfg.ConfigFileChecksum = hex.EncodeToString(checksum[:])

	cfg.Meta = meta
	cfg.YoLink = fc.YoLink
	cfg.YoLinkDefined = meta.IsDefined("yolink")
	cfg.Frigate = fc.Frigate
	cfg.FrigateDefined = meta.IsDefined("frigate")
	cfg.Kasa = fc.Kasa
	cfg.KasaDefined = meta.IsDefined("kasa")

	if fc.Server.Port != 0 {
		cfg.Port = fc.Server.Port
	}
	if fc.Server.LogLevel != "" {
		cfg.LogLevel = fc.Server.LogLevel
	}

	for i, r := range fc.Rules {
		if r.Name == "" {
			return fmt.Errorf("config file %s: rule %d: name is required", path, i)
		}
		cfg.Rules = append(cfg.Rules, RuleConfig{Name: r.Name, Trigger: r.Trigger, Actions: r.Actions})
	}

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
