package doctor

import (
	"strings"
	"testing"
)

func TestTrrModeRegex(t *testing.T) {
	cases := map[string]string{
		`user_pref("network.trr.mode", 2);`:   "2",
		`user_pref("network.trr.mode", 3);`:   "3",
		`user_pref("network.trr.mode",5);`:    "5",
		`user_pref("network.trr.mode",  0 );`: "0",
		`user_pref("network.trr.modex", 2);`:  "",
		`user_pref("network.trr.mode", "2");`: "",
		`user_pref("something.else", 2);`:     "",
	}
	for in, want := range cases {
		m := trrModeRE.FindStringSubmatch(in)
		got := ""
		if len(m) > 1 {
			got = m[1]
		}
		if got != want {
			t.Errorf("input %q: got %q, want %q", in, got, want)
		}
	}
}

func TestRenderWarnings(t *testing.T) {
	if RenderWarnings(nil) != "" {
		t.Error("empty warnings should produce empty string")
	}
	s := RenderWarnings([]Probe{
		{Browser: "Firefox", Installed: true, DoHActive: true},
		{Browser: "Chrome", Installed: true, DoHActive: true},
	})
	if !strings.Contains(s, "Firefox") || !strings.Contains(s, "Chrome") {
		t.Errorf("expected both browsers in warning, got %q", s)
	}
}

func TestRenderFull_SkipsNotInstalled(t *testing.T) {
	out := RenderFull([]Probe{
		{Browser: "Firefox", Installed: false},
		{Browser: "Chrome", Installed: true, DoHActive: false, Mode: "off"},
		{Browser: "Edge", Installed: true, DoHActive: true, Mode: "secure", Remediation: "turn it off"},
	})
	if strings.Contains(out, "Firefox") {
		t.Errorf("not-installed Firefox should be hidden:\n%s", out)
	}
	if !strings.Contains(out, "Chrome") || !strings.Contains(out, "Edge") {
		t.Errorf("installed browsers should appear:\n%s", out)
	}
	if !strings.Contains(out, "turn it off") {
		t.Errorf("remediation should appear for active DoH:\n%s", out)
	}
}

func TestRenderFull_NoInstalled(t *testing.T) {
	out := RenderFull([]Probe{{Browser: "Firefox", Installed: false}})
	if !strings.Contains(out, "no browsers detected") {
		t.Errorf("expected empty-state message:\n%s", out)
	}
}
