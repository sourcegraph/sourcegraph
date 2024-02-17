package resolvers

import (
	"context"
	"fmt"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/permssync"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/github"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/executor"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

var now = timeutil.Now().UnixNano()

func clock() time.Time {
	return time.Unix(0, atomic.LoadInt64(&now))
}

func mustParseGraphQLSchema(t *testing.T, db database.DB) *graphql.Schema {
	t.Helper()

	resolver := NewResolver(observation.TestContextTB(t), db)
	parsedSchema, err := graphqlbackend.NewSchemaWithAuthzResolver(db, resolver)
	if err != nil {
		t.Fatal(err)
	}

	return parsedSchema
}

func TestResolver_SetRepositoryPermissionsForUsers(t *testing.T) {
	t.Cleanup(licensing.TestingSkipFeatureChecks())
	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := (&Resolver{db: db}).SetRepositoryPermissionsForUsers(ctx, &graphqlbackend.RepoPermsArgs{})
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	tests := []struct {
		name               string
		config             *schema.PermissionsUserMapping
		mockVerifiedEmails []*database.UserEmail
		mockUsers          []*types.User
		gqlTests           func(database.DB) []*graphqlbackend.Test
		expUserIDs         map[int32]struct{}
		expAccounts        *extsvc.Accounts
	}{{
		name: "set permissions via email",
		config: &schema.PermissionsUserMapping{
			BindID: "email",
		},
		mockVerifiedEmails: []*database.UserEmail{
			{
				UserID: 1,
				Email:  "alice@example.com",
			},
		},
		gqlTests: func(db database.DB) []*graphqlbackend.Test {
			return []*graphqlbackend.Test{{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
							mutation {
								setRepositoryPermissionsForUsers(
									repository: "UmVwb3NpdG9yeTox",
									userPermissions: [
										{ bindID: "alice@example.com"},
										{ bindID: "bob"}
									]) {
									alwaysNil
								}
							}
						`,
				ExpectedResult: `
							{
								"setRepositoryPermissionsForUsers": {
									"alwaysNil": null
								}
							}
						`,
			},
			}
		},
		expUserIDs: map[int32]struct{}{1: {}},
		expAccounts: &extsvc.Accounts{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountIDs:  []string{"bob"},
		},
	}, {
		name: "set permissions via username",
		config: &schema.PermissionsUserMapping{
			BindID: "username",
		},
		mockUsers: []*types.User{
			{
				ID:       1,
				Username: "alice",
			},
		},
		gqlTests: func(db database.DB) []*graphqlbackend.Test {
			return []*graphqlbackend.Test{{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
						mutation {
							setRepositoryPermissionsForUsers(
								repository: "UmVwb3NpdG9yeTox",
								userPermissions: [
									{ bindID: "alice"},
									{ bindID: "bob"}
								]) {
								alwaysNil
							}
						}
					`,
				ExpectedResult: `
						{
							"setRepositoryPermissionsForUsers": {
								"alwaysNil": null
							}
						}
					`,
			}}
		},
		expUserIDs: map[int32]struct{}{1: {}},
		expAccounts: &extsvc.Accounts{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountIDs:  []string{"bob"},
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			globals.SetPermissionsUserMapping(test.config)

			users := dbmocks.NewStrictMockUserStore()
			users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
			users.GetByUsernamesFunc.SetDefaultReturn(test.mockUsers, nil)

			userEmails := dbmocks.NewStrictMockUserEmailsStore()
			userEmails.GetVerifiedEmailsFunc.SetDefaultReturn(test.mockVerifiedEmails, nil)

			repos := dbmocks.NewStrictMockRepoStore()
			repos.GetFunc.SetDefaultHook(func(_ context.Context, id api.RepoID) (*types.Repo, error) {
				return &types.Repo{ID: id}, nil
			})

			perms := dbmocks.NewStrictMockPermsStore()
			perms.TransactFunc.SetDefaultReturn(perms, nil)
			perms.DoneFunc.SetDefaultReturn(nil)
			perms.SetRepoPermsFunc.SetDefaultHook(func(_ context.Context, repoID int32, ids []authz.UserIDWithExternalAccountID, source authz.PermsSource) (*database.SetPermissionsResult, error) {
				expUserIDs := maps.Keys(test.expUserIDs)
				userIDs := make([]int32, len(ids))
				for i, u := range ids {
					userIDs[i] = u.UserID
				}
				if diff := cmp.Diff(expUserIDs, userIDs); diff != "" {
					return nil, errors.Errorf("userIDs expected: %v, got: %v", expUserIDs, userIDs)
				}
				if source != authz.SourceAPI {
					return nil, errors.Errorf("source expected: %s, got: %s", authz.SourceAPI, source)
				}

				return nil, nil
			})
			perms.SetRepoPendingPermissionsFunc.SetDefaultHook(func(_ context.Context, accounts *extsvc.Accounts, _ *authz.RepoPermissions) error {
				if diff := cmp.Diff(test.expAccounts, accounts); diff != "" {
					return errors.Errorf("accounts: %v", diff)
				}
				return nil
			})
			perms.MapUsersFunc.SetDefaultHook(func(ctx context.Context, s []string, pum *schema.PermissionsUserMapping) (map[string]int32, error) {
				if pum.BindID != test.config.BindID {
					return nil, errors.Errorf("unexpected BindID: %q", pum.BindID)
				}

				m := make(map[string]int32)
				if pum.BindID == "username" {
					for _, u := range test.mockUsers {
						m[u.Username] = u.ID
					}
				} else {
					for _, u := range test.mockVerifiedEmails {
						m[u.Email] = u.UserID
					}
				}

				return m, nil
			})

			db := dbmocks.NewStrictMockDB()
			db.UsersFunc.SetDefaultReturn(users)
			db.UserEmailsFunc.SetDefaultReturn(userEmails)
			db.ReposFunc.SetDefaultReturn(repos)
			db.PermsFunc.SetDefaultReturn(perms)

			graphqlbackend.RunTests(t, test.gqlTests(db))
		})
	}
}

func TestResolver_SetRepositoryPermissionsUnrestricted(t *testing.T) {
	// TODO: Factor out this common check
	t.Cleanup(licensing.TestingSkipFeatureChecks())
	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := (&Resolver{db: db}).SetRepositoryPermissionsForUsers(ctx, &graphqlbackend.RepoPermsArgs{})
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	var haveIDs []int32
	var haveUnrestricted bool

	perms := dbmocks.NewMockPermsStore()
	perms.SetRepoPermissionsUnrestrictedFunc.SetDefaultHook(func(ctx context.Context, ids []int32, unrestricted bool) error {
		haveIDs = ids
		haveUnrestricted = unrestricted
		return nil
	})
	users := dbmocks.NewStrictMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

	db := dbmocks.NewStrictMockDB()
	db.PermsFunc.SetDefaultReturn(perms)
	db.UsersFunc.SetDefaultReturn(users)

	gqlTests := []*graphqlbackend.Test{{
		Schema: mustParseGraphQLSchema(t, db),
		Query: `
						mutation {
							setRepositoryPermissionsUnrestricted(
								repositories: ["UmVwb3NpdG9yeTox","UmVwb3NpdG9yeToy","UmVwb3NpdG9yeToz"],
								unrestricted: true
								) {
								alwaysNil
							}
						}
					`,
		ExpectedResult: `
						{
							"setRepositoryPermissionsUnrestricted": {
								"alwaysNil": null
							}
						}
					`,
	}}

	graphqlbackend.RunTests(t, gqlTests)

	assert.Equal(t, haveIDs, []int32{1, 2, 3})
	assert.True(t, haveUnrestricted)
}

func TestResolver_ScheduleRepositoryPermissionsSync(t *testing.T) {
	t.Cleanup(licensing.TestingSkipFeatureChecks())
	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		result, err := (&Resolver{db: db}).ScheduleRepositoryPermissionsSync(ctx, &graphqlbackend.RepositoryIDArgs{})
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	users := dbmocks.NewStrictMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	r := &Resolver{db: db}

	const repoID = 1

	called := false
	permssync.MockSchedulePermsSync = func(_ context.Context, _ log.Logger, _ database.DB, req permssync.ScheduleSyncOpts) {
		called = true
		if len(req.RepoIDs) != 1 && req.RepoIDs[0] == api.RepoID(repoID) {
			t.Errorf("unexpected repoID argument. want=%d have=%d", repoID, req.RepoIDs[0])
		}
		if req.TriggeredByUserID != 1 {
			t.Errorf("unexpected TriggeredByUserID argument. want=%d have=%d", 1, req.TriggeredByUserID)
		}
	}
	t.Cleanup(func() { permssync.MockSchedulePermsSync = nil })

	_, err := r.ScheduleRepositoryPermissionsSync(ctx, &graphqlbackend.RepositoryIDArgs{
		Repository: graphqlbackend.MarshalRepositoryID(api.RepoID(repoID)),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Fatalf("SchedulePermsSync not called")
	}
}

func TestResolver_ScheduleUserPermissionsSync(t *testing.T) {
	t.Cleanup(licensing.TestingSkipFeatureChecks())
	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 123})

	t.Run("authenticated as non-admin and not the same user", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByIDFunc.SetDefaultReturn(&types.User{ID: 123}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		result, err := (&Resolver{db: db}).ScheduleUserPermissionsSync(ctx, &graphqlbackend.UserPermissionsSyncArgs{User: graphqlbackend.MarshalUserID(1)})
		if want := auth.ErrMustBeSiteAdminOrSameUser; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	users := dbmocks.NewStrictMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{ID: 123, SiteAdmin: true}, nil)

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	const userID = int32(1)

	t.Run("queue a user", func(t *testing.T) {
		r := &Resolver{db: db}

		called := false
		permssync.MockSchedulePermsSync = func(_ context.Context, _ log.Logger, _ database.DB, req permssync.ScheduleSyncOpts) {
			called = true
			if len(req.UserIDs) != 1 || req.UserIDs[0] != userID {
				t.Errorf("unexpected UserIDs argument. want=%d have=%v", userID, req.UserIDs)
			}
			if req.TriggeredByUserID != 123 {
				t.Errorf("unexpected TriggeredByUserID argument. want=%d have=%d", 1, req.TriggeredByUserID)
			}
		}
		t.Cleanup(func() { permssync.MockSchedulePermsSync = nil })

		_, err := r.ScheduleUserPermissionsSync(actor.WithActor(context.Background(), &actor.Actor{UID: 123}),
			&graphqlbackend.UserPermissionsSyncArgs{User: graphqlbackend.MarshalUserID(userID)})
		if err != nil {
			t.Fatal(err)
		}

		if !called {
			t.Fatal("expected SchedulePermsSync to be called but wasn't")
		}
	})

	t.Run("queue the same user, not a site-admin", func(t *testing.T) {
		userID := int32(123)
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: userID}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		r := &Resolver{db: db}

		called := false
		permssync.MockSchedulePermsSync = func(_ context.Context, _ log.Logger, _ database.DB, req permssync.ScheduleSyncOpts) {
			called = true
			if len(req.UserIDs) != 1 || req.UserIDs[0] != userID {
				t.Errorf("unexpected UserIDs argument. want=%d have=%v", userID, req.UserIDs)
			}
			if req.TriggeredByUserID != userID {
				t.Errorf("unexpected TriggeredByUserID argument. want=%d have=%d", 1, req.TriggeredByUserID)
			}
		}
		t.Cleanup(func() { permssync.MockSchedulePermsSync = nil })

		_, err := r.ScheduleUserPermissionsSync(actor.WithActor(context.Background(), &actor.Actor{UID: 123}),
			&graphqlbackend.UserPermissionsSyncArgs{User: graphqlbackend.MarshalUserID(123)})
		if err != nil {
			t.Fatal(err)
		}

		if !called {
			t.Fatal("expected SchedulePermsSync to be called but wasn't")
		}
	})

	t.Run("queue a user with options", func(t *testing.T) {
		r := &Resolver{db: db}

		called := false
		permssync.MockSchedulePermsSync = func(_ context.Context, _ log.Logger, _ database.DB, req permssync.ScheduleSyncOpts) {
			called = true
			if len(req.UserIDs) != 1 && req.UserIDs[0] == userID {
				t.Errorf("unexpected UserIDs argument. want=%d have=%d", userID, req.UserIDs[0])
			}
			if !req.Options.InvalidateCaches {
				t.Errorf("expected InvalidateCaches to be set, but wasn't")
			}
		}

		t.Cleanup(func() { permssync.MockSchedulePermsSync = nil })
		trueVal := true
		_, err := r.ScheduleUserPermissionsSync(actor.WithActor(context.Background(), &actor.Actor{UID: 123}), &graphqlbackend.UserPermissionsSyncArgs{
			User:    graphqlbackend.MarshalUserID(userID),
			Options: &struct{ InvalidateCaches *bool }{InvalidateCaches: &trueVal},
		})
		if err != nil {
			t.Fatal(err)
		}

		if !called {
			t.Fatal("expected SchedulePermsSync to be called but wasn't")
		}
	})
}

func TestResolver_SetRepositoryPermissionsForBitbucketProject(t *testing.T) {
	logger := logtest.Scoped(t)

	t.Cleanup(licensing.TestingSkipFeatureChecks())

	t.Run("disabled on dotcom", func(t *testing.T) {
		envvar.MockSourcegraphDotComMode(true)

		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.SetRepositoryPermissionsForBitbucketProject(ctx, nil)

		if !errors.Is(err, errDisabledSourcegraphDotCom) {
			t.Errorf("err: want %q, but got %q", errDisabledSourcegraphDotCom, err)
		}

		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}

		// Reset the env var for other tests.
		envvar.MockSourcegraphDotComMode(false)
	})

	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.SetRepositoryPermissionsForBitbucketProject(ctx, nil)

		if !errors.Is(err, auth.ErrMustBeSiteAdmin) {
			t.Errorf("err: want %q, but got %q", auth.ErrMustBeSiteAdmin, err)
		}

		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	t.Run("invalid code host", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.SetRepositoryPermissionsForBitbucketProject(ctx,
			&graphqlbackend.RepoPermsBitbucketProjectArgs{
				// Note: Usage of graphqlbackend.MarshalOrgID here is NOT a typo. Intentionally use an
				// incorrect format for the CodeHost ID.
				CodeHost: graphqlbackend.MarshalOrgID(1),
			},
		)

		if err == nil {
			t.Error("expected error, but got nil")
		}

		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	t.Run("non-Bitbucket code host", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		extSvc := dbmocks.NewMockExternalServiceStore()
		extSvc.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int64) (*types.ExternalService, error) {
			if id == 1 {
				return &types.ExternalService{
						ID:          1,
						Kind:        extsvc.KindBitbucketCloud,
						DisplayName: "github :)",
						Config:      extsvc.NewEmptyConfig(),
					},
					nil
			} else {
				return nil, errors.Errorf("Cannot find external service with given ID")
			}
		})
		db.ExternalServicesFunc.SetDefaultReturn(extSvc)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.SetRepositoryPermissionsForBitbucketProject(ctx,
			&graphqlbackend.RepoPermsBitbucketProjectArgs{
				CodeHost: graphqlbackend.MarshalExternalServiceID(1),
			},
		)

		assert.EqualError(t, err, fmt.Sprintf("expected Bitbucket Server external service, got: %s", extsvc.KindBitbucketCloud))
		require.Nil(t, result)
	})

	t.Run("job enqueued", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		bb := dbmocks.NewMockBitbucketProjectPermissionsStore()
		bb.EnqueueFunc.SetDefaultReturn(1, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.BitbucketProjectPermissionsFunc.SetDefaultReturn(bb)

		extSvc := dbmocks.NewMockExternalServiceStore()
		extSvc.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int64) (*types.ExternalService, error) {
			if id == 1 {
				return &types.ExternalService{
						ID:          1,
						Kind:        extsvc.KindBitbucketServer,
						DisplayName: "bb server no jokes here",
						Config:      extsvc.NewEmptyConfig(),
					},
					nil
			} else {
				return nil, errors.Errorf("Cannot find external service with given ID")
			}
		})
		db.ExternalServicesFunc.SetDefaultReturn(extSvc)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		t.Run("unrestricted not set", func(t *testing.T) {
			result, err := r.SetRepositoryPermissionsForBitbucketProject(ctx,
				&graphqlbackend.RepoPermsBitbucketProjectArgs{
					CodeHost: graphqlbackend.MarshalExternalServiceID(1),
				},
			)

			assert.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, &graphqlbackend.EmptyResponse{}, result)

		})

		t.Run("unrestricted set to false", func(t *testing.T) {
			u := false
			result, err := r.SetRepositoryPermissionsForBitbucketProject(ctx,
				&graphqlbackend.RepoPermsBitbucketProjectArgs{
					CodeHost:     graphqlbackend.MarshalExternalServiceID(1),
					Unrestricted: &u,
				},
			)

			assert.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, &graphqlbackend.EmptyResponse{}, result)
		})

		t.Run("unrestricted set to true", func(t *testing.T) {
			u := true
			result, err := r.SetRepositoryPermissionsForBitbucketProject(ctx,
				&graphqlbackend.RepoPermsBitbucketProjectArgs{
					CodeHost:     graphqlbackend.MarshalExternalServiceID(1),
					Unrestricted: &u,
				},
			)

			assert.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, &graphqlbackend.EmptyResponse{}, result)
		})
	})
}

func TestResolver_CancelPermissionsSyncJob(t *testing.T) {
	logger := logtest.Scoped(t)

	t.Cleanup(licensing.TestingSkipFeatureChecks())

	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.CancelPermissionsSyncJob(ctx, nil)

		require.EqualError(t, err, auth.ErrMustBeSiteAdmin.Error())
		require.Equal(t, graphqlbackend.CancelPermissionsSyncJobResultMessageError, result)
	})

	t.Run("invalid sync job ID", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.CancelPermissionsSyncJob(ctx,
			&graphqlbackend.CancelPermissionsSyncJobArgs{
				Job: graphqlbackend.MarshalRepositoryID(1337),
			},
		)

		require.Error(t, err)
		require.Equal(t, graphqlbackend.CancelPermissionsSyncJobResultMessageError, result)
	})

	t.Run("sync job not found", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		permissionSyncJobStore := dbmocks.NewMockPermissionSyncJobStore()
		permissionSyncJobStore.CancelQueuedJobFunc.SetDefaultReturn(database.MockPermissionsSyncJobNotFoundErr)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.PermissionSyncJobsFunc.SetDefaultReturn(permissionSyncJobStore)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		result, err := r.CancelPermissionsSyncJob(ctx,
			&graphqlbackend.CancelPermissionsSyncJobArgs{
				Job: marshalPermissionsSyncJobID(1337),
			},
		)

		require.NoError(t, err)
		require.Equal(t, graphqlbackend.CancelPermissionsSyncJobResultMessageNotFound, result)
	})

	t.Run("SQL error", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		permissionSyncJobStore := dbmocks.NewMockPermissionSyncJobStore()
		const errorText = "oops"
		permissionSyncJobStore.CancelQueuedJobFunc.SetDefaultReturn(errors.New(errorText))

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.PermissionSyncJobsFunc.SetDefaultReturn(permissionSyncJobStore)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		result, err := r.CancelPermissionsSyncJob(ctx,
			&graphqlbackend.CancelPermissionsSyncJobArgs{
				Job: marshalPermissionsSyncJobID(1337),
			},
		)

		require.EqualError(t, err, errorText)
		require.Equal(t, graphqlbackend.CancelPermissionsSyncJobResultMessageError, result)
	})

	t.Run("sync job successfully cancelled", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		permissionSyncJobStore := dbmocks.NewMockPermissionSyncJobStore()
		permissionSyncJobStore.CancelQueuedJobFunc.SetDefaultReturn(nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.PermissionSyncJobsFunc.SetDefaultReturn(permissionSyncJobStore)

		r := &Resolver{db: db, logger: logger}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		result, err := r.CancelPermissionsSyncJob(ctx,
			&graphqlbackend.CancelPermissionsSyncJobArgs{
				Job: marshalPermissionsSyncJobID(1337),
			},
		)

		require.Equal(t, graphqlbackend.CancelPermissionsSyncJobResultMessageSuccess, result)
		require.NoError(t, err)
	})
}

func TestResolver_CancelPermissionsSyncJob_GraphQLQuery(t *testing.T) {
	t.Cleanup(licensing.TestingSkipFeatureChecks())

	users := dbmocks.NewStrictMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

	permissionSyncJobStore := dbmocks.NewMockPermissionSyncJobStore()
	permissionSyncJobStore.CancelQueuedJobFunc.SetDefaultHook(func(_ context.Context, reason string, jobID int) error {
		if jobID == 1 && reason == "because" {
			return nil
		}
		return database.MockPermissionsSyncJobNotFoundErr
	})

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.PermissionSyncJobsFunc.SetDefaultReturn(permissionSyncJobStore)

	t.Run("sync job successfully canceled with reason", func(t *testing.T) {
		graphqlbackend.RunTest(t, &graphqlbackend.Test{
			Schema: mustParseGraphQLSchema(t, db),
			Query: fmt.Sprintf(`
				mutation {
					cancelPermissionsSyncJob(
						job: "%s",
						reason: "because"
					)
				}
			`, marshalPermissionsSyncJobID(1)),
			ExpectedResult: `
				{
					"cancelPermissionsSyncJob": "SUCCESS"
				}
			`,
		})
	})

	t.Run("sync job is already dequeued", func(t *testing.T) {
		graphqlbackend.RunTest(t, &graphqlbackend.Test{
			Schema: mustParseGraphQLSchema(t, db),
			Query: fmt.Sprintf(`
				mutation {
					cancelPermissionsSyncJob(
						job: "%s",
						reason: "cause"
					)
				}
			`, marshalPermissionsSyncJobID(42)),
			ExpectedResult: `
				{
					"cancelPermissionsSyncJob": "NOT_FOUND"
				}
			`,
		})
	})
}

func TestResolver_AuthorizedUserRepositories(t *testing.T) {
	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := (&Resolver{db: db}).AuthorizedUserRepositories(ctx, &graphqlbackend.AuthorizedRepoArgs{})
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	users := dbmocks.NewStrictMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	users.GetByVerifiedEmailFunc.SetDefaultHook(func(_ context.Context, email string) (*types.User, error) {
		if email == "alice@example.com" {
			return &types.User{ID: 1}, nil
		}
		return nil, database.MockUserNotFoundErr
	})
	users.GetByUsernameFunc.SetDefaultHook(func(_ context.Context, username string) (*types.User, error) {
		if username == "alice" {
			return &types.User{ID: 1}, nil
		}
		return nil, database.MockUserNotFoundErr
	})

	repos := dbmocks.NewStrictMockRepoStore()
	repos.GetByIDsFunc.SetDefaultHook(func(_ context.Context, ids ...api.RepoID) ([]*types.Repo, error) {
		repos := make([]*types.Repo, len(ids))
		for i, id := range ids {
			repos[i] = &types.Repo{ID: id}
		}
		return repos, nil
	})

	perms := dbmocks.NewStrictMockPermsStore()
	perms.LoadUserPermissionsFunc.SetDefaultHook(func(_ context.Context, userID int32) ([]authz.Permission, error) {
		return []authz.Permission{{
			UserID: userID,
			RepoID: 1,
		}}, nil
	})
	perms.LoadUserPendingPermissionsFunc.SetDefaultHook(func(_ context.Context, p *authz.UserPendingPermissions) error {
		p.IDs = map[int32]struct{}{2: {}, 3: {}, 4: {}, 5: {}}
		return nil
	})

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.ReposFunc.SetDefaultReturn(repos)
	db.PermsFunc.SetDefaultReturn(perms)

	tests := []struct {
		name     string
		gqlTests []*graphqlbackend.Test
	}{
		{
			name: "check authorized repos via email",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				{
					authorizedUserRepositories(
						first: 10,
						email: "alice@example.com") {
						nodes {
							id
						}
					}
				}
			`,
					ExpectedResult: fmt.Sprintf(`
				{
					"authorizedUserRepositories": {
						"nodes": [
							{"id":"%s"}
						]
    				}
				}
			`, graphqlbackend.MarshalRepositoryID(1)),
				},
			},
		},
		{
			name: "check authorized repos via username",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				{
					authorizedUserRepositories(
						first: 10,
						username: "alice") {
						nodes {
							id
						}
					}
				}
			`,
					ExpectedResult: fmt.Sprintf(`
				{
					"authorizedUserRepositories": {
						"nodes": [
							{"id":"%s"}
						]
    				}
				}
			`, graphqlbackend.MarshalRepositoryID(1)),
				},
			},
		},
		{
			name: "check pending authorized repos via email",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				{
					authorizedUserRepositories(
						first: 10,
						email: "bob@example.com") {
						nodes {
							id
						}
					}
				}
			`,
					ExpectedResult: fmt.Sprintf(`
				{
					"authorizedUserRepositories": {
						"nodes": [
							{"id":"%s"},{"id":"%s"},{"id":"%s"},{"id":"%s"}
						]
    				}
				}
			`, graphqlbackend.MarshalRepositoryID(2), graphqlbackend.MarshalRepositoryID(3), graphqlbackend.MarshalRepositoryID(4), graphqlbackend.MarshalRepositoryID(5)),
				},
			},
		},
		{
			name: "check pending authorized repos via username",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				{
					authorizedUserRepositories(
						first: 10,
						username: "bob") {
						nodes {
							id
						}
					}
				}
			`,
					ExpectedResult: fmt.Sprintf(`
				{
					"authorizedUserRepositories": {
						"nodes": [
							{"id":"%s"},{"id":"%s"},{"id":"%s"},{"id":"%s"}
						]
    				}
				}
			`, graphqlbackend.MarshalRepositoryID(2), graphqlbackend.MarshalRepositoryID(3), graphqlbackend.MarshalRepositoryID(4), graphqlbackend.MarshalRepositoryID(5)),
				},
			},
		},
		{
			name: "check pending authorized repos via username with pagination, page 1",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: fmt.Sprintf(`
				{
					authorizedUserRepositories(
						first: 2,
						after: "%s",
						username: "bob") {
						nodes {
							id
						}
					}
				}
			`, graphqlbackend.MarshalRepositoryID(2)),
					ExpectedResult: fmt.Sprintf(`
				{
					"authorizedUserRepositories": {
						"nodes": [
							{"id":"%s"},{"id":"%s"}
						]
                    }
				}
			`, graphqlbackend.MarshalRepositoryID(3), graphqlbackend.MarshalRepositoryID(4)),
				},
			},
		},
		{
			name: "check pending authorized repos via username with pagination, page 2",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: fmt.Sprintf(`
				{
					authorizedUserRepositories(
						first: 2,
						after: "%s",
						username: "bob") {
						nodes {
							id
						}
					}
				}
			`, graphqlbackend.MarshalRepositoryID(4)),
					ExpectedResult: fmt.Sprintf(`
				{
					"authorizedUserRepositories": {
						"nodes": [
							{"id":"%s"}
						]
    				}
				}
			`, graphqlbackend.MarshalRepositoryID(5)),
				},
			},
		},
		{
			name: "check pending authorized repos via username given no IDs after, after ID, return empty",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: fmt.Sprintf(`
						{
							authorizedUserRepositories(
								first: 2,
								after: "%s",
								username: "bob") {
								nodes {
									id
								}
							}
						}
					`, graphqlbackend.MarshalRepositoryID(5)),
					ExpectedResult: `
				{
					"authorizedUserRepositories": {
						"nodes": []
    				}
				}
			`,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graphqlbackend.RunTests(t, test.gqlTests)
		})
	}
}

func TestResolver_UsersWithPendingPermissions(t *testing.T) {

	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := (&Resolver{db: db}).UsersWithPendingPermissions(ctx)
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	users := dbmocks.NewStrictMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

	perms := dbmocks.NewStrictMockPermsStore()
	perms.ListPendingUsersFunc.SetDefaultReturn([]string{"alice", "bob"}, nil)

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.PermsFunc.SetDefaultReturn(perms)

	tests := []struct {
		name     string
		gqlTests []*graphqlbackend.Test
	}{
		{
			name: "list pending users with their bind IDs",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				{
					usersWithPendingPermissions
				}
			`,
					ExpectedResult: `
				{
					"usersWithPendingPermissions": [
						"alice",
						"bob"
					]
				}
			`,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graphqlbackend.RunTests(t, test.gqlTests)
		})
	}
}

func TestResolver_AuthzProviderTypes(t *testing.T) {
	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := (&Resolver{db: db}).AuthzProviderTypes(ctx)
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	t.Run("get authz provider types", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{
			SiteAdmin: true,
		}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		ghProvider, err := github.NewProvider("https://github.com", github.ProviderOptions{GitHubURL: mustURL(t, "https://github.com")})
		require.NoError(t, err)
		authz.SetProviders(false, []authz.Provider{ghProvider})
		result, err := (&Resolver{db: db}).AuthzProviderTypes(ctx)
		assert.NoError(t, err)
		assert.Equal(t, []string{"github"}, result)
	})
}

func mustURL(t *testing.T, u string) *url.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestResolver_AuthorizedUsers(t *testing.T) {
	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := (&Resolver{db: db}).AuthorizedUsers(ctx, &graphqlbackend.RepoAuthorizedUserArgs{})
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	tests := []struct {
		name                   string
		usersWithAuthorization []int32
		gqlTests               []*graphqlbackend.Test
	}{
		{
			name:                   "no authorized users",
			usersWithAuthorization: []int32{},
			gqlTests: []*graphqlbackend.Test{
				{
					Query: `
				{
					repository(name: "github.com/owner/repo") {
						authorizedUsers(first: 10) {
							nodes {
								id
							}
						}
					}
				}
			`,
					ExpectedResult: `
				{
					"repository": {
						"authorizedUsers": {
							"nodes":[]
						}
					}
				}
			`,
				},
			},
		},
		{
			name:                   "get authorized users",
			usersWithAuthorization: []int32{1, 2, 3, 4, 5},
			gqlTests: []*graphqlbackend.Test{
				{
					Query: `
				{
					repository(name: "github.com/owner/repo") {
						authorizedUsers(first: 10) {
							nodes {
								id
							}
						}
					}
				}
			`,
					ExpectedResult: fmt.Sprintf(`
				{
					"repository": {
						"authorizedUsers": {
							"nodes":[
								{"id":"%s"},{"id":"%s"},{"id":"%s"},{"id":"%s"},{"id":"%s"}
							]
						}
    				}
				}
			`, graphqlbackend.MarshalUserID(1), graphqlbackend.MarshalUserID(2), graphqlbackend.MarshalUserID(3), graphqlbackend.MarshalUserID(4), graphqlbackend.MarshalUserID(5)),
				},
			},
		},
		{
			name:                   "get authorized users with pagination, page 1",
			usersWithAuthorization: []int32{1, 2, 3, 4, 5},
			gqlTests: []*graphqlbackend.Test{
				{
					Query: fmt.Sprintf(`
{
					repository(name: "github.com/owner/repo") {
						authorizedUsers(
							first: 2,
							after: "%s") {
							nodes {
								id
							}
						}
					}
				}
			`, graphqlbackend.MarshalUserID(1)),
					ExpectedResult: fmt.Sprintf(`
				{
					"repository": {
						"authorizedUsers": {
							"nodes":[
								{"id":"%s"},{"id":"%s"}
							]
						}
    				}
				}
			`, graphqlbackend.MarshalUserID(2), graphqlbackend.MarshalUserID(3)),
				},
			},
		},
		{
			name:                   "get authorized users with pagination, page 2",
			usersWithAuthorization: []int32{1, 2, 3, 4, 5},
			gqlTests: []*graphqlbackend.Test{
				{
					Query: fmt.Sprintf(`
{
					repository(name: "github.com/owner/repo") {
						authorizedUsers(
							first: 2,
							after: "%s") {
							nodes {
								id
							}
						}
					}
				}
			`, graphqlbackend.MarshalUserID(3)),
					ExpectedResult: fmt.Sprintf(`
				{
					"repository": {
						"authorizedUsers": {
							"nodes":[
								{"id":"%s"},{"id":"%s"}
							]
						}
    				}
				}
			`, graphqlbackend.MarshalUserID(4), graphqlbackend.MarshalUserID(5)),
				},
			},
		},
		{
			name:                   "get authorized users given no IDs after, after ID, return empty",
			usersWithAuthorization: []int32{1, 2, 3, 4, 5},
			gqlTests: []*graphqlbackend.Test{
				{
					Query: fmt.Sprintf(`
{
					repository(name: "github.com/owner/repo") {
						authorizedUsers(
							first: 2,
							after: "%s") {
							nodes {
								id
							}
						}
					}
				}
			`, graphqlbackend.MarshalUserID(5)),
					ExpectedResult: `
				{
					"repository": {
						"authorizedUsers": {
							"nodes":[]
						}
                    }
				}
			`,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := dbmocks.NewStrictMockUserStore()
			users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
			users.ListFunc.SetDefaultHook(func(_ context.Context, opt *database.UsersListOptions) ([]*types.User, error) {
				users := make([]*types.User, len(opt.UserIDs))
				for i, id := range opt.UserIDs {
					users[i] = &types.User{ID: id}
				}
				return users, nil
			})

			repos := dbmocks.NewStrictMockRepoStore()
			repos.GetByNameFunc.SetDefaultHook(func(_ context.Context, repo api.RepoName) (*types.Repo, error) {
				return &types.Repo{ID: 1, Name: repo}, nil
			})
			repos.GetFunc.SetDefaultHook(func(_ context.Context, id api.RepoID) (*types.Repo, error) {
				return &types.Repo{ID: id}, nil
			})

			perms := dbmocks.NewStrictMockPermsStore()
			perms.LoadRepoPermissionsFunc.SetDefaultHook(func(_ context.Context, repoID int32) ([]authz.Permission, error) {
				permissions := make([]authz.Permission, len(test.usersWithAuthorization))
				for i, userID := range test.usersWithAuthorization {
					permissions[i] = authz.Permission{
						UserID: userID,
						RepoID: repoID,
					}
				}

				return permissions, nil
			})

			db := dbmocks.NewStrictMockDB()
			db.UsersFunc.SetDefaultReturn(users)
			db.ReposFunc.SetDefaultReturn(repos)
			db.PermsFunc.SetDefaultReturn(perms)

			for _, gqlTest := range test.gqlTests {
				gqlTest.Schema = mustParseGraphQLSchema(t, db)
			}
			graphqlbackend.RunTests(t, test.gqlTests)
		})
	}
}

func TestResolver_RepositoryPermissionsInfo(t *testing.T) {
	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := (&Resolver{db: db}).RepositoryPermissionsInfo(ctx, graphqlbackend.MarshalRepositoryID(1))
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	users := dbmocks.NewStrictMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

	repos := dbmocks.NewStrictMockRepoStore()
	repos.GetByNameFunc.SetDefaultHook(func(_ context.Context, repo api.RepoName) (*types.Repo, error) {
		return &types.Repo{ID: 1, Name: repo}, nil
	})
	repos.GetFunc.SetDefaultHook(func(_ context.Context, id api.RepoID) (*types.Repo, error) {
		return &types.Repo{ID: id}, nil
	})

	perms := dbmocks.NewStrictMockPermsStore()
	perms.LoadRepoPermissionsFunc.SetDefaultHook(func(_ context.Context, repoID int32) ([]authz.Permission, error) {
		return []authz.Permission{{RepoID: repoID, UserID: 42, UpdatedAt: clock()}}, nil
	})
	perms.IsRepoUnrestrictedFunc.SetDefaultReturn(false, nil)
	perms.ListRepoPermissionsFunc.SetDefaultReturn([]*database.RepoPermission{{User: &types.User{ID: 42}}}, nil)

	syncJobs := dbmocks.NewStrictMockPermissionSyncJobStore()
	syncJobs.GetLatestFinishedSyncJobFunc.SetDefaultReturn(&database.PermissionSyncJob{FinishedAt: clock()}, nil)

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.ReposFunc.SetDefaultReturn(repos)
	db.PermsFunc.SetDefaultReturn(perms)
	db.PermissionSyncJobsFunc.SetDefaultReturn(syncJobs)

	tests := []struct {
		name     string
		gqlTests []*graphqlbackend.Test
	}{
		{
			name: "get permissions information",
			gqlTests: []*graphqlbackend.Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				{
					repository(name: "github.com/owner/repo") {
						permissionsInfo {
							permissions
							syncedAt
							updatedAt
							unrestricted
							users(first: 1) {
								nodes {
									id
								}
							}
						}
					}
				}
			`,
					ExpectedResult: fmt.Sprintf(`
				{
					"repository": {
						"permissionsInfo": {
							"permissions": ["READ"],
							"syncedAt": "%[1]s",
							"updatedAt": "%[1]s",
							"unrestricted": false,
							"users": {
								"nodes": [
									{
										"id": "VXNlcjo0Mg=="
									}
								]
							}
						}
    				}
				}
			`, clock().Format(time.RFC3339)),
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graphqlbackend.RunTests(t, test.gqlTests)
		})
	}
}

func TestResolver_UserPermissionsInfo(t *testing.T) {
	t.Run("authenticated as non-admin, asking not for self", func(t *testing.T) {
		user := &types.User{ID: 42}

		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(user, nil)
		users.GetByIDFunc.SetDefaultReturn(user, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: user.ID})
		result, err := (&Resolver{db: db}).UserPermissionsInfo(ctx, graphqlbackend.MarshalUserID(1))
		if want := auth.ErrMustBeSiteAdminOrSameUser; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	t.Run("authenticated as non-admin, asking for self succeeds", func(t *testing.T) {
		user := &types.User{ID: 42}

		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(user, nil)
		users.GetByIDFunc.SetDefaultReturn(user, nil)

		perms := dbmocks.NewStrictMockPermsStore()
		perms.LoadUserPermissionsFunc.SetDefaultReturn([]authz.Permission{}, nil)

		syncJobs := dbmocks.NewStrictMockPermissionSyncJobStore()
		syncJobs.GetLatestFinishedSyncJobFunc.SetDefaultReturn(nil, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.PermsFunc.SetDefaultReturn(perms)
		db.PermissionSyncJobsFunc.SetDefaultReturn(syncJobs)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: user.ID})
		result, err := (&Resolver{db: db}).UserPermissionsInfo(ctx, graphqlbackend.MarshalUserID(user.ID))
		if err != nil {
			t.Errorf("err: want nil but got %v", err)
		}
		if result == nil {
			t.Errorf("result: want non-nil but got nil")
		}
	})

	userID := int32(9999)
	users := dbmocks.NewStrictMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: userID, SiteAdmin: true}, nil)
	users.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.User, error) {
		return &types.User{ID: id}, nil
	})

	perms := dbmocks.NewStrictMockPermsStore()
	perms.LoadUserPermissionsFunc.SetDefaultReturn([]authz.Permission{{UpdatedAt: clock(), Source: authz.SourceUserSync}}, nil)
	perms.ListUserPermissionsFunc.SetDefaultReturn([]*database.UserPermission{{Repo: &types.Repo{ID: 42}}}, nil)

	syncJobs := dbmocks.NewStrictMockPermissionSyncJobStore()
	syncJobs.GetLatestFinishedSyncJobFunc.SetDefaultReturn(&database.PermissionSyncJob{FinishedAt: clock()}, nil)

	repos := dbmocks.NewStrictMockRepoStore()
	repos.GetByNameFunc.SetDefaultHook(func(_ context.Context, name api.RepoName) (*types.Repo, error) {
		return &types.Repo{Name: name}, nil
	})

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.PermsFunc.SetDefaultReturn(perms)
	db.ReposFunc.SetDefaultReturn(repos)
	db.PermissionSyncJobsFunc.SetDefaultReturn(syncJobs)

	tests := []struct {
		name     string
		gqlTests []*graphqlbackend.Test
	}{
		{
			name: "get permissions information",
			gqlTests: []*graphqlbackend.Test{
				{
					Context: actor.WithActor(context.Background(), &actor.Actor{UID: userID}),
					Schema:  mustParseGraphQLSchema(t, db),
					Query: `
				{
					currentUser {
						permissionsInfo {
							permissions
							updatedAt
							source
							repositories(first: 1) {
								nodes {
									id
								}
							}
						}
					}
				}
			`,
					ExpectedResult: fmt.Sprintf(`
				{
					"currentUser": {
						"permissionsInfo": {
							"permissions": ["READ"],
							"updatedAt": "%[1]s",
							"source": "%[2]s",
							"repositories": {
								"nodes": [
									{
										"id": "UmVwb3NpdG9yeTo0Mg=="
									}
								]
							}
						}
    				}
				}
			`, clock().Format(time.RFC3339), authz.SourceUserSync.ToGraphQL()),
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graphqlbackend.RunTests(t, test.gqlTests)
		})
	}
}

func TestResolver_SetSubRepositoryPermissionsForUsers(t *testing.T) {
	t.Cleanup(licensing.TestingSkipFeatureChecks())

	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		subrepos := dbmocks.NewStrictMockSubRepoPermsStore()
		subrepos.UpsertFunc.SetDefaultReturn(nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.SubRepoPermsFunc.SetDefaultReturn(subrepos)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := (&Resolver{db: db}).SetSubRepositoryPermissionsForUsers(ctx, &graphqlbackend.SubRepoPermsArgs{})
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	t.Run("set sub-repo perms", func(t *testing.T) {
		usersStore := dbmocks.NewStrictMockUserStore()
		usersStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{
			ID:        1,
			SiteAdmin: true,
		}, nil)
		usersStore.GetByUsernameFunc.SetDefaultReturn(&types.User{ID: 1, Username: "foo"}, nil)
		usersStore.GetByVerifiedEmailFunc.SetDefaultReturn(&types.User{ID: 1, Username: "foo"}, nil)

		subReposStore := dbmocks.NewStrictMockSubRepoPermsStore()
		subReposStore.UpsertFunc.SetDefaultReturn(nil)

		reposStore := dbmocks.NewStrictMockRepoStore()
		reposStore.GetFunc.SetDefaultReturn(&types.Repo{ID: 1, Name: "foo"}, nil)

		db := dbmocks.NewStrictMockDB()
		db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
			return f(db)
		})
		db.UsersFunc.SetDefaultReturn(usersStore)
		db.SubRepoPermsFunc.SetDefaultReturn(subReposStore)
		db.ReposFunc.SetDefaultReturn(reposStore)

		perms := dbmocks.NewStrictMockPermsStore()
		perms.TransactFunc.SetDefaultReturn(perms, nil)
		perms.DoneFunc.SetDefaultReturn(nil)
		perms.MapUsersFunc.SetDefaultReturn(map[string]int32{"alice": 1}, nil)
		db.PermsFunc.SetDefaultReturn(perms)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		tests := []*graphqlbackend.Test{
			{
				Context: ctx,
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
						mutation {
  setSubRepositoryPermissionsForUsers(
    repository: "UmVwb3NpdG9yeTox"
    userPermissions: [{bindID: "alice", pathIncludes: ["/*"], pathExcludes: ["/*_test.go"]}]
  ) {
    alwaysNil
  }
}
					`,
				ExpectedResult: `
						{
							"setSubRepositoryPermissionsForUsers": {
								"alwaysNil": null
							}
						}
					`,
			},
			{
				Context: ctx,
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
						mutation {
  setSubRepositoryPermissionsForUsers(
    repository: "UmVwb3NpdG9yeTox"
    userPermissions: [{bindID: "alice", pathIncludes: ["/*"], pathExcludes: ["/*_test.go"], paths: ["-/*_test.go", "/*"]}]
  ) {
    alwaysNil
  }
}
					`,
				ExpectedResult: `
						{
							"setSubRepositoryPermissionsForUsers": {
								"alwaysNil": null
							}
						}
					`,
			},
			{
				Context: ctx,
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
						mutation {
  setSubRepositoryPermissionsForUsers(
    repository: "UmVwb3NpdG9yeTox"
    userPermissions: [{bindID: "alice", paths: ["-/*_test.go", "/*"]}]
  ) {
    alwaysNil
  }
}
					`,
				ExpectedResult: `
						{
							"setSubRepositoryPermissionsForUsers": {
								"alwaysNil": null
							}
						}
					`,
			},
			{
				Context: ctx,
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
						mutation {
  setSubRepositoryPermissionsForUsers(
    repository: "UmVwb3NpdG9yeTox"
    userPermissions: [{bindID: "alice", pathIncludes: ["/*_test.go"]}]
  ) {
    alwaysNil
  }
}
					`,
				ExpectedErrors: []*gqlerrors.QueryError{
					{
						Message: "either both pathIncludes and pathExcludes needs to be set, or paths needs to be set",
						Path:    []any{"setSubRepositoryPermissionsForUsers"},
					},
				},
				ExpectedResult: "null",
			},
		}

		graphqlbackend.RunTests(t, tests)

		// Assert that we actually tried to store perms
		h := subReposStore.UpsertFunc.History()
		if len(h) != 3 {
			t.Fatalf("Wanted 3 calls, got %d", len(h))
		}
	})
}

func TestResolver_BitbucketProjectPermissionJobs(t *testing.T) {
	t.Run("disabled on dotcom", func(t *testing.T) {
		envvar.MockSourcegraphDotComMode(true)

		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.BitbucketProjectPermissionJobs(ctx, nil)

		require.ErrorIs(t, err, errDisabledSourcegraphDotCom)
		require.Nil(t, result)

		// Reset the env var for other tests.
		envvar.MockSourcegraphDotComMode(false)
	})

	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.BitbucketProjectPermissionJobs(ctx, nil)

		require.ErrorIs(t, err, auth.ErrMustBeSiteAdmin)
		require.Nil(t, result)
	})

	t.Run("incorrect job status", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		test := &graphqlbackend.Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
query {
  bitbucketProjectPermissionJobs(status:"queueueueud") {
    totalCount,
    nodes {
      InternalJobID,
      State,
      Unrestricted,
      Permissions{
        bindID,
        permission
      }
    }
  }
}
					`,
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "Please provide one of the following job statuses: queued, processing, completed, canceled, errored, failed",
					Path:    []any{"bitbucketProjectPermissionJobs"},
				},
			},
		}

		graphqlbackend.RunTests(t, []*graphqlbackend.Test{test})
	})

	t.Run("all job fields successfully returned", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		bbProjects := dbmocks.NewMockBitbucketProjectPermissionsStore()
		entry := executor.ExecutionLogEntry{Key: "key", Command: []string{"command"}, StartTime: mustParseTime("2020-01-06"), ExitCode: pointers.Ptr(1), Out: "out", DurationMs: pointers.Ptr(1)}
		bbProjects.ListJobsFunc.SetDefaultReturn([]*types.BitbucketProjectPermissionJob{
			{
				ID:                1,
				State:             "queued",
				FailureMessage:    pointers.Ptr("failure massage"),
				QueuedAt:          mustParseTime("2020-01-01"),
				StartedAt:         pointers.Ptr(mustParseTime("2020-01-01")),
				FinishedAt:        pointers.Ptr(mustParseTime("2020-01-01")),
				ProcessAfter:      pointers.Ptr(mustParseTime("2020-01-01")),
				NumResets:         1,
				NumFailures:       2,
				LastHeartbeatAt:   mustParseTime("2020-01-05"),
				ExecutionLogs:     []types.ExecutionLogEntry{&entry},
				WorkerHostname:    "worker-hostname",
				ProjectKey:        "project-key",
				ExternalServiceID: 1,
				Permissions:       []types.UserPermission{{Permission: "read", BindID: "ayy@lmao.com"}},
				Unrestricted:      false,
			},
		}, nil)
		db.BitbucketProjectPermissionsFunc.SetDefaultReturn(bbProjects)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		test := &graphqlbackend.Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
query {
  bitbucketProjectPermissionJobs(count:1) {
    totalCount,
    nodes {
      InternalJobID,
      State,
      StartedAt,
      FailureMessage,
      QueuedAt,
      StartedAt,
      FinishedAt,
      ProcessAfter,
      NumResets,
      NumFailures,
      ProjectKey,
      ExternalServiceID,
      Unrestricted,
      Permissions{
        bindID,
        permission
      }
    }
  }
}
					`,
			ExpectedResult: `
{
  "bitbucketProjectPermissionJobs": {
    "totalCount": 1,
    "nodes": [
	  {
	    "InternalJobID": 1,
	    "State": "queued",
	    "StartedAt": "2020-01-01T00:00:00Z",
	    "FailureMessage": "failure massage",
	    "QueuedAt": "2020-01-01T00:00:00Z",
	    "FinishedAt": "2020-01-01T00:00:00Z",
	    "ProcessAfter": "2020-01-01T00:00:00Z",
	    "NumResets": 1,
	    "NumFailures": 2,
	    "ProjectKey": "project-key",
	    "ExternalServiceID": "RXh0ZXJuYWxTZXJ2aWNlOjE=",
	    "Unrestricted": false,
	    "Permissions": [
		  {
		    "bindID": "ayy@lmao.com",
		    "permission": "READ"
		  }
	    ]
	  }
    ]
  }
}
`,
		}

		graphqlbackend.RunTests(t, []*graphqlbackend.Test{test})
	})
}

func TestResolverPermissionsSyncJobs(t *testing.T) {
	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByIDFunc.SetDefaultReturn(&types.User{}, nil)
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := r.PermissionsSyncJobs(ctx, graphqlbackend.ListPermissionsSyncJobsArgs{})

		require.ErrorIs(t, err, auth.ErrMustBeSiteAdmin)
		require.Nil(t, result)
	})

	t.Run("authenticated as non-admin with current user's ID as userID filter", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1}, nil)
		users.GetByIDFunc.SetDefaultReturn(&types.User{ID: 1}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		userID := graphqlbackend.MarshalUserID(1)
		_, err := r.PermissionsSyncJobs(ctx, graphqlbackend.ListPermissionsSyncJobsArgs{UserID: &userID})

		require.NoError(t, err)
	})

	t.Run("authenticated as non-admin with different user's ID as userID filter", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1}, nil)
		users.GetByIDFunc.SetDefaultReturn(&types.User{ID: 1}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		userID := graphqlbackend.MarshalUserID(2)
		_, err := r.PermissionsSyncJobs(ctx, graphqlbackend.ListPermissionsSyncJobsArgs{UserID: &userID})

		require.ErrorIs(t, err, auth.ErrMustBeSiteAdminOrSameUser)
	})

	t.Run("authenticated as admin with different user's ID as userID filter", func(t *testing.T) {
		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)
		users.GetByIDFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		r := &Resolver{db: db}

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		userID := graphqlbackend.MarshalUserID(2)
		_, err := r.PermissionsSyncJobs(ctx, graphqlbackend.ListPermissionsSyncJobsArgs{UserID: &userID})

		require.NoError(t, err)
	})

	// Mocking users database queries.
	users := dbmocks.NewStrictMockUserStore()
	returnedUser := &types.User{ID: 1, SiteAdmin: true}
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(returnedUser, nil)
	users.GetByIDFunc.SetDefaultReturn(returnedUser, nil)

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	// Mocking permission jobs database queries.
	permissionSyncJobStore := dbmocks.NewMockPermissionSyncJobStore()
	timeFormat := "2006-01-02T15:04:05Z"
	queuedAt, err := time.Parse(timeFormat, "2023-03-02T15:04:05Z")
	require.NoError(t, err)
	finishedAt, err := time.Parse(timeFormat, "2023-03-02T15:05:05Z")
	require.NoError(t, err)

	codeHostStates := database.CodeHostStatusesSet{
		{ProviderID: "1", ProviderType: "github", Status: database.CodeHostStatusSuccess, Message: "success!"},
		{ProviderID: "2", ProviderType: "gitlab", Status: database.CodeHostStatusError, Message: "error!"},
	}

	// One job has a user who triggered it, another doesn't.
	jobs := []*database.PermissionSyncJob{
		{
			ID:                 3,
			State:              "COMPLETED",
			Reason:             database.ReasonManualUserSync,
			RepositoryID:       1,
			TriggeredByUserID:  1,
			QueuedAt:           queuedAt,
			StartedAt:          queuedAt,
			FinishedAt:         finishedAt,
			NumResets:          0,
			NumFailures:        0,
			WorkerHostname:     "worker.hostname",
			Cancel:             false,
			Priority:           database.HighPriorityPermissionsSync,
			NoPerms:            false,
			InvalidateCaches:   false,
			PermissionsAdded:   1337,
			PermissionsRemoved: 42,
			PermissionsFound:   404,
			CodeHostStates:     codeHostStates,
			IsPartialSuccess:   true,
		},
		{
			ID:               4,
			State:            "FAILED",
			Reason:           database.ReasonUserEmailRemoved,
			RepositoryID:     1,
			QueuedAt:         queuedAt,
			StartedAt:        queuedAt,
			WorkerHostname:   "worker.hostname",
			Cancel:           false,
			Priority:         database.HighPriorityPermissionsSync,
			NoPerms:          false,
			InvalidateCaches: false,
			CodeHostStates:   codeHostStates[1:],
			IsPartialSuccess: false,
		},
	}
	permissionSyncJobStore.ListFunc.SetDefaultReturn(jobs, nil)
	permissionSyncJobStore.CountFunc.SetDefaultReturn(len(jobs), nil)
	db.PermissionSyncJobsFunc.SetDefaultReturn(permissionSyncJobStore)

	// Mocking repository database queries.
	repoStore := dbmocks.NewMockRepoStore()
	repoStore.GetFunc.SetDefaultReturn(&types.Repo{ID: 1}, nil)
	db.ReposFunc.SetDefaultReturn(repoStore)

	// Creating a resolver and validating GraphQL schema.
	r := &Resolver{db: db}
	parsedSchema, err := graphqlbackend.NewSchemaWithAuthzResolver(db, r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	t.Run("all job fields successfully returned", func(t *testing.T) {
		graphqlbackend.RunTests(t, []*graphqlbackend.Test{{
			Context: ctx,
			Schema:  parsedSchema,
			Query: `
query {
  permissionsSyncJobs(first:2) {
	totalCount
	pageInfo { hasNextPage }
    nodes {
		id
		state
		failureMessage
		reason {
			group
			reason
		}
		cancellationReason
		triggeredByUser {
			id
		}
		queuedAt
		startedAt
		finishedAt
		processAfter
		ranForMs
		numResets
		numFailures
		lastHeartbeatAt
		workerHostname
		cancel
		subject {
			... on Repository {
				id
			}
		}
		priority
		noPerms
		invalidateCaches
		permissionsAdded
		permissionsRemoved
		permissionsFound
		codeHostStates {
			providerID
			providerType
			status
			message
		}
		partialSuccess
	}
  }
}
					`,
			ExpectedResult: `
{
	"permissionsSyncJobs": {
		"nodes": [
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjM=",
				"state": "COMPLETED",
				"failureMessage": null,
				"reason": {
					"group": "MANUAL",
					"reason": "REASON_MANUAL_USER_SYNC"
				},
				"cancellationReason": null,
				"triggeredByUser": {
					"id": "VXNlcjox"
				},
				"queuedAt": "2023-03-02T15:04:05Z",
				"startedAt": "2023-03-02T15:04:05Z",
				"finishedAt": "2023-03-02T15:05:05Z",
				"processAfter": null,
				"ranForMs": 60000,
				"numResets": 0,
				"numFailures": 0,
				"lastHeartbeatAt": null,
				"workerHostname": "worker.hostname",
				"cancel": false,
				"subject": {
					"id": "UmVwb3NpdG9yeTox"
				},
				"priority": "HIGH",
				"noPerms": false,
				"invalidateCaches": false,
				"permissionsAdded": 1337,
				"permissionsRemoved": 42,
				"permissionsFound": 404,
				"codeHostStates": [
					{
						"providerID": "1",
						"providerType": "github",
						"status": "SUCCESS",
						"message": "success!"
					},
					{
						"providerID": "2",
						"providerType": "gitlab",
						"status": "ERROR",
						"message": "error!"
					}
				],
				"partialSuccess": true
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjQ=",
				"state": "FAILED",
				"failureMessage": null,
				"reason": {
					"group": "SOURCEGRAPH",
					"reason": "REASON_USER_EMAIL_REMOVED"
				},
				"cancellationReason": null,
				"triggeredByUser": null,
				"queuedAt": "2023-03-02T15:04:05Z",
				"startedAt": "2023-03-02T15:04:05Z",
				"finishedAt": null,
				"processAfter": null,
				"ranForMs": 0,
				"numResets": 0,
				"numFailures": 0,
				"lastHeartbeatAt": null,
				"workerHostname": "worker.hostname",
				"cancel": false,
				"subject": {
					"id": "UmVwb3NpdG9yeTox"
				},
				"priority": "HIGH",
				"noPerms": false,
				"invalidateCaches": false,
				"permissionsAdded": 0,
				"permissionsRemoved": 0,
				"permissionsFound": 0,
				"codeHostStates": [
					{
						"providerID": "2",
						"providerType": "gitlab",
						"status": "ERROR",
						"message": "error!"
					}
				],
				"partialSuccess": false
			}
		],
		"pageInfo": {
			"hasNextPage": false
		},
		"totalCount": 2
	}
}`,
		}})
	})
}

func TestResolverPermissionsSyncJobsFiltering(t *testing.T) {
	// Mocking users database queries.
	users := dbmocks.NewStrictMockUserStore()
	returnedUser := &types.User{ID: 1, SiteAdmin: true}
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(returnedUser, nil)
	users.GetByIDFunc.SetDefaultReturn(returnedUser, nil)

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	// Mocking permission jobs database queries.
	permissionSyncJobStore := dbmocks.NewMockPermissionSyncJobStore()

	// One job has a user who triggered it, another doesn't.
	jobs := []*database.PermissionSyncJob{
		{
			ID:     5,
			State:  "QUEUED",
			Reason: database.ReasonGitHubUserAddedEvent,
		},
		{
			ID:     6,
			State:  "QUEUED",
			Reason: database.ReasonUserEmailRemoved,
		},
		{
			ID:     7,
			State:  "QUEUED",
			Reason: database.ReasonGitHubUserMembershipAddedEvent,
		},
		{
			ID:     8,
			State:  "COMPLETED",
			Reason: database.ReasonUserEmailVerified,
		},
		{
			ID:     9,
			State:  "COMPLETED",
			Reason: database.ReasonGitHubUserMembershipAddedEvent,
		},
		{
			ID:     10,
			State:  "COMPLETED",
			Reason: database.ReasonGitHubUserAddedEvent,
		},
	}

	doFilter := func(jobs []*database.PermissionSyncJob, opts database.ListPermissionSyncJobOpts) []*database.PermissionSyncJob {
		filtered := make([]*database.PermissionSyncJob, 0, len(jobs))
		for _, job := range jobs {
			if opts.ReasonGroup != "" {
				if job.Reason.ResolveGroup() != opts.ReasonGroup {
					continue
				}
			}
			if opts.State != "" {
				if job.State != opts.State {
					continue
				}
			}
			filtered = append(filtered, job)
		}
		return filtered
	}

	permissionSyncJobStore.ListFunc.SetDefaultHook(func(_ context.Context, opts database.ListPermissionSyncJobOpts) ([]*database.PermissionSyncJob, error) {
		filtered := doFilter(jobs, opts)

		if opts.PaginationArgs.First != nil && len(filtered) > *opts.PaginationArgs.First {
			filtered = filtered[:*opts.PaginationArgs.First+1]
		}
		return filtered, nil
	})
	permissionSyncJobStore.CountFunc.SetDefaultHook(func(_ context.Context, opts database.ListPermissionSyncJobOpts) (int, error) {
		return len(doFilter(jobs, opts)), nil
	})
	db.PermissionSyncJobsFunc.SetDefaultReturn(permissionSyncJobStore)

	// Mocking repository database queries.
	repoStore := dbmocks.NewMockRepoStore()
	repoStore.GetFunc.SetDefaultReturn(&types.Repo{ID: 1}, nil)
	db.ReposFunc.SetDefaultReturn(repoStore)

	// Creating a resolver and validating GraphQL schema.
	r := &Resolver{db: db}
	parsedSchema, err := graphqlbackend.NewSchemaWithAuthzResolver(db, r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	t.Run("filter by reason group", func(t *testing.T) {
		graphqlbackend.RunTests(t, []*graphqlbackend.Test{{
			Context: ctx,
			Schema:  parsedSchema,
			Query: `
query {
  permissionsSyncJobs(first: 10, reasonGroup: SOURCEGRAPH) {
	totalCount
	nodes {
		id
	}
  }
}
					`,
			ExpectedResult: `
{
	"permissionsSyncJobs": {
		"totalCount": 2,
		"nodes": [
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjY="
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjg="
			}
		]
	}
}`,
		}})
	})

	t.Run("filter by state", func(t *testing.T) {
		graphqlbackend.RunTests(t, []*graphqlbackend.Test{{
			Context: ctx,
			Schema:  parsedSchema,
			Query: `
query {
  permissionsSyncJobs(first: 10, state:COMPLETED) {
	totalCount
	nodes {
		id
	}
  }
}
					`,
			ExpectedResult: `
{
	"permissionsSyncJobs": {
		"totalCount": 3,
		"nodes": [
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjg="
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjk="
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjEw"
			}
		]
	}
}`,
		}})
	})

	t.Run("filter by reason group and state", func(t *testing.T) {
		graphqlbackend.RunTests(t, []*graphqlbackend.Test{{
			Context: ctx,
			Schema:  parsedSchema,
			Query: `
query {
  permissionsSyncJobs(first: 10, reasonGroup: WEBHOOK, state: COMPLETED) {
	totalCount
	nodes {
		id
	}
  }
}
					`,
			ExpectedResult: `
{
	"permissionsSyncJobs": {
		"totalCount": 2,
		"nodes": [
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjk="
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjEw"
			}
		]
	}
}`,
		}})
	})
}

func TestResolverPermissionsSyncJobsSearching(t *testing.T) {
	// Mocking users database queries.
	users := dbmocks.NewStrictMockUserStore()
	returnedUser := &types.User{ID: 1, SiteAdmin: true}
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(returnedUser, nil)
	users.GetByIDFunc.SetDefaultReturn(returnedUser, nil)

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	// Mocking permission jobs database queries.
	permissionSyncJobStore := dbmocks.NewMockPermissionSyncJobStore()

	// One job has a user who triggered it, another doesn't.
	jobs := []*database.PermissionSyncJob{
		{
			ID:     1,
			State:  "QUEUED",
			Reason: database.ReasonGitHubTeamRemovedFromRepoEvent,
		},
		{
			ID:     2,
			State:  "QUEUED",
			Reason: database.ReasonManualRepoSync,
		},
		{
			ID:     3,
			State:  "QUEUED",
			Reason: database.ReasonRepoOutdatedPermissions,
		},
		{
			ID:     4,
			State:  "COMPLETED",
			Reason: database.ReasonRepoNoPermissions,
		},
		{
			ID:     5,
			State:  "COMPLETED",
			Reason: database.ReasonGitHubTeamRemovedFromRepoEvent,
		},
		{
			ID:     6,
			State:  "COMPLETED",
			Reason: database.ReasonManualRepoSync,
		},
	}

	permissionSyncJobStore.ListFunc.SetDefaultHook(func(_ context.Context, opts database.ListPermissionSyncJobOpts) ([]*database.PermissionSyncJob, error) {
		if opts.SearchType == database.PermissionsSyncSearchTypeRepo && opts.Query == "repo" {
			return jobs[:4], nil
		}
		if opts.SearchType == database.PermissionsSyncSearchTypeUser && opts.Query == "user" {
			return jobs[3:], nil
		}
		return []*database.PermissionSyncJob{}, nil
	})
	permissionSyncJobStore.CountFunc.SetDefaultReturn(len(jobs)/2, nil)
	db.PermissionSyncJobsFunc.SetDefaultReturn(permissionSyncJobStore)

	// Mocking repository database queries.
	repoStore := dbmocks.NewMockRepoStore()
	repoStore.GetFunc.SetDefaultReturn(&types.Repo{ID: 1}, nil)
	db.ReposFunc.SetDefaultReturn(repoStore)

	// Creating a resolver and validating GraphQL schema.
	r := &Resolver{db: db}
	parsedSchema, err := graphqlbackend.NewSchemaWithAuthzResolver(db, r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	t.Run("search by repo name", func(t *testing.T) {
		graphqlbackend.RunTests(t, []*graphqlbackend.Test{{
			Context: ctx,
			Schema:  parsedSchema,
			Query: `
query {
  permissionsSyncJobs(first: 3, query: "repo", searchType: REPOSITORY) {
	totalCount
	nodes {
		id
	}
  }
}
					`,
			ExpectedResult: `
{
	"permissionsSyncJobs": {
		"totalCount": 3,
		"nodes": [
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjE="
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjI="
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjM="
			}
		]
	}
}`,
		}})
	})

	t.Run("search by user name", func(t *testing.T) {
		graphqlbackend.RunTests(t, []*graphqlbackend.Test{{
			Context: ctx,
			Schema:  parsedSchema,
			Query: `
query {
  permissionsSyncJobs(first: 3, query: "user", searchType: USER) {
	totalCount
	nodes {
		id
	}
  }
}
					`,
			ExpectedResult: `
{
	"permissionsSyncJobs": {
		"totalCount": 3,
		"nodes": [
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjQ="
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjU="
			},
			{
				"id": "UGVybWlzc2lvbnNTeW5jSm9iOjY="
			}
		]
	}
}`,
		}})
	})
}

func mustParseTime(v string) time.Time {
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		panic(err)
	}
	return t
}

func TestResolver_PermissionsSyncingStats(t *testing.T) {
	t.Run("authenticated as non-admin", func(t *testing.T) {
		user := &types.User{ID: 42}

		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(user, nil)
		users.GetByIDFunc.SetDefaultReturn(user, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: user.ID})
		_, err := (&Resolver{db: db}).PermissionsSyncingStats(ctx)
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
	})

	t.Run("successfully query all permissionsSyncingStats", func(t *testing.T) {
		user := &types.User{ID: 42, SiteAdmin: true}

		users := dbmocks.NewStrictMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(user, nil)
		users.GetByIDFunc.SetDefaultReturn(user, nil)

		permissionSyncJobStore := dbmocks.NewMockPermissionSyncJobStore()
		permissionSyncJobStore.CountFunc.SetDefaultReturn(2, nil)
		permissionSyncJobStore.CountUsersWithFailingSyncJobFunc.SetDefaultReturn(3, nil)
		permissionSyncJobStore.CountReposWithFailingSyncJobFunc.SetDefaultReturn(4, nil)

		perms := dbmocks.NewStrictMockPermsStore()
		perms.CountUsersWithNoPermsFunc.SetDefaultReturn(5, nil)
		perms.CountReposWithNoPermsFunc.SetDefaultReturn(6, nil)
		perms.CountUsersWithStalePermsFunc.SetDefaultReturn(7, nil)
		perms.CountReposWithStalePermsFunc.SetDefaultReturn(8, nil)

		db := dbmocks.NewStrictMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.PermissionSyncJobsFunc.SetDefaultReturn(permissionSyncJobStore)
		db.PermsFunc.SetDefaultReturn(perms)

		gqlTests := []*graphqlbackend.Test{{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				query {
					permissionsSyncingStats {
						queueSize
						usersWithLatestJobFailing
						reposWithLatestJobFailing
						usersWithNoPermissions
						reposWithNoPermissions
						usersWithStalePermissions
						reposWithStalePermissions
					}
				}
						`,
			ExpectedResult: `
				{
					"permissionsSyncingStats": {
						"queueSize": 2,
						"usersWithLatestJobFailing": 3,
						"reposWithLatestJobFailing": 4,
						"usersWithNoPermissions": 5,
						"reposWithNoPermissions": 6,
						"usersWithStalePermissions": 7,
						"reposWithStalePermissions": 8
					}
				}
						`,
		}}

		graphqlbackend.RunTests(t, gqlTests)
	})
}
