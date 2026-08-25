package yolink

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/tyandl/yolink-api/v2/pkg/controller"
)

// decodeTestSection wraps a snippet of TOML fields as the body of a single [t] table and
// returns the resulting Primitive + MetaData, so LoadSettings (which expects a table
// primitive) can be exercised directly without going through internal/config. An empty fields
// string exercises the "section not defined" path via defined=false at the call site instead.
func decodeTestSection(t *testing.T, fields string) (toml.MetaData, toml.Primitive) {
	t.Helper()

	var doc struct {
		T toml.Primitive `toml:"t"`
	}
	src := "[t]\n" + fields
	meta, err := toml.Decode(src, &doc)
	if err != nil {
		t.Fatalf("toml.Decode: %v", err)
	}
	return meta, doc.T
}

func TestLoadSettingsCloudDefaults(t *testing.T) {
	meta, section := decodeTestSection(t, "")
	settings, err := LoadSettings(meta, section, true)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	cloud, ok := settings.Options.(controller.CloudOptions)
	if !ok {
		t.Fatalf("Options = %#v, want controller.CloudOptions", settings.Options)
	}
	if cloud.APIHost != "" || cloud.MQTTBroker != "" {
		t.Errorf("unexpected cloud overrides: %+v", cloud)
	}
}

func TestLoadSettingsCloudFromFile(t *testing.T) {
	meta, section := decodeTestSection(t, `[t.cloud]
api_host = "https://staging.example.com"
mqtt_broker = "tcp://staging.example.com:8003"`)
	settings, err := LoadSettings(meta, section, true)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	cloud, ok := settings.Options.(controller.CloudOptions)
	if !ok {
		t.Fatalf("Options = %#v, want controller.CloudOptions", settings.Options)
	}
	if cloud.APIHost != "https://staging.example.com" || cloud.MQTTBroker != "tcp://staging.example.com:8003" {
		t.Errorf("unexpected cloud overrides: %+v", cloud)
	}
}

func TestLoadSettingsCloudEnvOverridesFile(t *testing.T) {
	t.Setenv("YOLINK_API_HOST", "https://env.example.com")
	t.Setenv("MQTT_BROKER", "tcp://env.example.com:8003")

	meta, section := decodeTestSection(t, `[t.cloud]
api_host = "https://file.example.com"
mqtt_broker = "tcp://file.example.com:8003"`)
	settings, err := LoadSettings(meta, section, true)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	cloud := settings.Options.(controller.CloudOptions)
	if cloud.APIHost != "https://env.example.com" || cloud.MQTTBroker != "tcp://env.example.com:8003" {
		t.Errorf("env should win over file: %+v", cloud)
	}
}

func TestLoadSettingsLocalFromFile(t *testing.T) {
	meta, section := decodeTestSection(t, `[t.local]
host = "192.168.5.72"
net_id = "C00295"
http_port = 1080
mqtt_port = 18080`)
	settings, err := LoadSettings(meta, section, true)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	local, ok := settings.Options.(controller.LocalOptions)
	if !ok {
		t.Fatalf("Options = %#v, want controller.LocalOptions", settings.Options)
	}
	want := controller.LocalOptions{Host: "192.168.5.72", NetId: "C00295", HTTPPort: 1080, MQTTPort: 18080}
	if local != want {
		t.Errorf("local = %+v, want %+v", local, want)
	}
}

func TestLoadSettingsLocalHostEnvAloneSwitchesMode(t *testing.T) {
	t.Setenv("YOLINK_LOCAL_HOST", "192.168.5.72")
	t.Setenv("YOLINK_NET_ID", "C00295")

	meta, section := decodeTestSection(t, "") // no [t.local] in the file at all
	settings, err := LoadSettings(meta, section, true)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	local, ok := settings.Options.(controller.LocalOptions)
	if !ok {
		t.Fatalf("Options = %#v, want controller.LocalOptions", settings.Options)
	}
	if local.Host != "192.168.5.72" || local.NetId != "C00295" {
		t.Errorf("unexpected local options: %+v", local)
	}
}

func TestLoadSettingsLocalEnvOverridesFile(t *testing.T) {
	t.Setenv("YOLINK_NET_ID", "C99999")
	t.Setenv("YOLINK_LOCAL_HTTP_PORT", "8080")

	meta, section := decodeTestSection(t, `[t.local]
host = "192.168.5.72"
net_id = "C00295"`)
	settings, err := LoadSettings(meta, section, true)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	local := settings.Options.(controller.LocalOptions)
	if local.NetId != "C99999" || local.HTTPPort != 8080 {
		t.Errorf("env should win over file: %+v", local)
	}
}

func TestLoadSettingsRejectsMixedCloudAndLocal(t *testing.T) {
	meta, section := decodeTestSection(t, `[t.cloud]
api_host = "https://api.yosmart.com"

[t.local]
host = "192.168.5.72"
net_id = "C00295"`)
	if _, err := LoadSettings(meta, section, true); err == nil {
		t.Fatal("expected error mixing [t.cloud] with [t.local], got nil")
	}
}

func TestLoadSettingsRejectsCloudEnvMixedWithLocalFile(t *testing.T) {
	t.Setenv("MQTT_BROKER", "tcp://cloud.example.com:8003")

	meta, section := decodeTestSection(t, `[t.local]
host = "192.168.5.72"
net_id = "C00295"`)
	if _, err := LoadSettings(meta, section, true); err == nil {
		t.Fatal("expected error mixing MQTT_BROKER env with [t.local] file section, got nil")
	}
}

func TestLoadSettingsCredentialsFromEnv(t *testing.T) {
	t.Setenv("YOLINK_CLIENT_ID", "id-123")
	t.Setenv("YOLINK_CLIENT_SECRET", "secret-456")

	meta, section := decodeTestSection(t, "")
	settings, err := LoadSettings(meta, section, false)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if settings.ClientID != "id-123" || settings.ClientSecret != "secret-456" {
		t.Errorf("unexpected credentials: %+v", settings)
	}
	if _, ok := settings.Options.(controller.CloudOptions); !ok {
		t.Errorf("Options = %#v, want controller.CloudOptions when section is not defined", settings.Options)
	}
}
