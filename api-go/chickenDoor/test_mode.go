package chickendoor

import (
	"fmt"
	"log"
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
// advance the deadline by one full configured interval and alternate direction.
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
	if controllerState != "online" || !h.motorRunningSince.IsZero() {
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

	h.mu.Lock()
	// Record the same motor lifecycle fields used by normal commands, allowing
	// the existing auto-stop tick and history accounting to remain authoritative.
	h.motorRunningSince = time.Now()
	h.motorRunningAction = action
	h.motorLimitReachedAt = time.Time{}
	if action == "open" {
		h.testModeNextAction = "close"
	} else {
		h.testModeNextAction = "open"
	}
	h.testModeNextAt = now.Add(time.Duration(h.testModeIntervalMinutes) * time.Minute)
	nextAction := h.testModeNextAction
	nextAt := h.testModeNextAt
	h.mu.Unlock()
	h.persistState()
	log.Printf("[chickendoor-test-mode] executed %s; next action %s at %s", action, nextAction, formatTestModeTime(nextAt))
}
