// Package hosts manages /etc/hosts block injection between marker lines.
//
// All writes go through a flock on /var/run/deepwork-apply.lock and an atomic
// temp-file-then-rename to prevent partial-state corruption on crash.
package hosts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const (
	MarkerStart = "# DEEPWORK_START"
	MarkerEnd   = "# DEEPWORK_END"
	LockPath    = "/var/run/deepwork-apply.lock"
	BlockIP     = "0.0.0.0"
	BlockIP6    = "::"
)

// Apply ensures hostsPath contains a deepwork block with exactly the given domains.
// Existing deepwork block (if any) is replaced. Lines outside the markers are preserved.
// Idempotent: running twice with the same input yields the same file.
func Apply(hostsPath string, domains []string) error {
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	return rewrite(hostsPath, domains)
}

// Clear removes the deepwork block from hostsPath entirely. Idempotent.
func Clear(hostsPath string) error {
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	return rewrite(hostsPath, nil)
}

// ExistingBlock returns the domains that are fully blocked (both IPv4 and IPv6
// lines present) inside the DEEPWORK markers of hostsPath. A domain with only
// one of the two is treated as not-yet-applied so reconcile will rewrite.
// Safe to call from user-space (read-only).
func ExistingBlock(hostsPath string) ([]string, error) {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return nil, err
	}
	v4 := map[string]bool{}
	v6 := map[string]bool{}
	inBlock := false
	for _, rawLine := range bytes.Split(content, []byte("\n")) {
		line := strings.TrimSpace(string(rawLine))
		if line == MarkerStart {
			inBlock = true
			continue
		}
		if line == MarkerEnd {
			break
		}
		if !inBlock || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case BlockIP:
			v4[fields[1]] = true
		case BlockIP6:
			v6[fields[1]] = true
		}
	}
	var out []string
	for d := range v4 {
		if v6[d] {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out, nil
}

func rewrite(hostsPath string, domains []string) error {
	orig, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", hostsPath, err)
	}
	info, err := os.Stat(hostsPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", hostsPath, err)
	}

	clean := stripBlock(orig)
	var out bytes.Buffer
	out.Write(clean)
	if !bytes.HasSuffix(clean, []byte("\n")) && len(clean) > 0 {
		out.WriteByte('\n')
	}
	if len(domains) > 0 {
		writeBlock(&out, domains)
	}

	return atomicWrite(hostsPath, out.Bytes(), info.Mode().Perm())
}

func stripBlock(content []byte) []byte {
	lines := bytes.Split(content, []byte("\n"))
	var kept [][]byte
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if trimmed == MarkerStart {
			inBlock = true
			continue
		}
		if trimmed == MarkerEnd {
			inBlock = false
			continue
		}
		if inBlock {
			continue
		}
		kept = append(kept, line)
	}
	return bytes.Join(kept, []byte("\n"))
}

func writeBlock(buf *bytes.Buffer, domains []string) {
	sorted := make([]string, len(domains))
	copy(sorted, domains)
	sort.Strings(sorted)

	buf.WriteString(MarkerStart)
	buf.WriteByte('\n')
	for _, d := range sorted {
		fmt.Fprintf(buf, "%s %s\n", BlockIP, d)
		fmt.Fprintf(buf, "%s %s\n", BlockIP6, d)
	}
	buf.WriteString(MarkerEnd)
	buf.WriteByte('\n')
}

func atomicWrite(target string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".deepwork-hosts-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("fsync temp: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		cleanup()
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

func acquireLock() (func(), error) {
	f, err := os.OpenFile(LockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock %s: %w", LockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("flock %s: %w", LockPath, err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
