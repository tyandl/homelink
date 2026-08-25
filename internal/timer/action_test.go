package timer

import "testing"

func decodeActionTOML(t *testing.T, tomlSrc string) (*ActionConfig, error) {
	t.Helper()
	meta, prim := decodeTestPrimitive(t, tomlSrc)
	return DecodeAction(meta, prim)
}

func TestDecodeActionSet(t *testing.T) {
	ac, err := decodeActionTOML(t, `device = "wash cycle"
action = "set"
duration = "45m"`)
	if err != nil {
		t.Fatalf("DecodeAction: %v", err)
	}
	if ac.Device != "wash cycle" || ac.Action != "set" || ac.Duration != "45m" {
		t.Errorf("unexpected action: %+v", ac)
	}
}

func TestDecodeActionCancel(t *testing.T) {
	ac, err := decodeActionTOML(t, `device = "wash cycle"
action = "cancel"`)
	if err != nil {
		t.Fatalf("DecodeAction: %v", err)
	}
	if ac.Device != "wash cycle" || ac.Action != "cancel" {
		t.Errorf("unexpected action: %+v", ac)
	}
}

func TestDecodeActionSetRequiresDuration(t *testing.T) {
	_, err := decodeActionTOML(t, `device = "wash cycle"
action = "set"`)
	if err == nil {
		t.Fatal("expected error for missing duration, got nil")
	}
}

func TestDecodeActionSetRejectsInvalidDuration(t *testing.T) {
	_, err := decodeActionTOML(t, `device = "wash cycle"
action = "set"
duration = "not-a-duration"`)
	if err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
}

func TestDecodeActionSetRejectsNonPositiveDuration(t *testing.T) {
	_, err := decodeActionTOML(t, `device = "wash cycle"
action = "set"
duration = "-5m"`)
	if err == nil {
		t.Fatal("expected error for non-positive duration, got nil")
	}
}

func TestDecodeActionCancelRejectsDuration(t *testing.T) {
	_, err := decodeActionTOML(t, `device = "wash cycle"
action = "cancel"
duration = "5m"`)
	if err == nil {
		t.Fatal("expected error for duration set with action=cancel, got nil")
	}
}

func TestDecodeActionRequiresDevice(t *testing.T) {
	_, err := decodeActionTOML(t, `action = "cancel"`)
	if err == nil {
		t.Fatal("expected error for missing device, got nil")
	}
}

func TestDecodeActionUnknownAction(t *testing.T) {
	_, err := decodeActionTOML(t, `device = "x"
action = "bogus"`)
	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
}
