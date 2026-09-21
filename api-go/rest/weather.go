package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

const (
	weatherDefaultStationID         = "IRIEDE66"
	weatherDefaultInterval          = 300 * time.Second
	weatherDefaultRequestsPerMinute = 1
	weatherMaxRequestsPerMinute     = 5
	weatherDBPath                   = "data/weather.db"
	weatherBucket                   = "weather"
	weatherSettingsKey              = "settings"
	weatherDataKey                  = "data"
	weatherAPIEndpoint              = "https://api.weather.com/v2/pws/observations/current"
)

// WeatherSettings contains all user-configurable Weather Underground values.
type WeatherSettings struct {
	StationID            string `json:"stationId"`
	APIKey               string `json:"apiKey"`
	Units                string `json:"units"`
	IntervalSeconds      int    `json:"intervalSeconds"`
	MaxRequestsPerMinute int    `json:"maxRequestsPerMinute"`
}

// WeatherObservation is the normalized current observation exposed to the web UI.
type WeatherObservation struct {
	StationID       string  `json:"stationId"`
	StationName     string  `json:"stationName"`
	ObservedAt      string  `json:"observedAt"`
	Temperature     float64 `json:"temperature"`
	FeelsLike       float64 `json:"feelsLike"`
	Humidity        float64 `json:"humidity"`
	WindSpeed       float64 `json:"windSpeed"`
	WindGust        float64 `json:"windGust"`
	WindDirection   string  `json:"windDirection"`
	Pressure        float64 `json:"pressure"`
	DewPoint        float64 `json:"dewPoint"`
	PrecipRate      float64 `json:"precipRate"`
	PrecipTotal     float64 `json:"precipTotal"`
	UV              float64 `json:"uv"`
	SolarRadiation  float64 `json:"solarRadiation"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Elevation       float64 `json:"elevation"`
	TemperatureUnit string  `json:"temperatureUnit"`
	WindUnit        string  `json:"windUnit"`
	PressureUnit    string  `json:"pressureUnit"`
	PrecipUnit      string  `json:"precipUnit"`
	UpdatedAt       string  `json:"updatedAt"`
}

// WeatherResponse is returned by the weather status endpoint.
type WeatherResponse struct {
	Settings    WeatherSettings     `json:"settings"`
	Observation *WeatherObservation `json:"observation,omitempty"`
	LastFetchAt string              `json:"lastFetchAt,omitempty"`
	Error       string              `json:"error,omitempty"`
	Configured  bool                `json:"configured"`
}

// WeatherService owns Weather Underground polling, caching, persistence, and HTTP handlers.
type WeatherService struct {
	mu          sync.RWMutex
	settings    WeatherSettings
	observation *WeatherObservation
	lastFetchAt time.Time
	lastError   string
	db          *bbolt.DB
	client      *http.Client
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	fetchMu     sync.Mutex
	lastRequest time.Time
}

type weatherAPIResponse struct {
	Observations []struct {
		StationID       string          `json:"stationID"`
		StationName     string          `json:"neighborhood"`
		ObsTimeUTC      string          `json:"obsTimeUtc"`
		Metric          weatherMetric   `json:"metric"`
		Imperial        weatherImperial `json:"imperial"`
		Humidity        float64         `json:"humidity"`
		Winddir         float64         `json:"winddir"`
		WinddirCardinal string          `json:"winddirCardinal"`
		Pressure        float64         `json:"pressure"`
		UV              float64         `json:"uv"`
		SolarRadiation  float64         `json:"solarRadiation"`
		Lat             float64         `json:"lat"`
		Lon             float64         `json:"lon"`
		Elevation       float64         `json:"elev"`
	} `json:"observations"`
}

type weatherMetric struct {
	Temp        float64 `json:"temp"`
	FeelsLike   float64 `json:"feelsLike"`
	WindSpeed   float64 `json:"windSpeed"`
	WindGust    float64 `json:"windGust"`
	Pressure    float64 `json:"pressure"`
	Dewpt       float64 `json:"dewpt"`
	PrecipRate  float64 `json:"precipRate"`
	PrecipTotal float64 `json:"precipTotal"`
}

type weatherImperial struct {
	Temp        float64 `json:"temp"`
	FeelsLike   float64 `json:"feelsLike"`
	WindSpeed   float64 `json:"windSpeed"`
	WindGust    float64 `json:"windGust"`
	Pressure    float64 `json:"pressure"`
	Dewpt       float64 `json:"dewpt"`
	PrecipRate  float64 `json:"precipRate"`
	PrecipTotal float64 `json:"precipTotal"`
}

// @brief Opens the WeatherService database and restores persisted state.
// @details
// The constructor creates the database directory when necessary, opens the
// bbolt database, initializes safe defaults and the HTTP client, and restores
// settings plus the last cached observation. Any load failure closes the newly
// opened database before returning so callers do not inherit a leaked resource.
// @param[in] dbPath Path to the bbolt database; an empty path uses the default.
// @return Initialized WeatherService, or an error when directory, database, or state initialization fails.
func NewWeatherService(dbPath string) (*WeatherService, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = weatherDBPath
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create weather database directory: %w", err)
	}
	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("open weather database: %w", err)
	}
	service := &WeatherService{
		settings: WeatherSettings{StationID: weatherDefaultStationID, Units: "metric", IntervalSeconds: int(weatherDefaultInterval.Seconds()), MaxRequestsPerMinute: weatherDefaultRequestsPerMinute},
		db:       db,
		client:   &http.Client{Timeout: 15 * time.Second},
		done:     make(chan struct{}),
	}
	if err := service.load(); err != nil {
		db.Close()
		return nil, err
	}
	return service, nil
}

// @brief Restores weather settings and the last successful observation from bbolt.
// @details
// Missing buckets or keys are treated as an empty first-run database. Present
// JSON values are decoded and settings are normalized before the read
// transaction completes, ensuring subsequent polling sees bounded values.
// @return Error when persisted settings cannot be decoded or the database view fails.
func (s *WeatherService) load() error {
	return s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(weatherBucket))
		if bucket == nil {
			return nil
		}
		if raw := bucket.Get([]byte(weatherSettingsKey)); raw != nil {
			if err := json.Unmarshal(raw, &s.settings); err != nil {
				return fmt.Errorf("decode weather settings: %w", err)
			}
		}
		if raw := bucket.Get([]byte(weatherDataKey)); raw != nil {
			var observation WeatherObservation
			if err := json.Unmarshal(raw, &observation); err == nil {
				s.observation = &observation
			}
		}
		s.normalizeSettings()
		return nil
	})
}

// @brief Persists settings and the latest observation atomically in bbolt.
// @details
// A read-locked snapshot is taken before opening the write transaction, which
// prevents the database transaction from holding the service mutex while JSON
// encoding occurs. The settings are always written; cached observation data is
// written when available.
// @return Error when serialization or the bbolt write transaction fails.
func (s *WeatherService) persist() error {
	s.mu.RLock()
	settings, observation := s.settings, s.observation
	s.mu.RUnlock()
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(weatherBucket))
		if err != nil {
			return err
		}
		settingsRaw, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		if err := bucket.Put([]byte(weatherSettingsKey), settingsRaw); err != nil {
			return err
		}
		if observation != nil {
			dataRaw, err := json.Marshal(observation)
			if err != nil {
				return err
			}
			return bucket.Put([]byte(weatherDataKey), dataRaw)
		}
		return nil
	})
}

// @brief Applies safe defaults and bounds to user-provided weather settings.
// @details
// Station identifiers are trimmed and uppercased, units are restricted to the
// supported values, polling intervals are bounded, and request rates are capped
// at the provider-safe maximum. The caller must hold the service write lock when
// invoking this method because it mutates s.settings.
func (s *WeatherService) normalizeSettings() {
	if strings.TrimSpace(s.settings.StationID) == "" {
		s.settings.StationID = weatherDefaultStationID
	}
	s.settings.StationID = strings.ToUpper(strings.TrimSpace(s.settings.StationID))
	if s.settings.Units != "imperial" {
		s.settings.Units = "metric"
	}
	if s.settings.IntervalSeconds < 30 {
		s.settings.IntervalSeconds = 30
	}
	if s.settings.IntervalSeconds > 3600 {
		s.settings.IntervalSeconds = 3600
	}
	if s.settings.MaxRequestsPerMinute < 1 {
		s.settings.MaxRequestsPerMinute = 1
	}
	if s.settings.MaxRequestsPerMinute > weatherMaxRequestsPerMinute {
		s.settings.MaxRequestsPerMinute = weatherMaxRequestsPerMinute
	}
}

// @brief Calculates the minimum delay between outbound weather requests.
// @details
// The calculation converts the configured requests-per-minute limit into a
// whole-second spacing and applies a defensive lower bound of one request per
// minute when invalid state is encountered.
// @return Minimum duration that must separate two provider requests.
func (s *WeatherService) minimumRequestInterval() time.Duration {
	s.mu.RLock()
	requestsPerMinute := s.settings.MaxRequestsPerMinute
	s.mu.RUnlock()
	if requestsPerMinute < 1 {
		requestsPerMinute = 1
	}
	return time.Duration((60+requestsPerMinute-1)/requestsPerMinute) * time.Second
}

// @brief Starts Weather Underground polling in a background goroutine.
// @details
// The method creates a dedicated cancellation context and immediately returns;
// the first refresh and all later scheduled refreshes run in runLoop.
func (s *WeatherService) Start() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.runLoop()
}

// @brief Refreshes the observation immediately and at the configured interval.
// @details
// The effective interval is never shorter than the request-rate spacing. A
// cancellation signal stops the timer and exits without starting another
// provider request.
func (s *WeatherService) runLoop() {
	defer close(s.done)
	s.refresh()
	for {
		s.mu.RLock()
		interval := time.Duration(s.settings.IntervalSeconds) * time.Second
		s.mu.RUnlock()
		if minimum := s.minimumRequestInterval(); interval < minimum {
			interval = minimum
		}
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			s.refresh()
		case <-s.ctx.Done():
			timer.Stop()
			return
		}
	}
}

// @brief Terminates polling and closes the weather database.
// @details
// If polling is active, cancellation is requested and the method waits for the
// loop to finish before closing bbolt. This ordering prevents background work
// from accessing a closed database.
func (s *WeatherService) Stop() {
	if s.cancel != nil {
		s.cancel()
		<-s.done
	}
	if s.db != nil {
		_ = s.db.Close()
	}
}

// @brief Fetches the configured station and updates cached observation state.
// @details
// Requests are serialized across scheduled and manual refreshes. The method
// enforces the configured provider spacing, rejects missing credentials, calls
// the Weather Underground endpoint, normalizes the first observation, updates
// the in-memory cache, and persists successful results. Errors remain visible
// through status while previously cached data is retained.
func (s *WeatherService) refresh() {
	// Serialize automatic, manual, and settings-triggered requests and enforce
	// the provider limit immediately before every outbound API call.
	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()
	wait := s.minimumRequestInterval() - time.Since(s.lastRequest)
	if !s.lastRequest.IsZero() && wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-s.ctx.Done():
			timer.Stop()
			return
		}
	}
	s.lastRequest = time.Now()
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	if strings.TrimSpace(settings.APIKey) == "" {
		s.mu.Lock()
		s.lastError = "Kein Weather-Underground-API-Key konfiguriert"
		s.mu.Unlock()
		return
	}
	query := url.Values{}
	query.Set("stationId", settings.StationID)
	query.Set("format", "json")
	query.Set("units", settings.Units)
	query.Set("numericPrecision", "decimal")
	query.Set("apiKey", settings.APIKey)
	request, err := http.NewRequestWithContext(s.ctx, http.MethodGet, weatherAPIEndpoint+"?"+query.Encode(), nil)
	if err != nil {
		s.recordError(err)
		return
	}
	response, err := s.client.Do(request)
	if err != nil {
		s.recordError(err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.recordError(fmt.Errorf("Weather Underground antwortet mit HTTP %d", response.StatusCode))
		return
	}
	var payload weatherAPIResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		s.recordError(err)
		return
	}
	if len(payload.Observations) == 0 {
		s.recordError(fmt.Errorf("keine Messung für Station %s", settings.StationID))
		return
	}
	observation := normalizeObservation(payload.Observations[0], settings.Units)
	now := time.Now().UTC()
	observation.UpdatedAt = now.Format(time.RFC3339)
	s.mu.Lock()
	s.observation = &observation
	s.lastFetchAt = now
	s.lastError = ""
	s.mu.Unlock()
	if err := s.persist(); err != nil {
		log.Printf("[weather] persist failed: %v", err)
	}
}

// @brief Records the latest refresh error while retaining cached data.
// @details
// The error is stored under the service mutex and logged for operational
// visibility. No cached observation is cleared, allowing the status endpoint to
// continue serving the last successful measurement during outages.
// @param[in] err Error produced by the failed refresh operation.
func (s *WeatherService) recordError(err error) {
	s.mu.Lock()
	s.lastError = err.Error()
	s.mu.Unlock()
	log.Printf("[weather] refresh failed: %v", err)
}

// @brief Maps a vendor observation into the stable frontend contract.
// @details
// Metric or imperial values are selected according to units, while station
// metadata and provider timestamps are copied into the normalized model. The
// function contains no shared-state mutation and is therefore safe to use from
// the refresh path after decoding a response.
// @param[in] raw Vendor observation containing metric and imperial measurements.
// @param[in] units Requested output unit system; imperial selects imperial values.
// @return Normalized observation suitable for persistence and API responses.
func normalizeObservation(raw struct {
	StationID       string          `json:"stationID"`
	StationName     string          `json:"neighborhood"`
	ObsTimeUTC      string          `json:"obsTimeUtc"`
	Metric          weatherMetric   `json:"metric"`
	Imperial        weatherImperial `json:"imperial"`
	Humidity        float64         `json:"humidity"`
	Winddir         float64         `json:"winddir"`
	WinddirCardinal string          `json:"winddirCardinal"`
	Pressure        float64         `json:"pressure"`
	UV              float64         `json:"uv"`
	SolarRadiation  float64         `json:"solarRadiation"`
	Lat             float64         `json:"lat"`
	Lon             float64         `json:"lon"`
	Elevation       float64         `json:"elev"`
}, units string) WeatherObservation {
	values := struct {
		Temp        float64
		FeelsLike   float64
		WindSpeed   float64
		WindGust    float64
		Pressure    float64
		Dewpt       float64
		PrecipRate  float64
		PrecipTotal float64
	}{
		Temp: raw.Metric.Temp, FeelsLike: raw.Metric.FeelsLike, WindSpeed: raw.Metric.WindSpeed,
		WindGust: raw.Metric.WindGust, Pressure: raw.Metric.Pressure, Dewpt: raw.Metric.Dewpt,
		PrecipRate: raw.Metric.PrecipRate, PrecipTotal: raw.Metric.PrecipTotal,
	}
	if units == "imperial" {
		values.Temp = raw.Imperial.Temp
		values.FeelsLike = raw.Imperial.FeelsLike
		values.WindSpeed = raw.Imperial.WindSpeed
		values.WindGust = raw.Imperial.WindGust
		values.Pressure = raw.Imperial.Pressure
		values.Dewpt = raw.Imperial.Dewpt
		values.PrecipRate = raw.Imperial.PrecipRate
		values.PrecipTotal = raw.Imperial.PrecipTotal
	}
	return WeatherObservation{
		StationID: raw.StationID, StationName: raw.StationName, ObservedAt: raw.ObsTimeUTC,
		Temperature: values.Temp, FeelsLike: values.FeelsLike, Humidity: raw.Humidity,
		WindSpeed: values.WindSpeed, WindGust: values.WindGust, WindDirection: raw.WinddirCardinal,
		Pressure: values.Pressure, DewPoint: values.Dewpt, PrecipRate: values.PrecipRate,
		PrecipTotal: values.PrecipTotal, UV: raw.UV, SolarRadiation: raw.SolarRadiation,
		Latitude: raw.Lat, Longitude: raw.Lon, Elevation: raw.Elevation,
		TemperatureUnit: map[bool]string{true: "°F", false: "°C"}[units == "imperial"],
		WindUnit:        map[bool]string{true: "mph", false: "km/h"}[units == "imperial"],
		PressureUnit:    map[bool]string{true: "inHg", false: "hPa"}[units == "imperial"],
		PrecipUnit:      map[bool]string{true: "in", false: "mm"}[units == "imperial"],
	}
}

// @brief Returns a thread-safe copy of the active weather settings.
// @details
// The read lock protects the settings snapshot from concurrent PUT requests and
// background polling while ensuring callers cannot mutate service state through
// the returned value.
// @return Current normalized weather settings.
func (s *WeatherService) GetSettings() WeatherSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// @brief Validates, stores, and persists new weather settings.
// @details
// An omitted API key preserves the existing key so UI updates to unrelated
// settings do not accidentally disable polling. The settings are normalized
// under the write lock and then persisted through bbolt.
// @param[in] settings New user-provided weather settings.
// @return Error when the normalized settings cannot be persisted.
func (s *WeatherService) SetSettings(settings WeatherSettings) error {
	s.mu.Lock()
	if strings.TrimSpace(settings.APIKey) == "" {
		settings.APIKey = s.settings.APIKey
	}
	s.settings = settings
	s.normalizeSettings()
	s.mu.Unlock()
	return s.persist()
}

// @brief Returns a consistent status snapshot for the weather API.
// @details
// The method copies the cached observation before releasing the read lock, so
// callers receive stable data even while a refresh updates the service. The
// response includes settings, configuration state, last fetch time, and the
// latest error without exposing mutable internal pointers.
// @return Consistent weather settings, observation, fetch timestamp, and error state.
func (s *WeatherService) GetStatus() WeatherResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var observation *WeatherObservation
	if s.observation != nil {
		copy := *s.observation
		observation = &copy
	}
	response := WeatherResponse{
		Settings:    s.settings,
		Observation: observation,
		Configured:  strings.TrimSpace(s.settings.APIKey) != "",
		Error:       s.lastError,
	}
	if !s.lastFetchAt.IsZero() {
		response.LastFetchAt = s.lastFetchAt.Format(time.RFC3339)
	}
	return response
}
