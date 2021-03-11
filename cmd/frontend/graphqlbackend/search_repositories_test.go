package graphqlbackend

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/search"
	searchbackend "github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestSearchRepositories(t *testing.T) {
	db := new(dbtesting.MockDB)

	repositories := []*search.RepositoryRevisions{
		{Repo: &types.RepoName{ID: 123, Name: "foo/one"}, Revs: []search.RevisionSpecifier{{RevSpec: ""}}},
		{Repo: &types.RepoName{ID: 456, Name: "foo/no-match"}, Revs: []search.RevisionSpecifier{{RevSpec: ""}}},
		{Repo: &types.RepoName{ID: 789, Name: "bar/one"}, Revs: []search.RevisionSpecifier{{RevSpec: ""}}},
	}

	zoekt := &searchbackend.Zoekt{Client: &searchbackend.FakeSearcher{}}

	mockSearchFilesInRepos = func(args *search.TextParameters) (matches []*FileMatchResolver, common *streaming.Stats, err error) {
		repos, err := getRepos(context.Background(), args.RepoPromise)
		if err != nil {
			return nil, nil, err
		}
		repoName := repos[0].Repo.Name
		switch repoName {
		case "foo/one":
			return []*FileMatchResolver{
				mkFileMatchResolver(db, result.FileMatch{
					URI:  "git://" + string(repoName) + "?1a2b3c#" + "f.go",
					Repo: &types.RepoName{ID: 123},
				}),
			}, &streaming.Stats{}, nil
		case "bar/one":
			return []*FileMatchResolver{
				mkFileMatchResolver(db, result.FileMatch{
					URI:  "git://" + string(repoName) + "?1a2b3c#" + "f.go",
					Repo: &types.RepoName{ID: 789},
				}),
			}, &streaming.Stats{}, nil
		case "foo/no-match":
			return []*FileMatchResolver{}, &streaming.Stats{}, nil
		default:
			return nil, &streaming.Stats{}, errors.New("Unexpected repo")
		}
	}

	cases := []struct {
		name string
		q    string
		want []string
	}{{
		name: "all",
		q:    "type:repo",
		want: []string{"bar/one", "foo/no-match", "foo/one"},
	}, {
		name: "pattern filter",
		q:    "type:repo foo/one",
		want: []string{"foo/one"},
	}, {
		name: "repohasfile",
		q:    "foo type:repo repohasfile:f.go",
		want: []string{"foo/one"},
	}, {
		name: "case yes match",
		q:    "foo case:yes",
		want: []string{"foo/no-match", "foo/one"},
	}, {
		name: "case no match",
		q:    "Foo case:no",
		want: []string{"foo/no-match", "foo/one"},
	}, {
		name: "case exclude all",
		q:    "Foo case:yes",
		want: []string{},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := query.ParseLiteral(tc.q)
			if err != nil {
				t.Fatal(err)
			}

			pattern, err := getPatternInfo(q, &getPatternInfoOptions{fileMatchLimit: 1})
			if err != nil {
				t.Fatal(err)
			}

			results, _, err := searchRepositoriesBatch(context.Background(), db, &search.TextParameters{
				PatternInfo: pattern,
				RepoPromise: (&search.Promise{}).Resolve(repositories),
				Query:       q,
				Zoekt:       zoekt,
			}, int32(100))
			if err != nil {
				t.Fatal(err)
			}

			var got []string
			for _, res := range results {
				r, ok := res.ToRepository()
				if !ok {
					t.Fatal("expected repo result")
				}
				got = append(got, r.Name())
			}
			sort.Strings(got)

			if !cmp.Equal(tc.want, got, cmpopts.EquateEmpty()) {
				t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(tc.want, got))
			}
		})
	}
}

func searchRepositoriesBatch(ctx context.Context, db dbutil.DB, args *search.TextParameters, limit int32) ([]SearchResultResolver, streaming.Stats, error) {
	return collectStream(func(stream Sender) error {
		return searchRepositories(ctx, db, args, limit, stream)
	})
}

func TestRepoShouldBeAdded(t *testing.T) {
	db := new(dbtesting.MockDB)

	mockSearchFilesInRepos = func(args *search.TextParameters) (matches []*FileMatchResolver, common *streaming.Stats, err error) {
		repos, err := getRepos(context.Background(), args.RepoPromise)
		if err != nil {
			return nil, nil, err
		}
		repoName := repos[0].Repo.Name
		switch repoName {
		case "foo/one":
			return []*FileMatchResolver{
				mkFileMatchResolver(db, result.FileMatch{
					URI:  "git://" + string(repoName) + "?1a2b3c#" + "foo.go",
					Repo: &types.RepoName{ID: 123},
				}),
			}, &streaming.Stats{}, nil
		case "foo/no-match":
			return []*FileMatchResolver{}, &streaming.Stats{}, nil
		default:
			return nil, &streaming.Stats{}, errors.New("Unexpected repo")
		}
	}

	zoekt := &searchbackend.Zoekt{Client: &searchbackend.FakeSearcher{}}

	t.Run("repo should be included in results, query has repoHasFile filter", func(t *testing.T) {
		repo := &search.RepositoryRevisions{Repo: &types.RepoName{ID: 123, Name: "foo/one"}, Revs: []search.RevisionSpecifier{{RevSpec: ""}}}
		mockSearchFilesInRepos = func(args *search.TextParameters) (matches []*FileMatchResolver, common *streaming.Stats, err error) {
			return []*FileMatchResolver{
				mkFileMatchResolver(db, result.FileMatch{
					URI:  "git://" + string(repo.Repo.Name) + "?1a2b3c#" + "foo.go",
					Repo: &types.RepoName{ID: 123},
				}),
			}, &streaming.Stats{}, nil
		}
		pat := &search.TextPatternInfo{Pattern: "", FilePatternsReposMustInclude: []string{"foo"}, IsRegExp: true, FileMatchLimit: 1, PathPatternsAreCaseSensitive: false, PatternMatchesContent: true, PatternMatchesPath: true}
		shouldBeAdded, err := repoShouldBeAdded(context.Background(), db, zoekt, repo, pat)
		if err != nil {
			t.Fatal(err)
		}
		if !shouldBeAdded {
			t.Errorf("Expected shouldBeAdded for repo %v to be true, but got false", repo)
		}
	})

	t.Run("repo shouldn't be included in results, query has repoHasFile filter ", func(t *testing.T) {
		repo := &search.RepositoryRevisions{Repo: &types.RepoName{Name: "foo/no-match"}, Revs: []search.RevisionSpecifier{{RevSpec: ""}}}
		mockSearchFilesInRepos = func(args *search.TextParameters) (matches []*FileMatchResolver, common *streaming.Stats, err error) {
			return []*FileMatchResolver{}, &streaming.Stats{}, nil
		}
		pat := &search.TextPatternInfo{Pattern: "", FilePatternsReposMustInclude: []string{"foo"}, IsRegExp: true, FileMatchLimit: 1, PathPatternsAreCaseSensitive: false, PatternMatchesContent: true, PatternMatchesPath: true}
		shouldBeAdded, err := repoShouldBeAdded(context.Background(), db, zoekt, repo, pat)
		if err != nil {
			t.Fatal(err)
		}
		if shouldBeAdded {
			t.Errorf("Expected shouldBeAdded for repo %v to be false, but got true", repo)
		}
	})

	t.Run("repo shouldn't be included in results, query has -repoHasFile filter", func(t *testing.T) {
		repo := &search.RepositoryRevisions{Repo: &types.RepoName{ID: 123, Name: "foo/one"}, Revs: []search.RevisionSpecifier{{RevSpec: ""}}}
		mockSearchFilesInRepos = func(args *search.TextParameters) (matches []*FileMatchResolver, common *streaming.Stats, err error) {
			return []*FileMatchResolver{
				mkFileMatchResolver(db, result.FileMatch{
					URI:  "git://" + string(repo.Repo.Name) + "?1a2b3c#" + "foo.go",
					Repo: &types.RepoName{ID: 123},
				}),
			}, &streaming.Stats{}, nil
		}
		pat := &search.TextPatternInfo{Pattern: "", FilePatternsReposMustExclude: []string{"foo"}, IsRegExp: true, FileMatchLimit: 1, PathPatternsAreCaseSensitive: false, PatternMatchesContent: true, PatternMatchesPath: true}
		shouldBeAdded, err := repoShouldBeAdded(context.Background(), db, zoekt, repo, pat)
		if err != nil {
			t.Fatal(err)
		}
		if shouldBeAdded {
			t.Errorf("Expected shouldBeAdded for repo %v to be false, but got true", repo)
		}
	})

	t.Run("repo should be included in results, query has -repoHasFile filter", func(t *testing.T) {
		repo := &search.RepositoryRevisions{Repo: &types.RepoName{Name: "foo/no-match"}, Revs: []search.RevisionSpecifier{{RevSpec: ""}}}
		mockSearchFilesInRepos = func(args *search.TextParameters) (matches []*FileMatchResolver, common *streaming.Stats, err error) {
			return []*FileMatchResolver{}, &streaming.Stats{}, nil
		}
		pat := &search.TextPatternInfo{Pattern: "", FilePatternsReposMustExclude: []string{"foo"}, IsRegExp: true, FileMatchLimit: 1, PathPatternsAreCaseSensitive: false, PatternMatchesContent: true, PatternMatchesPath: true}
		shouldBeAdded, err := repoShouldBeAdded(context.Background(), db, zoekt, repo, pat)
		if err != nil {
			t.Fatal(err)
		}
		if !shouldBeAdded {
			t.Errorf("Expected shouldBeAdded for repo %v to be true, but got false", repo)
		}
	})
}

// repoShouldBeAdded determines whether a repository should be included in the result set based on whether the repository fits in the subset
// of repostiories specified in the query's `repohasfile` and `-repohasfile` fields if they exist.
func repoShouldBeAdded(ctx context.Context, db dbutil.DB, zoekt *searchbackend.Zoekt, repo *search.RepositoryRevisions, pattern *search.TextPatternInfo) (bool, error) {
	repos := []*search.RepositoryRevisions{repo}
	args := search.TextParameters{
		PatternInfo: pattern,
		Zoekt:       zoekt,
	}
	rsta, err := reposToAdd(ctx, db, &args, repos)
	if err != nil {
		return false, err
	}
	return len(rsta) == 1, nil
}

func TestMatchRepos(t *testing.T) {
	want := makeRepositoryRevisions("foo/bar", "abc/foo")
	in := append(want, makeRepositoryRevisions("beef/bam", "qux/bas")...)
	pattern := regexp.MustCompile("foo")

	results := make(chan []*search.RepositoryRevisions)
	go func() {
		defer close(results)
		matchRepos(pattern, in, results)
	}()
	var repos []*search.RepositoryRevisions
	for matched := range results {
		repos = append(repos, matched...)
	}

	// because of the concurrency we cannot rely on the order of "repos" to be the
	// same as "want". Hence we create map of repo names and compare those.
	toMap := func(reporevs []*search.RepositoryRevisions) map[string]struct{} {
		out := map[string]struct{}{}
		for _, r := range reporevs {
			out[string(r.Repo.Name)] = struct{}{}
		}
		return out
	}
	if !reflect.DeepEqual(toMap(repos), toMap(want)) {
		t.Fatalf("expected %v, got %v", want, repos)
	}
}

func BenchmarkSearchRepositories(b *testing.B) {
	db := new(dbtesting.MockDB)

	n := 200 * 1000
	repos := make([]*search.RepositoryRevisions, n)
	for i := 0; i < n; i++ {
		repo := &types.RepoName{Name: api.RepoName("github.com/org/repo" + strconv.Itoa(i))}
		repos[i] = &search.RepositoryRevisions{Repo: repo, Revs: []search.RevisionSpecifier{{}}}
	}
	q, err := query.ParseLiteral("context.WithValue")
	if err != nil {
		b.Fatal(err)
	}
	options := &getPatternInfoOptions{}
	textPatternInfo, err := getPatternInfo(q, options)
	if err != nil {
		b.Fatal(err)
	}
	tp := search.TextParameters{
		PatternInfo: textPatternInfo,
		RepoPromise: (&search.Promise{}).Resolve(repos),
		Query:       q,
	}
	for i := 0; i < b.N; i++ {
		_, _, err = searchRepositoriesBatch(context.Background(), db, &tp, options.fileMatchLimit)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func mkFileMatchResolver(db dbutil.DB, fm result.FileMatch) *FileMatchResolver {
	var repo *RepositoryResolver
	if fm.Repo != nil {
		repo = NewRepositoryResolver(db, fm.Repo.ToRepo())
	}
	return &FileMatchResolver{
		db:           db,
		FileMatch:    fm,
		RepoResolver: repo,
	}
}
