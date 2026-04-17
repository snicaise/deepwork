// Package paths centralizes every filesystem path deepwork touches.
//
// When deepwork runs under sudo, UserHome / DataDir etc. resolve to the
// invoking user ($SUDO_USER), not to root. This is what lets `sudo deepwork
// install` seed ~/.deepwork in the user's home rather than /var/root.
package paths

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

const (
	BinDir           = "/usr/local/bin"
	DeepworkBin      = "/usr/local/bin/deepwork"
	DeepworkApplyBin = "/usr/local/bin/deepwork-apply"
	SudoersFile      = "/etc/sudoers.d/deepwork"
	HostsFile        = "/etc/hosts"

	PlistLabel    = "com.deepwork.scheduler"
	PlistFileName = "com.deepwork.scheduler.plist"
)

// InvokingUser returns the user that initiated the command.
// When running via sudo, this is $SUDO_USER; otherwise the current user.
func InvokingUser() (*user.User, error) {
	if s := os.Getenv("SUDO_USER"); s != "" {
		u, err := user.Lookup(s)
		if err != nil {
			return nil, fmt.Errorf("lookup SUDO_USER=%q: %w", s, err)
		}
		return u, nil
	}
	return user.Current()
}

// InvokingUID returns the numeric UID of the invoking user.
func InvokingUID() (int, error) {
	u, err := InvokingUser()
	if err != nil {
		return 0, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf("parse uid %q: %w", u.Uid, err)
	}
	return uid, nil
}

func UserHome() (string, error) {
	u, err := InvokingUser()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

func DataDir() (string, error) {
	home, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".deepwork"), nil
}

func ConfigFile() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.yml"), nil
}

func OverrideFile() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "override.json"), nil
}

func LogDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "logs"), nil
}

func LogFile() (string, error) {
	d, err := LogDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "deepwork.log"), nil
}

func LaunchAgentPlist() (string, error) {
	home, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", PlistFileName), nil
}
