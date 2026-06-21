package controller

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDeviceOf(t *testing.T) {
	device := &Device{deviceId: "d1"}
	if got := deviceOf(device); got != device {
		t.Fatalf("deviceOf(device) = %v", got)
	}
	// Home is not backed by a Device.
	home := &Home{}
	if got := deviceOf(home); got != nil {
		t.Fatalf("deviceOf(home) = %v, want nil", got)
	}
}

func TestDeviceIdFromTopic(t *testing.T) {
	cases := map[string]string{
		"yl-home/home1/d1/report": "d1",
		"yl-home/home1":           "",
		"":                        "",
	}
	for topic, want := range cases {
		if got := deviceIdFromTopic(topic); got != want {
			t.Errorf("deviceIdFromTopic(%q) = %q, want %q", topic, got, want)
		}
	}
}

func TestCallNilConnection(t *testing.T) {
	var c *connection
	if err := c.call(&Device{}, "X.y", nil, nil); err == nil {
		t.Fatal("expected error when no HTTP client is configured")
	}
}

func TestCallOverHTTP(t *testing.T) {
	// With no MQTT client, a device call is routed over HTTP and its response decoded.
	fake := newFakeYoLink(t)
	fake.handle("DoorSensor.getState", func(bddp basicDownloadDataPacket) any {
		if bddp.TargetDevice != "d1" || bddp.Token != "tok" {
			t.Errorf("device call missing target/token: %+v", bddp)
		}
		return map[string]any{"online": true}
	})
	home := fake.newHome(t)
	device := &Device{deviceId: "d1", token: "tok", deviceType: "DoorSensor"}
	device.clientContext = newClientContext(home.connection, device)

	var response struct {
		Online bool `json:"online"`
	}
	if err := device.Call("DoorSensor.getState", nil, &response); err != nil {
		t.Fatal(err)
	}
	if !response.Online {
		t.Fatal("expected online=true")
	}
}

func TestDecodeAsyncTyped(t *testing.T) {
	raw := make(chan RawResponse, 1)
	raw <- RawResponse{data: json.RawMessage(`{"online":true}`)}
	result := <-DecodeAsync[struct {
		Online bool `json:"online"`
	}](raw)
	if result.Err != nil || !result.Response.Online {
		t.Fatalf("result = %+v", result)
	}
}

func TestDecodeAsyncPropagatesError(t *testing.T) {
	raw := make(chan RawResponse, 1)
	raw <- RawResponse{err: errors.New("boom")}
	result := <-DecodeAsync[map[string]any](raw)
	if result.Err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestCallAsyncRequiresMqtt(t *testing.T) {
	c := &connection{}
	result := <-c.callAsync(&Device{deviceId: "d1"}, "X.y", nil)
	if result.err == nil {
		t.Fatal("expected error when MQTT is not initialized")
	}
}
