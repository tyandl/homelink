package flair

// SensorReadingAttributes are the timestamped sensor measurements from a Puck or Vent.
type SensorReadingAttributes struct {
	TemperatureC   float64 `json:"temperature-c"`
	Humidity       float64 `json:"humidity,omitempty"`         // Pucks only; 0–100 %RH
	RoomPressureMb float64 `json:"room-pressure-mb,omitempty"` // Pucks only; millibar
	DuctPressure   float64 `json:"duct-pressure,omitempty"`    // Vents only; Pascals
	UpdatedAt      string  `json:"updated-at,omitempty"`
}

// GetVentSensorReadings returns historical sensor readings for the given Vent.
// Pass params such as {"filter[from]": "2024-01-01T00:00:00Z"} to scope the results.
func (client *Client) GetVentSensorReadings(ventID string, params map[string]string) (Collection[SensorReadingAttributes], error) {
	raw, err := client.list("/api/vents/"+ventID+"/sensor-readings", params)
	if err != nil {
		return Collection[SensorReadingAttributes]{}, err
	}
	return decodeCollection[SensorReadingAttributes](raw)
}

// GetPuckSensorReadings returns historical sensor readings for the given Puck.
// Pass params such as {"filter[from]": "2024-01-01T00:00:00Z"} to scope the results.
func (client *Client) GetPuckSensorReadings(puckID string, params map[string]string) (Collection[SensorReadingAttributes], error) {
	raw, err := client.list("/api/pucks/"+puckID+"/sensor-readings", params)
	if err != nil {
		return Collection[SensorReadingAttributes]{}, err
	}
	return decodeCollection[SensorReadingAttributes](raw)
}
