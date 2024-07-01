package main

import (
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skipf("could not find ripgrep binary: %v", err)
	}
	err := do(nil, "foo bar")
	if err != nil {
		t.Fatal(err)
	}
}

func TestPlan(t *testing.T) {
	cases := []struct {
		Query string
		Args  []string
	}{{
		Query: "foo",
		Args:  []string{"--", "foo"},
	}, {
		Query: "foo r:bar",
		Args:  []string{"--", "foo"},
	}, {
		Query: "foo f:bar",
		Args:  []string{"--glob", "**bar**", "--", "foo"},
	}, {
		Query: `foo -f:_test\.go$`,
		Args:  []string{"--glob", "!**_test.go", "--", "foo"},
	}, {
		Query: "foo case:no",
		Args:  []string{"--ignore-case", "--", "foo"},
	}, {
		Query: "foo lang:go",
		Args:  []string{"--type", "go", "--", "foo"},
	}, {
		Query: "foo -lang:go",
		Args:  []string{"--type-not", "go", "--", "foo"},
	}}

	for _, tc := range cases {
		p, err := parse(tc.Query)
		if err != nil {
			t.Fatalf("failed to parse query %q: %v", tc.Query, err)
		}
		if len(p) != 1 {
			t.Fatalf("expected a basic query.Plan for %q: %v", tc.Query, p)
		}
		plan, err := plan(p[0].Pattern, p[0].Parameters)
		if err != nil {
			t.Fatalf("failed to plan for query %q: %v", tc.Query, err)
		}
		if d := cmp.Diff(tc.Args, plan.RipGrepArgs); d != "" {
			t.Fatalf("unexpected rg args for %q (-want, +got):\n%s", tc.Query, d)
		}
	}
}
