package flair

// RoomAttributes are the properties of a Flair Room within a Structure.
type RoomAttributes struct {
	Name                string  `json:"name"`
	CurrentTemperatureC float64 `json:"current-temperature-c"`
	SetPointC           float64 `json:"set-point-c"`
	SetPointF           float64 `json:"set-point-f"`
	HoldUntil           string  `json:"hold-until,omitempty"` // RFC3339; empty means no active hold
	HoldReason          string  `json:"hold-reason,omitempty"`
	PreheatPrecool      bool    `json:"preheat-precool"`
	Active              bool    `json:"active"`
	AirReturn           bool    `json:"air-return"`
	WindowsOpen         bool    `json:"windows-open"`
	Level               int     `json:"level"`
	RoomType            string  `json:"room-type,omitempty"`
	StateUpdatedAt      string  `json:"state-updated-at,omitempty"`
	CreatedAt           string  `json:"created-at,omitempty"`
	UpdatedAt           string  `json:"updated-at,omitempty"`
}

// RoomPatch describes fields to update on a Room. Nil fields are not sent.
type RoomPatch struct {
	Name           *string  `json:"name,omitempty"`
	SetPointC      *float64 `json:"set-point-c,omitempty"`
	HoldUntil      *string  `json:"hold-until,omitempty"`
	PreheatPrecool *bool    `json:"preheat-precool,omitempty"`
	Active         *bool    `json:"active,omitempty"`
	Level          *int     `json:"level,omitempty"`
}

// GetRooms returns all Rooms belonging to the given Structure.
func (client *Client) GetRooms(structureID string) (Collection[RoomAttributes], error) {
	raw, err := client.list("/api/structures/"+structureID+"/rooms", nil)
	if err != nil {
		return Collection[RoomAttributes]{}, err
	}
	return decodeCollection[RoomAttributes](raw)
}

// GetRoom returns a single Room by ID.
func (client *Client) GetRoom(id string) (Resource[RoomAttributes], error) {
	raw, err := client.get("/api/rooms/"+id, nil)
	if err != nil {
		return Resource[RoomAttributes]{}, err
	}
	return decodeResource[RoomAttributes](raw.Data)
}

// UpdateRoom applies patch to the Room with the given ID and returns the updated resource.
func (client *Client) UpdateRoom(id string, patch RoomPatch) (Resource[RoomAttributes], error) {
	raw, err := client.patch("/api/rooms/"+id, id, "rooms", patch)
	if err != nil {
		return Resource[RoomAttributes]{}, err
	}
	return decodeResource[RoomAttributes](raw.Data)
}
