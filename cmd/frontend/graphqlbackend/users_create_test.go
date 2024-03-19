package graphqlbackend

import (
	"context"
	"net/url"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/auth/userpasswd"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/txemail"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

type mockFuncs struct {
	dB             *dbmocks.MockDB
	authzStore     *dbmocks.MockAuthzStore
	usersStore     *dbmocks.MockUserStore
	userEmailStore *dbmocks.MockUserEmailsStore
}

func makeUsersCreateTestDB(t *testing.T) mockFuncs {
	users := dbmocks.NewMockUserStore()
	// This is the created user that is returned via the GraphQL API.
	users.CreateFunc.SetDefaultReturn(&types.User{ID: 1, Username: "alice"}, nil)
	// This refers to the user executing this API request.
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 2, SiteAdmin: true}, nil)

	authz := dbmocks.NewMockAuthzStore()
	authz.GrantPendingPermissionsFunc.SetDefaultReturn(nil)

	userEmails := dbmocks.NewMockUserEmailsStore()

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.AuthzFunc.SetDefaultReturn(authz)
	db.UserEmailsFunc.SetDefaultReturn(userEmails)

	return mockFuncs{
		dB:             db,
		usersStore:     users,
		authzStore:     authz,
		userEmailStore: userEmails,
	}
}

func TestCreateUser(t *testing.T) {
	mocks := makeUsersCreateTestDB(t)

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, mocks.dB),
			Query: `
				mutation {
					createUser(username: "alice") {
						user {
							id
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"createUser": {
						"user": {
							"id": "VXNlcjox"
						}
					}
				}
			`,
		},
	})

	mockrequire.CalledOnce(t, mocks.authzStore.GrantPendingPermissionsFunc)
	mockrequire.CalledOnce(t, mocks.usersStore.CreateFunc)
}

func TestCreateUserResetPasswordURL(t *testing.T) {
	backend.MockMakePasswordResetURL = func(_ context.Context, _ int32) (*url.URL, error) {
		return url.Parse("/reset-url?code=foobar")
	}
	userpasswd.MockResetPasswordEnabled = func() bool { return true }
	t.Cleanup(func() {
		backend.MockMakePasswordResetURL = nil
		userpasswd.MockResetPasswordEnabled = nil
	})

	t.Run("with SMTP disabled", func(t *testing.T) {
		mocks := makeUsersCreateTestDB(t)

		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				EmailSmtp: nil,
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, mocks.dB),
				Query: `
					mutation {
						createUser(username: "alice",email:"alice@sourcegraph.com",verifiedEmail:false) {
							user {
								id
							}
							resetPasswordURL
						}
					}
				`,
				ExpectedResult: `
					{
						"createUser": {
							"user": {
								"id": "VXNlcjox"
							},
							"resetPasswordURL": "http://example.com/reset-url?code=foobar"
						}
					}
				`,
			},
		})

		mockrequire.Called(t, mocks.authzStore.GrantPendingPermissionsFunc)
	})

	t.Run("with SMTP enabled", func(t *testing.T) {
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				EmailSmtp: &schema.SMTPServerConfig{},
			},
		})

		var sentMessage txemail.Message
		txemail.MockSend = func(_ context.Context, message txemail.Message) error {
			sentMessage = message
			return nil
		}
		t.Cleanup(func() {
			conf.Mock(nil)
			txemail.MockSend = nil
		})

		mocks := makeUsersCreateTestDB(t)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, mocks.dB),
				Query: `
					mutation {
						createUser(username: "alice",email:"alice@sourcegraph.com",verifiedEmail:false) {
							user {
								id
							}
							resetPasswordURL
						}
					}
				`,
				ExpectedResult: `
					{
						"createUser": {
							"user": {
								"id": "VXNlcjox"
							},
							"resetPasswordURL": "http://example.com/reset-url?code=foobar"
						}
					}
				`,
			},
		})

		data := sentMessage.Data.(userpasswd.SetPasswordEmailTemplateData)
		assert.Contains(t, data.URL, "http://example.com/reset-url")
		assert.Contains(t, data.URL, "&emailVerifyCode=")
		assert.Contains(t, data.URL, "&email=")

		mockrequire.Called(t, mocks.authzStore.GrantPendingPermissionsFunc)
		mockrequire.Called(t, mocks.userEmailStore.SetLastVerificationFunc)
	})

	t.Run("with SMTP enabled, without verifiedEmail", func(t *testing.T) {
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				EmailSmtp: &schema.SMTPServerConfig{},
			},
		})

		var sentMessage txemail.Message
		txemail.MockSend = func(_ context.Context, message txemail.Message) error {
			sentMessage = message
			return nil
		}
		t.Cleanup(func() {
			conf.Mock(nil)
			txemail.MockSend = nil
		})

		mocks := makeUsersCreateTestDB(t)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, mocks.dB),
				Query: `
					mutation {
						createUser(username: "alice",email:"alice@sourcegraph.com") {
							user {
								id
							}
							resetPasswordURL
						}
					}
				`,
				ExpectedResult: `
					{
						"createUser": {
							"user": {
								"id": "VXNlcjox"
							},
							"resetPasswordURL": "http://example.com/reset-url?code=foobar"
						}
					}
				`,
			},
		})

		// should not have tried to issue email verification
		data := sentMessage.Data.(userpasswd.SetPasswordEmailTemplateData)
		assert.Contains(t, data.URL, "http://example.com/reset-url")
		assert.NotContains(t, data.URL, "&emailVerifyCode=")
		assert.NotContains(t, data.URL, "&email=")

		mockrequire.Called(t, mocks.authzStore.GrantPendingPermissionsFunc)
		mockrequire.NotCalled(t, mocks.userEmailStore.SetLastVerificationFunc)
	})
}
