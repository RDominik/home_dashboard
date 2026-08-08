package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	chickendoorpkg "webgui-api/chickenDoor"
	"webgui-api/mqtt"
	restpkg "webgui-api/rest"
	wallboxpkg "webgui-api/wallbox"
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

// Debug: MQTT status + received messages
func mqttStatus(w http.ResponseWriter, r *http.Request) {
	var msgs map[string]any
	if mqttManager != nil {
		msgs = mqttManager.Messages()
	} else {
		msgs = make(map[string]any)
	}
	// consider connected if we have received any messages
	connected := len(msgs) > 0
	jsonResponse(w, map[string]any{"connected": connected, "messages": msgs})
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

	// REST worker also runs from main.
	restService := restpkg.NewRestService("rest/rest_config.json", "webgui/rest/varset", 60*time.Second)
	restService.Start(mqttManager)
	defer restService.Stop()
	log.Println("📡 REST service started from main")

	// ChickenDoor background service
	chickenDoorService := chickendoorpkg.New(mqttManager)
	chickenDoorService.Run()
	defer chickenDoorService.Stop()
	log.Println("🐔 ChickenDoor service started from main")

	// Router
	mux := http.NewServeMux()

	// Wallbox
	wb := wallboxpkg.NewHandler(mqttManager)
	mux.HandleFunc("/api/wallbox/status", wb.Status)
	mux.HandleFunc("/api/wallbox/set", wb.Set)
	mux.HandleFunc("/api/wallbox/history", wb.History)

	// Hühnerklappe
	mux.HandleFunc("/api/huehnerklappe/status", chickenDoorService.StatusHandler)
	mux.HandleFunc("/api/huehnerklappe/set", chickenDoorService.SetHandler)
	mux.HandleFunc("/api/huehnerklappe/", chickenDoorService.SleepScheduleHandler)

	// Inverter
	mux.HandleFunc("/api/inverter/summary", inverterSummary)

	// MQTT debug
	mux.HandleFunc("/api/mqtt/status", mqttStatus)

	// Heating
	mux.HandleFunc("/api/heating/summary", restpkg.HeatingSummary)
	mux.HandleFunc("/api/heating/history", restpkg.HeatingHistory)

	addr := ":8083"
	log.Printf("🚀 API server starting on %s", addr)
	if err := http.ListenAndServe(addr, cors(mux)); err != nil {
		log.Fatalf("❌ Server error: %v", err)
	}
}
