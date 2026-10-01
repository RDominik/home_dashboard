package chickendoor

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.etcd.io/bbolt"
)

// @brief Verifies inclusive-start and exclusive-end test-mode window boundaries.
// @param t Go test context.
func TestTestModeWindowBoundaries(t *testing.T) {
	location := scheduleNow().Location()
	tests := []struct {
		name string
		time string
		want bool
	}{
		{name: "before start", time: "07:59", want: false},
		{name: "at start", time: "08:00", want: true},
		{name: "inside", time: "12:30", want: true},
		{name: "at end", time: "20:00", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now, err := time.ParseInLocation("2006-01-02 15:04", "2026-06-12 "+test.time, location)
			if err != nil {
				t.Fatalf("parse test time: %v", err)
			}
			if got := isWithinTestModeWindow(now, "08:00", "20:00"); got != test.want {
				t.Fatalf("window membership at %s = %t, want %t", test.time, got, test.want)
			}
		})
	}
}

// @brief Verifies a test-mode window that crosses midnight and an empty window.
// @param t Go test context.
func TestTestModeWindowCrossesMidnight(t *testing.T) {
	location := scheduleNow().Location()
	tests := []struct {
		name  string
		time  string
		start string
		end   string
		want  bool
	}{
		{name: "late evening", time: "22:00", start: "21:00", end: "06:00", want: true},
		{name: "early morning", time: "05:59", start: "21:00", end: "06:00", want: true},
		{name: "at overnight end", time: "06:00", start: "21:00", end: "06:00", want: false},
		{name: "outside overnight window", time: "12:00", start: "21:00", end: "06:00", want: false},
		{name: "equal boundaries are empty", time: "22:00", start: "08:00", end: "08:00", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now, err := time.ParseInLocation("2006-01-02 15:04", "2026-06-12 "+test.time, location)
			if err != nil {
				t.Fatalf("parse test time: %v", err)
			}
			if got := isWithinTestModeWindow(now, test.start, test.end); got != test.want {
				t.Fatalf("window membership at %s = %t, want %t", test.time, got, test.want)
			}
		})
	}
}

// @brief Verifies that test-mode configuration sent through ui-state survives a bbolt reload.
// @details
// The test exercises the public PUT handler, its shared persistState path, and
// loadPersistedState on a fresh ChickenDoor instance. It ensures both the
// enable flag and all user-editable interval/window values are stored together
// instead of only remaining in the original process memory.
// @param t Go test context used for temporary database storage and assertions.
func TestTestModeSettingsPersistThroughUIState(t *testing.T) {
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

	service := &ChickenDoor{db: db, testModeIntervalMinutes: 30, testModeStartTime: "08:00", testModeEndTime: "20:00", testModeNextAction: "open"}
	request := httptest.NewRequest(http.MethodPut, "/api/huehnerklappe/ui-state", strings.NewReader(`{"testModeEnabled":true,"testModeIntervalMinutes":45,"testModeStartTime":"07:15","testModeEndTime":"21:30"}`))
	response := httptest.NewRecorder()
	service.UIStateHandler(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("UI-state PUT returned status %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	// Restore from the same durable bbolt database through the normal startup
	// loader, which models a new backend process after restart.
	restored := &ChickenDoor{db: db, testModeIntervalMinutes: 30, testModeStartTime: "08:00", testModeEndTime: "20:00", testModeNextAction: "open"}
	restored.loadPersistedState()
	if !restored.testModeEnabled {
		t.Fatal("test-mode enabled flag was not restored")
	}
	if restored.testModeIntervalMinutes != 45 {
		t.Fatalf("interval restored as %d, want 45 minutes", restored.testModeIntervalMinutes)
	}
	if restored.testModeStartTime != "07:15" || restored.testModeEndTime != "21:30" {
		t.Fatalf("window restored as %s-%s, want 07:15-21:30", restored.testModeStartTime, restored.testModeEndTime)
	}
}
