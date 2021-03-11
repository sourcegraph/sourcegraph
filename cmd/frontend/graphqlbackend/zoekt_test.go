package graphqlbackend

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/zoekt"
	zoektquery "github.com/google/zoekt/query"
	zoektrpc "github.com/google/zoekt/rpc"
	"github.com/keegancsmith/sqlf"

	searchrepos "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/search"
	searchbackend "github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	zoektutil "github.com/sourcegraph/sourcegraph/internal/search/zoekt"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestIndexedSearch(t *testing.T) {
	db := new(dbtesting.MockDB)

	zeroTimeoutCtx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	type args struct {
		ctx             context.Context
		query           string
		patternInfo     *search.TextPatternInfo
		repos           []*search.RepositoryRevisions
		useFullDeadline bool
		results         []zoekt.FileMatch
		since           func(time.Time) time.Duration
	}

	reposHEAD := makeRepositoryRevisions("foo/bar", "foo/foobar")
	zoektRepos := []*zoekt.RepoListEntry{{
		Repository: zoekt.Repository{
			Name:     "foo/bar",
			Branches: []zoekt.RepositoryBranch{{Name: "HEAD", Version: "barHEADSHA"}, {Name: "dev", Version: "bardevSHA"}, {Name: "main", Version: "barmainSHA"}},
		},
	}, {
		Repository: zoekt.Repository{
			Name:     "foo/foobar",
			Branches: []zoekt.RepositoryBranch{{Name: "HEAD", Version: "foobarHEADSHA"}},
		},
	}}

	tests := []struct {
		name               string
		args               args
		wantMatchCount     int
		wantMatchURLs      []string
		wantMatchInputRevs []string
		wantUnindexed      []*search.RepositoryRevisions
		wantCommon         streaming.Stats
		wantErr            bool
	}{
		{
			name: "no matches",
			args: args{
				ctx:             context.Background(),
				patternInfo:     &search.TextPatternInfo{},
				repos:           reposHEAD,
				useFullDeadline: false,
				since:           func(time.Time) time.Duration { return time.Second - time.Millisecond },
			},
			wantErr: false,
		},
		{
			name: "no matches timeout",
			args: args{
				ctx:             context.Background(),
				patternInfo:     &search.TextPatternInfo{},
				repos:           reposHEAD,
				useFullDeadline: false,
				since:           func(time.Time) time.Duration { return time.Minute },
			},
			wantCommon: streaming.Stats{
				Status: mkStatusMap(map[string]search.RepoStatus{
					"foo/bar":    search.RepoStatusTimedout,
					"foo/foobar": search.RepoStatusTimedout,
				}),
			},
		},
		{
			name: "context timeout",
			args: args{
				ctx:             zeroTimeoutCtx,
				patternInfo:     &search.TextPatternInfo{},
				repos:           reposHEAD,
				useFullDeadline: true,
				since:           func(time.Time) time.Duration { return 0 },
			},
			wantErr: true,
		},
		{
			name: "results",
			args: args{
				ctx:             context.Background(),
				patternInfo:     &search.TextPatternInfo{FileMatchLimit: 100},
				repos:           makeRepositoryRevisions("foo/bar", "foo/foobar"),
				useFullDeadline: false,
				results: []zoekt.FileMatch{
					{
						Repository: "foo/bar",
						Branches:   []string{"HEAD"},
						FileName:   "baz.go",
						LineMatches: []zoekt.LineMatch{
							{
								Line: []byte("I'm like 1.5+ hours into writing this test :'("),
								LineFragments: []zoekt.LineFragmentMatch{
									{LineOffset: 0, MatchLength: 5},
								},
							},
							{
								Line: []byte("I'm ready for the rain to stop."),
								LineFragments: []zoekt.LineFragmentMatch{
									{LineOffset: 0, MatchLength: 5},
									{LineOffset: 5, MatchLength: 10},
								},
							},
						},
					},
					{
						Repository: "foo/foobar",
						Branches:   []string{"HEAD"},
						FileName:   "baz.go",
						LineMatches: []zoekt.LineMatch{
							{
								Line: []byte("s/rain/pain"),
								LineFragments: []zoekt.LineFragmentMatch{
									{LineOffset: 0, MatchLength: 5},
									{LineOffset: 5, MatchLength: 2},
								},
							},
						},
					},
				},
				since: func(time.Time) time.Duration { return 0 },
			},
			wantMatchCount: 5,
			wantMatchURLs: []string{
				"git://foo/bar#baz.go",
				"git://foo/foobar#baz.go",
			},
			wantMatchInputRevs: []string{
				"",
				"",
			},
			wantErr: false,
		},
		{
			name: "results multi-branch",
			args: args{
				ctx:             context.Background(),
				patternInfo:     &search.TextPatternInfo{FileMatchLimit: 100},
				repos:           makeRepositoryRevisions("foo/bar@HEAD:dev:main"),
				useFullDeadline: false,
				results: []zoekt.FileMatch{
					{
						Repository: "foo/bar",
						// baz.go is the same in HEAD and dev
						Branches: []string{"HEAD", "dev"},
						FileName: "baz.go",
					},
					{
						Repository: "foo/bar",
						Branches:   []string{"dev"},
						FileName:   "bam.go",
					},
				},
				since: func(time.Time) time.Duration { return 0 },
			},
			wantMatchCount: 3,
			wantMatchURLs: []string{
				"git://foo/bar?HEAD#baz.go",
				"git://foo/bar?dev#baz.go",
				"git://foo/bar?dev#bam.go",
			},
			wantMatchInputRevs: []string{
				"HEAD",
				"dev",
				"dev",
			},
			wantErr: false,
		},
		{
			// if we search a branch that is indexed and unindexed, we should
			// split the repository revision into the indexed and unindexed
			// parts.
			name: "split branch",
			args: args{
				ctx:             context.Background(),
				patternInfo:     &search.TextPatternInfo{FileMatchLimit: 100},
				repos:           makeRepositoryRevisions("foo/bar@HEAD:unindexed"),
				useFullDeadline: false,
				results: []zoekt.FileMatch{
					{
						Repository: "foo/bar",
						Branches:   []string{"HEAD"},
						FileName:   "baz.go",
					},
				},
			},
			wantUnindexed: makeRepositoryRevisions("foo/bar@unindexed"),
			wantMatchURLs: []string{
				"git://foo/bar?HEAD#baz.go",
			},
			wantMatchCount:     1,
			wantMatchInputRevs: []string{"HEAD"},
		},
		{
			// Fallback to unindexed search if the query contains ref-globs.
			name: "ref-glob with explicit /*",
			args: args{
				ctx:             context.Background(),
				query:           "repo:foo/bar@*refs/heads/*",
				patternInfo:     &search.TextPatternInfo{FileMatchLimit: 100},
				repos:           makeRepositoryRevisions("foo/bar@HEAD"),
				useFullDeadline: false,
				results:         []zoekt.FileMatch{},
			},
			wantUnindexed:      makeRepositoryRevisions("foo/bar@HEAD"),
			wantMatchURLs:      nil,
			wantMatchInputRevs: nil,
		},
		{
			name: "ref-glob with implicit /*",
			args: args{
				ctx:             context.Background(),
				query:           "repo:foo/bar@*refs/tags",
				patternInfo:     &search.TextPatternInfo{FileMatchLimit: 100},
				repos:           makeRepositoryRevisions("foo/bar@HEAD"),
				useFullDeadline: false,
				results:         []zoekt.FileMatch{},
			},
			wantUnindexed:      makeRepositoryRevisions("foo/bar@HEAD"),
			wantMatchURLs:      nil,
			wantMatchInputRevs: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := query.ParseLiteral(tt.args.query)
			if err != nil {
				t.Fatal(err)
			}

			args := &search.TextParameters{
				Query:           q,
				PatternInfo:     tt.args.patternInfo,
				RepoPromise:     (&search.Promise{}).Resolve(tt.args.repos),
				UseFullDeadline: tt.args.useFullDeadline,
				Zoekt: &searchbackend.Zoekt{
					Client: &searchbackend.FakeSearcher{
						Result: &zoekt.SearchResult{Files: tt.args.results},
						Repos:  zoektRepos,
					},
					DisableCache: true,
				},
			}

			indexed, err := newIndexedSearchRequest(context.Background(), db, args, textRequest, StreamFunc(func(SearchEvent) {}))
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.wantUnindexed, indexed.Unindexed, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("unindexed mismatch (-want +got):\n%s", diff)
			}

			indexed.since = tt.args.since

			// This is a quick fix which will break once we enable the zoekt client for true streaming.
			// Once we return more than one event we have to account for the proper order of results
			// in the tests.
			gotResults, gotCommon, err := collectStream(func(stream Sender) error {
				return indexed.Search(tt.args.ctx, stream)
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("zoektSearchHEAD() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			gotFm, err := searchResultsToFileMatchResults(gotResults)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(&tt.wantCommon, &gotCommon, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("common mismatch (-want +got):\n%s", diff)
			}

			var gotMatchCount int
			var gotMatchURLs []string
			var gotMatchInputRevs []string
			for _, m := range gotFm {
				gotMatchCount += int(m.ResultCount())
				gotMatchURLs = append(gotMatchURLs, m.Resource())
				if m.InputRev != nil {
					gotMatchInputRevs = append(gotMatchInputRevs, *m.InputRev)
				}
			}
			if diff := cmp.Diff(tt.wantMatchURLs, gotMatchURLs); diff != "" {
				t.Errorf("match URLs mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantMatchInputRevs, gotMatchInputRevs); diff != "" {
				t.Errorf("match InputRevs mismatch (-want +got):\n%s", diff)
			}
			if gotMatchCount != tt.wantMatchCount {
				t.Errorf("gotMatchCount = %v, want %v", gotMatchCount, tt.wantMatchCount)
			}
		})
	}
}

func TestZoektIndexedRepos(t *testing.T) {
	repos := makeRepositoryRevisions(
		"foo/indexed-one@",
		"foo/indexed-two@",
		"foo/indexed-three@",
		"foo/unindexed-one",
		"foo/unindexed-two",
		"foo/multi-rev@a:b",
	)

	zoektRepos := map[string]*zoekt.Repository{}
	for _, r := range []*zoekt.Repository{{
		Name:     "foo/indexed-one",
		Branches: []zoekt.RepositoryBranch{{Name: "HEAD", Version: "deadbeef"}},
	}, {
		Name:     "foo/indexed-two",
		Branches: []zoekt.RepositoryBranch{{Name: "HEAD", Version: "deadbeef"}},
	}, {
		Name: "foo/indexed-three",
		Branches: []zoekt.RepositoryBranch{
			{Name: "HEAD", Version: "deadbeef"},
			{Name: "foobar", Version: "deadcow"},
		},
	}} {
		zoektRepos[r.Name] = r
	}

	makeIndexed := func(repos []*search.RepositoryRevisions) []*search.RepositoryRevisions {
		var indexed []*search.RepositoryRevisions
		for _, r := range repos {
			rev := &search.RepositoryRevisions{
				Repo: r.Repo,
				Revs: r.Revs,
			}
			indexed = append(indexed, rev)
		}
		return indexed
	}

	cases := []struct {
		name      string
		repos     []*search.RepositoryRevisions
		indexed   []*search.RepositoryRevisions
		unindexed []*search.RepositoryRevisions
	}{{
		name:      "all",
		repos:     repos,
		indexed:   makeIndexed(repos[:3]),
		unindexed: repos[3:],
	}, {
		name:      "one unindexed",
		repos:     repos[3:4],
		indexed:   repos[:0],
		unindexed: repos[3:4],
	}, {
		name:      "one indexed",
		repos:     repos[:1],
		indexed:   makeIndexed(repos[:1]),
		unindexed: repos[:0],
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			indexed, unindexed := zoektIndexedRepos(zoektRepos, tc.repos, nil)

			if diff := cmp.Diff(repoRevsSliceToMap(tc.indexed), indexed.repoRevs); diff != "" {
				t.Error("unexpected indexed:", diff)
			}
			if diff := cmp.Diff(tc.unindexed, unindexed); diff != "" {
				t.Error("unexpected unindexed:", diff)
			}
		})
	}
}

func Benchmark_zoektIndexedRepos(b *testing.B) {
	repoNames := []string{}
	zoektRepos := map[string]*zoekt.Repository{}

	for i := 0; i < 200000; i++ {
		indexedName := fmt.Sprintf("foo/indexed-%d@", i)
		unindexedName := fmt.Sprintf("foo/unindexed-%d@", i)

		repoNames = append(repoNames, indexedName, unindexedName)

		zoektName := strings.TrimSuffix(indexedName, "@")
		zoektRepos[zoektName] = &zoekt.Repository{
			Name:     zoektName,
			Branches: []zoekt.RepositoryBranch{{Name: "HEAD", Version: "deadbeef"}},
		}
	}

	repos := makeRepositoryRevisions(repoNames...)

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		_, _ = zoektIndexedRepos(zoektRepos, repos, nil)
	}
}

func TestZoektResultCountFactor(t *testing.T) {
	cases := []struct {
		name         string
		numRepos     int
		globalSearch bool
		pattern      *search.TextPatternInfo
		want         int
	}{
		{
			name:     "One repo implies max scaling factor",
			numRepos: 1,
			pattern:  &search.TextPatternInfo{},
			want:     100,
		},
		{
			name:     "Eleven repos implies a scaling factor between min and max",
			numRepos: 11,
			pattern:  &search.TextPatternInfo{},
			want:     8,
		},
		{
			name:     "More than 500 repos implies a min scaling factor",
			numRepos: 501,
			pattern:  &search.TextPatternInfo{},
			want:     1,
		},
		{
			name:     "Setting a count greater than defautl max results (30) adapts scaling factor",
			numRepos: 501,
			pattern:  &search.TextPatternInfo{FileMatchLimit: 100},
			want:     10,
		},
		{
			name:         "for global searches, k should be 1",
			numRepos:     0,
			globalSearch: true,
			pattern:      &search.TextPatternInfo{},
			want:         1,
		},
		{
			name:         "for global searches, k should be 1, adjusted by the FileMatchLimit",
			numRepos:     0,
			globalSearch: true,
			pattern:      &search.TextPatternInfo{FileMatchLimit: 100},
			want:         10,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := zoektutil.ResultCountFactor(tt.numRepos, tt.pattern.FileMatchLimit, tt.globalSearch)
			if tt.want != got {
				t.Fatalf("Want scaling factor %d but got %d", tt.want, got)
			}
		})
	}
}

func TestQueryToZoektQuery(t *testing.T) {
	cases := []struct {
		Name    string
		Type    indexedRequestType
		Pattern *search.TextPatternInfo
		Query   string
	}{
		{
			Name: "substr",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              false,
				Pattern:                      "foo",
				IncludePatterns:              nil,
				ExcludePattern:               "",
				PathPatternsAreCaseSensitive: false,
			},
			Query: "foo case:no",
		},
		{
			Name: "symbol substr",
			Type: symbolRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              false,
				Pattern:                      "foo",
				IncludePatterns:              nil,
				ExcludePattern:               "",
				PathPatternsAreCaseSensitive: false,
			},
			Query: "sym:foo case:no",
		},
		{
			Name: "regex",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              false,
				Pattern:                      "(foo).*?(bar)",
				IncludePatterns:              nil,
				ExcludePattern:               "",
				PathPatternsAreCaseSensitive: false,
			},
			Query: "(foo).*?(bar) case:no",
		},
		{
			Name: "path",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              false,
				Pattern:                      "foo",
				IncludePatterns:              []string{`\.go$`, `\.yaml$`},
				ExcludePattern:               `\bvendor\b`,
				PathPatternsAreCaseSensitive: false,
			},
			Query: `foo case:no f:\.go$ f:\.yaml$ -f:\bvendor\b`,
		},
		{
			Name: "case",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              true,
				Pattern:                      "foo",
				IncludePatterns:              []string{`\.go$`, `yaml`},
				ExcludePattern:               "",
				PathPatternsAreCaseSensitive: true,
			},
			Query: `foo case:yes f:\.go$ f:yaml`,
		},
		{
			Name: "casepath",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              true,
				Pattern:                      "foo",
				IncludePatterns:              []string{`\.go$`, `\.yaml$`},
				ExcludePattern:               `\bvendor\b`,
				PathPatternsAreCaseSensitive: true,
			},
			Query: `foo case:yes f:\.go$ f:\.yaml$ -f:\bvendor\b`,
		},
		{
			Name: "path matches only",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              false,
				Pattern:                      "test",
				IncludePatterns:              []string{},
				ExcludePattern:               ``,
				PathPatternsAreCaseSensitive: true,
				PatternMatchesContent:        false,
				PatternMatchesPath:           true,
			},
			Query: `f:test`,
		},
		{
			Name: "content matches only",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              false,
				Pattern:                      "test",
				IncludePatterns:              []string{},
				ExcludePattern:               ``,
				PathPatternsAreCaseSensitive: true,
				PatternMatchesContent:        true,
				PatternMatchesPath:           false,
			},
			Query: `c:test`,
		},
		{
			Name: "content and path matches 1",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              false,
				Pattern:                      "test",
				IncludePatterns:              []string{},
				ExcludePattern:               ``,
				PathPatternsAreCaseSensitive: true,
				PatternMatchesContent:        true,
				PatternMatchesPath:           true,
			},
			Query: `test`,
		},
		{
			Name: "content and path matches 2",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				IsCaseSensitive:              false,
				Pattern:                      "test",
				IncludePatterns:              []string{},
				ExcludePattern:               ``,
				PathPatternsAreCaseSensitive: true,
				PatternMatchesContent:        false,
				PatternMatchesPath:           false,
			},
			Query: `test`,
		},
		{
			Name: "repos must include",
			Type: textRequest,
			Pattern: &search.TextPatternInfo{
				IsRegExp:                     true,
				Pattern:                      "foo",
				FilePatternsReposMustInclude: []string{`\.go$`, `\.yaml$`},
				FilePatternsReposMustExclude: []string{`\.java$`, `\.xml$`},
			},
			Query: `foo (type:repo file:\.go$) (type:repo file:\.yaml$) -(type:repo file:\.java$) -(type:repo file:\.xml$)`,
		},
	}
	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			q, err := zoektquery.Parse(tt.Query)
			if err != nil {
				t.Fatalf("failed to parse %q: %v", tt.Query, err)
			}
			got, err := queryToZoektQuery(tt.Pattern, tt.Type)
			if err != nil {
				t.Fatal("queryToZoektQuery failed:", err)
			}
			if !queryEqual(got, q) {
				t.Fatalf("mismatched queries\ngot  %s\nwant %s", got.String(), q.String())
			}
		})
	}
}

func queryEqual(a, b zoektquery.Q) bool {
	sortChildren := func(q zoektquery.Q) zoektquery.Q {
		switch s := q.(type) {
		case *zoektquery.And:
			sort.Slice(s.Children, func(i, j int) bool {
				return s.Children[i].String() < s.Children[j].String()
			})
		case *zoektquery.Or:
			sort.Slice(s.Children, func(i, j int) bool {
				return s.Children[i].String() < s.Children[j].String()
			})
		}
		return q
	}
	return zoektquery.Map(a, sortChildren).String() == zoektquery.Map(b, sortChildren).String()
}

func BenchmarkSearchResults(b *testing.B) {
	db := new(dbtesting.MockDB)

	minimalRepos, _, zoektRepos := generateRepos(5000)
	zoektFileMatches := generateZoektMatches(50)

	z := &searchbackend.Zoekt{
		Client: &searchbackend.FakeSearcher{
			Repos:  zoektRepos,
			Result: &zoekt.SearchResult{Files: zoektFileMatches},
		},
		DisableCache: true,
	}

	ctx := context.Background()

	database.Mocks.Repos.List = func(_ context.Context, op database.ReposListOptions) ([]*types.Repo, error) {
		return minimalRepos, nil
	}
	database.Mocks.Repos.Count = func(ctx context.Context, opt database.ReposListOptions) (int, error) {
		return len(minimalRepos), nil
	}
	defer func() { database.Mocks = database.MockStores{} }()

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		q, err := query.ParseLiteral(`print index:only count:350`)
		if err != nil {
			b.Fatal(err)
		}
		resolver := &searchResolver{
			db: db,
			SearchInputs: &SearchInputs{
				Query:        q,
				UserSettings: &schema.Settings{},
			},
			zoekt:    z,
			reposMu:  &sync.Mutex{},
			resolved: &searchrepos.Resolved{},
		}
		results, err := resolver.Results(ctx)
		if err != nil {
			b.Fatal("Results:", err)
		}
		if int(results.MatchCount()) != len(zoektFileMatches) {
			b.Fatalf("wrong results length. want=%d, have=%d\n", len(zoektFileMatches), results.MatchCount())
		}
	}
}

func BenchmarkIntegrationSearchResults(b *testing.B) {
	db := dbtesting.GetDB(b)

	ctx := context.Background()

	_, repos, zoektRepos := generateRepos(5000)
	zoektFileMatches := generateZoektMatches(50)

	zoektClient, cleanup := zoektRPC(&searchbackend.FakeSearcher{
		Repos:  zoektRepos,
		Result: &zoekt.SearchResult{Files: zoektFileMatches},
	})
	defer cleanup()
	z := &searchbackend.Zoekt{
		Client:       &searchbackend.StreamSearchAdapter{zoektClient},
		DisableCache: true,
	}

	rows := make([]*sqlf.Query, 0, len(repos))
	for _, r := range repos {
		rows = append(rows, sqlf.Sprintf(
			"(%s, %s, %s, %s, %s, %s, %s)",
			r.Name,
			r.Description,
			r.Fork,
			true,
			r.ExternalRepo.ServiceType,
			r.ExternalRepo.ServiceID,
			r.ExternalRepo.ID,
		))
	}

	q := sqlf.Sprintf(`
		INSERT INTO repo (
			name,
			description,
			fork,
			enabled,
			external_service_type,
			external_service_id,
			external_id
		)
		VALUES %s`,
		sqlf.Join(rows, ","),
	)

	_, err := dbconn.Global.ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		q, err := query.ParseLiteral(`print index:only count:350`)
		if err != nil {
			b.Fatal(err)
		}
		resolver := &searchResolver{
			db: db,
			SearchInputs: &SearchInputs{
				Query: q,
			},
			zoekt:    z,
			reposMu:  &sync.Mutex{},
			resolved: &searchrepos.Resolved{},
		}
		results, err := resolver.Results(ctx)
		if err != nil {
			b.Fatal("Results:", err)
		}
		if int(results.MatchCount()) != len(zoektFileMatches) {
			b.Fatalf("wrong results length. want=%d, have=%d\n", len(zoektFileMatches), results.MatchCount())
		}
	}
}

func generateRepos(count int) ([]*types.Repo, []*types.Repo, []*zoekt.RepoListEntry) {
	var reposWithIDs []*types.Repo
	var repos []*types.Repo
	var zoektRepos []*zoekt.RepoListEntry

	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("repo-%d", i)

		repoWithIDs := &types.Repo{
			ID:   api.RepoID(i),
			Name: api.RepoName(name),
			ExternalRepo: api.ExternalRepoSpec{
				ID:          name,
				ServiceType: extsvc.TypeGitHub,
				ServiceID:   "https://github.com",
			}}

		reposWithIDs = append(reposWithIDs, repoWithIDs)

		repos = append(repos, &types.Repo{

			ID:           repoWithIDs.ID,
			Name:         repoWithIDs.Name,
			ExternalRepo: repoWithIDs.ExternalRepo,
			URI:          fmt.Sprintf("https://github.com/foobar/%s", repoWithIDs.Name),
			Description:  "this repositoriy contains a side project that I haven't maintained in 2 years",
		})

		zoektRepos = append(zoektRepos, &zoekt.RepoListEntry{
			Repository: zoekt.Repository{
				Name:     name,
				Branches: []zoekt.RepositoryBranch{{Name: "HEAD", Version: "deadbeef"}},
			},
		})
	}
	return reposWithIDs, repos, zoektRepos
}

func generateZoektMatches(count int) []zoekt.FileMatch {
	var zoektFileMatches []zoekt.FileMatch
	for i := 1; i <= count; i++ {
		repoName := fmt.Sprintf("repo-%d", i)
		fileName := fmt.Sprintf("foobar-%d.go", i)

		zoektFileMatches = append(zoektFileMatches, zoekt.FileMatch{
			Score:      5.0,
			FileName:   fileName,
			Repository: repoName, // Important: this needs to match a name in `repos`
			Branches:   []string{"master"},
			LineMatches: []zoekt.LineMatch{
				{
					Line: nil,
				},
			},
			Checksum: []byte{0, 1, 2},
		})
	}
	return zoektFileMatches
}

// zoektRPC starts zoekts rpc interface and returns a client to
// searcher. Useful for capturing CPU/memory usage when benchmarking the zoekt
// client.
func zoektRPC(s zoekt.Searcher) (zoekt.Searcher, func()) {
	mux := http.NewServeMux()
	mux.Handle(zoektrpc.DefaultRPCPath, zoektrpc.Server(s))
	ts := httptest.NewServer(mux)
	cl := zoektrpc.Client(strings.TrimPrefix(ts.URL, "http://"))
	return cl, func() {
		cl.Close()
		ts.Close()
	}
}

func TestZoektIndexedRepos_single(t *testing.T) {
	repoRev := func(revSpec string) *search.RepositoryRevisions {
		return &search.RepositoryRevisions{
			Repo: &types.RepoName{ID: api.RepoID(0), Name: "test/repo"},
			Revs: []search.RevisionSpecifier{
				{RevSpec: revSpec},
			},
		}
	}
	zoektRepos := map[string]*zoekt.Repository{
		"test/repo": {
			Name: "test/repo",
			Branches: []zoekt.RepositoryBranch{
				{
					Name:    "HEAD",
					Version: "df3f4e499698e48152b39cd655d8901eaf583fa5",
				},
				{
					Name:    "NOT-HEAD",
					Version: "8ec975423738fe7851676083ebf660a062ed1578",
				},
			},
		},
	}
	cases := []struct {
		rev           string
		wantIndexed   []*search.RepositoryRevisions
		wantUnindexed []*search.RepositoryRevisions
	}{
		{
			rev:           "",
			wantIndexed:   []*search.RepositoryRevisions{repoRev("")},
			wantUnindexed: []*search.RepositoryRevisions{},
		},
		{
			rev:           "HEAD",
			wantIndexed:   []*search.RepositoryRevisions{repoRev("HEAD")},
			wantUnindexed: []*search.RepositoryRevisions{},
		},
		{
			rev:           "df3f4e499698e48152b39cd655d8901eaf583fa5",
			wantIndexed:   []*search.RepositoryRevisions{repoRev("df3f4e499698e48152b39cd655d8901eaf583fa5")},
			wantUnindexed: []*search.RepositoryRevisions{},
		},
		{
			rev:           "df3f4e",
			wantIndexed:   []*search.RepositoryRevisions{repoRev("df3f4e")},
			wantUnindexed: []*search.RepositoryRevisions{},
		},
		{
			rev:           "d",
			wantIndexed:   []*search.RepositoryRevisions{},
			wantUnindexed: []*search.RepositoryRevisions{repoRev("d")},
		},
		{
			rev:           "HEAD^1",
			wantIndexed:   []*search.RepositoryRevisions{},
			wantUnindexed: []*search.RepositoryRevisions{repoRev("HEAD^1")},
		},
		{
			rev:           "8ec975423738fe7851676083ebf660a062ed1578",
			wantUnindexed: []*search.RepositoryRevisions{},
			wantIndexed:   []*search.RepositoryRevisions{repoRev("8ec975423738fe7851676083ebf660a062ed1578")},
		},
	}

	type ret struct {
		Indexed   map[string]*search.RepositoryRevisions
		Unindexed []*search.RepositoryRevisions
	}

	for _, tt := range cases {
		indexed, unindexed := zoektIndexedRepos(zoektRepos, []*search.RepositoryRevisions{repoRev(tt.rev)}, nil)
		got := ret{
			Indexed:   indexed.repoRevs,
			Unindexed: unindexed,
		}
		want := ret{
			Indexed:   repoRevsSliceToMap(tt.wantIndexed),
			Unindexed: tt.wantUnindexed,
		}
		if !cmp.Equal(want, got) {
			t.Errorf("%s mismatch (-want +got):\n%s", tt.rev, cmp.Diff(want, got))
		}
	}
}

func TestZoektFileMatchToSymbolResults(t *testing.T) {
	db := new(dbtesting.MockDB)
	symbolInfo := func(sym string) *zoekt.Symbol {
		return &zoekt.Symbol{
			Sym:        sym,
			Kind:       "kind",
			Parent:     "parent",
			ParentKind: "parentkind",
		}
	}

	file := &zoekt.FileMatch{
		FileName:   "bar.go",
		Repository: "foo",
		Language:   "go",
		Version:    "deadbeef",
		LineMatches: []zoekt.LineMatch{{
			// Skips missing symbol info (shouldn't happen in practice).
			Line:          []byte(""),
			LineNumber:    5,
			LineFragments: []zoekt.LineFragmentMatch{{}},
		}, {
			Line:       []byte("symbol a symbol b"),
			LineNumber: 10,
			LineFragments: []zoekt.LineFragmentMatch{{
				SymbolInfo: symbolInfo("a"),
			}, {
				SymbolInfo: symbolInfo("b"),
			}},
		}, {
			Line:       []byte("symbol c"),
			LineNumber: 15,
			LineFragments: []zoekt.LineFragmentMatch{{
				SymbolInfo: symbolInfo("c"),
			}},
		}, {
			Line:       []byte(`bar() { var regex = /.*\//; function baz() { }  } `),
			LineNumber: 20,
			LineFragments: []zoekt.LineFragmentMatch{{
				SymbolInfo: symbolInfo("baz"),
			}},
		}},
	}

	repo := NewRepositoryResolver(db, &types.Repo{Name: "foo"})

	results := zoektFileMatchToSymbolResults(repo, "master", file)
	var symbols []result.Symbol
	for _, res := range results {
		// Check the fields which are not specific to the symbol
		if got, want := res.Lang, "go"; got != want {
			t.Fatalf("lang: got %q want %q", got, want)
		}
		if got, want := res.BaseURI.URL.String(), "git://foo?master"; got != want {
			t.Fatalf("baseURI: got %q want %q", got, want)
		}

		symbols = append(symbols, res.Symbol)
	}

	want := []result.Symbol{{
		Name:    "a",
		Line:    10,
		Pattern: "/^symbol a symbol b$/",
	}, {
		Name:    "b",
		Line:    10,
		Pattern: "/^symbol a symbol b$/",
	}, {
		Name:    "c",
		Line:    15,
		Pattern: "/^symbol c$/",
	}, {
		Name:    "baz",
		Line:    20,
		Pattern: `/^bar() { var regex = \/.*\\\/\/; function baz() { }  } $/`,
	},
	}
	for i := range want {
		want[i].Kind = "kind"
		want[i].Parent = "parent"
		want[i].ParentKind = "parentkind"
		want[i].Path = "bar.go"
	}

	if diff := cmp.Diff(want, symbols); diff != "" {
		t.Fatalf("symbol mismatch (-want +got):\n%s", diff)
	}
}

func repoRevsSliceToMap(rs []*search.RepositoryRevisions) map[string]*search.RepositoryRevisions {
	m := map[string]*search.RepositoryRevisions{}
	for _, r := range rs {
		m[string(r.Repo.Name)] = r
	}
	return m
}

func TestContextWithoutDeadline(t *testing.T) {
	ctxWithDeadline, cancelWithDeadline := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWithDeadline()

	tr, ctxWithDeadline := trace.New(ctxWithDeadline, "", "")

	if _, ok := ctxWithDeadline.Deadline(); !ok {
		t.Fatal("expected context to have deadline")
	}

	ctxNoDeadline, cancelNoDeadline := contextWithoutDeadline(ctxWithDeadline)
	defer cancelNoDeadline()

	if _, ok := ctxNoDeadline.Deadline(); ok {
		t.Fatal("expected context to not have deadline")
	}

	// We want to keep trace info
	if tr2 := trace.TraceFromContext(ctxNoDeadline); tr != tr2 {
		t.Error("trace information not propogated")
	}

	// Calling cancelWithDeadline should cancel ctxNoDeadline
	cancelWithDeadline()
	select {
	case <-ctxNoDeadline.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("expected context to be done")
	}
}

func TestContextWithoutDeadline_cancel(t *testing.T) {
	ctxWithDeadline, cancelWithDeadline := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWithDeadline()
	ctxNoDeadline, cancelNoDeadline := contextWithoutDeadline(ctxWithDeadline)

	cancelNoDeadline()
	select {
	case <-ctxNoDeadline.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("expected context to be done")
	}
}

func TestBufferedSender(t *testing.T) {
	// We create an unbuffered Sender, which means a call to Send blocks if there is
	// no consumer.
	c := make(chan *zoekt.SearchResult)
	defer close(c)
	unbufferedMockSender := searchbackend.ZoektStreamFunc(func(event *zoekt.SearchResult) {
		c <- event
	})

	// We add a buffer to unbufferedMockSender. A call to Send should not block anymore.
	bufferedMockSender, cleanup := bufferedSender(1, unbufferedMockSender)
	defer cleanup()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Should not block.
		bufferedMockSender.Send(&zoekt.SearchResult{Files: generateZoektMatches(1)})

	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Errorf("bufferedMockSender.Send did not return in time")
	}
	// Drain the buffer to make sure that cleanup() can return.
	<-c
}
