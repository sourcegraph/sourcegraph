package graphqlbackend

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/go-langserver/pkg/lsp"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/gituri"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestMakeFileMatchURIFromSymbol(t *testing.T) {
	db := new(dbtesting.MockDB)

	symbol := result.Symbol{
		Name:    "test",
		Path:    "foo/bar",
		Line:    0,
		Pattern: "",
	}
	baseURI, _ := gituri.Parse("https://github.com/foo/bar")

	repoResolver := NewRepositoryResolver(db, &types.Repo{ID: 1, Name: "repo"})
	sr := &result.SymbolMatch{symbol, baseURI, "go"}

	tests := []struct {
		rev  string
		want string
	}{
		{"", "git://repo#foo/bar"},
		{"test", "git://repo?test#foo/bar"},
	}

	for _, test := range tests {
		got := makeFileMatchURI(repoResolver.URL(), test.rev, sr.URI().Fragment)
		if got != test.want {
			t.Errorf("rev(%v) got %v want %v", test.rev, got, test.want)
		}
	}
}

func TestLimitingSymbolResults(t *testing.T) {
	db := new(dbtesting.MockDB)

	t.Run("empty case", func(t *testing.T) {
		var res []*FileMatchResolver

		t.Run("symbol count is 0", func(t *testing.T) {
			nsym := symbolCount(res)
			if nsym != 0 {
				t.Errorf("symbolCount(res) = %d, want 0", nsym)
			}
		})

		t.Run("limiting does not change", func(t *testing.T) {
			res2 := limitSymbolResults(res, 0)
			if len(res2) != 0 {
				t.Errorf("res2 = %+v, want empty", res2)
			}
		})
	})

	t.Run("one file match, one symbol", func(t *testing.T) {
		res := mkSymbolFileMatchResolvers(db, []*result.SymbolMatch{{
			Symbol: result.Symbol{
				Name: "symbol-name-1",
			},
		}})

		t.Run("symbol count is 1", func(t *testing.T) {
			nsym := symbolCount(res)
			if nsym != 1 {
				t.Errorf("symbolCount(res) = %d, want 1", nsym)
			}
		})

		t.Run("limit 0 => no file matches", func(t *testing.T) {
			res2 := limitSymbolResults(res, 0)
			if len(res2) != 0 {
				t.Errorf("res2 = %+v, want empty", res2)
			}
		})

		t.Run("limit 1 => unchanged", func(t *testing.T) {
			res2 := limitSymbolResults(res, 1)
			if !reflect.DeepEqual(res2, res) {
				t.Errorf("res2 = %+v, want %+v", res2, res)
			}
		})
	})

	t.Run("two file matches, one symbol per file", func(t *testing.T) {
		res := mkSymbolFileMatchResolvers(db, []*result.SymbolMatch{{
			Symbol: result.Symbol{
				Name: "symbol-name-1",
			},
		}}, []*result.SymbolMatch{{
			Symbol: result.Symbol{
				Name: "symbol-name-2",
			},
		}})

		t.Run("symbol count is 2", func(t *testing.T) {
			nsym := symbolCount(res)
			if nsym != 2 {
				t.Errorf("symbolCount(res) = %d, want 2", nsym)
			}
		})

		t.Run("limit 0 => no file matches", func(t *testing.T) {
			res2 := limitSymbolResults(res, 0)
			if len(res2) != 0 {
				t.Errorf("res2 = %+v, want empty", res2)
			}
		})

		t.Run("limit 1 => one file match", func(t *testing.T) {
			wantRes2 := res[:1]
			res2 := limitSymbolResults(res, 1)
			if !reflect.DeepEqual(res2, wantRes2) {
				t.Errorf("res2 = %+v, want %+v", res2, wantRes2)
			}
		})

		t.Run("limit 2 => unchanged", func(t *testing.T) {
			res2 := limitSymbolResults(res, 2)
			if !reflect.DeepEqual(res2, res) {
				t.Errorf("res2 = %+v, want %+v", res2, res)
			}
		})
	})

	t.Run("two file matches, multiple symbols per file", func(t *testing.T) {
		res := mkSymbolFileMatchResolvers(db, []*result.SymbolMatch{
			{Symbol: result.Symbol{Name: "symbol-name-1"}},
			{Symbol: result.Symbol{Name: "symbol-name-2"}},
		}, []*result.SymbolMatch{
			{Symbol: result.Symbol{Name: "symbol-name-3"}},
			{Symbol: result.Symbol{Name: "symbol-name-4"}},
		})

		t.Run("symbol count is 4", func(t *testing.T) {
			nsym := symbolCount(res)
			if nsym != 4 {
				t.Errorf("symbolCount(res) = %d, want 2", nsym)
			}
		})

		testCases := []struct {
			name  string
			limit int
			want  []*FileMatchResolver
		}{
			{
				name: "limit 0 => no file matches",
				want: []*FileMatchResolver{},
			},
			{
				name:  "limit 1 => one file match with one symbol",
				limit: 1,
				want: mkSymbolFileMatchResolvers(db, []*result.SymbolMatch{
					{Symbol: result.Symbol{Name: "symbol-name-1"}},
				}),
			},
			{
				name:  "limit 2 => one file match with all symbols",
				limit: 2,
				want: mkSymbolFileMatchResolvers(db, []*result.SymbolMatch{
					{Symbol: result.Symbol{Name: "symbol-name-1"}},
					{Symbol: result.Symbol{Name: "symbol-name-2"}},
				}),
			},
			{
				name:  "limit 3 => two file matches with three symbols",
				limit: 3,
				want: mkSymbolFileMatchResolvers(db, []*result.SymbolMatch{
					{Symbol: result.Symbol{Name: "symbol-name-1"}},
					{Symbol: result.Symbol{Name: "symbol-name-2"}},
				}, []*result.SymbolMatch{
					{Symbol: result.Symbol{Name: "symbol-name-3"}},
				}),
			},
			{
				name:  "limit 4 => two file matches with all symbols",
				limit: 4,
				want: mkSymbolFileMatchResolvers(db, []*result.SymbolMatch{
					{Symbol: result.Symbol{Name: "symbol-name-1"}},
					{Symbol: result.Symbol{Name: "symbol-name-2"}},
				}, []*result.SymbolMatch{
					{Symbol: result.Symbol{Name: "symbol-name-3"}},
					{Symbol: result.Symbol{Name: "symbol-name-4"}},
				}),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				res2 := limitSymbolResults(res, tc.limit)
				if len(res2) != len(tc.want) {
					t.Errorf("len(res2)=%d, len(want)=%d", len(res2), len(tc.want))
				}

				if !reflect.DeepEqual(res2, tc.want) {
					t.Error(cmp.Diff(res2, tc.want))
				}
			})
		}
	})
}

func TestSymbolRange(t *testing.T) {
	t.Run("unescaped pattern", func(t *testing.T) {
		want := lsp.Range{
			Start: lsp.Position{Line: 0, Character: 37},
			End:   lsp.Position{Line: 0, Character: 40},
		}
		got := symbolRange(result.Symbol{Line: 1, Name: "baz", Pattern: `/^bar() { var regex = \/.*\\\/\/; function baz() { }  } $/`})
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatal(diff)
		}
	})
}

func mkSymbolFileMatchResolvers(db dbutil.DB, symbols ...[]*result.SymbolMatch) []*FileMatchResolver {
	var resolvers []*FileMatchResolver
	for _, s := range symbols {
		resolvers = append(resolvers, mkFileMatchResolver(db, result.FileMatch{
			Symbols: s,
		}))
	}
	return resolvers
}
