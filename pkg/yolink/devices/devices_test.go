package devices_test

import (
	"path/filepath"
	"testing"

	"github.com/tyandl/homelink/pkg/yolink/controller"
	// Imported for its init side effects: every generated device file registers its controller
	// with the controller registry. This test validates that wiring rather than the generated
	// device behavior itself.
	_ "github.com/tyandl/homelink/pkg/yolink/devices"
)

// TestDeviceRegistryMatchesDefinitions confirms that importing the devices package builds the
// controller registry, with exactly one registered device type per device definition file.
func TestDeviceRegistryMatchesDefinitions(t *testing.T) {
	definitions, err := filepath.Glob("*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) == 0 {
		t.Fatal("found no device definition (*.json) files")
	}

	registered := controller.RegisteredDeviceTypes()
	if len(registered) != len(definitions) {
		t.Fatalf("registry has %d device types, but there are %d device definition files",
			len(registered), len(definitions))
	}
}
