package state

import (
	"slices"
	"testing"
	"time"

	"github.com/sebastien/deepwork/internal/config"
)

func mustCfg(t *testing.T, y string) *config.Config {
	t.Helper()
	c, err := config.Parse([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const cfgYAML = `
timezone: UTC
sites: [linkedin.com, www.twitter.com]
schedules:
  - {name: work, days: [mon], start: "09:00", end: "12:00"}
`

func TestDesired_Inactive(t *testing.T) {
	c := mustCfg(t, cfgYAML)
	ts, _ := time.Parse(time.RFC3339, "2026-04-20T08:00:00Z")
	if got := Desired(c, ts, nil); len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

func TestDesired_ActiveExpands(t *testing.T) {
	c := mustCfg(t, cfgYAML)
	ts, _ := time.Parse(time.RFC3339, "2026-04-20T10:00:00Z")
	got := Desired(c, ts, nil)
	want := []string{"linkedin.com", "www.linkedin.com", "www.twitter.com", "twitter.com"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("Desired:\n got = %v\nwant = %v", got, want)
	}
}

func TestDesired_OverrideForcesActive(t *testing.T) {
	c := mustCfg(t, cfgYAML)
	ts, _ := time.Parse(time.RFC3339, "2026-04-20T08:00:00Z")
	ov := &Override{Until: ts.Add(1 * time.Hour)}
	got := Desired(c, ts, ov)
	if len(got) == 0 {
		t.Error("override should force blocking")
	}
}

func TestDesired_OverrideExpired(t *testing.T) {
	c := mustCfg(t, cfgYAML)
	ts, _ := time.Parse(time.RFC3339, "2026-04-20T08:00:00Z")
	ov := &Override{Until: ts.Add(-1 * time.Hour)}
	if got := Desired(c, ts, ov); len(got) != 0 {
		t.Errorf("expired override should not block, got %v", got)
	}
}

func TestDesired_ScheduleAndOverrideBothActive(t *testing.T) {
	c := mustCfg(t, cfgYAML)
	ts, _ := time.Parse(time.RFC3339, "2026-04-20T10:00:00Z")
	ov := &Override{Until: ts.Add(1 * time.Hour)}
	got := Desired(c, ts, ov)
	if len(got) != 4 {
		t.Errorf("want 4 entries, got %d: %v", len(got), got)
	}
}
