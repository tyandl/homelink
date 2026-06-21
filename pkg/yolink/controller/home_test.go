package controller

import (
	"encoding/json"
	"testing"
)

// seedHome registers a general-info and device-list handler and returns a connected Home.
func seedHome(t *testing.T, fake *fakeYoLink) *Home {
	fake.handle("Home.getGeneralInfo", func(basicDownloadDataPacket) any {
		return map[string]any{"id": "home1", "name": "Test Home"}
	})
	fake.handle("Home.getDeviceList", func(basicDownloadDataPacket) any {
		return map[string]any{"devices": []any{
			deviceJSON("d1", "Front Door", "DoorSensor"),
			deviceJSON("d2", "Lamp", "Switch"),
			deviceJSON("d3", "Back Door", "DoorSensor"),
		}}
	})
	return fake.newHome(t)
}

func TestHomeGeneralInfoCachedAndAccessors(t *testing.T) {
	fake := newFakeYoLink(t)
	home := seedHome(t, fake)

	if home.GetName() != "Test Home" || home.GetId() != "home1" {
		t.Fatalf("name/id = %q/%q", home.GetName(), home.GetId())
	}
	if home.GetType() != "Home" || home.GetModelName() != "Home" {
		t.Fatalf("type/model = %q/%q", home.GetType(), home.GetModelName())
	}
	// getGeneralInfo caches after the first call: a second access issues no new request.
	_ = home.GetId()
	count := 0
	for _, request := range fake.requests {
		if request.Method == "Home.getGeneralInfo" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("getGeneralInfo requested %d times, want 1", count)
	}
}

func TestHomeGetDeviceListAndLookups(t *testing.T) {
	fake := newFakeYoLink(t)
	home := seedHome(t, fake)

	devices, err := home.GetDeviceList()
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 3 {
		t.Fatalf("len = %d, want 3", len(devices))
	}

	byId, err := home.GetDeviceById("d2")
	if err != nil || byId.GetName() != "Lamp" {
		t.Fatalf("GetDeviceById = %v, %v", byId, err)
	}
	byName, err := home.GetDeviceByName("Front Door")
	if err != nil || byName.GetId() != "d1" {
		t.Fatalf("GetDeviceByName = %v, %v", byName, err)
	}
	doors, err := home.GetDevicesByType("DoorSensor")
	if err != nil || len(doors) != 2 {
		t.Fatalf("GetDevicesByType = %d, %v", len(doors), err)
	}

	if _, err := home.GetDeviceById("nope"); err == nil {
		t.Fatal("expected error for unknown id")
	}
	if _, err := home.GetDeviceByName("nope"); err == nil {
		t.Fatal("expected error for unknown name")
	}

	// The list is cached: a second call does not re-request it.
	if _, err := home.GetDeviceList(); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, request := range fake.requests {
		if request.Method == "Home.getDeviceList" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("getDeviceList requested %d times, want 1", count)
	}
}

func TestHomeSaveRestoreRoundTrip(t *testing.T) {
	fake := newFakeYoLink(t)
	home := seedHome(t, fake)

	blob, err := home.Save()
	if err != nil {
		t.Fatal(err)
	}
	// The saved blob carries the host, general info, and devices.
	var saved savedHome
	if err := json.Unmarshal(blob, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.HttpHost != fake.server.URL || saved.GeneralInfo.Id != "home1" || len(saved.Devices) != 3 {
		t.Fatalf("saved = %+v", saved)
	}

	// Restoring must not need any HTTP round-trips for general info or the device list.
	before := len(fake.requests)
	restored, err := RestoreHome(func() string { return "id" }, func() string { return "s" }, blob)
	if err != nil {
		t.Fatal(err)
	}
	if restored.GetId() != "home1" {
		t.Fatalf("restored id = %q", restored.GetId())
	}
	devices, err := restored.GetDeviceList()
	if err != nil || len(devices) != 3 {
		t.Fatalf("restored devices = %d, %v", len(devices), err)
	}
	// RestoreHome re-authenticates (one token fetch) but must not hit the API endpoint.
	if got := len(fake.requests) - before; got != 0 {
		t.Fatalf("restore issued %d API requests, want 0 (%s)", got, fake.requestMethods())
	}
}

func TestHomeBuildDeviceControllersFallback(t *testing.T) {
	// An unregistered device type falls back to the untyped *Device controller.
	home := &Home{}
	home.clientContext = newClientContext(&connection{}, home)
	devices := home.buildDeviceControllers([]Device{{deviceId: "x", deviceType: "NoSuchType", name: "Mystery"}})
	if len(devices) != 1 {
		t.Fatalf("len = %d", len(devices))
	}
	if _, ok := devices[0].(*Device); !ok {
		t.Fatalf("controller = %T, want *Device fallback", devices[0])
	}
}
