// Package config parses, validates, and resolves the deepwork YAML config.
//
// The config file lives at ~/.deepwork/config.yml and is re-read by the daemon
// on every tick (no fsnotify). Invalid configs are rejected at Load time with
// a user-actionable error.
package config

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sebastien/deepwork/internal/domain"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Timezone  string     `yaml:"timezone,omitempty"`
	Sites     []string   `yaml:"sites"`
	Schedules []Schedule `yaml:"schedules"`

	Location *time.Location  `yaml:"-"`
	Domains  []domain.Domain `yaml:"-"`
}

type Schedule struct {
	Name  string   `yaml:"name"`
	Days  []string `yaml:"days"`
	Start string   `yaml:"start"`
	End   string   `yaml:"end"`

	DaysSet  map[time.Weekday]struct{} `yaml:"-"`
	StartMin int                       `yaml:"-"`
	EndMin   int                       `yaml:"-"`
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday,
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
}

// Load reads, parses, and fully resolves the config at path.
// Returned Config has Location and Domains populated, and every Schedule has
// its DaysSet / StartMin / EndMin computed.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(raw)
}

// Parse is Load without file I/O — useful for tests.
// An all-whitespace input is treated as an empty-but-valid config.
func Parse(raw []byte) (*Config, error) {
	var c Config
	if len(bytes.TrimSpace(raw)) > 0 {
		dec := yaml.NewDecoder(bytes.NewReader(raw))
		dec.KnownFields(true)
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
	}
	if err := c.resolve(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) resolve() error {
	if c.Timezone == "" {
		c.Location = time.Local
	} else {
		loc, err := time.LoadLocation(c.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", c.Timezone, err)
		}
		c.Location = loc
	}

	c.Domains = make([]domain.Domain, 0, len(c.Sites))
	for _, s := range c.Sites {
		d, err := domain.Parse(s)
		if err != nil {
			return fmt.Errorf("sites: %w", err)
		}
		c.Domains = append(c.Domains, d)
	}

	for i := range c.Schedules {
		if err := c.Schedules[i].resolve(); err != nil {
			return fmt.Errorf("schedule %q: %w", c.Schedules[i].Name, err)
		}
	}
	return nil
}

func (s *Schedule) resolve() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if len(s.Days) == 0 {
		return fmt.Errorf("days is required")
	}
	s.DaysSet = make(map[time.Weekday]struct{}, len(s.Days))
	for _, d := range s.Days {
		wd, ok := weekdays[strings.ToLower(strings.TrimSpace(d))]
		if !ok {
			return fmt.Errorf("unknown day %q (use mon,tue,wed,thu,fri,sat,sun)", d)
		}
		s.DaysSet[wd] = struct{}{}
	}

	start, err := parseHHMM(s.Start)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	end, err := parseHHMM(s.End)
	if err != nil {
		return fmt.Errorf("end: %w", err)
	}
	if end <= start {
		return fmt.Errorf("end %q must be strictly after start %q (cross-midnight windows are not supported in V1)", s.End, s.Start)
	}
	s.StartMin = start
	s.EndMin = end
	return nil
}

func parseHHMM(s string) (int, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected HH:MM, got %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute in %q", s)
	}
	return h*60 + m, nil
}
