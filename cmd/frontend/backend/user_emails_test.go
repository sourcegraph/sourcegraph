package backend

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/perforce"
	"github.com/sourcegraph/sourcegraph/internal/txemail"
	"github.com/sourcegraph/sourcegraph/internal/txemail/txtypes"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestCheckEmailAbuse(t *testing.T) {
	ctx := testContext()

	cfg := conf.Get()
	cfg.EmailSmtp = &schema.SMTPServerConfig{}
	conf.Mock(cfg)
	defer func() {
		cfg.EmailSmtp = nil
		conf.Mock(cfg)
	}()

	dotcom.MockSourcegraphDotComMode(t, true)

	now := time.Now()

	tests := []struct {
		name       string
		mockEmails []*database.UserEmail
		hasQuote   bool
		expAbused  bool
		expReason  string
		expErr     error
	}{
		{
			name: "no verified email address",
			mockEmails: []*database.UserEmail{
				{
					Email: "alice@example.com",
				},
			},
			hasQuote:  false,
			expAbused: true,
			expReason: "a verified email is required before you can add additional email addressed to your account",
			expErr:    nil,
		},
		{
			name: "reached maximum number of unverified email addresses",
			mockEmails: []*database.UserEmail{
				{
					Email:      "alice@example.com",
					VerifiedAt: &now,
				},
				{
					Email: "alice2@example.com",
				},
				{
					Email: "alice3@example.com",
				},
				{
					Email: "alice4@example.com",
				},
			},
			hasQuote:  false,
			expAbused: true,
			expReason: "too many existing unverified email addresses",
			expErr:    nil,
		},
		{
			name: "no quota",
			mockEmails: []*database.UserEmail{
				{
					Email:      "alice@example.com",
					VerifiedAt: &now,
				},
			},
			hasQuote:  false,
			expAbused: true,
			expReason: "email address quota exceeded (contact support to increase the quota)",
			expErr:    nil,
		},

		{
			name: "no abuse",
			mockEmails: []*database.UserEmail{
				{
					Email:      "alice@example.com",
					VerifiedAt: &now,
				},
			},
			hasQuote:  true,
			expAbused: false,
			expReason: "",
			expErr:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := dbmocks.NewMockUserStore()
			users.CheckAndDecrementInviteQuotaFunc.SetDefaultReturn(test.hasQuote, nil)

			userEmails := dbmocks.NewMockUserEmailsStore()
			userEmails.ListByUserFunc.SetDefaultReturn(test.mockEmails, nil)

			db := dbmocks.NewMockDB()
			db.UsersFunc.SetDefaultReturn(users)
			db.UserEmailsFunc.SetDefaultReturn(userEmails)

			abused, reason, err := checkEmailAbuse(ctx, db, 1)
			if test.expErr != err {
				t.Fatalf("err: want %v but got %v", test.expErr, err)
			} else if test.expAbused != abused {
				t.Fatalf("abused: want %v but got %v", test.expAbused, abused)
			} else if test.expReason != reason {
				t.Fatalf("reason: want %q but got %q", test.expReason, reason)
			}
		})
	}
}

func TestSendUserEmailVerificationEmail(t *testing.T) {
	var sent *txemail.Message
	txemail.MockSend = func(ctx context.Context, message txemail.Message) error {
		sent = &message
		return nil
	}
	defer func() { txemail.MockSend = nil }()

	if err := SendUserEmailVerificationEmail(context.Background(), "Alan Johnson", "a@example.com", "c"); err != nil {
		t.Fatal(err)
	}
	if sent == nil {
		t.Fatal("want sent != nil")
	}
	if want := (txemail.Message{
		To:       []string{"a@example.com"},
		Template: verifyEmailTemplates,
		Data: struct {
			Username string
			URL      string
			Host     string
		}{
			Username: "Alan Johnson",
			URL:      "http://example.com/-/verify-email?code=c&email=a%40example.com",
			Host:     "example.com",
		},
	}); !reflect.DeepEqual(*sent, want) {
		t.Errorf("got %+v, want %+v", *sent, want)
	}
}

func TestSendUserEmailOnFieldUpdate(t *testing.T) {
	var sent *txemail.Message
	txemail.MockSend = func(ctx context.Context, message txemail.Message) error {
		sent = &message
		return nil
	}
	defer func() { txemail.MockSend = nil }()

	userEmails := dbmocks.NewMockUserEmailsStore()
	userEmails.GetPrimaryEmailFunc.SetDefaultReturn("a@example.com", true, nil)

	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{Username: "Foo"}, nil)

	db := dbmocks.NewMockDB()
	db.UserEmailsFunc.SetDefaultReturn(userEmails)
	db.UsersFunc.SetDefaultReturn(users)
	logger := logtest.Scoped(t)

	svc := NewUserEmailsService(db, logger)
	if err := svc.SendUserEmailOnFieldUpdate(context.Background(), 123, "updated password"); err != nil {
		t.Fatal(err)
	}
	if sent == nil {
		t.Fatal("want sent != nil")
	}
	if want := (txemail.Message{
		To:       []string{"a@example.com"},
		Template: updateAccountEmailTemplate,
		Data: struct {
			Email    string
			Change   string
			Username string
			Host     string
		}{
			Email:    "a@example.com",
			Change:   "updated password",
			Username: "Foo",
			Host:     "example.com",
		},
	}); !reflect.DeepEqual(*sent, want) {
		t.Errorf("got %+v, want %+v", *sent, want)
	}

	mockrequire.Called(t, userEmails.GetPrimaryEmailFunc)
	mockrequire.Called(t, users.GetByIDFunc)
}

func TestSendUserEmailOnTokenChange(t *testing.T) {
	var sent *txemail.Message
	txemail.MockSend = func(ctx context.Context, message txemail.Message) error {
		sent = &message
		return nil
	}
	defer func() { txemail.MockSend = nil }()

	userEmails := dbmocks.NewMockUserEmailsStore()
	userEmails.GetPrimaryEmailFunc.SetDefaultReturn("a@example.com", true, nil)

	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{Username: "Foo"}, nil)

	db := dbmocks.NewMockDB()
	db.UserEmailsFunc.SetDefaultReturn(userEmails)
	db.UsersFunc.SetDefaultReturn(users)
	logger := logtest.Scoped(t)

	svc := NewUserEmailsService(db, logger)
	tt := []struct {
		name      string
		tokenName string
		delete    bool
		template  txtypes.Templates
	}{
		{
			"Access Token deleted",
			"my-long-last-token",
			true,
			accessTokenDeletedEmailTemplate,
		},
		{
			"Access Token created",
			"heyo-new-token",
			false,
			accessTokenCreatedEmailTemplate,
		},
	}
	for _, item := range tt {
		t.Run(item.name, func(t *testing.T) {
			if err := svc.SendUserEmailOnAccessTokenChange(context.Background(), 123, item.tokenName, item.delete); err != nil {
				t.Fatal(err)
			}
			if sent == nil {
				t.Fatal("want sent != nil")
			}

			if want := (txemail.Message{
				To:       []string{"a@example.com"},
				Template: item.template,
				Data: struct {
					Email     string
					TokenName string
					Username  string
					Host      string
				}{
					Email:     "a@example.com",
					TokenName: item.tokenName,
					Username:  "Foo",
					Host:      "example.com",
				},
			}); !reflect.DeepEqual(*sent, want) {
				t.Errorf("got %+v, want %+v", *sent, want)
			}
		})
	}
}

type noopAuthzStore struct{}

func (*noopAuthzStore) GrantPendingPermissions(_ context.Context, _ *database.GrantPendingPermissionsArgs) error {
	return nil
}

func (*noopAuthzStore) AuthorizedRepos(_ context.Context, _ *database.AuthorizedReposArgs) ([]*types.Repo, error) {
	return []*types.Repo{}, nil
}

func (*noopAuthzStore) RevokeUserPermissions(_ context.Context, _ *database.RevokeUserPermissionsArgs) error {
	return nil
}

func (*noopAuthzStore) RevokeUserPermissionsList(_ context.Context, _ []*database.RevokeUserPermissionsArgs) error {
	return nil
}

func TestUserEmailsAddRemove(t *testing.T) {
	database.AuthzWith = func(basestore.ShareableStore) database.AuthzStore {
		return &noopAuthzStore{}
	}
	defer func() {
		database.AuthzWith = nil
	}()

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	txemail.DisableSilently()

	const email = "user@example.com"
	const email2 = "user.secondary@example.com"
	const username = "test-user"
	const verificationCode = "code"

	newUser := database.NewUser{
		Email:                 email,
		Username:              username,
		EmailVerificationCode: verificationCode,
	}

	createdUser, err := db.Users().Create(ctx, newUser)
	assert.NoError(t, err)

	svc := NewUserEmailsService(db, logger)

	// Unauthenticated user should fail
	assert.Error(t, svc.Add(ctx, createdUser.ID, email2))
	// Different user should fail
	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 99,
	})
	assert.Error(t, svc.Add(ctx, createdUser.ID, email2))

	// Add as a site admin (or internal actor) should pass
	ctx = actor.WithInternalActor(context.Background())
	// Add secondary e-mail
	assert.NoError(t, svc.Add(ctx, createdUser.ID, email2))

	// Add reset code
	code, err := db.Users().RenewPasswordResetCode(ctx, createdUser.ID)
	assert.NoError(t, err)

	// Remove as unauthenticated user should fail
	ctx = context.Background()
	assert.Error(t, svc.Remove(ctx, createdUser.ID, email2))

	// Remove as different user should fail
	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 99,
	})
	assert.Error(t, svc.Remove(ctx, createdUser.ID, email2))

	// Remove as a site admin (or internal actor) should pass
	ctx = actor.WithInternalActor(context.Background())
	assert.NoError(t, svc.Remove(ctx, createdUser.ID, email2))

	// Trying to change the password with the old code should fail
	changed, err := db.Users().SetPassword(ctx, createdUser.ID, code, "some-amazing-new-password")
	assert.NoError(t, err)
	assert.False(t, changed)

	// Can't remove primary e-mail
	assert.Error(t, svc.Remove(ctx, createdUser.ID, email))

	// Set email as verified, add a second user, and try to add the verified email
	svc.SetVerified(ctx, createdUser.ID, email, true)
	user2, err := db.Users().Create(ctx, database.NewUser{Username: "test-user-2"})
	require.NoError(t, err)

	require.Error(t, svc.Add(ctx, user2.ID, email))
}

func TestUserEmailsSetPrimary(t *testing.T) {
	database.AuthzWith = func(basestore.ShareableStore) database.AuthzStore {
		return &noopAuthzStore{}
	}
	defer func() {
		database.AuthzWith = nil
	}()

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	txemail.DisableSilently()

	const email = "user@example.com"
	const username = "test-user"
	const verificationCode = "code"

	newUser := database.NewUser{
		Email:                 email,
		Username:              username,
		EmailVerificationCode: verificationCode,
	}

	createdUser, err := db.Users().Create(ctx, newUser)
	assert.NoError(t, err)

	svc := NewUserEmailsService(db, logger)

	// Unauthenticated user should fail
	assert.Error(t, svc.SetPrimaryEmail(ctx, createdUser.ID, email))
	// Different user should fail
	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 99,
	})
	assert.Error(t, svc.SetPrimaryEmail(ctx, createdUser.ID, email))

	// As site admin (or internal actor) should pass
	ctx = actor.WithInternalActor(ctx)
	// Need to set e-mail as verified
	assert.NoError(t, svc.SetVerified(ctx, createdUser.ID, email, true))
	assert.NoError(t, svc.SetPrimaryEmail(ctx, createdUser.ID, email))

	fromDB, verified, err := db.UserEmails().GetPrimaryEmail(ctx, createdUser.ID)
	assert.NoError(t, err)
	assert.Equal(t, email, fromDB)
	assert.True(t, verified)
}

func TestUserEmailsSetVerified(t *testing.T) {
	database.AuthzWith = func(basestore.ShareableStore) database.AuthzStore {
		return &noopAuthzStore{}
	}
	defer func() {
		database.AuthzWith = nil
	}()

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	txemail.DisableSilently()

	const email = "user@example.com"
	const email2 = "user.secondary@example.com"
	const username = "test-user"
	const verificationCode = "code"

	newUser := database.NewUser{
		Email:                 email,
		Username:              username,
		EmailVerificationCode: verificationCode,
	}

	createdUser, err := db.Users().Create(ctx, newUser)
	assert.NoError(t, err)

	svc := NewUserEmailsService(db, logger)
	// Unauthenticated user should fail
	assert.Error(t, svc.SetVerified(ctx, createdUser.ID, email, true))
	// Different user should fail
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 99})

	// As site admin (or internal actor) should pass
	ctx = actor.WithInternalActor(ctx)
	// Need to set e-mail as verified
	assert.NoError(t, svc.SetVerified(ctx, createdUser.ID, email, true))

	// Confirm that unverified emails get deleted when an email is marked as verified
	assert.NoError(t, svc.SetVerified(ctx, createdUser.ID, email, false)) // first mark as unverified again

	user2, err := db.Users().Create(ctx, database.NewUser{Username: "test-user-2"})
	require.NoError(t, err)

	assert.NoError(t, svc.Add(ctx, user2.ID, email)) // Adding an unverified email is fine if all emails are unverified

	assert.NoError(t, svc.SetVerified(ctx, createdUser.ID, email, true)) // mark as verified again
	_, _, err = db.UserEmails().Get(ctx, user2.ID, email)                // This should produce an error as the email should no longer exist
	assert.Error(t, err)

	emails, err := db.UserEmails().GetVerifiedEmails(ctx, email, email2)
	assert.NoError(t, err)
	assert.Len(t, emails, 1)
	assert.Equal(t, email, emails[0].Email)
}

func TestUserEmailsResendVerificationEmail(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	txemail.DisableSilently()

	oldSend := txemail.MockSend
	t.Cleanup(func() {
		txemail.MockSend = oldSend
	})
	var sendCalled bool
	txemail.MockSend = func(ctx context.Context, message txemail.Message) error {
		sendCalled = true
		return nil
	}
	assertSendCalled := func(want bool) {
		assert.Equal(t, want, sendCalled)
		// Reset to false
		sendCalled = false
	}

	const email = "user@example.com"
	const username = "test-user"
	const verificationCode = "code"

	newUser := database.NewUser{
		Email:                 email,
		Username:              username,
		EmailVerificationCode: verificationCode,
	}

	createdUser, err := db.Users().Create(ctx, newUser)
	assert.NoError(t, err)

	svc := NewUserEmailsService(db, logger)
	now := time.Now()

	// Set that we sent the initial e-mail
	assert.NoError(t, db.UserEmails().SetLastVerification(ctx, createdUser.ID, email, verificationCode, now))

	// Unauthenticated user should fail
	assert.Error(t, svc.ResendVerificationEmail(ctx, createdUser.ID, email, now))
	assertSendCalled(false)

	// Different user should fail
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 99})
	assert.Error(t, svc.ResendVerificationEmail(ctx, createdUser.ID, email, now))
	assertSendCalled(false)

	// As site admin (or internal actor) should pass
	ctx = actor.WithInternalActor(ctx)
	// Set in the future so that we can resend
	now = now.Add(5 * time.Minute)
	assert.NoError(t, svc.ResendVerificationEmail(ctx, createdUser.ID, email, now))
	assertSendCalled(true)

	// Trying to send again too soon should fail
	assert.Error(t, svc.ResendVerificationEmail(ctx, createdUser.ID, email, now.Add(1*time.Second)))
	assertSendCalled(false)

	// Invalid e-mail
	assert.Error(t, svc.ResendVerificationEmail(ctx, createdUser.ID, "another@example.com", now.Add(5*time.Minute)))
	assertSendCalled(false)

	// Manually mark as verified
	assert.NoError(t, db.UserEmails().SetVerified(ctx, createdUser.ID, email, true))

	// Trying to send verification e-mail now should be a noop since we are already
	// verified
	assert.NoError(t, svc.ResendVerificationEmail(ctx, createdUser.ID, email, now.Add(10*time.Minute)))
	assertSendCalled(false)
}

func TestRemoveStalePerforceAccount(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	txemail.DisableSilently()

	const email = "user@example.com"
	const email2 = "user.secondary@example.com"
	const username = "test-user"
	const verificationCode = "code"

	newUser := database.NewUser{
		Email:                 email,
		Username:              username,
		EmailVerificationCode: verificationCode,
	}

	createdUser, err := db.Users().Create(ctx, newUser)
	assert.NoError(t, err)

	createdRepo := &types.Repo{
		Name:         "github.com/soucegraph/sourcegraph",
		URI:          "github.com/soucegraph/sourcegraph",
		ExternalRepo: api.ExternalRepoSpec{},
	}
	err = db.Repos().Create(ctx, createdRepo)
	require.NoError(t, err)

	svc := NewUserEmailsService(db, logger)
	ctx = actor.WithInternalActor(ctx)

	setup := func() {
		require.NoError(t, svc.Add(ctx, createdUser.ID, email2))

		spec := extsvc.AccountSpec{
			ServiceType: extsvc.TypePerforce,
			ServiceID:   "test-instance",
			// We use the email address as the account id for Perforce
			AccountID: email2,
		}
		perforceData := perforce.AccountData{
			Username: "user",
			Email:    email2,
		}
		serializedData, err := json.Marshal(perforceData)
		require.NoError(t, err)
		data := extsvc.AccountData{
			Data: extsvc.NewUnencryptedData(serializedData),
		}
		_, err = db.UserExternalAccounts().Insert(ctx,
			&extsvc.Account{
				UserID:      createdUser.ID,
				AccountSpec: spec,
				AccountData: data,
			})
		require.NoError(t, err)

		// Confirm that the external account was added
		accounts, err := db.UserExternalAccounts().List(ctx, database.ExternalAccountsListOptions{
			UserID:      createdUser.ID,
			ServiceType: extsvc.TypePerforce,
		})
		require.NoError(t, err)
		require.Len(t, accounts, 1)
	}

	assertRemovals := func(t *testing.T) {
		// Confirm that the external account is gone
		accounts, err := db.UserExternalAccounts().List(ctx, database.ExternalAccountsListOptions{
			UserID:      createdUser.ID,
			ServiceType: extsvc.TypePerforce,
		})
		require.NoError(t, err)
		require.Len(t, accounts, 0)
	}

	t.Run("OnDelete", func(t *testing.T) {
		setup()

		// Remove the email
		require.NoError(t, svc.Remove(ctx, createdUser.ID, email2))

		assertRemovals(t)
	})

	t.Run("OnUnverified", func(t *testing.T) {
		setup()

		// Mark the e-mail as unverified
		require.NoError(t, svc.SetVerified(ctx, createdUser.ID, email2, false))

		assertRemovals(t)
	})
}
