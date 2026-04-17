package launchd

import (
	"strings"
	"testing"
)

func TestRenderPlist(t *testing.T) {
	out, err := RenderPlist(PlistData{
		Label:   "com.deepwork.scheduler",
		Bin:     "/usr/local/bin/deepwork",
		LogFile: "/Users/alice/.deepwork/logs/deepwork.log",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"<key>Label</key>",
		"<string>com.deepwork.scheduler</string>",
		"<string>/usr/local/bin/deepwork</string>",
		"<string>tick</string>",
		"<key>StartInterval</key>",
		"<integer>60</integer>",
		"<key>RunAtLoad</key>",
		"<true/>",
		"<string>/Users/alice/.deepwork/logs/deepwork.log</string>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q:\n%s", want, s)
		}
	}
}
