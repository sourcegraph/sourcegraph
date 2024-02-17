package sourcegraphoperator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/cloud"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/sourcegraphoperator"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestAddSourcegraphOperatorExternalAccountBinding(t *testing.T) {
	// Enable SOAP
	cloud.MockSiteConfig(t, &cloud.SchemaSiteConfig{
		AuthProviders: &cloud.SchemaAuthProviders{
			SourcegraphOperator: &cloud.SchemaAuthProviderSourcegraphOperator{
				ClientID: "foobar",
			},
		},
	})
	defer cloud.MockSiteConfig(t, nil)
	// Initialize package
	Init()
	t.Cleanup(func() { providers.Update(auth.SourcegraphOperatorProviderType, nil) })
	// Assert handler is registered - we check this by making sure we get a site admin
	// error instead of an "unimplemented" error.
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: false}, nil)
	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	err := sourcegraphoperator.AddSourcegraphOperatorExternalAccount(context.Background(), db, 1, "foo", "")
	assert.ErrorIs(t, err, auth.ErrMustBeSiteAdmin)
}

func TestAddSourcegraphOperatorExternalAccount(t *testing.T) {
	ctx := context.Background()

	cli, err := httpcli.NewFactory(nil).Doer()
	require.NoError(t, err)

	soap := NewProvider(cloud.SchemaAuthProviderSourcegraphOperator{
		ClientID: "soap_client",
	}, cli)
	serviceID := soap.ConfigID().ID

	mockDB := func(siteAdmin bool) database.DB {
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{
			SiteAdmin: siteAdmin,
		}, nil)
		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)
		return db
	}

	for _, tc := range []struct {
		name string
		// db, user, and other setup
		setup func(t *testing.T) (userID int32, db database.DB)
		// accountDetails parameter
		accountDetails *accountDetailsBody
		// validate result of AddSourcegraphOperatorExternalAccount
		expectErr autogold.Value
		// assert state of the DB (optional)
		assert func(t *testing.T, uid int32, db database.DB)
	}{
		{
			name: "user is not a site admin",
			setup: func(t *testing.T) (int32, database.DB) {
				providers.MockProviders = []providers.Provider{soap}
				t.Cleanup(func() { providers.MockProviders = nil })

				return 42, mockDB(false)
			},
			accountDetails: &accountDetailsBody{
				ClientID:  "foobar",
				AccountID: "bob",
				ExternalAccountData: sourcegraphoperator.ExternalAccountData{
					ServiceAccount: true,
				},
			},
			expectErr: autogold.Expect(`must be site admin`),
		},
		{
			name: "provider does not exist",
			setup: func(t *testing.T) (int32, database.DB) {
				providers.MockProviders = nil
				return 42, mockDB(true)
			},
			expectErr: autogold.Expect("provider does not exist"),
		},
		{
			name: "incorrect details for SOAP provider",
			setup: func(t *testing.T) (int32, database.DB) {
				providers.MockProviders = []providers.Provider{soap}
				t.Cleanup(func() { providers.MockProviders = nil })

				return 42, mockDB(true)
			},
			accountDetails: &accountDetailsBody{
				ClientID:  "foobar",
				AccountID: "bob",
				ExternalAccountData: sourcegraphoperator.ExternalAccountData{
					ServiceAccount: true,
				},
			},
			expectErr: autogold.Expect(`unknown client ID "foobar"`),
		},
		{
			name: "new user associate",
			setup: func(t *testing.T) (int32, database.DB) {
				if testing.Short() {
					t.Skip()
				}

				providers.MockProviders = []providers.Provider{soap}
				t.Cleanup(func() { providers.MockProviders = nil })

				logger := logtest.NoOp(t)
				db := database.NewDB(logger, dbtest.NewDB(t))

				// We ensure the GlobalState is initialized so that the first user isn't
				// a site administrator.
				_, err := db.GlobalState().EnsureInitialized(ctx)
				require.NoError(t, err)

				u, err := db.Users().Create(
					ctx,
					database.NewUser{
						Username: "logan",
					},
				)
				require.NoError(t, err)

				err = db.Users().SetIsSiteAdmin(ctx, u.ID, true)
				require.NoError(t, err)

				return u.ID, db
			},
			accountDetails: &accountDetailsBody{
				ClientID:  "soap_client",
				AccountID: "bob",
				ExternalAccountData: sourcegraphoperator.ExternalAccountData{
					ServiceAccount: true,
				},
			},
			expectErr: autogold.Expect(nil),
			assert: func(t *testing.T, uid int32, db database.DB) {
				accts, err := db.UserExternalAccounts().List(ctx, database.ExternalAccountsListOptions{
					UserID: uid,
				})
				require.NoError(t, err)
				require.Len(t, accts, 1)
				assert.Equal(t, auth.SourcegraphOperatorProviderType, accts[0].ServiceType)
				assert.Equal(t, "bob", accts[0].AccountID)
				assert.Equal(t, "soap_client", accts[0].ClientID)
				assert.Equal(t, serviceID, accts[0].ServiceID)

				data, err := sourcegraphoperator.GetAccountData(ctx, accts[0].AccountData)
				require.NoError(t, err)
				assert.True(t, data.ServiceAccount)
			},
		},
		{
			name: "double associate is not allowed (prevents escalation)",
			setup: func(t *testing.T) (int32, database.DB) {
				if testing.Short() {
					t.Skip()
				}

				providers.MockProviders = []providers.Provider{soap}
				t.Cleanup(func() { providers.MockProviders = nil })

				logger := logtest.NoOp(t)
				db := database.NewDB(logger, dbtest.NewDB(t))

				// We ensure the GlobalState is initialized so that the first user isn't
				// a site administrator.
				_, err := db.GlobalState().EnsureInitialized(ctx)
				require.NoError(t, err)

				u, err := db.Users().Create(
					ctx,
					database.NewUser{
						Username: "bib",
					},
				)
				require.NoError(t, err)
				err = db.Users().SetIsSiteAdmin(ctx, u.ID, true)
				require.NoError(t, err)
				_, err = db.UserExternalAccounts().Upsert(ctx,
					&extsvc.Account{
						UserID: u.ID,
						AccountSpec: extsvc.AccountSpec{
							ServiceType: auth.SourcegraphOperatorProviderType,
							ServiceID:   serviceID,
							ClientID:    "soap_client",
							AccountID:   "bib",
						},
					}) // not a service account initially
				require.NoError(t, err)
				return u.ID, db
			},
			accountDetails: &accountDetailsBody{
				ClientID:  "soap_client",
				AccountID: "bob", // trying to change account ID
				ExternalAccountData: sourcegraphoperator.ExternalAccountData{
					ServiceAccount: true, // trying to promote themselves to service account
				},
			},
			expectErr: autogold.Expect("user already has an associated Sourcegraph Operator account"),
			assert: func(t *testing.T, uid int32, db database.DB) {
				accts, err := db.UserExternalAccounts().List(ctx, database.ExternalAccountsListOptions{
					UserID: uid,
				})
				require.NoError(t, err)
				require.Len(t, accts, 1)
				assert.Equal(t, auth.SourcegraphOperatorProviderType, accts[0].ServiceType)
				assert.Equal(t, "bib", accts[0].AccountID) // the original account
				assert.Equal(t, "soap_client", accts[0].ClientID)
				assert.Equal(t, serviceID, accts[0].ServiceID)

				data, err := sourcegraphoperator.GetAccountData(ctx, accts[0].AccountData)
				require.NoError(t, err)
				assert.False(t, data.ServiceAccount) // still not a service account
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uid, db := tc.setup(t)
			details, err := json.Marshal(tc.accountDetails)
			require.NoError(t, err)

			ctx := actor.WithActor(context.Background(), actor.FromMockUser(uid))
			err = addSourcegraphOperatorExternalAccount(ctx, db, uid, serviceID, string(details))
			if err != nil {
				tc.expectErr.Equal(t, err.Error())
			} else {
				tc.expectErr.Equal(t, nil)
			}
			if tc.assert != nil {
				tc.assert(t, uid, db)
			}
		})
	}
}
