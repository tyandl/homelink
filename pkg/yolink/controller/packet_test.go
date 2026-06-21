package controller

import "testing"

func TestNewBDDPHomeLevel(t *testing.T) {
	// A home-level call carries no targetDevice/token.
	bddp := newBDDP("Home.getDeviceList", nil, nil, nil)
	if bddp.Method != "Home.getDeviceList" {
		t.Fatalf("Method = %q", bddp.Method)
	}
	if bddp.TargetDevice != "" || bddp.Token != "" {
		t.Fatalf("expected empty target/token, got %q/%q", bddp.TargetDevice, bddp.Token)
	}
	if bddp.Time.IsZero() {
		t.Fatal("Time not set")
	}
}

func TestNewBDDPDeviceLevel(t *testing.T) {
	deviceId, token := "dev1", "tok1"
	params := map[string]string{"state": "open"}
	bddp := newBDDP("Switch.setState", params, &deviceId, &token)
	if bddp.TargetDevice != "dev1" || bddp.Token != "tok1" {
		t.Fatalf("target/token = %q/%q", bddp.TargetDevice, bddp.Token)
	}
	if bddp.Params == nil {
		t.Fatal("Params not set")
	}
}
