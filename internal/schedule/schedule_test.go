package schedule

import (
	"testing"
	"time"

	"github.com/sebastien/deepwork/internal/config"
)

func mustConfig(t *testing.T, yaml string) *config.Config {
	t.Helper()
	c, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const basicCfg = `
timezone: UTC
sites: [a.com]
schedules:
  - name: workdays
    days: [mon, tue, wed, thu, fri]
    start: "09:00"
    end: "12:00"
`

func TestIsActive(t *testing.T) {
	c := mustConfig(t, basicCfg)
	tc := []struct {
		when string
		want bool
	}{
		{"2026-04-20T09:00:00Z", true},
		{"2026-04-20T08:59:00Z", false},
		{"2026-04-20T11:59:00Z", true},
		{"2026-04-20T12:00:00Z", false},
		{"2026-04-18T10:00:00Z", false},
		{"2026-04-19T10:00:00Z", false},
	}
	for _, tt := range tc {
		ts, _ := time.Parse(time.RFC3339, tt.when)
		got, _ := IsActive(c, ts)
		if got != tt.want {
			t.Errorf("IsActive(%s) = %v, want %v", tt.when, got, tt.want)
		}
	}
}

func TestIsActive_OverlappingSchedules(t *testing.T) {
	c := mustConfig(t, `
timezone: UTC
sites: [a.com]
schedules:
  - {name: a, days: [mon], start: "09:00", end: "11:00"}
  - {name: b, days: [mon], start: "10:00", end: "12:00"}
`)
	ts, _ := time.Parse(time.RFC3339, "2026-04-20T10:30:00Z")
	active, match := IsActive(c, ts)
	if !active {
		t.Fatal("expected active")
	}
	if match.Name != "a" {
		t.Errorf("first match should be 'a', got %q", match.Name)
	}

	ts2, _ := time.Parse(time.RFC3339, "2026-04-20T11:30:00Z")
	active, match = IsActive(c, ts2)
	if !active || match.Name != "b" {
		t.Errorf("11:30 should match b, got active=%v match=%+v", active, match)
	}
}

func TestNextTransition(t *testing.T) {
	c := mustConfig(t, basicCfg)
	ts, _ := time.Parse(time.RFC3339, "2026-04-20T08:00:00Z")
	next, ok := NextTransition(c, ts)
	if !ok {
		t.Fatal("expected transition")
	}
	if next.Hour() != 9 || next.Minute() != 0 {
		t.Errorf("next = %v, want 09:00", next)
	}

	ts, _ = time.Parse(time.RFC3339, "2026-04-20T10:00:00Z")
	next, ok = NextTransition(c, ts)
	if !ok {
		t.Fatal("expected transition")
	}
	if next.Hour() != 12 || next.Minute() != 0 {
		t.Errorf("next = %v, want 12:00", next)
	}
}

func TestNextTransition_NoSchedules(t *testing.T) {
	c := mustConfig(t, "sites: []")
	_, ok := NextTransition(c, time.Now())
	if ok {
		t.Error("expected no transition with empty schedules")
	}
}

func TestNextTransition_CrossesWeek(t *testing.T) {
	c := mustConfig(t, `
timezone: UTC
sites: [a.com]
schedules:
  - {name: sat, days: [sat], start: "10:00", end: "11:00"}
`)
	ts, _ := time.Parse(time.RFC3339, "2026-04-20T10:00:00Z")
	next, ok := NextTransition(c, ts)
	if !ok {
		t.Fatal("expected transition")
	}
	if next.Weekday() != time.Saturday || next.Hour() != 10 {
		t.Errorf("next = %v, want Saturday 10:00", next)
	}
}

func TestIsActive_TimezoneAware(t *testing.T) {
	c := mustConfig(t, `
timezone: Europe/Paris
sites: [a.com]
schedules:
  - {name: morning, days: [mon], start: "09:00", end: "12:00"}
`)
	utc, _ := time.Parse(time.RFC3339, "2026-04-20T08:00:00Z")
	active, _ := IsActive(c, utc)
	if !active {
		t.Error("08:00 UTC = 10:00 Paris (CEST) should be active on Monday")
	}
	utc2, _ := time.Parse(time.RFC3339, "2026-04-20T06:00:00Z")
	active, _ = IsActive(c, utc2)
	if active {
		t.Error("06:00 UTC = 08:00 Paris should NOT be active")
	}
}

func TestIsActive_DSTSpringForward(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	c := mustConfig(t, `
timezone: Europe/Paris
sites: [a.com]
schedules:
  - {name: s, days: [sun], start: "04:00", end: "06:00"}
`)
	before := time.Date(2026, 3, 29, 1, 30, 0, 0, paris)
	active, _ := IsActive(c, before)
	if active {
		t.Error("01:30 Paris on DST day should not match 04:00-06:00 window")
	}
	after := time.Date(2026, 3, 29, 4, 30, 0, 0, paris)
	active, _ = IsActive(c, after)
	if !active {
		t.Error("04:30 Paris on DST day should match 04:00-06:00 window")
	}
}
