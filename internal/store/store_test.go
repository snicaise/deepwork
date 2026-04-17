package store

import (
	"testing"
	"time"
)

func TestActiveOverride(t *testing.T) {
	now := time.Now()
	s := &State{Enabled: true, Override: &Override{Until: now.Add(time.Hour)}}
	if s.ActiveOverride(now) == nil {
		t.Error("expected active override")
	}

	s = &State{Enabled: true, Override: &Override{Until: now.Add(-time.Hour)}}
	if s.ActiveOverride(now) != nil {
		t.Error("expected expired override to return nil")
	}
	if s.Override != nil {
		t.Error("expired override should be cleared")
	}

	s = &State{Enabled: true}
	if s.ActiveOverride(now) != nil {
		t.Error("no override should return nil")
	}
}
