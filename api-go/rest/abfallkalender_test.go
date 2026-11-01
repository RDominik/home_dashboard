package rest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// @brief Verifies service startup requests the configured Awido page immediately.
// @details An httptest server isolates the refresh lifecycle from the public Landkreis service while the status snapshot proves that the successful request advances the client reload version.
// @param[in] t Go test context used for temporary storage and lifecycle assertions.
func TestWasteCalendarRefreshesOnStartup(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html>calendar</html>")
	}))
	defer server.Close()

	dbPath := filepath.Join(t.TempDir(), "waste-calendar.db")
	service, err := NewWasteCalendarService(dbPath, server.URL)
	if err != nil {
		t.Fatalf("create calendar service: %v", err)
	}
	service.Start()
	t.Cleanup(service.Stop)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if service.Status().ReloadVersion > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	status := service.Status()
	if requests.Load() != 1 || status.ReloadVersion != 1 {
		t.Fatalf("startup refresh requests=%d, reloadVersion=%d; want one request", requests.Load(), status.ReloadVersion)
	}
	if status.InstanceID == "" || status.LastSuccessAt == "" || status.Error != "" {
		t.Fatalf("startup status = %#v; want instance identity and successful refresh", status)
	}
	service.Stop()

	restarted, err := NewWasteCalendarService(dbPath, server.URL)
	if err != nil {
		t.Fatalf("create restarted calendar service: %v", err)
	}
	restarted.Start()
	t.Cleanup(restarted.Stop)
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if restarted.Status().ReloadVersion > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	restartedStatus := restarted.Status()
	if requests.Load() != 2 || restartedStatus.ReloadVersion != 1 {
		t.Fatalf("restart refresh requests=%d, reloadVersion=%d; want one additional request", requests.Load(), restartedStatus.ReloadVersion)
	}
	if restartedStatus.InstanceID == status.InstanceID || restartedStatus.LastSuccessAt == "" {
		t.Fatalf("restart status = %#v; want new instance identity and successful refresh", restartedStatus)
	}
}

// @brief Verifies the configured refresh interval survives a fresh service instance.
// @details This round trip exercises the same bbolt persistence path used when the backend process restarts.
// @param[in] t Go test context used for temporary database storage and assertions.
func TestWasteCalendarSettingsPersist(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "waste-calendar.db")
	service, err := NewWasteCalendarService(dbPath, "")
	if err != nil {
		t.Fatalf("create initial calendar service: %v", err)
	}
	if err := service.SetSettings(WasteCalendarSettings{RefreshIntervalWeeks: 4}); err != nil {
		t.Fatalf("save refresh interval: %v", err)
	}
	service.Stop()

	restored, err := NewWasteCalendarService(dbPath, "")
	if err != nil {
		t.Fatalf("restore calendar service: %v", err)
	}
	defer restored.Stop()
	if got := restored.Settings().RefreshIntervalWeeks; got != 4 {
		t.Fatalf("restored refresh interval = %d weeks, want 4", got)
	}
}

// @brief Verifies invalid refresh intervals are rejected without changing saved settings.
// @param[in] t Go test context used for service setup and assertions.
func TestWasteCalendarRejectsInvalidRefreshInterval(t *testing.T) {
	service, err := NewWasteCalendarService(filepath.Join(t.TempDir(), "waste-calendar.db"), "")
	if err != nil {
		t.Fatalf("create calendar service: %v", err)
	}
	defer service.Stop()
	if err := service.SetSettings(WasteCalendarSettings{RefreshIntervalWeeks: 0}); err == nil {
		t.Fatal("zero-week refresh interval was accepted")
	}
	if got := service.Settings().RefreshIntervalWeeks; got != wasteCalendarDefaultIntervalWeeks {
		t.Fatalf("refresh interval changed after rejected value: %d weeks", got)
	}
}
