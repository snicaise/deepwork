package config

import (
	"strings"
	"testing"
	"time"
)

const validYAML = `
timezone: Europe/Paris
sites:
  - linkedin.com
  - www.twitter.com
schedules:
  - name: morning
    days: [mon, tue, wed, thu, fri]
    start: "09:00"
    end: "12:00"
  - name: weekend
    days: [sat, sun]
    start: "10:00"
    end: "11:30"
`

func TestParse_Valid(t *testing.T) {
	c, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if c.Location.String() != "Europe/Paris" {
		t.Errorf("Location = %v, want Europe/Paris", c.Location)
	}
	if len(c.Domains) != 2 {
		t.Errorf("Domains = %d, want 2", len(c.Domains))
	}
	if len(c.Schedules) != 2 {
		t.Fatalf("Schedules = %d, want 2", len(c.Schedules))
	}
	s := c.Schedules[0]
	if s.StartMin != 9*60 || s.EndMin != 12*60 {
		t.Errorf("morning: Start=%d End=%d, want 540/720", s.StartMin, s.EndMin)
	}
	if _, ok := s.DaysSet[time.Monday]; !ok {
		t.Errorf("morning: Monday missing from DaysSet")
	}
	if _, ok := s.DaysSet[time.Sunday]; ok {
		t.Errorf("morning: Sunday should not be in DaysSet")
	}
}

func TestParse_NoTimezone_DefaultsLocal(t *testing.T) {
	y := strings.ReplaceAll(validYAML, "timezone: Europe/Paris\n", "")
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if c.Location != time.Local {
		t.Errorf("Location = %v, want Local", c.Location)
	}
}

func TestParse_Failures(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		msg  string
	}{
		{
			"invalid timezone",
			`timezone: Not/A/Zone` + "\n" + validYAML[len("\ntimezone: Europe/Paris\n"):],
			"invalid timezone",
		},
		{
			"invalid domain",
			strings.Replace(validYAML, "linkedin.com", "not..valid", 1),
			"sites:",
		},
		{
			"unknown day",
			strings.Replace(validYAML, "[mon, tue, wed, thu, fri]", "[mon, xyz]", 1),
			"unknown day",
		},
		{
			"invalid time",
			strings.Replace(validYAML, `"09:00"`, `"9am"`, 1),
			"start",
		},
		{
			"end before start",
			strings.Replace(validYAML, `end: "12:00"`, `end: "08:00"`, 1),
			"strictly after start",
		},
		{
			"empty days",
			strings.Replace(validYAML, "[mon, tue, wed, thu, fri]", "[]", 1),
			"days is required",
		},
		{
			"unknown field",
			validYAML + "extra_field: oops\n",
			"parse YAML",
		},
		{
			"malformed yaml",
			"sites:\n  - linkedin.com\n  bad\n",
			"parse YAML",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.msg)
			}
		})
	}
}

func TestParse_EmptyConfigOK(t *testing.T) {
	c, err := Parse([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Domains) != 0 || len(c.Schedules) != 0 {
		t.Errorf("expected empty config, got %+v", c)
	}
	if c.Location != time.Local {
		t.Errorf("Location = %v, want Local", c.Location)
	}
}

func TestParseHHMM(t *testing.T) {
	good := map[string]int{
		"00:00": 0,
		"09:00": 540,
		"09:30": 570,
		"23:59": 23*60 + 59,
	}
	for s, want := range good {
		got, err := parseHHMM(s)
		if err != nil {
			t.Errorf("parseHHMM(%q): %v", s, err)
		}
		if got != want {
			t.Errorf("parseHHMM(%q) = %d, want %d", s, got, want)
		}
	}
	bad := []string{"", "9", "9:00:00", "24:00", "09:60", "ab:cd", "-1:00"}
	for _, s := range bad {
		if _, err := parseHHMM(s); err == nil {
			t.Errorf("parseHHMM(%q): expected error", s)
		}
	}
}
