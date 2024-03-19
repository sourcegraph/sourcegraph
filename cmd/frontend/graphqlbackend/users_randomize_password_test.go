package graphqlbackend

import (
	"context"
	"testing"

	"github.com/graph-gophers/graphql-go/errors"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/txemail"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestRandomizeUserPassword(t *testing.T) {
	userID := int32(42)
	userIDBase64 := string(MarshalUserID(userID))

	var (
		smtpEnabledConf = &conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{{Builtin: &schema.BuiltinAuthProvider{}}},
				EmailSmtp:     &schema.SMTPServerConfig{},
			}}
		smtpDisabledConf = &conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{{Builtin: &schema.BuiltinAuthProvider{}}},
			}}
	)

	db := dbmocks.NewMockDB()
	t.Run("Errors when resetting passwords is not enabled", func(t *testing.T) {
		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
					mutation($user: ID!) {
						randomizeUserPassword(user: $user) {
							resetPasswordURL
						}
					}
				`,
				ExpectedResult: "null",
				ExpectedErrors: []*errors.QueryError{
					{
						Message: "resetting passwords is not enabled",
						Path:    []any{"randomizeUserPassword"},
					},
				},
				Variables: map[string]any{"user": userIDBase64},
			},
		})
	})

	t.Run("DotCom mode", func(t *testing.T) {
		// Test dotcom mode
		dotcom.MockSourcegraphDotComMode(t, true)

		t.Run("Errors on DotCom when sending emails is not enabled", func(t *testing.T) {
			conf.Mock(smtpDisabledConf)
			defer conf.Mock(nil)

			RunTests(t, []*Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
					mutation($user: ID!) {
						randomizeUserPassword(user: $user) {
							resetPasswordURL
						}
					}
				`,
					ExpectedResult: "null",
					ExpectedErrors: []*errors.QueryError{
						{
							Message: "unable to reset password because email sending is not configured",
							Path:    []any{"randomizeUserPassword"},
						},
					},
					Variables: map[string]any{"user": userIDBase64},
				},
			})
		})

		t.Run("Does not return resetPasswordUrl when in Cloud", func(t *testing.T) {
			// Enable SMTP
			conf.Mock(smtpEnabledConf)
			defer conf.Mock(nil)

			users := dbmocks.NewMockUserStore()
			users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
			users.RandomizePasswordAndClearPasswordResetRateLimitFunc.SetDefaultReturn(nil)
			users.RenewPasswordResetCodeFunc.SetDefaultReturn("code", nil)
			users.GetByIDFunc.SetDefaultReturn(&types.User{Username: "alice"}, nil)

			userEmails := dbmocks.NewMockUserEmailsStore()
			userEmails.GetPrimaryEmailFunc.SetDefaultReturn("alice@foo.bar", false, nil)

			db.UsersFunc.SetDefaultReturn(users)
			db.UserEmailsFunc.SetDefaultReturn(userEmails)

			txemail.MockSend = func(ctx context.Context, message txemail.Message) error {
				return nil
			}
			defer func() {
				txemail.MockSend = nil
			}()

			RunTests(t, []*Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
					mutation($user: ID!) {
						randomizeUserPassword(user: $user) {
							resetPasswordURL
						}
					}
				`,
					ExpectedResult: `{
					"randomizeUserPassword": {
						"resetPasswordURL": null
					}
				}`,
					Variables: map[string]any{"user": userIDBase64},
				},
			})
		})
	})

	t.Run("Returns error if user is not site-admin", func(t *testing.T) {
		conf.Mock(smtpDisabledConf)
		defer conf.Mock(nil)

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: false}, nil)
		db.UsersFunc.SetDefaultReturn(users)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
					mutation($user: ID!) {
						randomizeUserPassword(user: $user) {
							resetPasswordURL
						}
					}
				`,
				ExpectedResult: "null",
				ExpectedErrors: []*errors.QueryError{
					{
						Message: "must be site admin",
						Path:    []any{"randomizeUserPassword"},
					},
				},
				Variables: map[string]any{"user": userIDBase64},
			},
		})
	})

	t.Run("Returns error when cannot parse user ID", func(t *testing.T) {
		conf.Mock(smtpDisabledConf)
		defer conf.Mock(nil)

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
		db.UsersFunc.SetDefaultReturn(users)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
					mutation($user: ID!) {
						randomizeUserPassword(user: $user) {
							resetPasswordURL
						}
					}
				`,
				ExpectedResult: "null",
				ExpectedErrors: []*errors.QueryError{
					{
						Message: "cannot parse user ID: illegal base64 data at input byte 4",
						Path:    []any{"randomizeUserPassword"},
					},
				},
				Variables: map[string]any{"user": "alice"},
			},
		})
	})

	t.Run("Returns resetPasswordUrl if user is site-admin", func(t *testing.T) {
		conf.Mock(smtpDisabledConf)
		defer conf.Mock(nil)

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
		users.RandomizePasswordAndClearPasswordResetRateLimitFunc.SetDefaultReturn(nil)
		users.RenewPasswordResetCodeFunc.SetDefaultReturn("code", nil)
		db.UsersFunc.SetDefaultReturn(users)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
					mutation($user: ID!) {
						randomizeUserPassword(user: $user) {
							resetPasswordURL
						}
					}
				`,
				ExpectedResult: `{
					"randomizeUserPassword": {
						"resetPasswordURL": "http://example.com/password-reset?code=code&userID=42"
					}
				}`,
				Variables: map[string]any{"user": userIDBase64},
			},
		})
	})

	t.Run("Returns resetPasswordUrl and sends email if user is site-admin", func(t *testing.T) {
		conf.Mock(smtpEnabledConf)
		defer conf.Mock(nil)

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
		users.RandomizePasswordAndClearPasswordResetRateLimitFunc.SetDefaultReturn(nil)
		users.RenewPasswordResetCodeFunc.SetDefaultReturn("code", nil)
		users.GetByIDFunc.SetDefaultReturn(&types.User{Username: "alice"}, nil)

		userEmails := dbmocks.NewMockUserEmailsStore()
		userEmails.GetPrimaryEmailFunc.SetDefaultReturn("alice@foo.bar", false, nil)

		db.UsersFunc.SetDefaultReturn(users)
		db.UserEmailsFunc.SetDefaultReturn(userEmails)

		sent := false
		txemail.MockSend = func(ctx context.Context, message txemail.Message) error {
			sent = true
			return nil
		}
		defer func() {
			txemail.MockSend = nil
		}()

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
					mutation($user: ID!) {
						randomizeUserPassword(user: $user) {
							resetPasswordURL
						}
					}
				`,
				ExpectedResult: `{
					"randomizeUserPassword": {
						"resetPasswordURL": "http://example.com/password-reset?code=code&userID=42"
					}
				}`,
				Variables: map[string]any{"user": userIDBase64},
			},
		})

		assert.True(t, sent, "should have sent email")
	})
}
