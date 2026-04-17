// Package domain validates FQDNs that are safe to write into /etc/hosts.
//
// The rules are deliberately strict — this is the security boundary for
// deepwork-apply, which runs as root via NOPASSWD sudo. Anything that passes
// Parse can be written verbatim into /etc/hosts.
package domain

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

const (
	maxTotalLen = 253
	maxLabelLen = 63
)

// Domain is a validated, normalized (lowercase) ASCII FQDN.
type Domain struct {
	ASCII string
}

// Parse validates s as an FQDN safe for /etc/hosts injection.
// Accepts only ASCII (punycode for IDN), rejects wildcards, IPs, and single labels.
func Parse(s string) (Domain, error) {
	if s == "" {
		return Domain{}, errors.New("empty domain")
	}
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	if strings.HasPrefix(s, "*.") || strings.Contains(s, "*") {
		return Domain{}, fmt.Errorf("wildcards not supported: %q", s)
	}
	if net.ParseIP(s) != nil {
		return Domain{}, fmt.Errorf("IP addresses not allowed: %q", s)
	}
	if len(s) > maxTotalLen {
		return Domain{}, fmt.Errorf("domain exceeds %d chars: %q", maxTotalLen, s)
	}
	if strings.HasPrefix(s, ".") || strings.HasSuffix(s, ".") {
		return Domain{}, fmt.Errorf("leading/trailing dot: %q", s)
	}

	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return Domain{}, fmt.Errorf("must be an FQDN (at least one dot): %q", s)
	}

	for _, label := range labels {
		if err := validateLabel(label); err != nil {
			return Domain{}, fmt.Errorf("invalid label %q in %q: %w", label, s, err)
		}
	}

	if len(labels[len(labels)-1]) < 2 {
		return Domain{}, fmt.Errorf("TLD must be ≥2 chars: %q", s)
	}

	return Domain{ASCII: s}, nil
}

func validateLabel(l string) error {
	if l == "" {
		return errors.New("empty label (consecutive dots?)")
	}
	if len(l) > maxLabelLen {
		return fmt.Errorf("label exceeds %d chars", maxLabelLen)
	}
	if l[0] == '-' || l[len(l)-1] == '-' {
		return errors.New("leading or trailing hyphen")
	}
	for i := 0; i < len(l); i++ {
		c := l[i]
		ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
		if !ok {
			return fmt.Errorf("invalid character %q", c)
		}
	}
	return nil
}

// Variants returns the domain plus the "www." form, deduplicated.
// Consumers feed all variants into /etc/hosts so both foo.com and www.foo.com are blocked.
func (d Domain) Variants() []string {
	if strings.HasPrefix(d.ASCII, "www.") {
		return []string{d.ASCII, strings.TrimPrefix(d.ASCII, "www.")}
	}
	return []string{d.ASCII, "www." + d.ASCII}
}

// ParseAll validates every input. On any failure, returns the first error.
func ParseAll(inputs []string) ([]Domain, error) {
	out := make([]Domain, 0, len(inputs))
	for _, s := range inputs {
		d, err := Parse(s)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
