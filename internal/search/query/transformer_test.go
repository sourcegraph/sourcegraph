package query

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hexops/autogold"
)

func toJSON(node Node) interface{} {
	switch n := node.(type) {
	case Operator:
		var jsons []interface{}
		for _, o := range n.Operands {
			jsons = append(jsons, toJSON(o))
		}

		switch n.Kind {
		case And:
			return struct {
				And []interface{} `json:"and"`
			}{
				And: jsons,
			}
		case Or:
			return struct {
				Or []interface{} `json:"or"`
			}{
				Or: jsons,
			}
		case Concat:
			return struct {
				Concat []interface{} `json:"concat"`
			}{
				Concat: jsons,
			}
		}
	case Parameter:
		return struct {
			Field   string `json:"field"`
			Value   string `json:"value"`
			Negated bool   `json:"negated"`
		}{
			Field:   n.Field,
			Value:   n.Value,
			Negated: n.Negated,
		}
	case Pattern:
		return struct {
			Value   string   `json:"value"`
			Negated bool     `json:"negated"`
			Labels  []string `json:"labels"`
		}{
			Value:   n.Value,
			Negated: n.Negated,
			Labels:  n.Annotation.Labels.String(),
		}
	}
	// unreachable.
	return struct{}{}
}

func nodesToJSON(nodes []Node) string {
	var jsons []interface{}
	for _, node := range nodes {
		jsons = append(jsons, toJSON(node))
	}
	json, err := json.Marshal(jsons)
	if err != nil {
		return ""
	}
	return string(json)
}

func TestSubstituteAliases(t *testing.T) {
	cases := []struct {
		input      string
		searchType SearchType
		want       string
	}{
		{
			input:      "r:repo g:repogroup f:file",
			searchType: SearchTypeRegex,
			want:       `[{"and":[{"field":"repo","value":"repo","negated":false},{"field":"repogroup","value":"repogroup","negated":false},{"field":"file","value":"file","negated":false}]}]`,
		},
		{
			input:      "r:repo content:^a-regexp:tbf$",
			searchType: SearchTypeRegex,
			want:       `[{"and":[{"field":"repo","value":"repo","negated":false},{"value":"^a-regexp:tbf$","negated":false,"labels":["Regexp"]}]}]`,
		},
		{
			input:      "r:repo content:^not-actually-a-regexp:tbf$",
			searchType: SearchTypeLiteral,
			want:       `[{"and":[{"field":"repo","value":"repo","negated":false},{"value":"^not-actually-a-regexp:tbf$","negated":false,"labels":["Literal"]}]}]`,
		},
	}

	for _, c := range cases {
		t.Run("substitute alises", func(t *testing.T) {
			query, _ := Parse(c.input, ParserOptions{SearchType: c.searchType})
			if diff := cmp.Diff(nodesToJSON(query), c.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestLowercaseFieldNames(t *testing.T) {
	input := "rEpO:foo PATTERN"
	want := `(and "repo:foo" "PATTERN")`
	query, _ := ParseAndOr(input, SearchTypeRegex)
	got := toString(LowercaseFieldNames(query))
	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatal(diff)
	}
}

func TestHoist(t *testing.T) {
	cases := []struct {
		input      string
		want       string
		wantErrMsg string
	}{
		{
			input: `repo:foo a or b`,
			want:  `"repo:foo" (or "a" "b")`,
		},
		{
			input: `repo:foo a or b file:bar`,
			want:  `"repo:foo" "file:bar" (or "a" "b")`,
		},
		{
			input: `repo:foo a or b or c file:bar`,
			want:  `"repo:foo" "file:bar" (or "a" "b" "c")`,
		},
		{
			input: "repo:foo bar { and baz {",
			want:  `"repo:foo" (and (concat "bar" "{") (concat "baz" "{"))`,
		},
		{
			input: "repo:foo bar { and baz { and qux {",
			want:  `"repo:foo" (and (concat "bar" "{") (concat "baz" "{") (concat "qux" "{"))`,
		},
		{
			input: `repo:foo a and b or c and d or e file:bar`,
			want:  `"repo:foo" "file:bar" (or (and "a" "b") (and "c" "d") "e")`,
		},
		// This next pattern is valid for the heuristic, even though the ordering of the
		// patterns 'a' and 'c' in the first and last position are not ordered next to the
		// 'or' keyword. This because no ordering is assumed for patterns vs. field:value
		// parameters in the grammar. To preserve relative ordering and check this would
		// impose significant complexity to PartitionParameters function during parsing, and
		// the PartitionSearchPattern helper function that the heurstic relies on. So: we
		// accept this heuristic behavior here.
		{
			input: `a repo:foo or b or file:bar c`,
			want:  `"repo:foo" "file:bar" (or "a" "b" "c")`,
		},
		// Errors.
		{
			input:      "repo:foo or a",
			wantErrMsg: "could not partition first or last expression",
		},
		{
			input:      "a or repo:foo",
			wantErrMsg: "could not partition first or last expression",
		},
		{
			input:      "repo:foo or repo:bar",
			wantErrMsg: "could not partition first or last expression",
		},
		{
			input:      "a b",
			wantErrMsg: "heuristic requires top-level and- or or-expression",
		},
		{
			input:      "repo:foo a or repo:foobar b or c file:bar",
			wantErrMsg: `inner expression (and "repo:foobar" "b") is not a pure pattern expression`,
		},
	}
	for _, c := range cases {
		t.Run("hoist", func(t *testing.T) {
			// To test Hoist, Use a simplified parse function that
			// does not perform the heuristic.
			parse := func(in string) []Node {
				parser := &parser{
					buf:        []byte(in),
					heuristics: parensAsPatterns,
					leafParser: SearchTypeRegex,
				}
				nodes, _ := parser.parseOr()
				return newOperator(nodes, And)
			}
			query := parse(c.input)
			hoistedQuery, err := Hoist(query)
			if err != nil {
				if diff := cmp.Diff(c.wantErrMsg, err.Error()); diff != "" {
					t.Error(diff)
				}
				return
			}
			got := toString(hoistedQuery)
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestSearchUppercase(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: `TeSt`,
			want:  `(and "TeSt" "case:yes")`,
		},
		{
			input: `test`,
			want:  `"test"`,
		},
		{
			input: `content:TeSt`,
			want:  `(and "TeSt" "case:yes")`,
		},
		{
			input: `content:test`,
			want:  `"test"`,
		},
		{
			input: `repo:foo TeSt`,
			want:  `(and "repo:foo" "TeSt" "case:yes")`,
		},
		{
			input: `repo:foo test`,
			want:  `(and "repo:foo" "test")`,
		},
		{
			input: `repo:foo content:TeSt`,
			want:  `(and "repo:foo" "TeSt" "case:yes")`,
		},
		{
			input: `repo:foo content:test`,
			want:  `(and "repo:foo" "test")`,
		},
		{
			input: `TeSt1 TesT2`,
			want:  `(and (concat "TeSt1" "TesT2") "case:yes")`,
		},
		{
			input: `TeSt1 test2`,
			want:  `(and (concat "TeSt1" "test2") "case:yes")`,
		},
	}
	for _, c := range cases {
		t.Run("searchUppercase", func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			got := toString(SearchUppercase(SubstituteAliases(SearchTypeRegex)(query)))
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestSubstituteOrForRegexp(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "foo or bar",
			want:  `"(foo)|(bar)"`,
		},
		{
			input: "(foo or (bar or baz))",
			want:  `"(foo)|(bar)|(baz)"`,
		},
		{
			input: "repo:foobar foo or (bar or baz)",
			want:  `(or "(bar)|(baz)" (and "repo:foobar" "foo"))`,
		},
		{
			input: "(foo or (bar or baz)) and foobar",
			want:  `(and "(foo)|(bar)|(baz)" "foobar")`,
		},
		{
			input: "(foo or (bar and baz))",
			want:  `(or "(foo)" (and "bar" "baz"))`,
		},
		{
			input: "foo or (bar and baz) or foobar",
			want:  `(or "(foo)|(foobar)" (and "bar" "baz"))`,
		},
		{
			input: "repo:foo a or b",
			want:  `(and "repo:foo" "(a)|(b)")`,
		},
	}
	for _, c := range cases {
		t.Run("Map query", func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			got := toString(substituteOrForRegexp(query))
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestSubstituteConcat(t *testing.T) {
	cases := []struct {
		input  string
		concat func([]Pattern) Pattern
		want   string
	}{
		{
			input:  "a b c d e f",
			concat: space,
			want:   `"a b c d e f"`,
		},
		{
			input:  "a (b and c) d",
			concat: space,
			want:   `"a" (and "b" "c") "d"`,
		},
		{
			input:  "a b (c and d) e f (g or h) (i j k)",
			concat: space,
			want:   `"a b" (and "c" "d") "e f" (or "g" "h") "(i j k)"`,
		},
		{
			input:  "(((a b c))) and d",
			concat: space,
			want:   `(and "(((a b c)))" "d")`,
		},
		{
			input:  `foo\d "bar*"`,
			concat: fuzzyRegexp,
			want:   `"(foo\\d).*?(bar\\*)"`,
		},
		{
			input:  `"bar*" foo\d "bar*" foo\d`,
			concat: fuzzyRegexp,
			want:   `"(bar\\*).*?(foo\\d).*?(bar\\*).*?(foo\\d)"`,
		},
		{
			input:  "a b (c and d) e f (g or h) (i j k)",
			concat: fuzzyRegexp,
			want:   `"(a).*?(b)" (and "c" "d") "(e).*?(f)" (or "g" "h") "(i j k)"`,
		},
		{
			input:  "(a not b not c d)",
			concat: space,
			want:   `"a" (not "b") (not "c") "d"`,
		},
	}
	for _, c := range cases {
		t.Run("Map query", func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			got := toString(Map(query, substituteConcat(c.concat)))
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestEllipsesForHoles(t *testing.T) {
	input := "if ... { ... }"
	want := `"if :[_] { :[_] }"`
	t.Run("Ellipses for holes", func(t *testing.T) {
		query, _ := Parse(input, ParserOptions{SearchType: SearchTypeStructural})
		got := toString(query)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatal(diff)
		}
	})
}

func TestConvertEmptyGroupsToLiteral(t *testing.T) {
	cases := []struct {
		input      string
		want       string
		wantLabels labels
	}{
		{
			input:      "func()",
			want:       `"func\\(\\)"`,
			wantLabels: Regexp,
		},
		{
			input:      "func(.*)",
			want:       `"func(.*)"`,
			wantLabels: Regexp,
		},
		{
			input:      `(search\()`,
			want:       `"(search\\()"`,
			wantLabels: Regexp,
		},
		{
			input:      `()search\(()`,
			want:       `"\\(\\)search\\(\\(\\)"`,
			wantLabels: Regexp,
		},
		{
			input:      `search\(`,
			want:       `"search\\("`,
			wantLabels: Regexp,
		},
		{
			input:      `\`,
			want:       `"\\"`,
			wantLabels: Regexp,
		},
		{
			input:      `search(`,
			want:       `"search\\("`,
			wantLabels: Regexp | HeuristicDanglingParens,
		},
		{
			input:      `"search("`,
			want:       `"search("`,
			wantLabels: Quoted | Literal,
		},
		{
			input:      `"search()"`,
			want:       `"search()"`,
			wantLabels: Quoted | Literal,
		},
	}
	for _, c := range cases {
		t.Run("Map query", func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			got := escapeParensHeuristic(query)[0].(Pattern)
			if diff := cmp.Diff(c.want, toString([]Node{got})); diff != "" {
				t.Error(diff)
			}
			if diff := cmp.Diff(c.wantLabels, got.Annotation.Labels); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestExpandOr(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: `a or b`,
			want:  `("a") OR ("b")`,
		},
		{
			input: `a and b AND c OR d`,
			want:  `("a" "b" "c") OR ("d")`,
		},
		{
			input: "(repo:a (file:b or file:c))",
			want:  `("repo:a" "file:b") OR ("repo:a" "file:c")`,
		},
		{
			input: "(repo:a (file:b or file:c) (file:d or file:e))",
			want:  `("repo:a" "file:b" "file:d") OR ("repo:a" "file:c" "file:d") OR ("repo:a" "file:b" "file:e") OR ("repo:a" "file:c" "file:e")`,
		},
		{
			input: "(repo:a (file:b or file:c) (a b) (x z))",
			want:  `("repo:a" "file:b" "(a b)" "(x z)") OR ("repo:a" "file:c" "(a b)" "(x z)")`,
		},
		{
			input: `a and b AND c or d and (e OR f) g h i or j`,
			want:  `("a" "b" "c") OR ("d" "e" "g" "h" "i") OR ("d" "f" "g" "h" "i") OR ("j")`,
		},
		{
			input: "(repo:a (file:b (file:c or file:d) (file:e or file:f)))",
			want:  `("repo:a" "file:b" "file:c" "file:e") OR ("repo:a" "file:b" "file:d" "file:e") OR ("repo:a" "file:b" "file:c" "file:f") OR ("repo:a" "file:b" "file:d" "file:f")`,
		},
		{
			input: "(repo:a (file:b (file:c or file:d) file:q (file:e or file:f)))",
			want:  `("repo:a" "file:b" "file:c" "file:q" "file:e") OR ("repo:a" "file:b" "file:d" "file:q" "file:e") OR ("repo:a" "file:b" "file:c" "file:q" "file:f") OR ("repo:a" "file:b" "file:d" "file:q" "file:f")`,
		},
	}
	for _, c := range cases {
		t.Run("Map query", func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			queries := Dnf(query)
			var queriesStr []string
			for _, q := range queries {
				queriesStr = append(queriesStr, toString(q))
			}
			got := "(" + strings.Join(queriesStr, ") OR (") + ")"
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestMap(t *testing.T) {
	cases := []struct {
		input string
		fns   []func(_ []Node) []Node
		want  string
	}{
		{
			input: "RePo:foo",
			fns:   []func(_ []Node) []Node{LowercaseFieldNames},
			want:  `"repo:foo"`,
		},
		{
			input: "RePo:foo r:bar",
			fns:   []func(_ []Node) []Node{LowercaseFieldNames, SubstituteAliases(SearchTypeRegex)},
			want:  `(and "repo:foo" "repo:bar")`,
		},
	}
	for _, c := range cases {
		t.Run("Map query", func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			got := toString(Map(query, c.fns...))
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestTranslateGlobToRegex(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "*",
			want:  "^[^/]*?$",
		},
		{
			input: "*repo",
			want:  "^[^/]*?repo$",
		},
		{
			input: "**.go",
			want:  "^.*?\\.go$",
		},
		{
			input: "foo**",
			want:  "^foo.*?$",
		},
		{
			input: "re*o",
			want:  "^re[^/]*?o$",
		},
		{
			input: "repo*",
			want:  "^repo[^/]*?$",
		},
		{
			input: "?",
			want:  "^.$",
		},
		{
			input: "?repo",
			want:  "^.repo$",
		},
		{
			input: "re?o",
			want:  "^re.o$",
		},
		{
			input: "repo?",
			want:  "^repo.$",
		},
		{
			input: "123",
			want:  "^123$",
		},
		{
			input: ".123",
			want:  "^\\.123$",
		},
		{
			input: "*.go",
			want:  "^[^/]*?\\.go$",
		},
		{
			input: "h[a-z]llo",
			want:  "^h[a-z]llo$",
		},
		{
			input: "h[!a-z]llo",
			want:  "^h[^a-z]llo$",
		},
		{
			input: "h[!abcde]llo",
			want:  "^h[^abcde]llo$",
		},
		{
			input: "h[]-]llo",
			want:  "^h[]-]llo$",
		},
		{
			input: "h\\[llo",
			want:  "^h\\[llo$",
		},
		{
			input: "h\\*llo",
			want:  "^h\\*llo$",
		},
		{
			input: "h\\?llo",
			want:  "^h\\?llo$",
		},
		{
			input: "fo[a-z]baz",
			want:  "^fo[a-z]baz$",
		},
		{
			input: "foo/**",
			want:  "^foo/.*?$",
		},
		{
			input: "[a-z0-9]",
			want:  "^[a-z0-9]$",
		},
		{
			input: "[abc-]",
			want:  "^[abc-]$",
		},
		{
			input: "[--0]",
			want:  "^[--0]$",
		},
		{
			input: "",
			want:  "",
		},
		{
			input: "[!a]",
			want:  "^[^a]$",
		},
		{
			input: "fo[a-b-c]",
			want:  "^fo[a-b-c]$",
		},
		{
			input: "[a-z--0]",
			want:  "^[a-z--0]$",
		},
		{
			input: "[^ab]",
			want:  "^[//^ab]$",
		},
		{
			input: "[^-z]",
			want:  "^[//^-z]$",
		},
		{
			input: "[a^b]",
			want:  "^[a^b]$",
		},
		{
			input: "[ab^]",
			want:  "^[ab^]$",
		},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, err := globToRegex(c.input)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Fatal(diff)
			}

			if _, err := regexp.Compile(got); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTranslateBadGlobPattern(t *testing.T) {
	cases := []struct {
		input string
	}{
		{input: "fo\\o"},
		{input: "fo[o"},
		{input: "[z-a]"},
		{input: "0[0300z0_0]\\"},
		{input: "[!]"},
		{input: "0["},
		{input: "[]"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			_, err := globToRegex(c.input)
			if diff := cmp.Diff(ErrBadGlobPattern.Error(), err.Error()); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestReporevToRegex(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "starting with github.com, no revision",
			arg:  "github.com/foo",
			want: "^github\\.com/foo.*?$",
		},
		{
			name: "starting with github.com, with revision",
			arg:  "github.com/foo@bar",
			want: "^github\\.com/foo$@bar",
		},
		{
			name: "starting with foo.com, no revision",
			arg:  "foo.com/bar",
			want: "^.*?foo\\.com/bar.*?$",
		},
		{
			name: "empty string",
			arg:  "",
			want: "",
		},
		{
			name: "many @",
			arg:  "foo@bar@bas",
			want: "^foo$@bar@bas",
		},
		{
			name: "just @",
			arg:  "@",
			want: "@",
		},
		{
			name: "fuzzy repo",
			arg:  "sourcegraph",
			want: "^.*?sourcegraph.*?$",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reporevToRegex(tt.arg)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("reporevToRegex() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFuzzifyRegexPatterns(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "repo:foo$", want: `"repo:foo"`},
		{in: "file:foo$", want: `"file:foo"`},
		{in: "repohasfile:foo$", want: `"repohasfile:foo"`},
		{in: "repo:foo$ file:bar$ author:foo", want: `(and "repo:foo" "file:bar" "author:foo")`},
		{in: "repo:foo$ ^bar$", want: `(and "repo:foo" "^bar$")`},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			query, _ := ParseAndOr(tt.in, SearchTypeRegex)
			got := toString(FuzzifyRegexPatterns(query))
			if got != tt.want {
				t.Fatalf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsNoGlobSyntax(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{
			in:   "foo",
			want: true,
		},
		{
			in:   "foo.bar",
			want: true,
		},
		{
			in:   "/foo.bar",
			want: true,
		},
		{
			in:   "path/to/file/foo.bar",
			want: true,
		},
		{
			in:   "github.com/org/repo",
			want: true,
		},
		{
			in:   "foo**",
			want: false,
		},
		{
			in:   "**foo",
			want: false,
		},
		{
			in:   "**foo**",
			want: false,
		},
		{
			in:   "*foo*",
			want: false,
		},
		{
			in:   "foo?",
			want: false,
		},
		{
			in:   "fo?o",
			want: false,
		},
		{
			in:   "fo[o]bar",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := ContainsNoGlobSyntax(tt.in); got != tt.want {
				t.Errorf("ContainsNoGlobSyntax() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFuzzifyGlobPattern(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{
			in:   "foo",
			want: "**foo**",
		},
		{
			in:   "sourcegraph/sourcegraph",
			want: "**sourcegraph/sourcegraph**",
		},
		{
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := fuzzifyGlobPattern(tt.in); got != tt.want {
				t.Errorf("fuzzifyGlobPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapGlobToRegex(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "repo:sourcegraph",
			want:  `"repo:^.*?sourcegraph.*?$"`,
		},
		{
			input: "repo:sourcegraph@commit-id",
			want:  `"repo:^sourcegraph$@commit-id"`,
		},
		{
			input: "repo:github.com/sourcegraph",
			want:  `"repo:^github\\.com/sourcegraph.*?$"`,
		},
		{
			input: "repo:github.com/sourcegraph/sourcegraph@v3.18.0",
			want:  `"repo:^github\\.com/sourcegraph/sourcegraph$@v3.18.0"`,
		},
		{
			input: "github.com/foo/bar",
			want:  `"github.com/foo/bar"`,
		},
		{
			input: "repo:**sourcegraph",
			want:  `"repo:^.*?sourcegraph$"`,
		},
		{
			input: "file:**foo.bar",
			want:  `"file:^.*?foo\\.bar$"`,
		},
		{
			input: "file:afile file:bfile file:**cfile",
			want:  `(and "file:^.*?afile.*?$" "file:^.*?bfile.*?$" "file:^.*?cfile$")`,
		},
		{
			input: "file:afile file:dir1/bfile",
			want:  `(and "file:^.*?afile.*?$" "file:^.*?dir1/bfile.*?$")`,
		},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			regexQuery, _ := mapGlobToRegex(query)
			got := toString(regexQuery)
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestConcatRevFilters(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "repo:foo",
			want:  `("repo:foo")`,
		},
		{
			input: "repo:foo rev:a",
			want:  `("repo:foo@a")`,
		},
		{
			input: "repo:foo repo:bar rev:a",
			want:  `("repo:foo@a" "repo:bar@a")`,
		},
		{
			input: "repo:foo bar and bas rev:a",
			want:  `("repo:foo@a" "bar" "bas")`,
		},
		{
			input: "(repo:foo rev:a) or (repo:foo rev:b)",
			want:  `("repo:foo@a") OR ("repo:foo@b")`,
		},
		{
			input: "repo:foo file:bas qux AND (rev:a or rev:b)",
			want:  `("repo:foo@a" "file:bas" "qux") OR ("repo:foo@b" "file:bas" "qux")`,
		},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			queries := Dnf(query)

			var queriesStr []string
			for _, q := range queries {
				qConcat := ConcatRevFilters(q)
				queriesStr = append(queriesStr, toString(qConcat))
			}
			got := "(" + strings.Join(queriesStr, ") OR (") + ")"
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestConcatRevFiltersTopLevelAnd(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: "repo:sourcegraph",
			want:  `"repo:sourcegraph"`,
		},
		{
			input: "repo:sourcegraph rev:b",
			want:  `"repo:sourcegraph@b"`,
		},
		{
			input: "repo:sourcegraph foo and bar rev:b",
			want:  `(and "repo:sourcegraph@b" "foo" "bar")`,
		},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			query, _ := ParseAndOr(c.input, SearchTypeRegex)
			qConcat := ConcatRevFilters(query)
			if diff := cmp.Diff(c.want, toString(qConcat)); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestQueryField(t *testing.T) {
	test := func(input, field string) string {
		q, _ := ParseLiteral(input)
		return OmitField(q, field)
	}

	autogold.Want("omit repo", "pattern").Equal(t, test("repo:stuff pattern", "repo"))
	autogold.Want("omit repo alias", "alias-pattern").Equal(t, test("r:stuff alias-pattern", "repo"))
}
