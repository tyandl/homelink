package flair

import "github.com/tyandl/homelink/pkg/flair/types"

// ThermostatAttributes are the properties of a thermostat connected to a Structure.
type ThermostatAttributes struct {
	Make             string `json:"make,omitempty"`
	Model            string `json:"model,omitempty"`
	FirmwareVersionS string `json:"firmware-version-s,omitempty"`
	IsConnected      bool   `json:"is-connected"`
	CreatedAt        string `json:"created-at,omitempty"`
	UpdatedAt        string `json:"updated-at,omitempty"`
}

// ThermostatStateAttributes represent the current operating state of a thermostat.
type ThermostatStateAttributes struct {
	Mode               types.HvacMode `json:"mode"`
	FanMode            string         `json:"fan-mode,omitempty"`
	CurrentCoolTempF   float64        `json:"current-cool-temp-f"`
	CurrentHeatTempF   float64        `json:"current-heat-temp-f"`
	CurrentTemperatureF float64       `json:"current-temperature-f"`
	UpdatedAt          string         `json:"updated-at,omitempty"`
}

// ThermostatStatePatch describes fields to update on a ThermostatState. Nil fields are not sent.
type ThermostatStatePatch struct {
	Mode             *types.HvacMode `json:"mode,omitempty"`
	FanMode          *string         `json:"fan-mode,omitempty"`
	CurrentCoolTempF *float64        `json:"current-cool-temp-f,omitempty"`
	CurrentHeatTempF *float64        `json:"current-heat-temp-f,omitempty"`
}

// GetThermostats returns all thermostats belonging to the given Structure.
func (client *Client) GetThermostats(structureID string) (Collection[ThermostatAttributes], error) {
	raw, err := client.list("/api/structures/"+structureID+"/thermostats", nil)
	if err != nil {
		return Collection[ThermostatAttributes]{}, err
	}
	return decodeCollection[ThermostatAttributes](raw)
}

// GetThermostat returns a single thermostat by ID.
func (client *Client) GetThermostat(id string) (Resource[ThermostatAttributes], error) {
	raw, err := client.get("/api/thermostats/"+id, nil)
	if err != nil {
		return Resource[ThermostatAttributes]{}, err
	}
	return decodeResource[ThermostatAttributes](raw.Data)
}

// GetThermostatState returns the current state of the thermostat with the given ID.
func (client *Client) GetThermostatState(thermostatID string) (Resource[ThermostatStateAttributes], error) {
	raw, err := client.get("/api/thermostats/"+thermostatID+"/current-state", nil)
	if err != nil {
		return Resource[ThermostatStateAttributes]{}, err
	}
	return decodeResource[ThermostatStateAttributes](raw.Data)
}

// UpdateThermostatState applies patch to the current state of the thermostat with the given ID.
func (client *Client) UpdateThermostatState(id string, patch ThermostatStatePatch) (Resource[ThermostatStateAttributes], error) {
	raw, err := client.patch("/api/thermostat-states/"+id, id, "thermostat-states", patch)
	if err != nil {
		return Resource[ThermostatStateAttributes]{}, err
	}
	return decodeResource[ThermostatStateAttributes](raw.Data)
}
