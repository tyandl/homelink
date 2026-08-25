// Package yolink adapts YoLink devices to engine.Source. It is the only
// package that knows about github.com/tyandl/yolink-api; nothing outside
// this package (other than main, which wires services together) needs to
// know how YoLink reports are shaped or authenticated.
package yolink

import (
	"fmt"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/tyandl/yolink-api/v2/pkg/controller"
	_ "github.com/tyandl/yolink-api/v2/pkg/devices" // registers device report types for JSON decoding
)

// Settings are YoLink's connection settings, loaded from the optional [yolink] config section
// (whose [yolink.cloud]/[yolink.local] sub-tables select the mode) plus YOLINK_*/MQTT_BROKER
// environment variables. Environment variables always win over the config file. Credentials
// are env-var only, matching the rest of homelink's secret handling.
type Settings struct {
	// Options is a controller.CloudOptions or controller.LocalOptions, ready to pass to
	// controller.NewHome.
	Options      controller.ConnectOptions
	ClientID     string
	ClientSecret string
}

// fileSettings is the [yolink] section's TOML schema. Cloud and Local are mutually exclusive
// sub-tables -- see LoadSettings. Nothing belongs directly on [yolink] itself: cloud and local
// settings are genuinely disjoint (no field is meaningful in both modes), so [yolink] is just
// the namespace the two sub-tables live under.
type fileSettings struct {
	Cloud *fileCloudSettings `toml:"cloud"`
	Local *fileLocalSettings `toml:"local"`
}

// fileCloudSettings is the [yolink.cloud] sub-table. Both fields are optional overrides of
// the production cloud endpoints, only needed to target a different cloud environment (e.g.
// staging).
type fileCloudSettings struct {
	APIHost    string `toml:"api_host"`
	MQTTBroker string `toml:"mqtt_broker"`
}

// fileLocalSettings is the [yolink.local] sub-table: connect to a Local Hub on the LAN
// instead of the cloud. Host and NetId are required once this table is present (by file or
// by YOLINK_LOCAL_HOST); HTTPPort/MQTTPort default to the hub's standard 1080/18080.
type fileLocalSettings struct {
	Host     string `toml:"host"`
	NetId    string `toml:"net_id"`
	HTTPPort int    `toml:"http_port"`
	MQTTPort int    `toml:"mqtt_port"`
}

// LoadSettings decodes the [yolink] section (if present), overlays environment variables, and
// resolves the result into a controller.ConnectOptions -- controller.LocalOptions if
// [yolink.local] (or YOLINK_LOCAL_HOST) is set, otherwise controller.CloudOptions (the
// default; [yolink.cloud] and its env vars are optional overrides of it). It returns an error
// if both [yolink.cloud] and [yolink.local] (by file or env var) are given, since a Home
// connects to either the cloud or one Local Hub, never both. It does not otherwise validate
// that credentials or LocalOptions.Host/NetId are set -- callers should only require
// credentials when a yolink trigger is actually configured, and controller.NewHome itself
// validates a LocalOptions value.
func LoadSettings(meta toml.MetaData, section toml.Primitive, defined bool) (Settings, error) {
	var fs fileSettings
	if defined {
		if err := meta.PrimitiveDecode(section, &fs); err != nil {
			return Settings{}, fmt.Errorf("yolink settings: %w", err)
		}
	}

	if host, netId := os.Getenv("YOLINK_LOCAL_HOST"), os.Getenv("YOLINK_NET_ID"); host != "" || netId != "" {
		if fs.Local == nil {
			fs.Local = &fileLocalSettings{}
		}
		if host != "" {
			fs.Local.Host = host
		}
		if netId != "" {
			fs.Local.NetId = netId
		}
	}
	if fs.Local != nil {
		if port, ok := envInt("YOLINK_LOCAL_HTTP_PORT"); ok {
			fs.Local.HTTPPort = port
		}
		if port, ok := envInt("YOLINK_LOCAL_MQTT_PORT"); ok {
			fs.Local.MQTTPort = port
		}
	}

	if apiHost, broker := os.Getenv("YOLINK_API_HOST"), os.Getenv("MQTT_BROKER"); apiHost != "" || broker != "" {
		if fs.Cloud == nil {
			fs.Cloud = &fileCloudSettings{}
		}
		if apiHost != "" {
			fs.Cloud.APIHost = apiHost
		}
		if broker != "" {
			fs.Cloud.MQTTBroker = broker
		}
	}

	var options controller.ConnectOptions
	switch {
	case fs.Local != nil && fs.Cloud != nil:
		return Settings{}, fmt.Errorf("yolink settings: [yolink.local] and [yolink.cloud] (or their env vars) cannot both be set -- a Home connects to either the cloud or one Local Hub, not both")
	case fs.Local != nil:
		options = controller.LocalOptions{
			Host:     fs.Local.Host,
			NetId:    fs.Local.NetId,
			HTTPPort: fs.Local.HTTPPort,
			MQTTPort: fs.Local.MQTTPort,
		}
	case fs.Cloud != nil:
		options = controller.CloudOptions{APIHost: fs.Cloud.APIHost, MQTTBroker: fs.Cloud.MQTTBroker}
	default:
		options = controller.CloudOptions{}
	}

	return Settings{
		Options:      options,
		ClientID:     os.Getenv("YOLINK_CLIENT_ID"),
		ClientSecret: os.Getenv("YOLINK_CLIENT_SECRET"),
	}, nil
}

// envInt parses the named environment variable as an int, returning ok=false if it is unset
// or not a valid integer.
func envInt(name string) (int, bool) {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return 0, false
	}
	return value, true
}
