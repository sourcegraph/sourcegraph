package resolvers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/own/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/fakedb"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/own"
	"github.com/sourcegraph/sourcegraph/internal/own/codeowners"
	codeownerspb "github.com/sourcegraph/sourcegraph/internal/own/codeowners/v1"
	owntypes "github.com/sourcegraph/sourcegraph/internal/own/types"
	rbactypes "github.com/sourcegraph/sourcegraph/internal/rbac/types"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	santaEmail = "santa@northpole.com"
	santaName  = "santa claus"
)

// userCtx returns a context where give user ID identifies logged in user.
func userCtx(userID int32) context.Context {
	ctx := context.Background()
	a := actor.FromUser(userID)
	return actor.WithActor(ctx, a)
}

// fakeOwnService returns given owners file and resolves owners to UnknownOwner.
type fakeOwnService struct {
	Ruleset        *codeowners.Ruleset
	AssignedOwners own.AssignedOwners
	Teams          own.AssignedTeams
}

func (s fakeOwnService) RulesetForRepo(context.Context, api.RepoName, api.RepoID, api.CommitID) (*codeowners.Ruleset, error) {
	return s.Ruleset, nil
}

// ResolverOwnersWithType here behaves in line with production
// OwnService implementation in case handle/email cannot be associated
// with anything - defaults to a Person with a nil person entity.
func (s fakeOwnService) ResolveOwnersWithType(_ context.Context, owners []*codeownerspb.Owner) ([]codeowners.ResolvedOwner, error) {
	var resolved []codeowners.ResolvedOwner
	for _, o := range owners {
		resolved = append(resolved, &codeowners.Person{
			Handle: o.Handle,
			Email:  o.Email,
		})
	}
	return resolved, nil
}

func (s fakeOwnService) AssignedOwnership(context.Context, api.RepoID, api.CommitID) (own.AssignedOwners, error) {
	return s.AssignedOwners, nil
}

func (s fakeOwnService) AssignedTeams(context.Context, api.RepoID, api.CommitID) (own.AssignedTeams, error) {
	return s.Teams, nil
}

// fakeGitServer is a limited gitserver.Client that returns a file for every Stat call.
type fakeGitserver struct {
	gitserver.Client
	files repoFiles
}

type repoPath struct {
	Repo     api.RepoName
	CommitID api.CommitID
	Path     string
}

func fakeOwnDb() *dbmocks.MockDB {
	db := dbmocks.NewMockDB()
	db.RecentContributionSignalsFunc.SetDefaultReturn(dbmocks.NewMockRecentContributionSignalStore())
	db.RecentViewSignalFunc.SetDefaultReturn(dbmocks.NewMockRecentViewSignalStore())
	db.AssignedOwnersFunc.SetDefaultReturn(dbmocks.NewMockAssignedOwnersStore())

	configStore := dbmocks.NewMockSignalConfigurationStore()
	configStore.IsEnabledFunc.SetDefaultReturn(true, nil)
	db.OwnSignalConfigurationsFunc.SetDefaultReturn(configStore)

	return db
}

type repoFiles map[repoPath]string

func (g fakeGitserver) NewFileReader(_ context.Context, repoName api.RepoName, commitID api.CommitID, file string) (io.ReadCloser, error) {
	if g.files == nil {
		return nil, os.ErrNotExist
	}
	content, ok := g.files[repoPath{Repo: repoName, CommitID: commitID, Path: file}]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader([]byte(content))), nil
}

// Stat is a fake implementation that returns a FileInfo
// indicating a regular file for every path it is given,
// except the ones that are actual ancestor paths of some file
// in fakeGitServer.files.
func (g fakeGitserver) Stat(_ context.Context, repoName api.RepoName, commitID api.CommitID, path string) (fs.FileInfo, error) {
	isDir := false
	p := repoPath{
		Repo:     repoName,
		CommitID: commitID,
		Path:     path,
	}
	if p.Path == "" {
		isDir = true
	} else {
		for q := range g.files {
			if p.Repo == q.Repo && p.CommitID == q.CommitID && strings.HasPrefix(q.Path, p.Path+"/") && q.Path != p.Path {
				isDir = true
			}
		}
	}
	return graphqlbackend.CreateFileInfo(path, isDir), nil
}

// TestBlobOwnershipPanelQueryPersonUnresolved mimics the blob ownership panel graphQL
// query, where the owner is unresolved. In that case if we have a handle, we only return
// it as `displayName`.
func TestBlobOwnershipPanelQueryPersonUnresolved(t *testing.T) {
	logger := logtest.Scoped(t)
	fakeDB := fakedb.New()
	db := fakeOwnDb()
	fakeDB.Wire(db)
	repoID := api.RepoID(1)
	own := fakeOwnService{
		Ruleset: codeowners.NewRuleset(
			codeowners.GitRulesetSource{Repo: repoID, Commit: "deadbeef", Path: "CODEOWNERS"},
			&codeownerspb.File{
				Rule: []*codeownerspb.Rule{
					{
						Pattern: "*.js",
						Owner: []*codeownerspb.Owner{
							{Handle: "js-owner"},
						},
						LineNumber: 1,
					},
				},
			}),
	}
	ctx := userCtx(fakeDB.AddUser(types.User{SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: repoID, Name: "github.com/sourcegraph/own"}, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return "deadbeef", nil
	}
	git := fakeGitserver{}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}
	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			fragment OwnerFields on Person {
				email
				avatarURL
				displayName
				user {
					username
					displayName
					url
				}
			}

			fragment CodeownersFileEntryFields on CodeownersFileEntry {
				title
				description
				codeownersFile {
					__typename
					url
				}
				ruleLineMatch
			}

			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: $revision) {
							blob(path: $currentPath) {
								ownership {
									nodes {
										owner {
											...OwnerFields
										}
										reasons {
											...CodeownersFileEntryFields
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"blob": {
						"ownership": {
							"nodes": [
								{
									"owner": {
										"email": "",
										"avatarURL": null,
										"displayName": "js-owner",
										"user": null
									},
									"reasons": [
										{
											"title": "codeowners",
											"description": "Owner is associated with a rule in a CODEOWNERS file.",
											"codeownersFile": {
												"__typename": "GitBlob",
												"url": "/github.com/sourcegraph/own@deadbeef/-/blob/CODEOWNERS"
											},
											"ruleLineMatch": 1
										}
									]
								}
							]
						}
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(42)),
			"revision":    "revision",
			"currentPath": "foo/bar.js",
		},
	})
}

func TestBlobOwnershipPanelQueryIngested(t *testing.T) {
	logger := logtest.Scoped(t)
	fakeDB := fakedb.New()
	db := fakeOwnDb()
	fakeDB.Wire(db)
	repoID := api.RepoID(1)
	own := fakeOwnService{
		Ruleset: codeowners.NewRuleset(
			codeowners.IngestedRulesetSource{ID: int32(repoID)},
			&codeownerspb.File{
				Rule: []*codeownerspb.Rule{
					{
						Pattern: "*.js",
						Owner: []*codeownerspb.Owner{
							{Handle: "js-owner"},
						},
						LineNumber: 1,
					},
				},
			}),
	}
	ctx := userCtx(fakeDB.AddUser(types.User{SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: repoID, Name: "github.com/sourcegraph/own"}, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return "deadbeef", nil
	}
	git := fakeGitserver{}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}
	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			fragment CodeownersFileEntryFields on CodeownersFileEntry {
				title
				description
				codeownersFile {
					__typename
					url
				}
				ruleLineMatch
			}

			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: $revision) {
							blob(path: $currentPath) {
								ownership {
									nodes {
										reasons {
											...CodeownersFileEntryFields
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"blob": {
						"ownership": {
							"nodes": [
								{
									"reasons": [
										{
											"title": "codeowners",
											"description": "Owner is associated with a rule in a CODEOWNERS file.",
											"codeownersFile": {
												"__typename": "VirtualFile",
												"url": "/github.com/sourcegraph/own/-/own/edit"
											},
											"ruleLineMatch": 1
										}
									]
								}
							]
						}
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(repoID)),
			"revision":    "revision",
			"currentPath": "foo/bar.js",
		},
	})
}

func TestBlobOwnershipPanelQueryTeamResolved(t *testing.T) {
	logger := logtest.Scoped(t)
	repo := &types.Repo{Name: "repo-name", ID: 42}
	team := &types.Team{Name: "fake-team", DisplayName: "The Fake Team"}
	parameterRevision := "revision-parameter"
	var resolvedRevision api.CommitID = "revision-resolved"
	git := fakeGitserver{
		files: repoFiles{
			{repo.Name, resolvedRevision, "CODEOWNERS"}: "*.js @fake-team",
		},
	}
	fakeDB := fakedb.New()
	db := dbmocks.NewMockDB()
	db.TeamsFunc.SetDefaultReturn(fakeDB.TeamStore)
	db.UsersFunc.SetDefaultReturn(fakeDB.UserStore)
	db.CodeownersFunc.SetDefaultReturn(dbmocks.NewMockCodeownersStore())
	db.RecentContributionSignalsFunc.SetDefaultReturn(dbmocks.NewMockRecentContributionSignalStore())
	db.RecentViewSignalFunc.SetDefaultReturn(dbmocks.NewMockRecentViewSignalStore())
	db.AssignedOwnersFunc.SetDefaultReturn(dbmocks.NewMockAssignedOwnersStore())
	db.AssignedTeamsFunc.SetDefaultReturn(dbmocks.NewMockAssignedTeamsStore())
	db.OwnSignalConfigurationsFunc.SetDefaultReturn(dbmocks.NewMockSignalConfigurationStore())
	own := own.NewService(git, db)
	ctx := userCtx(fakeDB.AddUser(types.User{SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(repo, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		if rev != parameterRevision {
			return "", errors.Newf("ResolveRev, got %q want %q", rev, parameterRevision)
		}
		return resolvedRevision, nil
	}
	if _, err := fakeDB.TeamStore.CreateTeam(ctx, team); err != nil {
		t.Fatalf("failed to create fake team: %s", err)
	}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}
	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: $revision) {
							blob(path: $currentPath) {
								ownership {
									nodes {
										owner {
											... on Team {
												displayName
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"blob": {
						"ownership": {
							"nodes": [
								{
									"owner": {
										"displayName": "The Fake Team"
									}
								}
							]
						}
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"revision":    parameterRevision,
			"currentPath": "foo/bar.js",
		},
	})
}

func TestBlobOwnershipPanelQueryExternalTeamResolved(t *testing.T) {
	logger := logtest.Scoped(t)
	repo := &types.Repo{Name: "repo-name", ExternalRepo: api.ExternalRepoSpec{ServiceType: "github"}, ID: 42}
	const ghTeamName = "sourcegraph/own"
	parameterRevision := "revision-parameter"
	var resolvedRevision api.CommitID = "revision-resolved"
	git := fakeGitserver{
		files: repoFiles{
			{repo.Name, resolvedRevision, "CODEOWNERS"}: fmt.Sprintf("*.js @%s", ghTeamName),
		},
	}
	fakeDB := fakedb.New()
	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(fakeDB.UserStore)
	db.TeamsFunc.SetDefaultReturn(fakeDB.TeamStore)
	db.CodeownersFunc.SetDefaultReturn(dbmocks.NewMockCodeownersStore())
	db.RecentContributionSignalsFunc.SetDefaultReturn(dbmocks.NewMockRecentContributionSignalStore())
	db.RecentViewSignalFunc.SetDefaultReturn(dbmocks.NewMockRecentViewSignalStore())
	db.AssignedOwnersFunc.SetDefaultReturn(dbmocks.NewMockAssignedOwnersStore())
	db.AssignedTeamsFunc.SetDefaultReturn(dbmocks.NewMockAssignedTeamsStore())
	db.OwnSignalConfigurationsFunc.SetDefaultReturn(dbmocks.NewMockSignalConfigurationStore())
	own := own.NewService(git, db)
	ctx := userCtx(fakeDB.AddUser(types.User{SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(repo, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		if rev != parameterRevision {
			return "", errors.Newf("ResolveRev, got %q want %q", rev, parameterRevision)
		}
		return resolvedRevision, nil
	}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}
	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: $revision) {
							blob(path: $currentPath) {
								ownership {
									nodes {
										owner {
											... on Team {
												id
												name
												displayName
												url
												avatarURL
												readonly
												parentTeam {
													id
												}
												viewerCanAdminister
												creator {
													id
												}
												external
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"blob": {
						"ownership": {
							"nodes": [
								{
									"owner": {
										"id": "VGVhbTow",
										"name": "sourcegraph/own",
										"displayName": "sourcegraph/own",
										"url": "",
										"avatarURL": null,
										"readonly": true,
										"parentTeam": null,
										"viewerCanAdminister": false,
										"creator": null,
										"external": true
									}
								}
							]
						}
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"revision":    parameterRevision,
			"currentPath": "foo/bar.js",
		},
	})

	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: $revision) {
							blob(path: $currentPath) {
								ownership {
									nodes {
										owner {
											... on Team {
												displayName
												members(first: 10) {
													totalCount
												}
												childTeams(first: 10) {
													totalCount
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{"node":{"commit":{"blob":null}}}`,
		ExpectedErrors: []*gqlerrors.QueryError{
			{Message: "cannot get child teams of external team", Path: []any{"node", "commit", "blob", "ownership", "nodes", 0, "owner", "childTeams"}},
			{Message: "cannot get members of external team", Path: []any{"node", "commit", "blob", "ownership", "nodes", 0, "owner", "members"}},
		},
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"revision":    parameterRevision,
			"currentPath": "foo/bar.js",
		},
	})
}

var paginationQuery = `
query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!, $after: String!) {
	node(id: $repo) {
		... on Repository {
			commit(rev: $revision) {
				blob(path: $currentPath) {
					ownership(first: 2, after: $after) {
						totalCount
						pageInfo {
							endCursor
							hasNextPage
						}
						nodes {
							owner {
								...on Person {
									displayName
								}
							}
						}
					}
				}
			}
		}
	}
}`

type paginationResponse struct {
	Node struct {
		Commit struct {
			Blob struct {
				Ownership struct {
					TotalCount int
					PageInfo   struct {
						EndCursor   *string
						HasNextPage bool
					}
					Nodes []struct {
						Owner struct {
							DisplayName string
						}
					}
				}
			}
		}
	}
}

func (r paginationResponse) hasNextPage() bool {
	return r.Node.Commit.Blob.Ownership.PageInfo.HasNextPage
}

func (r paginationResponse) consistentPageInfo() error {
	ownership := r.Node.Commit.Blob.Ownership
	if nextPage, hasCursor := ownership.PageInfo.HasNextPage, ownership.PageInfo.EndCursor != nil; nextPage != hasCursor {
		cursor := "<nil>"
		if ownership.PageInfo.EndCursor != nil {
			cursor = fmt.Sprintf("&%q", *ownership.PageInfo.EndCursor)
		}
		return errors.Newf("PageInfo.HasNextPage %v but PageInfo.EndCursor %s", nextPage, cursor)
	}
	return nil
}

func (r paginationResponse) ownerNames() []string {
	var owners []string
	for _, n := range r.Node.Commit.Blob.Ownership.Nodes {
		owners = append(owners, n.Owner.DisplayName)
	}
	return owners
}

// TestOwnershipPagination issues a number of queries using ownership(first) parameter
// to limit number of responses. It expects to see correct pagination behavior, that is:
// *  all results are eventually returned, in the expected order;
// *  each request returns correct pageInfo and totalCount;
func TestOwnershipPagination(t *testing.T) {
	logger := logtest.Scoped(t)
	fakeDB := fakedb.New()
	db := fakeOwnDb()
	fakeDB.Wire(db)
	rule := &codeownerspb.Rule{
		Pattern: "*.js",
		Owner: []*codeownerspb.Owner{
			{Handle: "js-owner-1"},
			{Handle: "js-owner-2"},
			{Handle: "js-owner-3"},
			{Handle: "js-owner-4"},
			{Handle: "js-owner-5"},
		},
	}

	own := fakeOwnService{
		Ruleset: codeowners.NewRuleset(
			codeowners.IngestedRulesetSource{},
			&codeownerspb.File{
				Rule: []*codeownerspb.Rule{rule},
			}),
	}
	ctx := userCtx(fakeDB.AddUser(types.User{SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(&types.Repo{}, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return "42", nil
	}
	git := fakeGitserver{}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}
	var after string
	var paginatedOwners [][]string
	var lastResponseData *paginationResponse
	// Limit iterations to number of owners total, so that the test
	// has a stop condition in case something malfunctions.
	for range len(rule.Owner) {
		var responseData paginationResponse
		variables := map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(42)),
			"revision":    "revision",
			"currentPath": "foo/bar.js",
			"after":       after,
		}
		response := schema.Exec(ctx, paginationQuery, "", variables)
		for _, err := range response.Errors {
			t.Errorf("GraphQL Exec, errors: %s", err)
		}
		if response.Data == nil {
			t.Fatal("GraphQL response has no data.")
		}
		if err := json.Unmarshal(response.Data, &responseData); err != nil {
			t.Fatalf("Cannot unmarshal GrapgQL JSON response: %s", err)
		}
		ownership := responseData.Node.Commit.Blob.Ownership
		if got, want := ownership.TotalCount, len(rule.Owner); got != want {
			t.Errorf("TotalCount, got %d want %d", got, want)
		}
		paginatedOwners = append(paginatedOwners, responseData.ownerNames())
		if err := responseData.consistentPageInfo(); err != nil {
			t.Error(err)
		}
		lastResponseData = &responseData
		if ownership.PageInfo.HasNextPage {
			after = *ownership.PageInfo.EndCursor
		} else {
			break
		}
	}
	if lastResponseData == nil {
		t.Error("No response received.")
	} else if lastResponseData.hasNextPage() {
		t.Error("Last response has next page information - result is not exhaustive.")
	}
	wantPaginatedOwners := [][]string{
		{
			"js-owner-1",
			"js-owner-2",
		},
		{
			"js-owner-3",
			"js-owner-4",
		},
		{
			"js-owner-5",
		},
	}
	if diff := cmp.Diff(wantPaginatedOwners, paginatedOwners); diff != "" {
		t.Errorf("returned owners -want+got: %s", diff)
	}
}

func TestOwnership_WithSignals(t *testing.T) {
	logger := logtest.Scoped(t)
	fakeDB := fakedb.New()
	db := fakeOwnDb()

	recentContribStore := dbmocks.NewMockRecentContributionSignalStore()
	recentContribStore.FindRecentAuthorsFunc.SetDefaultReturn([]database.RecentContributorSummary{{
		AuthorName:        santaName,
		AuthorEmail:       santaEmail,
		ContributionCount: 5,
	}}, nil)
	db.RecentContributionSignalsFunc.SetDefaultReturn(recentContribStore)

	recentViewStore := dbmocks.NewMockRecentViewSignalStore()
	recentViewStore.ListFunc.SetDefaultReturn([]database.RecentViewSummary{{
		UserID:     1,
		FilePathID: 1,
		ViewsCount: 10,
	}}, nil)
	db.RecentViewSignalFunc.SetDefaultReturn(recentViewStore)

	userEmails := dbmocks.NewMockUserEmailsStore()
	userEmails.GetPrimaryEmailFunc.SetDefaultReturn(santaEmail, true, nil)
	db.UserEmailsFunc.SetDefaultReturn(userEmails)

	db.UserExternalAccountsFunc.SetDefaultReturn(dbmocks.NewMockUserExternalAccountsStore())

	fakeDB.Wire(db)
	repoID := api.RepoID(1)
	own := fakeOwnService{
		Ruleset: codeowners.NewRuleset(
			codeowners.IngestedRulesetSource{ID: int32(repoID)},
			&codeownerspb.File{
				Rule: []*codeownerspb.Rule{
					{
						Pattern: "*.js",
						Owner: []*codeownerspb.Owner{
							{Handle: "js-owner"},
						},
						LineNumber: 1,
					},
				},
			}),
	}
	ctx := userCtx(fakeDB.AddUser(types.User{Username: santaName, DisplayName: santaName, SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: repoID, Name: "github.com/sourcegraph/own"}, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return "deadbeef", nil
	}
	git := fakeGitserver{}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}

	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			fragment CodeownersFileEntryFields on CodeownersFileEntry {
				title
				description
				codeownersFile {
					__typename
					url
				}
				ruleLineMatch
			}

			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: $revision) {
							blob(path: $currentPath) {
								ownership {
									totalOwners
									totalCount
									nodes {
										owner {
											...on Person {
												displayName
												email
											}
										}
										reasons {
											...CodeownersFileEntryFields
											...on RecentContributorOwnershipSignal {
											  title
											  description
											}
											... on RecentViewOwnershipSignal {
											  title
											  description
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"blob": {
						"ownership": {
							"totalOwners": 1,
							"totalCount": 3,
							"nodes": [
								{
									"owner": {
										"displayName": "js-owner",
										"email": ""
									},
									"reasons": [
										{
											"title": "codeowners",
											"description": "Owner is associated with a rule in a CODEOWNERS file.",
											"codeownersFile": {
												"__typename": "VirtualFile",
												"url": "/github.com/sourcegraph/own/-/own/edit"
											},
											"ruleLineMatch": 1
										}
									]
								},
								{
									"owner": {
										"displayName": "santa@northpole.com",
										"email": "santa@northpole.com"
									},
									"reasons": [
										{
											"title": "recent contributor",
											"description": "Associated because they have contributed to this file in the last 90 days."
										}
									]
								},
								{
									"owner": {
										"displayName": "santa claus",
										"email": ""
									},
									"reasons": [
										{
											"title": "recent view",
											"description": "Associated because they have viewed this file in the last 90 days."
										}
									]
								}
							]
						}
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(repoID)),
			"revision":    "revision",
			"currentPath": "foo/bar.js",
		},
	})
}

func TestTreeOwnershipSignals(t *testing.T) {
	logger := logtest.Scoped(t)
	fakeDB := fakedb.New()
	db := fakeOwnDb()

	recentContribStore := dbmocks.NewMockRecentContributionSignalStore()
	recentContribStore.FindRecentAuthorsFunc.SetDefaultReturn([]database.RecentContributorSummary{{
		AuthorName:        santaName,
		AuthorEmail:       santaEmail,
		ContributionCount: 5,
	}}, nil)
	db.RecentContributionSignalsFunc.SetDefaultReturn(recentContribStore)

	recentViewStore := dbmocks.NewMockRecentViewSignalStore()
	recentViewStore.ListFunc.SetDefaultReturn([]database.RecentViewSummary{{
		UserID:     1,
		FilePathID: 1,
		ViewsCount: 10,
	}}, nil)
	db.RecentViewSignalFunc.SetDefaultReturn(recentViewStore)

	userEmails := dbmocks.NewMockUserEmailsStore()
	userEmails.ListByUserFunc.SetDefaultReturn([]*database.UserEmail{
		{
			UserID:  1,
			Email:   santaEmail,
			Primary: true,
		},
	}, nil)
	db.UserEmailsFunc.SetDefaultReturn(userEmails)

	db.UserExternalAccountsFunc.SetDefaultReturn(dbmocks.NewMockUserExternalAccountsStore())

	fakeDB.Wire(db)
	repoID := api.RepoID(1)
	own := fakeOwnService{
		Ruleset: codeowners.NewRuleset(
			codeowners.IngestedRulesetSource{ID: int32(repoID)},
			&codeownerspb.File{
				Rule: []*codeownerspb.Rule{
					{
						Pattern: "*.js",
						Owner: []*codeownerspb.Owner{
							{Handle: "js-owner"},
						},
						LineNumber: 1,
					},
				},
			}),
	}
	ctx := userCtx(fakeDB.AddUser(types.User{Username: santaName, DisplayName: santaName, SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: repoID, Name: "github.com/sourcegraph/own"}, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return "deadbeef", nil
	}
	git := fakeGitserver{
		files: repoFiles{
			repoPath{
				Repo:     "github.com/sourcegraph/own",
				CommitID: "deadbeef",
				Path:     "foo/bar.js",
			}: "some JS code",
		},
	}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}

	test := &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					...on Repository {
						commit(rev: $revision) {
							path(path: $currentPath) {
								...on GitTree {
									ownership {
										nodes {
											owner {
												...on Person {
													displayName
													email
												}
											}
											reasons {
												...on RecentContributorOwnershipSignal {
													title
													description
												}
												...on RecentViewOwnershipSignal {
													title
													description
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"path": {
						"ownership": {
							"nodes": [
								{
									"owner": {
										"displayName": "santa@northpole.com",
										"email": "santa@northpole.com"
									},
									"reasons": [
										{
											"title": "recent contributor",
											"description": "Associated because they have contributed to this file in the last 90 days."
										}
									]
								},
								{
									"owner": {
										"displayName": "santa claus",
										"email": "santa@northpole.com"
									},
									"reasons": [
										{
											"title": "recent view",
											"description": "Associated because they have viewed this file in the last 90 days."
										}
									]
								}
							]
						}
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(repoID)),
			"revision":    "revision",
			"currentPath": "foo",
		},
	}
	graphqlbackend.RunTest(t, test)

	t.Run("disabled recent-contributor signal should not resolve", func(t *testing.T) {
		mockStore := dbmocks.NewMockSignalConfigurationStore()
		db.OwnSignalConfigurationsFunc.SetDefaultReturn(mockStore)
		mockStore.IsEnabledFunc.SetDefaultHook(func(ctx context.Context, s string) (bool, error) {
			t.Log(s)
			if s == owntypes.SignalRecentContributors {
				return false, nil
			}
			return true, nil
		})

		test.ExpectedResult = `{
			"node": {
				"commit": {
					"path": {
						"ownership": {
							"nodes": [
								{
									"owner": {
										"displayName": "santa claus",
										"email": "santa@northpole.com"
									},
									"reasons": [
										{
											"title": "recent view",
											"description": "Associated because they have viewed this file in the last 90 days."
										}
									]
								}
							]
						}
					}
				}
			}
		}
`
		graphqlbackend.RunTest(t, test)
	})

	t.Run("disabled recent-views signal should not resolve", func(t *testing.T) {
		mockStore := dbmocks.NewMockSignalConfigurationStore()
		db.OwnSignalConfigurationsFunc.SetDefaultReturn(mockStore)
		mockStore.IsEnabledFunc.SetDefaultHook(func(ctx context.Context, s string) (bool, error) {
			if s == owntypes.SignalRecentViews {
				return false, nil
			}
			return true, nil
		})

		test.ExpectedResult = `{
			"node": {
				"commit": {
					"path": {
						"ownership": {
							"nodes": [
								{
									"owner": {
										"displayName": "santa@northpole.com",
										"email": "santa@northpole.com"
									},
									"reasons": [
										{
											"title": "recent contributor",
											"description": "Associated because they have contributed to this file in the last 90 days."
										}
									]
								}
							]
						}
					}
				}
			}
		}
`
		graphqlbackend.RunTest(t, test)
	})
}

func TestCommitOwnershipSignals(t *testing.T) {
	logger := logtest.Scoped(t)
	fakeDB := fakedb.New()
	db := fakeOwnDb()

	recentContribStore := dbmocks.NewMockRecentContributionSignalStore()
	recentContribStore.FindRecentAuthorsFunc.SetDefaultReturn([]database.RecentContributorSummary{{
		AuthorName:        "santa claus",
		AuthorEmail:       "santa@northpole.com",
		ContributionCount: 5,
	}}, nil)
	db.RecentContributionSignalsFunc.SetDefaultReturn(recentContribStore)

	fakeDB.Wire(db)
	repoID := api.RepoID(1)

	ctx := userCtx(fakeDB.AddUser(types.User{SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: repoID, Name: "github.com/sourcegraph/own"}, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return "deadbeef", nil
	}
	git := fakeGitserver{}
	own := fakeOwnService{}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}
	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			query FetchOwnership($repo: ID!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: "revision") {
							ownership {
								nodes {
									owner {
										...on Person {
											displayName
											email
										}
									}
									reasons {
										...on RecentContributorOwnershipSignal {
											title
											description
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"ownership": {
						"nodes": [
							{
								"owner": {
									"displayName": "santa@northpole.com",
									"email": "santa@northpole.com"
								},
								"reasons": [
									{
										"title": "recent contributor",
										"description": "Associated because they have contributed to this file in the last 90 days."
									}
								]
							}
						]
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo": string(graphqlbackend.MarshalRepositoryID(repoID)),
		},
	})
}

func Test_SignalConfigurations(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	git := fakeGitserver{}
	own := fakeOwnService{}

	ctx := context.Background()

	admin, err := db.Users().Create(ctx, database.NewUser{Username: "admin"})
	require.NoError(t, err)

	user, err := db.Users().Create(ctx, database.NewUser{Username: "non-admin"})
	require.NoError(t, err)

	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}

	adminActor := actor.FromUser(admin.ID)
	adminCtx := actor.WithActor(ctx, adminActor)

	baseReadTest := &graphqlbackend.Test{
		Context: adminCtx,
		Schema:  schema,
		Query: `
			query asdf {
			  ownSignalConfigurations {
				name
				description
				isEnabled
				excludedRepoPatterns
			  }
			}`,
		ExpectedResult: `{
		  "ownSignalConfigurations": [
			{
			  "name": "recent-contributors",
			  "description": "Indexes contributors in each file using repository history.",
			  "isEnabled": false,
			  "excludedRepoPatterns": []
			},
			{
			  "name": "recent-views",
			  "description": "Indexes users that recently viewed files in Sourcegraph.",
			  "isEnabled": false,
			  "excludedRepoPatterns": []
			},
			{
			  "name": "analytics",
			  "description": "Indexes ownership data to present in aggregated views like Admin > Analytics > Own and Repo > Ownership",
			  "isEnabled": false,
			  "excludedRepoPatterns": []
			}
		  ]
		}`,
	}

	mutationTest := &graphqlbackend.Test{
		Context: ctx,
		Schema:  schema,
		Query: `
				mutation asdf($input:UpdateSignalConfigurationsInput!) {
				  updateOwnSignalConfigurations(input:$input) {
					isEnabled
					name
					description
					excludedRepoPatterns
				  }
				}`,
		Variables: map[string]any{"input": map[string]any{
			"configs": []any{map[string]any{
				"name": owntypes.SignalRecentContributors, "enabled": true, "excludedRepoPatterns": []any{"github.com/*"},
			}},
		}},
	}

	t.Run("admin access can read", func(t *testing.T) {
		graphqlbackend.RunTest(t, baseReadTest)
	})

	t.Run("user without admin access", func(t *testing.T) {
		userActor := actor.FromUser(user.ID)
		userCtx := actor.WithActor(ctx, userActor)

		expectedErrs := []*gqlerrors.QueryError{{
			Message: "must be site admin",
			Path:    []any{"updateOwnSignalConfigurations"},
		}}

		mutationTest.Context = userCtx
		mutationTest.ExpectedErrors = expectedErrs
		mutationTest.ExpectedResult = `null`

		graphqlbackend.RunTest(t, mutationTest)

		// ensure the configs didn't change despite the error
		configsFromDb, err := db.OwnSignalConfigurations().LoadConfigurations(ctx, database.LoadSignalConfigurationArgs{})
		require.NoError(t, err)
		autogold.Expect([]database.SignalConfiguration{
			{
				ID:          1,
				Name:        owntypes.SignalRecentContributors,
				Description: "Indexes contributors in each file using repository history.",
			},
			{
				ID:          2,
				Name:        owntypes.SignalRecentViews,
				Description: "Indexes users that recently viewed files in Sourcegraph.",
			},
			{
				ID:          3,
				Name:        "analytics",
				Description: "Indexes ownership data to present in aggregated views like Admin > Analytics > Own and Repo > Ownership",
			},
		}).Equal(t, configsFromDb)

		readTest := baseReadTest

		// ensure they can't read configs
		readTest.ExpectedErrors = expectedErrs
		readTest.ExpectedResult = "null"
		readTest.Context = userCtx
	})

	t.Run("user with admin access", func(t *testing.T) {
		mutationTest.Context = adminCtx
		mutationTest.ExpectedErrors = nil
		mutationTest.ExpectedResult = `{
		  "updateOwnSignalConfigurations": [
			{
			  "name": "recent-contributors",
			  "description": "Indexes contributors in each file using repository history.",
			  "isEnabled": true,
			  "excludedRepoPatterns": ["github.com/*"]
			},
			{
			  "name": "recent-views",
			  "description": "Indexes users that recently viewed files in Sourcegraph.",
			  "isEnabled": false,
			  "excludedRepoPatterns": []
			},
			{
			  "name": "analytics",
			  "description": "Indexes ownership data to present in aggregated views like Admin > Analytics > Own and Repo > Ownership",
			  "isEnabled": false,
			  "excludedRepoPatterns": []
			}
		  ]
		}`

		graphqlbackend.RunTest(t, mutationTest)
	})
}

func TestOwnership_WithAssignedOwnersAndTeams(t *testing.T) {
	logger := logtest.Scoped(t)
	fakeDB := fakedb.New()
	db := fakeOwnDb()

	userEmails := dbmocks.NewMockUserEmailsStore()
	userEmails.ListByUserFunc.SetDefaultHook(func(ctx context.Context, opts database.UserEmailsListOptions) ([]*database.UserEmail, error) {
		var email string
		switch opts.UserID {
		case 1:
			email = "assigned@owner1.com"
		case 2:
			email = "assigned@owner2.com"
		default:
			email = santaEmail
		}
		return []*database.UserEmail{
			{
				UserID: opts.UserID,
				Email:  email,
			},
		}, nil
	})
	db.UserEmailsFunc.SetDefaultReturn(userEmails)

	fakeDB.Wire(db)
	repoID := api.RepoID(1)
	assignedOwnerID1 := fakeDB.AddUser(types.User{Username: "assigned owner 1", DisplayName: "I am an assigned owner #1"})
	assignedOwnerID2 := fakeDB.AddUser(types.User{Username: "assigned owner 2", DisplayName: "I am an assigned owner #2"})
	assignedTeamID1 := fakeDB.AddTeam(&types.Team{Name: "assigned team 1"})
	assignedTeamID2 := fakeDB.AddTeam(&types.Team{Name: "assigned team 2"})
	own := fakeOwnService{
		Ruleset: codeowners.NewRuleset(
			codeowners.IngestedRulesetSource{ID: int32(repoID)},
			&codeownerspb.File{
				Rule: []*codeownerspb.Rule{
					{
						Pattern: "*.js",
						Owner: []*codeownerspb.Owner{
							{Handle: "js-owner"},
						},
						LineNumber: 1,
					},
				},
			},
		),
		AssignedOwners: own.AssignedOwners{
			"foo/bar.js": []database.AssignedOwnerSummary{{OwnerUserID: assignedOwnerID1, FilePath: "foo/bar.js", RepoID: repoID}},
			"foo":        []database.AssignedOwnerSummary{{OwnerUserID: assignedOwnerID2, FilePath: "foo", RepoID: repoID}},
		},
		Teams: own.AssignedTeams{
			"foo/bar.js": []database.AssignedTeamSummary{{OwnerTeamID: assignedTeamID1, FilePath: "foo/bar.js", RepoID: repoID}},
			"foo":        []database.AssignedTeamSummary{{OwnerTeamID: assignedTeamID2, FilePath: "foo", RepoID: repoID}},
		},
	}
	ctx := userCtx(fakeDB.AddUser(types.User{Username: santaName, DisplayName: santaName, SiteAdmin: true}))
	repos := dbmocks.NewMockRepoStore()
	db.ReposFunc.SetDefaultReturn(repos)
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: repoID, Name: "github.com/sourcegraph/own"}, nil)
	backend.Mocks.Repos.ResolveRev = func(_ context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return "deadbeef", nil
	}
	db.UserExternalAccountsFunc.SetDefaultReturn(dbmocks.NewMockUserExternalAccountsStore())
	git := fakeGitserver{}
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}

	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			fragment CodeownersFileEntryFields on CodeownersFileEntry {
				title
				description
				codeownersFile {
					__typename
					url
				}
				ruleLineMatch
			}

			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: $revision) {
							blob(path: $currentPath) {
								ownership {
									totalOwners
									totalCount
									nodes {
										owner {
											...on Person {
												displayName
												email
											}
											...on Team {
												name
											}
										}
										reasons {
											...CodeownersFileEntryFields
											...on RecentContributorOwnershipSignal {
											  title
											  description
											}
											... on RecentViewOwnershipSignal {
											  title
											  description
											}
											... on AssignedOwner {
											  title
											  description
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"blob": {
						"ownership": {
							"totalOwners": 5,
							"totalCount": 5,
							"nodes": [
								{
									"owner": {
										"displayName": "I am an assigned owner #1",
										"email": "assigned@owner1.com"
									},
									"reasons": [
										{
											"title": "assigned owner",
											"description": "Owner is manually assigned."
										}
									]
								},
								{
									"owner": {
										"displayName": "I am an assigned owner #2",
										"email": "assigned@owner2.com"
									},
									"reasons": [
										{
											"title": "assigned owner",
											"description": "Owner is manually assigned."
										}
									]
								},
								{
									"owner": {
										"name": "assigned team 1"
									},
									"reasons": [
										{
											"title": "assigned owner",
											"description": "Owner is manually assigned."
										}
									]
								},
								{
									"owner": {
										"name": "assigned team 2"
									},
									"reasons": [
										{
											"title": "assigned owner",
											"description": "Owner is manually assigned."
										}
									]
								},
								{
									"owner": {
										"displayName": "js-owner",
										"email": ""
									},
									"reasons": [
										{
											"title": "codeowners",
											"description": "Owner is associated with a rule in a CODEOWNERS file.",
											"codeownersFile": {
												"__typename": "VirtualFile",
												"url": "/github.com/sourcegraph/own/-/own/edit"
											},
											"ruleLineMatch": 1
										}
									]
								}
							]
						}
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(repoID)),
			"revision":    "revision",
			"currentPath": "foo/bar.js",
		},
	})

	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			query FetchOwnership($repo: ID!, $revision: String!, $currentPath: String!) {
				node(id: $repo) {
					... on Repository {
						commit(rev: $revision) {
							blob(path: $currentPath) {
								ownership {
									totalOwners
									totalCount
									nodes {
										owner {
											...on Person {
												displayName
												email
											}
											...on Team {
												name
											}
										}
										reasons {
											...on RecentContributorOwnershipSignal {
											  title
											  description
											}
											... on RecentViewOwnershipSignal {
											  title
											  description
											}
											... on AssignedOwner {
											  title
											  description
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
		ExpectedResult: `{
			"node": {
				"commit": {
					"blob": {
						"ownership": {
							"totalOwners": 2,
							"totalCount": 2,
							"nodes": [
								{
									"owner": {
										"displayName": "I am an assigned owner #2",
										"email": "assigned@owner2.com"
									},
									"reasons": [
										{
											"title": "assigned owner",
											"description": "Owner is manually assigned."
										}
									]
								},
								{
									"owner": {
										"name": "assigned team 2"
									},
									"reasons": [
										{
											"title": "assigned owner",
											"description": "Owner is manually assigned."
										}
									]
								}
							]
						}
					}
				}
			}
		}`,
		Variables: map[string]any{
			"repo":        string(graphqlbackend.MarshalRepositoryID(repoID)),
			"revision":    "revision",
			"currentPath": "foo",
		},
	})
}

func TestAssignOwner(t *testing.T) {
	logger := logtest.Scoped(t)
	testDB := dbtest.NewDB(t)
	db := database.NewDB(logger, testDB)
	git := fakeGitserver{}
	own := fakeOwnService{}
	ctx := context.Background()
	repo := types.Repo{Name: "test-repo-1", ID: 101}
	err := db.Repos().Create(ctx, &repo)
	require.NoError(t, err)
	// Creating 2 users, only "hasPermission" user has rights to assign owners.
	hasPermission, err := db.Users().Create(ctx, database.NewUser{Username: "has-permission"})
	require.NoError(t, err)
	noPermission, err := db.Users().Create(ctx, database.NewUser{Username: "no-permission"})
	require.NoError(t, err)
	// RBAC stuff below.
	permission, err := db.Permissions().Create(ctx, database.CreatePermissionOpts{
		Namespace: rbactypes.OwnershipNamespace,
		Action:    rbactypes.OwnershipAssignAction,
	})
	require.NoError(t, err)
	role, err := db.Roles().Create(ctx, "Can assign owners", false)
	require.NoError(t, err)
	err = db.RolePermissions().Assign(ctx, database.AssignRolePermissionOpts{
		RoleID:       role.ID,
		PermissionID: permission.ID,
	})
	require.NoError(t, err)
	err = db.UserRoles().Assign(ctx, database.AssignUserRoleOpts{
		UserID: hasPermission.ID,
		RoleID: role.ID,
	})
	require.NoError(t, err)
	// RBAC stuff finished. Creating a GraphQL schema.
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}

	adminCtx := actor.WithActor(ctx, actor.FromUser(hasPermission.ID))
	userCtx := actor.WithActor(ctx, actor.FromUser(noPermission.ID))

	getBaseTest := func() *graphqlbackend.Test {
		return &graphqlbackend.Test{
			Context: userCtx,
			Schema:  schema,
			Query: `
				mutation assignOwner($input:AssignOwnerOrTeamInput!) {
				  assignOwner(input:$input) {
					alwaysNil
				  }
				}`,
			Variables: map[string]any{"input": map[string]any{
				"assignedOwnerID": string(graphqlbackend.MarshalUserID(noPermission.ID)),
				"repoID":          string(graphqlbackend.MarshalRepositoryID(repo.ID)),
				"absolutePath":    "",
			}},
		}
	}

	removeOwners := func() {
		t.Helper()
		_, err := testDB.ExecContext(ctx, "DELETE FROM assigned_owners")
		require.NoError(t, err)
	}

	assertAssignedOwner := func(t *testing.T, ownerID, whoAssigned int32, repoID api.RepoID, path string) {
		t.Helper()
		owners, err := db.AssignedOwners().ListAssignedOwnersForRepo(ctx, repoID)
		require.NoError(t, err)
		require.Len(t, owners, 1)
		owner := owners[0]
		assert.Equal(t, ownerID, owner.OwnerUserID)
		assert.Equal(t, whoAssigned, owner.WhoAssignedUserID)
		assert.Equal(t, path, owner.FilePath)
	}

	assertNoAssignedOwners := func(t *testing.T, repoID api.RepoID) {
		t.Helper()
		owners, err := db.AssignedOwners().ListAssignedOwnersForRepo(ctx, repoID)
		require.NoError(t, err)
		require.Empty(t, owners)
	}

	t.Run("non-admin cannot assign owner", func(t *testing.T) {
		t.Cleanup(removeOwners)
		baseTest := getBaseTest()
		expectedErrs := []*gqlerrors.QueryError{{
			Message: "user is missing permission OWNERSHIP#ASSIGN",
			Path:    []any{"assignOwner"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"assignOwner":null}`
		graphqlbackend.RunTest(t, baseTest)
		assertNoAssignedOwners(t, repo.ID)
	})

	t.Run("bad request", func(t *testing.T) {
		t.Cleanup(removeOwners)
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		expectedErrs := []*gqlerrors.QueryError{{
			Message: "assigned user ID should not be 0",
			Path:    []any{"assignOwner"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"assignOwner":null}`
		baseTest.Variables = map[string]any{"input": map[string]any{
			"assignedOwnerID":   string(graphqlbackend.MarshalUserID(0)),
			"repoID":            string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"absolutePath":      "",
			"whoAssignedUserID": string(graphqlbackend.MarshalUserID(hasPermission.ID)),
		}}
		graphqlbackend.RunTest(t, baseTest)
		assertNoAssignedOwners(t, repo.ID)
	})

	t.Run("successfully assigned an owner", func(t *testing.T) {
		t.Cleanup(removeOwners)
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		baseTest.ExpectedResult = `{"assignOwner":{"alwaysNil": null}}`
		graphqlbackend.RunTest(t, baseTest)
		assertAssignedOwner(t, noPermission.ID, hasPermission.ID, repo.ID, "")
	})
}

func TestDeleteAssignedOwner(t *testing.T) {
	logger := logtest.Scoped(t)
	testDB := dbtest.NewDB(t)
	db := database.NewDB(logger, testDB)
	git := fakeGitserver{}
	own := fakeOwnService{}
	ctx := context.Background()
	repo := types.Repo{Name: "test-repo-1", ID: 101}
	err := db.Repos().Create(ctx, &repo)
	require.NoError(t, err)
	// Creating 2 users, only "hasPermission" user has rights to assign owners.
	hasPermission, err := db.Users().Create(ctx, database.NewUser{Username: "has-permission"})
	require.NoError(t, err)
	noPermission, err := db.Users().Create(ctx, database.NewUser{Username: "non-permission"})
	require.NoError(t, err)
	// Creating an existing assigned owner.
	require.NoError(t, db.AssignedOwners().Insert(ctx, noPermission.ID, repo.ID, "", hasPermission.ID))
	// RBAC stuff below.
	permission, err := db.Permissions().Create(ctx, database.CreatePermissionOpts{
		Namespace: rbactypes.OwnershipNamespace,
		Action:    rbactypes.OwnershipAssignAction,
	})
	require.NoError(t, err)
	role, err := db.Roles().Create(ctx, "Can assign owners", false)
	require.NoError(t, err)
	err = db.RolePermissions().Assign(ctx, database.AssignRolePermissionOpts{
		RoleID:       role.ID,
		PermissionID: permission.ID,
	})
	require.NoError(t, err)
	err = db.UserRoles().Assign(ctx, database.AssignUserRoleOpts{
		UserID: hasPermission.ID,
		RoleID: role.ID,
	})
	require.NoError(t, err)
	// RBAC stuff finished. Creating a GraphQL schema.
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}

	adminCtx := actor.WithActor(ctx, actor.FromUser(hasPermission.ID))
	userCtx := actor.WithActor(ctx, actor.FromUser(noPermission.ID))

	getBaseTest := func() *graphqlbackend.Test {
		return &graphqlbackend.Test{
			Context: userCtx,
			Schema:  schema,
			Query: `
				mutation removeAssignedOwner($input:AssignOwnerOrTeamInput!) {
				  removeAssignedOwner(input:$input) {
					alwaysNil
				  }
				}`,
			Variables: map[string]any{"input": map[string]any{
				"assignedOwnerID": string(graphqlbackend.MarshalUserID(noPermission.ID)),
				"repoID":          string(graphqlbackend.MarshalRepositoryID(repo.ID)),
				"absolutePath":    "",
			}},
		}
	}

	assertOwnerExists := func(t *testing.T) {
		t.Helper()
		owners, err := db.AssignedOwners().ListAssignedOwnersForRepo(ctx, repo.ID)
		require.NoError(t, err)
		require.Len(t, owners, 1)
		owner := owners[0]
		assert.Equal(t, noPermission.ID, owner.OwnerUserID)
		assert.Equal(t, hasPermission.ID, owner.WhoAssignedUserID)
		assert.Equal(t, "", owner.FilePath)
	}

	assertNoAssignedOwners := func(t *testing.T) {
		t.Helper()
		owners, err := db.AssignedOwners().ListAssignedOwnersForRepo(ctx, repo.ID)
		require.NoError(t, err)
		require.Empty(t, owners)
	}

	t.Run("cannot delete assigned owner without permission", func(t *testing.T) {
		baseTest := getBaseTest()
		expectedErrs := []*gqlerrors.QueryError{{
			Message: "user is missing permission OWNERSHIP#ASSIGN",
			Path:    []any{"removeAssignedOwner"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"removeAssignedOwner":null}`
		graphqlbackend.RunTest(t, baseTest)
		assertOwnerExists(t)
	})

	t.Run("bad request", func(t *testing.T) {
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		expectedErrs := []*gqlerrors.QueryError{{
			Message: "assigned user ID should not be 0",
			Path:    []any{"removeAssignedOwner"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"removeAssignedOwner":null}`
		baseTest.Variables = map[string]any{"input": map[string]any{
			"assignedOwnerID": string(graphqlbackend.MarshalUserID(0)),
			"repoID":          string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"absolutePath":    "",
		}}
		graphqlbackend.RunTest(t, baseTest)
		assertOwnerExists(t)
	})

	t.Run("assigned owner not found", func(t *testing.T) {
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		expectedErrs := []*gqlerrors.QueryError{{
			Message: `deleting assigned owner: cannot delete assigned owner with ID=1337 for "" path for repo with ID=1`,
			Path:    []any{"removeAssignedOwner"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"removeAssignedOwner":null}`
		baseTest.Variables = map[string]any{"input": map[string]any{
			"assignedOwnerID": string(graphqlbackend.MarshalUserID(1337)),
			"repoID":          string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"absolutePath":    "",
		}}
		graphqlbackend.RunTest(t, baseTest)
		assertOwnerExists(t)
	})

	t.Run("assigned owner successfully deleted", func(t *testing.T) {
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		baseTest.ExpectedResult = `{"removeAssignedOwner":{"alwaysNil": null}}`
		graphqlbackend.RunTest(t, baseTest)
		assertNoAssignedOwners(t)
	})
}

func TestAssignTeam(t *testing.T) {
	logger := logtest.Scoped(t)
	testDB := dbtest.NewDB(t)
	db := database.NewDB(logger, testDB)
	git := fakeGitserver{}
	own := fakeOwnService{}
	ctx := context.Background()
	repo := types.Repo{Name: "test-repo-1", ID: 101}
	err := db.Repos().Create(ctx, &repo)
	require.NoError(t, err)
	// Creating 2 users, only "hasPermission" user has rights to assign owners.
	hasPermission, err := db.Users().Create(ctx, database.NewUser{Username: "has-permission"})
	require.NoError(t, err)
	noPermission, err := db.Users().Create(ctx, database.NewUser{Username: "no-permission"})
	require.NoError(t, err)
	// Creating a team.
	team := createTeam(t, ctx, db, "team-A")
	// RBAC stuff below.
	permission, err := db.Permissions().Create(ctx, database.CreatePermissionOpts{
		Namespace: rbactypes.OwnershipNamespace,
		Action:    rbactypes.OwnershipAssignAction,
	})
	require.NoError(t, err)
	role, err := db.Roles().Create(ctx, "Can assign owners", false)
	require.NoError(t, err)
	err = db.RolePermissions().Assign(ctx, database.AssignRolePermissionOpts{
		RoleID:       role.ID,
		PermissionID: permission.ID,
	})
	require.NoError(t, err)
	err = db.UserRoles().Assign(ctx, database.AssignUserRoleOpts{
		UserID: hasPermission.ID,
		RoleID: role.ID,
	})
	require.NoError(t, err)
	// RBAC stuff finished. Creating a GraphQL schema.
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}

	adminCtx := actor.WithActor(ctx, actor.FromUser(hasPermission.ID))
	userCtx := actor.WithActor(ctx, actor.FromUser(noPermission.ID))

	getBaseTest := func() *graphqlbackend.Test {
		return &graphqlbackend.Test{
			Context: userCtx,
			Schema:  schema,
			Query: `
				mutation assignTeam($input:AssignOwnerOrTeamInput!) {
				  assignTeam(input:$input) {
					alwaysNil
				  }
				}`,
			Variables: map[string]any{"input": map[string]any{
				"assignedOwnerID": string(graphqlbackend.MarshalTeamID(team.ID)),
				"repoID":          string(graphqlbackend.MarshalRepositoryID(repo.ID)),
				"absolutePath":    "",
			}},
		}
	}

	removeTeams := func() {
		t.Helper()
		_, err := testDB.ExecContext(ctx, "DELETE FROM assigned_teams")
		require.NoError(t, err)
	}

	assertAssignedTeam := func(t *testing.T, ownerID, whoAssigned int32, repoID api.RepoID, path string) {
		t.Helper()
		owners, err := db.AssignedTeams().ListAssignedTeamsForRepo(ctx, repoID)
		require.NoError(t, err)
		require.Len(t, owners, 1)
		owner := owners[0]
		assert.Equal(t, ownerID, owner.OwnerTeamID)
		assert.Equal(t, whoAssigned, owner.WhoAssignedUserID)
		assert.Equal(t, path, owner.FilePath)
	}

	assertNoAssignedOwners := func(t *testing.T, repoID api.RepoID) {
		t.Helper()
		owners, err := db.AssignedTeams().ListAssignedTeamsForRepo(ctx, repoID)
		require.NoError(t, err)
		require.Empty(t, owners)
	}

	t.Run("non-admin cannot assign a team", func(t *testing.T) {
		t.Cleanup(removeTeams)
		baseTest := getBaseTest()
		expectedErrs := []*gqlerrors.QueryError{{
			Message: "user is missing permission OWNERSHIP#ASSIGN",
			Path:    []any{"assignTeam"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"assignTeam":null}`
		graphqlbackend.RunTest(t, baseTest)
		assertNoAssignedOwners(t, repo.ID)
	})

	t.Run("bad request", func(t *testing.T) {
		t.Cleanup(removeTeams)
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		expectedErrs := []*gqlerrors.QueryError{{
			Message: "assigned team ID should not be 0",
			Path:    []any{"assignTeam"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"assignTeam":null}`
		baseTest.Variables = map[string]any{"input": map[string]any{
			"assignedOwnerID":   string(graphqlbackend.MarshalTeamID(0)),
			"repoID":            string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"absolutePath":      "",
			"whoAssignedUserID": string(graphqlbackend.MarshalUserID(hasPermission.ID)),
		}}
		graphqlbackend.RunTest(t, baseTest)
		assertNoAssignedOwners(t, repo.ID)
	})

	t.Run("successfully assigned a team", func(t *testing.T) {
		t.Cleanup(removeTeams)
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		baseTest.ExpectedResult = `{"assignTeam":{"alwaysNil": null}}`
		graphqlbackend.RunTest(t, baseTest)
		assertAssignedTeam(t, team.ID, hasPermission.ID, repo.ID, "")
	})
}

func TestDeleteAssignedTeam(t *testing.T) {
	logger := logtest.Scoped(t)
	testDB := dbtest.NewDB(t)
	db := database.NewDB(logger, testDB)
	git := fakeGitserver{}
	own := fakeOwnService{}
	ctx := context.Background()
	repo := types.Repo{Name: "test-repo-1", ID: 101}
	err := db.Repos().Create(ctx, &repo)
	require.NoError(t, err)
	// Creating 2 users, only "hasPermission" user has rights to assign owners.
	hasPermission, err := db.Users().Create(ctx, database.NewUser{Username: "has-permission"})
	require.NoError(t, err)
	noPermission, err := db.Users().Create(ctx, database.NewUser{Username: "non-permission"})
	require.NoError(t, err)
	// Creating a team.
	team := createTeam(t, ctx, db, "team-A")
	// Creating an existing assigned team.
	require.NoError(t, db.AssignedTeams().Insert(ctx, team.ID, repo.ID, "", hasPermission.ID))
	// RBAC stuff below.
	permission, err := db.Permissions().Create(ctx, database.CreatePermissionOpts{
		Namespace: rbactypes.OwnershipNamespace,
		Action:    rbactypes.OwnershipAssignAction,
	})
	require.NoError(t, err)
	role, err := db.Roles().Create(ctx, "Can assign owners", false)
	require.NoError(t, err)
	err = db.RolePermissions().Assign(ctx, database.AssignRolePermissionOpts{
		RoleID:       role.ID,
		PermissionID: permission.ID,
	})
	require.NoError(t, err)
	err = db.UserRoles().Assign(ctx, database.AssignUserRoleOpts{
		UserID: hasPermission.ID,
		RoleID: role.ID,
	})
	require.NoError(t, err)
	// RBAC stuff finished. Creating a GraphQL schema.
	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, git, own, logger)}})
	if err != nil {
		t.Fatal(err)
	}

	adminCtx := actor.WithActor(ctx, actor.FromUser(hasPermission.ID))
	userCtx := actor.WithActor(ctx, actor.FromUser(noPermission.ID))

	getBaseTest := func() *graphqlbackend.Test {
		return &graphqlbackend.Test{
			Context: userCtx,
			Schema:  schema,
			Query: `
				mutation removeAssignedTeam($input:AssignOwnerOrTeamInput!) {
				  removeAssignedTeam(input:$input) {
					alwaysNil
				  }
				}`,
			Variables: map[string]any{"input": map[string]any{
				"assignedOwnerID": string(graphqlbackend.MarshalTeamID(team.ID)),
				"repoID":          string(graphqlbackend.MarshalRepositoryID(repo.ID)),
				"absolutePath":    "",
			}},
		}
	}

	assertTeamExists := func(t *testing.T) {
		t.Helper()
		teams, err := db.AssignedTeams().ListAssignedTeamsForRepo(ctx, repo.ID)
		require.NoError(t, err)
		require.Len(t, teams, 1)
		owner := teams[0]
		assert.Equal(t, team.ID, owner.OwnerTeamID)
		assert.Equal(t, hasPermission.ID, owner.WhoAssignedUserID)
		assert.Equal(t, "", owner.FilePath)
	}

	assertNoAssignedTeams := func(t *testing.T) {
		t.Helper()
		owners, err := db.AssignedTeams().ListAssignedTeamsForRepo(ctx, repo.ID)
		require.NoError(t, err)
		require.Empty(t, owners)
	}

	t.Run("cannot delete assigned owner without permission", func(t *testing.T) {
		baseTest := getBaseTest()
		expectedErrs := []*gqlerrors.QueryError{{
			Message: "user is missing permission OWNERSHIP#ASSIGN",
			Path:    []any{"removeAssignedTeam"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"removeAssignedTeam":null}`
		graphqlbackend.RunTest(t, baseTest)
		assertTeamExists(t)
	})

	t.Run("bad request", func(t *testing.T) {
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		expectedErrs := []*gqlerrors.QueryError{{
			Message: "assigned team ID should not be 0",
			Path:    []any{"removeAssignedTeam"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"removeAssignedTeam":null}`
		baseTest.Variables = map[string]any{"input": map[string]any{
			"assignedOwnerID": string(graphqlbackend.MarshalUserID(0)),
			"repoID":          string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"absolutePath":    "",
		}}
		graphqlbackend.RunTest(t, baseTest)
		assertTeamExists(t)
	})

	t.Run("assigned owner not found", func(t *testing.T) {
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		expectedErrs := []*gqlerrors.QueryError{{
			Message: `deleting assigned team: cannot delete assigned owner team with ID=1337 for "" path for repo with ID=1`,
			Path:    []any{"removeAssignedTeam"},
		}}
		baseTest.ExpectedErrors = expectedErrs
		baseTest.ExpectedResult = `{"removeAssignedTeam":null}`
		baseTest.Variables = map[string]any{"input": map[string]any{
			"assignedOwnerID": string(graphqlbackend.MarshalUserID(1337)),
			"repoID":          string(graphqlbackend.MarshalRepositoryID(repo.ID)),
			"absolutePath":    "",
		}}
		graphqlbackend.RunTest(t, baseTest)
		assertTeamExists(t)
	})

	t.Run("assigned owner successfully deleted", func(t *testing.T) {
		baseTest := getBaseTest()
		baseTest.Context = adminCtx
		baseTest.ExpectedResult = `{"removeAssignedTeam":{"alwaysNil": null}}`
		graphqlbackend.RunTest(t, baseTest)
		assertNoAssignedTeams(t)
	})
}

func TestDisplayOwnershipStats(t *testing.T) {
	db := dbmocks.NewMockDB()
	fakeRepoPaths := dbmocks.NewMockRepoPathStore()
	fakeRepoPaths.AggregateFileCountFunc.SetDefaultReturn(350000, nil)
	db.RepoPathsFunc.SetDefaultReturn(fakeRepoPaths)
	fakeOwnershipStats := dbmocks.NewMockOwnershipStatsStore()
	updateTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	fakeOwnershipStats.QueryAggregateCountsFunc.SetDefaultReturn(
		database.PathAggregateCounts{
			CodeownedFileCount:         150000,
			AssignedOwnershipFileCount: 20000,
			TotalOwnedFileCount:        165000,
			UpdatedAt:                  updateTime,
		}, nil)
	db.OwnershipStatsFunc.SetDefaultReturn(fakeOwnershipStats)
	ctx := context.Background()
	schema, err := graphqlbackend.NewSchema(db, nil, []graphqlbackend.OptionalResolver{{OwnResolver: resolvers.NewWithService(db, nil, nil, logtest.NoOp(t))}})
	require.NoError(t, err)
	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `
			query GetInstanceOwnStats {
				instanceOwnershipStats {
					totalFiles
					totalCodeownedFiles
					totalOwnedFiles
					totalAssignedOwnershipFiles
					updatedAt
				}
			}`,
		ExpectedResult: `
			{
				"instanceOwnershipStats": {
					"totalFiles": 350000,
					"totalCodeownedFiles": 150000,
					"totalOwnedFiles": 165000,
					"totalAssignedOwnershipFiles": 20000,
					"updatedAt": "2023-01-01T00:00:00Z"
				}
			}`,
	})
}

func createTeam(t *testing.T, ctx context.Context, db database.DB, teamName string) *types.Team {
	t.Helper()
	team, err := db.Teams().CreateTeam(ctx, &types.Team{Name: teamName})
	require.NoError(t, err)
	return team
}
