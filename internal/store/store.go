// Package store persists the runtime state (enabled/disabled + ephemeral override)
// to ~/.deepwork/runtime.json. Both the user-space CLI and the launchd-fired
// tick read and write this file.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sebastien/deepwork/internal/paths"
)

type State struct {
	Enabled  bool      `json:"enabled"`
	Override *Override `json:"override,omitempty"`
}

type Override struct {
	Until time.Time `json:"until"`
}

// Default is the state used when no file exists yet (post-install, pre-start).
func Default() *State { return &State{Enabled: true} }

// ActiveOverride returns the override if it's still in the future, else nil.
// Automatically clears expired overrides on disk when encountered.
func (s *State) ActiveOverride(now time.Time) *Override {
	if s.Override == nil {
		return nil
	}
	if now.Before(s.Override.Until) {
		return s.Override
	}
	s.Override = nil
	return nil
}

func Path() (string, error) {
	d, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "runtime.json"), nil
}

// Load reads ~/.deepwork/runtime.json. If the file doesn't exist, returns Default().
func Load() (*State, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &s, nil
}

// Save writes the state atomically (temp + rename).
func Save(s *State) error {
	p, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}
