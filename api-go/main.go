package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"webgui-api/mqtt"
)

var mqttManager *mqtt.Manager

// ---------- Helpers ----------

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

// ---------- CORS Middleware ----------

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- Wallbox ----------

// go-eCharger MQTT SET topic prefix
const goeSetPrefix = "go-eCharger/254959"

var goeAllowedKeys = map[string]bool{
	"amp": true, "frc": true, "psm": true, "dwo": true, "alw": true,
}

func wallboxStatus(w http.ResponseWriter, r *http.Request) {
	status := mqttManager.Messages()

	// nrg: default to empty array with 16 zeros
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

type wallboxSetRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func wallboxSet(writer http.ResponseWriter, r *http.Request) {
	var req wallboxSetRequest
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
	if err := mqttManager.Publish(topic, req.Value); err != nil {
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

func wallboxHistory(w http.ResponseWriter, r *http.Request) {
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

// ---------- Inverter ----------

func inverterSummary(writer http.ResponseWriter, r *http.Request) {
	status := mqttManager.Messages()
	log.Printf("🔍 Inverter summary requested, current message: %v", status)

	var carPower any = 0
	if nrg, ok := status["nrg"]; ok {
		if arr, ok := nrg.([]any); ok && len(arr) > 11 {
			carPower = arr[11]
		}
	}

	jsonResponse(writer, map[string]any{
		"timestamp":         nowISO(),
		"ppv":               status["ppv"],
		"house_consumption": status["house_consumption"],
		"battery_soc":       status["battery_soc"],
		"pbattery":          status["pbattery"],
		"car_power":         carPower,
	})
}

// ---------- System Update ----------

type stepResult struct {
	Step   string `json:"step"`
	OK     bool   `json:"ok"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
}

func systemUpdate(w http.ResponseWriter, r *http.Request) {
	repoDir := "/repo"
	var results []stepResult

	// 1. git pull
	gitCmd := exec.Command("git", "pull")
	gitCmd.Dir = repoDir
	gitOut, gitErr := gitCmd.Output()
	gitStderr := ""
	if gitErr != nil {
		if exitErr, ok := gitErr.(*exec.ExitError); ok {
			gitStderr = string(exitErr.Stderr)
		} else {
			gitStderr = gitErr.Error()
		}
	}
	gitOK := gitErr == nil
	results = append(results, stepResult{
		Step:   "git pull",
		OK:     gitOK,
		Stdout: strings.TrimSpace(string(gitOut)),
		Stderr: strings.TrimSpace(gitStderr),
	})
	if !gitOK {
		jsonResponse(w, map[string]any{"ok": false, "results": results})
		return
	}

	// 2. docker compose build + up
	buildCmd := exec.Command("docker", "compose", "-f", "docker-compose.webui.yml", "up", "--build", "-d")
	buildCmd.Dir = repoDir
	buildOut, buildErr := buildCmd.Output()
	buildStderr := ""
	if buildErr != nil {
		if exitErr, ok := buildErr.(*exec.ExitError); ok {
			buildStderr = string(exitErr.Stderr)
		} else {
			buildStderr = buildErr.Error()
		}
	}
	buildOK := buildErr == nil

	// Ausgabe auf 500 Zeichen begrenzen
	stdout := string(buildOut)
	if len(stdout) > 500 {
		stdout = stdout[len(stdout)-500:]
	}
	stderr := buildStderr
	if len(stderr) > 500 {
		stderr = stderr[len(stderr)-500:]
	}

	results = append(results, stepResult{
		Step:   "docker compose up --build -d",
		OK:     buildOK,
		Stdout: strings.TrimSpace(stdout),
		Stderr: strings.TrimSpace(stderr),
	})

	allOK := true
	for _, r := range results {
		if !r.OK {
			allOK = false
		}
	}
	jsonResponse(w, map[string]any{"ok": allOK, "results": results})
}

// ---------- Heating (Mock) ----------

func heatingSummary(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]any{
		"timestamp":     nowISO(),
		"boiler_temp":   72.5,
		"buffer_top":    68.3,
		"buffer_bottom": 45.8,
		"return_temp":   52.1,
		"outside_temp":  3.4,
		"feed_rate":     35,
		"burner_status": "on",
	})
}

func heatingHistory(w http.ResponseWriter, r *http.Request) {
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}

	var points []map[string]any
	end := time.Now().UTC()
	start := end.Add(-12 * time.Hour)
	t := start
	for !t.After(end) {
		min := t.Minute()
		points = append(points, map[string]any{
			"t":             t.Format("2006-01-02T15:04:05.000Z"),
			"boiler_temp":   70.0 + float64(min%10)*0.4,
			"buffer_top":    66.0 + float64(min%8)*0.3,
			"buffer_bottom": 44.0 + float64(min%6)*0.25,
			"return_temp":   50.0 + float64(min%12)*0.2,
			"feed_rate":     30 + (min%5)*3,
		})
		t = t.Add(5 * time.Minute)
	}
	jsonResponse(w, map[string]any{"series": points, "interval": interval})
}

// ---------- Main ----------

func main() {
	// MQTT starten
	var err error
	mqttManager, err = mqtt.NewManager("mqtt/broker_config.json")
	if err != nil {
		log.Fatalf("❌ MQTT config error: %v", err)
	}
	go mqttManager.Run()
	defer mqttManager.Stop()

	// Router
	mux := http.NewServeMux()

	// Wallbox
	mux.HandleFunc("/api/wallbox/status", wallboxStatus)
	mux.HandleFunc("/api/wallbox/set", wallboxSet)
	mux.HandleFunc("/api/wallbox/history", wallboxHistory)

	// Inverter
	mux.HandleFunc("/api/inverter/summary", inverterSummary)

	// System
	mux.HandleFunc("/api/system/update", systemUpdate)

	// Heating
	mux.HandleFunc("/api/heating/summary", heatingSummary)
	mux.HandleFunc("/api/heating/history", heatingHistory)

	addr := ":8083"
	log.Printf("🚀 API server starting on %s", addr)
	if err := http.ListenAndServe(addr, cors(mux)); err != nil {
		log.Fatalf("❌ Server error: %v", err)
	}
}
