package graphqlbackend

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

// 🚨 SECURITY: This tests that users can't create tokens for users they aren't allowed to do so for.
func TestMutation_CreateAccessToken(t *testing.T) {
	newMockAccessTokens := func(t *testing.T, wantCreatorUserID int32, wantScopes []string) database.AccessTokenStore {
		accessTokens := dbmocks.NewMockAccessTokenStore()
		accessTokens.CreateFunc.SetDefaultHook(func(_ context.Context, subjectUserID int32, scopes []string, note string, creatorUserID int32, expiresAt time.Time) (int64, string, error) {
			if want := int32(1); subjectUserID != want {
				t.Errorf("got %v, want %v", subjectUserID, want)
			}
			if !reflect.DeepEqual(scopes, wantScopes) {
				t.Errorf("got %q, want %q", scopes, wantScopes)
			}
			if want := "n"; note != want {
				t.Errorf("got %q, want %q", note, want)
			}
			if creatorUserID != wantCreatorUserID {
				t.Errorf("got %v, want %v", creatorUserID, wantCreatorUserID)
			}
			return 1, "t", nil
		})
		return accessTokens
	}

	const uid1GQLID = "VXNlcjox"
	defaultTokenDuration := int32(time.Hour.Seconds())

	t.Run("authenticated as user", func(t *testing.T) {
		accessTokens := newMockAccessTokens(t, 1, []string{authz.ScopeUserAll})
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)

		db := dbmocks.NewMockDB()
		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)

		RunTests(t, []*Test{
			{
				Context: actor.WithActor(context.Background(), &actor.Actor{UID: 1}),
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
				mutation {
					createAccessToken(user: "` + uid1GQLID + `", scopes: ["user:all"], note: "n", durationSeconds:` + strconv.Itoa(int(defaultTokenDuration)) + `) {
						id
						token
					}
				}
			`,
				ExpectedResult: `
				{
					"createAccessToken": {
						"id": "QWNjZXNzVG9rZW46MQ==",
						"token": "t"
					}
				}
			`,
			},
		})
	})

	t.Run("authenticated as user, using invalid scopes", func(t *testing.T) {
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		db := dbmocks.NewMockDB()
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{User: uid1GQLID /* no scopes */, Note: "n", DurationSeconds: &defaultTokenDuration})
		if err == nil {
			t.Error("err == nil")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("authenticated as user, expiration required not sent", func(t *testing.T) {
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		db := dbmocks.NewMockDB()
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{
					AllowNoExpiration: pointers.Ptr(false),
				},
			},
		})
		defer conf.Mock(nil)
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{User: uid1GQLID, Scopes: []string{"user:all"}, Note: "n"})
		if err == nil {
			t.Error("err == nil")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("authenticated as user, expiration in past", func(t *testing.T) {
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		db := dbmocks.NewMockDB()
		negativeDuration := int32(-1)
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{User: uid1GQLID, Scopes: []string{"user:all"}, Note: "n", DurationSeconds: &negativeDuration})
		if err == nil {
			t.Error("err == nil")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("authenticated as user, allow no expiration", func(t *testing.T) {
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		accessTokens := newMockAccessTokens(t, 1, []string{authz.ScopeUserAll})
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)

		db := dbmocks.NewMockDB()
		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)
		conf.Get().AuthAccessTokens = &schema.AuthAccessTokens{Allow: string(conf.AccessTokensAll), AllowNoExpiration: pointers.Ptr(true)}
		defer func() { conf.Get().AuthAccessTokens = nil }()
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{User: uid1GQLID, Scopes: []string{"user:all"}, Note: "n"})
		if err != nil {
			t.Errorf("got err %v, want nil", err)
		}
		if result == nil {
			t.Error("result == nil")
		}

	})

	t.Run("authenticated as user, expiration exceeds longest available option", func(t *testing.T) {
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		db := dbmocks.NewMockDB()
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{
					AllowNoExpiration:     pointers.Ptr(false),
					DefaultExpirationDays: pointers.Ptr(2),
					ExpirationOptionDays:  []int{1, 2, 3},
				},
			},
		})
		defer conf.Mock(nil)
		durationTooLongExpiration := int32(3*86400 + 1) // 3 days + 1 second
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{User: uid1GQLID, Scopes: []string{"user:all"}, Note: "n", DurationSeconds: &durationTooLongExpiration})
		if err == nil {
			t.Error("err == nil")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("authenticated as user, 0 duration", func(t *testing.T) {
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		db := dbmocks.NewMockDB()
		zero := int32(0)
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{User: uid1GQLID, Scopes: []string{"user:all"}, Note: "n", DurationSeconds: &zero})
		if err == nil {
			t.Error("err == nil")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("authenticated as user, using site-admin-only scopes", func(t *testing.T) {
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)

		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{
			User:            uid1GQLID,
			Scopes:          []string{authz.ScopeUserAll, authz.ScopeSiteAdminSudo},
			Note:            "n",
			DurationSeconds: &defaultTokenDuration,
		})
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("got err %v, want %v", err, want)
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("authenticated as site admin, using site-admin-only scopes", func(t *testing.T) {
		accessTokens := newMockAccessTokens(t, 1, []string{authz.ScopeSiteAdminSudo, authz.ScopeUserAll})
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

		db := dbmocks.NewMockDB()
		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)

		RunTests(t, []*Test{
			{
				Context: actor.WithActor(context.Background(), &actor.Actor{UID: 1}),
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
				mutation {
					createAccessToken(user: "` + uid1GQLID + `", scopes: ["user:all", "site-admin:sudo"], note: "n", durationSeconds: ` + strconv.Itoa(int(defaultTokenDuration)) + `) {
						id
						token
					}
				}
			`,
				ExpectedResult: `
				{
					"createAccessToken": {
						"id": "QWNjZXNzVG9rZW46MQ==",
						"token": "t"
					}
				}
			`,
			},
		})
	})

	t.Run("authenticated as different user who is a site-admin. Default config", func(t *testing.T) {
		const differentSiteAdminUID = 234

		accessTokens := newMockAccessTokens(t, differentSiteAdminUID, []string{authz.ScopeUserAll})
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: differentSiteAdminUID, SiteAdmin: true}, nil)

		db := dbmocks.NewMockDB()
		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)

		RunTests(t, []*Test{
			{
				Context: actor.WithActor(context.Background(), &actor.Actor{UID: differentSiteAdminUID}),
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
				mutation {
					createAccessToken(user: "` + uid1GQLID + `", scopes: ["user:all"], note: "n", durationSeconds: ` + strconv.Itoa(int(defaultTokenDuration)) + `) {
						id
						token
					}
				}
			`,
				ExpectedResult: `null`,
				ExpectedErrors: []*gqlerrors.QueryError{
					{
						Path:          []any{"createAccessToken"},
						Message:       "must be authenticated as user with id 1",
						ResolverError: &auth.InsufficientAuthorizationError{Message: fmt.Sprintf("must be authenticated as user with id %d", 1)},
					},
				},
			},
		})
	})

	t.Run("authenticated as different user who is a site-admin. Admin allowed", func(t *testing.T) {
		const differentSiteAdminUID = 234

		accessTokens := newMockAccessTokens(t, differentSiteAdminUID, []string{authz.ScopeUserAll})
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: differentSiteAdminUID, SiteAdmin: true}, nil)

		db := dbmocks.NewMockDB()
		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)

		conf.Get().AuthAccessTokens = &schema.AuthAccessTokens{Allow: string(conf.AccessTokensAdmin)}
		defer func() { conf.Get().AuthAccessTokens = nil }()

		RunTests(t, []*Test{
			{
				Context: actor.WithActor(context.Background(), &actor.Actor{UID: differentSiteAdminUID}),
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
				mutation {
					createAccessToken(user: "` + uid1GQLID + `", scopes: ["user:all"], note: "n", durationSeconds: ` + strconv.Itoa(int(defaultTokenDuration)) + `) {
						id
						token
					}
				}
			`,
				ExpectedResult: `
				{
					"createAccessToken": {
						"id": "QWNjZXNzVG9rZW46MQ==",
						"token": "t"
					}
				}
			`,
			},
		})
	})

	t.Run("unauthenticated", func(t *testing.T) {
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(nil, database.ErrNoCurrentUser)
		users.GetByIDFunc.SetDefaultReturn(&types.User{Username: "username"}, nil)

		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), nil)
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{User: uid1GQLID, Note: "n", DurationSeconds: &defaultTokenDuration})
		if err == nil {
			t.Error("Expected error, but there was none")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("authenticated as different non-site-admin user", func(t *testing.T) {
		const differentNonSiteAdminUID = 456
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: differentNonSiteAdminUID}, nil)
		users.GetByIDFunc.SetDefaultReturn(&types.User{Username: "username"}, nil)

		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: differentNonSiteAdminUID})
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx, &createAccessTokenInput{User: uid1GQLID, Note: "n", DurationSeconds: &defaultTokenDuration})
		if err == nil {
			t.Error("Expected error, but there was none")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("disable sudo access token creation on Sourcegraph.com", func(t *testing.T) {
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		dotcom.MockSourcegraphDotComMode(t, true)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		_, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx,
			&createAccessTokenInput{
				User:            MarshalUserID(1),
				Scopes:          []string{authz.ScopeUserAll, authz.ScopeSiteAdminSudo},
				DurationSeconds: &defaultTokenDuration,
			},
		)
		got := fmt.Sprintf("%v", err)
		want := `creation of access tokens with scope "site-admin:sudo" is disabled on Sourcegraph.com`
		assert.Equal(t, want, got)
	})

	t.Run("disable create access token for any user on Sourcegraph.com", func(t *testing.T) {
		db := dbmocks.NewMockDB()

		conf.Get().AuthAccessTokens = &schema.AuthAccessTokens{Allow: string(conf.AccessTokensAdmin)}
		defer func() { conf.Get().AuthAccessTokens = nil }()

		dotcom.MockSourcegraphDotComMode(t, true)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		_, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateAccessToken(ctx,
			&createAccessTokenInput{
				User:            MarshalUserID(1),
				Scopes:          []string{authz.ScopeUserAll},
				DurationSeconds: &defaultTokenDuration,
			},
		)
		got := fmt.Sprintf("%v", err)
		want := `access token configuration value "site-admin-create" is disabled on Sourcegraph.com`
		assert.Equal(t, want, got)
	})
}

// 🚨 SECURITY: This tests that users can't delete tokens they shouldn't be allowed to delete.
func TestMutation_DeleteAccessToken(t *testing.T) {
	newMockAccessTokens := func(t *testing.T) database.AccessTokenStore {
		accessTokens := dbmocks.NewMockAccessTokenStore()
		accessTokens.DeleteByIDFunc.SetDefaultHook(func(_ context.Context, id int64) error {
			if want := int64(1); id != want {
				t.Errorf("got %q, want %q", id, want)
			}
			return nil
		})
		accessTokens.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int64) (*database.AccessToken, error) {
			if want := int64(1); id != want {
				t.Errorf("got %d, want %d", id, want)
			}
			return &database.AccessToken{ID: 1, SubjectUserID: 2}, nil
		})
		return accessTokens
	}

	token1GQLID := graphql.ID("QWNjZXNzVG9rZW46MQ==")

	t.Run("authenticated as user", func(t *testing.T) {
		db := dbmocks.NewMockDB()
		db.AccessTokensFunc.SetDefaultReturn(newMockAccessTokens(t))

		RunTests(t, []*Test{
			{
				Context: actor.WithActor(context.Background(), &actor.Actor{UID: 2}),
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
				mutation {
					deleteAccessToken(byID: "` + string(token1GQLID) + `") {
						alwaysNil
					}
				}
			`,
				ExpectedResult: `
				{
					"deleteAccessToken": {
						"alwaysNil": null
					}
				}
			`,
			},
		})
	})

	t.Run("authenticated as different user who is a site-admin", func(t *testing.T) {
		const differentSiteAdminUID = 234

		users := dbmocks.NewMockUserStore()
		users.GetByIDFunc.SetDefaultReturn(&types.User{ID: differentSiteAdminUID, SiteAdmin: true}, nil)
		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.AccessTokensFunc.SetDefaultReturn(newMockAccessTokens(t))

		noExternalAccounts := dbmocks.NewMockUserExternalAccountsStore()
		noExternalAccounts.ListFunc.SetDefaultReturn(nil, nil)
		db.UserExternalAccountsFunc.SetDefaultReturn(noExternalAccounts)

		RunTests(t, []*Test{
			{
				Context: actor.WithActor(context.Background(), &actor.Actor{UID: differentSiteAdminUID}),
				Schema:  mustParseGraphQLSchema(t, db),
				Query: `
				mutation {
					deleteAccessToken(byID: "` + string(token1GQLID) + `") {
						alwaysNil
					}
				}
			`,
				ExpectedResult: `
				{
					"deleteAccessToken": {
						"alwaysNil": null
					}
				}
			`,
			},
		})

		// Should check that token owner is not a SOAP user
		assert.NotEmpty(t, noExternalAccounts.ListFunc.History())
	})

	t.Run("unauthenticated", func(t *testing.T) {
		users := dbmocks.NewMockUserStore()
		db := dbmocks.NewMockDB()
		db.AccessTokensFunc.SetDefaultReturn(newMockAccessTokens(t))
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), nil)
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).DeleteAccessToken(ctx, &deleteAccessTokenInput{ByID: &token1GQLID})
		if err == nil {
			t.Error("Expected error, but there was none")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("authenticated as different non-site-admin user", func(t *testing.T) {
		const differentNonSiteAdminUID = 456

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: differentNonSiteAdminUID}, nil)
		users.GetByIDFunc.SetDefaultReturn(&types.User{Username: "username"}, nil)
		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.AccessTokensFunc.SetDefaultReturn(newMockAccessTokens(t))

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: differentNonSiteAdminUID})
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).DeleteAccessToken(ctx, &deleteAccessTokenInput{ByID: &token1GQLID})
		if err == nil {
			t.Error("Expected error, but there was none")
		}
		if result != nil {
			t.Errorf("got result %v, want nil", result)
		}
	})

	t.Run("non-SOAP user cannot delete SOAP access token", func(t *testing.T) {
		const differentSiteAdminUID = 234

		users := dbmocks.NewMockUserStore()
		users.GetByIDFunc.SetDefaultReturn(&types.User{ID: differentSiteAdminUID, SiteAdmin: true}, nil)
		extAccounts := dbmocks.NewMockUserExternalAccountsStore()
		extAccounts.ListFunc.SetDefaultReturn([]*extsvc.Account{{
			AccountSpec: extsvc.AccountSpec{
				ServiceType: auth.SourcegraphOperatorProviderType,
			},
		}}, nil)
		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		db.AccessTokensFunc.SetDefaultReturn(newMockAccessTokens(t))
		db.UserExternalAccountsFunc.SetDefaultReturn(extAccounts)

		ctx := actor.WithActor(context.Background(), &actor.Actor{
			UID:                 differentSiteAdminUID,
			SourcegraphOperator: false,
		})
		result, err := newSchemaResolver(db, gitserver.NewTestClient(t)).
			DeleteAccessToken(ctx, &deleteAccessTokenInput{ByID: &token1GQLID})
		require.Error(t, err)
		autogold.Expect(`"sourcegraph-operator" user 2's token cannot be deleted by a non-"sourcegraph-operator" user`).Equal(t, err.Error())
		assert.Nil(t, result)
	})
}
