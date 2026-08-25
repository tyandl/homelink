// Package yolink adapts YoLink devices to engine.Source. It is the only
// package that knows about github.com/tyandl/yolink-api; nothing outside
// this package (other than main, which wires services together) needs to
// know how YoLink reports are shaped or authenticated.
package yolink

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	_ "github.com/tyandl/yolink-api/pkg/devices" // registers device report types for JSON decoding
)

const (
	defaultAPIHost    = "https://api.yosmart.com"
	defaultMQTTBroker = "tcp://mqtt.api.yosmart.com:8003"
)

// Settings are YoLink's connection settings, loaded from the optional
// [yolink] config section and YOLINK_* / MQTT_BROKER environment variables.
// Environment variables always win over the config file. Credentials are
// env-var only, matching the rest of homelink's secret handling.
type Settings struct {
	APIHost      string
	MQTTBroker   string
	ClientID     string
	ClientSecret string
}

type fileSettings struct {
	APIHost    string `toml:"api_host"`
	MQTTBroker string `toml:"mqtt_broker"`
}

// LoadSettings decodes the [yolink] section (if present) and overlays
// environment variables. It does not validate that credentials are set —
// callers should only call this, and only require credentials, when a
// yolink trigger is actually configured.
func LoadSettings(meta toml.MetaData, section toml.Primitive, defined bool) (Settings, error) {
	settings := Settings{APIHost: defaultAPIHost, MQTTBroker: defaultMQTTBroker}

	if defined {
		var fs fileSettings
		if err := meta.PrimitiveDecode(section, &fs); err != nil {
			return Settings{}, fmt.Errorf("yolink settings: %w", err)
		}
		if fs.APIHost != "" {
			settings.APIHost = fs.APIHost
		}
		if fs.MQTTBroker != "" {
			settings.MQTTBroker = fs.MQTTBroker
		}
	}

	if v := os.Getenv("YOLINK_API_HOST"); v != "" {
		settings.APIHost = v
	}
	if v := os.Getenv("MQTT_BROKER"); v != "" {
		settings.MQTTBroker = v
	}
	settings.ClientID = os.Getenv("YOLINK_CLIENT_ID")
	settings.ClientSecret = os.Getenv("YOLINK_CLIENT_SECRET")

	return settings, nil
}
