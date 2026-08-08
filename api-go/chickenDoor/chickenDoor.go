package chickendoor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"webgui-api/mqtt"
)

const nanoSetPrefix = "nano/esp32"

const scheduleWakeDelay = 10 * time.Second

var allowedSetKeys = map[string]bool{
	"engine":       true,
	"engine/sleep": true,
}

type setRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type sleepScheduleRequest struct {
	Count        int      `json:"count"`
	Timestamps   []string `json:"timestamps"`
	AwakeSeconds int      `json:"awakeSeconds"`
	Active       bool     `json:"active"`
}

type StatusResponse struct {
	Position         string `json:"position"`
	LastAction       string `json:"lastAction"`
	Error            string `json:"error,omitempty"`
	Battery          int    `json:"battery"`
	WakeReason       string `json:"wakeReason"`
	ControllerState  string `json:"controllerState"`
	SleepState       string `json:"sleepState"`
	IP               string `json:"ip"`
	Charging         string `json:"charging"`
	ScheduleActive   bool   `json:"scheduleActive"`
	ServerNowMs      int64  `json:"serverNowMs"`
	SleepCommandAtMs int64  `json:"sleepCommandAtMs,omitempty"`
	SleepingAtMs     int64  `json:"sleepingAtMs,omitempty"`
	OnlineAtMs       int64  `json:"onlineAtMs,omitempty"`
	WakeDeltaMs      int64  `json:"wakeDeltaMs,omitempty"`
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
	mu          sync.Mutex

	lastControllerState string
	lastSleepCommandAt  time.Time
	sleepingAt          time.Time
	onlineAt            time.Time
	wakeDeltaMs         int64

	scheduleAwakeSeconds int
	scheduleTimestamps   []string
	scheduleActive       bool
	scheduleWakeAt       time.Time

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// @brief Creates a new ChickenDoor instance.
// @param mqttManager MQTT manager used for publish and status access.
// @return Initialized ChickenDoor instance with a ready-to-use done channel.
func New(mqttManager *mqtt.Manager) *ChickenDoor {
	return &ChickenDoor{
		mqttManager: mqttManager,
		done:        make(chan struct{}),
	}
}

// @brief Parses a time-of-day string relative to base day and timezone.
//
// Supported formats: HH:MM and HH:MM:SS. The date is taken from base so the
// returned timestamp stays in the same day context.
// @param base Reference time providing date and location.
// @param value Time string to parse.
// @return Timestamp built for base day, or an error.
func parseScheduleTimestampForDay(base time.Time, value string) (time.Time, error) {
	layout := "15:04"
	if strings.Count(value, ":") == 2 {
		layout = "15:04:05"
	}

	parsed, err := time.ParseInLocation(layout, value, base.Location())
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(
		base.Year(),
		base.Month(),
		base.Day(),
		parsed.Hour(),
		parsed.Minute(),
		parsed.Second(),
		0,
		base.Location(),
	), nil
}

// @brief Computes the next valid schedule timestamp relative to now.
//
// Past timestamps are shifted to the next day; then the earliest candidate is
// returned.
// @param now Current reference time.
// @param timestamps List of time strings in HH:MM or HH:MM:SS format.
// @return Next execution timestamp, or an error for invalid input.
func nextScheduleTimestamp(now time.Time, timestamps []string) (time.Time, error) {
	if len(timestamps) == 0 {
		return time.Time{}, fmt.Errorf("keine timestamps konfiguriert")
	}

	candidates := make([]time.Time, 0, len(timestamps))
	for _, ts := range timestamps {
		candidate, err := parseScheduleTimestampForDay(now, ts)
		if err != nil {
			return time.Time{}, fmt.Errorf("ungültiger timestamp '%s': %w", ts, err)
		}
		if !candidate.After(now) {
			candidate = candidate.Add(24 * time.Hour)
		}
		candidates = append(candidates, candidate)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Before(candidates[j])
	})

	return candidates[0], nil
}

// @brief Calculates and sends a sleep command until the next schedule timestamp.
//
// The method atomically reads the current schedule, computes remaining time,
// publishes milliseconds to nano/esp32/sleepms, and updates lastSleepCommandAt.
// @param reason Context label for logging.
func (h *ChickenDoor) scheduleSleepUntilNext(reason string) {
	h.mu.Lock()
	active := h.scheduleActive
	timestamps := append([]string(nil), h.scheduleTimestamps...)
	h.mu.Unlock()

	if !active || len(timestamps) == 0 {
		return
	}

	now := time.Now()
	nextTs, err := nextScheduleTimestamp(now, timestamps)
	if err != nil {
		log.Printf("[chickendoor-schedule] next timestamp error (%s): %v", reason, err)
		return
	}

	duration := nextTs.Sub(now)
	if duration <= 0 {
		duration = time.Second
	}

	sleepSeconds := int(math.Ceil(duration.Seconds()))
	if sleepSeconds < 1 {
		sleepSeconds = 1
	}
	payloadMs := sleepSeconds * 1000

	if err := h.mqttManager.Publish(fmt.Sprintf("%s/sleepms", nanoSetPrefix), payloadMs); err != nil {
		log.Printf("[chickendoor-schedule] publish sleep failed (%s): %v", reason, err)
		return
	}

	h.mu.Lock()
	h.lastSleepCommandAt = time.Now()
	h.mu.Unlock()

	log.Printf("[chickendoor-schedule] sleep sent (%s): %ds until %s", reason, sleepSeconds, nextTs.Format("15:04:05"))
}

// @brief Updates transition state and timestamps for sleeping/online changes.
//
// On sleeping transition, sleepingAt is set. On sleeping->online, onlineAt,
// wakeDeltaMs, and (if active) scheduleWakeAt are updated.
// @param controllerState Raw state from status.
// @param sleepState Raw state from sleepms/status.
// @param stateTs Receive timestamp of the state message.
// @param hasStateTs Indicates whether stateTs is valid.
func (h *ChickenDoor) updateStateTracking(controllerState, sleepState string, stateTs time.Time, hasStateTs bool) {
	stateForTransition := normalizeControllerState(controllerState)
	if stateForTransition == "" {
		stateForTransition = normalizeControllerState(sleepState)
	}

	if stateForTransition == "" || !hasStateTs {
		return
	}

	h.mu.Lock()
	if (stateForTransition == "sleeping" || stateForTransition == "offline") && h.lastControllerState != stateForTransition {
		h.sleepingAt = stateTs
	}
	if stateForTransition == "online" && (h.lastControllerState == "sleeping" || h.lastControllerState == "offline") {
		h.onlineAt = stateTs
		if !h.sleepingAt.IsZero() && !h.onlineAt.Before(h.sleepingAt) {
			h.wakeDeltaMs = h.onlineAt.Sub(h.sleepingAt).Milliseconds()
		}
		if h.scheduleActive {
			h.scheduleWakeAt = time.Now().Add(scheduleWakeDelay)
		}
	}
	h.lastControllerState = stateForTransition
	h.mu.Unlock()
}

// @brief Executes one periodic scheduler tick.
//
// Reads MQTT states, updates transition tracking, and triggers the next sleep
// cycle after scheduleWakeAt has elapsed.
func (h *ChickenDoor) scheduleTick() {
	msgs := h.mqttManager.Messages()
	controllerState := toString(msgs["status"])
	sleepState := toString(firstValue(msgs, "sleepms/status", "sleepms_status"))

	stateTs, hasStateTs := h.firstMessageTimestamp("status", "sleepms/status", "sleepms_status")

	h.updateStateTracking(controllerState, sleepState, stateTs, hasStateTs)

	// Fast path: if scheduling is disabled or not armed, skip further checks.
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
	if currentState != "online" || time.Now().Before(wakeAt) {
		return
	}

	h.mu.Lock()
	shouldSchedule := h.scheduleActive && !h.scheduleWakeAt.IsZero() && !time.Now().Before(h.scheduleWakeAt)
	if shouldSchedule {
		h.scheduleWakeAt = time.Time{}
	}
	h.mu.Unlock()

	if shouldSchedule {
		h.scheduleSleepUntilNext("post-online-delay")
	}
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

// @brief Returns the current ChickenDoor status as JSON.
//
// Reads MQTT values including sleep/wakeup info, updates internal transition
// tracking, and returns consolidated status data for the UI.
// @param w HTTP response writer.
// @param r HTTP request (unused).
func (h *ChickenDoor) StatusHandler(w http.ResponseWriter, r *http.Request) {
	msgs := h.mqttManager.Messages()

	controllerState := toString(msgs["status"])
	sleepState := toString(firstValue(msgs, "sleepms/status", "sleepms_status"))
	wakeReason := toString(firstValue(msgs, "sleepms/wakeup_reason", "sleepms_wakeup_reason"))
	ip := toString(msgs["ip"])
	charging := toString(msgs["battery_charging"])
	battery := toInt(msgs["battery_percent"])
	position := toString(msgs["engine_status"])
	if position == "" {
		position = "unbekannt"
	}

	stateTs, hasStateTs := h.firstMessageTimestamp("status", "sleepms/status", "sleepms_status")
	h.updateStateTracking(controllerState, sleepState, stateTs, hasStateTs)

	h.mu.Lock()

	status := StatusResponse{
		Position:         position,
		LastAction:       toString(msgs["engine_set"]),
		Battery:          battery,
		WakeReason:       wakeReason,
		ControllerState:  controllerState,
		SleepState:       sleepState,
		IP:               ip,
		Charging:         charging,
		ScheduleActive:   h.scheduleActive,
		ServerNowMs:      time.Now().UnixMilli(),
		SleepCommandAtMs: unixMillisOrZero(h.lastSleepCommandAt),
		SleepingAtMs:     unixMillisOrZero(h.sleepingAt),
		OnlineAtMs:       unixMillisOrZero(h.onlineAt),
		WakeDeltaMs:      h.wakeDeltaMs,
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
	if status.Charging == "" {
		status.Charging = "-"
	}

	jsonResponse(w, status)
}

// @brief Sends a manual control command to the chicken door.
//
// Only whitelist keys are allowed. "engine" is published directly to
// nano/esp32/engine, and "engine/sleep" is mapped to nano/esp32/sleepms with
// seconds converted to milliseconds.
// @param w HTTP response writer.
// @param r HTTP request with JSON body {key, value}.
func (h *ChickenDoor) SetHandler(w http.ResponseWriter, r *http.Request) {
	var req setRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Ungültiger JSON body")
		return
	}

	if !allowedSetKeys[req.Key] {
		keys := make([]string, 0, len(allowedSetKeys))
		for key := range allowedSetKeys {
			keys = append(keys, key)
		}
		jsonResponse(w, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Key '%s' nicht erlaubt. Erlaubt: %s", req.Key, strings.Join(keys, ", ")),
		})
		return
	}

	topic := fmt.Sprintf("%s/%s/set", nanoSetPrefix, req.Key)
	if req.Key == "engine" {
		topic = fmt.Sprintf("%s/engine", nanoSetPrefix)
	}
	payload := req.Value
	if req.Key == "engine/sleep" {
		topic = fmt.Sprintf("%s/sleepms", nanoSetPrefix)
		payload = toInt(req.Value) * 1000
	}
	if err := h.mqttManager.Publish(topic, payload); err != nil {
		jsonResponse(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	if req.Key == "engine/sleep" {
		h.mu.Lock()
		h.lastSleepCommandAt = time.Now()
		h.mu.Unlock()
	}

	jsonResponse(w, map[string]any{
		"ok":    true,
		"topic": topic,
		"key":   req.Key,
		"value": req.Value,
	})
}

// @brief Validates a time string for the sleep schedule.
// @param value Time string.
// @return true for HH:MM or HH:MM:SS, otherwise false.
func isValidScheduleTimestamp(value string) bool {
	if _, err := time.Parse("15:04", value); err == nil {
		return true
	}
	if _, err := time.Parse("15:04:05", value); err == nil {
		return true
	}
	return false
}

// @brief Stores or updates the sleep schedule configuration.
//
// Validates count, timestamps, and awakeSeconds. When active, scheduleWakeAt is
// set to now so runLoop computes the next sleep cycle.
// @param w HTTP response writer.
// @param r HTTP request with JSON body {count,timestamps,awakeSeconds,active}.
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
	for _, ts := range req.Timestamps {
		trimmed := strings.TrimSpace(ts)
		if trimmed == "" {
			jsonError(w, http.StatusBadRequest, "Leere Timestamps sind nicht erlaubt")
			return
		}
		if !isValidScheduleTimestamp(trimmed) {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("Ungültiger Timestamp '%s'. Erlaubt: HH:MM oder HH:MM:SS", trimmed))
			return
		}
		cleanedTimestamps = append(cleanedTimestamps, trimmed)
	}

	h.mu.Lock()
	h.scheduleAwakeSeconds = req.AwakeSeconds
	h.scheduleTimestamps = append([]string(nil), cleanedTimestamps...)
	h.scheduleActive = req.Active
	if req.Active {
		// Trigger processing in runLoop; next timestamp is calculated there.
		h.scheduleWakeAt = time.Now()
	} else {
		h.scheduleWakeAt = time.Time{}
	}
	h.mu.Unlock()

	jsonResponse(w, map[string]any{
		"ok":           true,
		"stored":       true,
		"count":        req.Count,
		"timestamps":   cleanedTimestamps,
		"awakeSeconds": req.AwakeSeconds,
		"active":       req.Active,
	})
}

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
}
