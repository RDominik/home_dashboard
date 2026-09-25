package chickendoor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"

	"webgui-api/mqtt"
)

const nanoSetPrefix = "nano/esp32"
const motorRuntimeOpenTopic = nanoSetPrefix + "/runtime/open"
const motorRuntimeCloseTopic = nanoSetPrefix + "/runtime/close"
const stateDBPathDefault = "data/chickendoor.db"
const stateBucketName = "chickendoor"
const stateKey = "state"

const defaultMotorAutoStopSeconds = 15

type StatusResponse struct {
	Position         string                 `json:"position"`
	LastAction       string                 `json:"lastAction"`
	Error            string                 `json:"error,omitempty"`
	Battery          string                 `json:"battery"`
	WakeReason       string                 `json:"wakeReason"`
	ControllerState  string                 `json:"controllerState"`
	SleepState       string                 `json:"sleepState"`
	IP               string                 `json:"ip"`
	Charging         string                 `json:"charging"`
	LimitClose       string                 `json:"limitClose,omitempty"`
	LimitOpen        string                 `json:"limitOpen,omitempty"`
	ScheduleActive   bool                   `json:"scheduleActive"`
	ScheduleTimezone string                 `json:"scheduleTimezone"`
	ServerNowMs      int64                  `json:"serverNowMs"`
	SleepCommandAtMs int64                  `json:"sleepCommandAtMs,omitempty"`
	SleepingAtMs     int64                  `json:"sleepingAtMs,omitempty"`
	OnlineAtMs       int64                  `json:"onlineAtMs,omitempty"`
	WakeDeltaMs      int64                  `json:"wakeDeltaMs,omitempty"`
	ScheduleHistory  []ScheduleHistoryEntry `json:"scheduleHistory,omitempty"`
}

// @brief Represents a single execution cycle in the schedule history.
//
// Tracks the planned sleep duration, actual sleep/wake timestamps, battery state,
// final motor end position, and the duration the motor ran in seconds.
type ScheduleHistoryEntry struct {
	// @brief Planned sleep duration in seconds for this cycle.
	SleepSeconds int `json:"sleepSeconds"`

	// @brief Battery percentage string at time of sleep command.
	BatteryPercent string `json:"batteryPercent,omitempty"`

	// @brief Epoch timestamp in milliseconds when the sleep command was published.
	SleepCommandAtMs int64 `json:"sleepCommandAtMs,omitempty"`

	// @brief Epoch timestamp in milliseconds when the controller confirmed entering sleep state.
	SleepingAtMs int64 `json:"sleepingAtMs,omitempty"`

	// @brief Epoch timestamp in milliseconds when the controller woke up and became online.
	WokeUpAtMs int64 `json:"wokeUpAtMs,omitempty"`

	// @brief Final end position reached by the door after motor movement (e.g. "open", "closed", "offen", "geschlossen", "stop").
	EndPosition string `json:"endPosition,omitempty"`

	// @brief Duration in seconds that the motor ran during this wake-up cycle.
	MotorDurationSec float64 `json:"motorDurationSec,omitempty"`
}

// @brief Converts a timestamp to Unix milliseconds.
// @param ts Input timestamp.
// @return Unix milliseconds, or 0 if ts is the zero value.
func unixMillisOrZero(ts time.Time) int64 {
	if ts.IsZero() {
		return 0
	}
	return ts.UnixMilli()
}

type ChickenDoor struct {
	mqttManager *mqtt.Manager
	db          *bbolt.DB
	mu          sync.Mutex

	lastControllerState string
	// lastStateMessageAt stores the latest accepted MQTT state timestamp used for
	// transition detection. This suppresses duplicate schedule transitions caused
	// by out-of-order or repeated state payloads that would otherwise retrigger
	// the same sleeping->online cycle.
	lastStateMessageAt time.Time
	lastSleepCommandAt time.Time
	sleepingAt         time.Time
	onlineAt           time.Time
	wakeDeltaMs        int64

	scheduleAwakeSeconds  int
	scheduleTimestamps    []string
	scheduleEntries       []ScheduleEntry
	scheduleActive        bool
	scheduleWakeAt        time.Time
	pendingScheduleAction string
	scheduleSleepPending  bool
	// scheduleEarlyWakePending distinguishes a wake-too-early sleep retry from
	// the normal post-motor sleep phase, so only the former is recorded as wait.
	scheduleEarlyWakePending  bool
	scheduleHistory           []ScheduleHistoryEntry
	sleepTime                 int
	motorAutoStopOpenSeconds  int
	motorAutoStopCloseSeconds int
	sleepUntil                string
	controlMode               string
	historyExpanded           bool
	// motorRunningSince stores the timestamp when the motor was commanded or detected to start moving.
	motorRunningSince time.Time
	// motorRunningAction stores the active motor command direction ("open", "close", "stop").
	motorRunningAction string
	// motorLimitReachedAt stores the timestamp when a limit switch became active during movement.
	motorLimitReachedAt time.Time
	// completedMotorDuration stores the last completed motor run until the next
	// schedule history row is created. This prevents a finished run from being
	// written into the previous sleep row while the new row does not exist yet.
	completedMotorDuration float64
	// completedMotorPosition stores the end position paired with the completed run.
	completedMotorPosition string

	lastStatusPosition   string
	lastStatusAction     string
	lastStatusBattery    string
	lastStatusWakeReason string
	lastStatusController string
	lastStatusSleep      string
	lastStatusIP         string
	lastStatusCharging   string
	lastStatusLimitClose string
	lastStatusLimitOpen  string

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

type persistedState struct {
	ScheduleAwakeSeconds  int             `json:"scheduleAwakeSeconds"`
	ScheduleTimestamps    []string        `json:"scheduleTimestamps"`
	ScheduleEntries       []ScheduleEntry `json:"scheduleEntries,omitempty"`
	ScheduleActive        bool            `json:"scheduleActive"`
	PendingScheduleAction string          `json:"pendingScheduleAction,omitempty"`
	ScheduleSleepPending  bool            `json:"scheduleSleepPending,omitempty"`
	// ScheduleEarlyWakePending persists whether the pending sleep exists solely
	// because the controller woke before its planned schedule timestamp.
	ScheduleEarlyWakePending  bool                   `json:"scheduleEarlyWakePending,omitempty"`
	ScheduleHistory           []ScheduleHistoryEntry `json:"scheduleHistory"`
	SleepTime                 int                    `json:"sleepTime"`
	MotorAutoStopSeconds      int                    `json:"motorAutoStopSeconds,omitempty"`
	MotorAutoStopOpenSeconds  int                    `json:"motorAutoStopOpenSeconds"`
	MotorAutoStopCloseSeconds int                    `json:"motorAutoStopCloseSeconds"`
	SleepUntil                string                 `json:"sleepUntil"`
	ControlMode               string                 `json:"controlMode"`
	HistoryExpanded           bool                   `json:"historyExpanded"`
	LastStatusPosition        string                 `json:"lastStatusPosition"`
	LastStatusAction          string                 `json:"lastStatusAction"`
	LastStatusBattery         string                 `json:"lastStatusBattery"`
	LastStatusWakeReason      string                 `json:"lastStatusWakeReason"`
	LastStatusController      string                 `json:"lastStatusController"`
	LastStatusSleep           string                 `json:"lastStatusSleep"`
	LastStatusIP              string                 `json:"lastStatusIP"`
	LastStatusCharging        string                 `json:"lastStatusCharging"`
	LastStatusLimitClose      string                 `json:"lastStatusLimitClose,omitempty"`
	LastStatusLimitOpen       string                 `json:"lastStatusLimitOpen,omitempty"`
}

type uiStateRequest struct {
	SleepTime                 int             `json:"sleepTime"`
	MotorAutoStopSeconds      int             `json:"motorAutoStopSeconds,omitempty"`
	MotorAutoStopOpenSeconds  int             `json:"motorAutoStopOpenSeconds"`
	MotorAutoStopCloseSeconds int             `json:"motorAutoStopCloseSeconds"`
	SleepUntil                string          `json:"sleepUntil"`
	ControlMode               string          `json:"controlMode"`
	HistoryExpanded           bool            `json:"historyExpanded"`
	ScheduleTimestamps        []string        `json:"scheduleTimestamps"`
	ScheduleEntries           []ScheduleEntry `json:"scheduleEntries"`
	AwakeSeconds              int             `json:"awakeSeconds"`
}

// @brief Clamps motor auto-stop setting to the allowed range.
// @param seconds Desired timeout in seconds.
// @return Value constrained to 1..60 seconds.
func clampMotorAutoStopSeconds(seconds int) int {
	if seconds < 1 {
		return 1
	}
	if seconds > 60 {
		return 60
	}
	return seconds
}

// @brief Publishes the configured directional motor runtime to the controller.
// @details
// The controller receives separate runtimes because opening and closing can
// require different movement durations. The backend continues enforcing the
// same directional timeout locally, while this MQTT value lets the controller
// apply the matching stop limit for the next movement command.
// @param action Motor direction, expected to be "open" or "close".
// @param seconds Requested directional motor runtime in seconds.
// @return An error when the MQTT publish cannot be completed.
func (h *ChickenDoor) publishMotorRuntime(action string, seconds int) error {
	seconds = clampMotorAutoStopSeconds(seconds)
	topic := motorRuntimeOpenTopic
	if action == "close" {
		topic = motorRuntimeCloseTopic
	}
	return h.mqttManager.Publish(topic, seconds)
}

// @brief Opens/creates the local bbolt state database.
// @return Open DB handle or nil when opening fails.
func openStateDB() *bbolt.DB {
	path := strings.TrimSpace(os.Getenv("CHICKENDOOR_DB_PATH"))
	if path == "" {
		path = stateDBPathDefault
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("[chickendoor-state] mkdir failed for %s: %v", path, err)
		return nil
	}

	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Printf("[chickendoor-state] open failed for %s: %v", path, err)
		return nil
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		_, createErr := tx.CreateBucketIfNotExists([]byte(stateBucketName))
		return createErr
	}); err != nil {
		log.Printf("[chickendoor-state] bucket init failed: %v", err)
		_ = db.Close()
		return nil
	}

	log.Printf("[chickendoor-state] using %s", path)
	return db
}

// @brief Loads persisted schedule state from local DB.
func (h *ChickenDoor) loadPersistedState() {
	if h.db == nil {
		return
	}

	var raw []byte
	if err := h.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(stateBucketName))
		if bucket == nil {
			return nil
		}
		stored := bucket.Get([]byte(stateKey))
		if len(stored) == 0 {
			return nil
		}
		raw = append([]byte(nil), stored...)
		return nil
	}); err != nil {
		log.Printf("[chickendoor-state] load failed: %v", err)
		return
	}

	if len(raw) == 0 {
		return
	}

	var state persistedState
	if err := json.Unmarshal(raw, &state); err != nil {
		log.Printf("[chickendoor-state] decode failed: %v", err)
		return
	}

	h.mu.Lock()
	h.scheduleAwakeSeconds = state.ScheduleAwakeSeconds
	h.scheduleTimestamps = append([]string(nil), state.ScheduleTimestamps...)
	h.scheduleEntries = append([]ScheduleEntry(nil), state.ScheduleEntries...)
	if len(h.scheduleEntries) == 0 {
		for _, timestamp := range h.scheduleTimestamps {
			h.scheduleEntries = append(h.scheduleEntries, ScheduleEntry{Timestamp: timestamp, Action: "none"})
		}
	}
	h.scheduleActive = state.ScheduleActive
	h.pendingScheduleAction = state.PendingScheduleAction
	h.scheduleSleepPending = state.ScheduleSleepPending
	h.scheduleEarlyWakePending = state.ScheduleEarlyWakePending
	h.scheduleHistory = append([]ScheduleHistoryEntry(nil), state.ScheduleHistory...)
	h.sleepTime = state.SleepTime
	// Migrate the former single timeout into both directional settings when a
	// database created by an older backend is opened for the first time.
	legacyRuntime := clampMotorAutoStopSeconds(state.MotorAutoStopSeconds)
	if state.MotorAutoStopOpenSeconds > 0 {
		h.motorAutoStopOpenSeconds = clampMotorAutoStopSeconds(state.MotorAutoStopOpenSeconds)
	} else if state.MotorAutoStopSeconds > 0 {
		h.motorAutoStopOpenSeconds = legacyRuntime
	} else {
		h.motorAutoStopOpenSeconds = defaultMotorAutoStopSeconds
	}
	if state.MotorAutoStopCloseSeconds > 0 {
		h.motorAutoStopCloseSeconds = clampMotorAutoStopSeconds(state.MotorAutoStopCloseSeconds)
	} else if state.MotorAutoStopSeconds > 0 {
		h.motorAutoStopCloseSeconds = legacyRuntime
	} else {
		h.motorAutoStopCloseSeconds = defaultMotorAutoStopSeconds
	}
	h.sleepUntil = state.SleepUntil
	h.controlMode = state.ControlMode
	h.historyExpanded = state.HistoryExpanded
	h.lastStatusPosition = state.LastStatusPosition
	h.lastStatusAction = state.LastStatusAction
	h.lastStatusBattery = state.LastStatusBattery
	h.lastStatusWakeReason = state.LastStatusWakeReason
	h.lastStatusController = state.LastStatusController
	h.lastStatusSleep = state.LastStatusSleep
	h.lastStatusIP = state.LastStatusIP
	h.lastStatusCharging = state.LastStatusCharging
	h.lastStatusLimitClose = state.LastStatusLimitClose
	h.lastStatusLimitOpen = state.LastStatusLimitOpen
	historyRepaired := repairLegacyWaitHistory(h.scheduleHistory, h.scheduleEntries)
	if h.controlMode == "" {
		if h.scheduleActive {
			h.controlMode = "schedule"
		} else {
			h.controlMode = "manual"
		}
	}
	h.mu.Unlock()
	if historyRepaired {
		// Persist the one-time correction so the history stays repaired after
		// subsequent restarts instead of only being fixed in the in-memory view.
		h.persistState()
	}

	log.Printf("[chickendoor-state] restored schedule: active=%t timestamps=%d history=%d", h.scheduleActive, len(h.scheduleTimestamps), len(h.scheduleHistory))
}

// @brief Persists current schedule state/history to local DB.
func (h *ChickenDoor) persistState() {
	if h.db == nil {
		return
	}

	h.mu.Lock()
	state := persistedState{
		ScheduleAwakeSeconds:      h.scheduleAwakeSeconds,
		ScheduleTimestamps:        append([]string(nil), h.scheduleTimestamps...),
		ScheduleEntries:           append([]ScheduleEntry(nil), h.scheduleEntries...),
		ScheduleActive:            h.scheduleActive,
		PendingScheduleAction:     h.pendingScheduleAction,
		ScheduleSleepPending:      h.scheduleSleepPending,
		ScheduleEarlyWakePending:  h.scheduleEarlyWakePending,
		ScheduleHistory:           append([]ScheduleHistoryEntry(nil), h.scheduleHistory...),
		SleepTime:                 h.sleepTime,
		MotorAutoStopOpenSeconds:  h.motorAutoStopOpenSeconds,
		MotorAutoStopCloseSeconds: h.motorAutoStopCloseSeconds,
		SleepUntil:                h.sleepUntil,
		ControlMode:               h.controlMode,
		HistoryExpanded:           h.historyExpanded,
		LastStatusPosition:        h.lastStatusPosition,
		LastStatusAction:          h.lastStatusAction,
		LastStatusBattery:         h.lastStatusBattery,
		LastStatusWakeReason:      h.lastStatusWakeReason,
		LastStatusController:      h.lastStatusController,
		LastStatusSleep:           h.lastStatusSleep,
		LastStatusIP:              h.lastStatusIP,
		LastStatusCharging:        h.lastStatusCharging,
		LastStatusLimitClose:      h.lastStatusLimitClose,
		LastStatusLimitOpen:       h.lastStatusLimitOpen,
	}
	h.mu.Unlock()

	payload, err := json.Marshal(state)
	if err != nil {
		log.Printf("[chickendoor-state] encode failed: %v", err)
		return
	}

	if err := h.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(stateBucketName))
		if bucket == nil {
			return fmt.Errorf("state bucket missing")
		}
		return bucket.Put([]byte(stateKey), payload)
	}); err != nil {
		log.Printf("[chickendoor-state] persist failed: %v", err)
	}
}

// @brief Creates a new ChickenDoor instance.
// @param mqttManager MQTT manager used for publishing and status access.
// @return Initialized ChickenDoor instance with a ready-to-use done channel.
func New(mqttManager *mqtt.Manager) *ChickenDoor {
	h := &ChickenDoor{
		mqttManager:               mqttManager,
		db:                        openStateDB(),
		done:                      make(chan struct{}),
		motorAutoStopOpenSeconds:  defaultMotorAutoStopSeconds,
		motorAutoStopCloseSeconds: defaultMotorAutoStopSeconds,
	}
	h.loadPersistedState()
	return h
}

// @brief Returns true when engine_status indicates movement is still ongoing.
// @param position Raw engine status text from MQTT.
// @return True if status looks like moving/opening/closing, otherwise false.
func isMotorRunningPosition(position string) bool {
	value := strings.ToLower(strings.TrimSpace(position))
	if value == "" {
		return false
	}

	return strings.Contains(value, "opening") ||
		strings.Contains(value, "closing") ||
		strings.Contains(value, "moving") ||
		strings.Contains(value, "run") ||
		strings.Contains(value, "fahrt") ||
		strings.Contains(value, "laeuft")
}

// @brief Returns true if limit switch payload indicates the switch is pressed/active.
//
// Checks for typical active state keywords such as "active", "on", "1", "true",
// "high", "pressed", "triggered", or "closed". The comparison is deliberately
// case-insensitive because MQTT payload casing depends on the controller firmware.
// @param val Raw limit switch payload string.
// @return true if the limit switch is active, otherwise false.
func isLimitActive(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "active" || v == "on" || v == "1" || v == "true" || v == "high" ||
		v == "pressed" || v == "triggered" || v == "closed"
}

// @brief Resolves semantic door position ("geschlossen", "offen", "in Bewegung", etc.)
//
// Evaluates only the open and close limit switches. If neither switch is
// active, the position is reported as "Zwischenposition".
//
// @param limitClose Current state of close limit switch (e.g. "ACTIVE" / "released").
// @param limitOpen Current state of open limit switch (e.g. "ACTIVE" / "released").
// @param engineStatus Raw engine status string from MQTT.
// @param lastAction Last commanded action ("open", "close", "stop").
// @return Resolved end position ("geschlossen", "offen", "in Bewegung", "Zwischenposition", or raw status).
func resolveDoorPosition(limitClose, limitOpen, engineStatus, lastAction string) string {
	closeActive := isLimitActive(limitClose)
	openActive := isLimitActive(limitOpen)

	if closeActive && !openActive {
		return "geschlossen"
	}
	if openActive && !closeActive {
		return "offen"
	}

	return "Zwischenposition"
}

// @brief Enforces automatic motor stop after the configured timeout and tracks run duration.
//
// When engine_status still indicates movement after motorAutoStopSeconds, this
// method publishes "stop" to the engine topic, calculates the total run duration,
// updates the latest schedule history entry with end position derived from limit switches
// and runtime, and clears the running timer.
func (h *ChickenDoor) autoStopTick() {
	msgs := h.mqttManager.Messages()
	position := toString(msgs["engine_status"])
	limitClose := toString(h.latestMessageValue(msgs, "limit_close", "limit/close"))
	limitOpen := toString(h.latestMessageValue(msgs, "limit_open", "limit/open"))
	runningFromMqtt := isMotorRunningPosition(position)

	h.mu.Lock()
	if limitClose == "" {
		limitClose = h.lastStatusLimitClose
	} else {
		h.lastStatusLimitClose = limitClose
	}
	if limitOpen == "" {
		limitOpen = h.lastStatusLimitOpen
	} else {
		h.lastStatusLimitOpen = limitOpen
	}

	timeoutSeconds := h.motorAutoStopOpenSeconds
	if h.motorRunningAction == "close" || h.motorRunningAction == "schließen" || h.motorRunningAction == "schliessen" {
		timeoutSeconds = h.motorAutoStopCloseSeconds
	}
	timeoutSeconds = clampMotorAutoStopSeconds(timeoutSeconds)
	if h.motorRunningAction == "close" || h.motorRunningAction == "schließen" || h.motorRunningAction == "schliessen" {
		h.motorAutoStopCloseSeconds = timeoutSeconds
	} else {
		h.motorAutoStopOpenSeconds = timeoutSeconds
	}

	now := time.Now()
	// Case 1: Motor is not recorded as running yet.
	if h.motorRunningSince.IsZero() {
		if !runningFromMqtt {
			h.mu.Unlock()
			return
		}
		// Motor was reported running via MQTT unsolicited.
		h.motorRunningSince = now
		h.motorRunningAction = position
		h.motorLimitReachedAt = time.Time{}
		h.mu.Unlock()
		return
	}

	// Case 2: Motor is actively running.
	runningSince := h.motorRunningSince
	runningAction := h.motorRunningAction
	elapsed := now.Sub(runningSince)
	elapsedSec := math.Round(elapsed.Seconds()*10) / 10
	shouldStopByTimeout := elapsed >= time.Duration(timeoutSeconds)*time.Second

	closeActive := isLimitActive(limitClose)
	openActive := isLimitActive(limitOpen)

	// Check if a target limit switch has been reached for the active movement.
	limitReached := false
	if (runningAction == "close" || runningAction == "schließen" || runningAction == "schliessen") && closeActive {
		limitReached = true
	} else if (runningAction == "open" || runningAction == "öffnen" || runningAction == "oeffnen") && openActive {
		limitReached = true
	}

	if limitReached {
		if h.motorLimitReachedAt.IsZero() {
			var hitTime time.Time
			if closeActive {
				if ts, ok := h.firstMessageTimestamp("limit_close", "limit/close"); ok && ts.After(runningSince) {
					hitTime = ts
				}
			}
			if openActive && hitTime.IsZero() {
				if ts, ok := h.firstMessageTimestamp("limit_open", "limit/open"); ok && ts.After(runningSince) {
					hitTime = ts
				}
			}
			if hitTime.IsZero() {
				hitTime = now
			}
			h.motorLimitReachedAt = hitTime
		}
	}

	// Movement concludes when:
	// 1) Target limit switch was hit, OR
	// 2) Auto-stop timeout expired.
	movementDone := false
	if !h.motorLimitReachedAt.IsZero() {
		if now.Sub(h.motorLimitReachedAt) >= 500*time.Millisecond || !runningFromMqtt {
			movementDone = true
		}
	} else if shouldStopByTimeout {
		movementDone = true
	}

	// Calculate current duration
	var currentDuration float64
	if !h.motorLimitReachedAt.IsZero() {
		currentDuration = math.Round(h.motorLimitReachedAt.Sub(runningSince).Seconds()*10) / 10
	} else {
		currentDuration = elapsedSec
	}
	if currentDuration < 0.1 {
		currentDuration = 0.1
	}

	doorPos := resolveDoorPosition(limitClose, limitOpen, position, runningAction)

	if movementDone {
		// Hold the completed result until scheduleSleepUntilNext creates the new
		// row. The previous row describes the wake-up that started this action and
		// must remain unchanged.
		h.completedMotorDuration = currentDuration
		if doorPos != "" && doorPos != "unbekannt" {
			h.completedMotorPosition = doorPos
		}
		h.motorRunningSince = time.Time{}
		h.motorLimitReachedAt = time.Time{}
		h.motorRunningAction = ""
		log.Printf("[chickendoor-autostop] movement finished (duration=%.1fs, endPosition=%s, timeout=%t)", currentDuration, doorPos, shouldStopByTimeout)
	}
	h.mu.Unlock()

	if shouldStopByTimeout {
		if err := h.mqttManager.Publish(fmt.Sprintf("%s/engine", nanoSetPrefix), "stop"); err != nil {
			log.Printf("[chickendoor-autostop] publish stop failed after %ds: %v", timeoutSeconds, err)
		} else {
			log.Printf("[chickendoor-autostop] auto-stop sent after %ds", timeoutSeconds)
		}
		h.mu.Lock()
		h.lastStatusAction = "stop"
		h.mu.Unlock()
		h.persistState()
	}
}

// @brief Returns the most recently received value for any of the given MQTT key aliases.
//
// Different MQTT configurations can expose the same end switch as either a
// flattened key such as "limit_close" or a slash-style key such as
// "limit/close". Selecting by receive timestamp prevents an older alias from
// hiding a newer ACTIVE or released state.
// @param msgs Snapshot of MQTT values.
// @param keys Equivalent MQTT key aliases.
// @return Value belonging to the newest received alias, or nil when none exists.
func (h *ChickenDoor) latestMessageValue(msgs map[string]any, keys ...string) any {
	var latestValue any
	var latestAt time.Time
	for _, key := range keys {
		value, receivedAt, ok := h.mqttManager.MessageWithTimestamp(key)
		if !ok {
			continue
		}
		if latestAt.IsZero() || receivedAt.After(latestAt) {
			latestValue = value
			latestAt = receivedAt
		}
	}
	if latestAt.IsZero() {
		return firstValue(msgs, keys...)
	}
	return latestValue
}

// @brief Robustly converts arbitrary values to string representation.
// @param value Input value from MQTT/JSON.
// @return String representation; booleans are normalized to "true"/"false".
func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// @brief Robustly converts an arbitrary value to int.
//
// Supports float64 (typical from JSON), int, and numeric strings.
// Non-parseable values return 0.
// @param value Input value.
// @return Integer value, or 0 if conversion fails.
func toInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		var parsed int
		if _, err := fmt.Sscanf(typed, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}

// @brief Normalizes state strings to core states sleeping/online.
// @param state Raw state value.
// @return "sleeping", "online", or the normalized original value.
func normalizeControllerState(state string) string {
	value := strings.ToLower(strings.TrimSpace(state))
	switch {
	case strings.Contains(value, "sleep"):
		return "sleeping"
	case strings.Contains(value, "offline"):
		return "offline"
	case strings.Contains(value, "online"):
		return "online"
	default:
		return value
	}
}

// @brief Returns the first existing value in msgs for a prioritized key list.
// @param msgs MQTT message map.
// @param keys Prioritized key order.
// @return Found value, or nil if none of the keys exist.
func firstValue(msgs map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := msgs[key]; ok {
			return value
		}
	}
	return nil
}

// @brief Returns the first available receive timestamp for the given keys.
// @param keys Prioritized key order.
// @return Timestamp and true on hit, otherwise zero time and false.
func (h *ChickenDoor) firstMessageTimestamp(keys ...string) (time.Time, bool) {
	for _, key := range keys {
		_, ts, ok := h.mqttManager.MessageWithTimestamp(key)
		if ok {
			return ts, true
		}
	}
	return time.Time{}, false
}

// @brief Writes a JSON response with Content-Type application/json.
// @param w HTTP response writer.
// @param data Any serializable response object.
func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// @brief Writes a JSON error response with an HTTP status code.
// @param w HTTP response writer.
// @param code HTTP status code.
// @param msg Error message text.
func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

// @brief Central API entry point for all /api/huehnerklappe/* routes.
//
// Dispatches requests to the specialized handlers based on URL path.
// Supported paths:
// - /api/huehnerklappe/status
// - /api/huehnerklappe/set
// - /api/huehnerklappe/sleep-schedule
// - /api/huehnerklappe/ (backward-compatible alias for sleep-schedule)
// @param w HTTP response writer.
// @param r HTTP request.
func (h *ChickenDoor) APIHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/huehnerklappe/status":
		h.StatusHandler(w, r)
	case "/api/huehnerklappe/ui-state":
		h.UIStateHandler(w, r)
	case "/api/huehnerklappe/set":
		h.SetHandler(w, r)
	case "/api/huehnerklappe/sleep-schedule", "/api/huehnerklappe/":
		h.SleepScheduleHandler(w, r)
	default:
		jsonError(w, http.StatusNotFound, "unknown huehnerklappe endpoint")
	}
}

// @brief Returns the current ChickenDoor status as JSON.
//
// Reads MQTT values including sleep/wakeup info, updates internal transition
// tracking, and returns consolidated status data for the UI. If live MQTT
// values are not yet available, the last persisted values are used as a
// fallback so the UI still shows meaningful data after startup.
// @param w HTTP response writer.
// @param r HTTP request (unused).
func (h *ChickenDoor) StatusHandler(w http.ResponseWriter, r *http.Request) {
	msgs := h.mqttManager.Messages()

	controllerState := toString(msgs["status"])
	sleepState := toString(firstValue(msgs, "sleepms/status", "sleepms_status"))
	wakeReason := toString(firstValue(msgs, "sleepms/wakeup_reason", "sleepms_wakeup_reason"))
	ip := toString(msgs["ip"])
	charging := toString(msgs["battery_charging"])
	limitClose := toString(h.latestMessageValue(msgs, "limit_close", "limit/close"))
	limitOpen := toString(h.latestMessageValue(msgs, "limit_open", "limit/open"))
	battery := ""
	if value, ok := msgs["battery_percent"]; ok && value != nil {
		battery = fmt.Sprintf("%v", value)
	}
	position := toString(msgs["engine_status"])
	if position == "" {
		position = "unbekannt"
	}
	engineAction := toString(msgs["engine_set"])
	if strings.TrimSpace(engineAction) == "" {
		engineAction = toString(msgs["engine"])
	}

	h.mu.Lock()
	if position == "unbekannt" && h.lastStatusPosition != "" {
		position = h.lastStatusPosition
	}
	if battery == "" {
		battery = h.lastStatusBattery
	}
	if wakeReason == "" {
		wakeReason = h.lastStatusWakeReason
	}
	if controllerState == "" {
		controllerState = h.lastStatusController
	}
	if sleepState == "" {
		sleepState = h.lastStatusSleep
	}
	if ip == "" {
		ip = h.lastStatusIP
	}
	if charging == "" {
		charging = h.lastStatusCharging
	}
	if limitClose == "" {
		limitClose = h.lastStatusLimitClose
	}
	if limitOpen == "" {
		limitOpen = h.lastStatusLimitOpen
	}
	h.mu.Unlock()

	stateTs, hasStateTs := h.firstMessageTimestamp("status", "sleepms/status", "sleepms_status")
	h.updateStateTracking(controllerState, sleepState, stateTs, hasStateTs)

	// End switches are authoritative for a final position. The current engine
	// action is only a fallback while the door is moving or between positions.
	doorPos := resolveDoorPosition(limitClose, limitOpen, position, engineAction)

	h.mu.Lock()
	// Persist the latest non-empty status values so the next UI load can fall
	// back to them if MQTT is still empty.
	if toString(msgs["status"]) != "" || sleepState != "" || wakeReason != "" || ip != "" || charging != "" || battery != "" || position != "" || limitClose != "" || limitOpen != "" {
		h.lastStatusPosition = doorPos
		h.lastStatusAction = engineAction
		h.lastStatusBattery = battery
		h.lastStatusWakeReason = wakeReason
		h.lastStatusController = controllerState
		h.lastStatusSleep = sleepState
		h.lastStatusIP = ip
		h.lastStatusCharging = charging
		if limitClose != "" {
			h.lastStatusLimitClose = limitClose
		}
		if limitOpen != "" {
			h.lastStatusLimitOpen = limitOpen
		}

	}
	historyCopy := append([]ScheduleHistoryEntry(nil), h.scheduleHistory...)

	status := StatusResponse{
		Position:         doorPos,
		LastAction:       engineAction,
		Battery:          battery,
		WakeReason:       wakeReason,
		ControllerState:  controllerState,
		SleepState:       sleepState,
		IP:               ip,
		Charging:         charging,
		LimitClose:       limitClose,
		LimitOpen:        limitOpen,
		ScheduleActive:   h.scheduleActive,
		ScheduleTimezone: scheduleNow().Location().String(),
		ServerNowMs:      time.Now().UnixMilli(),
		SleepCommandAtMs: unixMillisOrZero(h.lastSleepCommandAt),
		SleepingAtMs:     unixMillisOrZero(h.sleepingAt),
		OnlineAtMs:       unixMillisOrZero(h.onlineAt),
		WakeDeltaMs:      h.wakeDeltaMs,
		ScheduleHistory:  historyCopy,
	}
	h.mu.Unlock()

	if status.LastAction == "" {
		status.LastAction = "-"
	}
	if status.ControllerState == "" {
		status.ControllerState = "-"
	}
	if status.SleepState == "" {
		status.SleepState = "-"
	}
	if status.WakeReason == "" {
		status.WakeReason = "-"
	}
	if status.IP == "" {
		status.IP = "-"
	}
	if status.Battery == "" {
		status.Battery = "-"
	}
	if status.Charging == "" {
		status.Charging = "-"
	}
	if status.LimitClose == "" {
		status.LimitClose = "-"
	}
	if status.LimitOpen == "" {
		status.LimitOpen = "-"
	}

	// Store the final status snapshot so it survives container restarts.
	h.persistState()
	jsonResponse(w, status)
}

// @brief Returns or stores the shared UI settings for the Hühnerklappe page.
//
// GET returns the currently persisted values. PUT stores the provided values
// so all browsers load the same shared settings.
// @param w HTTP response writer.
// @param r HTTP request.
func (h *ChickenDoor) UIStateHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.mu.Lock()
		response := map[string]any{
			"sleepTime":                 h.sleepTime,
			"motorAutoStopOpenSeconds":  h.motorAutoStopOpenSeconds,
			"motorAutoStopCloseSeconds": h.motorAutoStopCloseSeconds,
			"sleepUntil":                h.sleepUntil,
			"controlMode":               h.controlMode,
			"historyExpanded":           h.historyExpanded,
			"scheduleTimestamps":        append([]string(nil), h.scheduleTimestamps...),
			"scheduleEntries":           append([]ScheduleEntry(nil), h.scheduleEntries...),
			"awakeSeconds":              h.scheduleAwakeSeconds,
			"scheduleActive":            h.scheduleActive,
		}
		h.mu.Unlock()
		jsonResponse(w, response)
		return
	case http.MethodPut:
		var req uiStateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "Ungültiger JSON body")
			return
		}

		h.mu.Lock()
		h.sleepTime = req.SleepTime
		if req.MotorAutoStopOpenSeconds > 0 {
			h.motorAutoStopOpenSeconds = clampMotorAutoStopSeconds(req.MotorAutoStopOpenSeconds)
		}
		if req.MotorAutoStopCloseSeconds > 0 {
			h.motorAutoStopCloseSeconds = clampMotorAutoStopSeconds(req.MotorAutoStopCloseSeconds)
		}
		// Accept the old field for clients that have not migrated yet. It updates
		// both directions, preserving the previous single-value behavior.
		if req.MotorAutoStopSeconds > 0 {
			runtime := clampMotorAutoStopSeconds(req.MotorAutoStopSeconds)
			h.motorAutoStopOpenSeconds = runtime
			h.motorAutoStopCloseSeconds = runtime
		}
		h.sleepUntil = strings.TrimSpace(req.SleepUntil)
		if req.ControlMode == "manual" || req.ControlMode == "schedule" {
			h.controlMode = req.ControlMode
		}
		h.historyExpanded = req.HistoryExpanded
		if req.AwakeSeconds >= 0 {
			h.scheduleAwakeSeconds = req.AwakeSeconds
		}
		if len(req.ScheduleTimestamps) > 0 {
			cleaned := make([]string, 0, len(req.ScheduleTimestamps))
			for _, ts := range req.ScheduleTimestamps {
				trimmed := strings.TrimSpace(ts)
				if trimmed == "" {
					continue
				}
				cleaned = append(cleaned, trimmed)
			}
			if len(cleaned) > 0 {
				h.scheduleTimestamps = append([]string(nil), cleaned...)
				entries := make([]ScheduleEntry, 0, len(cleaned))
				for _, timestamp := range cleaned {
					entries = append(entries, ScheduleEntry{Timestamp: timestamp, Action: "none"})
				}
				h.scheduleEntries = entries
			}
		}
		if len(req.ScheduleEntries) > 0 {
			h.scheduleEntries = append([]ScheduleEntry(nil), req.ScheduleEntries...)
			h.scheduleTimestamps = make([]string, 0, len(req.ScheduleEntries))
			for _, entry := range req.ScheduleEntries {
				h.scheduleTimestamps = append(h.scheduleTimestamps, entry.Timestamp)
			}
		}
		h.mu.Unlock()

		h.persistState()
		jsonResponse(w, map[string]any{"ok": true})
		return
	default:
		w.Header().Set("Allow", "GET, PUT")
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// @brief Sends a manual control command to the chicken door.
//
// Only whitelist keys are allowed. "engine" is published directly to
// nano/esp32/engine, and "engine/sleep" is mapped to nano/esp32/sleepms with
// seconds converted to milliseconds.
// @param w HTTP response writer.
// @param r HTTP request with JSON body {key, value}.
// @brief Validates a time string for the sleep schedule.
// @param value Time string.
// @return true for HH:MM or HH:MM:SS, otherwise false.
// @brief Stores or updates the sleep schedule configuration.
//
// Validates count, timestamps, and awakeSeconds. When active, scheduleWakeAt is
// set to now so runLoop computes the next sleep cycle.
// @param w HTTP response writer.
// @param r HTTP request with JSON body {count,timestamps,awakeSeconds,active}.
// @brief Starts the background service including the scheduler loop.
//
// Initializes a cancellable context and starts runLoop in a goroutine.
func (cd *ChickenDoor) Run() {
	cd.ctx, cd.cancel = context.WithCancel(context.Background())
	log.Println("🐔 ChickenDoor service starting...")

	go cd.runLoop()
}

// @brief Main service loop.
//
// Runs scheduleTick every second and exits immediately on context cancel.
// On exit, done is closed and the ticker is stopped.
func (cd *ChickenDoor) runLoop() {
	defer func() {
		close(cd.done)
		log.Println("🐔 ChickenDoor service stopped")
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cd.ctx.Done():
			return
		case <-ticker.C:
			cd.scheduleTick()
			cd.autoStopTick()
		}
	}
}

// @brief Stops the service gracefully.
//
// Signals shutdown via cancel() and waits on done for runLoop to fully exit.
func (cd *ChickenDoor) Stop() {
	if cd.cancel != nil {
		log.Println("🐔 Stopping ChickenDoor service...")
		cd.cancel()
		<-cd.done
	}

	if cd.db != nil {
		if err := cd.db.Close(); err != nil {
			log.Printf("[chickendoor-state] close failed: %v", err)
		}
	}
}
