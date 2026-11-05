package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

const (
	wasteCalendarDefaultURL           = "https://awido.cubefour.de/Customer/kelheim/v3/Calendar2.aspx"
	wasteCalendarDefaultIntervalWeeks = 1
	wasteCalendarMaxIntervalWeeks     = 52
	wasteCalendarPollInterval         = time.Minute
	wasteCalendarDBPath               = "data/waste_calendar.db"
	wasteCalendarBucket               = "wasteCalendar"
	wasteCalendarStateKey             = "state"
	wasteCalendarLocationID           = "00000000-0000-0000-0000-000000001228"
	wasteCalendarFractionIDs          = "2,6,9,11,12"
)

// @brief Contains the user-configurable waste-calendar refresh interval.
// @details The first refresh interval is one week; supported values are 1 through 52 weeks.
type WasteCalendarSettings struct {
	// @brief Number of weeks between scheduled Awido refreshes.
	RefreshIntervalWeeks int `json:"refreshIntervalWeeks"`
}

// @brief Describes the current Awido refresh state returned to the frontend.
// @details The embedded Awido page remains responsible for rendering its native calendar and location controls.
type WasteCalendarStatus struct {
	// @brief Fixed location shown by this dashboard page.
	Location string `json:"location"`
	// @brief Public Awido calendar URL embedded by the frontend.
	CalendarURL string `json:"calendarUrl"`
	// @brief Unique identity of this backend process, changed on every start.
	InstanceID string `json:"instanceId"`
	// @brief Increments after each backend-issued calendar refresh attempt.
	ReloadVersion uint64 `json:"reloadVersion"`
	// @brief Configured refresh interval in weeks.
	RefreshIntervalWeeks int `json:"refreshIntervalWeeks"`
	// @brief Timestamp of the latest refresh attempt, if one has started.
	LastAttemptAt string `json:"lastAttemptAt,omitempty"`
	// @brief Timestamp of the latest successful upstream response.
	LastSuccessAt string `json:"lastSuccessAt,omitempty"`
	// @brief Expected time of the next scheduled refresh.
	NextRefreshAt string `json:"nextRefreshAt,omitempty"`
	// @brief True while the backend is requesting the upstream calendar page.
	Refreshing bool `json:"refreshing"`
	// @brief Latest upstream error, empty after a successful request.
	Error string `json:"error,omitempty"`
	// @brief Nearest future pickup and every waste fraction scheduled for that same date.
	NextPickup *WasteCalendarPickup `json:"nextPickup,omitempty"`
}

// @brief Describes the next scheduled waste pickup for the selected Jachenhausen location.
// @details Date is a local calendar date in YYYY-MM-DD format; fractions contains normalized names suitable for stable frontend icon mapping.
type WasteCalendarPickup struct {
	// @brief Local date on which the next one or more waste fractions are collected.
	Date string `json:"date"`
	// @brief Normalized waste fractions collected on Date, including simultaneous collections.
	Fractions []string `json:"fractions"`
}

// @brief Holds the durable subset of waste-calendar service state.
// @details Process identity and reload version are intentionally ephemeral; they identify lifecycle events to connected browsers.
type wasteCalendarPersistedState struct {
	RefreshIntervalWeeks int                  `json:"refreshIntervalWeeks"`
	LastAttemptAt        string               `json:"lastAttemptAt,omitempty"`
	LastSuccessAt        string               `json:"lastSuccessAt,omitempty"`
	NextPickup           *WasteCalendarPickup `json:"nextPickup,omitempty"`
}

// @brief Owns the cached refresh schedule and upstream health check for the Awido calendar.
// @details A real GET is issued at service startup and at the configured interval. The browser reloads the original embedded calendar whenever the backend's instance or reload version changes.
type WasteCalendarService struct {
	mu            sync.RWMutex
	refreshMu     sync.Mutex
	settings      WasteCalendarSettings
	lastAttemptAt time.Time
	lastSuccessAt time.Time
	lastError     string
	nextPickup    *WasteCalendarPickup
	instanceID    string
	reloadVersion uint64
	refreshing    bool
	db            *bbolt.DB
	client        *http.Client
	calendarURL   string
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	refreshNow    chan struct{}
	stopOnce      sync.Once
}

// @brief Opens the waste-calendar state database and restores its refresh settings.
// @details Missing records use the weekly default. A caller-supplied source URL allows isolated service tests without reaching the public Awido server.
// @param[in] dbPath File path of the bbolt database; blank selects the application data path.
// @param[in] calendarURL Public calendar page URL; blank selects the Landkreis Kelheim Awido page.
// @return Initialized calendar service, or an error if its database or persisted state cannot be opened.
func NewWasteCalendarService(dbPath, calendarURL string) (*WasteCalendarService, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = wasteCalendarDBPath
	}
	if strings.TrimSpace(calendarURL) == "" {
		calendarURL = wasteCalendarDefaultURL
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create waste-calendar database directory: %w", err)
	}
	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("open waste-calendar database: %w", err)
	}
	service := &WasteCalendarService{
		settings:    WasteCalendarSettings{RefreshIntervalWeeks: wasteCalendarDefaultIntervalWeeks},
		db:          db,
		client:      &http.Client{Timeout: 20 * time.Second},
		calendarURL: calendarURL,
		done:        make(chan struct{}),
	}
	if err := service.load(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return service, nil
}

// @brief Restores refresh settings and the latest upstream request timestamps from bbolt.
// @details Missing buckets or keys represent a first-run database and retain the weekly default rather than producing a startup error.
// @return Error when persisted JSON is invalid or the database view cannot be read.
func (s *WasteCalendarService) load() error {
	return s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(wasteCalendarBucket))
		if bucket == nil {
			return nil
		}
		raw := bucket.Get([]byte(wasteCalendarStateKey))
		if len(raw) == 0 {
			return nil
		}
		var state wasteCalendarPersistedState
		if err := json.Unmarshal(raw, &state); err != nil {
			return fmt.Errorf("decode waste-calendar state: %w", err)
		}
		if state.RefreshIntervalWeeks >= 1 && state.RefreshIntervalWeeks <= wasteCalendarMaxIntervalWeeks {
			s.settings.RefreshIntervalWeeks = state.RefreshIntervalWeeks
		}
		if state.LastAttemptAt != "" {
			s.lastAttemptAt, _ = time.Parse(time.RFC3339Nano, state.LastAttemptAt)
		}
		if state.LastSuccessAt != "" {
			s.lastSuccessAt, _ = time.Parse(time.RFC3339Nano, state.LastSuccessAt)
		}
		if state.NextPickup != nil {
			pickup := *state.NextPickup
			pickup.Fractions = append([]string(nil), state.NextPickup.Fractions...)
			s.nextPickup = &pickup
		}
		return nil
	})
}

// @brief Persists settings and upstream timestamps as one bbolt snapshot.
// @details The service lock is released before starting the bbolt write transaction so network and database work do not block status readers.
// @return Error when JSON encoding or the bbolt update fails.
func (s *WasteCalendarService) persist() error {
	s.mu.RLock()
	state := wasteCalendarPersistedState{
		RefreshIntervalWeeks: s.settings.RefreshIntervalWeeks,
		LastAttemptAt:        formatWasteCalendarTime(s.lastAttemptAt),
		LastSuccessAt:        formatWasteCalendarTime(s.lastSuccessAt),
	}
	if s.nextPickup != nil {
		pickup := *s.nextPickup
		pickup.Fractions = append([]string(nil), s.nextPickup.Fractions...)
		state.NextPickup = &pickup
	}
	s.mu.RUnlock()

	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode waste-calendar state: %w", err)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(wasteCalendarBucket))
		if err != nil {
			return err
		}
		return bucket.Put([]byte(wasteCalendarStateKey), raw)
	})
}

// @brief Starts the immediate refresh and the periodic refresh scheduler.
// @details The instance identity changes at every process start, allowing already-open pages to detect a backend restart and reload their iframe.
func (s *WasteCalendarService) Start() {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.done = make(chan struct{})
	s.refreshNow = make(chan struct{}, 1)
	s.instanceID = time.Now().UTC().Format(time.RFC3339Nano)
	ctx := s.ctx
	s.mu.Unlock()
	go s.runLoop(ctx)
}

// @brief Runs the startup fetch and checks once per minute whether the saved interval has elapsed.
// @details Refresh requests from the menu share the same loop, so overlapping timer, startup, and manual loads cannot issue duplicate requests.
// @param[in] ctx Cancellation context owned by Start.
func (s *WasteCalendarService) runLoop(ctx context.Context) {
	defer close(s.done)
	s.refresh(ctx)
	ticker := time.NewTicker(wasteCalendarPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.refreshNow:
			s.refresh(ctx)
		case now := <-ticker.C:
			if s.refreshDue(now) {
				s.refresh(ctx)
			}
		}
	}
}

// @brief Determines whether the configured time between upstream refresh attempts has elapsed.
// @param[in] now Current comparison timestamp.
// @return True when a refresh is due or no attempt has been recorded.
func (s *WasteCalendarService) refreshDue(now time.Time) bool {
	s.mu.RLock()
	lastAttemptAt := s.lastAttemptAt
	intervalWeeks := s.settings.RefreshIntervalWeeks
	s.mu.RUnlock()
	if lastAttemptAt.IsZero() {
		return true
	}
	interval := time.Duration(intervalWeeks*7*24) * time.Hour
	return !now.Before(lastAttemptAt.Add(interval))
}

// @brief Requests the Awido calendar page and updates refresh state for all connected clients.
// @details Every request advances ReloadVersion when it finishes, even after an upstream error, so the browser can still refresh the embedded calendar independently.
// @param[in] ctx Request context cancelled when the service shuts down.
func (s *WasteCalendarService) refresh(ctx context.Context) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if err := ctx.Err(); err != nil {
		return
	}

	attemptAt := time.Now().UTC()
	s.mu.Lock()
	s.refreshing = true
	s.lastAttemptAt = attemptAt
	s.lastError = ""
	s.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.calendarURL, nil)
	if err == nil {
		request.Header.Set("User-Agent", "ETA-WebGUI-Abfallkalender/1.0")
		var response *http.Response
		response, err = s.client.Do(request)
		if err == nil {
			if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				err = fmt.Errorf("Awido antwortet mit HTTP %d", response.StatusCode)
			} else if err = drainWasteCalendarBody(response.Body); err != nil {
				err = fmt.Errorf("read Awido response: %w", err)
			}
			_ = response.Body.Close()
		}
	}
	var nextPickup *WasteCalendarPickup
	if err == nil {
		nextPickup, err = s.fetchNextPickup(ctx, attemptAt)
	}

	s.mu.Lock()
	s.refreshing = false
	s.reloadVersion++
	if err != nil {
		s.lastError = err.Error()
	} else {
		s.lastSuccessAt = time.Now().UTC()
		s.nextPickup = nextPickup
	}
	s.mu.Unlock()
	if persistErr := s.persist(); persistErr != nil {
		log.Printf("[waste-calendar] persist refresh state failed: %v", persistErr)
	}
	if err != nil {
		log.Printf("[waste-calendar] refresh failed: %v", err)
		return
	}
	log.Printf("[waste-calendar] upstream calendar checked at %s", attemptAt.Format(time.RFC3339))
}

// @brief Returns a consistent snapshot of settings and refresh metadata.
// @return Current calendar status including the next scheduled refresh time.
func (s *WasteCalendarService) Status() WasteCalendarStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := WasteCalendarStatus{
		Location:             "Stadt Riedenburg, Ortsteil Jachenhausen",
		CalendarURL:          s.calendarURL,
		InstanceID:           s.instanceID,
		ReloadVersion:        s.reloadVersion,
		RefreshIntervalWeeks: s.settings.RefreshIntervalWeeks,
		Refreshing:           s.refreshing,
		Error:                s.lastError,
	}
	if s.nextPickup != nil {
		pickup := *s.nextPickup
		pickup.Fractions = append([]string(nil), s.nextPickup.Fractions...)
		status.NextPickup = &pickup
	}
	status.LastAttemptAt = formatWasteCalendarTime(s.lastAttemptAt)
	status.LastSuccessAt = formatWasteCalendarTime(s.lastSuccessAt)
	if !s.lastAttemptAt.IsZero() {
		interval := time.Duration(s.settings.RefreshIntervalWeeks*7*24) * time.Hour
		status.NextRefreshAt = s.lastAttemptAt.Add(interval).Format(time.RFC3339)
	}
	return status
}

// @brief Returns the currently configured refresh settings.
// @return Copy of the persisted interval settings.
func (s *WasteCalendarService) Settings() WasteCalendarSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// @brief Validates and persists a new weekly refresh interval.
// @param[in] settings Candidate refresh settings from the API request.
// @return Error when the interval is outside the supported 1 through 52 week range or persistence fails.
func (s *WasteCalendarService) SetSettings(settings WasteCalendarSettings) error {
	if settings.RefreshIntervalWeeks < 1 || settings.RefreshIntervalWeeks > wasteCalendarMaxIntervalWeeks {
		return fmt.Errorf("refreshIntervalWeeks muss zwischen 1 und %d liegen", wasteCalendarMaxIntervalWeeks)
	}
	s.mu.Lock()
	previous := s.settings
	s.settings = settings
	s.mu.Unlock()
	if err := s.persist(); err != nil {
		s.mu.Lock()
		s.settings = previous
		s.mu.Unlock()
		return fmt.Errorf("persist waste-calendar settings: %w", err)
	}
	return nil
}

// @brief Queues a manual refresh request for the service loop.
// @return Error if the service has not started; repeated requests are coalesced into one upstream load.
func (s *WasteCalendarService) RequestRefresh() error {
	s.mu.RLock()
	refreshNow := s.refreshNow
	running := s.cancel != nil
	s.mu.RUnlock()
	if !running {
		return fmt.Errorf("waste-calendar service is not running")
	}
	select {
	case refreshNow <- struct{}{}:
	default:
	}
	return nil
}

// @brief Stops the scheduler and closes its database after background work exits.
// @details The stop operation is idempotent and waits for the current upstream request to observe context cancellation before closing bbolt.
func (s *WasteCalendarService) Stop() {
	s.stopOnce.Do(func() {
		s.mu.RLock()
		cancel := s.cancel
		done := s.done
		s.mu.RUnlock()
		if cancel != nil {
			cancel()
			<-done
		}
		if s.db != nil {
			if err := s.db.Close(); err != nil {
				log.Printf("[waste-calendar] database close failed: %v", err)
			}
		}
	})
}

// @brief Formats a calendar timestamp as RFC3339 or an empty string.
// @param[in] value Time value to encode.
// @return UTC RFC3339 timestamp, or empty string when the value is zero.
func formatWasteCalendarTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

// @brief Drains at most the configured response-body limit before closing an HTTP response.
// @details This helper keeps successful calendar refresh requests from retaining network resources when the page HTML is not parsed by the dashboard.
// @param[in] body HTTP response body to consume.
// @return Error if reading the response body fails.
func drainWasteCalendarBody(body io.Reader) error {
	_, err := io.Copy(io.Discard, io.LimitReader(body, 4<<20))
	return err
}

// @brief Fetches the nearest upcoming pickup from Awido's official iCalendar export.
// @details The current year is checked first and the following year is requested only when the current calendar has no future event. This supports year-end dates while keeping normal refreshes to one export request.
// @param[in] ctx Request context cancelled when the service shuts down.
// @param[in] now Reference timestamp used to select the local current date and calendar year.
// @return Earliest pickup with all same-day fractions, or an error when Awido cannot provide a usable calendar.
func (s *WasteCalendarService) fetchNextPickup(ctx context.Context, now time.Time) (*WasteCalendarPickup, error) {
	for year := now.In(time.Local).Year(); year <= now.In(time.Local).Year()+1; year++ {
		calendarURL, err := buildWasteCalendarICSURL(s.calendarURL, year)
		if err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, calendarURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create Awido iCalendar request: %w", err)
		}
		request.Header.Set("User-Agent", "ETA-WebGUI-Abfallkalender/1.0")
		response, err := s.client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("request Awido iCalendar export: %w", err)
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			_ = response.Body.Close()
			return nil, fmt.Errorf("Awido iCalendar export antwortet mit HTTP %d", response.StatusCode)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		_ = response.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read Awido iCalendar export: %w", readErr)
		}
		pickup, err := parseWasteCalendarICS(string(body), now)
		if err != nil {
			return nil, err
		}
		if pickup != nil {
			return pickup, nil
		}
	}
	return nil, fmt.Errorf("Awido iCalendar export enthält keinen anstehenden Abholtermin")
}

// @brief Builds the fixed-location Awido iCalendar export URL for one calendar year.
// @details The page endpoint is replaced with Awido's documented KalenderICS.aspx export endpoint; only the configured calendar host and path are reused, while the location and fraction selection remain fixed to Jachenhausen.
// @param[in] calendarURL Public calendar page URL configured for the service.
// @param[in] year Calendar year requested from Awido.
// @return Encoded iCalendar export URL or an error if the configured page URL is invalid.
func buildWasteCalendarICSURL(calendarURL string, year int) (string, error) {
	parsed, err := url.Parse(calendarURL)
	if err != nil {
		return "", fmt.Errorf("parse Awido calendar URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("parse Awido calendar URL: absolute URL required")
	}
	parsed.Path = path.Join(path.Dir(parsed.Path), "KalenderICS.aspx")
	query := parsed.Query()
	query.Set("oid", wasteCalendarLocationID)
	query.Set("jahr", fmt.Sprintf("%d", year))
	query.Set("fraktionen", wasteCalendarFractionIDs)
	query.Set("reminder", "-1.08:00")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// @brief Parses an Awido iCalendar document and groups the nearest future events by local date.
// @details RFC 5545 folded lines are unfolded before VEVENT processing. Only the four collection fractions used by the Landkreis calendar are normalized; all matching events on the earliest date are returned together.
// @param[in] raw Complete iCalendar text returned by Awido.
// @param[in] now Reference timestamp that determines today's local date.
// @return Earliest pickup and its normalized fractions, nil when no future supported event exists, or an error for an invalid reference date.
func parseWasteCalendarICS(raw string, now time.Time) (*WasteCalendarPickup, error) {
	if !strings.Contains(raw, "BEGIN:VCALENDAR") || !strings.Contains(raw, "END:VCALENDAR") {
		return nil, fmt.Errorf("Awido iCalendar export ist ungültig")
	}
	localNow := now.In(time.Local)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.Local)
	physicalLines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(physicalLines))
	for _, line := range physicalLines {
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(lines) > 0 {
			lines[len(lines)-1] += line[1:]
			continue
		}
		lines = append(lines, line)
	}

	type calendarEvent struct {
		date     time.Time
		fraction string
	}
	events := make([]calendarEvent, 0)
	insideEvent := false
	var eventDate time.Time
	var eventFraction string
	for _, line := range lines {
		if line == "BEGIN:VEVENT" {
			insideEvent = true
			eventDate = time.Time{}
			eventFraction = ""
			continue
		}
		if line == "END:VEVENT" {
			if insideEvent && !eventDate.IsZero() && eventFraction != "" && !eventDate.Before(today) {
				events = append(events, calendarEvent{date: eventDate, fraction: eventFraction})
			}
			insideEvent = false
			continue
		}
		if !insideEvent {
			continue
		}
		property, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch {
		case strings.HasPrefix(strings.ToUpper(property), "DTSTART"):
			if len(value) < len("20060102") {
				continue
			}
			parsedDate, err := time.ParseInLocation("20060102", value[:8], time.Local)
			if err == nil {
				eventDate = parsedDate
			}
		case strings.EqualFold(property, "SUMMARY"):
			eventFraction = normalizeWasteFraction(value)
		}
	}
	if len(events) == 0 {
		return nil, nil
	}
	sort.Slice(events, func(left, right int) bool { return events[left].date.Before(events[right].date) })
	nearestDate := events[0].date
	fractions := make(map[string]struct{})
	for _, event := range events {
		if !event.date.Equal(nearestDate) {
			break
		}
		fractions[event.fraction] = struct{}{}
	}
	orderedFractions := make([]string, 0, len(fractions))
	for fraction := range fractions {
		orderedFractions = append(orderedFractions, fraction)
	}
	sort.Strings(orderedFractions)
	return &WasteCalendarPickup{Date: nearestDate.Format("2006-01-02"), Fractions: orderedFractions}, nil
}

// @brief Maps an Awido event summary to a stable waste-fraction key.
// @details The source may append a municipality name to each summary, so matching is case-insensitive and uses the known collection names rather than the full display string.
// @param[in] summary Event summary supplied in the ICS VEVENT.
// @return Stable frontend key or an empty string for unsupported event summaries.
func normalizeWasteFraction(summary string) string {
	lower := strings.ToLower(summary)
	switch {
	case strings.Contains(lower, "biotonne"):
		return "bio"
	case strings.Contains(lower, "papierabfuhr"):
		return "paper"
	case strings.Contains(lower, "restmüll"), strings.Contains(lower, "restmuell"):
		return "residual"
	case strings.Contains(lower, "gelber sack"), strings.Contains(lower, "gelbe tonne"):
		return "yellow"
	default:
		return ""
	}
}
