package hosts

import (
	"slices"
	"testing"
)

func TestExistingBlock_ReadsDomains(t *testing.T) {
	path := setupHosts(t, baseHosts)
	if err := rewrite(path, []string{"c.com", "a.com", "b.com"}); err != nil {
		t.Fatal(err)
	}
	got, err := ExistingBlock(path)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{"a.com", "b.com", "c.com"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExistingBlock_NoBlock(t *testing.T) {
	path := setupHosts(t, baseHosts)
	got, err := ExistingBlock(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}
