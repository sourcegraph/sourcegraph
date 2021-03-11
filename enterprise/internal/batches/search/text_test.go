package search

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-multierror"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search/syntax"
)

func TestChangesetSearch(t *testing.T) {
	t.Run("parse error", func(t *testing.T) {
		if _, err := ParseTextSearch(`:`); err == nil {
			t.Errorf("unexpected nil error")
		}
	})

	t.Run("invalid field", func(t *testing.T) {
		if _, err := ParseTextSearch(`x:`); err == nil {
			t.Errorf("unexpected nil error")
		} else if errs, ok := err.(*multierror.Error); !ok {
			t.Errorf("unexpected error of type %T: %+v", err, err)
		} else if diff := cmp.Diff([]error{
			ErrUnsupportedField{
				ErrExpr: ErrExpr{Pos: 0, Input: `x:`},
				Field:   "x",
			},
		}, errs.Errors); diff != "" {
			t.Errorf("unexpected error (-want +have):\n%s", diff)
		}
	})

	t.Run("invalid value type", func(t *testing.T) {
		if _, err := ParseTextSearch(`/foo/`); err == nil {
			t.Errorf("unexpected nil error")
		} else if errs, ok := err.(*multierror.Error); !ok {
			t.Errorf("unexpected error of type %T: %+v", err, err)
		} else if diff := cmp.Diff([]error{
			ErrUnsupportedValueType{
				ErrExpr:   ErrExpr{Pos: 1, Input: `/foo/`},
				ValueType: syntax.TokenPattern,
			},
		}, errs.Errors); diff != "" {
			t.Errorf("unexpected error (-want +have):\n%s", diff)
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		if _, err := ParseTextSearch(`x: /foo/`); err == nil {
			t.Errorf("unexpected nil error")
		} else if errs, ok := err.(*multierror.Error); !ok {
			t.Errorf("unexpected error of type %T: %+v", err, err)
		} else if diff := cmp.Diff([]error{
			ErrUnsupportedField{
				ErrExpr: ErrExpr{Pos: 0, Input: `x: /foo/`},
				Field:   "x",
			},
			ErrUnsupportedValueType{
				ErrExpr:   ErrExpr{Pos: 4, Input: `x: /foo/`},
				ValueType: syntax.TokenPattern,
			},
		}, errs.Errors); diff != "" {
			t.Errorf("unexpected error (-want +have):\n%s", diff)
		}
	})

	t.Run("success", func(t *testing.T) {
		for name, tc := range map[string]struct {
			input string
			want  []TextSearchTerm
		}{
			"empty string": {
				input: ``,
				want:  []TextSearchTerm{},
			},
			"single word": {
				input: `foo`,
				want: []TextSearchTerm{
					{Term: "foo"},
				},
			},
			"negated single word": {
				input: `-foo`,
				want: []TextSearchTerm{
					{Term: "foo", Not: true},
				},
			},
			"quoted phrase": {
				input: `"foo bar"`,
				want: []TextSearchTerm{
					{Term: "foo bar"},
				},
			},
			"negated quoted phrase": {
				input: `-"foo bar"`,
				want: []TextSearchTerm{
					{Term: "foo bar", Not: true},
				},
			},
			"multiple exprs": {
				input: `foo "foo bar" -quux -"baz"`,
				want: []TextSearchTerm{
					{Term: "foo"},
					{Term: "foo bar"},
					{Term: "quux", Not: true},
					{Term: "baz", Not: true},
				},
			},
		} {
			t.Run(name, func(t *testing.T) {
				terms, err := ParseTextSearch(tc.input)
				if err != nil {
					t.Errorf("unexpected error: %+v", err)
				}

				if diff := cmp.Diff(tc.want, terms); diff != "" {
					t.Errorf("unexpected terms (-want +have):\n%s", diff)
				}
			})
		}
	})
}
