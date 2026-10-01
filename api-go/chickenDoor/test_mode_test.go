package chickendoor

import (
	"testing"
	"time"
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
