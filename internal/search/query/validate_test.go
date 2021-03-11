package query

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAndOrQuery_Validation(t *testing.T) {
	cases := []struct {
		input      string
		searchType SearchType // nil value is regexp
		want       string
	}{
		{
			input: "index:foo",
			want:  `invalid value "foo" for field "index". Valid values are: yes, only, no`,
		},
		{
			input: "case:yes case:no",
			want:  `field "case" may not be used more than once`,
		},
		{
			input: "repo:[",
			want:  "error parsing regexp: missing closing ]: `[`",
		},
		{
			input: "-index:yes",
			want:  `field "index" does not support negation`,
		},
		{
			input: "lang:c lang:go lang:stephenhas9cats",
			want:  `unknown language: "stephenhas9cats"`,
		},
		{
			input: "stable:???",
			want:  `invalid boolean "???"`,
		},
		{
			input: "count:sedonuts",
			want:  "field count has value sedonuts, sedonuts is not a number",
		},
		{
			input: "count:10000000000000000",
			want:  "field count has a value that is out of range, try making it smaller",
		},
		{
			input: "count:-1",
			want:  "field count requires a positive number",
		},
		{
			input: "+",
			want:  "error parsing regexp: missing argument to repetition operator: `+`",
		},
		{
			input: `\\\`,
			want:  "error parsing regexp: trailing backslash at end of expression: ``",
		},
		{
			input:      `-content:"foo"`,
			want:       "the query contains a negated search pattern. Structural search does not support negated search patterns at the moment",
			searchType: SearchTypeStructural,
		},
		{
			input:      `NOT foo`,
			want:       "the query contains a negated search pattern. Structural search does not support negated search patterns at the moment",
			searchType: SearchTypeStructural,
		},
		{
			input: "repo:foo rev:a rev:b",
			want:  `field "rev" may not be used more than once`,
		},
		{
			input: "repo:foo@a rev:b",
			want:  "invalid syntax. You specified both @ and rev: for a repo: filter and I don't know how to interpret this. Remove either @ or rev: and try again",
		},
		{
			input: "repo:foo author:rob@saucegraph.com",
			want:  `your query contains the field 'author', which requires type:commit or type:diff in the query`,
		},
		{
			input: "repohasfile:README type:symbol yolo",
			want:  "repohasfile is not compatible for type:symbol. Subscribe to https://github.com/sourcegraph/sourcegraph/issues/4610 for updates",
		},
		{
			input: "foo context:a context:b",
			want:  `field "context" may not be used more than once`,
		},
		{
			input: "-context:a",
			want:  `field "context" does not support negation`,
		},
		{
			input: "type:symbol select:symbol.timelime",
			want:  "invalid field 'timelime' on select type 'symbol'",
		},
	}
	for _, c := range cases {
		t.Run("validate and/or query", func(t *testing.T) {
			_, err := Parse(c.input, ParserOptions{c.searchType, false})
			if err == nil {
				t.Fatal(fmt.Sprintf("expected test for %s to fail", c.input))
			}
			if diff := cmp.Diff(c.want, err.Error()); diff != "" {
				t.Fatal(diff)
			}

		})

	}
}

func TestAndOrQuery_IsCaseSensitive(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "yes",
			input: "case:yes",
			want:  true,
		},
		{
			name:  "no (explicit)",
			input: "case:no",
			want:  false,
		},
		{
			name:  "no (default)",
			input: "case:no",
			want:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			query, err := ParseRegexp(c.input)
			if err != nil {
				t.Fatal(err)
			}
			got := query.IsCaseSensitive()
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestAndOrQuery_RegexpPatterns(t *testing.T) {
	type want struct {
		values        []string
		negatedValues []string
	}
	c := struct {
		query string
		field string
		want
	}{
		query: "r:a r:b -r:c",
		field: "repo",
		want: want{
			values:        []string{"a", "b"},
			negatedValues: []string{"c"},
		},
	}
	t.Run("for regexp field", func(t *testing.T) {
		query, err := ParseRegexp(c.query)
		if err != nil {
			t.Fatal(err)
		}
		gotValues, gotNegatedValues := query.RegexpPatterns(c.field)
		if diff := cmp.Diff(c.want.values, gotValues); diff != "" {
			t.Error(diff)
		}
		if diff := cmp.Diff(c.want.negatedValues, gotNegatedValues); diff != "" {
			t.Error(diff)
		}
	})
}

func TestPartitionSearchPattern(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "x",
			want:  `"x"`,
		},
		{
			input: "file:foo",
			want:  `"file:foo"`,
		},
		{
			input: "x y",
			want:  `(concat "x" "y")`,
		},
		{
			input: "x or y",
			want:  `(or "x" "y")`,
		},
		{
			input: "x and y",
			want:  `(and "x" "y")`,
		},
		{
			input: "file:foo x y",
			want:  `"file:foo" (concat "x" "y")`,
		},
		{
			input: "file:foo (x y)",
			want:  `"file:foo" "(x y)"`,
		},
		{
			input: "(file:foo x) y",
			want:  "cannot evaluate: unable to partition pure search pattern",
		},
		{
			input: "file:foo (x and y)",
			want:  `"file:foo" (and "x" "y")`,
		},
		{
			input: "file:foo x and y",
			want:  `"file:foo" (and "x" "y")`,
		},
		{
			input: "file:foo (x or y)",
			want:  `"file:foo" (or "x" "y")`,
		},
		{
			input: "file:foo x or y",
			want:  `"file:foo" (or "x" "y")`,
		},
		{
			input: "(file:foo x) or y",
			want:  "cannot evaluate: unable to partition pure search pattern",
		},
		{
			input: "file:foo and content:x",
			want:  `"file:foo" "content:x"`,
		},
		{
			input: "repo:foo and file:bar and x",
			want:  `"repo:foo" "file:bar" "x"`,
		},
		{
			input: "repo:foo and (file:bar or file:baz) and x",
			want:  "cannot evaluate: unable to partition pure search pattern",
		},
	}
	for _, tt := range cases {
		t.Run("partition search pattern", func(t *testing.T) {
			q, _ := ParseAndOr(tt.input, SearchTypeRegex)
			scopeParameters, pattern, err := PartitionSearchPattern(q)
			if err != nil {
				if diff := cmp.Diff(tt.want, err.Error()); diff != "" {
					t.Fatal(diff)
				}
				return
			}
			result := scopeParameters
			if pattern != nil {
				result = append(scopeParameters, pattern)
			}
			got := toString(result)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestForAll(t *testing.T) {
	nodes := []Node{
		Parameter{Field: "repo", Value: "foo"},
		Parameter{Field: "repo", Value: "bar"},
	}
	result := forAll(nodes, func(node Node) bool {
		_, ok := node.(Parameter)
		return ok
	})
	if !result {
		t.Errorf("Expected all nodes to be parameters.")
	}
}

func TestContainsRefGlobs(t *testing.T) {
	tests := []struct {
		query    string
		want     bool
		globbing bool
	}{
		{
			query: "repo:foo",
			want:  false,
		},
		{
			query: "repo:foo@bar",
			want:  false,
		},
		{
			query: "repo:foo@*ref/tags",
			want:  true,
		},
		{
			query: "repo:foo@*!refs/tags",
			want:  true,
		},
		{
			query: "repo:foo@bar:*refs/heads",
			want:  true,
		},
		{
			query: "repo:foo@refs/tags/v3.14.3",
			want:  false,
		},
		{
			query: "repo:foo@*refs/tags/v3.14.?",
			want:  true,
		},
		{
			query:    "repo:*foo*@v3.14.3",
			globbing: true,
			want:     false,
		},
		{
			query: "repo:foo@v3.14.3 repo:foo@*refs/tags/v3.14.* bar",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			qInfo, err := Parse(tt.query, ParserOptions{SearchType: SearchTypeLiteral, Globbing: tt.globbing})
			if err != nil {
				t.Error(err)
			}
			got := ContainsRefGlobs(qInfo)
			if got != tt.want {
				t.Errorf("got %t, expected %t", got, tt.want)
			}
		})
	}
}

func TestHasTypeRepo(t *testing.T) {
	tests := []struct {
		query           string
		wantHasTypeRepo bool
	}{
		{
			query:           "sourcegraph type:repo",
			wantHasTypeRepo: true,
		},
		{
			query:           "sourcegraph type:symbol type:repo",
			wantHasTypeRepo: true,
		},
		{
			query:           "(sourcegraph type:repo) or (goreman type:repo)",
			wantHasTypeRepo: true,
		},
		{
			query:           "sourcegraph repohasfile:Dockerfile type:repo",
			wantHasTypeRepo: true,
		},
		{
			query:           "repo:sourcegraph type:repo",
			wantHasTypeRepo: true,
		},
		{
			query:           "repo:sourcegraph",
			wantHasTypeRepo: false,
		},
		{
			query:           "repository",
			wantHasTypeRepo: false,
		},
		{
			query:           "",
			wantHasTypeRepo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			q, err := ParseLiteral(tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if got := HasTypeRepo(q); got != tt.wantHasTypeRepo {
				t.Fatalf("got %t, expected %t", got, tt.wantHasTypeRepo)
			}
		})
	}
}
