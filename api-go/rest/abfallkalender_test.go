package rest

import (
	"fmt"
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
	pickupDate := time.Now().In(time.Local).AddDate(0, 0, 2).Format("20060102")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/Customer/kelheim/v3/Calendar2.aspx":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, "<html>calendar</html>")
		case "/Customer/kelheim/v3/KalenderICS.aspx":
			w.Header().Set("Content-Type", "text/calendar")
			_, _ = fmt.Fprintf(w, "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nSUMMARY:Biotonne in Stadt Riedenburg\r\nDTSTART;VALUE=DATE:%s\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nSUMMARY:Papierabfuhr in Stadt Riedenburg\r\nDTSTART;VALUE=DATE:%s\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", pickupDate, pickupDate)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dbPath := filepath.Join(t.TempDir(), "waste-calendar.db")
	calendarURL := server.URL + "/Customer/kelheim/v3/Calendar2.aspx"
	service, err := NewWasteCalendarService(dbPath, calendarURL)
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
	if requests.Load() != 2 || status.ReloadVersion != 1 {
		t.Fatalf("startup refresh requests=%d, reloadVersion=%d; want page and ICS requests", requests.Load(), status.ReloadVersion)
	}
	if status.InstanceID == "" || status.LastSuccessAt == "" || status.Error != "" {
		t.Fatalf("startup status = %#v; want instance identity and successful refresh", status)
	}
	if status.NextPickup == nil || status.NextPickup.Date != time.Now().In(time.Local).AddDate(0, 0, 2).Format("2006-01-02") || len(status.NextPickup.Fractions) != 2 {
		t.Fatalf("startup next pickup = %#v; want two fractions grouped on the nearest date", status.NextPickup)
	}
	service.Stop()

	restarted, err := NewWasteCalendarService(dbPath, calendarURL)
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
	if requests.Load() != 4 || restartedStatus.ReloadVersion != 1 {
		t.Fatalf("restart refresh requests=%d, reloadVersion=%d; want one page and ICS request", requests.Load(), restartedStatus.ReloadVersion)
	}
	if restartedStatus.InstanceID == status.InstanceID || restartedStatus.LastSuccessAt == "" {
		t.Fatalf("restart status = %#v; want new instance identity and successful refresh", restartedStatus)
	}
	if restartedStatus.NextPickup == nil || len(restartedStatus.NextPickup.Fractions) != 2 {
		t.Fatalf("restart next pickup = %#v; want persisted same-day fractions", restartedStatus.NextPickup)
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
