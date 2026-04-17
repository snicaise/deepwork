// Package doctor inspects installed browsers for DNS-over-HTTPS settings.
//
// DoH bypasses the system resolver and /etc/hosts, so if a browser has DoH on
// deepwork's block is silently ineffective for that browser. We warn but never
// fail — a non-zero exit would break scripts that rely on `deepwork status`.
//
// Design choice (from plan A.2.3): for Chromium-family browsers we only warn
// on explicit "secure" mode. "automatic" on stock macOS effectively means no
// DoH, so flagging it would produce false positives for ~95% of users.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sebastien/deepwork/internal/paths"
)

// Probe is the result of inspecting one browser.
type Probe struct {
	Browser     string
	Installed   bool
	DoHActive   bool
	Mode        string
	Remediation string
}

// CheckAll runs every probe and returns results in stable order.
// Non-installed browsers are reported with Installed=false — callers should skip them when rendering.
func CheckAll() []Probe {
	return []Probe{
		checkFirefox(),
		checkChromium("Chrome", "Google Chrome.app", "com.google.Chrome",
			"Settings → Privacy and security → Security → Use secure DNS: Off"),
		checkChromium("Edge", "Microsoft Edge.app", "com.microsoft.Edge",
			"Settings → Privacy, search, and services → Security → Use secure DNS: Off"),
		checkChromium("Brave", "Brave Browser.app", "com.brave.Browser",
			"Settings → Privacy and security → Security → Use secure DNS: Off"),
		checkChromium("Arc", "Arc.app", "com.thebrowser.Browser",
			"Arc Settings → Profiles → Use secure DNS: Off (or see chrome://settings/security)"),
	}
}

// Warnings returns only the probes for installed browsers with DoH active.
func Warnings() []Probe {
	var out []Probe
	for _, p := range CheckAll() {
		if p.Installed && p.DoHActive {
			out = append(out, p)
		}
	}
	return out
}

var trrModeRE = regexp.MustCompile(`user_pref\("network\.trr\.mode",\s*(\d+)\s*\)`)

func checkFirefox() Probe {
	p := Probe{
		Browser:     "Firefox",
		Remediation: "Open about:config, set network.trr.mode to 5 (disables TRR)",
	}
	home, err := paths.UserHome()
	if err != nil {
		return p
	}
	profilesDir := filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
	if _, err := os.Stat(profilesDir); err != nil {
		return p
	}
	p.Installed = true

	profiles, err := os.ReadDir(profilesDir)
	if err != nil {
		return p
	}

	// user.js overrides prefs.js on every Firefox startup — scan user.js last so it wins.
	for _, prof := range profiles {
		if !prof.IsDir() {
			continue
		}
		for _, name := range []string{"prefs.js", "user.js"} {
			mode := readFirefoxTRR(filepath.Join(profilesDir, prof.Name(), name))
			if mode != "" {
				p.Mode = mode
				p.DoHActive = mode == "2" || mode == "3"
			}
		}
	}
	return p
}

func readFirefoxTRR(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	m := trrModeRE.FindSubmatch(b)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func checkChromium(browserName, appBundle, defaultsDomain, remediation string) Probe {
	p := Probe{Browser: browserName, Remediation: remediation}
	if !chromiumInstalled(appBundle, defaultsDomain) {
		return p
	}
	p.Installed = true
	mode := readDefaults(defaultsDomain, "DnsOverHttpsMode")
	p.Mode = mode
	// Per plan A.2.3: warn only on explicit "secure". "automatic" is noise on stock macOS.
	p.DoHActive = mode == "secure"
	return p
}

func chromiumInstalled(appBundle, defaultsDomain string) bool {
	home, _ := paths.UserHome()
	candidates := []string{
		filepath.Join("/Applications", appBundle),
		filepath.Join(home, "Applications", appBundle),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	// Fallback: if defaults domain is registered, the browser has been run at least once.
	return defaultsDomainExists(defaultsDomain)
}

func defaultsDomainExists(domain string) bool {
	out, err := exec.Command("defaults", "domains").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), domain)
}

func readDefaults(domain, key string) string {
	out, err := exec.Command("defaults", "read", domain, key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// RenderWarnings formats a short one-liner suitable for `deepwork status`.
// Returns "" if no warnings.
func RenderWarnings(ps []Probe) string {
	if len(ps) == 0 {
		return ""
	}
	names := make([]string, 0, len(ps))
	for _, p := range ps {
		names = append(names, p.Browser)
	}
	return fmt.Sprintf("⚠  DoH active on %s — blocking may be bypassed (run `deepwork doctor`)", strings.Join(names, ", "))
}

// RenderFull formats the full doctor output with per-browser sections.
func RenderFull(ps []Probe) string {
	var sb strings.Builder
	sb.WriteString("deepwork doctor — DNS-over-HTTPS (DoH) diagnostic\n\n")
	any := false
	for _, p := range ps {
		if !p.Installed {
			continue
		}
		any = true
		marker := "✓"
		if p.DoHActive {
			marker = "⚠"
		}
		modeStr := p.Mode
		if modeStr == "" {
			modeStr = "(unset)"
		}
		fmt.Fprintf(&sb, "%s %-8s mode=%s doh=%v\n", marker, p.Browser, modeStr, p.DoHActive)
		if p.DoHActive {
			fmt.Fprintf(&sb, "    → %s\n", p.Remediation)
		}
	}
	if !any {
		sb.WriteString("(no browsers detected)\n")
	}
	sb.WriteString("\nSafari: uses the system resolver, no DoH, not affected.\n")
	return sb.String()
}
