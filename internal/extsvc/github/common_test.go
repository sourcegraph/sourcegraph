package github

import (
	"testing"
)

func TestSplitRepositoryNameWithOwner(t *testing.T) {
	owner, name, err := SplitRepositoryNameWithOwner("a/b")
	if err != nil {
		t.Fatal(err)
	}
	if want := "a"; owner != want {
		t.Errorf("got owner %q, want %q", owner, want)
	}
	if want := "b"; name != want {
		t.Errorf("got name %q, want %q", name, want)
	}
}
