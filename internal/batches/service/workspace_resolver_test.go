package service

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	streamapi "github.com/sourcegraph/sourcegraph/internal/search/streaming/api"
	streamhttp "github.com/sourcegraph/sourcegraph/internal/search/streaming/http"
	"github.com/sourcegraph/sourcegraph/internal/types"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestSetDefaultQueryCount(t *testing.T) {
	for in, want := range map[string]string{
		"":                     hardCodedCount,
		"count:10":             "count:10",
		"count:all":            "count:all",
		"r:foo":                "r:foo" + hardCodedCount,
		"r:foo count:10":       "r:foo count:10",
		"r:foo count:10 f:bar": "r:foo count:10 f:bar",
		"r:foo count:":         "r:foo count:" + hardCodedCount,
		"r:foo count:xyz":      "r:foo count:xyz" + hardCodedCount,
	} {
		t.Run(in, func(t *testing.T) {
			have := setDefaultQueryCount(in)
			if have != want {
				t.Errorf("unexpected query: have %q; want %q", have, want)
			}
		})
	}
}

func TestService_ResolveWorkspacesForBatchSpec(t *testing.T) {
	ctx := context.Background()

	logger := logtest.Scoped(t)

	db := database.NewDB(logger, dbtest.NewDB(t))
	s := store.New(db, &observation.TestContext, nil)

	u := bt.CreateTestUser(t, db, false)

	rs, _ := bt.CreateTestRepos(t, ctx, db, 7)
	unsupported, _ := bt.CreateAWSCodeCommitTestRepos(t, ctx, db, 1)
	// Allow access to all repos but rs[4].
	bt.MockRepoPermissions(t, db, u.ID, rs[0].ID, rs[1].ID, rs[2].ID, rs[3].ID, rs[5].ID, rs[6].ID, unsupported[0].ID)

	defaultBranches := map[api.RepoName]defaultBranch{
		rs[0].Name:          {branch: "branch-1", commit: api.CommitID("6f152ece24b9424edcd4da2b82989c5c2bea64c3")},
		rs[1].Name:          {branch: "branch-2", commit: api.CommitID("2840a42c7809c22b16fda7099c725d1ef197961c")},
		rs[2].Name:          {branch: "branch-3", commit: api.CommitID("aead85d33485e115b33ec4045c55bac97e03fd26")},
		rs[3].Name:          {branch: "branch-4", commit: api.CommitID("26ac0350471daac3401a9314fd64e714370837a6")},
		rs[4].Name:          {branch: "branch-6", commit: api.CommitID("010b133ece7a79187cad209b27099232485a5476")},
		rs[5].Name:          {branch: "branch-7", commit: api.CommitID("ee0c70114fc1a92c96ceae495519a4d3df979efe")},
		unsupported[0].Name: {branch: "branch-5", commit: api.CommitID("c167bd633e2868585b86ef129d07f63dee46b84a")},
		// No entry for rs[6], is not cloned yet. This is to test we don't error when some repos are results of a
		// search but not yet cloned.
	}
	steps := []batcheslib.Step{{Run: "echo 1"}}
	buildRepoWorkspace := func(repo *types.Repo, branch, commit string, fileMatches []string) *RepoWorkspace {
		sort.Strings(fileMatches)
		if branch == "" {
			branch = defaultBranches[repo.Name].branch
		}
		if commit == "" {
			commit = string(defaultBranches[repo.Name].commit)
		}
		return &RepoWorkspace{
			RepoRevision: &RepoRevision{
				Repo:        repo,
				Branch:      branch,
				Commit:      api.CommitID(commit),
				FileMatches: fileMatches,
			},
			Path:               "",
			OnlyFetchWorkspace: false,
		}
	}
	buildIgnoredRepoWorkspace := func(repo *types.Repo, branch, commit string, fileMatches []string) *RepoWorkspace {
		ws := buildRepoWorkspace(repo, branch, commit, fileMatches)
		ws.Ignored = true
		return ws
	}
	buildUnsupportedRepoWorkspace := func(repo *types.Repo, branch, commit string, fileMatches []string) *RepoWorkspace {
		ws := buildRepoWorkspace(repo, branch, commit, fileMatches)
		ws.Unsupported = true
		return ws
	}

	newGitserverClient := func(commitMap map[api.CommitID]bool, branches map[string]api.CommitID) gitserver.Client {
		gitserverClient := gitserver.NewMockClient()
		gitserverClient.GetDefaultBranchFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, short bool) (string, api.CommitID, error) {
			if res, ok := defaultBranches[repo]; ok {
				return res.branch, res.commit, nil
			}
			return "", "", &gitdomain.RepoNotExistError{Repo: repo}
		})

		gitserverClient.StatFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, commit api.CommitID, s string) (fs.FileInfo, error) {
			hasBatchIgnore, ok := commitMap[commit]
			if !ok {
				return nil, errors.Newf("unknown commit: %s", commit)
			}
			if hasBatchIgnore {
				return &fileutil.FileInfo{Name_: ".batchignore", Mode_: 0}, nil
			}
			return nil, os.ErrNotExist
		})

		gitserverClient.ResolveRevisionFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, spec string, rro gitserver.ResolveRevisionOptions) (api.CommitID, error) {
			if commit, ok := branches[spec]; ok {
				return commit, nil
			}
			return "", errors.Newf("unknown spec: %s", spec)
		})

		return gitserverClient
	}

	t.Run("repositoriesMatchingQuery", func(t *testing.T) {
		batchSpec := &batcheslib.BatchSpec{
			On: []batcheslib.OnQueryOrRepository{
				{RepositoriesMatchingQuery: "repohasfile:horse.txt"},
				// In our test the search API returns the same results for both.
				{RepositoriesMatchingQuery: "repohasfile:horse.txt duplicate"},
				// This query returns 0 results.
				{RepositoriesMatchingQuery: "select:repo r:sourcegraph"},
			},
			Steps: steps,
		}

		gs := newGitserverClient(map[api.CommitID]bool{
			defaultBranches[rs[0].Name].commit:          false,
			defaultBranches[rs[1].Name].commit:          true,
			defaultBranches[rs[2].Name].commit:          true,
			defaultBranches[rs[3].Name].commit:          false,
			defaultBranches[unsupported[0].Name].commit: false,
		}, nil)

		eventMatches := []streamhttp.EventMatch{
			&streamhttp.EventContentMatch{
				Type:         streamhttp.ContentMatchType,
				Path:         "repo-0/test",
				RepositoryID: int32(rs[0].ID),
			},
			&streamhttp.EventContentMatch{
				Type:         streamhttp.ContentMatchType,
				Path:         "repo-0/duplicate-test",
				RepositoryID: int32(rs[0].ID),
			},
			&streamhttp.EventRepoMatch{
				Type:         streamhttp.RepoMatchType,
				RepositoryID: int32(rs[1].ID),
			},
			&streamhttp.EventPathMatch{
				Type:         streamhttp.PathMatchType,
				Path:         "repo-2/readme",
				RepositoryID: int32(rs[2].ID),
			},
			&streamhttp.EventSymbolMatch{
				Type:         streamhttp.SymbolMatchType,
				Path:         "repo-3/readme",
				RepositoryID: int32(rs[3].ID),
			},
			&streamhttp.EventPathMatch{
				Type:         streamhttp.PathMatchType,
				Path:         "unsupported/path",
				RepositoryID: int32(unsupported[0].ID),
			},
			// Result for rs[6] which is not cloned yet.
			&streamhttp.EventPathMatch{
				Type:         streamhttp.RepoMatchType,
				RepositoryID: int32(rs[6].ID),
			},
		}
		searchMatches := map[string][]streamhttp.EventMatch{
			"repohasfile:horse.txt count:all":           eventMatches,
			"repohasfile:horse.txt duplicate count:all": eventMatches,
			// No results for this one. rs[5] should not appear in the result, as
			// it didn't match anything in the search results.
			"select:repo r:sourcegraph count:all": {},
		}

		want := []*RepoWorkspace{
			buildRepoWorkspace(rs[0], "", "", []string{"repo-0/test", "repo-0/duplicate-test"}),
			buildIgnoredRepoWorkspace(rs[1], "", "", []string{}),
			buildIgnoredRepoWorkspace(rs[2], "", "", []string{"repo-2/readme"}),
			buildRepoWorkspace(rs[3], "", "", []string{"repo-3/readme"}),
			buildUnsupportedRepoWorkspace(unsupported[0], "", "", []string{"unsupported/path"}),
		}
		resolveWorkspacesAndCompare(t, s, gs, u, searchMatches, batchSpec, want)
	})

	t.Run("repositories", func(t *testing.T) {
		batchSpec := &batcheslib.BatchSpec{
			On: []batcheslib.OnQueryOrRepository{
				{Repository: string(rs[0].Name)},
				{Repository: string(rs[1].Name), Branch: "non-default-branch"},
				{Repository: string(rs[2].Name), Branches: []string{"other-non-default-branch", "yet-another-non-default-branch"}},
				{Repository: string(rs[3].Name)},
				{Repository: string(unsupported[0].Name)},
			},
			Steps: steps,
		}

		gs := newGitserverClient(
			map[api.CommitID]bool{
				defaultBranches[rs[0].Name].commit: false,
				"d34db33f":                         false,
				"c0ff33":                           false,
				"b33a":                             false,
				defaultBranches[rs[3].Name].commit: true,
				defaultBranches[unsupported[0].Name].commit: false,
			},
			map[string]api.CommitID{
				defaultBranches[rs[0].Name].branch:          defaultBranches[rs[0].Name].commit,
				"non-default-branch":                        "d34db33f",
				"other-non-default-branch":                  "c0ff33",
				"yet-another-non-default-branch":            "b33a",
				defaultBranches[rs[3].Name].branch:          defaultBranches[rs[3].Name].commit,
				defaultBranches[unsupported[0].Name].branch: defaultBranches[unsupported[0].Name].commit,
			},
		)

		want := []*RepoWorkspace{
			buildRepoWorkspace(rs[0], "", "", []string{}),
			buildRepoWorkspace(rs[1], "non-default-branch", "d34db33f", []string{}),
			buildRepoWorkspace(rs[2], "other-non-default-branch", "c0ff33", []string{}),
			buildRepoWorkspace(rs[2], "yet-another-non-default-branch", "b33a", []string{}),
			buildIgnoredRepoWorkspace(rs[3], "", "", []string{}),
			buildUnsupportedRepoWorkspace(unsupported[0], "", "", []string{}),
		}

		resolveWorkspacesAndCompare(t, s, gs, u, map[string][]streamhttp.EventMatch{}, batchSpec, want)
	})

	t.Run("repositories overriding previous queries", func(t *testing.T) {
		batchSpec := &batcheslib.BatchSpec{
			On: []batcheslib.OnQueryOrRepository{
				// This query is just a placeholder; we'll set up the search
				// results further down to return rs[2].
				{RepositoriesMatchingQuery: "r:rs-2"},
				{Repository: string(rs[0].Name)},
				{Repository: string(rs[1].Name), Branch: "non-default-branch"},
				{Repository: string(rs[1].Name), Branch: "a-different-non-default-branch"},
				{Repository: string(rs[2].Name), Branches: []string{"other-non-default-branch", "yet-another-non-default-branch"}},
				{Repository: string(rs[3].Name)},
				{Repository: string(unsupported[0].Name)},
			},
			Steps: steps,
		}

		gs := newGitserverClient(
			map[api.CommitID]bool{
				defaultBranches[rs[0].Name].commit: false,
				"d34db33f":                         false,
				"c4a1":                             false,
				"c0ff33":                           false,
				"b33a":                             false,
				defaultBranches[rs[3].Name].commit: true,
				defaultBranches[unsupported[0].Name].commit: false,
			},
			map[string]api.CommitID{
				defaultBranches[rs[0].Name].branch:          defaultBranches[rs[0].Name].commit,
				"non-default-branch":                        "d34db33f",
				"a-different-non-default-branch":            "c4a1",
				"other-non-default-branch":                  "c0ff33",
				"yet-another-non-default-branch":            "b33a",
				defaultBranches[rs[3].Name].branch:          defaultBranches[rs[3].Name].commit,
				defaultBranches[unsupported[0].Name].branch: defaultBranches[unsupported[0].Name].commit,
			},
		)

		searchMatches := map[string][]streamhttp.EventMatch{
			"r:rs-2 count:all": {
				&streamhttp.EventPathMatch{
					Type:         streamhttp.PathMatchType,
					Path:         "repo-2/readme",
					RepositoryID: int32(rs[2].ID),
					Branches:     []string{"main"},
				},
			},
		}

		want := []*RepoWorkspace{
			buildRepoWorkspace(rs[0], "", "", []string{}),
			// Note that only the last rs[1] result is included.
			buildRepoWorkspace(rs[1], "a-different-non-default-branch", "c4a1", []string{}),
			// Note that this doesn't include rs[2] "main".
			buildRepoWorkspace(rs[2], "other-non-default-branch", "c0ff33", []string{}),
			buildRepoWorkspace(rs[2], "yet-another-non-default-branch", "b33a", []string{}),
			buildIgnoredRepoWorkspace(rs[3], "", "", []string{}),
			buildUnsupportedRepoWorkspace(unsupported[0], "", "", []string{}),
		}

		resolveWorkspacesAndCompare(t, s, gs, u, searchMatches, batchSpec, want)
	})

	t.Run("repositoriesMatchingQuery and repositories", func(t *testing.T) {
		batchSpec := &batcheslib.BatchSpec{
			On: []batcheslib.OnQueryOrRepository{
				{RepositoriesMatchingQuery: "repohasfile:horse.txt"},
				{Repository: string(rs[2].Name)},
				{Repository: string(rs[3].Name)},
			},
			Steps: steps,
		}

		gs := newGitserverClient(
			map[api.CommitID]bool{
				defaultBranches[rs[0].Name].commit:          false,
				defaultBranches[rs[1].Name].commit:          false,
				defaultBranches[rs[2].Name].commit:          false,
				defaultBranches[rs[3].Name].commit:          false,
				defaultBranches[unsupported[0].Name].commit: false,
			},
			map[string]api.CommitID{
				defaultBranches[rs[2].Name].branch: defaultBranches[rs[2].Name].commit,
				defaultBranches[rs[3].Name].branch: defaultBranches[rs[3].Name].commit,
			},
		)

		eventMatches := []streamhttp.EventMatch{
			&streamhttp.EventContentMatch{
				Type:         streamhttp.ContentMatchType,
				Path:         "test",
				RepositoryID: int32(rs[0].ID),
			},
			&streamhttp.EventRepoMatch{
				Type:         streamhttp.RepoMatchType,
				RepositoryID: int32(rs[1].ID),
			},
			// Included in the search results and explicitly listed
			&streamhttp.EventRepoMatch{
				Type:         streamhttp.RepoMatchType,
				RepositoryID: int32(rs[2].ID),
			},
			&streamhttp.EventRepoMatch{
				Type:         streamhttp.RepoMatchType,
				RepositoryID: int32(unsupported[0].ID),
			},
		}
		searchMatches := map[string][]streamhttp.EventMatch{
			"repohasfile:horse.txt count:all": eventMatches,
		}

		want := []*RepoWorkspace{
			buildRepoWorkspace(rs[0], "", "", []string{"test"}),
			buildRepoWorkspace(rs[1], "", "", []string{}),
			buildRepoWorkspace(rs[2], "", "", []string{}),
			buildRepoWorkspace(rs[3], "", "", []string{}),
			buildUnsupportedRepoWorkspace(unsupported[0], "", "", []string{}),
		}

		resolveWorkspacesAndCompare(t, s, gs, u, searchMatches, batchSpec, want)
	})

	t.Run("workspaces with skipped steps", func(t *testing.T) {
		conditionalSteps := []batcheslib.Step{
			// Step should only execute in rs[1]
			{Run: "echo 1", If: fmt.Sprintf(`${{ eq repository.name %q }}`, rs[1].Name)},
		}
		batchSpec := &batcheslib.BatchSpec{
			On: []batcheslib.OnQueryOrRepository{
				{Repository: string(rs[0].Name)},
				{Repository: string(rs[1].Name)},
			},
			Steps: conditionalSteps,
		}

		gs := newGitserverClient(
			map[api.CommitID]bool{
				defaultBranches[rs[0].Name].commit: false,
				defaultBranches[rs[1].Name].commit: false,
			},
			map[string]api.CommitID{
				defaultBranches[rs[0].Name].branch: defaultBranches[rs[0].Name].commit,
				defaultBranches[rs[1].Name].branch: defaultBranches[rs[1].Name].commit,
			},
		)

		ws1 := buildRepoWorkspace(rs[1], "", "", []string{})

		// ws0 has no steps to run, so it is excluded.
		// TODO: Later we might want to add an additional flag to the workspace
		// to indicate this in the UI.
		want := []*RepoWorkspace{ws1}
		resolveWorkspacesAndCompare(t, s, gs, u, map[string][]streamhttp.EventMatch{}, batchSpec, want)
	})
}

func resolveWorkspacesAndCompare(t *testing.T, s *store.Store, gs gitserver.Client, u *types.User, matches map[string][]streamhttp.EventMatch, spec *batcheslib.BatchSpec, want []*RepoWorkspace) {
	t.Helper()

	wr := &workspaceResolver{
		store:               s,
		gitserverClient:     gs,
		frontendInternalURL: newStreamSearchTestServer(t, matches),
	}
	ctx := actor.WithActor(context.Background(), actor.FromUser(u.ID))
	have, err := wr.ResolveWorkspacesForBatchSpec(ctx, spec)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatalf("returned workspaces wrong. (-want +got):\n%s", diff)
	}
}

func newStreamSearchTestServer(t *testing.T, matches map[string][]streamhttp.EventMatch) string {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		q, err := url.QueryUnescape(req.URL.Query().Get("q"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if q == "" {
			http.Error(w, "no query passed", http.StatusBadRequest)
			return
		}

		v := req.URL.Query().Get("v")
		if v != searchAPIVersion {
			http.Error(w, "wrong search api version", http.StatusBadRequest)
			return
		}

		match, ok := matches[q]
		if !ok {
			t.Logf("unknown query %q", q)
			http.Error(w, fmt.Sprintf("unknown query %q", q), http.StatusBadRequest)
			return
		}

		type ev struct {
			Name  string
			Value any
		}
		ew, err := streamhttp.NewWriter(w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		require.NoError(t, ew.Event("progress", ev{
			Name:  "progress",
			Value: &streamapi.Progress{MatchCount: len(match)},
		}))
		require.NoError(t, ew.Event("matches", match))
		require.NoError(t, ew.Event("done", struct{}{}))
	}))

	t.Cleanup(ts.Close)

	return ts.URL
}

type defaultBranch struct {
	branch string
	commit api.CommitID
}

func TestFindWorkspaces(t *testing.T) {
	repoRevs := []*RepoRevision{
		{Repo: &types.Repo{ID: 1, Name: "github.com/sourcegraph/automation-testing"}, FileMatches: []string{}},
		{Repo: &types.Repo{ID: 2, Name: "github.com/sourcegraph/sourcegraph"}, FileMatches: []string{}},
		{Repo: &types.Repo{ID: 3, Name: "bitbucket.sgdev.org/SOUR/automation-testing"}, FileMatches: []string{}},
		// This one has file matches.
		{
			Repo: &types.Repo{
				ID:   4,
				Name: "github.com/sourcegraph/src-cli",
			},
			FileMatches: []string{"a/b", "a/b/c", "d/e/f"},
		},
	}
	steps := []batcheslib.Step{{Run: "echo 1"}}

	type finderResults map[repoRevKey][]string

	tests := map[string]struct {
		spec          *batcheslib.BatchSpec
		finderResults finderResults

		// workspaces in which repo/path they are executed
		wantWorkspaces []*RepoWorkspace
		wantErr        error
	}{
		"no workspace configuration": {
			spec:          &batcheslib.BatchSpec{Steps: steps},
			finderResults: finderResults{},
			wantWorkspaces: []*RepoWorkspace{
				{RepoRevision: repoRevs[0], Path: ""},
				{RepoRevision: repoRevs[1], Path: ""},
				{RepoRevision: repoRevs[2], Path: ""},
				{RepoRevision: repoRevs[3], Path: ""},
			},
		},

		"workspace configuration matching no repos": {
			spec: &batcheslib.BatchSpec{
				Steps: steps,
				Workspaces: []batcheslib.WorkspaceConfiguration{
					{In: "this-does-not-match", RootAtLocationOf: "package.json"},
				},
			},
			finderResults: finderResults{},
			wantWorkspaces: []*RepoWorkspace{
				{RepoRevision: repoRevs[0], Path: ""},
				{RepoRevision: repoRevs[1], Path: ""},
				{RepoRevision: repoRevs[2], Path: ""},
				{RepoRevision: repoRevs[3], Path: ""},
			},
		},

		"workspace configuration matching 2 repos with no results": {
			spec: &batcheslib.BatchSpec{
				Steps: steps,
				Workspaces: []batcheslib.WorkspaceConfiguration{
					{In: "*automation-testing", RootAtLocationOf: "package.json"},
				},
			},
			finderResults: finderResults{
				repoRevs[0].Key(): []string{},
				repoRevs[2].Key(): []string{},
			},
			wantWorkspaces: []*RepoWorkspace{
				{RepoRevision: repoRevs[1], Path: ""},
				{RepoRevision: repoRevs[3], Path: ""},
			},
		},

		"workspace configuration matching 2 repos with 3 results each": {
			spec: &batcheslib.BatchSpec{
				Steps: steps,
				Workspaces: []batcheslib.WorkspaceConfiguration{
					{In: "*automation-testing", RootAtLocationOf: "package.json"},
				},
			},
			finderResults: finderResults{
				repoRevs[0].Key(): {"a/b", "a/b/c", "d/e/f"},
				repoRevs[2].Key(): {"a/b", "a/b/c", "d/e/f"},
			},
			wantWorkspaces: []*RepoWorkspace{
				{RepoRevision: repoRevs[0], Path: "a/b"},
				{RepoRevision: repoRevs[0], Path: "a/b/c"},
				{RepoRevision: repoRevs[0], Path: "d/e/f"},
				{RepoRevision: repoRevs[1], Path: ""},
				{RepoRevision: repoRevs[2], Path: "a/b"},
				{RepoRevision: repoRevs[2], Path: "a/b/c"},
				{RepoRevision: repoRevs[2], Path: "d/e/f"},
				{RepoRevision: repoRevs[3], Path: ""},
			},
		},

		"workspace configuration matches repo with OnlyFetchWorkspace": {
			spec: &batcheslib.BatchSpec{
				Steps: steps,
				Workspaces: []batcheslib.WorkspaceConfiguration{
					{
						OnlyFetchWorkspace: true,
						In:                 "*automation-testing",
						RootAtLocationOf:   "package.json",
					},
				},
			},
			finderResults: finderResults{
				repoRevs[0].Key(): {"a/b", "a/b/c", "d/e/f"},
				repoRevs[2].Key(): {"a/b", "a/b/c", "d/e/f"},
			},
			wantWorkspaces: []*RepoWorkspace{
				{RepoRevision: repoRevs[0], Path: "a/b", OnlyFetchWorkspace: true},
				{RepoRevision: repoRevs[0], Path: "a/b/c", OnlyFetchWorkspace: true},
				{RepoRevision: repoRevs[0], Path: "d/e/f", OnlyFetchWorkspace: true},
				{RepoRevision: repoRevs[1], Path: ""},
				{RepoRevision: repoRevs[2], Path: "a/b", OnlyFetchWorkspace: true},
				{RepoRevision: repoRevs[2], Path: "a/b/c", OnlyFetchWorkspace: true},
				{RepoRevision: repoRevs[2], Path: "d/e/f", OnlyFetchWorkspace: true},
				{RepoRevision: repoRevs[3], Path: ""},
			},
		},

		"workspace configuration without 'in' matches all": {
			spec: &batcheslib.BatchSpec{
				Steps: steps,
				Workspaces: []batcheslib.WorkspaceConfiguration{
					{
						RootAtLocationOf: "package.json",
					},
				},
			},
			finderResults: finderResults{
				repoRevs[0].Key(): {"a/b"},
				repoRevs[2].Key(): {"a/b"},
			},
			wantWorkspaces: []*RepoWorkspace{
				{RepoRevision: repoRevs[0], Path: "a/b"},
				{RepoRevision: repoRevs[2], Path: "a/b"},
			},
		},
		"workspace configuration matching two repos": {
			spec: &batcheslib.BatchSpec{
				Steps: steps,
				Workspaces: []batcheslib.WorkspaceConfiguration{
					{
						RootAtLocationOf: "package.json",
						In:               string(repoRevs[0].Repo.Name),
					},
					{
						RootAtLocationOf: "go.mod",
						In:               string(repoRevs[0].Repo.Name),
					},
				},
			},
			finderResults: finderResults{
				repoRevs[0].Key(): {"a/b"},
			},
			wantErr: errors.New(`repository github.com/sourcegraph/automation-testing matches multiple workspaces.in globs in the batch spec. glob: "github.com/sourcegraph/automation-testing"`),
		},
		"workspace gets subset of search_result_paths": {
			spec: &batcheslib.BatchSpec{
				Steps: steps,
				Workspaces: []batcheslib.WorkspaceConfiguration{
					{
						In:               "*src-cli",
						RootAtLocationOf: "package.json",
					},
				},
			},
			finderResults: finderResults{
				repoRevs[3].Key(): {"a/b", "d"},
			},
			wantWorkspaces: []*RepoWorkspace{
				{RepoRevision: repoRevs[0], Path: ""},
				{RepoRevision: repoRevs[1], Path: ""},
				{RepoRevision: repoRevs[2], Path: ""},
				{RepoRevision: &RepoRevision{Repo: repoRevs[3].Repo, Branch: repoRevs[3].Branch, Commit: repoRevs[3].Commit, FileMatches: []string{"a/b", "a/b/c"}}, Path: "a/b"},
				{RepoRevision: &RepoRevision{Repo: repoRevs[3].Repo, Branch: repoRevs[3].Branch, Commit: repoRevs[3].Commit, FileMatches: []string{"d/e/f"}}, Path: "d"},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			finder := &mockDirectoryFinder{results: tt.finderResults}
			workspaces, err := findWorkspaces(context.Background(), tt.spec, finder, repoRevs)
			if err != nil {
				if tt.wantErr != nil {
					require.Exactly(t, tt.wantErr.Error(), err.Error(), "wrong error returned")
				} else {
					t.Fatalf("unexpected err: %s", err)
				}
			}

			// Sort by ID, easier than by name for tests.
			sort.Slice(workspaces, func(i, j int) bool {
				if workspaces[i].Repo.ID == workspaces[j].Repo.ID {
					return workspaces[i].Path < workspaces[j].Path
				}
				return workspaces[i].Repo.ID < workspaces[j].Repo.ID
			})

			if diff := cmp.Diff(tt.wantWorkspaces, workspaces); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type mockDirectoryFinder struct {
	results map[repoRevKey][]string
}

func (m *mockDirectoryFinder) FindDirectoriesInRepos(ctx context.Context, fileName string, repos ...*RepoRevision) (map[repoRevKey][]string, error) {
	return m.results, nil
}
