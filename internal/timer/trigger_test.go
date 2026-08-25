package timer

import "testing"

func decodeTriggerTOML(t *testing.T, tomlSrc string) (TriggerConfig, error) {
	t.Helper()
	meta, prim := decodeTestPrimitive(t, tomlSrc)
	return DecodeTrigger(meta, prim)
}

func TestDecodeTriggerExpired(t *testing.T) {
	tc, err := decodeTriggerTOML(t, `device = "wash cycle"
event = "expired"`)
	if err != nil {
		t.Fatalf("DecodeTrigger: %v", err)
	}
	if tc.Device != "wash cycle" || tc.Event != "expired" {
		t.Errorf("unexpected trigger: %+v", tc)
	}
}

func TestDecodeTriggerTimeOfDay(t *testing.T) {
	tc, err := decodeTriggerTOML(t, `device = "afternoon"
event = "time_of_day"
at = "16:00"`)
	if err != nil {
		t.Fatalf("DecodeTrigger: %v", err)
	}
	if tc.Device != "afternoon" || tc.Event != "time_of_day" || tc.At != "16:00" {
		t.Errorf("unexpected trigger: %+v", tc)
	}
}

func TestDecodeTriggerTimeOfDayRequiresAt(t *testing.T) {
	_, err := decodeTriggerTOML(t, `device = "afternoon"
event = "time_of_day"`)
	if err == nil {
		t.Fatal("expected error for missing at, got nil")
	}
}

func TestDecodeTriggerTimeOfDayInvalidAtFormat(t *testing.T) {
	_, err := decodeTriggerTOML(t, `device = "afternoon"
event = "time_of_day"
at = "4pm"`)
	if err == nil {
		t.Fatal("expected error for invalid at format, got nil")
	}
}

func TestDecodeTriggerExpiredRejectsAt(t *testing.T) {
	_, err := decodeTriggerTOML(t, `device = "wash cycle"
event = "expired"
at = "16:00"`)
	if err == nil {
		t.Fatal("expected error for at set with event=expired, got nil")
	}
}

func TestDecodeTriggerRequiresDevice(t *testing.T) {
	_, err := decodeTriggerTOML(t, `event = "expired"`)
	if err == nil {
		t.Fatal("expected error for missing device, got nil")
	}
}

func TestDecodeTriggerUnknownEvent(t *testing.T) {
	_, err := decodeTriggerTOML(t, `device = "x"
event = "bogus"`)
	if err == nil {
		t.Fatal("expected error for unknown event, got nil")
	}
}
