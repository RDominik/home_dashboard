package chickendoor

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// @brief Validates the strict HH:MM format used by the test-mode daily window.
// @details
// The test mode intentionally accepts minute precision only, unlike the normal
// sleep schedule which also accepts seconds. Rejecting seconds keeps UI input,
// persisted values, and window boundary comparisons deterministic.
// @param value Candidate local-time string.
// @return true when value parses as a valid HH:MM time.
func isValidTestModeTime(value string) bool {
	_, err := time.Parse("15:04", strings.TrimSpace(value))
	return err == nil
}

// @brief Formats a test-mode deadline using the local schedule timezone.
// @param value Deadline to display.
// @return Local HH:MM:SS timestamp, or an empty string when no deadline exists.
func formatTestModeTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(scheduleNow().Location()).Format("15:04:05")
}

// @brief Formats a timestamp for bbolt persistence, returning empty for zero time.
// @param value Timestamp to serialize.
// @return RFC3339 timestamp string or an empty string.
func formatRFC3339OrEmpty(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

// @brief Calculates the sleep duration needed to reach a test-cycle deadline.
// @details
// The controller receives whole seconds. The value is rounded upward to avoid
// waking before the scheduled action time, while a minimum of one second avoids
// issuing an immediate zero-duration sleep when the awake allowance consumed
// the complete configured interval.
// @param now Current timestamp in the schedule timezone.
// @param deadline Intended timestamp of the next alternating test action.
// @return Positive sleep duration in seconds, rounded up and clamped to at least one.
func testModeSleepSecondsUntil(now, deadline time.Time) int {
	seconds := int(math.Ceil(deadline.Sub(now).Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

// @brief Determines whether a local wall-clock time falls inside the test window.
// @details
// The start boundary is inclusive and the end boundary is exclusive. A window
// whose start is later than its end spans midnight; equal boundaries represent
// an empty window rather than an accidental all-day motor test.
// @param now Current time in any location; it is converted to the schedule zone.
// @param start Configured HH:MM start boundary.
// @param end Configured HH:MM end boundary.
// @return true when now is within the configured daily window.
func isWithinTestModeWindow(now time.Time, start, end string) bool {
	startTime, startErr := time.Parse("15:04", start)
	endTime, endErr := time.Parse("15:04", end)
	if startErr != nil || endErr != nil {
		return false
	}

	localNow := now.In(scheduleNow().Location())
	currentMinute := localNow.Hour()*60 + localNow.Minute()
	startMinute := startTime.Hour()*60 + startTime.Minute()
	endMinute := endTime.Hour()*60 + endTime.Minute()
	if startMinute == endMinute {
		return false
	}
	if startMinute < endMinute {
		return currentMinute >= startMinute && currentMinute < endMinute
	}
	return currentMinute >= startMinute || currentMinute < endMinute
}

// @brief Executes one eligible alternating test-mode motor action per interval.
// @details
// Test mode is deliberately independent of schedule activation: an active
// timestamp schedule pauses test actions, but this routine never publishes or
// changes nano/esp32/schedule_active. Actions require an online controller and
// an idle motor. When a controller is unavailable at a deadline, the due action
// remains pending and can run once it is online again; successful commands
// alternate direction and request controller sleep after the motor has
// finished. The next action deadline is measured from the previous motor
// command. The controller stays awake for the configured maximum-awake
// duration after that command, then sleeps only for the remaining interval.
// When the awake duration reaches or exceeds the interval, a one-second sleep
// is used as the minimum practical sleep before the next action.
// Exiting the daily window clears the deadline, so the next day starts with a
// complete interval instead of replaying a missed action. Motor runtime and
// local auto-stop continue to use the same directional enforcement as manual
// and scheduled movement.
func (h *ChickenDoor) testModeTick() {
	now := scheduleNow()
	msgs := h.mqttManager.Messages()
	liveControllerState := normalizeControllerState(toString(msgs["status"]))

	h.mu.Lock()
	if !h.testModeEnabled || h.scheduleActive {
		h.mu.Unlock()
		return
	}
	if !isWithinTestModeWindow(now, h.testModeStartTime, h.testModeEndTime) {
		h.testModeNextAt = time.Time{}
		h.mu.Unlock()
		return
	}
	// Do not arm a second interval while the current motor action is still in
	// progress or while its post-movement sleep command is waiting to be sent.
	if !h.motorRunningSince.IsZero() || h.testModeSleepPending {
		h.mu.Unlock()
		return
	}
	if h.testModeNextAt.IsZero() {
		interval := time.Duration(h.testModeIntervalMinutes) * time.Minute
		h.testModeNextAt = now.Add(interval)
		h.mu.Unlock()
		// Persist the first deadline so a backend restart during the interval
		// does not silently restart the full wait period.
		h.persistState()
		return
	}
	if now.Before(h.testModeNextAt) {
		h.mu.Unlock()
		return
	}
	controllerState := liveControllerState
	if controllerState == "" {
		controllerState = normalizeControllerState(h.lastStatusController)
	}
	sleepState := normalizeControllerState(toString(firstValue(msgs, "sleepms/status", "sleepms_status")))
	if controllerState != "online" || sleepState == "sleeping" || sleepState == "offline" {
		h.mu.Unlock()
		return
	}

	action := h.testModeNextAction
	if action != "open" && action != "close" {
		action = "open"
	}
	runtimeSeconds := h.motorAutoStopOpenSeconds
	if action == "close" {
		runtimeSeconds = h.motorAutoStopCloseSeconds
	}
	maxAwakeSeconds := h.testModeMaxAwakeSeconds
	interval := h.testModeIntervalMinutes
	if maxAwakeSeconds < runtimeSeconds {
		maxAwakeSeconds = runtimeSeconds
	}
	if interval < 1 {
		interval = 1
	}
	h.mu.Unlock()

	// Publish the controller-side runtime before the movement command, matching
	// the established manual and scheduled motor-command contract.
	if err := h.publishMotorRuntime(action, runtimeSeconds); err != nil {
		log.Printf("[chickendoor-test-mode] runtime publish failed before %s: %v", action, err)
		return
	}
	if err := h.mqttManager.Publish(fmt.Sprintf("%s/engine", nanoSetPrefix), action); err != nil {
		log.Printf("[chickendoor-test-mode] action %s failed: %v", action, err)
		return
	}

	actionAt := time.Now()
	msgs = h.mqttManager.Messages()
	battery := ""
	if value, ok := msgs["battery_percent"]; ok && value != nil {
		battery = fmt.Sprintf("%v", value)
	}
	h.mu.Lock()
	// Record the same motor lifecycle fields used by normal commands, allowing
	// the existing auto-stop tick and history accounting to remain authoritative.
	h.motorRunningSince = actionAt
	h.motorRunningAction = action
	h.motorLimitReachedAt = time.Time{}
	h.testModeSleepAt = actionAt.Add(time.Duration(maxAwakeSeconds) * time.Second)
	deadline := actionAt.Add(time.Duration(interval) * time.Minute)
	h.testModeNextAt = deadline
	if action == "open" {
		h.testModeNextAction = "close"
	} else {
		h.testModeNextAction = "open"
	}
	h.testModeSleepPending = true
	h.testModeHistory = append(h.testModeHistory, TestModeHistoryEntry{
		Action: action, ActionAtMs: actionAt.UnixMilli(), BatteryPercent: battery,
		MaxAwakeSeconds: maxAwakeSeconds,
	})
	if len(h.testModeHistory) > scheduleHistorySize {
		h.testModeHistory = h.testModeHistory[len(h.testModeHistory)-scheduleHistorySize:]
	}
	nextAction := h.testModeNextAction
	sleepAt := h.testModeSleepAt
	h.mu.Unlock()
	h.persistState()
	log.Printf("[chickendoor-test-mode] executed %s; motor awake limit at %s, next action %s", action, formatTestModeTime(sleepAt), nextAction)
}

// @brief Puts the controller to sleep after a test-mode motor action completes.
// @details
// The pending flag remains set if MQTT publication fails, allowing the regular
// one-second tick to retry without sending duplicate motor commands. The
// persisted next-action deadline is established when the motor command is
// issued; after the maximum-awake allowance, sleep covers only the remaining
// interval. Thus backend eligibility and the controller's requested wake stay
// aligned across process restarts.
// @param h ChickenDoor service whose test-mode movement has completed.
func (h *ChickenDoor) publishTestModeSleep() {
	h.mu.Lock()
	if !h.testModeSleepPending || !h.testModeEnabled {
		h.mu.Unlock()
		return
	}
	sleepAt := h.testModeSleepAt
	deadline := h.testModeNextAt
	motorRunning := !h.motorRunningSince.IsZero()
	h.mu.Unlock()
	if motorRunning || (!sleepAt.IsZero() && time.Now().Before(sleepAt)) {
		return
	}
	// Keep the requested action-to-action cadence: the awake allowance is a
	// phase within the interval rather than extra time added before it.
	sleepSeconds := testModeSleepSecondsUntil(scheduleNow(), deadline)
	sleepMilliseconds := sleepSeconds * 1000
	if err := h.mqttManager.Publish(fmt.Sprintf("%s/sleepms", nanoSetPrefix), sleepMilliseconds); err != nil {
		log.Printf("[chickendoor-test-mode] sleep publish failed; retrying: %v", err)
		return
	}

	h.mu.Lock()
	// Settings may have been disabled while the MQTT publish was in flight. The
	// device command has already been accepted, so clear pending and preserve the
	// actual wake deadline regardless; a disabled mode will simply ignore it.
	h.testModeSleepPending = false
	h.testModeNextAt = deadline
	h.testModeSleepAt = time.Time{}
	h.lastSleepCommandAt = time.Now()
	if count := len(h.testModeHistory); count > 0 {
		row := &h.testModeHistory[count-1]
		row.SleepSeconds = sleepSeconds
		row.SleepCommandAtMs = h.lastSleepCommandAt.UnixMilli()
	}
	h.mu.Unlock()
	h.persistState()
	log.Printf("[chickendoor-test-mode] controller sleeping for %d seconds; next action at %s", sleepSeconds, formatTestModeTime(deadline))
}
