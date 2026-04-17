// Package launchd generates the deepwork LaunchAgent plist and wraps
// launchctl bootstrap/bootout/kickstart.
package launchd

import (
	"bytes"
	"fmt"
	"os/exec"
	"text/template"

	"github.com/sebastien/deepwork/internal/paths"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Bin}}</string>
		<string>tick</string>
	</array>
	<key>StartInterval</key>
	<integer>60</integer>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.LogFile}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogFile}}</string>
</dict>
</plist>
`

type PlistData struct {
	Label   string
	Bin     string
	LogFile string
}

// RenderPlist produces the XML plist for the deepwork LaunchAgent.
func RenderPlist(d PlistData) ([]byte, error) {
	t, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Bootstrap loads the plist into the given user's gui launchd domain.
// Must be called as root or as that user.
func Bootstrap(plistPath string, uid int) error {
	return run("bootstrap", fmt.Sprintf("gui/%d", uid), plistPath)
}

// Bootout unloads the deepwork agent. Tolerates "not loaded" error.
func Bootout(uid int) error {
	err := run("bootout", fmt.Sprintf("gui/%d/%s", uid, paths.PlistLabel))
	if err != nil && isNotLoadedErr(err) {
		return nil
	}
	return err
}

// Kickstart forces the agent to run immediately. -k kills and restarts if running.
func Kickstart(uid int) error {
	return run("kickstart", "-k", fmt.Sprintf("gui/%d/%s", uid, paths.PlistLabel))
}

func run(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %v: %w (output: %s)", args, err, bytes.TrimSpace(out))
	}
	return nil
}

func isNotLoadedErr(err error) bool {
	s := err.Error()
	return bytes.Contains([]byte(s), []byte("Could not find specified service")) ||
		bytes.Contains([]byte(s), []byte("No such process"))
}
