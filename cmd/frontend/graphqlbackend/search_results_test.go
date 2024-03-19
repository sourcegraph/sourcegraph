package graphqlbackend

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/zoekt"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/search"
	searchbackend "github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/settings"
	"github.com/sourcegraph/sourcegraph/internal/telemetry/telemetrytest"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestSearchResults(t *testing.T) {
	if os.Getenv("CI") != "" {
		// #25936: Some unit tests rely on external services that break
		// in CI but not locally. They should be removed or improved.
		t.Skip("TestSearchResults only works in local dev and is not reliable in CI")
	}

	ctx := context.Background()
	db := dbmocks.NewMockDB()

	getResults := func(t *testing.T, query, version string) []string {
		r, err := newSchemaResolver(db, gitserver.NewTestClient(t)).Search(ctx, &SearchArgs{Query: query, Version: version})
		require.Nil(t, err)

		results, err := r.Results(ctx)
		require.NoError(t, err)

		resultDescriptions := make([]string, len(results.Matches))
		for i, match := range results.Matches {
			// NOTE: Only supports one match per line. If we need to test other cases,
			// just remove that assumption in the following line of code.
			switch m := match.(type) {
			case *result.RepoMatch:
				resultDescriptions[i] = fmt.Sprintf("repo:%s", m.Name)
			case *result.FileMatch:
				resultDescriptions[i] = fmt.Sprintf("%s:%d", m.Path, m.ChunkMatches[0].Ranges[0].Start.Line)
			default:
				t.Fatal("unexpected result type:", match)
			}
		}
		// dedupe results since we expect our clients to do dedupping
		if len(resultDescriptions) > 1 {
			sort.Strings(resultDescriptions)
			dedup := resultDescriptions[:1]
			for _, s := range resultDescriptions[1:] {
				if s != dedup[len(dedup)-1] {
					dedup = append(dedup, s)
				}
			}
			resultDescriptions = dedup
		}
		return resultDescriptions
	}
	testCallResults := func(t *testing.T, query, version string, want []string) {
		t.Helper()
		results := getResults(t, query, version)
		if d := cmp.Diff(want, results); d != "" {
			t.Errorf("unexpected results (-want, +got):\n%s", d)
		}
	}

	searchVersions := []string{"V1", "V2"}

	t.Run("repo: only", func(t *testing.T) {
		settings.MockCurrentUserFinal = &schema.Settings{}
		defer func() { settings.MockCurrentUserFinal = nil }()

		repos := dbmocks.NewMockRepoStore()
		repos.ListMinimalReposFunc.SetDefaultHook(func(ctx context.Context, opt database.ReposListOptions) ([]types.MinimalRepo, error) {
			require.Equal(t, []string{"r", "p"}, opt.IncludePatterns)
			return []types.MinimalRepo{{ID: 1, Name: "repo"}}, nil
		})
		db.ReposFunc.SetDefaultReturn(repos)

		for _, v := range searchVersions {
			testCallResults(t, `repo:r repo:p`, v, []string{"repo:repo"})
			mockrequire.Called(t, repos.ListMinimalReposFunc)
		}
	})
}

func TestSearchResolver_DynamicFilters(t *testing.T) {
	repo := types.MinimalRepo{Name: "testRepo"}
	repoMatch := &result.RepoMatch{Name: "testRepo"}
	fileMatch := func(path string) *result.FileMatch {
		return mkFileMatch(repo, path)
	}

	rev := "develop3.0"
	fileMatchRev := fileMatch("/testFile.md")
	fileMatchRev.InputRev = &rev

	type testCase struct {
		descr                           string
		searchResults                   []result.Match
		expectedDynamicFilterStrsRegexp map[string]int
	}

	tests := []testCase{
		{
			descr:         "single repo match",
			searchResults: []result.Match{repoMatch},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`type:repo`: 1,
			},
		},

		{
			descr:         "single file match without revision in query",
			searchResults: []result.Match{fileMatch("/testFile.md")},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`: 1,
				`lang:markdown`:   1,
				`type:path`:       1,
			},
		},

		{
			descr:         "single file match with specified revision",
			searchResults: []result.Match{fileMatchRev},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$@develop3.0`: 1,
				`lang:markdown`:              1,
				`type:path`:                  1,
			},
		},
		{
			descr:         "file match from a language with two file extensions, using first extension",
			searchResults: []result.Match{fileMatch("/testFile.yml")},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`: 1,
				`lang:yaml`:       1,
				`type:path`:       1,
			},
		},
		{
			descr:         "file match from a language with two file extensions, using second extension",
			searchResults: []result.Match{fileMatch("/testFile.yaml")},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`: 1,
				`lang:yaml`:       1,
				`type:path`:       1,
			},
		},
		{
			descr:         "file match which matches one of the common file filters",
			searchResults: []result.Match{fileMatch("/anything/node_modules/testFile.md")},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`:          1,
				`-file:(^|/)node_modules/`: 1,
				`lang:markdown`:            1,
				`type:path`:                1,
			},
		},
		{
			descr:         "file match which matches one of the common file filters",
			searchResults: []result.Match{fileMatch("/node_modules/testFile.md")},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`:          1,
				`-file:(^|/)node_modules/`: 1,
				`lang:markdown`:            1,
				`type:path`:                1,
			},
		},
		{
			descr: "file match which matches one of the common file filters",
			searchResults: []result.Match{
				fileMatch("/foo_test.go"),
				fileMatch("/foo.go"),
			},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`:  2,
				`-file:_test\.go$`: 1,
				`lang:go`:          2,
				`type:path`:        2,
			},
		},

		{
			descr: "prefer rust to renderscript",
			searchResults: []result.Match{
				fileMatch("/channel.rs"),
			},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`: 1,
				`lang:rust`:       1,
				`type:path`:       1,
			},
		},

		{
			descr: "javascript filters",
			searchResults: []result.Match{
				fileMatch("/jsrender.min.js.map"),
				fileMatch("playground/react/lib/app.js.map"),
				fileMatch("assets/javascripts/bootstrap.min.js"),
			},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`:  3,
				`-file:\.min\.js$`: 1,
				`-file:\.js\.map$`: 2,
				`lang:javascript`:  1,
				`type:path`:        3,
			},
		},

		// If there are no search results, no filters should be displayed.
		{
			descr:                           "no results",
			searchResults:                   []result.Match{},
			expectedDynamicFilterStrsRegexp: map[string]int{},
		},
		{
			descr:         "values containing spaces are quoted",
			searchResults: []result.Match{fileMatch("/.gitignore")},
			expectedDynamicFilterStrsRegexp: map[string]int{
				`repo:^testRepo$`:    1,
				`lang:"ignore list"`: 1,
				`type:path`:          1,
			},
		},
	}

	settings.MockCurrentUserFinal = &schema.Settings{}
	defer func() { settings.MockCurrentUserFinal = nil }()

	var expectedDynamicFilterStrs map[string]int
	for _, test := range tests {
		t.Run(test.descr, func(t *testing.T) {
			actualDynamicFilters := (&SearchResultsResolver{db: dbmocks.NewMockDB(), Matches: test.searchResults}).DynamicFilters(context.Background())
			actualDynamicFilterStrs := make(map[string]int)

			for _, filter := range actualDynamicFilters {
				actualDynamicFilterStrs[filter.Value()] = int(filter.Count())
			}

			expectedDynamicFilterStrs = test.expectedDynamicFilterStrsRegexp
			if diff := cmp.Diff(expectedDynamicFilterStrs, actualDynamicFilterStrs); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestSearchResultsHydration(t *testing.T) {
	id := 42
	repoName := "reponame-foobar"
	fileName := "foobar.go"

	repoWithIDs := &types.Repo{
		ID:   api.RepoID(id),
		Name: api.RepoName(repoName),
		ExternalRepo: api.ExternalRepoSpec{
			ID:          repoName,
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com",
		},
	}

	hydratedRepo := &types.Repo{
		ID:           repoWithIDs.ID,
		ExternalRepo: repoWithIDs.ExternalRepo,
		Name:         repoWithIDs.Name,
		URI:          fmt.Sprintf("github.com/my-org/%s", repoWithIDs.Name),
		Description:  "This is a description of a repository",
		Fork:         false,
	}

	db := dbmocks.NewMockDB()

	repos := dbmocks.NewMockRepoStore()
	repos.GetFunc.SetDefaultReturn(hydratedRepo, nil)
	repos.ListMinimalReposFunc.SetDefaultHook(func(ctx context.Context, opt database.ReposListOptions) ([]types.MinimalRepo, error) {
		if opt.OnlyPrivate {
			return nil, nil
		}
		return []types.MinimalRepo{{ID: repoWithIDs.ID, Name: repoWithIDs.Name}}, nil
	})
	repos.CountFunc.SetDefaultReturn(0, nil)
	db.ReposFunc.SetDefaultReturn(repos)

	zoektRepo := &zoekt.RepoListEntry{
		Repository: zoekt.Repository{
			ID:       uint32(repoWithIDs.ID),
			Name:     string(repoWithIDs.Name),
			Branches: []zoekt.RepositoryBranch{{Name: "HEAD", Version: "deadbeef"}},
		},
	}

	zoektFileMatches := []zoekt.FileMatch{{
		Score:        5.0,
		FileName:     fileName,
		RepositoryID: uint32(repoWithIDs.ID),
		Repository:   string(repoWithIDs.Name), // Important: this needs to match a name in `repos`
		Branches:     []string{"master"},
		ChunkMatches: make([]zoekt.ChunkMatch, 1),
		Checksum:     []byte{0, 1, 2},
	}}

	z := &searchbackend.FakeStreamer{
		Repos:   []*zoekt.RepoListEntry{zoektRepo},
		Results: []*zoekt.SearchResult{{Files: zoektFileMatches}},
	}

	// Act in a user context
	var ctxUser int32 = 1234
	ctx := actor.WithActor(context.Background(), actor.FromMockUser(ctxUser))

	query := `foobar index:only count:350`
	literalPatternType := "literal"
	cli := client.Mocked(job.RuntimeClients{
		Logger: logtest.Scoped(t),
		DB:     db,
		Zoekt:  z,
	})
	searchInputs, err := cli.Plan(
		ctx,
		"V2",
		&literalPatternType,
		query,
		search.Precise,
		search.Batch,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	resolver := &searchResolver{
		client:       cli,
		db:           db,
		SearchInputs: searchInputs,
	}
	results, err := resolver.Results(ctx)
	if err != nil {
		t.Fatal("Results:", err)
	}
	// We want one file match and one repository match
	wantMatchCount := 2
	if int(results.MatchCount()) != wantMatchCount {
		t.Fatalf("wrong results length. want=%d, have=%d\n", wantMatchCount, results.MatchCount())
	}

	for _, r := range results.Results() {
		switch r := r.(type) {
		case *FileMatchResolver:
			assertRepoResolverHydrated(ctx, t, r.Repository(), hydratedRepo)

		case *RepositoryResolver:
			assertRepoResolverHydrated(ctx, t, r, hydratedRepo)
		}
	}
}

func TestSearchResultsResolver_ApproximateResultCount(t *testing.T) {
	type fields struct {
		results             []result.Match
		searchResultsCommon streaming.Stats
		alert               *search.Alert
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "empty",
			fields: fields{},
			want:   "0",
		},

		{
			name: "file matches",
			fields: fields{
				results: []result.Match{&result.FileMatch{}},
			},
			want: "1",
		},

		{
			name: "file matches limit hit",
			fields: fields{
				results:             []result.Match{&result.FileMatch{}},
				searchResultsCommon: streaming.Stats{IsLimitHit: true},
			},
			want: "1+",
		},

		{
			name: "symbol matches",
			fields: fields{
				results: []result.Match{
					&result.FileMatch{
						Symbols: []*result.SymbolMatch{
							// 1
							{},
							// 2
							{},
						},
					},
				},
			},
			want: "2",
		},

		{
			name: "symbol matches limit hit",
			fields: fields{
				results: []result.Match{
					&result.FileMatch{
						Symbols: []*result.SymbolMatch{
							// 1
							{},
							// 2
							{},
						},
					},
				},
				searchResultsCommon: streaming.Stats{IsLimitHit: true},
			},
			want: "2+",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &SearchResultsResolver{
				db:          dbmocks.NewMockDB(),
				Stats:       tt.fields.searchResultsCommon,
				Matches:     tt.fields.results,
				SearchAlert: tt.fields.alert,
			}
			if got := sr.ApproximateResultCount(); got != tt.want {
				t.Errorf("searchResultsResolver.ApproximateResultCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareSearchResults(t *testing.T) {
	makeResult := func(repo, file string) *result.FileMatch {
		return &result.FileMatch{File: result.File{
			Repo: types.MinimalRepo{Name: api.RepoName(repo)},
			Path: file,
		}}
	}

	tests := []struct {
		name    string
		a       *result.FileMatch
		b       *result.FileMatch
		aIsLess bool
	}{
		{
			name:    "alphabetical order",
			a:       makeResult("arepo", "afile"),
			b:       makeResult("arepo", "bfile"),
			aIsLess: true,
		},
		{
			name:    "same length, different files",
			a:       makeResult("arepo", "bfile"),
			b:       makeResult("arepo", "afile"),
			aIsLess: false,
		},
		{
			name:    "different repo, no exact patterns",
			a:       makeResult("arepo", "file"),
			b:       makeResult("brepo", "afile"),
			aIsLess: true,
		},
		{
			name:    "repo matches only",
			a:       makeResult("arepo", ""),
			b:       makeResult("brepo", ""),
			aIsLess: true,
		},
		{
			name:    "repo match and file match, same repo",
			a:       makeResult("arepo", "file"),
			b:       makeResult("arepo", ""),
			aIsLess: false,
		},
		{
			name:    "repo match and file match, different repos",
			a:       makeResult("arepo", ""),
			b:       makeResult("brepo", "file"),
			aIsLess: true,
		},
	}
	for _, tt := range tests {
		t.Run("test", func(t *testing.T) {
			if got := tt.a.Key().Less(tt.b.Key()); got != tt.aIsLess {
				t.Errorf("compareSearchResults() = %v, aIsLess %v", got, tt.aIsLess)
			}
		})
	}
}

func TestEvaluateAnd(t *testing.T) {
	db := dbmocks.NewMockDB()

	tests := []struct {
		name         string
		query        string
		zoektMatches int
		filesSkipped int
		wantAlert    bool
	}{
		{
			name:         "zoekt returns enough matches, exhausted",
			query:        "foo and bar index:only count:5",
			zoektMatches: 5,
			filesSkipped: 0,
			wantAlert:    false,
		},
		{
			name:         "zoekt returns enough matches, not exhausted",
			query:        "foo and bar index:only count:50",
			zoektMatches: 50,
			filesSkipped: 1,
			wantAlert:    false,
		},
	}

	minimalRepos, zoektRepos := generateRepos(5000)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zoektFileMatches := generateZoektMatches(tt.zoektMatches)
			z := &searchbackend.FakeStreamer{
				Repos:   zoektRepos,
				Results: []*zoekt.SearchResult{{Files: zoektFileMatches, Stats: zoekt.Stats{FilesSkipped: tt.filesSkipped}}},
			}

			ctx := context.Background()

			repos := dbmocks.NewMockRepoStore()
			repos.ListMinimalReposFunc.SetDefaultHook(func(ctx context.Context, opt database.ReposListOptions) ([]types.MinimalRepo, error) {
				if len(opt.IncludePatterns) > 0 || len(opt.ExcludePattern) > 0 {
					return nil, nil
				}
				repoNames := make([]types.MinimalRepo, len(minimalRepos))
				for i := range minimalRepos {
					repoNames[i] = types.MinimalRepo{ID: minimalRepos[i].ID, Name: minimalRepos[i].Name}
				}
				return repoNames, nil
			})
			repos.CountFunc.SetDefaultReturn(len(minimalRepos), nil)
			db.ReposFunc.SetDefaultReturn(repos)

			literalPatternType := "literal"
			cli := client.Mocked(job.RuntimeClients{
				Logger: logtest.Scoped(t),
				DB:     db,
				Zoekt:  z,
			})
			searchInputs, err := cli.Plan(
				context.Background(),
				"V2",
				&literalPatternType,
				tt.query,
				search.Precise,
				search.Batch,
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}

			resolver := &searchResolver{
				client:       cli,
				db:           db,
				SearchInputs: searchInputs,
			}
			results, err := resolver.Results(ctx)
			if err != nil {
				t.Fatal("Results:", err)
			}
			if tt.wantAlert {
				if results.SearchAlert == nil {
					t.Errorf("Expected alert")
				}
			} else if int(results.MatchCount()) != len(zoektFileMatches) {
				t.Errorf("wrong results length. want=%d, have=%d\n", len(zoektFileMatches), results.MatchCount())
			}
		})
	}
}

func TestZeroElapsedMilliseconds(t *testing.T) {
	r := &SearchResultsResolver{}
	if got := r.ElapsedMilliseconds(); got != 0 {
		t.Fatalf("got %d, want %d", got, 0)
	}
}

// Detailed filtering tests are below in TestSubRepoFilterFunc, this test is more
// of an integration test to ensure that things are threaded through correctly
// from the resolver
func TestSubRepoFiltering(t *testing.T) {
	tts := []struct {
		name        string
		searchQuery string
		wantCount   int
		checker     func() authz.SubRepoPermissionChecker
	}{
		{
			name:        "simple search without filtering",
			searchQuery: "foo",
			wantCount:   3,
		},
		{
			name:        "simple search with filtering",
			searchQuery: "foo ",
			wantCount:   2,
			checker: func() authz.SubRepoPermissionChecker {
				checker := authz.NewMockSubRepoPermissionChecker()
				checker.EnabledFunc.SetDefaultHook(func() bool {
					return true
				})
				// We'll just block the third file
				checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
					if strings.Contains(content.Path, "3") {
						return authz.None, nil
					}
					return authz.Read, nil
				})
				checker.EnabledForRepoIDFunc.SetDefaultHook(func(context.Context, api.RepoID) (bool, error) {
					return true, nil
				})
				return checker
			},
		},
	}

	zoektFileMatches := generateZoektMatches(3)
	mockZoekt := &searchbackend.FakeStreamer{
		Repos: []*zoekt.RepoListEntry{},
		Results: []*zoekt.SearchResult{{
			Files: zoektFileMatches,
		}},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			if tt.checker != nil {
				old := authz.DefaultSubRepoPermsChecker
				t.Cleanup(func() { authz.DefaultSubRepoPermsChecker = old })
				authz.DefaultSubRepoPermsChecker = tt.checker()
			}

			repos := dbmocks.NewMockRepoStore()
			repos.ListMinimalReposFunc.SetDefaultReturn([]types.MinimalRepo{}, nil)
			repos.CountFunc.SetDefaultReturn(0, nil)

			gss := dbmocks.NewMockGlobalStateStore()
			gss.GetFunc.SetDefaultReturn(database.GlobalState{SiteID: "a"}, nil)

			db := dbmocks.NewMockDB()
			db.GlobalStateFunc.SetDefaultReturn(gss)
			db.ReposFunc.SetDefaultReturn(repos)
			db.EventLogsFunc.SetDefaultHook(func() database.EventLogStore {
				return dbmocks.NewMockEventLogStore()
			})
			db.TelemetryEventsExportQueueFunc.SetDefaultReturn(
				telemetrytest.NewMockEventsExportQueueStore())

			literalPatternType := "literal"
			cli := client.Mocked(job.RuntimeClients{
				Logger: logtest.Scoped(t),
				DB:     db,
				Zoekt:  mockZoekt,
			})
			searchInputs, err := cli.Plan(
				context.Background(),
				"V2",
				&literalPatternType,
				tt.searchQuery,
				search.Precise,
				search.Batch,
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}

			resolver := searchResolver{
				client:       cli,
				SearchInputs: searchInputs,
				db:           db,
			}

			ctx := context.Background()
			ctx = actor.WithActor(ctx, &actor.Actor{
				UID: 1,
			})
			rr, err := resolver.Results(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if len(rr.Matches) != tt.wantCount {
				t.Fatalf("Want %d matches, got %d", tt.wantCount, len(rr.Matches))
			}
		})
	}
}
