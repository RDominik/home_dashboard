package rest

import (
	"encoding/json"
	"net/http"
	"time"
)

// @brief Dispatches all REST package HTTP endpoints through one service receiver.
// @details
// The dispatcher keeps route selection in the central REST handler file so the
// application entry point needs only one HTTP handler registration. Weather
// requests are forwarded to the WeatherService-backed handlers, while heating
// requests use the shared ETA state and transformation helpers. Unknown paths
// receive a Not Found response without changing any domain state.
// @param[in,out] w HTTP response writer used to return the endpoint response.
// @param[in] r Incoming HTTP request containing the path and HTTP method.
func (rs *RestService) APIHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/weather/status", "/api/weather/settings", "/api/weather/refresh":
		rs.weatherAPIHandler(w, r)
	case "/api/heating/summary":
		HeatingSummary(w, r)
	case "/api/heating/history":
		HeatingHistory(w, r)
	default:
		http.Error(w, "unknown REST endpoint", http.StatusNotFound)
	}
}

// @brief Dispatches the supported Weather Underground API operations.
// @details
// This second-level dispatcher keeps the top-level REST route table compact and
// delegates each weather operation to a focused handler. It deliberately leaves
// method validation to the operation-specific handler so each endpoint can
// return its own precise Allow header and status code behavior.
// @param[in,out] w HTTP response writer used to return the weather response.
// @param[in] r Incoming HTTP request containing the weather path and method.
func (rs *RestService) weatherAPIHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/weather/status":
		rs.weatherStatusHandler(w, r)
	case "/api/weather/settings":
		rs.weatherSettingsHandler(w, r)
	case "/api/weather/refresh":
		rs.weatherRefreshHandler(w, r)
	default:
		apiJSON(w, http.StatusNotFound, map[string]string{"error": "unknown weather endpoint"})
	}
}

// @brief Returns the cached weather observation and active settings.
// @details
// The handler is read-only and exposes the WeatherService snapshot through its
// public getter. It does not trigger a provider request, which keeps status
// polling cheap and ensures that the server-side request-rate limit cannot be
// bypassed by frequent UI reads.
// @param[in,out] w HTTP response writer receiving the JSON status payload.
// @param[in] r Incoming HTTP request; only GET is accepted.
func (rs *RestService) weatherStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apiJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	apiJSON(w, http.StatusOK, rs.weather.GetStatus())
}

// @brief Reads or updates persisted Weather Underground settings.
// @details
// GET returns the normalized settings currently held by the WeatherService.
// PUT decodes the request body, persists the validated settings through the
// service setter, and starts a background refresh so a successful configuration
// change can become visible without blocking the HTTP response. Unsupported
// methods are rejected and advertise the permitted methods through Allow.
// @param[in,out] w HTTP response writer receiving the settings or error payload.
// @param[in] r Incoming HTTP request containing the method and, for PUT, JSON settings.
func (rs *RestService) weatherSettingsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		apiJSON(w, http.StatusOK, rs.weather.GetSettings())
	case http.MethodPut:
		var settings WeatherSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			apiJSON(w, http.StatusBadRequest, map[string]string{"error": "ungültiger JSON body"})
			return
		}
		if err := rs.weather.SetSettings(settings); err != nil {
			apiJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		go rs.weather.refresh()
		apiJSON(w, http.StatusOK, map[string]any{"ok": true, "settings": rs.weather.GetSettings()})
	default:
		w.Header().Set("Allow", "GET, PUT")
		apiJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// @brief Starts an immediate asynchronous weather refresh.
// @details
// The refresh is launched in a goroutine so the HTTP request acknowledges the
// accepted operation without waiting for the remote Weather Underground API.
// The WeatherService serializes this request with scheduled polling and applies
// the configured minimum request interval before making an outbound call.
// @param[in,out] w HTTP response writer receiving the acceptance response.
// @param[in] r Incoming HTTP request; only POST is accepted.
func (rs *RestService) weatherRefreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apiJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	go rs.weather.refresh()
	apiJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// @brief Returns the latest ETA in-memory payload as JSON.
// @details
// The endpoint reads the most recently published ETA payload without initiating
// a new upstream request. When no payload has been received yet, it returns an
// empty JSON object so clients receive a stable JSON response shape rather than
// a null value or an HTTP error.
// @param[in,out] w HTTP response writer receiving the summary payload.
// @param[in] r Incoming HTTP request; the request is accepted for endpoint compatibility.
func HeatingSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if payload, ok := getLatestPayload(); ok {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{})
}

// @brief Returns recent heating history points for the heating dashboard.
// @details
// The current implementation generates a deterministic twelve-hour series in
// five-minute steps. The requested interval is retained in the response for
// frontend compatibility, while an omitted interval defaults to 5m. This
// handler performs no persistence or upstream I/O and therefore remains safe to
// call repeatedly during dashboard rendering.
// @param[in,out] w HTTP response writer receiving the history JSON payload.
// @param[in] r Incoming HTTP request containing the optional interval query parameter.
func HeatingHistory(w http.ResponseWriter, r *http.Request) {
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}

	var points []map[string]any
	end := time.Now().UTC()
	start := end.Add(-12 * time.Hour)
	for current := start; !current.After(end); current = current.Add(5 * time.Minute) {
		minute := current.Minute()
		points = append(points, map[string]any{
			"t":             current.Format("2006-01-02T15:04:05.000Z"),
			"boiler_temp":   70.0 + float64(minute%10)*0.4,
			"buffer_top":    66.0 + float64(minute%8)*0.3,
			"buffer_bottom": 44.0 + float64(minute%6)*0.25,
			"return_temp":   50.0 + float64(minute%12)*0.2,
			"feed_rate":     30 + (minute%5)*3,
		})
	}

	apiJSON(w, http.StatusOK, map[string]any{"series": points, "interval": interval})
}

// @brief Writes a JSON API response with a consistent content type and status.
// @details
// The helper centralizes response headers, status transmission, and JSON
// encoding for all REST handlers in this file. Encoding failures are ignored
// after the response has started because an HTTP status or body cannot be safely
// replaced at that point; handlers therefore pass only JSON-marshalable values.
// @param[in,out] w HTTP response writer receiving the encoded response.
// @param[in] status HTTP status code written before encoding the payload.
// @param[in] value JSON-marshalable response value.
func apiJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
