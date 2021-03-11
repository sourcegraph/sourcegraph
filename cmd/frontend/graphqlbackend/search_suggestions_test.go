package graphqlbackend

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/atomic"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/inventory"
	searchrepos "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestSearchSuggestions(t *testing.T) {
	db := new(dbtesting.MockDB)

	limitOffset := &database.LimitOffset{Limit: searchrepos.SearchLimits().MaxRepos + 1}

	getSuggestions := func(t *testing.T, query, version string) []string {
		t.Helper()
		r, err := (&schemaResolver{db: db}).Search(context.Background(), &SearchArgs{Query: query, Version: version})
		if err != nil {
			t.Fatal("Search:", err)
		}
		results, err := r.Suggestions(context.Background(), &searchSuggestionsArgs{})
		if err != nil {
			t.Fatal("Suggestions:", err)
		}
		resultDescriptions := make([]string, len(results))
		for i, result := range results {
			resultDescriptions[i] = testStringResult(result)
		}
		return resultDescriptions
	}
	testSuggestions := func(t *testing.T, query, version string, want []string) {
		t.Helper()
		got := getSuggestions(t, query, version)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got != want\ngot:  %v\nwant: %v", got, want)
		}
	}

	mockSearchSymbols = func(ctx context.Context, args *search.TextParameters, limit int) (res []*FileMatchResolver, common *streaming.Stats, err error) {
		// TODO test symbol suggestions
		return nil, nil, nil
	}
	defer func() { mockSearchSymbols = nil }()

	mockDecodedViewerFinalSettings = &schema.Settings{}
	defer func() { mockDecodedViewerFinalSettings = nil }()

	searchVersions := []string{"V1", "V2"}

	t.Run("empty", func(t *testing.T) {
		for _, v := range searchVersions {
			testSuggestions(t, "", v, []string{})
		}
	})

	t.Run("whitespace", func(t *testing.T) {
		for _, v := range searchVersions {
			testSuggestions(t, " ", v, []string{})
		}
	})

	t.Run("single term", func(t *testing.T) {
		mockDecodedViewerFinalSettings = &schema.Settings{}
		defer func() { mockDecodedViewerFinalSettings = nil }()

		var calledReposListNamesAll, calledReposListFoo bool
		database.Mocks.Repos.ListRepoNames = func(_ context.Context, op database.ReposListOptions) ([]*types.RepoName, error) {

			// Validate that the following options are invariant
			// when calling the DB through Repos.List, no matter how
			// many times it is called for a single Search(...) operation.
			assertEqual(t, op.LimitOffset, limitOffset)

			if reflect.DeepEqual(op.IncludePatterns, []string{"foo"}) {
				// when treating term as repo: field
				calledReposListFoo = true
				return []*types.RepoName{{Name: "foo-repo"}}, nil
			} else {
				// when treating term as text query
				calledReposListNamesAll = true
				return []*types.RepoName{{Name: "bar-repo"}}, nil
			}
		}
		database.Mocks.Repos.Count = mockCount
		database.Mocks.Repos.MockGetByName(t, "repo", 1)
		backend.Mocks.Repos.MockResolveRev_NoCheck(t, api.CommitID("deadbeef"))

		defer func() { database.Mocks = database.MockStores{} }()
		git.Mocks.ResolveRevision = func(rev string, opt git.ResolveRevisionOptions) (api.CommitID, error) {
			return api.CommitID("deadbeef"), nil
		}
		defer git.ResetMocks()

		calledSearchFilesInRepos := atomic.NewBool(false)
		mockSearchFilesInRepos = func(args *search.TextParameters) ([]*FileMatchResolver, *streaming.Stats, error) {
			calledSearchFilesInRepos.Store(true)
			if want := "foo"; args.PatternInfo.Pattern != want {
				t.Errorf("got %q, want %q", args.PatternInfo.Pattern, want)
			}
			fm := mkFileMatch(db, &types.RepoName{Name: "repo"}, "dir/file")
			fm.URI = "git://repo?rev#dir/file"
			fm.CommitID = "rev"
			return []*FileMatchResolver{fm}, &streaming.Stats{}, nil
		}
		defer func() { mockSearchFilesInRepos = nil }()
		for _, v := range searchVersions {
			testSuggestions(t, "foo", v, []string{"repo:foo-repo", "file:dir/file"})
			if !calledReposListNamesAll {
				t.Error("!calledReposListNamesAll")
			}
			if !calledReposListFoo {
				t.Error("!calledReposListFoo")
			}
			if !calledSearchFilesInRepos.Load() {
				t.Error("!calledSearchFilesInRepos")
			}
		}
	})

	t.Run("repogroup: and single term", func(t *testing.T) {
		t.Skip("TODO(slimsag): this test is not reliable")
		var mu sync.Mutex

		mockDecodedViewerFinalSettings = &schema.Settings{}
		defer func() { mockDecodedViewerFinalSettings = nil }()

		var calledReposListRepoNamesInGroup, calledReposListFooRepo3 bool
		database.Mocks.Repos.ListRepoNames = func(_ context.Context, op database.ReposListOptions) ([]*types.RepoName, error) {
			mu.Lock()
			defer mu.Unlock()
			wantReposInGroup := database.ReposListOptions{IncludePatterns: []string{`^foo-repo1$|^repo3$`}, LimitOffset: limitOffset}    // when treating term as repo: field
			wantFooRepo3 := database.ReposListOptions{IncludePatterns: []string{"foo", `^foo-repo1$|^repo3$`}, LimitOffset: limitOffset} // when treating term as repo: field
			if reflect.DeepEqual(op, wantReposInGroup) {
				calledReposListRepoNamesInGroup = true
				return []*types.RepoName{
					{Name: "foo-repo1"},
					{Name: "repo3"},
				}, nil
			} else if reflect.DeepEqual(op, wantFooRepo3) {
				calledReposListFooRepo3 = true
				return []*types.RepoName{{Name: "foo-repo1"}}, nil
			}
			t.Errorf("got %+v, want %+v or %+v", op, wantReposInGroup, wantFooRepo3)
			return nil, nil
		}
		database.Mocks.Repos.Count = mockCount
		defer func() { database.Mocks = database.MockStores{} }()
		database.Mocks.Repos.MockGetByName(t, "repo", 1)
		backend.Mocks.Repos.MockResolveRev_NoCheck(t, api.CommitID("deadbeef"))

		calledSearchFilesInRepos := atomic.NewBool(false)
		mockSearchFilesInRepos = func(args *search.TextParameters) ([]*FileMatchResolver, *streaming.Stats, error) {
			mu.Lock()
			defer mu.Unlock()
			calledSearchFilesInRepos.Store(true)
			if args.PatternInfo.Pattern != "." && args.PatternInfo.Pattern != "foo" {
				t.Errorf("got %q, want %q", args.PatternInfo.Pattern, `"foo" or "."`)
			}
			mk := func(name api.RepoName, path string) *FileMatchResolver {
				fm := mkFileMatch(db, &types.RepoName{Name: name}, path)
				fm.URI = fileMatchURI(name, "rev", path)
				fm.CommitID = "rev"
				return fm
			}
			return []*FileMatchResolver{
				mk("repo3", "dir/foo-repo3-file-name-match"),
				mk("repo1", "dir/foo-repo1-file-name-match"),
				mk("repo", "dir/file-content-match"),
			}, &streaming.Stats{}, nil
		}
		defer func() { mockSearchFilesInRepos = nil }()

		calledResolveRepoGroups := false
		searchrepos.MockResolveRepoGroups = func() (map[string][]searchrepos.RepoGroupValue, error) {
			mu.Lock()
			defer mu.Unlock()
			calledResolveRepoGroups = true
			return map[string][]searchrepos.RepoGroupValue{
				"baz": {
					searchrepos.RepoPath("foo-repo1"),
					searchrepos.RepoPath("repo3"),
				},
			}, nil
		}
		defer func() { searchrepos.MockResolveRepoGroups = nil }()
		for _, v := range searchVersions {
			testSuggestions(t, "repogroup:baz foo", v, []string{"repo:foo-repo1", "file:dir/foo-repo3-file-name-match", "file:dir/foo-repo1-file-name-match", "file:dir/file-content-match"})
			if !calledReposListRepoNamesInGroup {
				t.Error("!calledReposListRepoNamesInGroup")
			}
			if !calledReposListFooRepo3 {
				t.Error("!calledReposListFooRepo3")
			}
			if !calledSearchFilesInRepos.Load() {
				t.Error("!calledSearchFilesInRepos")
			}
			if !calledResolveRepoGroups {
				t.Error("!calledResolveRepoGroups")
			}

		}
	})

	t.Run("repo: field", func(t *testing.T) {
		var mu sync.Mutex

		mockDecodedViewerFinalSettings = &schema.Settings{}
		defer func() { mockDecodedViewerFinalSettings = nil }()

		calledReposListRepoNames := false
		database.Mocks.Repos.ListRepoNames = func(_ context.Context, op database.ReposListOptions) ([]*types.RepoName, error) {
			mu.Lock()
			defer mu.Unlock()
			calledReposListRepoNames = true

			// Validate that the following options are invariant
			// when calling the DB through Repos.List, no matter how
			// many times it is called for a single Search(...) operation.
			assertEqual(t, op.LimitOffset, limitOffset)
			assertEqual(t, op.IncludePatterns, []string{"foo"})

			return []*types.RepoName{{Name: "foo-repo"}}, nil
		}
		database.Mocks.Repos.Count = mockCount
		defer func() { database.Mocks.Repos.ListRepoNames = nil }()

		// Mock to bypass language suggestions.
		mockShowLangSuggestions = func() ([]SearchSuggestionResolver, error) { return nil, nil }
		defer func() { mockShowLangSuggestions = nil }()

		calledSearchFilesInRepos := atomic.NewBool(false)
		mockSearchFilesInRepos = func(args *search.TextParameters) ([]*FileMatchResolver, *streaming.Stats, error) {
			mu.Lock()
			defer mu.Unlock()
			calledSearchFilesInRepos.Store(true)
			repos, err := getRepos(context.Background(), args.RepoPromise)
			if err != nil {
				t.Error(err)
			}
			if want := "foo-repo"; len(repos) != 1 || string(repos[0].Repo.Name) != want {
				t.Errorf("got %q, want %q", repos, want)
			}
			return []*FileMatchResolver{
				mkFileMatch(db, &types.RepoName{Name: "foo-repo"}, "dir/file"),
			}, &streaming.Stats{}, nil
		}
		defer func() { mockSearchFilesInRepos = nil }()

		for _, v := range searchVersions {
			testSuggestions(t, "repo:foo", v, []string{"repo:foo-repo", "file:dir/file"})
			if !calledReposListRepoNames {
				t.Error("!calledReposListRepoNames")
			}
			if !calledSearchFilesInRepos.Load() {
				t.Error("!calledSearchFilesInRepos")
			}
		}
	})

	t.Run("repo: field for language suggestions", func(t *testing.T) {
		mockDecodedViewerFinalSettings = &schema.Settings{}
		defer func() { mockDecodedViewerFinalSettings = nil }()

		database.Mocks.Repos.List = func(_ context.Context, have database.ReposListOptions) ([]*types.Repo, error) {
			want := database.ReposListOptions{
				IncludePatterns: []string{"foo"},
				LimitOffset: &database.LimitOffset{
					Limit: 1,
				},
			}
			if diff := cmp.Diff(have, want); diff != "" {
				t.Error(diff)
			}
			return []*types.Repo{{Name: "foo-repo"}}, nil
		}
		database.Mocks.Repos.ListRepoNames = func(_ context.Context, have database.ReposListOptions) ([]*types.RepoName, error) {
			want := database.ReposListOptions{
				IncludePatterns: []string{"foo"},
				LimitOffset: &database.LimitOffset{
					Limit: 1,
				},
			}
			if diff := cmp.Diff(have, want); diff != "" {
				t.Error(diff)
			}
			return []*types.RepoName{{Name: "foo-repo"}}, nil
		}
		database.Mocks.Repos.Count = mockCount
		defer func() { database.Mocks.Repos.List = nil }()
		defer func() { database.Mocks.Repos.ListRepoNames = nil }()
		git.Mocks.ResolveRevision = func(rev string, opt git.ResolveRevisionOptions) (api.CommitID, error) {
			return api.CommitID("deadbeef"), nil
		}
		defer git.ResetMocks()

		calledReposGetInventory := false
		backend.Mocks.Repos.GetInventory = func(_ context.Context, _ *types.Repo, _ api.CommitID) (*inventory.Inventory, error) {
			calledReposGetInventory = true
			return &inventory.Inventory{
				Languages: []inventory.Lang{
					{Name: "Go"},
					{Name: "TypeScript"},
					{Name: "Java"},
				},
			}, nil
		}
		defer func() { backend.Mocks.Repos.GetInventory = nil }()

		// Mock to bypass other suggestions.
		mockShowRepoSuggestions = func() ([]SearchSuggestionResolver, error) { return nil, nil }
		defer func() { mockShowRepoSuggestions = nil }()
		mockShowFileSuggestions = func() ([]SearchSuggestionResolver, error) { return nil, nil }
		defer func() { mockShowFileSuggestions = nil }()
		mockShowSymbolMatches = func() ([]SearchSuggestionResolver, error) { return nil, nil }
		defer func() { mockShowSymbolMatches = nil }()

		for _, v := range searchVersions {
			testSuggestions(t, "repo:foo@master", v, []string{"lang:go", "lang:java", "lang:typescript"})
			if !calledReposGetInventory {
				t.Error("!calledReposGetInventory")
			}
		}
	})

	t.Run("repo: and file: field", func(t *testing.T) {
		var mu sync.Mutex

		mockDecodedViewerFinalSettings = &schema.Settings{}
		defer func() { mockDecodedViewerFinalSettings = nil }()

		calledReposListRepoNames := false
		database.Mocks.Repos.ListRepoNames = func(_ context.Context, op database.ReposListOptions) ([]*types.RepoName, error) {
			mu.Lock()
			defer mu.Unlock()
			calledReposListRepoNames = true

			// Validate that the following options are invariant
			// when calling the DB through Repos.List, no matter how
			// many times it is called for a single Search(...) operation.
			assertEqual(t, op.LimitOffset, limitOffset)
			assertEqual(t, op.IncludePatterns, []string{"foo"})

			return []*types.RepoName{{Name: "foo-repo"}}, nil
		}
		database.Mocks.Repos.Count = mockCount
		defer func() { database.Mocks.Repos.ListRepoNames = nil }()

		// Mock to bypass language suggestions.
		mockShowLangSuggestions = func() ([]SearchSuggestionResolver, error) { return nil, nil }
		defer func() { mockShowLangSuggestions = nil }()

		calledSearchFilesInRepos := atomic.NewBool(false)
		mockSearchFilesInRepos = func(args *search.TextParameters) ([]*FileMatchResolver, *streaming.Stats, error) {
			mu.Lock()
			defer mu.Unlock()
			calledSearchFilesInRepos.Store(true)
			repos, err := getRepos(context.Background(), args.RepoPromise)
			if err != nil {
				t.Error(err)
			}
			if want := "foo-repo"; len(repos) != 1 || string(repos[0].Repo.Name) != want {
				t.Errorf("got %q, want %q", repos, want)
			}
			return []*FileMatchResolver{
				mkFileMatch(db, &types.RepoName{Name: "foo-repo"}, "dir/bar-file"),
			}, &streaming.Stats{}, nil
		}
		defer func() { mockSearchFilesInRepos = nil }()

		for _, v := range searchVersions {
			testSuggestions(t, "repo:foo file:bar", v, []string{"file:dir/bar-file"})
			if !calledReposListRepoNames {
				t.Error("!calledReposListRepoNames")
			}
			if !calledSearchFilesInRepos.Load() {
				t.Error("!calledSearchFilesInRepos")
			}
		}
	})
}
