package flair

// GeofenceAttributes are the properties of a geographic boundary used for presence detection.
type GeofenceAttributes struct {
	Name           string  `json:"name"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	RadiusM        float64 `json:"radius"`          // metres
	Enabled        bool    `json:"enabled"`
	TriggerOnEntry bool    `json:"trigger-on-entry"`
	TriggerOnExit  bool    `json:"trigger-on-exit"`
	CreatedAt      string  `json:"created-at,omitempty"`
	UpdatedAt      string  `json:"updated-at,omitempty"`
}

// GeofencePatch describes fields to update on a Geofence. Nil fields are not sent.
type GeofencePatch struct {
	Name           *string  `json:"name,omitempty"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	RadiusM        *float64 `json:"radius,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	TriggerOnEntry *bool    `json:"trigger-on-entry,omitempty"`
	TriggerOnExit  *bool    `json:"trigger-on-exit,omitempty"`
}

// GetGeofences returns all Geofences belonging to the given Structure.
func (client *Client) GetGeofences(structureID string) (Collection[GeofenceAttributes], error) {
	raw, err := client.list("/api/structures/"+structureID+"/geofences", nil)
	if err != nil {
		return Collection[GeofenceAttributes]{}, err
	}
	return decodeCollection[GeofenceAttributes](raw)
}

// GetGeofence returns a single Geofence by ID.
func (client *Client) GetGeofence(id string) (Resource[GeofenceAttributes], error) {
	raw, err := client.get("/api/geofences/"+id, nil)
	if err != nil {
		return Resource[GeofenceAttributes]{}, err
	}
	return decodeResource[GeofenceAttributes](raw.Data)
}

// CreateGeofence creates a new Geofence belonging to the given Structure.
func (client *Client) CreateGeofence(structureID string, attrs GeofenceAttributes) (Resource[GeofenceAttributes], error) {
	raw, err := client.post("/api/geofences", "geofences", attrs, map[string]relWrapper{
		"structure": {Data: relItem{ID: structureID, Type: "structures"}},
	})
	if err != nil {
		return Resource[GeofenceAttributes]{}, err
	}
	return decodeResource[GeofenceAttributes](raw.Data)
}

// UpdateGeofence applies patch to the Geofence with the given ID.
func (client *Client) UpdateGeofence(id string, patch GeofencePatch) (Resource[GeofenceAttributes], error) {
	raw, err := client.patch("/api/geofences/"+id, id, "geofences", patch)
	if err != nil {
		return Resource[GeofenceAttributes]{}, err
	}
	return decodeResource[GeofenceAttributes](raw.Data)
}

// DeleteGeofence deletes the Geofence with the given ID.
func (client *Client) DeleteGeofence(id string) error {
	return client.deleteResource("/api/geofences/" + id)
}
