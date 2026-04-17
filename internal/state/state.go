// Package state computes the set of domains that should be present in /etc/hosts
// for a given config, time, and optional ephemeral override.
package state

import (
	"time"

	"github.com/sebastien/deepwork/internal/config"
	"github.com/sebastien/deepwork/internal/domain"
	"github.com/sebastien/deepwork/internal/schedule"
)

// Override is a user-triggered "block now until X" directive set by `deepwork now <dur>`.
// It forces the blocked state regardless of schedules while time.Now() < Until.
type Override struct {
	Until time.Time
}

// Desired returns the full list of domain strings that should appear in /etc/hosts
// (each domain expanded into its variants, e.g. both foo.com and www.foo.com).
// An empty slice means no block.
func Desired(c *config.Config, now time.Time, ov *Override) []string {
	active, _ := schedule.IsActive(c, now)
	if !active && ov != nil && now.Before(ov.Until) {
		active = true
	}
	if !active {
		return nil
	}
	return expand(c.Domains)
}

func expand(domains []domain.Domain) []string {
	seen := make(map[string]struct{}, len(domains)*2)
	out := make([]string, 0, len(domains)*2)
	for _, d := range domains {
		for _, v := range d.Variants() {
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}
