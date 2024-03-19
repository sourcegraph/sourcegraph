package repos

import (
	stdcmp "cmp"
	"context"
	"flag"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/go-cmp/cmp"
	"github.com/grafana/regexp"
	"github.com/sourcegraph/zoekt"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/iterator"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func toParsedRepoFilters(repoRevs ...string) []query.ParsedRepoFilter {
	repoFilters := make([]query.ParsedRepoFilter, len(repoRevs))
	for i, r := range repoRevs {
		parsedFilter, err := query.ParseRepositoryRevisions(r)
		if err != nil {
			panic(errors.Errorf("unexpected error parsing repo filter %s", r))
		}
		repoFilters[i] = parsedFilter
	}
	return repoFilters
}

func TestRevisionValidation(t *testing.T) {
	mockGitserver := gitserver.NewMockClient()
	mockGitserver.ResolveRevisionFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, spec string, opt gitserver.ResolveRevisionOptions) (api.CommitID, error) {
		// trigger errors
		if spec == "bad_commit" {
			return "", &gitdomain.BadCommitError{}
		}
		if spec == "deadline_exceeded" {
			return "", context.DeadlineExceeded
		}

		// known revisions
		m := map[string]struct{}{
			"revBar": {},
			"revBas": {},
		}
		if _, ok := m[spec]; ok {
			return "", nil
		}
		return "", &gitdomain.RevisionNotFoundError{Repo: "repoFoo", Spec: spec}
	})
	mockGitserver.ListRefsFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName) ([]gitdomain.Ref, error) {
		return []gitdomain.Ref{{
			Name: "refs/heads/revBar",
		}, {
			Name: "refs/heads/revBas",
		}}, nil
	})

	tests := []struct {
		repoFilters  []string
		wantRepoRevs []*search.RepositoryRevisions
		wantErr      error
	}{
		{
			repoFilters: []string{"repoFoo@revBar:^revBas"},
			wantRepoRevs: []*search.RepositoryRevisions{{
				Repo: types.MinimalRepo{Name: "repoFoo"},
				Revs: []string{"revBar", "^revBas"},
			}},
		},
		{
			repoFilters: []string{"repoFoo@*refs/heads/*:*!refs/heads/revBas"},
			wantRepoRevs: []*search.RepositoryRevisions{{
				Repo: types.MinimalRepo{Name: "repoFoo"},
				Revs: []string{"revBar"},
			}},
		},
		{
			repoFilters: []string{"repoFoo@revBar:^revQux"},
			wantRepoRevs: []*search.RepositoryRevisions{{
				Repo: types.MinimalRepo{Name: "repoFoo"},
				Revs: []string{"revBar"},
			}},
			wantErr: &MissingRepoRevsError{
				Missing: []RepoRevSpecs{{
					Repo: types.MinimalRepo{Name: "repoFoo"},
					Revs: []query.RevisionSpecifier{{
						RevSpec: "^revQux",
					}},
				}},
			},
		},
		{
			repoFilters:  []string{"repoFoo@revBar:bad_commit"},
			wantRepoRevs: nil,
			wantErr:      &gitdomain.BadCommitError{},
		},
		{
			repoFilters:  []string{"repoFoo@revBar:^bad_commit"},
			wantRepoRevs: nil,
			wantErr:      &gitdomain.BadCommitError{},
		},
		{
			repoFilters:  []string{"repoFoo@revBar:deadline_exceeded"},
			wantRepoRevs: nil,
			wantErr:      context.DeadlineExceeded,
		},
		{
			repoFilters: []string{"repoFoo"},
			wantRepoRevs: []*search.RepositoryRevisions{{
				Repo: types.MinimalRepo{Name: "repoFoo"},
				Revs: []string{""},
			}},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.repoFilters[0], func(t *testing.T) {
			repos := dbmocks.NewMockRepoStore()
			repos.ListMinimalReposFunc.SetDefaultReturn([]types.MinimalRepo{{Name: "repoFoo"}}, nil)
			db := dbmocks.NewMockDB()
			db.ReposFunc.SetDefaultReturn(repos)

			op := search.RepoOptions{RepoFilters: toParsedRepoFilters(tt.repoFilters...)}
			repositoryResolver := NewResolver(logtest.Scoped(t), db, nil, nil, defaults.NewConnectionCache(logtest.Scoped(t)), nil)
			repositoryResolver.gitserver = mockGitserver
			resolved, _, err := repositoryResolver.resolve(context.Background(), op)
			if !errors.Is(err, tt.wantErr) {
				diff := cmp.Diff(tt.wantErr, errors.UnwrapAll(err))
				t.Error(diff)
			}
			if diff := cmp.Diff(tt.wantRepoRevs, resolved.RepoRevs); diff != "" {
				t.Error(diff)
			}
			mockrequire.Called(t, repos.ListMinimalReposFunc)
		})
	}
}

// TestSearchRevspecs tests a repository name against a list of
// repository specs with optional revspecs, and determines whether
// we get the expected error, list of matching rev specs, or list
// of clashing revspecs (if no matching rev specs were found)
func TestSearchRevspecs(t *testing.T) {
	type testCase struct {
		descr    string
		specs    []string
		repo     string
		err      error
		matched  []query.RevisionSpecifier
		clashing []query.RevisionSpecifier
	}

	tests := []testCase{
		{
			descr:    "simple match",
			specs:    []string{"foo"},
			repo:     "foo",
			err:      nil,
			matched:  []query.RevisionSpecifier{{RevSpec: ""}},
			clashing: nil,
		},
		{
			descr:    "single revspec",
			specs:    []string{".*o@123456"},
			repo:     "foo",
			err:      nil,
			matched:  []query.RevisionSpecifier{{RevSpec: "123456"}},
			clashing: nil,
		},
		{
			descr:    "revspec plus unspecified rev",
			specs:    []string{".*o@123456", "foo"},
			repo:     "foo",
			err:      nil,
			matched:  []query.RevisionSpecifier{{RevSpec: "123456"}},
			clashing: nil,
		},
		{
			descr:    "revspec plus unspecified rev, but backwards",
			specs:    []string{".*o", "foo@123456"},
			repo:     "foo",
			err:      nil,
			matched:  []query.RevisionSpecifier{{RevSpec: "123456"}},
			clashing: nil,
		},
		{
			descr:    "conflicting revspecs",
			specs:    []string{".*o@123456", "foo@234567"},
			repo:     "foo",
			err:      nil,
			matched:  nil,
			clashing: []query.RevisionSpecifier{{RevSpec: "123456"}, {RevSpec: "234567"}},
		},
		{
			descr:    "overlapping revspecs",
			specs:    []string{".*o@a:b", "foo@b:c"},
			repo:     "foo",
			err:      nil,
			matched:  []query.RevisionSpecifier{{RevSpec: "b"}},
			clashing: nil,
		},
		{
			descr:    "multiple overlapping revspecs",
			specs:    []string{".*o@a:b:c", "foo@b:c:d"},
			repo:     "foo",
			err:      nil,
			matched:  []query.RevisionSpecifier{{RevSpec: "b"}, {RevSpec: "c"}},
			clashing: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.descr, func(t *testing.T) {
			repoRevs := toParsedRepoFilters(test.specs...)
			_, pats := findPatternRevs(repoRevs)
			if test.err != nil {
				t.Errorf("missing expected error: wanted '%s'", test.err.Error())
			}
			matched, clashing := getRevsForMatchedRepo(api.RepoName(test.repo), pats)
			if !reflect.DeepEqual(matched, test.matched) {
				t.Errorf("matched repo mismatch: actual: %#v, expected: %#v", matched, test.matched)
			}
			if !reflect.DeepEqual(clashing, test.clashing) {
				t.Errorf("clashing repo mismatch: actual: %#v, expected: %#v", clashing, test.clashing)
			}
		})
	}
}

func BenchmarkGetRevsForMatchedRepo(b *testing.B) {
	b.Run("2 conflicting", func(b *testing.B) {
		repoRevs := toParsedRepoFilters(".*o@123456", "foo@234567")
		_, pats := findPatternRevs(repoRevs)
		for range b.N {
			_, _ = getRevsForMatchedRepo("foo", pats)
		}
	})

	b.Run("multiple overlapping", func(b *testing.B) {
		repoRevs := toParsedRepoFilters(".*o@a:b:c:d", "foo@b:c:d:e", "foo@c:d:e:f")
		_, pats := findPatternRevs(repoRevs)
		for range b.N {
			_, _ = getRevsForMatchedRepo("foo", pats)
		}
	})
}

func TestResolverIterator(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))

	for i := 1; i <= 5; i++ {
		r := &types.Repo{
			Name:  api.RepoName(fmt.Sprintf("github.com/foo/bar%d", i)),
			Stars: i * 100,
		}

		if err := db.Repos().Create(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	gsClient := gitserver.NewMockClient()
	gsClient.ResolveRevisionFunc.SetDefaultHook(func(_ context.Context, name api.RepoName, spec string, _ gitserver.ResolveRevisionOptions) (api.CommitID, error) {
		if spec == "bad_commit" {
			return "", &gitdomain.BadCommitError{}
		}
		// All repos have the revision except foo/bar5
		if name == "github.com/foo/bar5" {
			return "", &gitdomain.RevisionNotFoundError{}
		}
		return "", nil
	})

	resolver := NewResolver(logtest.Scoped(t), db, gsClient, nil, defaults.NewConnectionCache(logtest.Scoped(t)), nil)
	all, _, err := resolver.resolve(ctx, search.RepoOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Assertation that we get the cursor we expect
	{
		want := types.MultiCursor{
			{Column: "stars", Direction: "prev", Value: fmt.Sprint(all.RepoRevs[3].Repo.Stars)},
			{Column: "id", Direction: "prev", Value: fmt.Sprint(all.RepoRevs[3].Repo.ID)},
		}
		_, next, err := resolver.resolve(ctx, search.RepoOptions{
			Limit: 3,
		})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(next, want); diff != "" {
			t.Errorf("unexpected cursor (-have, +want):\n%s", diff)
		}
	}

	allAtRev, _, err := resolver.resolve(ctx, search.RepoOptions{RepoFilters: toParsedRepoFilters("foo/bar[0-4]@rev")})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		opts  search.RepoOptions
		pages []Resolved
		err   error
	}{
		{
			name:  "default limit 500, no cursors",
			opts:  search.RepoOptions{},
			pages: []Resolved{all},
		},
		{
			name: "with limit 3, no cursors",
			opts: search.RepoOptions{
				Limit: 3,
			},
			pages: []Resolved{
				{
					RepoRevs: all.RepoRevs[:3],
				},
				{
					RepoRevs: all.RepoRevs[3:],
				},
			},
		},
		{
			name: "with limit 3 and fatal error",
			opts: search.RepoOptions{
				Limit:       3,
				RepoFilters: toParsedRepoFilters("foo/bar[0-5]@bad_commit"),
			},
			err:   &gitdomain.BadCommitError{},
			pages: nil,
		},
		{
			name: "with limit 3 and missing repo revs",
			opts: search.RepoOptions{
				Limit:       3,
				RepoFilters: toParsedRepoFilters("foo/bar[0-5]@rev"),
			},
			err: &MissingRepoRevsError{Missing: []RepoRevSpecs{
				{
					Repo: all.RepoRevs[0].Repo, // corresponds to foo/bar5
					Revs: []query.RevisionSpecifier{
						{
							RevSpec: "rev",
						},
					},
				},
			}},
			pages: []Resolved{
				{
					RepoRevs: allAtRev.RepoRevs[:2],
				},
				{
					RepoRevs: allAtRev.RepoRevs[2:],
				},
			},
		},
		{
			name: "with limit 3 and cursor",
			opts: search.RepoOptions{
				Limit: 3,
				Cursors: types.MultiCursor{
					{Column: "stars", Direction: "prev", Value: fmt.Sprint(all.RepoRevs[3].Repo.Stars)},
					{Column: "id", Direction: "prev", Value: fmt.Sprint(all.RepoRevs[3].Repo.ID)},
				},
			},
			pages: []Resolved{
				{
					RepoRevs: all.RepoRevs[3:],
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewResolver(logtest.Scoped(t), db, gsClient, nil, defaults.NewConnectionCache(logtest.Scoped(t)), nil)
			it := r.Iterator(ctx, tc.opts)

			var pages []Resolved

			for it.Next() {
				page := it.Current()
				pages = append(pages, page)
			}

			err = it.Err()
			if !errors.Is(err, tc.err) {
				diff := cmp.Diff(errors.UnwrapAll(err), tc.err)
				t.Errorf("%s unexpected error (-have, +want):\n%s", tc.name, diff)
			}

			if diff := cmp.Diff(pages, tc.pages); diff != "" {
				t.Errorf("%s unexpected pages (-have, +want):\n%s", tc.name, diff)
			}
		})
	}
}

func TestResolverIterateRepoRevs(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))

	// intentionally nil so it panics if we call it
	var gsClient gitserver.Client = nil

	var all []RepoRevSpecs
	for i := 1; i <= 5; i++ {
		r := &types.Repo{
			Name:  api.RepoName(fmt.Sprintf("github.com/foo/bar%d", i)),
			Stars: i * 100,
		}

		if err := db.Repos().Create(ctx, r); err != nil {
			t.Fatal(err)
		}

		all = append(all, RepoRevSpecs{Repo: types.MinimalRepo{
			ID:    r.ID,
			Name:  r.Name,
			Stars: r.Stars,
		}})
	}

	withRevSpecs := func(rrs []RepoRevSpecs, revs ...query.RevisionSpecifier) []RepoRevSpecs {
		var with []RepoRevSpecs
		for _, r := range rrs {
			with = append(with, RepoRevSpecs{
				Repo: r.Repo,
				Revs: revs,
			})
		}
		return with
	}

	for _, tc := range []struct {
		name    string
		opts    search.RepoOptions
		want    []RepoRevSpecs
		wantErr string
	}{
		{
			name: "default",
			opts: search.RepoOptions{},
			want: withRevSpecs(all, query.RevisionSpecifier{}),
		},
		{
			name: "specific repo",
			opts: search.RepoOptions{
				RepoFilters: toParsedRepoFilters("foo/bar1"),
			},
			want: withRevSpecs(all[:1], query.RevisionSpecifier{}),
		},
		{
			name: "no repos",
			opts: search.RepoOptions{
				RepoFilters: toParsedRepoFilters("horsegraph"),
			},
			wantErr: ErrNoResolvedRepos.Error(),
		},

		// The next block of test cases would normally reach out to gitserver
		// and fail. But because we haven't reached out we should still get
		// back a list. See the corresponding cases in TestResolverIterator.
		{
			name: "no gitserver revspec",
			opts: search.RepoOptions{
				RepoFilters: toParsedRepoFilters("foo/bar[0-5]@bad_commit"),
			},
			want: withRevSpecs(all, query.RevisionSpecifier{RevSpec: "bad_commit"}),
		},
		{
			name: "no gitserver refglob",
			opts: search.RepoOptions{
				RepoFilters: toParsedRepoFilters("foo/bar[0-5]@*refs/heads/foo*"),
			},
			want: withRevSpecs(all, query.RevisionSpecifier{RefGlob: "refs/heads/foo*"}),
		},
		{
			name: "no gitserver excluderefglob",
			opts: search.RepoOptions{
				RepoFilters: toParsedRepoFilters("foo/bar[0-5]@*!refs/heads/foo*"),
			},
			want: withRevSpecs(all, query.RevisionSpecifier{ExcludeRefGlob: "refs/heads/foo*"}),
		},
		{
			name: "no gitserver multiref",
			opts: search.RepoOptions{
				RepoFilters: toParsedRepoFilters("foo/bar[0-5]@foo:bar"),
			},
			want: withRevSpecs(all, query.RevisionSpecifier{RevSpec: "foo"}, query.RevisionSpecifier{RevSpec: "bar"}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewResolver(logger, db, gsClient, nil, defaults.NewConnectionCache(logtest.Scoped(t)), nil)
			got, err := iterator.Collect(r.IterateRepoRevs(ctx, tc.opts))

			var gotErr string
			if err != nil {
				gotErr = err.Error()
			}
			if diff := cmp.Diff(gotErr, tc.wantErr); diff != "" {
				t.Errorf("unexpected error (-have, +want):\n%s", diff)
			}

			// copy want because we will mutate it when sorting
			var want []RepoRevSpecs
			want = append(want, tc.want...)

			compare := func(a, b RepoRevSpecs) int {
				return stdcmp.Compare(a.Repo.ID, b.Repo.ID)
			}
			slices.SortFunc(got, compare)
			slices.SortFunc(want, compare)

			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("unexpected (-have, +want):\n%s", diff)
			}
		})
	}
}

func TestResolveRepositoriesWithSearchContext(t *testing.T) {
	searchContext := &types.SearchContext{ID: 1, Name: "searchcontext"}
	repoA := types.MinimalRepo{ID: 1, Name: "example.com/a"}
	repoB := types.MinimalRepo{ID: 2, Name: "example.com/b"}
	searchContextRepositoryRevisions := []*types.SearchContextRepositoryRevisions{
		{Repo: repoA, Revisions: []string{"branch-1", "branch-3"}},
		{Repo: repoB, Revisions: []string{"branch-2"}},
	}

	gsClient := gitserver.NewMockClient()
	gsClient.ResolveRevisionFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, spec string, _ gitserver.ResolveRevisionOptions) (api.CommitID, error) {
		return api.CommitID(spec), nil
	})

	repos := dbmocks.NewMockRepoStore()
	repos.ListMinimalReposFunc.SetDefaultHook(func(ctx context.Context, op database.ReposListOptions) ([]types.MinimalRepo, error) {
		if op.SearchContextID != searchContext.ID {
			t.Fatalf("got %q, want %q", op.SearchContextID, searchContext.ID)
		}
		return []types.MinimalRepo{repoA, repoB}, nil
	})

	sc := dbmocks.NewMockSearchContextsStore()
	sc.GetSearchContextFunc.SetDefaultHook(func(ctx context.Context, opts database.GetSearchContextOptions) (*types.SearchContext, error) {
		if opts.Name != searchContext.Name {
			t.Fatalf("got %q, want %q", opts.Name, searchContext.Name)
		}
		return searchContext, nil
	})
	sc.GetSearchContextRepositoryRevisionsFunc.SetDefaultHook(func(ctx context.Context, searchContextID int64) ([]*types.SearchContextRepositoryRevisions, error) {
		if searchContextID != searchContext.ID {
			t.Fatalf("got %q, want %q", searchContextID, searchContext.ID)
		}
		return searchContextRepositoryRevisions, nil
	})

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)
	db.SearchContextsFunc.SetDefaultReturn(sc)

	op := search.RepoOptions{
		SearchContextSpec: "searchcontext",
	}
	repositoryResolver := NewResolver(logtest.Scoped(t), db, gsClient, nil, defaults.NewConnectionCache(logtest.Scoped(t)), nil)
	resolved, _, err := repositoryResolver.resolve(context.Background(), op)
	if err != nil {
		t.Fatal(err)
	}
	wantRepositoryRevisions := []*search.RepositoryRevisions{
		{Repo: repoA, Revs: searchContextRepositoryRevisions[0].Revisions},
		{Repo: repoB, Revs: searchContextRepositoryRevisions[1].Revisions},
	}
	if !reflect.DeepEqual(resolved.RepoRevs, wantRepositoryRevisions) {
		t.Errorf("got repository revisions %+v, want %+v", resolved.RepoRevs, wantRepositoryRevisions)
	}
}

func TestRepoHasFileContent(t *testing.T) {
	repoA := types.MinimalRepo{ID: 1, Name: "example.com/1"}
	repoB := types.MinimalRepo{ID: 2, Name: "example.com/2"}
	repoC := types.MinimalRepo{ID: 3, Name: "example.com/3"}
	repoD := types.MinimalRepo{ID: 4, Name: "example.com/4"}
	repoE := types.MinimalRepo{ID: 5, Name: "example.com/5"}

	mkHead := func(repo types.MinimalRepo) *search.RepositoryRevisions {
		return &search.RepositoryRevisions{
			Repo: repo,
			Revs: []string{""},
		}
	}

	repos := dbmocks.NewMockRepoStore()
	repos.ListMinimalReposFunc.SetDefaultHook(func(context.Context, database.ReposListOptions) ([]types.MinimalRepo, error) {
		return []types.MinimalRepo{repoA, repoB, repoC, repoD, repoE}, nil
	})

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)

	mockGitserver := gitserver.NewMockClient()
	mockGitserver.ResolveRevisionFunc.SetDefaultHook(func(_ context.Context, name api.RepoName, _ string, _ gitserver.ResolveRevisionOptions) (api.CommitID, error) {
		if name == repoE.Name {
			return "", &gitdomain.RevisionNotFoundError{}
		}
		return "", nil
	})

	unindexedCorpus := map[string]map[string]map[string]struct{}{
		string(repoC.Name): {
			"pathC": {
				"lineC": {},
				"line1": {},
				"line2": {},
			},
		},
		string(repoD.Name): {
			"pathD": {
				"lineD": {},
				"line1": {},
				"line2": {},
			},
		},
	}
	searcher.MockSearch = func(ctx context.Context, repo api.RepoName, repoID api.RepoID, commit api.CommitID, p *search.TextPatternInfo, fetchTimeout time.Duration, onMatch func(*protocol.FileMatch)) (limitHit bool, err error) {
		var pattern string
		if pt, ok := p.Query.(*protocol.PatternNode); ok {
			pattern = pt.Value
		} else {
			return false, errors.New("expected a simple regex pattern")
		}

		if r, ok := unindexedCorpus[string(repo)]; ok {
			for path, lines := range r {
				if len(p.IncludePaths) == 0 || p.IncludePaths[0] == path {
					for line := range lines {
						if pattern == line || pattern == "" {
							onMatch(&protocol.FileMatch{})
						}
					}
				}
			}
		}
		return false, nil
	}

	cases := []struct {
		name          string
		filters       []query.RepoHasFileContentArgs
		matchingRepos zoekt.ReposMap
		expected      []*search.RepositoryRevisions
	}{{
		name:          "no filters",
		filters:       nil,
		matchingRepos: nil,
		expected: []*search.RepositoryRevisions{
			mkHead(repoA),
			mkHead(repoB),
			mkHead(repoC),
			mkHead(repoD),
			mkHead(repoE),
		},
	}, {
		name: "bad path",
		filters: []query.RepoHasFileContentArgs{{
			Path: "bad path",
		}},
		matchingRepos: nil,
		expected:      []*search.RepositoryRevisions{},
	}, {
		name: "one indexed path",
		filters: []query.RepoHasFileContentArgs{{
			Path: "pathB",
		}},
		matchingRepos: zoekt.ReposMap{
			2: {
				Branches: []zoekt.RepositoryBranch{{
					Name: "HEAD",
				}},
			},
		},
		expected: []*search.RepositoryRevisions{
			mkHead(repoB),
		},
	}, {
		name: "one unindexed path",
		filters: []query.RepoHasFileContentArgs{{
			Path: "pathC",
		}},
		matchingRepos: nil,
		expected: []*search.RepositoryRevisions{
			mkHead(repoC),
		},
	}, {
		name: "one negated unindexed path",
		filters: []query.RepoHasFileContentArgs{{
			Path:    "pathC",
			Negated: true,
		}},
		matchingRepos: nil,
		expected: []*search.RepositoryRevisions{
			mkHead(repoD),
			mkHead(repoE),
		},
	}, {
		name: "path but no content",
		filters: []query.RepoHasFileContentArgs{{
			Path:    "pathC",
			Content: "lineB",
		}},
		matchingRepos: nil,
		expected:      []*search.RepositoryRevisions{},
	}, {
		name: "path and content",
		filters: []query.RepoHasFileContentArgs{{
			Path:    "pathC",
			Content: "lineC",
		}},
		matchingRepos: nil,
		expected: []*search.RepositoryRevisions{
			mkHead(repoC),
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Only repos A and B are indexed
			mockZoekt := NewMockStreamer()
			mockZoekt.ListFunc.PushReturn(&zoekt.RepoList{
				ReposMap: zoekt.ReposMap{
					uint32(repoA.ID): {
						Branches: []zoekt.RepositoryBranch{{Name: "HEAD"}},
					},
					uint32(repoB.ID): {
						Branches: []zoekt.RepositoryBranch{{Name: "HEAD"}},
					},
				},
			}, nil)

			mockZoekt.ListFunc.PushReturn(&zoekt.RepoList{
				ReposMap: tc.matchingRepos,
			}, nil)

			res := NewResolver(logtest.Scoped(t), db, mockGitserver, endpoint.Static("test"), defaults.NewConnectionCache(logtest.Scoped(t)), mockZoekt)
			resolved, _, err := res.resolve(context.Background(), search.RepoOptions{
				RepoFilters:    toParsedRepoFilters(".*"),
				HasFileContent: tc.filters,
			})
			require.NoError(t, err)

			require.Equal(t, tc.expected, resolved.RepoRevs)
		})
	}
}

func TestRepoHasCommitAfter(t *testing.T) {
	repoA := types.MinimalRepo{ID: 1, Name: "example.com/1"}
	repoB := types.MinimalRepo{ID: 2, Name: "example.com/2"}
	repoC := types.MinimalRepo{ID: 3, Name: "example.com/3"}
	repoD := types.MinimalRepo{ID: 4, Name: "example.com/4"}

	mkHead := func(repo types.MinimalRepo) *search.RepositoryRevisions {
		return &search.RepositoryRevisions{
			Repo: repo,
			Revs: []string{""},
		}
	}

	mockGitserver := gitserver.NewMockClient()
	mockGitserver.HasCommitAfterFunc.SetDefaultHook(func(_ context.Context, repoName api.RepoName, _ string, _ string) (bool, error) {
		switch repoName {
		case repoA.Name:
			return true, nil
		case repoB.Name:
			return true, nil
		case repoC.Name:
			return false, nil
		case repoD.Name:
			return false, &gitdomain.RevisionNotFoundError{}
		default:
			panic("unreachable")
		}
	})

	repos := dbmocks.NewMockRepoStore()
	repos.ListMinimalReposFunc.SetDefaultHook(func(_ context.Context, opts database.ReposListOptions) ([]types.MinimalRepo, error) {
		res := []types.MinimalRepo{}
		for _, r := range []types.MinimalRepo{repoA, repoB, repoC, repoD} {
			if matched, _ := regexp.MatchString(opts.IncludePatterns[0], string(r.Name)); matched {
				res = append(res, r)
			}
		}
		return res, nil
	})

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)

	cases := []struct {
		name        string
		nameFilter  string
		commitAfter *query.RepoHasCommitAfterArgs
		expected    []*search.RepositoryRevisions
		err         error
	}{{
		name:        "no filters",
		nameFilter:  ".*",
		commitAfter: nil,
		expected: []*search.RepositoryRevisions{
			mkHead(repoA),
			mkHead(repoB),
			mkHead(repoC),
			mkHead(repoD),
		},
		err: nil,
	}, {
		name:       "commit after",
		nameFilter: ".*",
		commitAfter: &query.RepoHasCommitAfterArgs{
			TimeRef: "yesterday",
		},
		expected: []*search.RepositoryRevisions{
			mkHead(repoA),
			mkHead(repoB),
		},
		err: nil,
	}, {
		name:       "err commit after",
		nameFilter: "repoD",
		commitAfter: &query.RepoHasCommitAfterArgs{
			TimeRef: "yesterday",
		},
		expected: nil,
		err:      ErrNoResolvedRepos,
	}, {
		name:       "no commit after",
		nameFilter: "repoC",
		commitAfter: &query.RepoHasCommitAfterArgs{
			TimeRef: "yesterday",
		},
		expected: nil,
		err:      ErrNoResolvedRepos,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := NewResolver(logtest.Scoped(t), db, nil, endpoint.Static("test"), defaults.NewConnectionCache(logtest.Scoped(t)), nil)
			res.gitserver = mockGitserver
			resolved, _, err := res.resolve(context.Background(), search.RepoOptions{
				RepoFilters: toParsedRepoFilters(tc.nameFilter),
				CommitAfter: tc.commitAfter,
			})
			require.Equal(t, tc.err, err)
			require.Equal(t, tc.expected, resolved.RepoRevs)
		})
	}
}
