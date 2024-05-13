package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"

	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/zoekt"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func buildCursor(node *types.Repo) *string {
	cursor := MarshalRepositoryCursor(
		&types.Cursor{
			Column: "name",
			Value:  fmt.Sprintf("%s@%d", node.Name, node.ID),
		},
	)

	return &cursor
}

func buildCursorBySize(node *types.Repo, size int64) *string {
	cursor := MarshalRepositoryCursor(
		&types.Cursor{
			Column: "gr.repo_size_bytes",
			Value:  fmt.Sprintf("%d@%d", size, node.ID),
		},
	)

	return &cursor
}

func TestRepositoriesSourceType(t *testing.T) {
	r1 := types.Repo{
		ID:           1,
		Name:         "repo1",
		ExternalRepo: api.ExternalRepoSpec{ServiceType: extsvc.TypeGitHub},
	}
	r2 := types.Repo{
		ID:           2,
		Name:         "repo2",
		ExternalRepo: api.ExternalRepoSpec{ServiceType: extsvc.TypePerforce},
	}

	repos := dbmocks.NewMockRepoStore()
	repos.ListFunc.SetDefaultReturn([]*types.Repo{&r1, &r2}, nil)
	repos.GetFunc.SetDefaultHook(func(ctx context.Context, repoID api.RepoID) (*types.Repo, error) {
		if repoID == 1 {
			return &r1, nil
		}

		return &r2, nil
	})

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				{
					repositories(first: 10) {
						nodes {
						  name
						  sourceType
						}
					}
				}
			`,
			ExpectedResult: `
				{
				  "repositories": {
					"nodes": [
					  {
						"name": "repo1",
						"sourceType": "GIT_REPOSITORY"
					  },
					  {
						"name": "repo2",
						"sourceType": "PERFORCE_DEPOT"
					  }
					]
				  }
				}
			`,
		},
	})
}

func TestRepositoriesCloneStatusFiltering(t *testing.T) {
	mockRepos := []*types.Repo{
		{ID: 1, Name: "repo1"}, // not_cloned
		{ID: 2, Name: "repo2"}, // cloning
		{ID: 3, Name: "repo3"}, // cloned
	}

	repos := dbmocks.NewMockRepoStore()
	repos.ListFunc.SetDefaultHook(func(ctx context.Context, opt database.ReposListOptions) ([]*types.Repo, error) {
		if opt.NoCloned {
			return mockRepos[0:2], nil
		}
		if opt.OnlyCloned {
			return mockRepos[2:], nil
		}

		if opt.CloneStatus == types.CloneStatusNotCloned {
			return mockRepos[:1], nil
		}
		if opt.CloneStatus == types.CloneStatusCloning {
			return mockRepos[1:2], nil
		}
		if opt.CloneStatus == types.CloneStatusCloned {
			return mockRepos[2:], nil
		}

		return mockRepos, nil
	})
	repos.CountFunc.SetDefaultReturn(len(mockRepos), nil)

	users := dbmocks.NewMockUserStore()

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)
	db.UsersFunc.SetDefaultReturn(users)

	schema := mustParseGraphQLSchema(t, db)

	t.Run("not as a site admin", func(t *testing.T) {
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1}, nil)

		RunTests(t, []*Test{
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 3) {
						nodes { name }
						totalCount
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo1" },
							{ "name": "repo2" },
							{ "name": "repo3" }
						],
						"totalCount": 0,
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
			},
		})
	})

	t.Run("as a site admin", func(t *testing.T) {
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

		RunTests(t, []*Test{
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 3) {
						nodes { name }
						totalCount
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo1" },
							{ "name": "repo2" },
							{ "name": "repo3" }
						],
						"totalCount": 3,
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
			},
			{
				Schema: schema,
				// cloned and notCloned are true by default
				// this test ensures the behavior is the same
				// when setting them explicitly
				Query: `
				{
					repositories(first: 3, cloned: true, notCloned: true) {
						nodes { name }
						totalCount
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo1" },
							{ "name": "repo2" },
							{ "name": "repo3" }
						],
						"totalCount": 3,
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
			},
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 2) {
						nodes { name }
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo1" },
							{ "name": "repo2" }
						],
						"pageInfo": {"hasNextPage": true}
					}
				}
			`,
			},
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 3, cloned: false) {
						nodes { name }
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo1" },
							{ "name": "repo2" }
						],
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
			},
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 3, notCloned: false) {
						nodes { name }
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo3" }
						],
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
			},
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 3, notCloned: false, cloned: false) {
						nodes { name }
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: "null",
				ExpectedErrors: []*gqlerrors.QueryError{
					{
						Path:          []any{"repositories"},
						Message:       "excluding cloned and not cloned repos leaves an empty set",
						ResolverError: errors.New("excluding cloned and not cloned repos leaves an empty set"),
					},
				},
			},
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 3, cloneStatus: CLONED) {
						nodes { name }
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo3" }
						],
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
			},
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 3, cloneStatus: CLONING) {
						nodes { name }
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo2" }
						],
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
			},
			{
				Schema: schema,
				Query: `
				{
					repositories(first: 3, cloneStatus: NOT_CLONED) {
						nodes { name }
						pageInfo { hasNextPage }
					}
				}
			`,
				ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo1" }
						],
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
			},
		})
	})
}

func TestRepositoriesIndexingFiltering(t *testing.T) {
	mockRepos := map[string]bool{
		"repo-indexed-1":     true,
		"repo-indexed-2":     true,
		"repo-not-indexed-3": false,
		"repo-not-indexed-4": false,
	}

	filterRepos := func(t *testing.T, opt database.ReposListOptions) []*types.Repo {
		t.Helper()
		var repos types.Repos
		for n, idx := range mockRepos {
			if opt.NoIndexed && idx {
				continue
			}
			if opt.OnlyIndexed && !idx {
				continue
			}
			repos = append(repos, &types.Repo{Name: api.RepoName(n)})
		}
		sort.Sort(repos)
		return repos
	}
	repos := dbmocks.NewMockRepoStore()
	repos.ListFunc.SetDefaultHook(func(_ context.Context, opt database.ReposListOptions) ([]*types.Repo, error) {
		return filterRepos(t, opt), nil
	})
	repos.CountFunc.SetDefaultHook(func(_ context.Context, opt database.ReposListOptions) (int, error) {
		repos := filterRepos(t, opt)
		return len(repos), nil
	})

	users := dbmocks.NewMockUserStore()

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)
	db.UsersFunc.SetDefaultReturn(users)

	schema := mustParseGraphQLSchema(t, db)

	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

	RunTests(t, []*Test{
		{
			Schema: schema,
			Query: `
				{
					repositories(first: 5) {
						nodes { name }
						totalCount
						pageInfo { hasNextPage }
					}
				}
			`,
			ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo-indexed-1" },
							{ "name": "repo-indexed-2" },
							{ "name": "repo-not-indexed-3" },
							{ "name": "repo-not-indexed-4" }
						],
						"totalCount": 4,
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
		},
		{
			Schema: schema,
			// indexed and notIndexed are true by default
			// this test ensures the behavior is the same
			// when setting them explicitly
			Query: `
				{
					repositories(first: 5, indexed: true, notIndexed: true) {
						nodes { name }
						totalCount
						pageInfo { hasNextPage }
					}
				}
			`,
			ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo-indexed-1" },
							{ "name": "repo-indexed-2" },
							{ "name": "repo-not-indexed-3" },
							{ "name": "repo-not-indexed-4" }
						],
						"totalCount": 4,
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
		},
		{
			Schema: schema,
			Query: `
				{
					repositories(first: 5, indexed: false) {
						nodes { name }
						totalCount
						pageInfo { hasNextPage }
					}
				}
			`,
			ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo-not-indexed-3" },
							{ "name": "repo-not-indexed-4" }
						],
						"totalCount": 2,
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
		},
		{
			Schema: schema,
			Query: `
				{
					repositories(first: 5, notIndexed: false) {
						nodes { name }
						totalCount
						pageInfo { hasNextPage }
					}
				}
			`,
			ExpectedResult: `
				{
					"repositories": {
						"nodes": [
							{ "name": "repo-indexed-1" },
							{ "name": "repo-indexed-2" }
						],
						"totalCount": 2,
						"pageInfo": {"hasNextPage": false}
					}
				}
			`,
		},
		{
			Schema: schema,
			Query: `
				{
					repositories(first: 5, notIndexed: false, indexed: false) {
						nodes { name }
						totalCount
						pageInfo { hasNextPage }
					}
				}
			`,
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Path:          []any{"repositories"},
					Message:       "excluding indexed and not indexed repos leaves an empty set",
					ResolverError: errors.New("excluding indexed and not indexed repos leaves an empty set"),
				},
			},
		},
	})
}

func TestRepositories_CursorPagination(t *testing.T) {
	mockRepos := []*types.Repo{
		{ID: 0, Name: "repo1"},
		{ID: 1, Name: "repo2"},
		{ID: 2, Name: "repo3"},
	}

	repos := dbmocks.NewMockRepoStore()
	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)

	buildQuery := func(first int, after string) string {
		var args []string
		if first != 0 {
			args = append(args, fmt.Sprintf("first: %d", first))
		}
		if after != "" {
			args = append(args, fmt.Sprintf("after: %q", after))
		}

		return fmt.Sprintf(`{ repositories(%s) { nodes { name } pageInfo { endCursor } } }`, strings.Join(args, ", "))
	}

	t.Run("Initial page without a cursor present", func(t *testing.T) {
		repos.ListFunc.SetDefaultReturn(mockRepos[0:2], nil)

		RunTest(t, &Test{
			Schema: mustParseGraphQLSchema(t, db),
			Query:  buildQuery(1, ""),
			ExpectedResult: fmt.Sprintf(`
				{
					"repositories": {
						"nodes": [{
							"name": "repo1"
						}],
						"pageInfo": {
						  "endCursor": "%s"
						}
					}
				}
			`, *buildCursor(mockRepos[0])),
		})
	})

	t.Run("Second page in ascending order", func(t *testing.T) {
		repos.ListFunc.SetDefaultReturn(mockRepos[1:], nil)

		RunTest(t, &Test{
			Schema: mustParseGraphQLSchema(t, db),
			Query:  buildQuery(1, *buildCursor(mockRepos[0])),
			ExpectedResult: fmt.Sprintf(`
				{
					"repositories": {
						"nodes": [{
							"name": "repo2"
						}],
						"pageInfo": {
						  "endCursor": "%s"
						}
					}
				}
			`, *buildCursor(mockRepos[1])),
		})
	})

	t.Run("Second page in descending order", func(t *testing.T) {
		repos.ListFunc.SetDefaultReturn(mockRepos[1:], nil)

		RunTest(t, &Test{
			Schema: mustParseGraphQLSchema(t, db),
			Query:  buildQuery(1, *buildCursor(mockRepos[0])),
			ExpectedResult: fmt.Sprintf(`
				{
					"repositories": {
						"nodes": [{
							"name": "repo2"
						}],
						"pageInfo": {
						  "endCursor": "%s"
						}
					}
				}
			`, *buildCursor(mockRepos[1])),
		})
	})

	t.Run("Initial page with no further rows to fetch", func(t *testing.T) {
		repos.ListFunc.SetDefaultReturn(mockRepos, nil)

		RunTest(t, &Test{
			Schema: mustParseGraphQLSchema(t, db),
			Query:  buildQuery(3, ""),
			ExpectedResult: fmt.Sprintf(`
				{
					"repositories": {
						"nodes": [{
							"name": "repo1"
						}, {
							"name": "repo2"
						}, {
							"name": "repo3"
						}],
						"pageInfo": {
						  "endCursor": "%s"
						}
					}
				}
			`, *buildCursor(mockRepos[2])),
		})
	})

	t.Run("With no repositories present", func(t *testing.T) {
		repos.ListFunc.SetDefaultReturn(nil, nil)

		RunTest(t, &Test{
			Schema: mustParseGraphQLSchema(t, db),
			Query:  buildQuery(1, ""),
			ExpectedResult: `
				{
					"repositories": {
						"nodes": [],
						"pageInfo": {
						  "endCursor": null
						}
					}
				}
			`,
		})
	})

	t.Run("With an invalid cursor provided", func(t *testing.T) {
		repos.ListFunc.SetDefaultReturn(nil, nil)

		RunTest(t, &Test{
			Schema:         mustParseGraphQLSchema(t, db),
			Query:          buildQuery(1, "invalid-cursor-value"),
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Path:          []any{"repositories", "nodes"},
					Message:       `cannot unmarshal repository cursor type: ""`,
					ResolverError: errors.New(`cannot unmarshal repository cursor type: ""`),
				},
				{
					Path:          []any{"repositories", "pageInfo"},
					Message:       `cannot unmarshal repository cursor type: ""`,
					ResolverError: errors.New(`cannot unmarshal repository cursor type: ""`),
				},
			},
		})
	})
}

func TestRepositories_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	schema := mustParseGraphQLSchema(t, db)

	repos := []struct {
		repo        *types.Repo
		size        int64
		cloneStatus types.CloneStatus
		indexed     bool
		lastError   string
	}{
		{repo: &types.Repo{Name: "repo0"}, size: 20, cloneStatus: types.CloneStatusNotCloned},
		{repo: &types.Repo{Name: "repo1"}, size: 30, cloneStatus: types.CloneStatusNotCloned, lastError: "repo1 error"},
		{repo: &types.Repo{Name: "repo2"}, size: 40, cloneStatus: types.CloneStatusCloning},
		{repo: &types.Repo{Name: "repo3"}, size: 50, cloneStatus: types.CloneStatusCloning, lastError: "repo3 error"},
		{repo: &types.Repo{Name: "repo4"}, size: 60, cloneStatus: types.CloneStatusCloned},
		{repo: &types.Repo{Name: "repo5"}, size: 10, cloneStatus: types.CloneStatusCloned, lastError: "repo5 error"},
		{repo: &types.Repo{Name: "repo6"}, size: 70, cloneStatus: types.CloneStatusCloned, indexed: false},
		{repo: &types.Repo{Name: "repo7"}, size: 80, cloneStatus: types.CloneStatusCloned, indexed: true},
	}

	for _, rsc := range repos {
		if err := db.Repos().Create(ctx, rsc.repo); err != nil {
			t.Fatal(err)
		}

		gitserverRepos := db.GitserverRepos()
		if err := gitserverRepos.SetRepoSize(ctx, rsc.repo.Name, rsc.size, "shard-1"); err != nil {
			t.Fatal(err)
		}
		switch rsc.cloneStatus {
		case types.CloneStatusCloned:
			if err := gitserverRepos.SetCloned(ctx, rsc.repo.Name, "shard-1"); err != nil {
				t.Fatal(err)
			}
		case types.CloneStatusCloning:
			if err := gitserverRepos.SetCloning(ctx, rsc.repo.Name, "shard-1"); err != nil {
				t.Fatal(err)
			}
		case types.CloneStatusNotCloned:
			if err := gitserverRepos.SetNotCloned(ctx, rsc.repo.Name); err != nil {
				t.Fatal(err)
			}
		}

		if rsc.indexed {
			err := db.ZoektRepos().UpdateIndexStatuses(ctx, zoekt.ReposMap{
				uint32(rsc.repo.ID): {},
			})
			if err != nil {
				t.Fatal(err)
			}
		}

		if msg := rsc.lastError; msg != "" {
			if err := gitserverRepos.SetLastError(ctx, rsc.repo.Name, msg, "shard-1"); err != nil {
				t.Fatal(err)
			}
		}
	}

	admin, err := db.Users().Create(ctx, database.NewUser{Username: "admin", Password: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	ctx = actor.WithActor(ctx, actor.FromUser(admin.ID))

	tests := []repositoriesQueryTest{
		// first
		{
			args:             "first: 2",
			wantRepos:        []string{"repo0", "repo1"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[1].repo),
		},
		// second page with first, after args
		{
			args:             fmt.Sprintf(`first: 2, after: "%s"`, *buildCursor(repos[0].repo)),
			wantRepos:        []string{"repo1", "repo2"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: true,
			wantStartCursor:  buildCursor(repos[1].repo),
			wantEndCursor:    buildCursor(repos[2].repo),
		},
		// last page with first, after args
		{
			args:             fmt.Sprintf(`first: 2, after: "%s"`, *buildCursor(repos[5].repo)),
			wantRepos:        []string{"repo6", "repo7"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: true,
			wantStartCursor:  buildCursor(repos[6].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		// last
		{
			args:             "last: 2",
			wantRepos:        []string{"repo6", "repo7"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: true,
			wantStartCursor:  buildCursor(repos[6].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		// second last page with last, before args
		{
			args:             fmt.Sprintf(`last: 2, before: "%s"`, *buildCursor(repos[6].repo)),
			wantRepos:        []string{"repo4", "repo5"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: true,
			wantStartCursor:  buildCursor(repos[4].repo),
			wantEndCursor:    buildCursor(repos[5].repo),
		},
		// back to first page with last, before args
		{
			args:             fmt.Sprintf(`last: 2, before: "%s"`, *buildCursor(repos[2].repo)),
			wantRepos:        []string{"repo0", "repo1"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[1].repo),
		},
		// descending first
		{
			args:             "first: 2, descending: true",
			wantRepos:        []string{"repo7", "repo6"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[7].repo),
			wantEndCursor:    buildCursor(repos[6].repo),
		},
		// descending second page with first, after args
		{
			args:             fmt.Sprintf(`first: 2, descending: true, after: "%s"`, *buildCursor(repos[6].repo)),
			wantRepos:        []string{"repo5", "repo4"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: true,
			wantStartCursor:  buildCursor(repos[5].repo),
			wantEndCursor:    buildCursor(repos[4].repo),
		},
		// descending last page with first, after args
		{
			args:             fmt.Sprintf(`first: 2, descending: true, after: "%s"`, *buildCursor(repos[2].repo)),
			wantRepos:        []string{"repo1", "repo0"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: true,
			wantStartCursor:  buildCursor(repos[1].repo),
			wantEndCursor:    buildCursor(repos[0].repo),
		},
		// descending last
		{
			args:             "last: 2, descending: true",
			wantRepos:        []string{"repo1", "repo0"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: true,
			wantStartCursor:  buildCursor(repos[1].repo),
			wantEndCursor:    buildCursor(repos[0].repo),
		},
		// descending second last page with last, before args
		{
			args:             fmt.Sprintf(`last: 2, descending: true, before: "%s"`, *buildCursor(repos[3].repo)),
			wantRepos:        []string{"repo5", "repo4"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: true,
			wantStartCursor:  buildCursor(repos[5].repo),
			wantEndCursor:    buildCursor(repos[4].repo),
		},
		// descending back to first page with last, before args
		{
			args:             fmt.Sprintf(`last: 2, descending: true, before: "%s"`, *buildCursor(repos[5].repo)),
			wantRepos:        []string{"repo7", "repo6"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[7].repo),
			wantEndCursor:    buildCursor(repos[6].repo),
		},
		// cloned
		{
			// cloned only says whether to "Include cloned repositories.", it doesn't exclude non-cloned.
			args:             "first: 10, cloned: true",
			wantRepos:        []string{"repo0", "repo1", "repo2", "repo3", "repo4", "repo5", "repo6", "repo7"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		{
			args:             "first: 10, cloned: false",
			wantRepos:        []string{"repo0", "repo1", "repo2", "repo3"},
			wantTotalCount:   4,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[3].repo),
		},
		{
			args:             "cloned: false, first: 2",
			wantRepos:        []string{"repo0", "repo1"},
			wantTotalCount:   4,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[1].repo),
		},
		// notCloned
		{
			args:             "first: 10, notCloned: true",
			wantRepos:        []string{"repo0", "repo1", "repo2", "repo3", "repo4", "repo5", "repo6", "repo7"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		{
			args:             "first: 10, notCloned: false",
			wantRepos:        []string{"repo4", "repo5", "repo6", "repo7"},
			wantTotalCount:   4,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[4].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		// failedFetch
		{
			args:             "first: 10, failedFetch: true",
			wantRepos:        []string{"repo1", "repo3", "repo5"},
			wantTotalCount:   3,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[1].repo),
			wantEndCursor:    buildCursor(repos[5].repo),
		},
		{
			args:             "failedFetch: true, first: 2",
			wantRepos:        []string{"repo1", "repo3"},
			wantTotalCount:   3,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[1].repo),
			wantEndCursor:    buildCursor(repos[3].repo),
		},
		{
			args:             "first: 10, failedFetch: false",
			wantRepos:        []string{"repo0", "repo1", "repo2", "repo3", "repo4", "repo5", "repo6", "repo7"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		// cloneStatus
		{
			args:             "first: 10, cloneStatus:NOT_CLONED",
			wantRepos:        []string{"repo0", "repo1"},
			wantTotalCount:   2,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[1].repo),
		},
		{
			args:             "first: 10, cloneStatus:CLONING",
			wantRepos:        []string{"repo2", "repo3"},
			wantTotalCount:   2,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[2].repo),
			wantEndCursor:    buildCursor(repos[3].repo),
		},
		{
			args:             "first: 10, cloneStatus:CLONED",
			wantRepos:        []string{"repo4", "repo5", "repo6", "repo7"},
			wantTotalCount:   4,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[4].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		{
			args:             "cloneStatus:NOT_CLONED, first: 1",
			wantRepos:        []string{"repo0"},
			wantTotalCount:   2,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[0].repo),
		},
		// indexed
		{
			// indexed only says whether to "Include indexed repositories.", it doesn't exclude non-indexed.
			args:             "first: 10, indexed: true",
			wantRepos:        []string{"repo0", "repo1", "repo2", "repo3", "repo4", "repo5", "repo6", "repo7"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		{
			args:             "first: 10, indexed: false",
			wantRepos:        []string{"repo0", "repo1", "repo2", "repo3", "repo4", "repo5", "repo6"},
			wantTotalCount:   7,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[6].repo),
		},
		{
			args:             "indexed: false, first: 2",
			wantRepos:        []string{"repo0", "repo1"},
			wantTotalCount:   7,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[1].repo),
		},
		// notIndexed
		{
			args:             "first: 10, notIndexed: true",
			wantRepos:        []string{"repo0", "repo1", "repo2", "repo3", "repo4", "repo5", "repo6", "repo7"},
			wantTotalCount:   8,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[0].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		{
			args:             "first: 10, notIndexed: false",
			wantRepos:        []string{"repo7"},
			wantTotalCount:   1,
			wantNextPage:     false,
			wantPreviousPage: false,
			wantStartCursor:  buildCursor(repos[7].repo),
			wantEndCursor:    buildCursor(repos[7].repo),
		},
		{
			args:             "orderBy:SIZE, descending:false, first: 5",
			wantRepos:        []string{"repo5", "repo0", "repo1", "repo2", "repo3"},
			wantTotalCount:   8,
			wantNextPage:     true,
			wantPreviousPage: false,
			wantStartCursor:  buildCursorBySize(repos[5].repo, repos[5].size),
			wantEndCursor:    buildCursorBySize(repos[3].repo, repos[3].size),
		},
	}

	for _, tt := range tests {
		t.Run(tt.args, func(t *testing.T) {
			runRepositoriesQuery(t, ctx, schema, tt)
		})
	}
}

type repositoriesQueryTest struct {
	args             string
	wantRepos        []string
	wantTotalCount   int
	wantEndCursor    *string
	wantStartCursor  *string
	wantNextPage     bool
	wantPreviousPage bool
}

func runRepositoriesQuery(t *testing.T, ctx context.Context, schema *graphql.Schema, want repositoriesQueryTest) {
	t.Helper()

	type node struct {
		Name string `json:"name"`
	}

	type pageInfo struct {
		HasNextPage     bool    `json:"hasNextPage"`
		HasPreviousPage bool    `json:"hasPreviousPage"`
		StartCursor     *string `json:"startCursor"`
		EndCursor       *string `json:"endCursor"`
	}

	type repositories struct {
		Nodes      []node   `json:"nodes"`
		TotalCount *int     `json:"totalCount"`
		PageInfo   pageInfo `json:"pageInfo"`
	}

	type expected struct {
		Repositories repositories `json:"repositories"`
	}

	nodes := make([]node, 0, len(want.wantRepos))
	for _, name := range want.wantRepos {
		nodes = append(nodes, node{Name: name})
	}

	ex := expected{
		Repositories: repositories{
			Nodes:      nodes,
			TotalCount: &want.wantTotalCount,
			PageInfo: pageInfo{
				HasNextPage:     want.wantNextPage,
				HasPreviousPage: want.wantPreviousPage,
				StartCursor:     want.wantStartCursor,
				EndCursor:       want.wantEndCursor,
			},
		},
	}

	marshaled, err := json.Marshal(ex)
	if err != nil {
		t.Fatalf("failed to marshal expected repositories query result: %s", err)
	}

	query := fmt.Sprintf(`
	{
		repositories(%s) {
			nodes {
				name
			}
			totalCount
			pageInfo {
				hasNextPage
				hasPreviousPage
				startCursor
				endCursor
			}
		}
	}`, want.args)

	RunTest(t, &Test{
		Context:        ctx,
		Schema:         schema,
		Query:          query,
		ExpectedResult: string(marshaled),
	})
}
