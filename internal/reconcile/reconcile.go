// Package reconcile is the tick-time logic: read config, read runtime state,
// compute desired /etc/hosts block, compare to actual, and shell to
// `sudo deepwork-apply` only when reconciliation is needed.
package reconcile

import (
	"fmt"
	"os/exec"
	"slices"
	"time"

	"github.com/sebastien/deepwork/internal/config"
	"github.com/sebastien/deepwork/internal/hosts"
	"github.com/sebastien/deepwork/internal/paths"
	"github.com/sebastien/deepwork/internal/state"
	"github.com/sebastien/deepwork/internal/store"
)

// Result describes what the tick decided and did.
type Result struct {
	Desired []string
	Current []string
	Changed bool
	Action  string // "apply", "clear", or "noop"
}

// Once runs one reconciliation pass. Returns nil if either everything is
// already in sync or if the apply succeeded.
func Once(cfg *config.Config, st *store.State, now time.Time) (Result, error) {
	var desired []string
	if st.Enabled {
		desired = state.Desired(cfg, now, toStateOverride(st.ActiveOverride(now)))
	}
	current, err := hosts.ExistingBlock(paths.HostsFile)
	if err != nil {
		return Result{}, fmt.Errorf("read /etc/hosts: %w", err)
	}
	slices.Sort(desired)
	slices.Sort(current)

	r := Result{Desired: desired, Current: current}
	if slices.Equal(desired, current) {
		r.Action = "noop"
		return r, nil
	}
	r.Changed = true

	if len(desired) == 0 {
		r.Action = "clear"
		return r, run("clear")
	}
	r.Action = "apply"
	args := append([]string{"apply"}, desired...)
	return r, run(args...)
}

func toStateOverride(o *store.Override) *state.Override {
	if o == nil {
		return nil
	}
	return &state.Override{Until: o.Until}
}

// run invokes `sudo -n /usr/local/bin/deepwork-apply <args>`.
// -n fails fast instead of prompting — NOPASSWD makes the call succeed silently.
func run(args ...string) error {
	full := append([]string{"-n", paths.DeepworkApplyBin}, args...)
	cmd := exec.Command("sudo", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo deepwork-apply %v: %w (output: %s)", args, err, out)
	}
	return nil
}
