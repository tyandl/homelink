package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/tyandl/homelink/pkg/yolink/controller"
	"github.com/tyandl/homelink/pkg/yolink/devices"
)

func TestParseDoFlags(t *testing.T) {
	device, params, err := parseDoFlags([]string{"--on", "Front Door", "--state", "open", "--delay=5"})
	if err != nil {
		t.Fatal(err)
	}
	if device != "Front Door" {
		t.Fatalf("device = %q", device)
	}
	if params["state"] != "open" || params["delay"] != "5" {
		t.Fatalf("params = %v", params)
	}
}

func TestParseDoFlagsErrors(t *testing.T) {
	if _, _, err := parseDoFlags([]string{"bare"}); err == nil {
		t.Error("expected error for non-flag argument")
	}
	if _, _, err := parseDoFlags([]string{"--state"}); err == nil {
		t.Error("expected error for flag with no value")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "third"); got != "third" {
		t.Fatalf("got %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestIsNumber(t *testing.T) {
	for _, n := range []string{"5", "-3", "3.14", "0"} {
		if !isNumber(n) {
			t.Errorf("isNumber(%q) = false", n)
		}
	}
	for _, s := range []string{"open", "", "5x", "true"} {
		if isNumber(s) {
			t.Errorf("isNumber(%q) = true", s)
		}
	}
}

func TestCoerceParams(t *testing.T) {
	out := coerceParams(map[string]any{"delay": "5", "on": "true", "name": "porch"})
	if string(out["delay"]) != "5" {
		t.Errorf("delay = %s, want bare 5", out["delay"])
	}
	if string(out["on"]) != "true" {
		t.Errorf("on = %s, want bare true", out["on"])
	}
	if string(out["name"]) != `"porch"` {
		t.Errorf("name = %s, want quoted", out["name"])
	}
}

func TestBuildParamsNumericAndString(t *testing.T) {
	// Numeric coercion fills an int field.
	v, err := buildParams(reflect.TypeOf(devices.DoorSensorSetAttributesParams{}), map[string]any{"openRemindDelay": "30"})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Interface().(devices.DoorSensorSetAttributesParams); got.OpenRemindDelay == nil || *got.OpenRemindDelay != 30 {
		t.Fatalf("OpenRemindDelay = %v, want 30", got.OpenRemindDelay)
	}

	// A string-valued enum field decodes from its plain string.
	s, err := buildParams(reflect.TypeOf(devices.SwitchSetStateParams{}), map[string]any{"state": "open"})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Interface().(devices.SwitchSetStateParams); string(got.State) != "open" {
		t.Fatalf("State = %q", got.State)
	}
}

func TestConfigureLogging(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "warning", "error"} {
		if err := configureLogging(level); err != nil {
			t.Errorf("configureLogging(%q) = %v", level, err)
		}
	}
	if err := configureLogging("bogus"); err == nil {
		t.Error("expected error for invalid level")
	}
}

func TestEmitFormats(t *testing.T) {
	value := map[string]any{"state": "open"}

	outputFormat = "json"
	if got := captureStdout(t, func() { _ = emit(value) }); strings.TrimSpace(got) != `{"state":"open"}` {
		t.Fatalf("json emit = %q", got)
	}

	outputFormat = "go"
	if got := captureStdout(t, func() { _ = emit(value) }); !strings.Contains(got, "state:open") {
		t.Fatalf("go emit = %q", got)
	}
	outputFormat = "json" // restore default for other tests
}

// TestCallTypedMethod exercises the reflection dispatch that `do` uses: it resolves the
// concrete typed device and calls its generated method through the live Call pathway.
func TestCallTypedMethod(t *testing.T) {
	home := fakeHome(t, map[string]any{
		"Home.getDeviceList": map[string]any{"devices": []any{
			map[string]any{"deviceId": "d1", "name": "Front Door", "token": "tok", "type": "DoorSensor"},
		}},
		"DoorSensor.getState": map[string]any{
			"online":   true,
			"state":    map[string]any{"state": "open", "battery": 3, "delay": 0, "version": "1", "stateChangedAt": 1782065854130},
			"deviceId": "d1",
			"reportAt": "2026-06-21T18:17:34.130Z",
		},
	})
	device, err := home.GetDeviceByName("Front Door")
	if err != nil {
		t.Fatal(err)
	}

	result, typed, err := callTypedMethod(device, "DoorSensor.getState", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !typed {
		t.Fatal("typed = false, want true")
	}
	response, ok := result.(devices.DoorSensorGetStateResponse)
	if !ok {
		t.Fatalf("result = %T, want DoorSensorGetStateResponse", result)
	}
	if !response.Online {
		t.Error("Online = false")
	}
}

func TestCallTypedMethodNotACommand(t *testing.T) {
	home := fakeHome(t, map[string]any{
		"Home.getDeviceList": map[string]any{"devices": []any{
			map[string]any{"deviceId": "d1", "name": "Front Door", "token": "tok", "type": "DoorSensor"},
		}},
	})
	device, _ := home.GetDeviceByName("Front Door")

	// A method the device doesn't expose is not a typed command; the caller falls back to raw.
	if _, typed, err := callTypedMethod(device, "DoorSensor.bogus", nil); typed || err != nil {
		t.Fatalf("typed=%v err=%v, want false/nil", typed, err)
	}
}

// fakeHome starts a fake YoLink server returning the given per-method response data and
// returns a Home connected to it.
func fakeHome(t *testing.T, responses map[string]any) *controller.Home {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/open/yolink/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 7200})
			return
		}
		body, _ := io.ReadAll(r.Body)
		var bddp struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &bddp)
		out := map[string]any{"method": bddp.Method, "code": "000000", "time": 0}
		if data, ok := responses[bddp.Method]; ok {
			out["data"] = data
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(server.Close)

	home, err := controller.NewHome(server.URL, func() string { return "id" }, func() string { return "s" })
	if err != nil {
		t.Fatal(err)
	}
	return home
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	fn()
	_ = writer.Close()
	os.Stdout = original

	out, _ := io.ReadAll(reader)
	return string(out)
}
