package controller

import (
	"encoding/json"
)

// Device is the generic device record returned by the YoLink API. Its fields are
// unexported and read through the getter methods.
type Device struct {
	deviceId       string
	deviceUDID     string
	name           string
	token          string
	deviceType     string
	parentDeviceId string
	modelName      string
	serviceZone    string
	*clientContext
}

func (d *Device) GetId() string             { return d.deviceId }
func (d *Device) GetDeviceUDID() string     { return d.deviceUDID }
func (d *Device) GetName() string           { return d.name }
func (d *Device) GetType() string           { return d.deviceType }
func (d *Device) GetParentDeviceId() string { return d.parentDeviceId }
func (d *Device) GetModelName() string      { return d.modelName }
func (d *Device) GetServiceZone() string    { return d.serviceZone }

// getDevice returns the embedded Device itself, satisfying deviceContainer. It is promoted
// to every controller that embeds *Device, so a DeviceController can be recognized as
// device-backed (and its Device extracted) via a type assertion, without reflection.
func (d *Device) getDevice() *Device { return d }

// ParseEvent is the fallback for unknown device types: it has no typed event payloads, so
// it wraps the raw payload in Unknown. Generated device controllers override this.
func (d *Device) ParseEvent(message MqttMessage) (Report, bool) {
	event, data, ok := decodeEnvelope(message.Payload)
	if !ok {
		return Report{}, false
	}
	return Report{Device: d, Event: event, Data: Unknown{Json: data}}, true
}

// MarshalJSON renders the device in the YoLink wire format. Because Device's fields are
// unexported (and so invisible to encoding/json), encoding goes through a wire struct — the
// inverse of UnmarshalJSON — so a saved device round-trips back through it.
func (d *Device) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		DeviceId       string `json:"deviceId"`
		DeviceUDID     string `json:"deviceUDID"`
		Name           string `json:"name"`
		Token          string `json:"token"`
		Type           string `json:"type"`
		ParentDeviceId string `json:"parentDeviceId"`
		ModelName      string `json:"modelName"`
		ServiceZone    string `json:"serviceZone"`
	}{
		DeviceId:       d.deviceId,
		DeviceUDID:     d.deviceUDID,
		Name:           d.name,
		Token:          d.token,
		Type:           d.deviceType,
		ParentDeviceId: d.parentDeviceId,
		ModelName:      d.modelName,
		ServiceZone:    d.serviceZone,
	})
}

// UnmarshalJSON populates the device from the YoLink wire format. Because Device's fields
// are unexported (and so invisible to encoding/json), decoding goes through a wire struct.
func (d *Device) UnmarshalJSON(data []byte) error {
	var wire struct {
		DeviceId       string `json:"deviceId"`
		DeviceUDID     string `json:"deviceUDID"`
		Name           string `json:"name"`
		Token          string `json:"token"`
		Type           string `json:"type"`
		ParentDeviceId string `json:"parentDeviceId"`
		ModelName      string `json:"modelName"`
		ServiceZone    string `json:"serviceZone"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	d.deviceId = wire.DeviceId
	d.deviceUDID = wire.DeviceUDID
	d.name = wire.Name
	d.token = wire.Token
	d.deviceType = wire.Type
	d.parentDeviceId = wire.ParentDeviceId
	d.modelName = wire.ModelName
	d.serviceZone = wire.ServiceZone
	return nil
}

// Unknown wraps the original JSON payload of an event that has no typed response (an
// unrecognized method, or a device type with no typed controller).
type Unknown struct {
	Json json.RawMessage
}

// String renders the raw payload as its JSON text rather than a byte slice, so it prints
// readably under %v/%+v, prefixed to flag it as an unrecognized event.
func (u Unknown) String() string {
	return "Unknown" + string(u.Json)
}
