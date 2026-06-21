package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// Home provides access to the home-level YoLink API. It is not a device.
type Home struct {
	deviceList  []DeviceController
	generalInfo *generalInfoResponse
	*clientContext
}

// NewHome returns a Home connected to the YoLink HTTP API, authenticating with the given
// client credentials (the access token is fetched and refreshed automatically). Its general
// info and device list are fetched lazily on first access.
func NewHome(httpHost string, clientId func() string, clientSecret func() string) (*Home, error) {
	client, err := newConnection(httpHost, clientId, clientSecret)
	if err != nil {
		return nil, err
	}
	// Build the Home, then bind the connection to it so home-wide Call/Subscribe/ParseEvent
	// route through the Home itself.
	home := &Home{}
	home.clientContext = newClientContext(client, home)
	slog.Info("yolink home initialized", "host", httpHost)
	return home, nil
}

// RestoreHome rebuilds a Home previously serialized with Save. It authenticates with the
// given client credentials (as NewHome does) but takes the HTTP host, general info, and
// device list from the saved JSON blob, so no Home.getGeneralInfo or Home.getDeviceList
// round-trips are needed.
func RestoreHome(clientId func() string, clientSecret func() string, saved []byte) (*Home, error) {
	var state savedHome
	if err := json.Unmarshal(saved, &state); err != nil {
		return nil, fmt.Errorf("restore home: %w", err)
	}
	home, err := NewHome(state.HttpHost, clientId, clientSecret)
	if err != nil {
		return nil, err
	}
	home.generalInfo = &state.GeneralInfo
	home.deviceList = home.buildDeviceControllers(state.Devices)
	return home, nil
}

// Save serializes the home to a JSON blob holding its HTTP host, general info, and device
// list, suitable for restoring later with RestoreHome (without re-fetching them over HTTP).
// Credentials are deliberately excluded; they are supplied again at restore time.
func (h *Home) Save() ([]byte, error) {
	deviceControllers, err := h.GetDeviceList()
	if err != nil {
		return nil, err
	}
	devices := make([]Device, 0, len(deviceControllers))
	for _, controller := range deviceControllers {
		container, ok := controller.(deviceContainer)
		if !ok {
			continue // not backed by a Device; nothing to serialize
		}
		devices = append(devices, *container.getDevice())
	}
	return json.Marshal(savedHome{
		HttpHost:    h.httpClient.httpHost,
		GeneralInfo: h.getGeneralInfo(),
		Devices:     devices,
	})
}

// DeviceController implementation

func (h *Home) GetId() string {
	info := h.getGeneralInfo()
	return info.Id
}
func (h *Home) GetName() string {
	info := h.getGeneralInfo()
	if info.Name == "" {
		return "Home"
	}
	return info.Name
}
func (h *Home) GetType() string      { return "Home" }
func (h *Home) GetModelName() string { return "Home" }

// ParseEvent resolves the reporting device from the message topic and delegates parsing to
// it, so a home-wide subscription types each report by its actual device.
func (h *Home) ParseEvent(message MqttMessage) (Report, bool) {
	device, err := h.GetDeviceById(deviceIdFromTopic(message.Topic))
	if err != nil {
		return Report{}, false
	}
	return device.ParseEvent(message)
}

// getGeneralInfo returns the home general info, performing the HTTP request only on the
// first call and caching the result for subsequent calls.
func (h *Home) getGeneralInfo() generalInfoResponse {
	if h.generalInfo != nil {
		// return cached value if available
		return *h.generalInfo
	}
	var response generalInfoResponse
	if err := h.Call("Home.getGeneralInfo", nil, &response); err != nil {
		slog.Warn("failed to get home general info", "error", err)
		return response
	}
	h.generalInfo = &response
	return *h.generalInfo
}

// GetDeviceList returns every device in the home, performing the HTTP request only on the
// first call and caching the result for subsequent calls.
func (h *Home) GetDeviceList() ([]DeviceController, error) {
	if h.deviceList != nil {
		return h.deviceList, nil
	}
	var response deviceListResponse
	if err := h.Call("Home.getDeviceList", nil, &response); err != nil {
		return nil, err
	}
	h.deviceList = h.buildDeviceControllers(response.Devices)
	slog.Debug("fetched device list", "count", len(h.deviceList))
	return h.deviceList, nil
}

// buildDeviceControllers wraps each Device in its registered typed controller (falling back
// to the untyped Device when no constructor is registered) and binds the home's shared
// connection to each, so every controller can issue typed Call/Subscribe requests.
func (h *Home) buildDeviceControllers(devices []Device) []DeviceController {
	deviceControllers := make([]DeviceController, len(devices))
	for i, device := range devices {
		var controller DeviceController
		if constructor, ok := deviceFactories[device.deviceType]; ok {
			controller = constructor(&device)
		} else {
			// No registered constructor for this device type, so use the untyped Device as a fallback.
			slog.Warn("no registered controller for device type; using untyped Device fallback", "deviceType", device.deviceType)
			controller = &device
		}
		device.clientContext = newClientContext(h.connection, controller) // bind the shared connection to this device's controller for typed Call/Subscribe
		deviceControllers[i] = controller
	}
	return deviceControllers
}

// GetDeviceById returns the device with the given device ID, or an error if no such device
// exists. The underlying device list is fetched at most once.
func (h *Home) GetDeviceById(deviceId string) (DeviceController, error) {
	deviceList, err := h.GetDeviceList()
	if err != nil {
		return nil, err
	}
	for _, device := range deviceList {
		if device.GetId() == deviceId {
			return device, nil
		}
	}
	return nil, fmt.Errorf("No device with ID %s", deviceId)
}

// GetDeviceByName returns the device with the given name, or an error if no such device
// exists. The underlying device list is fetched at most once.
func (h *Home) GetDeviceByName(name string) (DeviceController, error) {
	deviceList, err := h.GetDeviceList()
	if err != nil {
		return nil, err
	}
	for _, device := range deviceList {
		if device.GetName() == name {
			return device, nil
		}
	}
	return nil, fmt.Errorf("No device with name %s", name)
}

// GetDevicesByType returns all devices of the given type (e.g. "DoorSensor"). The underlying
// device list is fetched at most once.
func (h *Home) GetDevicesByType(deviceType string) ([]DeviceController, error) {
	deviceList, err := h.GetDeviceList()
	if err != nil {
		return nil, err
	}
	var matches []DeviceController
	for _, device := range deviceList {
		if device.GetType() == deviceType {
			matches = append(matches, device)
		}
	}
	return matches, nil
}

//// MQTT

// InitializeMqtt builds and connects the home's MQTT client, authenticating with the home's
// access token. It must be called before any MQTT-backed feature (device CallAsync, or
// Subscribe) is used.
func (h *Home) InitializeMqtt(mqttBroker string) error {
	return h.connection.InitializeMqtt(mqttBroker, h.GetId())
}

// CloseMqtt disconnects and releases the home's MQTT client. After this, MQTT-backed features
// are unavailable until InitializeMqtt is called again.
func (h *Home) CloseMqtt() {
	h.connection.CloseMqtt()
}

//// Support types and functions

// savedHome is the JSON wire format produced by Save and consumed by RestoreHome. It holds
// everything needed to rebuild a Home except the client credentials, which are supplied again
// at restore time.
type savedHome struct {
	HttpHost    string              `json:"httpHost"`
	GeneralInfo generalInfoResponse `json:"generalInfo"`
	Devices     []Device            `json:"devices"`
}

// generalInfoResponse holds home general info, including the home ID required for MQTT topic subscription.
type generalInfoResponse struct {
	Id   string `json:"id"`   // Home ID used as MQTT topic segment
	Name string `json:"name"` // Home name
}

// deviceListResponse is the payload of a Home.getDeviceList response.
type deviceListResponse struct {
	Devices []Device `json:"devices"`
}
