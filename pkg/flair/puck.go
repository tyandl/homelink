package flair

import "github.com/tyandl/homelink/pkg/flair/types"

// PuckAttributes are the properties of a Flair Puck (the smart hub/sensor device).
type PuckAttributes struct {
	Name             string                   `json:"name"`
	TemperatureC     float64                  `json:"temperature-c"`
	Humidity         float64                  `json:"humidity"`         // 0–100 %RH
	RoomPressureMb   float64                  `json:"room-pressure-mb"` // millibar
	FirmwareVersionS string                   `json:"firmware-version-s,omitempty"`
	HardwareVersionS string                   `json:"hardware-version-s,omitempty"`
	IsGateway        bool                     `json:"is-gateway"`
	PuckDisplayColor types.PuckColorSelection `json:"puck-display-color,omitempty"`
	BackgroundColor  types.PuckColorSelection `json:"background-color,omitempty"`
	Rssi             int                      `json:"rssi"`
	IsActive         bool                     `json:"is-active"`
	Inactive         bool                     `json:"inactive"`
	BeaconInterval   int                      `json:"beacon-interval,omitempty"` // seconds
	CreatedAt        string                   `json:"created-at,omitempty"`
	UpdatedAt        string                   `json:"updated-at,omitempty"`
}

// PuckStateAttributes are the most recent sensor readings from a Puck.
type PuckStateAttributes struct {
	TemperatureC     float64                  `json:"temperature-c"`
	Humidity         float64                  `json:"humidity"`
	RoomPressureMb   float64                  `json:"room-pressure-mb"`
	Rssi             int                      `json:"rssi"`
	IsGateway        bool                     `json:"is-gateway"`
	PuckDisplayColor types.PuckColorSelection `json:"puck-display-color,omitempty"`
	UpdatedAt        string                   `json:"updated-at,omitempty"`
}

// GetPucks returns all Pucks belonging to the given Structure.
func (client *Client) GetPucks(structureID string) (Collection[PuckAttributes], error) {
	raw, err := client.list("/api/structures/"+structureID+"/pucks", nil)
	if err != nil {
		return Collection[PuckAttributes]{}, err
	}
	return decodeCollection[PuckAttributes](raw)
}

// GetPuck returns a single Puck by ID.
func (client *Client) GetPuck(id string) (Resource[PuckAttributes], error) {
	raw, err := client.get("/api/pucks/"+id, nil)
	if err != nil {
		return Resource[PuckAttributes]{}, err
	}
	return decodeResource[PuckAttributes](raw.Data)
}

// GetPuckState returns the current state of the Puck with the given ID.
func (client *Client) GetPuckState(puckID string) (Resource[PuckStateAttributes], error) {
	raw, err := client.get("/api/pucks/"+puckID+"/current-state", nil)
	if err != nil {
		return Resource[PuckStateAttributes]{}, err
	}
	return decodeResource[PuckStateAttributes](raw.Data)
}
