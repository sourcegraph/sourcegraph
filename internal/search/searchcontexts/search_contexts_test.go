package searchcontexts

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestResolvingValidSearchContextSpecs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		searchContextSpec     string
		wantSearchContextName string
	}{
		{name: "resolve global search context", searchContextSpec: "global", wantSearchContextName: "global"},
		{name: "resolve empty search context as global", searchContextSpec: "", wantSearchContextName: "global"},
		{name: "resolve namespaced search context", searchContextSpec: "@user/test", wantSearchContextName: "test"},
		{name: "resolve namespaced search context with / in name", searchContextSpec: "@user/test/version", wantSearchContextName: "test/version"},
	}

	ns := dbmocks.NewMockNamespaceStore()
	ns.GetByNameFunc.SetDefaultHook(func(ctx context.Context, name string) (*database.Namespace, error) {
		if name == "user" {
			return &database.Namespace{Name: name, User: 1}, nil
		}
		if name == "org" {
			return &database.Namespace{Name: name, Organization: 1}, nil
		}
		return nil, errors.Errorf(`want "user" or "org", got %q`, name)
	})

	sc := dbmocks.NewMockSearchContextsStore()
	sc.GetSearchContextFunc.SetDefaultHook(func(_ context.Context, opts database.GetSearchContextOptions) (*types.SearchContext, error) {
		return &types.SearchContext{Name: opts.Name}, nil
	})

	db := dbmocks.NewMockDB()
	db.NamespacesFunc.SetDefaultReturn(ns)
	db.SearchContextsFunc.SetDefaultReturn(sc)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchContext, err := ResolveSearchContextSpec(context.Background(), db, tt.searchContextSpec)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSearchContextName, searchContext.Name)
		})
	}

	mockrequire.Called(t, ns.GetByNameFunc)
	mockrequire.Called(t, sc.GetSearchContextFunc)
}

func TestResolvingInvalidSearchContextSpecs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		searchContextSpec string
		wantErr           string
	}{
		{name: "invalid format", searchContextSpec: "+user", wantErr: "search context not found"},
		{name: "user not found", searchContextSpec: "@user", wantErr: "search context \"@user\" not found"},
		{name: "org not found", searchContextSpec: "@org", wantErr: "search context \"@org\" not found"},
		{name: "empty user not found", searchContextSpec: "@", wantErr: "search context not found"},
	}

	ns := dbmocks.NewMockNamespaceStore()
	ns.GetByNameFunc.SetDefaultReturn(&database.Namespace{}, nil)

	sc := dbmocks.NewMockSearchContextsStore()
	sc.GetSearchContextFunc.SetDefaultReturn(nil, errors.New("search context not found"))

	db := dbmocks.NewMockDB()
	db.NamespacesFunc.SetDefaultReturn(ns)
	db.SearchContextsFunc.SetDefaultReturn(sc)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveSearchContextSpec(context.Background(), db, tt.searchContextSpec)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}

	mockrequire.Called(t, ns.GetByNameFunc)
	mockrequire.Called(t, sc.GetSearchContextFunc)
}

func TestResolvingInvalidSearchContextSpecs_Cloud(t *testing.T) {
	dotcom.MockSourcegraphDotComMode(t, true)

	tests := []struct {
		name              string
		searchContextSpec string
		wantErr           string
	}{
		{name: "org not a member", searchContextSpec: "@org-not-member", wantErr: "namespace not found"},
		{name: "org not a member with sub-context", searchContextSpec: "@org-not-member/random", wantErr: "namespace not found"},
	}

	ns := dbmocks.NewMockNamespaceStore()
	ns.GetByNameFunc.SetDefaultHook(func(ctx context.Context, name string) (*database.Namespace, error) {
		if name == "org-not-member" {
			return &database.Namespace{Name: name, Organization: 1}, nil
		}
		return &database.Namespace{}, nil
	})

	orgs := dbmocks.NewMockOrgMemberStore()
	orgs.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, &database.ErrOrgMemberNotFound{})

	db := dbmocks.NewMockDB()
	db.NamespacesFunc.SetDefaultReturn(ns)
	db.OrgMembersFunc.SetDefaultReturn(orgs)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveSearchContextSpec(context.Background(), db, tt.searchContextSpec)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}

	mockrequire.Called(t, ns.GetByNameFunc)
	mockrequire.Called(t, orgs.GetByOrgIDAndUserIDFunc)
}

func TestConstructingSearchContextSpecs(t *testing.T) {
	tests := []struct {
		name                  string
		searchContext         *types.SearchContext
		wantSearchContextSpec string
	}{
		{name: "global search context", searchContext: GetGlobalSearchContext(), wantSearchContextSpec: "global"},
		{name: "user auto-defined search context", searchContext: &types.SearchContext{Name: "user", NamespaceUserID: 1, AutoDefined: true}, wantSearchContextSpec: "@user"},
		{name: "org auto-defined search context", searchContext: &types.SearchContext{Name: "org", NamespaceOrgID: 1, AutoDefined: true}, wantSearchContextSpec: "@org"},
		{name: "user namespaced search context", searchContext: &types.SearchContext{ID: 1, Name: "context", NamespaceUserID: 1, NamespaceUserName: "user"}, wantSearchContextSpec: "@user/context"},
		{name: "org namespaced search context", searchContext: &types.SearchContext{ID: 1, Name: "context", NamespaceOrgID: 1, NamespaceOrgName: "org"}, wantSearchContextSpec: "@org/context"},
		{name: "instance-level search context", searchContext: &types.SearchContext{ID: 1, Name: "instance-level-context"}, wantSearchContextSpec: "instance-level-context"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchContextSpec := GetSearchContextSpec(tt.searchContext)
			if searchContextSpec != tt.wantSearchContextSpec {
				t.Fatalf("got %q, expected %q", searchContextSpec, tt.wantSearchContextSpec)
			}
		})
	}
}

func createRepos(ctx context.Context, repoStore database.RepoStore) ([]types.MinimalRepo, error) {
	err := repoStore.Create(ctx, &types.Repo{Name: "github.com/example/a"}, &types.Repo{Name: "github.com/example/b"})
	if err != nil {
		return nil, err
	}
	repoA, err := repoStore.GetByName(ctx, "github.com/example/a")
	if err != nil {
		return nil, err
	}
	repoB, err := repoStore.GetByName(ctx, "github.com/example/b")
	if err != nil {
		return nil, err
	}
	return []types.MinimalRepo{{ID: repoA.ID, Name: repoA.Name}, {ID: repoB.ID, Name: repoB.Name}}, nil
}

func TestResolvingSearchContextRepoNames(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	internalCtx := actor.WithInternalActor(context.Background())
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	u := db.Users()
	r := db.Repos()

	user, err := u.Create(internalCtx, database.NewUser{Username: "u", Password: "p"})
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	repos, err := createRepos(internalCtx, r)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: user.ID})
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	repositoryRevisions := []*types.SearchContextRepositoryRevisions{
		{Repo: repos[0], Revisions: []string{"branch-1"}},
		{Repo: repos[1], Revisions: []string{"branch-2"}},
	}

	searchContext, err := CreateSearchContextWithRepositoryRevisions(ctx, db, &types.SearchContext{Name: "searchcontext"}, repositoryRevisions)
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}

	gotRepos, err := r.ListMinimalRepos(ctx, database.ReposListOptions{SearchContextID: searchContext.ID})
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	if !reflect.DeepEqual(repos, gotRepos) {
		t.Fatalf("wanted %+v repositories, got %+v", repos, gotRepos)
	}
}

func TestSearchContextWriteAccessValidation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	internalCtx := actor.WithInternalActor(context.Background())
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	u := db.Users()

	org, err := db.Orgs().Create(internalCtx, "myorg", nil)
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	// First user is the site admin
	user1, err := u.Create(internalCtx, database.NewUser{Username: "u1", Password: "p"})
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	// Second user is not a site-admin and is a member of the org
	user2, err := u.Create(internalCtx, database.NewUser{Username: "u2", Password: "p"})
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	db.OrgMembers().Create(internalCtx, org.ID, user2.ID)
	// Third user is not a site-admin and is not a member of the org
	user3, err := u.Create(internalCtx, database.NewUser{Username: "u3", Password: "p"})
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}

	tests := []struct {
		name            string
		namespaceUserID int32
		namespaceOrgID  int32
		public          bool
		userID          int32
		wantErr         string
	}{
		{
			name:    "current user must be authenticated",
			userID:  0,
			wantErr: "current user not found",
		},
		{
			name:            "current user must match the user namespace",
			namespaceUserID: user2.ID,
			userID:          user3.ID,
			wantErr:         "search context user does not match current user",
		},
		{
			name:           "current user must be a member of the org namespace",
			namespaceOrgID: org.ID,
			userID:         user3.ID,
			wantErr:        "org member not found",
		},
		{
			name:    "non site-admin users are not valid for instance-level contexts",
			userID:  user2.ID,
			wantErr: "current user must be site-admin",
		},
		{
			name:            "site-admin is invalid for private user search context",
			namespaceUserID: user2.ID,
			userID:          user1.ID,
			wantErr:         "search context user does not match current user",
		},
		{
			name:           "site-admin is invalid for private org search context",
			namespaceOrgID: org.ID,
			userID:         user1.ID,
			wantErr:        "org member not found",
		},
		{
			name:   "site-admin is valid for private instance-level context",
			userID: user1.ID,
		},
		{
			name:            "site-admin is valid for any public user search context",
			namespaceUserID: user2.ID,
			public:          true,
			userID:          user1.ID,
		},
		{
			name:           "site-admin is valid for any public org search context",
			namespaceOrgID: org.ID,
			public:         true,
			userID:         user1.ID,
		},
		{
			name:            "current user is valid if matches the user namespace",
			namespaceUserID: user2.ID,
			userID:          user2.ID,
		},
		{
			name:           "current user is valid if a member of the org namespace",
			namespaceOrgID: org.ID,
			userID:         user2.ID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := actor.WithActor(context.Background(), &actor.Actor{UID: tt.userID})

			err := ValidateSearchContextWriteAccessForCurrentUser(ctx, db, tt.namespaceUserID, tt.namespaceOrgID, tt.public)

			expectErr := tt.wantErr != ""
			if !expectErr && err != nil {
				t.Fatalf("expected no error, got %s", err)
			}
			if expectErr && err == nil {
				t.Fatalf("wanted error, got none")
			}
			if expectErr && err != nil && !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("wanted error containing %s, got %s", tt.wantErr, err)
			}
		})
	}
}

func TestCreatingSearchContexts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	internalCtx := actor.WithInternalActor(context.Background())
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	u := db.Users()

	user1, err := u.Create(internalCtx, database.NewUser{Username: "u1", Password: "p"})
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	repos, err := createRepos(internalCtx, db.Repos())
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}

	existingSearchContext, err := db.SearchContexts().CreateSearchContextWithRepositoryRevisions(
		internalCtx,
		&types.SearchContext{Name: "existing"},
		[]*types.SearchContextRepositoryRevisions{},
	)
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}

	tooLongName := strings.Repeat("x", 33)
	tooLongRevision := strings.Repeat("x", 256)
	tests := []struct {
		name                string
		searchContext       *types.SearchContext
		userID              int32
		repositoryRevisions []*types.SearchContextRepositoryRevisions
		wantErr             string
	}{
		{
			name:          "cannot create search context with global name",
			searchContext: &types.SearchContext{Name: "global"},
			wantErr:       "cannot override global search context",
		},
		{
			name:          "cannot create search context with invalid name",
			searchContext: &types.SearchContext{Name: "invalid name"},
			userID:        user1.ID,
			wantErr:       "\"invalid name\" is not a valid search context name",
		},
		{
			name:          "can create search context with non-space separators",
			searchContext: &types.SearchContext{Name: "version_1.2-final/3"},
			userID:        user1.ID,
		},
		{
			name:          "cannot create search context with name too long",
			searchContext: &types.SearchContext{Name: tooLongName},
			userID:        user1.ID,
			wantErr:       fmt.Sprintf("search context name %q exceeds maximum allowed length (32)", tooLongName),
		},
		{
			name:          "cannot create search context with description too long",
			searchContext: &types.SearchContext{Name: "ctx", Description: strings.Repeat("x", 1025)},
			userID:        user1.ID,
			wantErr:       "search context description exceeds maximum allowed length (1024)",
		},
		{
			name:          "cannot create search context if it already exists",
			searchContext: existingSearchContext,
			userID:        user1.ID,
			wantErr:       "search context already exists",
		},
		{
			name:          "cannot create search context with revisions too long",
			searchContext: &types.SearchContext{Name: "ctx"},
			userID:        user1.ID,
			repositoryRevisions: []*types.SearchContextRepositoryRevisions{
				{Repo: repos[0], Revisions: []string{tooLongRevision}},
			},
			wantErr: fmt.Sprintf("revision %q exceeds maximum allowed length (255)", tooLongRevision),
		},
		{
			name:          "can create search context with repo:has query",
			searchContext: &types.SearchContext{Name: "repo_has_kvp", Query: "repo:has(key:value)"},
			userID:        user1.ID,
		},
		{
			name:          "can create search context with repo:has.tag query",
			searchContext: &types.SearchContext{Name: "repo_has_tag", Query: "repo:has.tag(tag)"},
			userID:        user1.ID,
		},
		{
			name:          "can create search context with repo:has.key query",
			searchContext: &types.SearchContext{Name: "repo_has_key", Query: "repo:has.key(key)"},
			userID:        user1.ID,
		},
		{
			name:          "cannot create search context with unsupported repo field predicate in query",
			searchContext: &types.SearchContext{Name: "unsupported_repo_predicate", Query: "repo:has.content(foo)"},
			userID:        user1.ID,
			wantErr:       fmt.Sprintf("unsupported repo field predicate in search context query: %q", "has.content(foo)"),
		},
		{
			name:          "can create search context query with empty revision",
			searchContext: &types.SearchContext{Name: "empty_revision", Query: "repo:foo/bar@"},
			userID:        user1.ID,
		},
		{
			name:          "cannot create search context query with ref glob",
			searchContext: &types.SearchContext{Name: "unsupported_ref_glob", Query: "repo:foo/bar@*refs/tags/*"},
			userID:        user1.ID,
			wantErr:       fmt.Sprintf("unsupported rev glob in search context query: %q", "foo/bar@*refs/tags/*"),
		},
		{
			name:          "cannot create search context query with exclude ref glob",
			searchContext: &types.SearchContext{Name: "uunsupported_ref_glob", Query: "repo:foo/bar@*!refs/tags/*"},
			userID:        user1.ID,
			wantErr:       fmt.Sprintf("unsupported rev glob in search context query: %q", "foo/bar@*!refs/tags/*"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := actor.WithActor(context.Background(), &actor.Actor{UID: tt.userID})

			_, err := CreateSearchContextWithRepositoryRevisions(ctx, db, tt.searchContext, tt.repositoryRevisions)

			expectErr := tt.wantErr != ""
			if !expectErr && err != nil {
				t.Fatalf("expected no error, got %s", err)
			}
			if expectErr && err == nil {
				t.Fatalf("wanted error, got none")
			}
			if expectErr && err != nil && !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("wanted error containing %s, got %s", tt.wantErr, err)
			}
		})
	}
}

func TestUpdatingSearchContexts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	internalCtx := actor.WithInternalActor(context.Background())
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	u := db.Users()

	user1, err := u.Create(internalCtx, database.NewUser{Username: "u1", Password: "p"})
	require.NoError(t, err)

	repos, err := createRepos(internalCtx, db.Repos())
	require.NoError(t, err)

	var scs []*types.SearchContext
	for i := range 6 {
		sc, err := db.SearchContexts().CreateSearchContextWithRepositoryRevisions(
			internalCtx,
			&types.SearchContext{Name: strconv.Itoa(i)},
			[]*types.SearchContextRepositoryRevisions{},
		)
		require.NoError(t, err)
		scs = append(scs, sc)
	}

	set := func(sc *types.SearchContext, f func(*types.SearchContext)) *types.SearchContext {
		copied := *sc
		f(&copied)
		return &copied
	}

	tests := []struct {
		name                string
		update              *types.SearchContext
		repositoryRevisions []*types.SearchContextRepositoryRevisions
		userID              int32
		wantErr             string
	}{
		{
			name:    "cannot create search context with global name",
			update:  &types.SearchContext{Name: "global"},
			wantErr: "cannot update global search context",
		},
		{
			name:    "cannot update search context to use an invalid name",
			update:  set(scs[0], func(sc *types.SearchContext) { sc.Name = "invalid name" }),
			wantErr: "not a valid search context name",
		},
		{
			name:    "cannot update search context with name too long",
			update:  set(scs[1], func(sc *types.SearchContext) { sc.Name = strings.Repeat("x", 33) }),
			wantErr: "exceeds maximum allowed length (32)",
		},
		{
			name:    "cannot update search context with description too long",
			update:  set(scs[2], func(sc *types.SearchContext) { sc.Description = strings.Repeat("x", 1025) }),
			wantErr: "search context description exceeds maximum allowed length (1024)",
		},
		{
			name:   "cannot update search context with revisions too long",
			update: scs[3],
			repositoryRevisions: []*types.SearchContextRepositoryRevisions{
				{Repo: repos[0], Revisions: []string{strings.Repeat("x", 256)}},
			},
			wantErr: "exceeds maximum allowed length (255)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := actor.WithActor(context.Background(), &actor.Actor{UID: user1.ID})

			updated, err := UpdateSearchContextWithRepositoryRevisions(ctx, db, tt.update, tt.repositoryRevisions)
			if tt.wantErr != "" {
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.update, updated)
		})
	}
}

func TestDeletingAutoDefinedSearchContext(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	internalCtx := actor.WithInternalActor(context.Background())
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	u := db.Users()

	user1, err := u.Create(internalCtx, database.NewUser{Username: "u1", Password: "p"})
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}

	autoDefinedSearchContext := GetGlobalSearchContext()
	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: user1.ID})
	err = DeleteSearchContext(ctx, db, autoDefinedSearchContext)

	wantErr := "cannot delete auto-defined search context"
	if err == nil {
		t.Fatalf("wanted error, got none")
	}
	if err != nil && !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("wanted error containing %s, got %s", wantErr, err)
	}
}

func TestParseRepoOpts(t *testing.T) {
	for _, tc := range []struct {
		in  string
		out []RepoOpts
		err error
	}{
		{
			in: "(r:foo or r:bar) case:yes archived:only visibility:private (rev:HEAD or rev:TAIL)",
			out: []RepoOpts{
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns:       []string{"foo"},
						CaseSensitivePatterns: true,
						OnlyArchived:          true,
						OnlyPrivate:           true,
						NoForks:               true,
					},
					RevSpecs: []string{"HEAD"},
				},
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns:       []string{"bar"},
						CaseSensitivePatterns: true,
						OnlyArchived:          true,
						OnlyPrivate:           true,
						NoForks:               true,
					},
					RevSpecs: []string{"HEAD"},
				},
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns:       []string{"foo"},
						CaseSensitivePatterns: true,
						OnlyArchived:          true,
						OnlyPrivate:           true,
						NoForks:               true,
					},
					RevSpecs: []string{"TAIL"},
				},
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns:       []string{"bar"},
						CaseSensitivePatterns: true,
						OnlyArchived:          true,
						OnlyPrivate:           true,
						NoForks:               true,
					},
					RevSpecs: []string{"TAIL"},
				},
			},
		},
		{
			in: "r:foo|bar@HEAD:TAIL archived:yes",
			out: []RepoOpts{
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns: []string{"foo|bar"},
						NoForks:         true,
					},
					RevSpecs: []string{"HEAD", "TAIL"},
				},
			},
		},
		{
			in: "r:foo|bar@HEAD f:^sub/dir lang:go",
			out: []RepoOpts{
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns: []string{"foo|bar"},
						NoForks:         true,
						NoArchived:      true,
					},
					RevSpecs: []string{"HEAD"},
				},
			},
		},
		{
			in: "(r:foo (rev:HEAD or rev:TAIL)) or r:bar@main:dev",
			out: []RepoOpts{
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns: []string{"foo"},
						NoForks:         true,
						NoArchived:      true,
					},
					RevSpecs: []string{"HEAD"},
				},
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns: []string{"foo"},
						NoForks:         true,
						NoArchived:      true,
					},
					RevSpecs: []string{"TAIL"},
				},
				{
					ReposListOptions: database.ReposListOptions{
						IncludePatterns: []string{"bar"},
						NoForks:         true,
						NoArchived:      true,
					},
					RevSpecs: []string{"main", "dev"},
				},
			},
		},
	} {
		t.Run(tc.in, func(t *testing.T) {
			have, err := ParseRepoOpts(tc.in)
			if err != nil {
				t.Fatal(err)
			}

			want := tc.out
			opts := cmpopts.IgnoreUnexported(database.ReposListOptions{})
			if diff := cmp.Diff(have, want, opts); diff != "" {
				t.Errorf("mismatch: (-have, +want): %s", diff)
			}
		})
	}
}

func Test_validateSearchContextQuery(t *testing.T) {
	cases := []struct {
		query   string
		wantErr bool
	}{{
		query:   "repo:has(key:value)",
		wantErr: false,
	}, {
		query:   "repo:has.tag(mytag)",
		wantErr: false,
	}, {
		query:   "repo:has.key(mykey)",
		wantErr: false,
	}, {
		query:   "repo:has.topic(mytopic)",
		wantErr: false,
	}, {
		query:   "repo:has.path(mytopic)",
		wantErr: true,
	}, {
		query:   "repo:has.description(mytopic)",
		wantErr: false,
	}, {
		query:   "lang:go",
		wantErr: false,
	}, {
		query:   "fork:yes",
		wantErr: false,
	}, {
		query:   "archived:yes",
		wantErr: false,
	}, {
		query:   "case:yes",
		wantErr: false,
	}, {
		query:   "file:test",
		wantErr: false,
	}, {
		query:   "visibility:public",
		wantErr: false,
	}, {
		query:   "type:commit author:camden",
		wantErr: true,
	}, {
		query:   "type:diff author:camden",
		wantErr: true,
	}, {
		query:   "testpattern",
		wantErr: true,
	}}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			err := validateSearchContextQuery(tc.query)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
