package flair

import "github.com/tyandl/homelink/pkg/flair/types"

// HvacUnitAttributes are the properties of an HVAC unit connected to a Structure.
type HvacUnitAttributes struct {
	Make          string `json:"make,omitempty"`
	Model         string `json:"model,omitempty"`
	Type          string `json:"type,omitempty"` // e.g. "heat-pump", "default"
	Fuel          string `json:"fuel,omitempty"`
	HeatingStages int    `json:"heating-stages"`
	CoolingStages int    `json:"cooling-stages"`
	CreatedAt     string `json:"created-at,omitempty"`
	UpdatedAt     string `json:"updated-at,omitempty"`
}

// HvacUnitStateAttributes represent the current operating state of an HVAC unit.
type HvacUnitStateAttributes struct {
	Mode      types.HvacMode `json:"mode"`
	FanSpeed  string         `json:"fan-speed,omitempty"`
	UpdatedAt string         `json:"updated-at,omitempty"`
}

// HvacUnitStatePatch describes fields to update on an HvacUnitState. Nil fields are not sent.
type HvacUnitStatePatch struct {
	Mode     *types.HvacMode `json:"mode,omitempty"`
	FanSpeed *string         `json:"fan-speed,omitempty"`
}

// GetHvacUnits returns all HVAC units belonging to the given Structure.
func (client *Client) GetHvacUnits(structureID string) (Collection[HvacUnitAttributes], error) {
	raw, err := client.list("/api/structures/"+structureID+"/hvac-units", nil)
	if err != nil {
		return Collection[HvacUnitAttributes]{}, err
	}
	return decodeCollection[HvacUnitAttributes](raw)
}

// GetHvacUnit returns a single HVAC unit by ID.
func (client *Client) GetHvacUnit(id string) (Resource[HvacUnitAttributes], error) {
	raw, err := client.get("/api/hvac-units/"+id, nil)
	if err != nil {
		return Resource[HvacUnitAttributes]{}, err
	}
	return decodeResource[HvacUnitAttributes](raw.Data)
}

// GetHvacUnitState returns the current state of the HVAC unit with the given ID.
func (client *Client) GetHvacUnitState(hvacUnitID string) (Resource[HvacUnitStateAttributes], error) {
	raw, err := client.get("/api/hvac-units/"+hvacUnitID+"/current-state", nil)
	if err != nil {
		return Resource[HvacUnitStateAttributes]{}, err
	}
	return decodeResource[HvacUnitStateAttributes](raw.Data)
}

// UpdateHvacUnitState applies patch to the current state of the HVAC unit with the given ID.
func (client *Client) UpdateHvacUnitState(id string, patch HvacUnitStatePatch) (Resource[HvacUnitStateAttributes], error) {
	raw, err := client.patch("/api/hvac-unit-states/"+id, id, "hvac-unit-states", patch)
	if err != nil {
		return Resource[HvacUnitStateAttributes]{}, err
	}
	return decodeResource[HvacUnitStateAttributes](raw.Data)
}
