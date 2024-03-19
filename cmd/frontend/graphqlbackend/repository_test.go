package graphqlbackend

import (
	"context"
	"fmt"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/log/logtest"
)

const exampleCommitSHA1 = "1234567890123456789012345678901234567890"

func TestRepository_Commit(t *testing.T) {
	backend.Mocks.Repos.ResolveRev = func(ctx context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		assert.Equal(t, "abc", rev)
		return exampleCommitSHA1, nil
	}
	backend.Mocks.Repos.MockGetCommit_Return_NoCheck(t, &gitdomain.Commit{ID: exampleCommitSHA1})
	defer func() {
		backend.Mocks = backend.MockServices{}
	}()

	repos := dbmocks.NewMockRepoStore()
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: 2, Name: "github.com/gorilla/mux"}, nil)

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)

	RunTest(t, &Test{
		Schema: mustParseGraphQLSchema(t, db),
		Query: `
				{
					repository(name: "github.com/gorilla/mux") {
						commit(rev: "abc") {
							oid
						}
					}
				}
			`,
		ExpectedResult: `
				{
					"repository": {
						"commit": {
							"oid": "` + exampleCommitSHA1 + `"
						}
					}
				}
			`,
	})
}

func TestRepository_Changelist(t *testing.T) {
	repo := &types.Repo{ID: 2, Name: "github.com/gorilla/mux"}

	backend.Mocks.Repos.ResolveRev = func(ctx context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		assert.Equal(t, api.RepoName("github.com/gorilla/mux"), repo)
		return exampleCommitSHA1, nil
	}

	repos := dbmocks.NewMockRepoStore()
	repos.GetFunc.SetDefaultReturn(repo, nil)
	repos.GetByNameFunc.SetDefaultReturn(repo, nil)

	repoCommitsChangelists := dbmocks.NewMockRepoCommitsChangelistsStore()
	repoCommitsChangelists.GetRepoCommitChangelistFunc.SetDefaultReturn(&types.RepoCommit{
		ID:                   1,
		RepoID:               2,
		CommitSHA:            dbutil.CommitBytea(exampleCommitSHA1),
		PerforceChangelistID: 123,
	}, nil)

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)
	db.RepoCommitsChangelistsFunc.SetDefaultReturn(repoCommitsChangelists)

	RunTest(t, &Test{
		Schema: mustParseGraphQLSchema(t, db),
		Query: `
				{
					repository(name: "github.com/gorilla/mux") {
						changelist(cid: "123") {
							cid
							canonicalURL
							commit {
								oid
							}
						}
					}
				}
			`,
		ExpectedResult: fmt.Sprintf(`
				{
					"repository": {
						"changelist": {
							"cid": "123",
							"canonicalURL": "/github.com/gorilla/mux/-/changelist/123",
"commit": {
	"oid": "%s"
}
						}
					}
				}
			`, exampleCommitSHA1),
	})
}

func TestRepositoryHydration(t *testing.T) {
	t.Parallel()

	makeRepos := func() (*types.Repo, *types.Repo) {
		const id = 42
		name := fmt.Sprintf("repo-%d", id)

		minimal := types.Repo{
			ID:   api.RepoID(id),
			Name: api.RepoName(name),
		}

		hydrated := minimal
		hydrated.ExternalRepo = api.ExternalRepoSpec{
			ID:          name,
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com",
		}
		hydrated.URI = fmt.Sprintf("github.com/foobar/%s", name)
		hydrated.Description = "This is a description of a repository"
		hydrated.Fork = false

		return &minimal, &hydrated
	}

	ctx := context.Background()

	t.Run("hydrated without errors", func(t *testing.T) {
		minimalRepo, hydratedRepo := makeRepos()

		rs := dbmocks.NewMockRepoStore()
		rs.GetFunc.SetDefaultReturn(hydratedRepo, nil)
		db := dbmocks.NewMockDB()
		db.ReposFunc.SetDefaultReturn(rs)

		repoResolver := NewRepositoryResolver(db, gitserver.NewTestClient(t), minimalRepo)
		assertRepoResolverHydrated(ctx, t, repoResolver, hydratedRepo)
		mockrequire.CalledOnce(t, rs.GetFunc)
	})

	t.Run("hydration results in errors", func(t *testing.T) {
		minimalRepo, _ := makeRepos()

		dbErr := errors.New("cannot load repo")

		rs := dbmocks.NewMockRepoStore()
		rs.GetFunc.SetDefaultReturn(nil, dbErr)
		db := dbmocks.NewMockDB()
		db.ReposFunc.SetDefaultReturn(rs)

		repoResolver := NewRepositoryResolver(db, gitserver.NewTestClient(t), minimalRepo)
		_, err := repoResolver.Description(ctx)
		require.ErrorIs(t, err, dbErr)

		// Another call to make sure err does not disappear
		_, err = repoResolver.URI(ctx)
		require.ErrorIs(t, err, dbErr)

		mockrequire.CalledOnce(t, rs.GetFunc)
	})
}

func assertRepoResolverHydrated(ctx context.Context, t *testing.T, r *RepositoryResolver, hydrated *types.Repo) {
	t.Helper()

	description, err := r.Description(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if description != hydrated.Description {
		t.Fatalf("wrong Description. want=%q, have=%q", hydrated.Description, description)
	}

	uri, err := r.URI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if uri != hydrated.URI {
		t.Fatalf("wrong URI. want=%q, have=%q", hydrated.URI, uri)
	}
}

func TestRepositoryLabel(t *testing.T) {
	test := func(name string) string {
		r := &RepositoryResolver{
			name:   api.RepoName(name),
			logger: logtest.Scoped(t),
		}
		markdown, _ := r.Label()
		html, err := markdown.HTML()
		if err != nil {
			t.Fatal(err)
		}
		return html
	}

	autogold.Expect(`<p><a href="/repo%20with%20spaces" rel="nofollow">repo with spaces</a></p>
`).Equal(t, test("repo with spaces"))
}

func TestRepository_DefaultBranch(t *testing.T) {
	ctx := context.Background()
	ts := []struct {
		name                    string
		getDefaultBranchRefName string
		getDefaultBranchErr     error
		wantBranch              *GitRefResolver
		wantErr                 error
	}{
		{
			name:                    "ref exists",
			getDefaultBranchRefName: "refs/heads/main",
			wantBranch:              &GitRefResolver{name: "refs/heads/main"},
		},
		{
			// When clone is in progress GetDefaultBranch returns "", nil
			name: "clone in progress",
			// Expect it to not fail and not return a resolver.
			wantBranch: nil,
			wantErr:    nil,
		},
		{
			name:                "symbolic ref fails",
			getDefaultBranchErr: errors.New("bad git error"),
			wantErr:             errors.New("bad git error"),
		},
	}
	for _, tt := range ts {
		t.Run(tt.name, func(t *testing.T) {
			gsClient := gitserver.NewMockClient()
			gsClient.GetDefaultBranchFunc.SetDefaultReturn(tt.getDefaultBranchRefName, "", tt.getDefaultBranchErr)

			res := &RepositoryResolver{name: "repo", logger: logtest.Scoped(t), gitserverClient: gsClient}
			branch, err := res.DefaultBranch(ctx)
			if tt.wantErr != nil && err != nil {
				if tt.wantErr.Error() != err.Error() {
					t.Fatalf("incorrect error message, want=%q have=%q", tt.wantErr.Error(), err.Error())
				}
			} else if tt.wantErr != err {
				t.Fatalf("incorrect error, want=%v have=%v", tt.wantErr, err)
			}
			if branch == nil && tt.wantBranch != nil {
				t.Fatal("invalid nil resolver returned")
			}
			if branch != nil && tt.wantBranch == nil {
				t.Fatalf("expected nil resolver but got %q", branch.name)
			}
			if tt.wantBranch != nil && branch.name != tt.wantBranch.name {
				t.Fatalf("wrong resolver returned, want=%q have=%q", branch.name, tt.wantBranch.name)
			}
		})
	}
}
