package wallbox

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"webgui-api/mqtt"
)

// go-eCharger MQTT SET topic prefix
const goeSetPrefix = "go-eCharger/254959"

var goeAllowedKeys = map[string]bool{
	"amp": true, "frc": true, "psm": true, "dwo": true, "alw": true, "ato": true,
}

// Handler holds the MQTT manager for wallbox endpoints.
type Handler struct {
	mqtt *mqtt.Manager
}

// NewHandler creates a new wallbox Handler.
func NewHandler(m *mqtt.Manager) *Handler {
	return &Handler{mqtt: m}
}

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
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

// Status handles GET /api/wallbox/status
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	status := h.mqtt.Messages()

	nrg, ok := status["nrg"]
	if !ok {
		nrg = make([]any, 16)
	}

	jsonResponse(w, map[string]any{
		"timestamp":   nowISO(),
		"amp":         status["amp"],
		"frc":         status["frc"],
		"psm":         status["psm"],
		"car":         status["car"],
		"nrg":         nrg,
		"modelStatus": status["modelStatus"],
	})
}

type setRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// Set handles POST /api/wallbox/set
func (h *Handler) Set(writer http.ResponseWriter, r *http.Request) {
	var req setRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(writer, http.StatusBadRequest, "Ungültiger JSON body")
		return
	}
	print("Received wallbox set request:", req.Key, req.Value)
	if !goeAllowedKeys[req.Key] {
		keys := make([]string, 0, len(goeAllowedKeys))
		for k := range goeAllowedKeys {
			keys = append(keys, k)
		}
		jsonResponse(writer, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Key '%s' nicht erlaubt. Erlaubt: %s", req.Key, strings.Join(keys, ", ")),
		})
		return
	}

	topic := fmt.Sprintf("%s/%s/set", goeSetPrefix, req.Key)
	if err := h.mqtt.Publish(topic, req.Value); err != nil {
		jsonResponse(writer, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	jsonResponse(writer, map[string]any{
		"ok":    true,
		"topic": topic,
		"key":   req.Key,
		"value": req.Value,
	})
}

// History handles GET /api/wallbox/history
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}

	var points []map[string]any
	end := time.Now().UTC()
	start := end.Add(-2 * time.Hour)
	t := start
	for !t.After(end) {
		points = append(points, map[string]any{
			"t":             t.Format("2006-01-02T15:04:05.000Z"),
			"amp":           6 + ((t.Minute() % 5) - 2),
			"currentEnergy": 1000 + (t.Minute() * 3),
		})
		t = t.Add(5 * time.Minute)
	}
	jsonResponse(w, map[string]any{"series": points, "interval": interval})
}
