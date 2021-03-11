package store

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search"
)

func TestTextSearchTermToClause(t *testing.T) {
	for name, tc := range map[string]struct {
		term      search.TextSearchTerm
		fields    []string
		wantArgs  []interface{}
		wantQuery string
	}{
		"one positive field": {
			term:      search.TextSearchTerm{Term: "foo"},
			fields:    []string{"field"},
			wantArgs:  []interface{}{"foo"},
			wantQuery: `(field ~* ('\m'||$1||'\M'))`,
		},
		"one negative field": {
			term:      search.TextSearchTerm{Term: "foo", Not: true},
			fields:    []string{"field"},
			wantArgs:  []interface{}{"foo"},
			wantQuery: `(field !~* ('\m'||$1||'\M'))`,
		},
		"two positive fields": {
			term:      search.TextSearchTerm{Term: "foo"},
			fields:    []string{"field", "paddock"},
			wantArgs:  []interface{}{"foo", "foo"},
			wantQuery: `(field ~* ('\m'||$1||'\M') OR paddock ~* ('\m'||$2||'\M'))`,
		},
		"two negative fields": {
			term:      search.TextSearchTerm{Term: "foo", Not: true},
			fields:    []string{"field", "paddock"},
			wantArgs:  []interface{}{"foo", "foo"},
			wantQuery: `(field !~* ('\m'||$1||'\M') AND paddock !~* ('\m'||$2||'\M'))`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			fields := make([]*sqlf.Query, len(tc.fields))
			for i, field := range tc.fields {
				fields[i] = sqlf.Sprintf(field)
			}

			query := textSearchTermToClause(tc.term, fields...)
			args := query.Args()
			if diff := cmp.Diff(args, tc.wantArgs); diff != "" {
				t.Errorf("unexpected arguments (-have +want):\n%s", diff)
			}
			if have := query.Query(sqlf.PostgresBindVar); have != tc.wantQuery {
				t.Errorf("unexpected query: have=%q want=%q", have, tc.wantQuery)
			}
		})
	}
}
