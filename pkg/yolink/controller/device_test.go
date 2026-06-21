package controller

import (
	"encoding/json"
	"testing"
)

const doorWire = `{"deviceId":"d1","deviceUDID":"u1","name":"Front Door","token":"tok","type":"DoorSensor","parentDeviceId":"p1","modelName":"YS7704","serviceZone":"us"}`

func TestDeviceUnmarshalGetters(t *testing.T) {
	var device Device
	if err := json.Unmarshal([]byte(doorWire), &device); err != nil {
		t.Fatal(err)
	}
	checks := map[string]struct{ got, want string }{
		"Id":          {device.GetId(), "d1"},
		"UDID":        {device.GetDeviceUDID(), "u1"},
		"Name":        {device.GetName(), "Front Door"},
		"Type":        {device.GetType(), "DoorSensor"},
		"Parent":      {device.GetParentDeviceId(), "p1"},
		"ModelName":   {device.GetModelName(), "YS7704"},
		"ServiceZone": {device.GetServiceZone(), "us"},
	}
	for field, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", field, c.got, c.want)
		}
	}
}

func TestDeviceJSONRoundTrip(t *testing.T) {
	var device Device
	if err := json.Unmarshal([]byte(doorWire), &device); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(&device)
	if err != nil {
		t.Fatal(err)
	}
	var again Device
	if err := json.Unmarshal(data, &again); err != nil {
		t.Fatal(err)
	}
	if again != device {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", again, device)
	}
}

func TestDeviceParseEventFallback(t *testing.T) {
	// The untyped Device wraps an unrecognized payload in Unknown.
	var device Device
	_ = json.Unmarshal([]byte(doorWire), &device)
	report, ok := device.ParseEvent(MqttMessage{Payload: []byte(`{"event":"DoorSensor.Report","data":{"state":"open"}}`)})
	if !ok {
		t.Fatal("ParseEvent returned ok=false")
	}
	unknown, isUnknown := report.Data.(Unknown)
	if !isUnknown {
		t.Fatalf("Data = %T, want Unknown", report.Data)
	}
	if report.Event != "DoorSensor.Report" {
		t.Fatalf("Event = %q", report.Event)
	}
	if string(unknown.Json) != `{"state":"open"}` {
		t.Fatalf("Unknown.Json = %s", unknown.Json)
	}
}

func TestDeviceParseEventBadPayload(t *testing.T) {
	var device Device
	if _, ok := device.ParseEvent(MqttMessage{Payload: []byte("not json")}); ok {
		t.Fatal("expected ok=false for undecodable payload")
	}
}

func TestUnknownString(t *testing.T) {
	u := Unknown{Json: json.RawMessage(`{"a":1}`)}
	if got := u.String(); got != `Unknown{"a":1}` {
		t.Fatalf("String() = %q", got)
	}
}
