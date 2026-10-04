package chickendoor

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// @brief Identifies an allowed manual control request.
type setRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

var allowedSetKeys = map[string]bool{
	"engine":           true,
	"engine/sleep":     true,
	"engineMaxRuntime": true,
}

// @brief Parses a requested motor maximum runtime in whole seconds.
// @details
// The setting is intentionally constrained to the same 1..60 second range as
// directional auto-stop settings. Fractional JSON numbers and unsupported
// payload types are rejected rather than silently truncated.
// @param value Value decoded from the set endpoint's JSON request.
// @return Valid runtime in seconds, or an error describing invalid input.
func parseEngineMaxRuntime(value any) (int, error) {
	var seconds int
	switch typed := value.(type) {
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("engineMaxRuntime muss eine ganze Sekundenzahl sein")
		}
		seconds = int(typed)
	case int:
		seconds = typed
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("engineMaxRuntime muss eine ganze Sekundenzahl sein")
		}
		seconds = parsed
	default:
		return 0, fmt.Errorf("engineMaxRuntime muss als Sekundenzahl angegeben werden")
	}
	if seconds < 1 || seconds > 60 {
		return 0, fmt.Errorf("engineMaxRuntime muss zwischen 1 und 60 Sekunden liegen")
	}
	return seconds, nil
}

// @brief Sends a manual control command to the chicken door.
//
// Publishes engine commands directly and maps engine/sleep seconds to the
// controller's sleepms topic. Manual motor runs use the same completion buffer
// as scheduled runs so their duration is stored in the next history row. The
// engineMaxRuntime key validates and publishes a shared seconds-based ceiling
// to nano/esp32/engineMaxRuntime and persists it after broker acceptance.
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
	if req.Key == "engineMaxRuntime" {
		seconds, err := parseEngineMaxRuntime(req.Value)
		if err != nil {
			jsonResponse(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if err := h.mqttManager.Publish(engineMaxRuntimeTopic, seconds); err != nil {
			jsonResponse(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}

		// Persist only after broker acceptance so the displayed shared setting
		// does not claim a configuration that was never successfully published.
		h.mu.Lock()
		h.engineMaxRuntimeSeconds = seconds
		h.mu.Unlock()
		h.persistState()
		jsonResponse(w, map[string]any{
			"ok":    true,
			"topic": engineMaxRuntimeTopic,
			"key":   req.Key,
			"value": seconds,
		})
		return
	}

	topic := fmt.Sprintf("%s/%s/set", nanoSetPrefix, req.Key)
	if req.Key == "engine" {
		topic = fmt.Sprintf("%s/engine", nanoSetPrefix)
		command := strings.ToLower(strings.TrimSpace(toString(req.Value)))
		if command == "open" || command == "close" {
			h.mu.Lock()
			runtimeSeconds := h.motorAutoStopOpenSeconds
			if command == "close" {
				runtimeSeconds = h.motorAutoStopCloseSeconds
			}
			h.mu.Unlock()
			if err := h.publishMotorRuntime(command, runtimeSeconds); err != nil {
				jsonResponse(w, map[string]any{"ok": false, "error": err.Error()})
				return
			}
		}
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

	if req.Key == "engine" {
		command := strings.ToLower(strings.TrimSpace(toString(req.Value)))
		h.mu.Lock()
		if command == "open" || command == "close" {
			h.motorRunningSince = time.Now()
			h.motorRunningAction = command
			h.motorLimitReachedAt = time.Time{}
		} else if command == "stop" {
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
				h.completedMotorDuration = elapsedSec
				doorPos := resolveDoorPosition(h.lastStatusLimitClose, h.lastStatusLimitOpen, "stop", "stop")
				if doorPos != "" && doorPos != "unbekannt" {
					h.completedMotorPosition = doorPos
				}
			}
			h.motorRunningSince = time.Time{}
			h.motorRunningAction = ""
			h.motorLimitReachedAt = time.Time{}
		}
		h.mu.Unlock()
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
