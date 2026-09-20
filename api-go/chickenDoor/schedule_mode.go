package chickendoor

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const scheduleActiveTopic = nanoSetPrefix + "/schedule_active"
const scheduleRetryDelay = 5 * time.Second
const scheduleHistorySize = 20
const defaultScheduleTimezone = "Europe/Berlin"
const scheduleEarlyWakeTolerance = 2 * time.Minute

var scheduleLocation = struct {
	once sync.Once
	loc  *time.Location
}{
	loc: time.Local,
}

// @brief Returns the current schedule time in the configured timezone.
// @return Current time in SCHEDULE_TIMEZONE, TZ, Europe/Berlin, or local time.
func scheduleNow() time.Time {
	scheduleLocation.once.Do(func() {
		tzName := strings.TrimSpace(os.Getenv("SCHEDULE_TIMEZONE"))
		if tzName == "" {
			tzName = strings.TrimSpace(os.Getenv("TZ"))
		}
		if tzName == "" {
			tzName = defaultScheduleTimezone
		}

		loc, err := time.LoadLocation(tzName)
		if err != nil {
			log.Printf("[chickendoor-schedule] timezone load failed (%s): %v; fallback to local (%s)", tzName, err, time.Local.String())
			return
		}

		scheduleLocation.loc = loc
		log.Printf("[chickendoor-schedule] timezone set to %s", scheduleLocation.loc.String())
	})

	return time.Now().In(scheduleLocation.loc)
}

// @brief Publishes the schedule activation state to the controller.
// @param active Whether scheduled operation is enabled.
// @param reason Context used to identify the publication in logs.
func (h *ChickenDoor) publishScheduleActive(active bool, reason string) {
	if err := h.mqttManager.Publish(scheduleActiveTopic, active); err != nil {
		log.Printf("[chickendoor-schedule] publish schedule_active failed (%s): %v", reason, err)
		return
	}

	log.Printf("[chickendoor-schedule] published %s=%t (%s)", scheduleActiveTopic, active, reason)
}

// @brief Describes the HTTP payload used to configure the sleep schedule.
type sleepScheduleRequest struct {
	Count        int      `json:"count"`
	Timestamps   []string `json:"timestamps"`
	Actions      []string `json:"actions"`
	AwakeSeconds int      `json:"awakeSeconds"`
	Active       bool     `json:"active"`
}

// @brief Represents one configured timestamp and its requested motor action.
type ScheduleEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
}

// @brief Parses a schedule time for the calendar day of a reference timestamp.
// @param base Reference time providing the date and location.
// @param value Schedule time in HH:MM or HH:MM:SS format.
// @return Parsed timestamp on the reference day, or a validation error.
func parseScheduleTimestampForDay(base time.Time, value string) (time.Time, error) {
	layout := "15:04"
	if strings.Count(value, ":") == 2 {
		layout = "15:04:05"
	}

	parsed, err := time.ParseInLocation(layout, value, base.Location())
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(base.Year(), base.Month(), base.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, base.Location()), nil
}

// @brief Computes the next timestamp from a list of legacy schedule times.
// @param now Reference timestamp.
// @param timestamps Schedule times in HH:MM or HH:MM:SS format.
// @return Next occurrence, or an error when no valid entry exists.
func nextScheduleTimestamp(now time.Time, timestamps []string) (time.Time, error) {
	entries := make([]ScheduleEntry, 0, len(timestamps))
	for _, timestamp := range timestamps {
		entries = append(entries, ScheduleEntry{Timestamp: timestamp, Action: "none"})
	}
	entry, err := nextScheduleEntry(now, entries)
	if err != nil {
		return time.Time{}, err
	}
	result, err := parseScheduleTimestampForDay(now, entry.Timestamp)
	if err != nil {
		return time.Time{}, err
	}
	if !result.After(now) {
		result = result.Add(24 * time.Hour)
	}
	return result, nil
}

// @brief Selects the earliest upcoming configured schedule entry.
// @param now Reference timestamp.
// @param entries Schedule entries including their actions.
// @return Earliest upcoming entry, or an error for empty/invalid input.
func nextScheduleEntry(now time.Time, entries []ScheduleEntry) (ScheduleEntry, error) {
	if len(entries) == 0 {
		return ScheduleEntry{}, fmt.Errorf("keine timestamps konfiguriert")
	}

	type candidateEntry struct {
		entry ScheduleEntry
		time  time.Time
	}
	candidates := make([]candidateEntry, 0, len(entries))
	for _, entry := range entries {
		candidate, err := parseScheduleTimestampForDay(now, entry.Timestamp)
		if err != nil {
			return ScheduleEntry{}, fmt.Errorf("ungültiger timestamp '%s': %w", entry.Timestamp, err)
		}
		if !candidate.After(now) {
			candidate = candidate.Add(24 * time.Hour)
		}
		candidates = append(candidates, candidateEntry{entry: entry, time: candidate})
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].time.Before(candidates[j].time) })
	return candidates[0].entry, nil
}

// @brief Publishes sleep until the next schedule and creates its history row.
// @param reason Context used to identify the scheduling attempt in logs.
// @return true when the sleep command was published successfully.
func (h *ChickenDoor) scheduleSleepUntilNext(reason string) bool {
	h.mu.Lock()
	active := h.scheduleActive
	entries := append([]ScheduleEntry(nil), h.scheduleEntries...)
	if len(entries) == 0 {
		for _, timestamp := range h.scheduleTimestamps {
			entries = append(entries, ScheduleEntry{Timestamp: timestamp, Action: "none"})
		}
	}
	lastKnownBattery := h.lastStatusBattery
	h.mu.Unlock()

	if !active || len(entries) == 0 {
		return false
	}

	now := scheduleNow()
	nextEntry, err := nextScheduleEntry(now, entries)
	if err != nil {
		log.Printf("[chickendoor-schedule] next timestamp error (%s): %v", reason, err)
		return false
	}
	nextTs, err := parseScheduleTimestampForDay(now, nextEntry.Timestamp)
	if err != nil {
		log.Printf("[chickendoor-schedule] next timestamp error (%s): %v", reason, err)
		return false
	}
	if !nextTs.After(now) {
		nextTs = nextTs.Add(24 * time.Hour)
	}

	duration := nextTs.Sub(now)
	if duration <= 0 {
		duration = time.Second
	}
	sleepSeconds := int(math.Ceil(duration.Seconds()))
	if sleepSeconds < 1 {
		sleepSeconds = 1
	}

	if err := h.mqttManager.Publish(fmt.Sprintf("%s/sleepms", nanoSetPrefix), sleepSeconds*1000); err != nil {
		log.Printf("[chickendoor-schedule] publish sleep failed (%s): %v", reason, err)
		return false
	}

	batteryPercent := ""
	if msgs := h.mqttManager.Messages(); msgs != nil {
		if value, ok := msgs["battery_percent"]; ok && value != nil {
			batteryPercent = fmt.Sprintf("%v", value)
		}
	}
	if batteryPercent == "" {
		batteryPercent = lastKnownBattery
	}

	h.mu.Lock()
	currentEndPosition := h.completedMotorPosition
	currentMotorDuration := h.completedMotorDuration
	if !h.motorRunningSince.IsZero() {
		var elapsedSec float64
		if !h.motorLimitReachedAt.IsZero() {
			elapsedSec = math.Round(h.motorLimitReachedAt.Sub(h.motorRunningSince).Seconds()*10) / 10
		} else {
			elapsedSec = math.Round(time.Since(h.motorRunningSince).Seconds()*10) / 10
		}
		if elapsedSec < 0.1 {
			elapsedSec = 0.1
		}
		currentMotorDuration = elapsedSec
		doorPos := resolveDoorPosition(h.lastStatusLimitClose, h.lastStatusLimitOpen, h.lastStatusPosition, h.motorRunningAction)
		if doorPos != "" && doorPos != "unbekannt" {
			currentEndPosition = doorPos
		}
	}
	h.motorRunningSince = time.Time{}
	h.motorRunningAction = ""
	h.motorLimitReachedAt = time.Time{}
	h.pendingScheduleAction = normalizeScheduleAction(nextEntry.Action)
	h.scheduleSleepPending = false
	nowTs := time.Now()
	h.lastSleepCommandAt = nowTs
	h.scheduleHistory = append(h.scheduleHistory, ScheduleHistoryEntry{SleepSeconds: sleepSeconds, BatteryPercent: batteryPercent, SleepCommandAtMs: nowTs.UnixMilli(), EndPosition: currentEndPosition, MotorDurationSec: currentMotorDuration})
	h.completedMotorDuration = 0
	h.completedMotorPosition = ""
	if len(h.scheduleHistory) > scheduleHistorySize {
		h.scheduleHistory = h.scheduleHistory[len(h.scheduleHistory)-scheduleHistorySize:]
	}
	h.mu.Unlock()
	h.persistState()

	log.Printf("[chickendoor-schedule] sleep sent (%s): %ds until %s", reason, sleepSeconds, nextTs.Format("15:04:05"))
	return true
}

// @brief Normalizes user-facing schedule action names to MQTT commands.
// @param action Raw configured action.
// @return "open", "close", "stop", or "none".
func normalizeScheduleAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open", "öffnen", "oeffnen":
		return "open"
	case "close", "schließen", "schliessen":
		return "close"
	case "stop":
		return "stop"
	default:
		return "none"
	}
}

// @brief Checks whether a schedule target is already reached by an end switch.
// @param action Requested action.
// @param limitClose Raw close limit state.
// @param limitOpen Raw open limit state.
// @return true when the target position is already active.
func scheduleActionAlreadyAtTarget(action, limitClose, limitOpen string) bool {
	switch normalizeScheduleAction(action) {
	case "open":
		return isLimitActive(limitOpen)
	case "close":
		return isLimitActive(limitClose)
	default:
		return false
	}
}

// @brief Finds the action nearest to a reference time within a tolerance window.
// @param now Reference timestamp.
// @param entries Configured schedule entries.
// @param maxDiff Maximum accepted absolute difference.
// @return Matching normalized action, or "none".
func findMatchingScheduleAction(now time.Time, entries []ScheduleEntry, maxDiff time.Duration) string {
	if len(entries) == 0 {
		return "none"
	}
	bestDiff := time.Duration(1<<63 - 1)
	bestAction := "none"
	for _, entry := range entries {
		cand, err := parseScheduleTimestampForDay(now, entry.Timestamp)
		if err != nil {
			continue
		}
		for _, candidate := range []time.Time{cand, cand.Add(-24 * time.Hour), cand.Add(24 * time.Hour)} {
			diff := now.Sub(candidate)
			if diff < 0 {
				diff = -diff
			}
			if diff < bestDiff {
				bestDiff = diff
				bestAction = normalizeScheduleAction(entry.Action)
			}
		}
	}
	if bestDiff <= maxDiff {
		return bestAction
	}
	return "none"
}

// @brief Executes a normalized schedule motor action and records its start.
// @param action Configured or normalized motor action.
func (h *ChickenDoor) executeScheduleAction(action string) {
	action = normalizeScheduleAction(action)
	if action == "none" {
		return
	}
	if action == "open" || action == "close" {
		h.mu.Lock()
		runtimeSeconds := h.motorAutoStopSeconds
		h.mu.Unlock()
		if err := h.publishMotorRuntime(runtimeSeconds); err != nil {
			log.Printf("[chickendoor-schedule] runtime publish failed before action %s: %v", action, err)
			return
		}
	}
	if err := h.mqttManager.Publish(fmt.Sprintf("%s/engine", nanoSetPrefix), action); err != nil {
		log.Printf("[chickendoor-schedule] action %s failed: %v", action, err)
		return
	}
	h.mu.Lock()
	if action == "open" || action == "close" {
		h.motorRunningSince = time.Now()
		h.motorRunningAction = action
		h.motorLimitReachedAt = time.Time{}
	} else {
		h.motorRunningSince = time.Time{}
		h.motorRunningAction = ""
		h.motorLimitReachedAt = time.Time{}
	}
	h.lastStatusAction = action
	h.mu.Unlock()
	h.persistState()
	log.Printf("[chickendoor-schedule] action sent: %s", action)
}

// @brief Classifies whether a controller wake occurred before its planned time.
// @param onlineTs Timestamp at which the controller became online.
// @return before-planned flag, tolerance flag, and lead duration.
func (h *ChickenDoor) classifyWakeTiming(onlineTs time.Time) (bool, bool, time.Duration) {
	if onlineTs.IsZero() || len(h.scheduleHistory) == 0 {
		return false, false, 0
	}
	last := h.scheduleHistory[len(h.scheduleHistory)-1]
	if last.SleepCommandAtMs <= 0 || last.SleepSeconds <= 0 {
		return false, false, 0
	}
	plannedWake := time.UnixMilli(last.SleepCommandAtMs).Add(time.Duration(last.SleepSeconds) * time.Second)
	if !onlineTs.Before(plannedWake) {
		return false, false, 0
	}
	lead := plannedWake.Sub(onlineTs)
	return true, lead <= scheduleEarlyWakeTolerance, lead
}

// @brief Updates sleep/online transition timestamps and arms schedule wake handling.
// @param controllerState Raw controller status.
// @param sleepState Raw sleep status.
// @param stateTs Receive timestamp of the state message.
// @param hasStateTs Whether stateTs is valid.
func (h *ChickenDoor) updateStateTracking(controllerState, sleepState string, stateTs time.Time, hasStateTs bool) {
	stateForTransition := normalizeControllerState(controllerState)
	if stateForTransition == "" {
		stateForTransition = normalizeControllerState(sleepState)
	}
	if stateForTransition == "" || !hasStateTs {
		return
	}
	changed := false
	h.mu.Lock()
	if !h.lastStateMessageAt.IsZero() && !stateTs.After(h.lastStateMessageAt) {
		h.mu.Unlock()
		return
	}
	h.lastStateMessageAt = stateTs
	if (stateForTransition == "sleeping" || stateForTransition == "offline") && h.lastControllerState != stateForTransition {
		h.sleepingAt = stateTs
		changed = true
		if n := len(h.scheduleHistory); n > 0 && h.scheduleHistory[n-1].SleepingAtMs == 0 {
			h.scheduleHistory[n-1].SleepingAtMs = unixMillisOrZero(stateTs)
		}
	}
	if stateForTransition == "online" && (h.lastControllerState == "sleeping" || h.lastControllerState == "offline") {
		h.onlineAt = stateTs
		changed = true
		if !h.sleepingAt.IsZero() && !h.onlineAt.Before(h.sleepingAt) {
			h.wakeDeltaMs = h.onlineAt.Sub(h.sleepingAt).Milliseconds()
		}
		if n := len(h.scheduleHistory); n > 0 {
			if h.scheduleHistory[n-1].SleepingAtMs == 0 {
				h.scheduleHistory[n-1].SleepingAtMs = unixMillisOrZero(h.sleepingAt)
			}
			if h.scheduleHistory[n-1].WokeUpAtMs == 0 {
				h.scheduleHistory[n-1].WokeUpAtMs = unixMillisOrZero(stateTs)
			}
		}
		if h.scheduleActive {
			beforePlanned, withinTolerance, lead := h.classifyWakeTiming(stateTs)
			if beforePlanned && !withinTolerance {
				h.scheduleSleepPending = true
				h.scheduleWakeAt = time.Now()
				log.Printf("[chickendoor-schedule] early wake outside tolerance (%s) -> skip action, schedule sleep until planned wake", lead.Round(time.Second))
			} else {
				h.scheduleWakeAt = time.Now()
				if beforePlanned {
					log.Printf("[chickendoor-schedule] early wake within tolerance (%s) -> execute normal action", lead.Round(time.Second))
				} else {
					log.Printf("[chickendoor-schedule] wake on/after planned time -> execute normal action")
				}
			}
		}
	}
	h.lastControllerState = stateForTransition
	h.mu.Unlock()
	if changed {
		h.persistState()
	}
}

// @brief Performs one schedule transition tick and arms the next sleep cycle.
func (h *ChickenDoor) scheduleTick() {
	msgs := h.mqttManager.Messages()
	controllerState := toString(msgs["status"])
	sleepState := toString(firstValue(msgs, "sleepms/status", "sleepms_status"))
	stateTs, hasStateTs := h.firstMessageTimestamp("status", "sleepms/status", "sleepms_status")
	h.updateStateTracking(controllerState, sleepState, stateTs, hasStateTs)

	h.mu.Lock()
	active := h.scheduleActive
	wakeAt := h.scheduleWakeAt
	h.mu.Unlock()
	if !active || wakeAt.IsZero() {
		return
	}
	currentState := normalizeControllerState(controllerState)
	if currentState == "" {
		currentState = normalizeControllerState(sleepState)
	}
	if time.Now().Before(wakeAt) || currentState == "sleeping" || currentState == "offline" {
		return
	}

	h.mu.Lock()
	shouldSchedule := h.scheduleActive && !h.scheduleWakeAt.IsZero() && !time.Now().Before(h.scheduleWakeAt)
	if shouldSchedule {
		h.scheduleWakeAt = time.Time{}
	}
	h.mu.Unlock()
	if !shouldSchedule {
		return
	}

	h.mu.Lock()
	action := h.pendingScheduleAction
	waitingToSleep := h.scheduleSleepPending
	actionAlreadyAtTarget := false
	if !waitingToSleep {
		if action == "" || action == "none" {
			action = findMatchingScheduleAction(time.Now(), h.scheduleEntries, 15*time.Minute)
			log.Printf("[chickendoor-schedule] fallback matching action resolved: '%s'", action)
		}
		limitClose := h.lastStatusLimitClose
		limitOpen := h.lastStatusLimitOpen
		if messages := h.mqttManager.Messages(); messages != nil {
			if value := h.latestMessageValue(messages, "limit_close", "limit/close"); value != nil {
				limitClose = toString(value)
			}
			if value := h.latestMessageValue(messages, "limit_open", "limit/open"); value != nil {
				limitOpen = toString(value)
			}
		}
		actionAlreadyAtTarget = scheduleActionAlreadyAtTarget(action, limitClose, limitOpen)
		h.pendingScheduleAction = ""
		h.scheduleSleepPending = true
		h.scheduleWakeAt = time.Now().Add(time.Duration(h.scheduleAwakeSeconds) * time.Second)
	} else {
		h.scheduleSleepPending = false
	}
	h.mu.Unlock()
	h.persistState()

	if !waitingToSleep {
		if actionAlreadyAtTarget {
			log.Printf("[chickendoor-schedule] action %s already fulfilled by end switch; motor command skipped", action)
			return
		}
		h.executeScheduleAction(action)
		return
	}
	if ok := h.scheduleSleepUntilNext("post-online-delay"); !ok {
		h.mu.Lock()
		if h.scheduleActive {
			h.scheduleWakeAt = time.Now().Add(scheduleRetryDelay)
		}
		h.mu.Unlock()
		log.Printf("[chickendoor-schedule] retry armed in %s", scheduleRetryDelay)
	}
}

// @brief Validates a schedule time in HH:MM or HH:MM:SS format.
// @param value Candidate time string.
// @return true when the value is a supported schedule timestamp.
func isValidScheduleTimestamp(value string) bool {
	if _, err := time.Parse("15:04", value); err == nil {
		return true
	}
	if _, err := time.Parse("15:04:05", value); err == nil {
		return true
	}
	return false
}

// @brief Stores or updates the shared sleep schedule configuration.
// @param w HTTP response writer.
// @param r HTTP request with schedule timestamps, actions, and activation state.
func (h *ChickenDoor) SleepScheduleHandler(w http.ResponseWriter, r *http.Request) {
	var req sleepScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Ungültiger JSON body")
		return
	}

	if req.Count <= 0 {
		jsonError(w, http.StatusBadRequest, "count muss > 0 sein")
		return
	}
	if len(req.Timestamps) != req.Count {
		jsonError(w, http.StatusBadRequest, "count stimmt nicht mit timestamps-Länge überein")
		return
	}
	if req.AwakeSeconds < 0 {
		jsonError(w, http.StatusBadRequest, "awakeSeconds darf nicht negativ sein")
		return
	}

	cleanedTimestamps := make([]string, 0, len(req.Timestamps))
	cleanedEntries := make([]ScheduleEntry, 0, len(req.Timestamps))
	for index, timestamp := range req.Timestamps {
		trimmed := strings.TrimSpace(timestamp)
		if trimmed == "" {
			jsonError(w, http.StatusBadRequest, "Leere Timestamps sind nicht erlaubt")
			return
		}
		if !isValidScheduleTimestamp(trimmed) {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("Ungültiger Timestamp '%s'. Erlaubt: HH:MM oder HH:MM:SS", trimmed))
			return
		}
		cleanedTimestamps = append(cleanedTimestamps, trimmed)
		action := "none"
		if index < len(req.Actions) {
			action = normalizeScheduleAction(req.Actions[index])
		}
		cleanedEntries = append(cleanedEntries, ScheduleEntry{Timestamp: trimmed, Action: action})
	}

	h.mu.Lock()
	h.scheduleAwakeSeconds = req.AwakeSeconds
	h.scheduleTimestamps = append([]string(nil), cleanedTimestamps...)
	h.scheduleEntries = append([]ScheduleEntry(nil), cleanedEntries...)
	h.scheduleActive = req.Active
	if req.Active {
		h.controlMode = "schedule"
	} else {
		h.controlMode = "manual"
	}
	h.scheduleWakeAt = time.Time{}
	h.mu.Unlock()

	h.publishScheduleActive(req.Active, "sleep-schedule-handler")
	h.persistState()
	if req.Active {
		h.scheduleSleepUntilNext("schedule-activation")
	}

	jsonResponse(w, map[string]any{
		"ok":           true,
		"stored":       true,
		"count":        req.Count,
		"timestamps":   cleanedTimestamps,
		"entries":      cleanedEntries,
		"awakeSeconds": req.AwakeSeconds,
		"active":       req.Active,
	})
}
