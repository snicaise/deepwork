package domain

import (
	"slices"
	"strings"
	"testing"
)

func TestParse_Valid(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"linkedin.com", "linkedin.com"},
		{"www.linkedin.com", "www.linkedin.com"},
		{"sub.sub.example.com", "sub.sub.example.com"},
		{"X.COM", "x.com"},
		{"  twitter.com  ", "twitter.com"},
		{"news.ycombinator.com", "news.ycombinator.com"},
		{"xn--nxasmq6b.com", "xn--nxasmq6b.com"},
		{"a.ai", "a.ai"},
		{"foo.local", "foo.local"},
		{"foo-bar.example.com", "foo-bar.example.com"},
	}
	for _, tc := range cases {
		d, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if d.ASCII != tc.want {
			t.Errorf("Parse(%q) = %q, want %q", tc.in, d.ASCII, tc.want)
		}
	}
}

func TestParse_Invalid(t *testing.T) {
	cases := []string{
		"",
		".",
		".com",
		"com.",
		"a..b",
		"-foo.com",
		"foo-.com",
		"foo_bar.com",
		"foo bar.com",
		"café.com",
		"*.example.com",
		"*",
		"foo.*",
		"1.2.3.4",
		"::1",
		"localhost",
		"foo",
		"foo.c",
		strings.Repeat("a", 64) + ".com",
		strings.Repeat("a.", 128) + "com",
	}
	for _, in := range cases {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q): expected error, got nil", in)
		}
	}
}

func TestVariants(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"linkedin.com", []string{"linkedin.com", "www.linkedin.com"}},
		{"www.linkedin.com", []string{"www.linkedin.com", "linkedin.com"}},
		{"mail.google.com", []string{"mail.google.com", "www.mail.google.com"}},
	}
	for _, tc := range cases {
		d, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.in, err)
		}
		got := d.Variants()
		if !slices.Equal(got, tc.want) {
			t.Errorf("Variants(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseAll_PropagatesFirstError(t *testing.T) {
	_, err := ParseAll([]string{"good.com", "bad..domain", "also-good.com"})
	if err == nil {
		t.Fatal("expected error from middle element")
	}
	if !strings.Contains(err.Error(), "bad..domain") {
		t.Errorf("error should mention the bad input, got: %v", err)
	}
}
