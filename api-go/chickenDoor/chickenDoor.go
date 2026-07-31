package chickendoor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"webgui-api/mqtt"
)

const nanoSetPrefix = "nano/esp32"

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
	ServerNowMs      int64  `json:"serverNowMs"`
	SleepCommandAtMs int64  `json:"sleepCommandAtMs,omitempty"`
	SleepingAtMs     int64  `json:"sleepingAtMs,omitempty"`
	OnlineAtMs       int64  `json:"onlineAtMs,omitempty"`
	WakeDeltaMs      int64  `json:"wakeDeltaMs,omitempty"`
}

func unixMillisOrZero(ts time.Time) int64 {
	if ts.IsZero() {
		return 0
	}
	return ts.UnixMilli()
}

type APIHandler struct {
	mqttManager *mqtt.Manager
	mu          sync.Mutex

	lastControllerState string
	lastSleepCommandAt  time.Time
	sleepingAt          time.Time
	onlineAt            time.Time
	wakeDeltaMs         int64

	scheduleAwakeSeconds int
	scheduleTimestamps   []string
}

func NewAPIHandler(mqttManager *mqtt.Manager) *APIHandler {
	return &APIHandler{mqttManager: mqttManager}
}

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

func normalizeControllerState(state string) string {
	value := strings.ToLower(strings.TrimSpace(state))
	switch {
	case strings.Contains(value, "sleep"):
		return "sleeping"
	case strings.Contains(value, "online"):
		return "online"
	default:
		return value
	}
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func (h *APIHandler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	msgs := h.mqttManager.Messages()

	controllerState := toString(msgs["status"])
	sleepState := toString(msgs["sleepms_status"])
	wakeReason := toString(msgs["sleepms_wakeup_reason"])
	ip := toString(msgs["ip"])
	charging := toString(msgs["battery_charging"])
	battery := toInt(msgs["battery_percent"])
	position := toString(msgs["engine_status"])
	if position == "" {
		position = "unbekannt"
	}

	stateForTransition := normalizeControllerState(controllerState)
	if stateForTransition == "" {
		stateForTransition = normalizeControllerState(sleepState)
	}

	_, stateTs, hasStateTs := h.mqttManager.MessageWithTimestamp("status")
	if !hasStateTs {
		_, stateTs, hasStateTs = h.mqttManager.MessageWithTimestamp("sleepms_status")
	}

	h.mu.Lock()
	if stateForTransition != "" && hasStateTs {
		if stateForTransition == "sleeping" && h.lastControllerState != "sleeping" {
			h.sleepingAt = stateTs
		}
		if stateForTransition == "online" && h.lastControllerState == "sleeping" {
			h.onlineAt = stateTs
			if !h.sleepingAt.IsZero() && !h.onlineAt.Before(h.sleepingAt) {
				h.wakeDeltaMs = h.onlineAt.Sub(h.sleepingAt).Milliseconds()
			}
		}
		h.lastControllerState = stateForTransition
	}

	status := StatusResponse{
		Position:         position,
		LastAction:       toString(msgs["engine_set"]),
		Battery:          battery,
		WakeReason:       wakeReason,
		ControllerState:  controllerState,
		SleepState:       sleepState,
		IP:               ip,
		Charging:         charging,
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

func (h *APIHandler) SetHandler(w http.ResponseWriter, r *http.Request) {
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

func isValidScheduleTimestamp(value string) bool {
	if _, err := time.Parse("15:04", value); err == nil {
		return true
	}
	if _, err := time.Parse("15:04:05", value); err == nil {
		return true
	}
	return false
}

func (h *APIHandler) SleepScheduleHandler(w http.ResponseWriter, r *http.Request) {
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
	h.mu.Unlock()

	jsonResponse(w, map[string]any{
		"ok":           true,
		"stored":       true,
		"count":        req.Count,
		"timestamps":   cleanedTimestamps,
		"awakeSeconds": req.AwakeSeconds,
	})
}

// ChickenDoor represents the chicken door module
type ChickenDoor struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a new ChickenDoor instance
func New() *ChickenDoor {
	return &ChickenDoor{
		done: make(chan struct{}),
	}
}

// Run starts the chicken door service
func (cd *ChickenDoor) Run() {
	cd.ctx, cd.cancel = context.WithCancel(context.Background())
	log.Println("🐔 ChickenDoor service starting...")

	go cd.runLoop()
}

// runLoop is the main service loop
func (cd *ChickenDoor) runLoop() {
	defer func() {
		close(cd.done)
		log.Println("🐔 ChickenDoor service stopped")
	}()

	// TODO: Add your service logic here
	<-cd.ctx.Done()
}

// Stop gracefully stops the chicken door service
func (cd *ChickenDoor) Stop() {
	if cd.cancel != nil {
		log.Println("🐔 Stopping ChickenDoor service...")
		cd.cancel()
		<-cd.done
	}
}
