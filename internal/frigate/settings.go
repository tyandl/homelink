package frigate

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Settings are Frigate's connection settings, loaded from the optional
// [frigate] config section and FRIGATE_* environment variables. Environment
// variables always win over the config file. Credentials are env-var only.
type Settings struct {
	BaseURL            string
	User               string
	Password           string
	InsecureSkipVerify bool
}

type fileSettings struct {
	BaseURL string `toml:"base_url"`
}

// LoadSettings decodes the [frigate] section (if present) and overlays
// environment variables. It does not require BaseURL to be set — callers
// should only require it when a frigate action is actually configured.
func LoadSettings(meta toml.MetaData, section toml.Primitive, defined bool) (Settings, error) {
	var settings Settings

	if defined {
		var fs fileSettings
		if err := meta.PrimitiveDecode(section, &fs); err != nil {
			return Settings{}, fmt.Errorf("frigate settings: %w", err)
		}
		settings.BaseURL = fs.BaseURL
	}

	if v := os.Getenv("FRIGATE_BASE_URL"); v != "" {
		settings.BaseURL = v
	}
	settings.User = os.Getenv("FRIGATE_USER")
	settings.Password = os.Getenv("FRIGATE_PASSWORD")
	if v := os.Getenv("FRIGATE_INSECURE_SKIP_VERIFY"); v == "true" || v == "1" {
		settings.InsecureSkipVerify = true
	}

	return settings, nil
}
