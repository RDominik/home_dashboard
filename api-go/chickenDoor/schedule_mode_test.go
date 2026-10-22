package chickendoor

import (
	"testing"
	"time"
)

// @brief Ensures the newest state topic wins over a stale controller status.
func TestSelectLatestControllerState(t *testing.T) {
	statusAt := time.Date(2026, time.October, 20, 10, 0, 0, 0, time.UTC)
	sleepAt := statusAt.Add(time.Second)

	state, receivedAt, ok := selectLatestControllerState("online", statusAt, true, "sleeping", sleepAt, true)
	if !ok || state != "sleeping" || !receivedAt.Equal(sleepAt) {
		t.Fatalf("selected state = %q at %s, available=%t; want sleeping at %s", state, receivedAt, ok, sleepAt)
	}

	state, receivedAt, ok = selectLatestControllerState("online", sleepAt, true, "sleeping", statusAt, true)
	if !ok || state != "online" || !receivedAt.Equal(sleepAt) {
		t.Fatalf("selected state = %q at %s, available=%t; want online at %s", state, receivedAt, ok, sleepAt)
	}
}

// @brief Ensures a one-second early wake waits online instead of sleeping again.
func TestScheduleEarlyWakeWithinToleranceWaitsForAction(t *testing.T) {
	plannedWake := time.Now().Add(2 * time.Second)
	sleepCommandAt := plannedWake.Add(-60 * time.Second)
	service := &ChickenDoor{
		lastControllerState: "online",
		scheduleActive:      true,
		scheduleHistory: []ScheduleHistoryEntry{{
			SleepCommandAtMs: sleepCommandAt.UnixMilli(),
			SleepSeconds:    60,
		}},
	}

	sleepingAt := sleepCommandAt.Add(time.Second)
	service.updateStateTracking("sleeping", "", sleepingAt, true)
	wokeAt := plannedWake.Add(-time.Second)
	service.updateStateTracking("online", "", wokeAt, true)

	if service.scheduleSleepPending || service.scheduleEarlyWakePending {
		t.Fatalf("early wake queued another sleep: sleepPending=%t earlyWakePending=%t", service.scheduleSleepPending, service.scheduleEarlyWakePending)
	}
	remaining := time.Until(service.scheduleWakeAt)
	if remaining < 0 || remaining > 2*time.Second {
		t.Fatalf("action wait remaining = %s, want approximately 1 second", remaining)
	}
	if service.scheduleHistory[0].SleepingAtMs != sleepingAt.UnixMilli() {
		t.Fatalf("sleep acknowledgement = %d, want %d", service.scheduleHistory[0].SleepingAtMs, sleepingAt.UnixMilli())
	}
}
