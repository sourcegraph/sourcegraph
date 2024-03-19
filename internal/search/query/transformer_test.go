package query

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
)

func TestExperimentalPhraseBoost(t *testing.T) {
	test := func(input string, searchType SearchType) string {
		plan, err := Pipeline(
			Init(input, SearchTypeKeyword))
		require.NoError(t, err)

		plan = MapPlan(plan, ExperimentalPhraseBoost)

		return plan.ToQ().String()
	}

	// expect phrase query
	autogold.Expect(`(or "foo bar bas" (and "foo" "bar" "bas"))`).Equal(t, test("foo bar bas", SearchTypeKeyword))
	autogold.Expect(`(or "foo bar bas" (and "foo" "bar" "bas"))`).Equal(t, test("(foo and bar) and bas", SearchTypeKeyword))
	autogold.Expect(`(or "* int func(" (and "*" "int" "func("))`).Equal(t, test("* int func(", SearchTypeKeyword))
	autogold.Expect(`(or "foo bar bas qux" (and "foo bar" "bas" "qux"))`).Equal(t, test(`"foo bar" bas qux`, SearchTypeKeyword))

	// expect no phrase query
	autogold.Expect(`"foo bar bas"`).Equal(t, test("/foo bar bas/", SearchTypeKeyword))
	autogold.Expect(`(and "foo" "bar" "ba.*")`).Equal(t, test("foo bar /ba.*/", SearchTypeKeyword))
	autogold.Expect(`"foo"`).Equal(t, test("foo", SearchTypeKeyword))
	autogold.Expect(`(and "foo" "bar")`).Equal(t, test("foo bar", SearchTypeKeyword))
	autogold.Expect(`(and "foo" "bar")`).Equal(t, test("foo and bar", SearchTypeKeyword))
	autogold.Expect(`(and "foo" (not "bar"))`).Equal(t, test("foo not bar", SearchTypeKeyword))
	autogold.Expect(`(and "foo" "bar" (not "bas") "quz")`).Equal(t, test("foo bar not bas quz", SearchTypeKeyword))
	autogold.Expect(`(or "foo" "bar" "bas")`).Equal(t, test("foo or bar or bas", SearchTypeKeyword))
	autogold.Expect(`(or (and "foo" "bar") (and "quz" "biz"))`).Equal(t, test("foo and bar or (quz and biz)", SearchTypeKeyword))
}

func TestSubstituteAliases(t *testing.T) {
	test := func(input string, searchType SearchType) string {
		query, _ := ParseSearchType(input, searchType)
		json, _ := ToJSON(query)
		return json
	}

	autogold.Expect(`[{"and":[{"field":"repo","value":"repo","negated":false,"labels":["IsAlias"],"range":{"start":{"line":0,"column":0},"end":{"line":0,"column":6}}},{"field":"file","value":"file","negated":false,"labels":["IsAlias"],"range":{"start":{"line":0,"column":7},"end":{"line":0,"column":13}}}]}]`).
		Equal(t, test("r:repo f:file", SearchTypeRegex))

	autogold.Expect(`[{"and":[{"field":"repo","value":"repo","negated":false,"labels":["IsAlias"],"range":{"start":{"line":0,"column":0},"end":{"line":0,"column":6}}},{"value":"^a-regexp:tbf$","negated":false,"labels":["IsContent","Regexp"],"range":{"start":{"line":0,"column":7},"end":{"line":0,"column":29}}}]}]`).
		Equal(t, test("r:repo content:^a-regexp:tbf$", SearchTypeRegex))

	autogold.Expect(`[{"and":[{"field":"repo","value":"repo","negated":false,"labels":["IsAlias"],"range":{"start":{"line":0,"column":0},"end":{"line":0,"column":6}}},{"value":"^not-actually-a-regexp:tbf$","negated":false,"labels":["IsContent","Literal"],"range":{"start":{"line":0,"column":7},"end":{"line":0,"column":42}}}]}]`).
		Equal(t, test("r:repo content:^not-actually-a-regexp:tbf$", SearchTypeLiteral))

	autogold.Expect(`[{"field":"file","value":"foo","negated":false,"labels":["IsAlias"],"range":{"start":{"line":0,"column":0},"end":{"line":0,"column":8}}}]`).
		Equal(t, test("path:foo", SearchTypeLiteral))

	autogold.Expect(`[{"and":[{"field":"repo","value":"repo","negated":false,"labels":["IsAlias"],"range":{"start":{"line":0,"column":0},"end":{"line":0,"column":6}}},{"value":"foo","negated":false,"labels":["Literal","QuotesAsLiterals","Standard"],"range":{"start":{"line":0,"column":7},"end":{"line":0,"column":10}}},{"value":"bar","negated":false,"labels":["Literal","QuotesAsLiterals","Standard"],"range":{"start":{"line":0,"column":11},"end":{"line":0,"column":14}}}]}]`).
		Equal(t, test("r:repo foo bar", SearchTypeKeyword))

	autogold.Expect(`[{"and":[{"value":"foo","negated":false,"labels":["Literal","QuotesAsLiterals","Standard"],"range":{"start":{"line":0,"column":0},"end":{"line":0,"column":3}}},{"value":"bar","negated":false,"labels":["Literal","QuotesAsLiterals","Standard"],"range":{"start":{"line":0,"column":4},"end":{"line":0,"column":7}}},{"value":"bas","negated":false,"labels":["Regexp"],"range":{"start":{"line":0,"column":8},"end":{"line":0,"column":13}}}]}]`).
		Equal(t, test("foo bar /bas/", SearchTypeKeyword))
}

func TestLowercaseFieldNames(t *testing.T) {
	input := "rEpO:foo PATTERN"
	want := `(and "repo:foo" "PATTERN")`
	query, _ := Parse(input, SearchTypeRegex)
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
			input: `repo:foo file:bar a and b or c`,
			want:  `"repo:foo" "file:bar" (or (and "a" "b") "c")`,
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
			input: `repo:foo a or b file:bar`,
			want:  `"repo:foo" "file:bar" (or "a" "b")`,
		},
		{
			input: `repo:foo a or b or c file:bar`,
			want:  `"repo:foo" "file:bar" (or "a" "b" "c")`,
		},
		{
			input: `repo:foo a and b or c and d or e file:bar`,
			want:  `"repo:foo" "file:bar" (or (and "a" "b") (and "c" "d") "e")`,
		},
		// Errors.
		{
			input:      "a repo:foo or b",
			wantErrMsg: "unnatural order: patterns not followed by parameter",
		},
		{
			input:      "a repo:foo q or b",
			wantErrMsg: "unnatural order: patterns not followed by parameter",
		},
		{
			input:      "repo:bar a repo:foo or b",
			wantErrMsg: "unnatural order: patterns not followed by parameter",
		},
		{
			input:      `a repo:foo or b or file:bar c`,
			wantErrMsg: "unnatural order: patterns not followed by parameter",
		},
		{
			input:      "repo:foo or a",
			wantErrMsg: "could not partition first expression",
		},
		{
			input:      "a or repo:foo",
			wantErrMsg: "unnatural order: patterns not followed by parameter",
		},
		{
			input:      "repo:foo or repo:bar",
			wantErrMsg: "could not partition first expression",
		},
		{
			input:      "a b",
			wantErrMsg: "heuristic requires top-level and- or or-expression",
		},
		{
			input:      "repo:foo a or repo:foobar b or c file:bar",
			wantErrMsg: `inner expression (and "repo:foobar" "b") is not a pure pattern expression`,
		},
		{
			input:      "repo:a b or c repo:b d or e",
			wantErrMsg: `inner expression (and "repo:b" (concat "c" "d")) is not a pure pattern expression`,
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
				return NewOperator(nodes, And)
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

func TestConcat(t *testing.T) {
	test := func(input string, searchType SearchType) string {
		query, _ := ParseSearchType(input, searchType)
		json, _ := PrettyJSON(query)
		return json
	}

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test("a b c d e f", SearchTypeLiteral)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test("(a not b not c d)", SearchTypeLiteral)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test("(((a b c))) and d", SearchTypeLiteral)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test(`foo\d "bar*"`, SearchTypeRegex)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test(`"bar*" foo\d "bar*" foo\d`, SearchTypeRegex)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test("a b (c and d) e f (g or h) (i j k)", SearchTypeRegex)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test(`/alsace/ bourgogne bordeaux /champagne/`, SearchTypeStandard)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test(`alsace /bourgogne/ bordeaux`, SearchTypeStandard)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test(`alsace /bourgogne/ bordeaux`, SearchTypeLucky)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test(`/alsace/ bourgogne bordeaux /champagne/`, SearchTypeKeyword)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test(`alsace /bourgogne/ bordeaux`, SearchTypeKeyword)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test("a b c d e f", SearchTypeKeyword)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test("(a not b not c d)", SearchTypeKeyword)))
	})

	t.Run("", func(t *testing.T) {
		autogold.ExpectFile(t, autogold.Raw(test("(((a b c))) and d", SearchTypeKeyword)))
	})
}

func TestEllipsesForHoles(t *testing.T) {
	input := "if ... { ... }"
	want := `"if :[_] { :[_] }"`
	t.Run("Ellipses for holes", func(t *testing.T) {
		query, _ := Run(InitStructural(input))
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
			query, _ := Parse(c.input, SearchTypeRegex)
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

func TestPipeline(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{{
		input: `a or b`,
		want:  `(or "a" "b")`,
	}, {
		input: `a and b AND c OR d`,
		want:  `(or (and "a" "b" "c") "d")`,
	}, {
		input: `(repo:a (file:b or file:c))`,
		want:  `(or (and "repo:a" "file:b") (and "repo:a" "file:c"))`,
	}, {
		input: `(repo:a (file:b or file:c) (file:d or file:e))`,
		want:  `(or (and "repo:a" "file:b" "file:d") (and "repo:a" "file:c" "file:d") (and "repo:a" "file:b" "file:e") (and "repo:a" "file:c" "file:e"))`,
	}, {
		input: `(repo:a (file:b or file:c) (a b) (x z))`,
		want:  `(or (and "repo:a" "file:b" "(a b) (x z)") (and "repo:a" "file:c" "(a b) (x z)"))`,
	}, {
		input: `a and b AND c or d and (e OR f) and g h i or j`,
		want:  `(or (and "a" "b" "c") (and "d" (or "e" "f") "g h i") "j")`,
	}, {
		input: `(a or b) and c`,
		want:  `(and (or "a" "b") "c")`,
	}, {
		input: `(repo:a (file:b (file:c or file:d) (file:e or file:f)))`,
		want:  `(or (and "repo:a" "file:b" "file:c" "file:e") (and "repo:a" "file:b" "file:d" "file:e") (and "repo:a" "file:b" "file:c" "file:f") (and "repo:a" "file:b" "file:d" "file:f"))`,
	}, {
		input: `(repo:a (file:b (file:c or file:d) file:q (file:e or file:f)))`,
		want:  `(or (and "repo:a" "file:b" "file:c" "file:q" "file:e") (and "repo:a" "file:b" "file:d" "file:q" "file:e") (and "repo:a" "file:b" "file:c" "file:q" "file:f") (and "repo:a" "file:b" "file:d" "file:q" "file:f"))`,
	}, {
		input: `(repo:a b) or (repo:c d)`,
		want:  `(or (and "repo:a" "b") (and "repo:c" "d"))`,
		// Bug. See: https://github.com/sourcegraph/sourcegraph/issues/34018
		// }, {
		// 	input: `repo:a b or repo:c d`,
		// 	want:  `(or (and "repo:a" "b") (and "repo:c" "d"))`,
	}, {
		input: `(repo:a b) and (repo:c d)`,
		want:  `(and "repo:a" "repo:c" "b" "d")`,
	}, {
		input: `(repo:a or repo:b) (c or d)`,
		want:  `(or (and "repo:a" (or "c" "d")) (and "repo:b" (or "c" "d")))`,
	}, {
		input: `(repo:a (b or c)) or (repo:d e f)`,
		want:  `(or (and "repo:a" (or "b" "c")) (and "repo:d" "e f"))`,
	}, {
		input: `((repo:a b) or c) or (repo:d e f)`,
		want:  `(or (and "repo:a" "b") "c" (and "repo:d" "e f"))`,
	}, {
		input: `(repo:a or repo:b) (c and (d or e))`,
		want:  `(or (and "repo:a" "c" (or "d" "e")) (and "repo:b" "c" (or "d" "e")))`,
	}}
	for _, c := range cases {
		t.Run("Map query", func(t *testing.T) {
			plan, err := Pipeline(Init(c.input, SearchTypeLiteral))
			require.NoError(t, err)
			got := plan.ToQ().String()
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
			query, _ := Parse(c.input, SearchTypeRegex)
			got := toString(Map(query, c.fns...))
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
			want:  `("repo:foo@a" (and "bar" "bas"))`,
		},
		{
			input: "(repo:foo rev:a) or (repo:foo rev:b)",
			want:  `("repo:foo@a") OR ("repo:foo@b")`,
		},
		{
			input: "repo:foo file:bas qux AND (rev:a or rev:b)",
			want:  `("repo:foo@a" "file:bas" "qux") OR ("repo:foo@b" "file:bas" "qux")`,
		},
		{
			input: "repo:foo rev:4.2.1 repo:has.file(content:fix)",
			want:  `("repo:foo@4.2.1" "repo:has.file(content:fix)")`,
		},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			plan, _ := Pipeline(InitRegexp(c.input))

			var queriesStr []string
			for _, basic := range plan {
				p := ConcatRevFilters(basic)
				queriesStr = append(queriesStr, toString(p.ToParseTree()))
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
			plan, _ := Pipeline(InitRegexp(c.input))
			p := MapPlan(plan, ConcatRevFilters)
			if diff := cmp.Diff(c.want, toString(p.ToQ())); diff != "" {
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

	autogold.Expect("pattern").Equal(t, test("repo:stuff pattern", "repo"))
	autogold.Expect("alias-pattern").Equal(t, test("r:stuff alias-pattern", "repo"))
}

func TestSubstituteCountAll(t *testing.T) {
	test := func(input string) string {
		query, _ := Parse(input, SearchTypeLiteral)
		q := SubstituteCountAll(query)
		return toString(q)
	}

	autogold.Expect(`(and "count:99999999" "foo")`).Equal(t, test("foo count:all"))
	autogold.Expect(`(and "count:99999999" "foo")`).Equal(t, test("foo count:ALL"))
	autogold.Expect(`(and "count:3" "foo")`).Equal(t, test("foo count:3"))
	autogold.Expect(`(or (and "count:3" "foo") (and "count:99999999" "bar"))`).Equal(t, test("(foo count:3) or (bar count:all)"))
}
