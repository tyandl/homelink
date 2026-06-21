package controller

// deviceFactories maps a device Type (e.g. "DoorSensor") to a constructor that builds the
// typed controller for that type. Generated device files register themselves here from
// their init funcs.
var deviceFactories = map[string]func(device *Device) DeviceController{}

// RegisterDeviceFactory records the controller constructor for a device Type. Generated
// device files (in the devices package) call this from their init funcs.
func RegisterDeviceFactory(deviceType string, constructor func(device *Device) DeviceController) {
	deviceFactories[deviceType] = constructor
}

// RegisteredDeviceTypes returns the device types that have a registered controller
// constructor — every type the library can build a typed controller for. The generated
// devices package populates these from its init funcs.
func RegisteredDeviceTypes() []string {
	deviceTypes := make([]string, 0, len(deviceFactories))
	for deviceType := range deviceFactories {
		deviceTypes = append(deviceTypes, deviceType)
	}
	return deviceTypes
}
