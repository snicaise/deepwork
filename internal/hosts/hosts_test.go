package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const baseHosts = `##
# Host Database
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
`

func setupHosts(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readHosts(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The real Apply/Clear acquire a lock at /var/run/deepwork-apply.lock which
// requires root. Tests exercise rewrite() directly, which contains the interesting logic.

func TestRewrite_InsertsBlock(t *testing.T) {
	path := setupHosts(t, baseHosts)
	if err := rewrite(path, []string{"linkedin.com", "www.linkedin.com"}); err != nil {
		t.Fatal(err)
	}
	got := readHosts(t, path)
	if !strings.Contains(got, MarkerStart) || !strings.Contains(got, MarkerEnd) {
		t.Errorf("markers missing:\n%s", got)
	}
	if !strings.Contains(got, "0.0.0.0 linkedin.com") {
		t.Errorf("expected linkedin.com line, got:\n%s", got)
	}
	if !strings.Contains(got, "127.0.0.1\tlocalhost") {
		t.Errorf("original content not preserved:\n%s", got)
	}
}

func TestRewrite_Idempotent(t *testing.T) {
	path := setupHosts(t, baseHosts)
	domains := []string{"a.com", "b.com"}
	if err := rewrite(path, domains); err != nil {
		t.Fatal(err)
	}
	first := readHosts(t, path)
	if err := rewrite(path, domains); err != nil {
		t.Fatal(err)
	}
	second := readHosts(t, path)
	if first != second {
		t.Errorf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestRewrite_ReplacesExistingBlock(t *testing.T) {
	path := setupHosts(t, baseHosts)
	if err := rewrite(path, []string{"old.com"}); err != nil {
		t.Fatal(err)
	}
	if err := rewrite(path, []string{"new.com"}); err != nil {
		t.Fatal(err)
	}
	got := readHosts(t, path)
	if strings.Contains(got, "old.com") {
		t.Errorf("old domain still present:\n%s", got)
	}
	if !strings.Contains(got, "new.com") {
		t.Errorf("new domain missing:\n%s", got)
	}
	if strings.Count(got, MarkerStart) != 1 || strings.Count(got, MarkerEnd) != 1 {
		t.Errorf("expected exactly one block, got:\n%s", got)
	}
}

func TestRewrite_ClearRemovesBlockPreservesRest(t *testing.T) {
	path := setupHosts(t, baseHosts)
	if err := rewrite(path, []string{"foo.com"}); err != nil {
		t.Fatal(err)
	}
	if err := rewrite(path, nil); err != nil {
		t.Fatal(err)
	}
	got := readHosts(t, path)
	if strings.Contains(got, MarkerStart) || strings.Contains(got, MarkerEnd) {
		t.Errorf("markers still present after clear:\n%s", got)
	}
	if strings.Contains(got, "foo.com") {
		t.Errorf("domain still present after clear:\n%s", got)
	}
	if !strings.Contains(got, "127.0.0.1\tlocalhost") {
		t.Errorf("non-deepwork content lost after clear:\n%s", got)
	}
}

func TestRewrite_PreservesUserEditsOutsideMarkers(t *testing.T) {
	initial := baseHosts + "\n# my custom entry\n10.0.0.5 mydev.local\n"
	path := setupHosts(t, initial)
	if err := rewrite(path, []string{"block.com"}); err != nil {
		t.Fatal(err)
	}
	if err := rewrite(path, nil); err != nil {
		t.Fatal(err)
	}
	got := readHosts(t, path)
	if !strings.Contains(got, "10.0.0.5 mydev.local") {
		t.Errorf("user edit lost:\n%s", got)
	}
	if !strings.Contains(got, "# my custom entry") {
		t.Errorf("user comment lost:\n%s", got)
	}
}

func TestRewrite_SortedOutput(t *testing.T) {
	path := setupHosts(t, baseHosts)
	if err := rewrite(path, []string{"c.com", "a.com", "b.com"}); err != nil {
		t.Fatal(err)
	}
	got := readHosts(t, path)
	ai := strings.Index(got, "a.com")
	bi := strings.Index(got, "b.com")
	ci := strings.Index(got, "c.com")
	if !(ai < bi && bi < ci) {
		t.Errorf("domains not sorted: a=%d b=%d c=%d\n%s", ai, bi, ci, got)
	}
}

func TestStripBlock_HandlesMissingNewlines(t *testing.T) {
	in := []byte("127.0.0.1 localhost\n# DEEPWORK_START\n0.0.0.0 x.com\n# DEEPWORK_END")
	got := stripBlock(in)
	if strings.Contains(string(got), "x.com") {
		t.Errorf("block not stripped: %q", got)
	}
}

func TestRewrite_EmitsIPv6Line(t *testing.T) {
	path := setupHosts(t, baseHosts)
	if err := rewrite(path, []string{"linkedin.com"}); err != nil {
		t.Fatal(err)
	}
	got := readHosts(t, path)
	if !strings.Contains(got, "0.0.0.0 linkedin.com") {
		t.Errorf("missing v4 line:\n%s", got)
	}
	if !strings.Contains(got, ":: linkedin.com") {
		t.Errorf("missing v6 line — IPv6 traffic unblocked:\n%s", got)
	}
}

func TestExistingBlock_RequiresBothFamilies(t *testing.T) {
	v4Only := baseHosts + "# DEEPWORK_START\n0.0.0.0 a.com\n# DEEPWORK_END\n"
	path := setupHosts(t, v4Only)
	got, err := ExistingBlock(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("v4-only block should count as not-applied, got %v", got)
	}

	both := baseHosts + "# DEEPWORK_START\n0.0.0.0 a.com\n:: a.com\n# DEEPWORK_END\n"
	path2 := setupHosts(t, both)
	got2, err := ExistingBlock(path2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || got2[0] != "a.com" {
		t.Errorf("expected [a.com], got %v", got2)
	}
}
