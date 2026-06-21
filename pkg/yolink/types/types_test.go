package types

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

func TestTimestampRoundTrip(t *testing.T) {
	// The API encodes timestamps as Unix epoch milliseconds.
	original := Timestamp{Time: time.UnixMilli(1782065854130)}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "1782065854130" {
		t.Fatalf("marshaled = %s, want 1782065854130", data)
	}

	var restored Timestamp
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.Equal(original.Time) {
		t.Fatalf("restored = %v, want %v", restored.Time, original.Time)
	}
}

func TestTimestampUnmarshalRejectsNonNumber(t *testing.T) {
	var ts Timestamp
	if err := json.Unmarshal([]byte(`"nope"`), &ts); err == nil {
		t.Fatal("expected error unmarshaling a non-numeric timestamp")
	}
}

func TestBatteryLevelOrdinalRoundTrip(t *testing.T) {
	// BatteryLevel is sent on the wire as an ordinal 0-4 but modeled as a named string.
	cases := map[BatteryLevel]string{
		BatteryLevelCritical: "0",
		BatteryLevelLow:      "1",
		BatteryLevelMedium:   "2",
		BatteryLevelHigh:     "3",
		BatteryLevelFull:     "4",
	}
	for level, wantOrdinal := range cases {
		data, err := json.Marshal(level)
		if err != nil {
			t.Fatalf("marshal %s: %v", level, err)
		}
		if string(data) != wantOrdinal {
			t.Fatalf("marshal %s = %s, want %s", level, data, wantOrdinal)
		}
		var got BatteryLevel
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if got != level {
			t.Fatalf("round-trip = %s, want %s", got, level)
		}
	}
}

func TestBatteryLevelUnmarshalOutOfRange(t *testing.T) {
	var level BatteryLevel
	if err := json.Unmarshal([]byte("5"), &level); err == nil {
		t.Fatal("expected out-of-range ordinal to error")
	}
}

func TestBatteryLevelMarshalUnknown(t *testing.T) {
	if _, err := json.Marshal(BatteryLevel("bogus")); err == nil {
		t.Fatal("expected unknown BatteryLevel to error on marshal")
	}
}

func TestTemperatureUnitUnmarshalUppercases(t *testing.T) {
	var unit TemperatureUnit
	if err := json.Unmarshal([]byte(`"c"`), &unit); err != nil {
		t.Fatal(err)
	}
	if unit != Celsius {
		t.Fatalf("unit = %q, want C", unit)
	}
}

func TestTemperatureConversions(t *testing.T) {
	freezing := NewTemperature(0, Celsius)
	if got := freezing.ToF(); got.Value() != 32 || got.Unit() != Fahrenheit {
		t.Fatalf("0C -> %v, want 32F", got)
	}
	boiling := NewTemperature(212, Fahrenheit)
	if got := boiling.ToC(); got.Value() != 100 || got.Unit() != Celsius {
		t.Fatalf("212F -> %v, want 100C", got)
	}
	// Converting to the same unit is a no-op.
	if got := freezing.ToC(); got.Value() != 0 {
		t.Fatalf("0C -> ToC = %v, want 0", got)
	}
}

func TestTemperatureString(t *testing.T) {
	if got := NewTemperature(21.5, Celsius).String(); got != "21.5°C" {
		t.Fatalf("String = %q, want 21.5°C", got)
	}
}

func TestTemperatureJSON(t *testing.T) {
	// Unmarshaling a bare number yields a Celsius temperature; marshaling emits the number.
	var temp Temperature
	if err := json.Unmarshal([]byte("18.5"), &temp); err != nil {
		t.Fatal(err)
	}
	if temp.Value() != 18.5 || temp.Unit() != Celsius {
		t.Fatalf("unmarshaled = %v", temp)
	}
	data, err := json.Marshal(temp)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "18.5" {
		t.Fatalf("marshaled = %s, want 18.5", data)
	}
}

func TestStatusCodeString(t *testing.T) {
	if got := StatusSuccess.String(); got != "Success" {
		t.Fatalf("StatusSuccess.String() = %q", got)
	}
	if got := StatusCode("123456").String(); got != "unknown status code: 123456" {
		t.Fatalf("unknown.String() = %q", got)
	}
}

func TestStatusCodeIsSuccess(t *testing.T) {
	if !StatusSuccess.IsSuccess() {
		t.Fatal("StatusSuccess.IsSuccess() = false")
	}
	if StatusDeviceBusy.IsSuccess() {
		t.Fatal("StatusDeviceBusy.IsSuccess() = true")
	}
}

func TestNewMessageId(t *testing.T) {
	uuidV4 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := NewMessageId()
		if !uuidV4.MatchString(id) {
			t.Fatalf("NewMessageId() = %q, not a v4 UUID", id)
		}
		if seen[id] {
			t.Fatalf("NewMessageId() produced a duplicate: %q", id)
		}
		seen[id] = true
	}
}

func TestLoraInfoString(t *testing.T) {
	info := LoraInfo{NetId: "010204", DevNetType: "A", Signal: -70, GatewayId: "gw1", Gateways: 2}
	want := "{NetId:010204 DevNetType:A Signal:-70 GatewayId:gw1 Gateways:2}"
	if got := info.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestAuthResponseUnmarshal(t *testing.T) {
	var auth AuthResponse
	if err := json.Unmarshal([]byte(`{"access_token":"abc","token_type":"bearer","expires_in":7200,"scope":["create"]}`), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.AccessToken != "abc" || auth.ExpiresIn != 7200 || len(auth.Scope) != 1 {
		t.Fatalf("unmarshaled = %+v", auth)
	}
}
