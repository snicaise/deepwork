package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/sebastien/deepwork/internal/config"
	"github.com/sebastien/deepwork/internal/doctor"
	"github.com/sebastien/deepwork/internal/launchd"
	"github.com/sebastien/deepwork/internal/lifecycle"
	"github.com/sebastien/deepwork/internal/paths"
	"github.com/sebastien/deepwork/internal/reconcile"
	"github.com/sebastien/deepwork/internal/schedule"
	"github.com/sebastien/deepwork/internal/store"
)

func cmdInstall() error   { return lifecycle.Install() }
func cmdUninstall() error { return lifecycle.Uninstall() }

func cmdStart() error {
	s, err := store.Load()
	if err != nil {
		return err
	}
	s.Enabled = true
	if err := store.Save(s); err != nil {
		return err
	}
	if err := kickstart(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	fmt.Println("scheduling enabled")
	return nil
}

func cmdStop() error {
	s, err := store.Load()
	if err != nil {
		return err
	}
	s.Enabled = false
	s.Override = nil
	if err := store.Save(s); err != nil {
		return err
	}
	if err := kickstart(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	fmt.Println("scheduling disabled")
	return nil
}

func cmdNow(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: deepwork now <duration> (e.g. 25m, 2h, 1h30m)")
	}
	d, err := time.ParseDuration(args[0])
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", args[0], err)
	}
	if d <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	s, err := store.Load()
	if err != nil {
		return err
	}
	until := time.Now().Add(d)
	s.Override = &store.Override{Until: until}
	if err := store.Save(s); err != nil {
		return err
	}
	if err := kickstart(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	fmt.Printf("blocking for %s (until %s)\n", d, until.Format("15:04"))
	return nil
}

func cmdEdit() error {
	cfgPath, err := paths.ConfigFile()
	if err != nil {
		return err
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, cfgPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited non-zero: %w", err)
	}
	if _, err := config.Load(cfgPath); err != nil {
		fmt.Fprintln(os.Stderr, "warning: config has errors and will be ignored by the daemon:")
		fmt.Fprintln(os.Stderr, " ", err)
		return nil
	}
	if err := kickstart(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	fmt.Println("config validated, daemon kickstarted")
	return nil
}

func cmdTick() error {
	cfgPath, err := paths.ConfigFile()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config invalid, skipping tick:", err)
		return nil
	}
	s, err := store.Load()
	if err != nil {
		return err
	}
	r, err := reconcile.Once(cfg, s, time.Now())
	if err != nil {
		return err
	}
	if r.Changed {
		fmt.Printf("tick: %s (%d domains)\n", r.Action, len(r.Desired))
	}
	// Persist state mutation from expired override clearing
	return store.Save(s)
}

func cmdStatus() error {
	cfgPath, _ := paths.ConfigFile()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	s, err := store.Load()
	if err != nil {
		return err
	}
	now := time.Now()

	if !s.Enabled {
		fmt.Println("○ Disabled (run `deepwork start` to re-enable)")
	} else if ov := s.ActiveOverride(now); ov != nil {
		fmt.Printf("● Active (override) — until %s\n", ov.Until.Format("15:04"))
	} else if active, sched := schedule.IsActive(cfg, now); active {
		endWall := time.Date(now.Year(), now.Month(), now.Day(), sched.EndMin/60, sched.EndMin%60, 0, 0, cfg.Location)
		fmt.Printf("● Active — %q (until %s)\n", sched.Name, endWall.Format("15:04"))
	} else if next, ok := schedule.NextTransition(cfg, now); ok {
		fmt.Printf("○ Inactive — next window at %s\n", next.Format("Mon 15:04"))
	} else {
		fmt.Println("○ Inactive — no schedules configured")
	}
	fmt.Printf("  %d sites blocked, %d schedules configured\n", len(cfg.Domains), len(cfg.Schedules))
	if warn := doctor.RenderWarnings(doctor.Warnings()); warn != "" {
		fmt.Println()
		fmt.Println(warn)
	}
	return nil
}

func cmdDoctor() error {
	fmt.Print(doctor.RenderFull(doctor.CheckAll()))
	return nil
}

func kickstart() error {
	uid, err := paths.InvokingUID()
	if err != nil {
		return err
	}
	return launchd.Kickstart(uid)
}
