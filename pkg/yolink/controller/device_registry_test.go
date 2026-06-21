package controller

import "testing"

func TestRegisterDeviceFactory(t *testing.T) {
	const deviceType = "TestRegistryWidget"
	// Sanity: not registered to begin with.
	if _, ok := deviceFactories[deviceType]; ok {
		t.Fatalf("%s unexpectedly pre-registered", deviceType)
	}
	t.Cleanup(func() { delete(deviceFactories, deviceType) })

	RegisterDeviceFactory(deviceType, func(device *Device) DeviceController { return device })
	constructor, ok := deviceFactories[deviceType]
	if !ok {
		t.Fatal("factory not registered")
	}
	device := &Device{deviceId: "w1"}
	if got := constructor(device); got.GetId() != "w1" {
		t.Fatalf("constructed controller id = %q", got.GetId())
	}
}
