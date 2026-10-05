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

// @brief Verifies sleep duration preserves the action interval and never sleeps for zero seconds.
// @param t Go test context.
func TestTestModeSleepSecondsUntilDeadline(t *testing.T) {
	now := time.Date(2026, time.June, 12, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		deadline time.Time
		want     int
	}{
		{name: "remaining interval", deadline: now.Add(45*time.Minute + 250*time.Millisecond), want: 2701},
		{name: "deadline consumed by awake allowance", deadline: now.Add(-time.Second), want: 1},
		{name: "deadline now", deadline: now, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := testModeSleepSecondsUntil(now, test.deadline); got != test.want {
				t.Fatalf("sleep seconds = %d, want %d", got, test.want)
			}
		})
	}
}

// @brief Verifies test-mode controller transitions update only test history.
// @details
// A successful test sleep command may be acknowledged after the pending flag
// has already been cleared. This regression test proves the in-flight history
// row continues to own both sleep and wake acknowledgements and that the last
// schedule row remains unchanged.
// @param t Go test context.
func TestTestModeTransitionsDoNotModifyScheduleHistory(t *testing.T) {
	sleepCommandAt := time.Date(2026, time.June, 12, 10, 0, 0, 0, time.UTC).UnixMilli()
	service := &ChickenDoor{
		lastControllerState: "online",
		scheduleHistory:     []ScheduleHistoryEntry{{SleepSeconds: 600}},
		testModeHistory:     []TestModeHistoryEntry{{Action: "open", SleepCommandAtMs: sleepCommandAt}},
	}
	sleepingAt := time.UnixMilli(sleepCommandAt + 5000)
	service.updateStateTracking("sleeping", "sleeping", sleepingAt, true)
	if service.scheduleHistory[0].SleepingAtMs != 0 {
		t.Fatalf("test-mode sleeping ACK contaminated schedule history: %#v", service.scheduleHistory[0])
	}
	if service.testModeHistory[0].SleepingAtMs != sleepingAt.UnixMilli() {
		t.Fatalf("test-mode sleeping ACK missing from test history: %#v", service.testModeHistory[0])
	}
	wokeAt := sleepingAt.Add(2 * time.Minute)
	service.updateStateTracking("online", "online", wokeAt, true)
	if service.scheduleHistory[0].WokeUpAtMs != 0 {
		t.Fatalf("test-mode wake ACK contaminated schedule history: %#v", service.scheduleHistory[0])
	}
	if service.testModeHistory[0].WokeUpAtMs != wokeAt.UnixMilli() {
		t.Fatalf("test-mode wake ACK missing from test history: %#v", service.testModeHistory[0])
	}
}

// @brief Verifies every shared ChickenDoor UI setting survives a bbolt reload.
// @details
// The test exercises the public PUT handler, its shared persistState path, and
// loadPersistedState on a fresh ChickenDoor instance. It covers manual sleep,
// schedule action/time/awake configuration, directional auto-stop settings,
// the shared motor ceiling, UI preferences, and all editable test-mode values.
// @param t Go test context used for temporary database storage and assertions.
func TestAllChickenDoorSettingsPersistThroughUIState(t *testing.T) {
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
	service.testModeHistory = []TestModeHistoryEntry{{Action: "open", ActionAtMs: 1781244000000, MaxAwakeSeconds: 45}}
	request := httptest.NewRequest(http.MethodPut, "/api/huehnerklappe/ui-state", strings.NewReader(`{"sleepTime":95,"sleepUntil":"06:45","controlMode":"schedule","historyExpanded":true,"scheduleTimestamps":["06:30:00"],"scheduleEntries":[{"timestamp":"06:30:00","action":"open"}],"awakeSeconds":45,"motorAutoStopOpenSeconds":27,"motorAutoStopCloseSeconds":18,"engineMaxRuntimeSeconds":40,"testModeEnabled":true,"testModeIntervalMinutes":45,"testModeStartTime":"07:15","testModeEndTime":"21:30","testModeMaxAwakeSeconds":75}`))
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
	if restored.testModeMaxAwakeSeconds != 75 {
		t.Fatalf("max awake time restored as %d seconds, want 75", restored.testModeMaxAwakeSeconds)
	}
	if len(restored.testModeHistory) != 1 || restored.testModeHistory[0].Action != "open" {
		t.Fatalf("test-mode history was not restored: %#v", restored.testModeHistory)
	}
	if restored.sleepTime != 95 || restored.sleepUntil != "06:45" || restored.controlMode != "schedule" || !restored.historyExpanded {
		t.Fatalf("manual/shared UI settings not restored: sleepTime=%d sleepUntil=%q controlMode=%q historyExpanded=%t", restored.sleepTime, restored.sleepUntil, restored.controlMode, restored.historyExpanded)
	}
	if restored.scheduleAwakeSeconds != 45 || len(restored.scheduleEntries) != 1 || restored.scheduleEntries[0] != (ScheduleEntry{Timestamp: "06:30:00", Action: "open"}) {
		t.Fatalf("schedule settings not restored: awake=%d entries=%#v", restored.scheduleAwakeSeconds, restored.scheduleEntries)
	}
	if restored.motorAutoStopOpenSeconds != 27 || restored.motorAutoStopCloseSeconds != 18 || restored.engineMaxRuntimeSeconds != 40 {
		t.Fatalf("motor runtime settings not restored: open=%d close=%d max=%d", restored.motorAutoStopOpenSeconds, restored.motorAutoStopCloseSeconds, restored.engineMaxRuntimeSeconds)
	}
}

// @brief Verifies repeated UI-state saves do not reset an enabled test cycle deadline.
// @details
// The frontend sends the full shared UI state whenever unrelated controls or
// tabs change. An unchanged true enable flag must therefore leave an armed
// wake deadline intact, otherwise the controller could remain asleep while the
// backend keeps moving the expected action time.
// @param t Go test context.
func TestTestModeSettingsSavePreservesArmedDeadline(t *testing.T) {
	deadline := time.Now().Add(10 * time.Minute)
	service := &ChickenDoor{
		testModeEnabled:         true,
		testModeIntervalMinutes: 10,
		testModeStartTime:       "08:00",
		testModeEndTime:         "20:00",
		testModeNextAction:      "close",
		testModeNextAt:          deadline,
	}
	request := httptest.NewRequest(http.MethodPut, "/api/huehnerklappe/ui-state", strings.NewReader(`{"testModeEnabled":true,"testModeIntervalMinutes":10,"testModeStartTime":"08:00","testModeEndTime":"20:00"}`))
	response := httptest.NewRecorder()
	service.UIStateHandler(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("UI-state PUT returned status %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !service.testModeNextAt.Equal(deadline) {
		t.Fatalf("repeated settings save changed next action deadline from %s to %s", deadline, service.testModeNextAt)
	}
}
