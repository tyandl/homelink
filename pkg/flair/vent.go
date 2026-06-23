package flair

// VentAttributes are the properties of a Flair smart vent.
type VentAttributes struct {
	Name             string  `json:"name"`
	PercentOpen      int     `json:"percent-open"`  // 0–100
	DuctPressure     float64 `json:"duct-pressure"` // Pascals
	DuctTemperatureC float64 `json:"duct-temperature-c"`
	MotorRunTime     int     `json:"motor-run-time"` // seconds
	FirmwareVersionS string  `json:"firmware-version-s,omitempty"`
	Rssi             int     `json:"rssi"` // dBm
	HasBuzz          bool    `json:"has-buzz"`
	IsActive         bool    `json:"is-active"`
	Inactive         bool    `json:"inactive"`
	CreatedAt        string  `json:"created-at,omitempty"`
	UpdatedAt        string  `json:"updated-at,omitempty"`
}

// VentPatch describes fields to update on a Vent. Nil fields are not sent.
type VentPatch struct {
	Name        *string `json:"name,omitempty"`
	PercentOpen *int    `json:"percent-open,omitempty"`
}

// VentStateAttributes are the most recent sensor readings and position of a Vent.
type VentStateAttributes struct {
	PercentOpen      int     `json:"percent-open"`
	DuctPressure     float64 `json:"duct-pressure"`
	DuctTemperatureC float64 `json:"duct-temperature-c"`
	MotorRunTime     int     `json:"motor-run-time"`
	Rssi             int     `json:"rssi"`
	UpdatedAt        string  `json:"updated-at,omitempty"`
}

// GetVents returns all Vents belonging to the given Room.
func (client *Client) GetVents(roomID string) (Collection[VentAttributes], error) {
	raw, err := client.list("/api/rooms/"+roomID+"/vents", nil)
	if err != nil {
		return Collection[VentAttributes]{}, err
	}
	return decodeCollection[VentAttributes](raw)
}

// GetVent returns a single Vent by ID.
func (client *Client) GetVent(id string) (Resource[VentAttributes], error) {
	raw, err := client.get("/api/vents/"+id, nil)
	if err != nil {
		return Resource[VentAttributes]{}, err
	}
	return decodeResource[VentAttributes](raw.Data)
}

// UpdateVent applies patch to the Vent with the given ID and returns the updated resource.
func (client *Client) UpdateVent(id string, patch VentPatch) (Resource[VentAttributes], error) {
	raw, err := client.patch("/api/vents/"+id, id, "vents", patch)
	if err != nil {
		return Resource[VentAttributes]{}, err
	}
	return decodeResource[VentAttributes](raw.Data)
}

// GetVentState returns the current state of the Vent with the given ID.
func (client *Client) GetVentState(ventID string) (Resource[VentStateAttributes], error) {
	raw, err := client.get("/api/vents/"+ventID+"/current-state", nil)
	if err != nil {
		return Resource[VentStateAttributes]{}, err
	}
	return decodeResource[VentStateAttributes](raw.Data)
}
