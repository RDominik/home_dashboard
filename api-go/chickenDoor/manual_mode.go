package chickendoor

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

// @brief Identifies an allowed manual control request.
type setRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

var allowedSetKeys = map[string]bool{
	"engine":       true,
	"engine/sleep": true,
}

// @brief Sends a manual control command to the chicken door.
//
// Publishes engine commands directly and maps engine/sleep seconds to the
// controller's sleepms topic. Manual motor runs use the same completion buffer
// as scheduled runs so their duration is stored in the next history row.
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
