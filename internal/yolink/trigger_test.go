package yolink

import (
	"testing"

	"github.com/tyandl/yolink-api/v2/pkg/controller"
	"github.com/tyandl/yolink-api/v2/pkg/devices"
	"github.com/tyandl/yolink-api/v2/pkg/types"
)

// TestLockPredicatesMatchStatusChange guards against a real production bug: Lock devices
// report state changes via both "Lock.Alert" and "Lock.StatusChange" (LockStatusChangeResponse
// carries the same State field as LockAlertResponse), but the predicates originally only
// checked "Lock.Alert" -- so a genuine lock/unlock that arrived as a StatusChange report was
// silently ignored and its rule never fired.
func TestLockPredicatesMatchStatusChange(t *testing.T) {
	unlockedChange := controller.Report{
		Event: "Lock.StatusChange",
		Data:  devices.LockStatusChangeResponse{State: types.LockStateUnlocked},
	}
	lockedChange := controller.Report{
		Event: "Lock.StatusChange",
		Data:  devices.LockStatusChangeResponse{State: types.LockStateLocked},
	}

	if !eventPredicates["lock.unlocked"](unlockedChange) {
		t.Error("lock.unlocked should match an unlocked Lock.StatusChange report")
	}
	if eventPredicates["lock.locked"](unlockedChange) {
		t.Error("lock.locked should not match an unlocked Lock.StatusChange report")
	}
	if !eventPredicates["lock.locked"](lockedChange) {
		t.Error("lock.locked should match a locked Lock.StatusChange report")
	}
	if eventPredicates["lock.unlocked"](lockedChange) {
		t.Error("lock.unlocked should not match a locked Lock.StatusChange report")
	}
}

func TestLockPredicatesMatchAlert(t *testing.T) {
	unlockedAlert := controller.Report{
		Event: "Lock.Alert",
		Data:  devices.LockAlertResponse{State: types.LockStateUnlocked},
	}
	lockedAlert := controller.Report{
		Event: "Lock.Alert",
		Data:  devices.LockAlertResponse{State: types.LockStateLocked},
	}

	if !eventPredicates["lock.unlocked"](unlockedAlert) {
		t.Error("lock.unlocked should match an unlocked Lock.Alert report")
	}
	if !eventPredicates["lock.locked"](lockedAlert) {
		t.Error("lock.locked should match a locked Lock.Alert report")
	}
}

func TestLockPredicatesMatchV2Alert(t *testing.T) {
	unlockedAlert := controller.Report{
		Event: "LockV2.Alert",
		Data:  devices.LockV2AlertResponse{Lock: types.LockStateUnlocked},
	}
	lockedAlert := controller.Report{
		Event: "LockV2.Alert",
		Data:  devices.LockV2AlertResponse{Lock: types.LockStateLocked},
	}

	if !eventPredicates["lock.unlocked"](unlockedAlert) {
		t.Error("lock.unlocked should match an unlocked LockV2.Alert report")
	}
	if !eventPredicates["lock.locked"](lockedAlert) {
		t.Error("lock.locked should match a locked LockV2.Alert report")
	}
}

// TestLockPredicatesIgnoreUnrelatedEvents guards the other direction: a report that merely
// carries lock-shaped data under a different event name (or a different device's report data
// entirely) must not match.
func TestLockPredicatesIgnoreUnrelatedEvents(t *testing.T) {
	wrongEventName := controller.Report{
		Event: "Lock.Report", // LockReportResponse, not Alert/StatusChange
		Data:  devices.LockReportResponse{State: types.LockStateUnlocked},
	}
	if eventPredicates["lock.unlocked"](wrongEventName) || eventPredicates["lock.locked"](wrongEventName) {
		t.Error("lock predicates should not match Lock.Report")
	}
}
