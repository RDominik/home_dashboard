package enyaq

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"

	"webgui-api/mqtt"
)

const (
	defaultDBPath          = "data/enyaq.db"
	dbBucketConfig         = "enyaq_config"
	dbBucketState          = "enyaq_state"
	dbBucketHistory        = "enyaq_history"
	dbKeyConfig            = "config"
	dbKeyState             = "state"
	dbKeyHistory           = "history"
	defaultPollIntervalMin = 10
	maxHistoryEntries      = 50
	httpTimeout            = 15 * time.Second
	mqttTopicPrefix        = "enyaq"
)

// @brief Configuration settings for the Škoda Enyaq integration.
//
// All fields are persisted in bbolt so they survive service and container restarts.
type Config struct {
	// @brief The secret API token (e.g. Bearer token, MyŠkoda access token or custom API key).
	APIToken string `json:"apiToken"`

	// @brief The 17-character Vehicle Identification Number (VIN / FIN).
	VIN string `json:"vin"`

	// @brief The target API endpoint URL used to query vehicle telemetry.
	APIURL string `json:"apiUrl"`

	// @brief Polling interval in minutes (default is 10 minutes).
	PollIntervalMinutes int `json:"pollIntervalMinutes"`

	// @brief Flag indicating whether background polling is actively enabled.
	AutoPollEnabled bool `json:"autoPollEnabled"`

	// @brief Timestamp when the configuration was last modified.
	LastConfigUpdate string `json:"lastConfigUpdate,omitempty"`
}

// @brief Telemetry and status data for the Škoda Enyaq.
//
// Represents battery, charging, climate, lock, and odometer status.
type VehicleData struct {
	// @brief Current state of charge of the traction battery in percent (0..100).
	BatteryLevelPct float64 `json:"batteryLevelPct"`

	// @brief Target charge limit percentage (e.g. 80% or 100%).
	TargetSoCPct float64 `json:"targetSoCPct"`

	// @brief Estimated remaining driving range in kilometers.
	RemainingRangeKm float64 `json:"remainingRangeKm"`

	// @brief Current charging status (e.g. "charging", "idle", "complete", "conserving", "error", "disconnected").
	ChargingState string `json:"chargingState"`

	// @brief Instantaneous charging power in kilowatts (kW).
	ChargePowerKw float64 `json:"chargePowerKw"`

	// @brief Charging rate in kilometers per hour.
	ChargeRateKmPerHour float64 `json:"chargeRateKmPerHour"`

	// @brief Estimated remaining charging duration in minutes until target SoC is reached.
	RemainingChargingTimeMin int `json:"remainingChargingTimeMin"`

	// @brief Charging mode ("ac", "dc", or "none").
	ChargingMode string `json:"chargingMode"`

	// @brief Cable / plug connection state ("connected", "disconnected").
	PlugConnectionState string `json:"plugConnectionState"`

	// @brief Charging cable lock status ("locked", "unlocked").
	PlugLockState string `json:"plugLockState"`

	// @brief Total odometer reading / mileage in kilometers.
	MileageKm int `json:"mileageKm"`

	// @brief Overall vehicle lock status ("locked", "unlocked", "open").
	LockState string `json:"lockState"`

	// @brief True if all passenger doors are closed.
	DoorsClosed bool `json:"doorsClosed"`

	// @brief True if all side windows are closed.
	WindowsClosed bool `json:"windowsClosed"`

	// @brief True if trunk / tailgate is closed.
	TrunkClosed bool `json:"trunkClosed"`

	// @brief True if bonnet / engine hood is closed.
	BonnetClosed bool `json:"bonnetClosed"`

	// @brief True if parking lights or headlights are active.
	LightsOn bool `json:"lightsOn"`

	// @brief Target interior temperature in degrees Celsius (°C).
	TargetTemperatureC float64 `json:"targetTemperatureC"`

	// @brief Current climatisation state ("off", "heating", "cooling", "ventilation").
	ClimatisationState string `json:"climatisationState"`

	// @brief True if windscreen / front window heating is turned on.
	WindowHeatingFront bool `json:"windowHeatingFront"`

	// @brief True if rear window heating is turned on.
	WindowHeatingRear bool `json:"windowHeatingRear"`

	// @brief Outside / ambient temperature in degrees Celsius (°C).
	OutdoorTemperatureC float64 `json:"outdoorTemperatureC"`

	// @brief Model descriptor (e.g. "Škoda Enyaq Coupé iV").
	ModelName string `json:"modelName"`

	// @brief ISO8601 timestamp of the last successful data fetch.
	LastUpdated string `json:"lastUpdated"`

	// @brief Status description of the last poll ("ok", "error", "idle", "no_token").
	LastPollStatus string `json:"lastPollStatus"`

	// @brief Detailed error message from the last poll attempt if it failed.
	LastError string `json:"lastError,omitempty"`

	// @brief Raw JSON payload response from vehicle API for inspection and debugging.
	RawPayload string `json:"rawPayload,omitempty"`
}

// @brief Represents a single historical sample of Enyaq telemetry.
type HistoryEntry struct {
	// @brief ISO8601 timestamp of when the telemetry was recorded.
	Timestamp string `json:"timestamp"`

	// @brief Battery state of charge in percent at record time.
	BatteryLevelPct float64 `json:"batteryLevelPct"`

	// @brief Remaining driving range in kilometers at record time.
	RemainingRangeKm float64 `json:"remainingRangeKm"`

	// @brief Charging state at record time.
	ChargingState string `json:"chargingState"`

	// @brief Charging power in kW at record time.
	ChargePowerKw float64 `json:"chargePowerKw"`

	// @brief Total mileage in kilometers at record time.
	MileageKm int `json:"mileageKm"`

	// @brief Lock state at record time.
	LockState string `json:"lockState"`
}

// @brief Public sanitized config response for UI display (masks secret token).
type ConfigResponse struct {
	HasToken            bool   `json:"hasToken"`
	TokenPreview        string `json:"tokenPreview"`
	VIN                 string `json:"vin"`
	APIURL              string `json:"apiUrl"`
	PollIntervalMinutes int    `json:"pollIntervalMinutes"`
	AutoPollEnabled     bool   `json:"autoPollEnabled"`
	LastConfigUpdate    string `json:"lastConfigUpdate,omitempty"`
}

// @brief Complete status response payload returned by the GET /api/enyaq/status endpoint.
type StatusResponse struct {
	Config          ConfigResponse `json:"config"`
	Data            VehicleData    `json:"data"`
	History         []HistoryEntry `json:"history"`
	ServerTime      string         `json:"serverTime"`
	NextPollInSec   int            `json:"nextPollInSec"`
	PollIntervalMin int            `json:"pollIntervalMin"`
}

// @brief Core service managing Škoda Enyaq polling, persistence, MQTT publishing, and HTTP handling.
type Service struct {
	mqttManager *mqtt.Manager
	db          *bbolt.DB
	httpClient  *http.Client
	mu          sync.RWMutex

	config     Config
	data       VehicleData
	history    []HistoryEntry
	lastPollAt time.Time
	nextPollAt time.Time

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// @brief Opens or creates the bbolt database for Enyaq persistence.
// @return Opened bbolt DB instance, or nil on fatal failure.
func openEnyaqDB() *bbolt.DB {
	path := strings.TrimSpace(os.Getenv("ENYAQ_DB_PATH"))
	if path == "" {
		path = defaultDBPath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("[enyaq-db] mkdir failed for %s: %v", path, err)
		return nil
	}

	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Printf("[enyaq-db] open failed for %s: %v", path, err)
		return nil
	}

	// Initialize all required buckets in a single transaction
	if err := db.Update(func(tx *bbolt.Tx) error {
		for _, bName := range []string{dbBucketConfig, dbBucketState, dbBucketHistory} {
			if _, err := tx.CreateBucketIfNotExists([]byte(bName)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Printf("[enyaq-db] bucket init failed: %v", err)
		_ = db.Close()
		return nil
	}

	log.Printf("[enyaq-db] initialized database at %s", path)
	return db
}

// @brief Constructs a new Enyaq Service instance and restores persisted state.
// @param mqttManager MQTT manager instance used to publish telemetry topics.
// @return Initialized Service pointer.
func NewService(mqttManager *mqtt.Manager) *Service {
	// Custom HTTP client with timeout and relaxed TLS for local proxies / gateways
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}
	client := &http.Client{
		Timeout:   httpTimeout,
		Transport: transport,
	}

	s := &Service{
		mqttManager: mqttManager,
		db:          openEnyaqDB(),
		httpClient:  client,
		done:        make(chan struct{}),
		config: Config{
			PollIntervalMinutes: defaultPollIntervalMin,
			AutoPollEnabled:     true,
		},
		data: VehicleData{
			ModelName:           "Škoda Enyaq",
			LastPollStatus:      "idle",
			ChargingState:       "disconnected",
			PlugConnectionState: "disconnected",
			PlugLockState:       "unlocked",
			LockState:           "locked",
			DoorsClosed:         true,
			WindowsClosed:       true,
			TrunkClosed:         true,
			BonnetClosed:        true,
		},
		history: make([]HistoryEntry, 0),
	}

	// Load persisted data from bbolt
	s.loadPersistedState()

	return s
}

// @brief Loads persisted config, state, and history from bbolt.
func (s *Service) loadPersistedState() {
	if s.db == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.db.View(func(tx *bbolt.Tx) error {
		// 1. Restore Config
		if bConfig := tx.Bucket([]byte(dbBucketConfig)); bConfig != nil {
			if raw := bConfig.Get([]byte(dbKeyConfig)); len(raw) > 0 {
				var cfg Config
				if err := json.Unmarshal(raw, &cfg); err == nil {
					if cfg.PollIntervalMinutes <= 0 {
						cfg.PollIntervalMinutes = defaultPollIntervalMin
					}
					s.config = cfg
					log.Printf("[enyaq-state] restored config (vin=%s, autoPoll=%t, interval=%dm)", cfg.VIN, cfg.AutoPollEnabled, cfg.PollIntervalMinutes)
				}
			}
		}

		// 2. Restore State Data
		if bState := tx.Bucket([]byte(dbBucketState)); bState != nil {
			if raw := bState.Get([]byte(dbKeyState)); len(raw) > 0 {
				var data VehicleData
				if err := json.Unmarshal(raw, &data); err == nil {
					s.data = data
					log.Printf("[enyaq-state] restored vehicle data (battery=%.1f%%, range=%.0fkm)", data.BatteryLevelPct, data.RemainingRangeKm)
				}
			}
		}

		// 3. Restore History
		if bHistory := tx.Bucket([]byte(dbBucketHistory)); bHistory != nil {
			if raw := bHistory.Get([]byte(dbKeyHistory)); len(raw) > 0 {
				var hist []HistoryEntry
				if err := json.Unmarshal(raw, &hist); err == nil {
					s.history = hist
					log.Printf("[enyaq-state] restored %d history entries", len(hist))
				}
			}
		}

		return nil
	})
}

// @brief Persists the current configuration to bbolt.
func (s *Service) persistConfig() {
	if s.db == nil {
		return
	}

	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

	raw, err := json.Marshal(cfg)
	if err != nil {
		log.Printf("[enyaq-state] config marshal error: %v", err)
		return
	}

	_ = s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(dbBucketConfig))
		if b == nil {
			return fmt.Errorf("bucket %s missing", dbBucketConfig)
		}
		return b.Put([]byte(dbKeyConfig), raw)
	})
}

// @brief Persists the current vehicle state and history to bbolt.
func (s *Service) persistStateAndHistory() {
	if s.db == nil {
		return
	}

	s.mu.RLock()
	data := s.data
	hist := append([]HistoryEntry(nil), s.history...)
	s.mu.RUnlock()

	rawData, err := json.Marshal(data)
	if err != nil {
		log.Printf("[enyaq-state] state marshal error: %v", err)
		return
	}

	rawHist, err := json.Marshal(hist)
	if err != nil {
		log.Printf("[enyaq-state] history marshal error: %v", err)
		return
	}

	_ = s.db.Update(func(tx *bbolt.Tx) error {
		if bState := tx.Bucket([]byte(dbBucketState)); bState != nil {
			_ = bState.Put([]byte(dbKeyState), rawData)
		}
		if bHist := tx.Bucket([]byte(dbBucketHistory)); bHist != nil {
			_ = bHist.Put([]byte(dbKeyHistory), rawHist)
		}
		return nil
	})
}

// @brief Publishes all vehicle telemetry properties to dedicated MQTT topics.
func (s *Service) publishToMQTT() {
	if s.mqttManager == nil {
		return
	}

	s.mu.RLock()
	data := s.data
	s.mu.RUnlock()

	// 1. Publish complete status object
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/status", mqttTopicPrefix), data)

	// 2. Publish individual telemetry values for easy dashboard & home automation integration
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/battery_soc", mqttTopicPrefix), data.BatteryLevelPct)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/target_soc", mqttTopicPrefix), data.TargetSoCPct)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/range_km", mqttTopicPrefix), data.RemainingRangeKm)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/charging_state", mqttTopicPrefix), data.ChargingState)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/charge_power_kw", mqttTopicPrefix), data.ChargePowerKw)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/remaining_charging_time_min", mqttTopicPrefix), data.RemainingChargingTimeMin)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/plug_connected", mqttTopicPrefix), strings.EqualFold(data.PlugConnectionState, "connected"))
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/plug_locked", mqttTopicPrefix), strings.EqualFold(data.PlugLockState, "locked"))
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/mileage_km", mqttTopicPrefix), data.MileageKm)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/lock_state", mqttTopicPrefix), data.LockState)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/climatisation_state", mqttTopicPrefix), data.ClimatisationState)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/target_temperature_c", mqttTopicPrefix), data.TargetTemperatureC)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/outdoor_temperature_c", mqttTopicPrefix), data.OutdoorTemperatureC)
	_ = s.mqttManager.Publish(fmt.Sprintf("%s/last_updated", mqttTopicPrefix), data.LastUpdated)

	log.Printf("[enyaq-mqtt] published telemetry (SoC=%.1f%%, Range=%.0fkm, State=%s)", data.BatteryLevelPct, data.RemainingRangeKm, data.ChargingState)
}

// @brief Parses dynamic JSON payload from various vehicle API formats (MyŠkoda, MEB, Smartcar, Tronity, generic).
// @param raw The raw JSON byte slice received from the HTTP response.
// @param target The VehicleData struct to populate.
// @return Error if parsing fails.
func parseVehicleDataPayload(raw []byte, target *VehicleData) error {
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return fmt.Errorf("JSON decoding failed: %w", err)
	}

	target.RawPayload = string(raw)

	// Helper to extract nested numbers or values
	findNumber := func(keys ...string) (float64, bool) {
		for _, k := range keys {
			parts := strings.Split(k, ".")
			var current any = generic
			found := true
			for _, part := range parts {
				if m, ok := current.(map[string]any); ok {
					if val, exists := m[part]; exists {
						current = val
					} else {
						found = false
						break
					}
				} else {
					found = false
					break
				}
			}
			if found && current != nil {
				switch num := current.(type) {
				case float64:
					return num, true
				case int:
					return float64(num), true
				case int64:
					return float64(num), true
				}
			}
		}
		return 0, false
	}

	findString := func(keys ...string) (string, bool) {
		for _, k := range keys {
			parts := strings.Split(k, ".")
			var current any = generic
			found := true
			for _, part := range parts {
				if m, ok := current.(map[string]any); ok {
					if val, exists := m[part]; exists {
						current = val
					} else {
						found = false
						break
					}
				} else {
					found = false
					break
				}
			}
			if found && current != nil {
				if str, ok := current.(string); ok && strings.TrimSpace(str) != "" {
					return strings.TrimSpace(str), true
				}
			}
		}
		return "", false
	}

	findBool := func(keys ...string) (bool, bool) {
		for _, k := range keys {
			parts := strings.Split(k, ".")
			var current any = generic
			found := true
			for _, part := range parts {
				if m, ok := current.(map[string]any); ok {
					if val, exists := m[part]; exists {
						current = val
					} else {
						found = false
						break
					}
				} else {
					found = false
					break
				}
			}
			if found && current != nil {
				if b, ok := current.(bool); ok {
					return b, true
				}
			}
		}
		return false, false
	}

	// 1. Battery & Range
	if val, ok := findNumber(
		"batteryLevelPct",
		"battery_level_pct",
		"vehicle.charging.status.battery.stateOfChargeInPercent",
		"charging.status.battery.stateOfChargeInPercent",
		"charging.batteryStatus.stateOfChargeInPercent",
		"battery.stateOfChargeInPercent",
		"soc",
		"stateOfChargeInPercent",
		"batteryLevel",
	); ok {
		target.BatteryLevelPct = val
	}
	if val, ok := findNumber(
		"targetSoCPct",
		"target_soc_pct",
		"vehicle.charging.settings.targetStateOfChargeInPercent",
		"charging.settings.targetStateOfChargeInPercent",
		"charging.targetStateOfChargeInPercent",
		"targetStateOfChargeInPercent",
		"targetSoc",
	); ok {
		target.TargetSoCPct = val
	}
	if val, ok := findNumber(
		"remainingRangeKm",
		"remaining_range_km",
		"vehicle.charging.status.battery.remainingCruisingRangeInMeters",
		"charging.status.battery.remainingCruisingRangeInMeters",
		"remainingCruisingRangeInMeters",
	); ok {
		// Convert meters to kilometers if value is large (> 1000)
		if val > 1000 {
			target.RemainingRangeKm = val / 1000.0
		} else {
			target.RemainingRangeKm = val
		}
	} else if val, ok := findNumber(
		"vehicle.charging.status.battery.remainingRangeInKilometers",
		"battery.remainingRangeInKilometers",
		"charging.batteryStatus.remainingRangeInKilometers",
		"range",
		"cruisingRangeElectricKm",
		"remainingRange",
	); ok {
		target.RemainingRangeKm = val
	}

	// 2. Charging Details
	if val, ok := findString(
		"chargingState",
		"charging_state",
		"vehicle.charging.status.state",
		"charging.status.state",
		"charging.chargingStatus.chargingState",
		"status.chargingState",
	); ok {
		switch strings.ToUpper(val) {
		case "CHARGING":
			target.ChargingState = "charging"
		case "READY_FOR_CHARGING", "CONSERVING":
			target.ChargingState = "idle"
		case "CONNECT_CABLE":
			target.ChargingState = "connected"
		case "DISCHARGING":
			target.ChargingState = "discharging"
		default:
			target.ChargingState = strings.ToLower(val)
		}
	}
	if val, ok := findNumber(
		"chargePowerKw",
		"charge_power_kw",
		"vehicle.charging.status.chargePowerInKw",
		"charging.status.chargePowerInKw",
		"charging.chargingStatus.chargePowerInKw",
		"chargePower",
		"powerKw",
	); ok {
		target.ChargePowerKw = val
	}
	if val, ok := findNumber(
		"chargeRateKmPerHour",
		"charge_rate_km_per_h",
		"vehicle.charging.status.chargingRateInKilometersPerHour",
		"charging.status.chargingRateInKilometersPerHour",
		"charging.chargingStatus.chargeRateInKilometersPerHour",
		"chargeRate",
	); ok {
		target.ChargeRateKmPerHour = val
	}
	if val, ok := findNumber(
		"remainingChargingTimeMin",
		"remaining_charging_time_min",
		"vehicle.charging.status.remainingTimeToFullyChargedInMinutes",
		"charging.status.remainingTimeToFullyChargedInMinutes",
		"charging.chargingStatus.remainingChargingTimeToCompleteInMinutes",
		"remainingChargingTimeInMinutes",
	); ok {
		target.RemainingChargingTimeMin = int(val)
	}
	if val, ok := findString(
		"chargingMode",
		"charging_mode",
		"vehicle.charging.status.chargeType",
		"charging.status.chargeType",
		"charging.chargingStatus.chargingMode",
	); ok {
		target.ChargingMode = strings.ToLower(val)
	}
	if val, ok := findString(
		"plugConnectionState",
		"plug_connection_state",
		"charging.plugStatus.plugConnectionState",
	); ok {
		target.PlugConnectionState = val
	} else if target.ChargingState == "charging" || target.ChargingState == "connected" {
		target.PlugConnectionState = "connected"
	}
	if val, ok := findString(
		"plugLockState",
		"plug_lock_state",
		"charging.plugStatus.plugLockState",
	); ok {
		target.PlugLockState = val
	} else if target.ChargingState == "charging" {
		target.PlugLockState = "locked"
	}

	// 3. Mileage & Vehicle Status
	if val, ok := findNumber(
		"mileageKm",
		"mileage_km",
		"vehicle.odometer.mileageInKm",
		"odometer.mileageInKm",
		"odometerInKilometers",
		"mileageInKm",
		"mileage",
		"odometer",
	); ok {
		target.MileageKm = int(val)
	}
	if val, ok := findString(
		"lockState",
		"lock_state",
		"vehicle.status.overall.doorsLocked",
		"status.overall.doorsLocked",
		"vehicle.status.overall.locked",
		"access.accessStatus.overallStatus",
		"lockStatus",
	); ok {
		if strings.EqualFold(val, "YES") || strings.EqualFold(val, "LOCKED") {
			target.LockState = "locked"
		} else if strings.EqualFold(val, "NO") || strings.EqualFold(val, "UNLOCKED") {
			target.LockState = "unlocked"
		} else {
			target.LockState = strings.ToLower(val)
		}
	}
	if val, ok := findString("vehicle.status.overall.doors", "status.overall.doors"); ok {
		target.DoorsClosed = strings.EqualFold(val, "CLOSED")
	} else if val, ok := findBool("doorsClosed", "doors_closed", "access.accessStatus.doorsClosed"); ok {
		target.DoorsClosed = val
	}
	if val, ok := findString("vehicle.status.overall.windows", "status.overall.windows"); ok {
		target.WindowsClosed = strings.EqualFold(val, "CLOSED")
	} else if val, ok := findBool("windowsClosed", "windows_closed", "access.accessStatus.windowsClosed"); ok {
		target.WindowsClosed = val
	}
	if val, ok := findBool("trunkClosed", "trunk_closed", "access.accessStatus.trunkClosed"); ok {
		target.TrunkClosed = val
	}
	if val, ok := findBool("bonnetClosed", "bonnet_closed", "access.accessStatus.bonnetClosed"); ok {
		target.BonnetClosed = val
	}
	if val, ok := findString("vehicle.status.overall.lights", "status.overall.lights"); ok {
		target.LightsOn = strings.EqualFold(val, "ON")
	} else if val, ok := findBool("lightsOn", "lights_on", "lights.lightsStatus.lightsOn"); ok {
		target.LightsOn = val
	}

	// 4. Climatisation
	if val, ok := findNumber(
		"targetTemperatureC",
		"target_temperature_c",
		"vehicle.airConditioning.targetTemperature.value",
		"airConditioning.targetTemperature.value",
		"climatisation.targetTemperatureInCelsius",
		"targetTemperature",
	); ok {
		target.TargetTemperatureC = val
	}
	if val, ok := findString(
		"climatisationState",
		"climatisation_state",
		"vehicle.airConditioning.state",
		"airConditioning.state",
		"climatisation.climatisationStatus.climatisationState",
	); ok {
		target.ClimatisationState = strings.ToLower(val)
	}
	if val, ok := findString("vehicle.airConditioning.windowHeating.front", "airConditioning.windowHeating.front"); ok {
		target.WindowHeatingFront = strings.EqualFold(val, "ON")
	} else if val, ok := findBool("windowHeatingFront", "window_heating_front", "climatisation.windowHeatingStatus.front"); ok {
		target.WindowHeatingFront = val
	}
	if val, ok := findString("vehicle.airConditioning.windowHeating.rear", "airConditioning.windowHeating.rear"); ok {
		target.WindowHeatingRear = strings.EqualFold(val, "ON")
	} else if val, ok := findBool("windowHeatingRear", "window_heating_rear", "climatisation.windowHeatingStatus.rear"); ok {
		target.WindowHeatingRear = val
	}
	if val, ok := findNumber(
		"outdoorTemperatureC",
		"outdoor_temperature_c",
		"climatisation.outdoorTemperatureInCelsius",
		"outsideTemperature",
	); ok {
		target.OutdoorTemperatureC = val
	}

	// 5. Model
	if val, ok := findString("vehicle.name", "modelName", "model_name", "model", "vehicleName"); ok {
		target.ModelName = val
	}

	return nil
}

// @brief Performs a telemetry fetch from the configured vehicle API using HTTP and Bearer token.
// @return Error if the fetch fails.
func (s *Service) FetchVehicleData() error {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

	nowStr := time.Now().UTC().Format(time.RFC3339)

	if strings.TrimSpace(cfg.APIToken) == "" && strings.TrimSpace(cfg.APIURL) == "" {
		s.mu.Lock()
		s.data.LastPollStatus = "no_token"
		s.data.LastError = "Kein API-Token oder API-URL konfiguriert. Bitte im Token-Fenster hinterlegen."
		s.lastPollAt = time.Now()
		s.nextPollAt = time.Now().Add(time.Duration(cfg.PollIntervalMinutes) * time.Minute)
		s.mu.Unlock()
		s.persistStateAndHistory()
		return fmt.Errorf("no api token or url configured")
	}

	targetURL := strings.TrimSpace(cfg.APIURL)
	if targetURL == "" {
		// Default to official MyŠkoda Public API endpoint (https://public.api.connect.skoda-auto.cz/docs/swagger-ui/index.html)
		if cfg.VIN != "" {
			targetURL = fmt.Sprintf("https://public.api.connect.skoda-auto.cz/api/v1/vehicles/%s", cfg.VIN)
		} else {
			targetURL = "https://public.api.connect.skoda-auto.cz/api/v1/vehicles"
		}
	}

	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		s.recordPollError(fmt.Sprintf("Request-Erstellung fehlgeschlagen: %v", err))
		return err
	}

	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(cfg.APIToken); token != "" {
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		} else {
			req.Header.Set("Authorization", token)
		}
	}
	if cfg.VIN != "" {
		req.Header.Set("X-VIN", cfg.VIN)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.recordPollError(fmt.Sprintf("HTTP-Anfrage fehlgeschlagen: %v", err))
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.recordPollError(fmt.Sprintf("Lesen der Antwort fehlgeschlagen: %v", err))
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := fmt.Sprintf("API meldet HTTP-Status %d: %s", resp.StatusCode, string(bodyBytes))
		s.recordPollError(errMsg)
		return fmt.Errorf(errMsg)
	}

	s.mu.Lock()
	updatedData := s.data
	if err := parseVehicleDataPayload(bodyBytes, &updatedData); err != nil {
		s.mu.Unlock()
		s.recordPollError(fmt.Sprintf("Payload-Parsing fehlgeschlagen: %v", err))
		return err
	}

	updatedData.LastUpdated = nowStr
	updatedData.LastPollStatus = "ok"
	updatedData.LastError = ""
	s.data = updatedData
	s.lastPollAt = time.Now()
	s.nextPollAt = time.Now().Add(time.Duration(cfg.PollIntervalMinutes) * time.Minute)

	// Append to history
	histEntry := HistoryEntry{
		Timestamp:        nowStr,
		BatteryLevelPct:  updatedData.BatteryLevelPct,
		RemainingRangeKm: updatedData.RemainingRangeKm,
		ChargingState:    updatedData.ChargingState,
		ChargePowerKw:    updatedData.ChargePowerKw,
		MileageKm:        updatedData.MileageKm,
		LockState:        updatedData.LockState,
	}
	s.history = append(s.history, histEntry)
	if len(s.history) > maxHistoryEntries {
		s.history = s.history[len(s.history)-maxHistoryEntries:]
	}
	s.mu.Unlock()

	s.persistStateAndHistory()
	s.publishToMQTT()

	log.Printf("[enyaq] telemetry updated successfully from %s", targetURL)
	return nil
}

// @brief Records a poll error, persists status, and publishes error state to MQTT.
// @param msg Error message text.
func (s *Service) recordPollError(msg string) {
	log.Printf("[enyaq-poll] error: %s", msg)
	s.mu.Lock()
	s.data.LastPollStatus = "error"
	s.data.LastError = msg
	s.lastPollAt = time.Now()
	cfg := s.config
	s.nextPollAt = time.Now().Add(time.Duration(cfg.PollIntervalMinutes) * time.Minute)
	s.mu.Unlock()

	s.persistStateAndHistory()

	if s.mqttManager != nil {
		_ = s.mqttManager.Publish(fmt.Sprintf("%s/last_error", mqttTopicPrefix), msg)
		_ = s.mqttManager.Publish(fmt.Sprintf("%s/status_state", mqttTopicPrefix), "error")
	}
}

// @brief Updates vehicle telemetry manually or via simulation/direct payload.
// @param data New VehicleData to set.
func (s *Service) SetVehicleData(data VehicleData) {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	data.LastUpdated = nowStr
	data.LastPollStatus = "ok"
	data.LastError = ""
	s.data = data
	s.lastPollAt = time.Now()

	histEntry := HistoryEntry{
		Timestamp:        nowStr,
		BatteryLevelPct:  data.BatteryLevelPct,
		RemainingRangeKm: data.RemainingRangeKm,
		ChargingState:    data.ChargingState,
		ChargePowerKw:    data.ChargePowerKw,
		MileageKm:        data.MileageKm,
		LockState:        data.LockState,
	}
	s.history = append(s.history, histEntry)
	if len(s.history) > maxHistoryEntries {
		s.history = s.history[len(s.history)-maxHistoryEntries:]
	}
	s.mu.Unlock()

	s.persistStateAndHistory()
	s.publishToMQTT()
}

// @brief Background runLoop that performs periodic telemetry polling every 10 minutes.
func (s *Service) runLoop() {
	defer func() {
		close(s.done)
		log.Println("🛑 Škoda Enyaq service stopped")
	}()

	log.Printf("🚗 Škoda Enyaq background poller started (interval: %d min)", s.config.PollIntervalMinutes)

	// Trigger an initial fetch after a short startup delay (5 seconds)
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(5 * time.Second):
		s.mu.RLock()
		autoPoll := s.config.AutoPollEnabled
		hasToken := strings.TrimSpace(s.config.APIToken) != "" || strings.TrimSpace(s.config.APIURL) != ""
		s.mu.RUnlock()

		if autoPoll && hasToken {
			log.Println("[enyaq] executing initial telemetry poll...")
			_ = s.FetchVehicleData()
		} else {
			log.Println("[enyaq] initial poll skipped (no token configured or auto-poll disabled)")
		}
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			autoPoll := s.config.AutoPollEnabled
			interval := s.config.PollIntervalMinutes
			if interval <= 0 {
				interval = defaultPollIntervalMin
			}
			lastPoll := s.lastPollAt
			hasCredentials := strings.TrimSpace(s.config.APIToken) != "" || strings.TrimSpace(s.config.APIURL) != ""
			s.mu.RUnlock()

			if !autoPoll || !hasCredentials {
				continue
			}

			// Check if interval has elapsed since last poll
			if lastPoll.IsZero() || time.Since(lastPoll) >= time.Duration(interval)*time.Minute {
				log.Printf("[enyaq] scheduled 10-minute poll triggered (interval: %d min)", interval)
				_ = s.FetchVehicleData()
			}
		}
	}
}

// @brief Starts the background polling loop.
func (s *Service) Start() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.runLoop()
}

// @brief Gracefully stops the service and closes DB.
func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
		<-s.done
	}

	if s.db != nil {
		_ = s.db.Close()
	}
}

// @brief Writes a JSON response helper.
func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// @brief Writes a JSON error helper.
func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

// @brief Central HTTP request router for /api/enyaq/* endpoints.
//
// Supported routes:
// - GET  /api/enyaq/status: full status, telemetry data, config overview, and history
// - GET  /api/enyaq/config: configuration details with token masked
// - PUT  /api/enyaq/config: update token, vin, api url, poll interval, auto-poll
// - POST /api/enyaq/fetch: trigger immediate manual poll
// - PUT  /api/enyaq/data: directly update/simulate vehicle telemetry
//
// @param w HTTP response writer.
// @param r HTTP request.
func (s *Service) APIHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/enyaq")
	path = strings.TrimPrefix(path, "/")

	switch path {
	case "status", "":
		if r.Method != http.MethodGet {
			jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		s.handleGetStatus(w, r)

	case "config":
		if r.Method == http.MethodGet {
			s.handleGetConfig(w, r)
		} else if r.Method == http.MethodPut || r.Method == http.MethodPost {
			s.handleUpdateConfig(w, r)
		} else {
			jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}

	case "fetch":
		if r.Method != http.MethodPost {
			jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		s.handleManualFetch(w, r)

	case "data":
		if r.Method == http.MethodPut || r.Method == http.MethodPost {
			s.handleSetData(w, r)
		} else {
			jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}

	default:
		jsonError(w, http.StatusNotFound, "Endpoint not found")
	}
}

// @brief Handles GET /api/enyaq/status.
func (s *Service) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.config
	data := s.data
	hist := append([]HistoryEntry(nil), s.history...)
	nextPoll := s.nextPollAt
	s.mu.RUnlock()

	hasToken := strings.TrimSpace(cfg.APIToken) != ""
	preview := ""
	if hasToken {
		trimmed := strings.TrimSpace(cfg.APIToken)
		if len(trimmed) > 8 {
			preview = fmt.Sprintf("%s...%s", trimmed[:4], trimmed[len(trimmed)-4:])
		} else {
			preview = "••••••••"
		}
	}

	nextSec := 0
	if !nextPoll.IsZero() && nextPoll.After(time.Now()) {
		nextSec = int(time.Until(nextPoll).Seconds())
	}

	resp := StatusResponse{
		Config: ConfigResponse{
			HasToken:            hasToken,
			TokenPreview:        preview,
			VIN:                 cfg.VIN,
			APIURL:              cfg.APIURL,
			PollIntervalMinutes: cfg.PollIntervalMinutes,
			AutoPollEnabled:     cfg.AutoPollEnabled,
			LastConfigUpdate:    cfg.LastConfigUpdate,
		},
		Data:            data,
		History:         hist,
		ServerTime:      time.Now().UTC().Format(time.RFC3339),
		NextPollInSec:   nextSec,
		PollIntervalMin: cfg.PollIntervalMinutes,
	}

	jsonResponse(w, resp)
}

// @brief Handles GET /api/enyaq/config.
func (s *Service) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

	hasToken := strings.TrimSpace(cfg.APIToken) != ""
	preview := ""
	if hasToken {
		trimmed := strings.TrimSpace(cfg.APIToken)
		if len(trimmed) > 8 {
			preview = fmt.Sprintf("%s...%s", trimmed[:4], trimmed[len(trimmed)-4:])
		} else {
			preview = "••••••••"
		}
	}

	resp := ConfigResponse{
		HasToken:            hasToken,
		TokenPreview:        preview,
		VIN:                 cfg.VIN,
		APIURL:              cfg.APIURL,
		PollIntervalMinutes: cfg.PollIntervalMinutes,
		AutoPollEnabled:     cfg.AutoPollEnabled,
		LastConfigUpdate:    cfg.LastConfigUpdate,
	}

	jsonResponse(w, resp)
}

// @brief Request body structure for updating configuration.
type updateConfigRequest struct {
	APIToken            *string `json:"apiToken,omitempty"`
	VIN                 *string `json:"vin,omitempty"`
	APIURL              *string `json:"apiUrl,omitempty"`
	PollIntervalMinutes *int    `json:"pollIntervalMinutes,omitempty"`
	AutoPollEnabled     *bool   `json:"autoPollEnabled,omitempty"`
}

// @brief Handles PUT /api/enyaq/config.
func (s *Service) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req updateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Ungültiger JSON-Body")
		return
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	s.mu.Lock()
	if req.APIToken != nil {
		s.config.APIToken = strings.TrimSpace(*req.APIToken)
	}
	if req.VIN != nil {
		s.config.VIN = strings.TrimSpace(*req.VIN)
	}
	if req.APIURL != nil {
		s.config.APIURL = strings.TrimSpace(*req.APIURL)
	}
	if req.PollIntervalMinutes != nil {
		interval := *req.PollIntervalMinutes
		if interval < 1 {
			interval = 1
		} else if interval > 1440 {
			interval = 1440
		}
		s.config.PollIntervalMinutes = interval
	}
	if req.AutoPollEnabled != nil {
		s.config.AutoPollEnabled = *req.AutoPollEnabled
	}
	s.config.LastConfigUpdate = nowStr
	s.nextPollAt = time.Now().Add(time.Duration(s.config.PollIntervalMinutes) * time.Minute)
	s.mu.Unlock()

	s.persistConfig()

	log.Println("[enyaq-config] configuration updated and saved to database")
	jsonResponse(w, map[string]any{"ok": true, "message": "Konfiguration erfolgreich gespeichert"})
}

// @brief Handles POST /api/enyaq/fetch.
func (s *Service) handleManualFetch(w http.ResponseWriter, r *http.Request) {
	log.Println("[enyaq] manual telemetry fetch requested via API")
	if err := s.FetchVehicleData(); err != nil {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Abfrage fehlgeschlagen: %v", err))
		return
	}

	s.mu.RLock()
	data := s.data
	s.mu.RUnlock()

	jsonResponse(w, map[string]any{
		"ok":      true,
		"message": "Fahrzeugdaten erfolgreich aktualisiert",
		"data":    data,
	})
}

// @brief Handles PUT/POST /api/enyaq/data (manual override / mock / test data).
func (s *Service) handleSetData(w http.ResponseWriter, r *http.Request) {
	var data VehicleData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonError(w, http.StatusBadRequest, "Ungültiger JSON-Body")
		return
	}

	s.SetVehicleData(data)
	jsonResponse(w, map[string]any{
		"ok":      true,
		"message": "Fahrzeugdaten erfolgreich manuell gesetzt",
		"data":    data,
	})
}
