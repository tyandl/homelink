// Package kasa implements engine.Actuator for TP-Link Kasa devices. It does
// not speak the Kasa local protocol itself — devices on current firmware
// require KLAP, which has no mature Go implementation, so this package
// delegates to a small sidecar HTTP agent (kasa-agent) that wraps the
// actively-maintained python-kasa library. Devices are addressed by name
// only: the agent owns discovery (scanning its configured subnet and
// caching each device's self-reported alias), so this package never learns
// or stores an IP.
package kasa

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Settings are Kasa's agent connection settings, loaded from the optional
// [kasa] config section and the KASA_AGENT_BASE_URL environment variable.
type Settings struct {
	// AgentBaseURL is the base URL of the kasa-agent sidecar, e.g.
	// "http://kasa-agent:8090".
	AgentBaseURL string
}

type fileSettings struct {
	AgentBaseURL string `toml:"agent_base_url"`
}

// LoadSettings decodes the [kasa] section (if present) and overlays
// KASA_AGENT_BASE_URL. It does not require AgentBaseURL to be set — callers
// should only require it when a kasa action is actually configured.
func LoadSettings(meta toml.MetaData, section toml.Primitive, defined bool) (Settings, error) {
	var settings Settings

	if defined {
		var fs fileSettings
		if err := meta.PrimitiveDecode(section, &fs); err != nil {
			return Settings{}, fmt.Errorf("kasa settings: %w", err)
		}
		settings.AgentBaseURL = fs.AgentBaseURL
	}

	if v := os.Getenv("KASA_AGENT_BASE_URL"); v != "" {
		settings.AgentBaseURL = v
	}

	return settings, nil
}
