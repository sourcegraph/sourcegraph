package protocol

import (
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/api"
)

func TestNormalizeRepo(t *testing.T) {
	cases := map[api.RepoName]api.RepoName{
		"foobar":                "foobar",
		"FooBar":                "FooBar",
		"foo/bar":               "foo/bar",
		"github.com/FooBar.git": "github.com/FooBar.git",

		// trying to escape gitserver root
		"/etc/passwd":                       "etc/passwd",
		"../../../etc/passwd":               "etc/passwd",
		"foobar.git/../etc/passwd":          "etc/passwd",
		"foobar.git/../../../../etc/passwd": "etc/passwd",

		// Degenerate cases
		"foo/bar/../..":  "",
		"/foo/bar/../..": "",
	}

	for k, want := range cases {
		if got := NormalizeRepo(k); got != want {
			t.Errorf("NormalizeRepo(%q): got %q want %q", k, got, want)
		}
	}
}
