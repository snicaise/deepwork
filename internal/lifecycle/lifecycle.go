// Package lifecycle implements `deepwork install` and `deepwork uninstall`.
//
// Both commands must be invoked as root (via `sudo deepwork install/uninstall`).
// Install performs, in order:
//  1. Copy current deepwork + deepwork-apply binaries to /usr/local/bin/
//  2. Write /etc/sudoers.d/deepwork with NOPASSWD for deepwork-apply
//  3. Validate the sudoers file with visudo -cf
//  4. Create ~/.deepwork/, ~/.deepwork/logs/ (owned by $SUDO_USER)
//  5. Seed ~/.deepwork/config.yml if not already present
//  6. Render the LaunchAgent plist into ~/Library/LaunchAgents/ (owned by $SUDO_USER)
//  7. launchctl bootstrap the plist into the user's gui domain
package lifecycle

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/sebastien/deepwork/internal/launchd"
	"github.com/sebastien/deepwork/internal/paths"
)

//go:embed sample_config.yml
var sampleConfig []byte

// Install runs the full install sequence. Must be invoked as root.
func Install() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("install must be run as root: sudo deepwork install")
	}
	u, err := paths.InvokingUser()
	if err != nil {
		return err
	}
	if u.Uid == "0" {
		return fmt.Errorf("install must be invoked via sudo from a non-root user (got SUDO_USER=%q)", os.Getenv("SUDO_USER"))
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	srcBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	srcApply := filepath.Join(filepath.Dir(srcBin), "deepwork-apply")
	if _, err := os.Stat(srcApply); err != nil {
		return fmt.Errorf("deepwork-apply not found next to %s: %w", srcBin, err)
	}

	if err := copyBinary(srcBin, paths.DeepworkBin, 0755); err != nil {
		return err
	}
	if err := copyBinary(srcApply, paths.DeepworkApplyBin, 0755); err != nil {
		return err
	}

	if err := writeSudoers(u.Username); err != nil {
		return err
	}

	dataDir, err := paths.DataDir()
	if err != nil {
		return err
	}
	logDir, _ := paths.LogDir()
	if err := mkdirChown(dataDir, uid, gid, 0755); err != nil {
		return err
	}
	if err := mkdirChown(logDir, uid, gid, 0755); err != nil {
		return err
	}

	cfgPath, _ := paths.ConfigFile()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, sampleConfig, 0644); err != nil {
			return fmt.Errorf("seed config: %w", err)
		}
		if err := os.Chown(cfgPath, uid, gid); err != nil {
			return fmt.Errorf("chown config: %w", err)
		}
	}

	plistPath, err := paths.LaunchAgentPlist()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return err
	}
	_ = os.Chown(filepath.Dir(plistPath), uid, gid)

	logFile, _ := paths.LogFile()
	plist, err := launchd.RenderPlist(launchd.PlistData{
		Label:   paths.PlistLabel,
		Bin:     paths.DeepworkBin,
		LogFile: logFile,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(plistPath, plist, 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	if err := os.Chown(plistPath, uid, gid); err != nil {
		return fmt.Errorf("chown plist: %w", err)
	}

	if err := launchd.Bootout(uid); err != nil {
		// Tolerate: it may not have been loaded before
	}
	if err := launchd.Bootstrap(plistPath, uid); err != nil {
		return fmt.Errorf("bootstrap plist: %w", err)
	}

	return nil
}

// Uninstall reverses Install. Must be invoked as root. Leaves ~/.deepwork/ intact
// so user config survives reinstall.
func Uninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("uninstall must be run as root: sudo deepwork uninstall")
	}
	uid, err := paths.InvokingUID()
	if err != nil {
		return err
	}

	if err := launchd.Bootout(uid); err != nil {
		fmt.Fprintf(os.Stderr, "warning: launchctl bootout: %v\n", err)
	}
	plistPath, err := paths.LaunchAgentPlist()
	if err == nil {
		_ = os.Remove(plistPath)
	}

	if err := exec.Command(paths.DeepworkApplyBin, "clear").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: clear /etc/hosts block: %v\n", err)
	}

	_ = os.Remove(paths.SudoersFile)
	_ = os.Remove(paths.DeepworkBin)
	_ = os.Remove(paths.DeepworkApplyBin)

	return nil
}

func copyBinary(src, dst string, mode os.FileMode) error {
	if src == dst {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("copy %s: %w", dst, err)
	}
	if err := out.Chmod(mode); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", dst, err)
	}
	return nil
}

func writeSudoers(username string) error {
	content := fmt.Sprintf("%s ALL=(root) NOPASSWD: %s\n", username, paths.DeepworkApplyBin)
	tmp := paths.SudoersFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0440); err != nil {
		return fmt.Errorf("write sudoers tmp: %w", err)
	}
	if err := exec.Command("visudo", "-cf", tmp).Run(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("visudo validation failed: %w", err)
	}
	if err := os.Rename(tmp, paths.SudoersFile); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install sudoers: %w", err)
	}
	return nil
}

func mkdirChown(path string, uid, gid int, mode os.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", path, err)
	}
	return nil
}
