package flair

// ScheduleAttributes are the properties of a Flair Schedule.
type ScheduleAttributes struct {
	Name          string   `json:"name"`
	DaysOfWeek    []string `json:"days-of-week,omitempty"` // e.g. ["Monday","Wednesday"]
	IsAutoSchedule bool    `json:"is-auto-schedule"`
	CreatedAt     string   `json:"created-at,omitempty"`
	UpdatedAt     string   `json:"updated-at,omitempty"`
}

// SchedulePatch describes fields to update on a Schedule. Nil fields are not sent.
type SchedulePatch struct {
	Name       *string   `json:"name,omitempty"`
	DaysOfWeek []string  `json:"days-of-week,omitempty"`
}

// ScheduleEventAttributes are the properties of a time-based event within a Schedule.
type ScheduleEventAttributes struct {
	StartHour     int     `json:"start-hour"`
	StartMinute   int     `json:"start-minute"`
	TemperatureC  float64 `json:"temperature-c"`
	CreatedAt     string  `json:"created-at,omitempty"`
	UpdatedAt     string  `json:"updated-at,omitempty"`
}

// ScheduleEventPatch describes fields to update on a ScheduleEvent. Nil fields are not sent.
type ScheduleEventPatch struct {
	StartHour    *int     `json:"start-hour,omitempty"`
	StartMinute  *int     `json:"start-minute,omitempty"`
	TemperatureC *float64 `json:"temperature-c,omitempty"`
}

// GetSchedules returns all Schedules belonging to the given Structure.
func (client *Client) GetSchedules(structureID string) (Collection[ScheduleAttributes], error) {
	raw, err := client.list("/api/structures/"+structureID+"/schedules", nil)
	if err != nil {
		return Collection[ScheduleAttributes]{}, err
	}
	return decodeCollection[ScheduleAttributes](raw)
}

// GetSchedule returns a single Schedule by ID.
func (client *Client) GetSchedule(id string) (Resource[ScheduleAttributes], error) {
	raw, err := client.get("/api/schedules/"+id, nil)
	if err != nil {
		return Resource[ScheduleAttributes]{}, err
	}
	return decodeResource[ScheduleAttributes](raw.Data)
}

// CreateSchedule creates a new Schedule belonging to the given Structure.
func (client *Client) CreateSchedule(structureID string, attrs ScheduleAttributes) (Resource[ScheduleAttributes], error) {
	raw, err := client.post("/api/schedules", "schedules", attrs, map[string]relWrapper{
		"structure": {Data: relItem{ID: structureID, Type: "structures"}},
	})
	if err != nil {
		return Resource[ScheduleAttributes]{}, err
	}
	return decodeResource[ScheduleAttributes](raw.Data)
}

// UpdateSchedule applies patch to the Schedule with the given ID.
func (client *Client) UpdateSchedule(id string, patch SchedulePatch) (Resource[ScheduleAttributes], error) {
	raw, err := client.patch("/api/schedules/"+id, id, "schedules", patch)
	if err != nil {
		return Resource[ScheduleAttributes]{}, err
	}
	return decodeResource[ScheduleAttributes](raw.Data)
}

// DeleteSchedule deletes the Schedule with the given ID.
func (client *Client) DeleteSchedule(id string) error {
	return client.deleteResource("/api/schedules/" + id)
}

// GetScheduleEvents returns all events belonging to the given Schedule.
func (client *Client) GetScheduleEvents(scheduleID string) (Collection[ScheduleEventAttributes], error) {
	raw, err := client.list("/api/schedules/"+scheduleID+"/schedule-events", nil)
	if err != nil {
		return Collection[ScheduleEventAttributes]{}, err
	}
	return decodeCollection[ScheduleEventAttributes](raw)
}

// CreateScheduleEvent creates a new event within the given Schedule.
func (client *Client) CreateScheduleEvent(scheduleID string, attrs ScheduleEventAttributes) (Resource[ScheduleEventAttributes], error) {
	raw, err := client.post("/api/schedule-events", "schedule-events", attrs, map[string]relWrapper{
		"schedule": {Data: relItem{ID: scheduleID, Type: "schedules"}},
	})
	if err != nil {
		return Resource[ScheduleEventAttributes]{}, err
	}
	return decodeResource[ScheduleEventAttributes](raw.Data)
}

// UpdateScheduleEvent applies patch to the ScheduleEvent with the given ID.
func (client *Client) UpdateScheduleEvent(id string, patch ScheduleEventPatch) (Resource[ScheduleEventAttributes], error) {
	raw, err := client.patch("/api/schedule-events/"+id, id, "schedule-events", patch)
	if err != nil {
		return Resource[ScheduleEventAttributes]{}, err
	}
	return decodeResource[ScheduleEventAttributes](raw.Data)
}

// DeleteScheduleEvent deletes the ScheduleEvent with the given ID.
func (client *Client) DeleteScheduleEvent(id string) error {
	return client.deleteResource("/api/schedule-events/" + id)
}
