package search

import (
	"regexp/syntax" //nolint:depguard // using the grafana fork of regexp clashes with zoekt, which uses the std regexp/syntax.
	"testing"

	"github.com/grafana/regexp"
	"github.com/hexops/autogold/v2"
	"github.com/sourcegraph/zoekt/query"
	"github.com/stretchr/testify/require"
)

func TestLongestLiteral(t *testing.T) {
	cases := map[string]string{
		"foo":       "foo",
		"FoO":       "FoO",
		"(?m:^foo)": "foo",
		"(?m:^FoO)": "FoO",
		"[Z]":       "Z",

		`\wddSuballocation\(dump`:    "ddSuballocation(dump",
		`\wfoo(\dlongest\wbam)\dbar`: "longest",

		`(foo\dlongest\dbar)`:  "longest",
		`(foo\dlongest\dbar)+`: "longest",
		`(foo\dlongest\dbar)*`: "",

		"(foo|bar)":     "",
		"[A-Z]":         "",
		"[^A-Z]":        "",
		"[abB-Z]":       "",
		"([abB-Z]|FoO)": "",
		`[@-\[]`:        "",
		`\S`:            "",
	}

	metaLiteral := "AddSuballocation(dump->guid(), system_allocator_name)"
	cases[regexp.QuoteMeta(metaLiteral)] = metaLiteral

	for expr, want := range cases {
		re, err := syntax.Parse(expr, syntax.Perl)
		if err != nil {
			t.Fatal(expr, err)
		}
		re = re.Simplify()
		got := longestLiteral(re)
		if want != got {
			t.Errorf("longestLiteral(%q) == %q != %q", expr, got, want)
		}
	}
}

func TestToZoektQuery(t *testing.T) {
	m := &andMatchTree{
		children: []matchTree{
			&orMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re:        regexp.MustCompile("aaaaa"),
						isNegated: true,
					},
					&regexMatchTree{
						re:    regexp.MustCompile("bbbb*"),
						boost: true,
					},
				},
			},
			&regexMatchTree{
				re:         regexp.MustCompile("cccc?"),
				ignoreCase: true,
			},
		},
	}

	cases := []struct {
		name         string
		matchContent bool
		matchPath    bool
		want         autogold.Value
	}{{
		name:         "matches content only",
		matchContent: true,
		matchPath:    false,
		want:         autogold.Expect(`(and (or (not case_regex:"aaaaa") (boost 20.00 case_regex:"bbbb*")) regex:"cccc?")`),
	}, {
		name:         "matches path only",
		matchContent: false,
		matchPath:    true,
		want:         autogold.Expect(`(and (or (not case_file_regex:"aaaaa") (boost 20.00 case_file_regex:"bbbb*")) file_regex:"cccc?")`),
	}, {
		name:         "matches content and path",
		matchContent: true,
		matchPath:    true,
		want:         autogold.Expect(`(and (or (not case_regex:"aaaaa") (not case_file_regex:"aaaaa") (boost 20.00 (or case_regex:"bbbb*" case_file_regex:"bbbb*"))) (or regex:"cccc?" file_regex:"cccc?"))`),
	},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := m.ToZoektQuery(c.matchContent, c.matchPath)
			if err != nil {
				t.Fatal(err)
			}
			c.want.Equal(t, query.Simplify(got).String())
		})
	}
}

func TestMatchesString(t *testing.T) {
	m := &orMatchTree{
		children: []matchTree{
			&andMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re:        regexp.MustCompile("aaa"),
						isNegated: true,
					},
					&regexMatchTree{
						re: regexp.MustCompile("bbb*"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("ccc*"),
					},
				},
			},
			&regexMatchTree{
				re:         regexp.MustCompile("ddd?"),
				ignoreCase: true,
			},
		},
	}

	cases := []struct {
		name      string
		test      string
		wantMatch bool
	}{
		{
			name:      "first 'or' clause matches",
			test:      "bbbbbccc",
			wantMatch: true,
		},
		{
			name:      "negated 'and' clause does not match",
			test:      "aaabbbccc",
			wantMatch: false,
		},
		{
			name:      "case insensitive 'or' clause matches",
			test:      "zzzzzDDD",
			wantMatch: true,
		},
	}

	for _, c := range cases {
		t.Run(c.test, func(t *testing.T) {
			match := m.MatchesString(c.test)
			require.Equal(t, c.wantMatch, match)
		})
	}
}

func TestMatchesFile(t *testing.T) {
	m := &orMatchTree{
		children: []matchTree{
			&andMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re:        regexp.MustCompile("and"),
						isNegated: true,
					},
					&regexMatchTree{
						re: regexp.MustCompile("file"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("the"),
					},
				},
			},
			&regexMatchTree{
				re:         regexp.MustCompile("here"),
				ignoreCase: true,
			},
		},
	}

	cases := []struct {
		m           matchTree
		file        string
		wantMatch   bool
		wantMatches int
	}{
		{
			m:           m,
			file:        "this is the first file",
			wantMatch:   true,
			wantMatches: 2,
		},
		{
			m:           m,
			file:        "here is the second fileee",
			wantMatch:   true,
			wantMatches: 3,
		},
		{
			m:           m,
			file:        "... and another file!",
			wantMatch:   false,
			wantMatches: 0,
		},
		{
			m: &regexMatchTree{
				re:        regexp.MustCompile("excluded"),
				isNegated: true,
			},
			file: "here's a file",
			// this matches, but produces no matched ranges
			wantMatch:   true,
			wantMatches: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			match, matches := c.m.MatchesFile([]byte(c.file), 1000)
			require.Equal(t, c.wantMatch, match)
			require.Equal(t, c.wantMatches, len(matches))
		})
	}
}

func TestMatchesFileLimits(t *testing.T) {
	file := []byte("the file that mentions file a lot ... the file file")

	cases := []struct {
		name        string
		m           matchTree
		limit       int
		wantMatches int
	}{
		{
			name: "'or' matchTree with limit",
			m: &orMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re: regexp.MustCompile("file"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("the"),
					},
				},
			},
			limit:       3,
			wantMatches: 3,
		},
		{
			name: "'or' matchTree without limit",
			m: &orMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re: regexp.MustCompile("file"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("the"),
					},
				},
			},
			limit:       10,
			wantMatches: 6,
		},
		{
			name: "'and' matchTree with limit",
			m: &andMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re: regexp.MustCompile("file"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("the"),
					},
				},
			},
			limit:       3,
			wantMatches: 3,
		},
		{
			name: "'and' matchTree with negation",
			m: &andMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re:        regexp.MustCompile("excluded"),
						isNegated: true,
					},
					&regexMatchTree{
						re: regexp.MustCompile("file"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("the"),
					},
				},
			},
			limit:       3,
			wantMatches: 3,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, matches := c.m.MatchesFile(file, c.limit)
			require.Equal(t, c.wantMatches, len(matches))
		})
	}
}

func TestMergeMatches(t *testing.T) {
	file := []byte("philodendron and monsteras")

	cases := []struct {
		name        string
		m           matchTree
		limit       int
		wantMatches [][]int
	}{
		{
			name: "'or' matchTree with range overlap",
			m: &orMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re: regexp.MustCompile("steras"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("monster"),
					},
				},
			},
			limit:       100, // random high limit
			wantMatches: [][]int{{17, 24}},
		},
		{
			name: "'and' matchTree with adjacent ranges",
			m: &andMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re: regexp.MustCompile("dendron"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("philo"),
					},
				},
			},
			limit:       100, // random high limit
			wantMatches: [][]int{{0, 5}, {5, 12}},
		},
		{
			name: "limit applied after merging",
			m: &orMatchTree{
				children: []matchTree{
					&regexMatchTree{
						re: regexp.MustCompile("monster"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("tera"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("philo"),
					},
					&regexMatchTree{
						re: regexp.MustCompile("lode"),
					},
				},
			},
			limit:       2,
			wantMatches: [][]int{{0, 5}, {17, 24}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, matches := c.m.MatchesFile(file, c.limit)
			require.Equal(t, c.wantMatches, matches)
		})
	}
}
