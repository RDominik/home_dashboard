package chickendoor

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.etcd.io/bbolt"
)

// @brief Verifies supported whole-second motor runtime values and validation boundaries.
// @details The parser accepts JSON numeric values and numeric strings, but rejects
// values outside the supported 1–60 second range, fractions, and unrelated types.
// @param t Go test context.
func TestParseEngineMaxRuntime(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    int
		wantErr bool
	}{
		{name: "minimum", value: float64(1), want: 1},
		{name: "ordinary integer", value: float64(27), want: 27},
		{name: "maximum", value: "60", want: 60},
		{name: "zero rejected", value: float64(0), wantErr: true},
		{name: "above maximum rejected", value: float64(61), wantErr: true},
		{name: "fraction rejected", value: float64(2.5), wantErr: true},
		{name: "boolean rejected", value: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseEngineMaxRuntime(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseEngineMaxRuntime(%v) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("parseEngineMaxRuntime(%v) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

// @brief Verifies the global runtime ceiling limits directional settings without raising them.
// @param t Go test context.
func TestCapMotorRuntimeSeconds(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		maximum int
		want    int
	}{
		{name: "directional setting is lower", seconds: 18, maximum: 27, want: 18},
		{name: "global maximum caps opening runtime", seconds: 27, maximum: 18, want: 18},
		{name: "unset global maximum preserves directional value", seconds: 27, maximum: 0, want: 27},
		{name: "directional value is clamped", seconds: 90, maximum: 0, want: 60},
		{name: "global maximum is clamped", seconds: 60, maximum: 90, want: 60},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := capMotorRuntimeSeconds(test.seconds, test.maximum); got != test.want {
				t.Fatalf("capMotorRuntimeSeconds(%d, %d) = %d, want %d", test.seconds, test.maximum, got, test.want)
			}
		})
	}
}

// @brief Verifies the maximum runtime setting survives a bbolt reload.
// @details This exercises the regular state encoder and startup loader so the
// controller limit remains available to UI clients after an API restart.
// @param t Go test context.
func TestEngineMaxRuntimePersistsInBbolt(t *testing.T) {
	db, err := bbolt.Open(t.TempDir()+"/chickendoor.db", 0o600, nil)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, createErr := tx.CreateBucketIfNotExists([]byte(stateBucketName))
		return createErr
	}); err != nil {
		t.Fatalf("create state bucket: %v", err)
	}

	service := &ChickenDoor{db: db, engineMaxRuntimeSeconds: 27}
	service.persistState()
	restored := &ChickenDoor{db: db}
	restored.loadPersistedState()
	if restored.engineMaxRuntimeSeconds != 27 {
		t.Fatalf("restored engine maximum = %d, want 27", restored.engineMaxRuntimeSeconds)
	}
}

// @brief Verifies the HTTP setting endpoint rejects out-of-range values before MQTT access.
// @param t Go test context.
func TestSetHandlerRejectsInvalidEngineMaxRuntime(t *testing.T) {
	service := &ChickenDoor{engineMaxRuntimeSeconds: 27}
	request := httptest.NewRequest(http.MethodPut, "/api/huehnerklappe/set", strings.NewReader(`{"key":"engineMaxRuntime","value":61}`))
	response := httptest.NewRecorder()
	service.SetHandler(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("set endpoint returned status %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `"ok":false`) {
		t.Fatalf("expected invalid runtime response, got %s", response.Body.String())
	}
	if service.engineMaxRuntimeSeconds != 27 {
		t.Fatalf("invalid request changed stored runtime to %d", service.engineMaxRuntimeSeconds)
	}
	if engineMaxRuntimeTopic != "nano/esp32/engineMaxRuntime" {
		t.Fatalf("engine maximum MQTT topic = %q", engineMaxRuntimeTopic)
	}
}
