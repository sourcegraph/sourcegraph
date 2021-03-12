package query

import (
	"reflect"
	"testing"
)

func TestRepoContainsPredicate(t *testing.T) {
	t.Run("ParseParams", func(t *testing.T) {
		type test struct {
			name     string
			params   string
			expected *RepoContainsPredicate
		}

		valid := []test{
			{`file`, `file:test`, &RepoContainsPredicate{File: "test"}},
			{`file regex`, `file:test(a|b)*.go`, &RepoContainsPredicate{File: "test(a|b)*.go"}},
			{`content`, `content:test`, &RepoContainsPredicate{Content: "test"}},
			{`unnamed content`, `test`, &RepoContainsPredicate{Content: "test"}},

			// TODO (@camdencheek) Query parsing currently checks parameter names against an allowlist.
			// This will be a problem as soon as we add more fields. Might make sense to do
			// as part of #19075
			{`unrecognized field`, `abc:test`, &RepoContainsPredicate{Content: "abc:test"}},
		}

		for _, tc := range valid {
			t.Run(tc.name, func(t *testing.T) {
				p := &RepoContainsPredicate{}
				err := p.ParseParams(tc.params)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}

				if !reflect.DeepEqual(tc.expected, p) {
					t.Fatalf("expected %#v, got %#v", tc.expected, p)
				}
			})
		}

		invalid := []test{
			{`empty`, ``, nil},
		}

		for _, tc := range invalid {
			t.Run(tc.name, func(t *testing.T) {
				p := &RepoContainsPredicate{}
				err := p.ParseParams(tc.params)
				if err == nil {
					t.Fatal("expected error but got none")
				}
			})
		}
	})
}
