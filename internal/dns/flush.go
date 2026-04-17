// Package dns flushes the macOS DNS resolver cache.
//
// Both commands require root — invoke this from deepwork-apply, not from
// user-space code.
package dns

import (
	"fmt"
	"os/exec"
)

// Flush runs dscacheutil + killall -HUP mDNSResponder.
// Non-zero exit from either is returned as an error.
func Flush() error {
	if err := exec.Command("dscacheutil", "-flushcache").Run(); err != nil {
		return fmt.Errorf("dscacheutil -flushcache: %w", err)
	}
	if err := exec.Command("killall", "-HUP", "mDNSResponder").Run(); err != nil {
		return fmt.Errorf("killall -HUP mDNSResponder: %w", err)
	}
	return nil
}
