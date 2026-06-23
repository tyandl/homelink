package flair

import "github.com/tyandl/homelink/pkg/flair/types"

// StructureAttributes are the properties of a Flair Structure (a home or property).
type StructureAttributes struct {
	Name                      string                 `json:"name"`
	Mode                      types.SystemMode       `json:"mode"`
	SetPointTemperatureC      float64                `json:"set-point-temperature-c"`
	SetPointMode              types.SetPointMode     `json:"set-point-mode"`
	HomeAwayMode              types.StructureMode    `json:"home-away-mode"`
	TemperatureScale          types.TemperatureScale `json:"temperature-scale"`
	TargetTemperatureC        float64                `json:"target-temperature-c"`
	IsActive                  bool                   `json:"is-active"`
	GeofencingEnabled         bool                   `json:"geofencing-enabled"`
	SchedulingEnabled         bool                   `json:"scheduling-enabled"`
	DuctPressureEnabled       bool                   `json:"duct-pressure-enabled"`
	MinVentOpenPercentCooling int                    `json:"min-vent-open-percent-cooling"`
	MinVentOpenPercentHeating int                    `json:"min-vent-open-percent-heating"`
	SetupComplete             bool                   `json:"setup-complete"`
	CountryCode               string                 `json:"country-code,omitempty"`
	PostalCode                string                 `json:"postal-code,omitempty"`
	Timezone                  string                 `json:"timezone,omitempty"`
	CreatedAt                 string                 `json:"created-at,omitempty"`
	UpdatedAt                 string                 `json:"updated-at,omitempty"`
}

// StructurePatch describes fields to update on a Structure. Nil fields are not sent.
// Use Ptr() to set a value: Ptr(SystemModeManual), Ptr(true), etc.
type StructurePatch struct {
	Name                      *string                 `json:"name,omitempty"`
	Mode                      *types.SystemMode       `json:"mode,omitempty"`
	SetPointTemperatureC      *float64                `json:"set-point-temperature-c,omitempty"`
	SetPointMode              *types.SetPointMode     `json:"set-point-mode,omitempty"`
	HomeAwayMode              *types.StructureMode    `json:"home-away-mode,omitempty"`
	TemperatureScale          *types.TemperatureScale `json:"temperature-scale,omitempty"`
	GeofencingEnabled         *bool                   `json:"geofencing-enabled,omitempty"`
	SchedulingEnabled         *bool                   `json:"scheduling-enabled,omitempty"`
	DuctPressureEnabled       *bool                   `json:"duct-pressure-enabled,omitempty"`
	MinVentOpenPercentCooling *int                    `json:"min-vent-open-percent-cooling,omitempty"`
	MinVentOpenPercentHeating *int                    `json:"min-vent-open-percent-heating,omitempty"`
}

// GetStructures returns all Structures accessible to the authenticated user.
func (client *Client) GetStructures() (Collection[StructureAttributes], error) {
	raw, err := client.list("/api/structures", nil)
	if err != nil {
		return Collection[StructureAttributes]{}, err
	}
	return decodeCollection[StructureAttributes](raw)
}

// GetStructure returns a single Structure by ID.
func (client *Client) GetStructure(id string) (Resource[StructureAttributes], error) {
	raw, err := client.get("/api/structures/"+id, nil)
	if err != nil {
		return Resource[StructureAttributes]{}, err
	}
	return decodeResource[StructureAttributes](raw.Data)
}

// UpdateStructure applies patch to the Structure with the given ID and returns the updated resource.
func (client *Client) UpdateStructure(id string, patch StructurePatch) (Resource[StructureAttributes], error) {
	raw, err := client.patch("/api/structures/"+id, id, "structures", patch)
	if err != nil {
		return Resource[StructureAttributes]{}, err
	}
	return decodeResource[StructureAttributes](raw.Data)
}
