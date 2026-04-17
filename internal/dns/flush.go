// Package dns flushes the macOS DNS resolver cache.
//
// All commands require root — invoke this from deepwork-apply, not from
// user-space code.
package dns

import (
	"fmt"
	"os/exec"
)

// Flush runs dscacheutil + killall -HUP mDNSResponder, then restarts the
// WebKit network process so Safari (and any app embedding a WKWebView) drops
// its in-process DNS cache. Without this, Safari keeps resolving to the old IP
// even after /etc/hosts changes.
//
// Non-zero exit from dscacheutil or mDNSResponder is fatal. The WebKit kill is
// best-effort — if no WebKit.Networking process is running, killall returns 1
// and we swallow it.
func Flush() error {
	if err := exec.Command("dscacheutil", "-flushcache").Run(); err != nil {
		return fmt.Errorf("dscacheutil -flushcache: %w", err)
	}
	if err := exec.Command("killall", "-HUP", "mDNSResponder").Run(); err != nil {
		return fmt.Errorf("killall -HUP mDNSResponder: %w", err)
	}
	_ = exec.Command("killall", "-9", "com.apple.WebKit.Networking").Run()
	return nil
}
