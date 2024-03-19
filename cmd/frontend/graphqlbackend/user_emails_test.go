package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz/permssync"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/fakedb"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/txemail"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func init() {
	txemail.DisableSilently()
}

func TestUserEmail_ViewerCanManuallyVerify(t *testing.T) {
	t.Parallel()

	db := dbmocks.NewMockDB()
	t.Run("only allowed by site admin", func(t *testing.T) {
		users := dbmocks.NewMockUserStore()
		db.UsersFunc.SetDefaultReturn(users)

		tests := []struct {
			name    string
			ctx     context.Context
			setup   func()
			allowed bool
		}{
			{
				name: "unauthenticated",
				ctx:  context.Background(),
				setup: func() {
					users.GetByCurrentAuthUserFunc.SetDefaultHook(func(ctx context.Context) (*types.User, error) {
						return nil, database.ErrNoCurrentUser
					})
				},
				allowed: false,
			},
			{
				name: "non site admin",
				ctx:  actor.WithActor(context.Background(), &actor.Actor{UID: 2}),
				setup: func() {
					users.GetByCurrentAuthUserFunc.SetDefaultHook(func(ctx context.Context) (*types.User, error) {
						return &types.User{
							ID:        2,
							SiteAdmin: false,
						}, nil
					})
				},
				allowed: false,
			},
			{
				name: "site admin",
				ctx:  actor.WithActor(context.Background(), &actor.Actor{UID: 2}),
				setup: func() {
					users.GetByCurrentAuthUserFunc.SetDefaultHook(func(ctx context.Context) (*types.User, error) {
						return &types.User{
							ID:        2,
							SiteAdmin: true,
						}, nil
					})
				},
				allowed: true,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				test.setup()

				ok, _ := (&userEmailResolver{
					db: db,
				}).ViewerCanManuallyVerify(test.ctx)
				assert.Equal(t, test.allowed, ok, "ViewerCanManuallyVerify")
			})
		}
	})
}

func TestSetUserEmailVerified(t *testing.T) {
	t.Run("only allowed by site admins", func(t *testing.T) {
		t.Parallel()

		db := dbmocks.NewMockDB()

		db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
			return f(db)
		})

		ffs := dbmocks.NewMockFeatureFlagStore()
		db.FeatureFlagsFunc.SetDefaultReturn(ffs)

		users := dbmocks.NewMockUserStore()
		db.UsersFunc.SetDefaultReturn(users)
		userEmails := dbmocks.NewMockUserEmailsStore()
		db.UserEmailsFunc.SetDefaultReturn(userEmails)
		db.SubRepoPermsFunc.SetDefaultReturn(dbmocks.NewMockSubRepoPermsStore())

		tests := []struct {
			name    string
			ctx     context.Context
			setup   func()
			wantErr string
		}{
			{
				name: "unauthenticated",
				ctx:  context.Background(),
				setup: func() {
					users.GetByCurrentAuthUserFunc.SetDefaultHook(func(ctx context.Context) (*types.User, error) {
						return nil, database.ErrNoCurrentUser
					})
					users.GetByIDFunc.SetDefaultHook(func(ctx context.Context, i int32) (*types.User, error) {
						return nil, nil
					})
				},
				wantErr: "not authenticated",
			},
			{
				name: "another user",
				ctx:  actor.WithActor(context.Background(), &actor.Actor{UID: 2}),
				setup: func() {
					users.GetByCurrentAuthUserFunc.SetDefaultHook(func(ctx context.Context) (*types.User, error) {
						return &types.User{
							ID:        2,
							SiteAdmin: false,
						}, nil
					})
				},
				wantErr: "must be site admin",
			},
			{
				name: "site admin",
				ctx:  actor.WithActor(context.Background(), &actor.Actor{UID: 2}),
				setup: func() {
					users.GetByCurrentAuthUserFunc.SetDefaultHook(func(ctx context.Context) (*types.User, error) {
						return &types.User{
							ID:        2,
							SiteAdmin: true,
						}, nil
					})
					userEmails.SetVerifiedFunc.SetDefaultHook(func(ctx context.Context, i int32, s string, b bool) error {
						// We just care at this point that we passed user authorization
						return errors.Errorf("short circuit")
					})
				},
				wantErr: "short circuit",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				test.setup()

				_, err := newSchemaResolver(db, gitserver.NewTestClient(t)).SetUserEmailVerified(
					test.ctx,
					&setUserEmailVerifiedArgs{
						User: MarshalUserID(1),
					},
				)
				got := fmt.Sprintf("%v", err)
				assert.Equal(t, test.wantErr, got)
			})
		}
	})

	tests := []struct {
		name                                string
		gqlTests                            func(db database.DB) []*Test
		expectCalledGrantPendingPermissions bool
	}{
		{
			name: "set an email to be verified",
			gqlTests: func(db database.DB) []*Test {
				return []*Test{{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
						mutation {
							setUserEmailVerified(user: "VXNlcjox", email: "alice@example.com", verified: true) {
								alwaysNil
							}
						}
					`,
					ExpectedResult: `
						{
							"setUserEmailVerified": {
								"alwaysNil": null
							}
						}
					`,
				}}
			},
			expectCalledGrantPendingPermissions: true,
		},
		{
			name: "set an email to be unverified",
			gqlTests: func(db database.DB) []*Test {
				return []*Test{{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
						mutation {
							setUserEmailVerified(user: "VXNlcjox", email: "alice@example.com", verified: false) {
								alwaysNil
							}
						}
					`,
					ExpectedResult: `
						{
							"setUserEmailVerified": {
								"alwaysNil": null
							}
						}
					`,
				}}
			},
			expectCalledGrantPendingPermissions: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := dbmocks.NewMockUserStore()
			users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

			userEmails := dbmocks.NewMockUserEmailsStore()
			userEmails.SetVerifiedFunc.SetDefaultReturn(nil)

			authz := dbmocks.NewMockAuthzStore()
			authz.GrantPendingPermissionsFunc.SetDefaultReturn(nil)

			userExternalAccounts := dbmocks.NewMockUserExternalAccountsStore()
			userExternalAccounts.DeleteFunc.SetDefaultReturn(nil)

			permssync.MockSchedulePermsSync = func(_ context.Context, _ log.Logger, _ database.DB, _ permssync.ScheduleSyncOpts) {}
			t.Cleanup(func() { permssync.MockSchedulePermsSync = nil })

			db := dbmocks.NewMockDB()
			db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
				return f(db)
			})

			db.UsersFunc.SetDefaultReturn(users)
			db.UserEmailsFunc.SetDefaultReturn(userEmails)
			db.AuthzFunc.SetDefaultReturn(authz)
			db.UserExternalAccountsFunc.SetDefaultReturn(userExternalAccounts)
			db.SubRepoPermsFunc.SetDefaultReturn(dbmocks.NewMockSubRepoPermsStore())

			RunTests(t, test.gqlTests(db))

			if test.expectCalledGrantPendingPermissions {
				mockrequire.Called(t, authz.GrantPendingPermissionsFunc)
			} else {
				mockrequire.NotCalled(t, authz.GrantPendingPermissionsFunc)
			}
		})
	}
}

func TestPrimaryEmail(t *testing.T) {
	primaryEmailQuery := `query hasPrimaryEmail($id: ID!){
		node(id: $id) {
			... on User {
				primaryEmail {
					email
				}
			}
		}
	}`
	type primaryEmail struct {
		Email string
	}
	type node struct {
		PrimaryEmail *primaryEmail
	}
	type primaryEmailResponse struct {
		Node node
	}

	now := time.Now()
	for name, testCase := range map[string]struct {
		emails []*database.UserEmail
		want   primaryEmailResponse
	}{
		"no emails": {
			want: primaryEmailResponse{
				Node: node{
					PrimaryEmail: nil,
				},
			},
		},
		"has primary email": {
			emails: []*database.UserEmail{
				{
					Email:      "primary@example.com",
					Primary:    true,
					VerifiedAt: &now,
				},
				{
					Email:      "secondary@example.com",
					VerifiedAt: &now,
				},
			},
			want: primaryEmailResponse{
				Node: node{
					PrimaryEmail: &primaryEmail{
						Email: "primary@example.com",
					},
				},
			},
		},
		"no primary email": {
			emails: []*database.UserEmail{
				{
					Email:      "not-primary@example.com",
					VerifiedAt: &now,
				},
				{
					Email:      "not-primary-either@example.com",
					VerifiedAt: &now,
				},
			},
			want: primaryEmailResponse{
				Node: node{
					PrimaryEmail: nil,
				},
			},
		},
		"no verified email": {
			emails: []*database.UserEmail{
				{
					Email:   "primary@example.com",
					Primary: true,
				},
				{
					Email: "not-primary@example.com",
				},
			},
			want: primaryEmailResponse{
				Node: node{
					PrimaryEmail: nil,
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			fs := fakedb.New()
			db := dbmocks.NewMockDB()
			emails := dbmocks.NewMockUserEmailsStore()
			emails.ListByUserFunc.SetDefaultHook(func(_ context.Context, ops database.UserEmailsListOptions) ([]*database.UserEmail, error) {
				var emails []*database.UserEmail
				for _, m := range testCase.emails {
					if ops.OnlyVerified && m.VerifiedAt == nil {
						continue
					}
					copy := *m
					copy.UserID = ops.UserID
					emails = append(emails, &copy)
				}
				return emails, nil
			})
			db.UserEmailsFunc.SetDefaultReturn(emails)
			fs.Wire(db)
			ctx := actor.WithActor(context.Background(), actor.FromUser(fs.AddUser(types.User{SiteAdmin: true})))
			userID := fs.AddUser(types.User{
				Username: "horse",
			})
			result := mustParseGraphQLSchema(t, db).Exec(ctx, primaryEmailQuery, "", map[string]any{
				"id": string(relay.MarshalID("User", userID)),
			})
			if len(result.Errors) != 0 {
				t.Fatal(result.Errors)
			}
			var resultData primaryEmailResponse
			if err := json.Unmarshal(result.Data, &resultData); err != nil {
				t.Fatalf("cannot unmarshal result data: %s", err)
			}
			if diff := cmp.Diff(testCase.want, resultData); diff != "" {
				t.Errorf("result data, -want+got: %s", diff)
			}
		})
	}
}
