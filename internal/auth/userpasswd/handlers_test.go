package userpasswd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/session"
	"github.com/sourcegraph/sourcegraph/internal/telemetry"
	"github.com/sourcegraph/sourcegraph/internal/telemetry/telemetrytest"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestCheckEmailAbuse(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	farFuture := now.AddDate(100, 0, 0)

	tests := []struct {
		name      string
		mockEmail *database.UserEmail
		mockErr   error
		expAbused bool
		expReason string
		expErr    error
	}{
		{
			name:      "no emails found",
			mockEmail: nil,
			mockErr:   database.MockUserEmailNotFoundErr,
			expAbused: false,
			expReason: "",
			expErr:    nil,
		},
		{
			name: "needs cool down",
			mockEmail: &database.UserEmail{
				LastVerificationSentAt: &farFuture,
			},
			mockErr:   nil,
			expAbused: true,
			expReason: "too frequent attempt since last verification email sent",
			expErr:    nil,
		},

		{
			name: "no abuse",
			mockEmail: &database.UserEmail{
				LastVerificationSentAt: &yesterday,
			},
			mockErr:   nil,
			expAbused: false,
			expReason: "",
			expErr:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userEmails := dbmocks.NewMockUserEmailsStore()
			userEmails.GetLatestVerificationSentEmailFunc.SetDefaultReturn(test.mockEmail, test.mockErr)
			db := dbmocks.NewMockDB()
			db.UserEmailsFunc.SetDefaultReturn(userEmails)

			abused, reason, err := checkEmailAbuse(context.Background(), db, "fake@localhost")
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

func TestCheckEmailFormat(t *testing.T) {
	for name, test := range map[string]struct {
		email string
		err   error
		code  int
	}{
		"valid":   {email: "foo@bar.pl", err: nil},
		"invalid": {email: "foo@", err: errors.Newf("mail: missing '@' or angle-addr")},
		"toolong": {email: "a012345678901234567890123456789012345678901234567890123456789@0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789.comeeeeqwqwwe", err: errors.Newf("maximum email length is 320, got 326")},
	} {
		t.Run(name, func(t *testing.T) {
			err := CheckEmailFormat(test.email)
			if test.err == nil {
				if err != nil {
					t.Fatalf("err: want nil but got %v", err)
				}
			} else {
				if test.err.Error() != err.Error() {
					t.Fatalf("err: want %v but got %v", test.err, err)
				}
			}
		})
	}
}

func TestHandleSignIn_Lockout(t *testing.T) {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			AuthProviders: []schema.AuthProviders{
				{
					Builtin: &schema.BuiltinAuthProvider{
						Type: providerType,
					},
				},
			},
		},
	})
	defer conf.Mock(nil)

	gss := dbmocks.NewMockGlobalStateStore()
	gss.GetFunc.SetDefaultReturn(database.GlobalState{SiteID: "a"}, nil)

	users := dbmocks.NewMockUserStore()
	users.GetByUsernameFunc.SetDefaultReturn(&types.User{ID: 1}, nil)
	db := dbmocks.NewMockDB()
	db.GlobalStateFunc.SetDefaultReturn(gss)
	db.UsersFunc.SetDefaultReturn(users)
	db.EventLogsFunc.SetDefaultReturn(dbmocks.NewMockEventLogStore())
	db.SecurityEventLogsFunc.SetDefaultReturn(dbmocks.NewMockSecurityEventLogsStore())
	db.UserEmailsFunc.SetDefaultReturn(dbmocks.NewMockUserEmailsStore())

	lockout := NewMockLockoutStore()
	logger := logtest.NoOp(t)
	if testing.Verbose() {
		logger = logtest.Scoped(t)
	}
	h := HandleSignIn(logger, db, lockout, telemetry.NewEventRecorder(telemetrytest.NewMockEventsStore()))

	// Normal authentication fail before lockout
	{
		req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
		assert.Equal(t, "Authentication failed\n", resp.Body.String())
	}

	// Getting error for locked out
	{
		lockout.IsLockedOutFunc.SetDefaultReturn("reason", true)
		lockout.SendUnlockAccountEmailFunc.SetDefaultReturn(nil)
		req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Equal(t, `Account has been locked out due to "reason"`+"\n", resp.Body.String())
	}
}

func TestHandleAccount_Unlock(t *testing.T) {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			AuthProviders: []schema.AuthProviders{
				{
					Builtin: &schema.BuiltinAuthProvider{
						Type: providerType,
					},
				},
			},
		},
	})
	defer conf.Mock(nil)

	db := dbmocks.NewMockDB()
	db.EventLogsFunc.SetDefaultReturn(dbmocks.NewMockEventLogStore())
	db.SecurityEventLogsFunc.SetDefaultReturn(dbmocks.NewMockSecurityEventLogsStore())

	lockout := NewMockLockoutStore()
	logger := logtest.NoOp(t)
	if testing.Verbose() {
		logger = logtest.Scoped(t)
	}
	h := HandleUnlockAccount(logger, db, lockout)

	// bad request if missing token or user id
	{
		req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		h(resp, req)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Equal(t, "Bad request: missing token\n", resp.Body.String())
	}

	// Getting error for invalid token
	{
		lockout.VerifyUnlockAccountTokenAndResetFunc.SetDefaultReturn(false, errors.Newf("invalid token provided"))
		req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{ "token": "abcd" }`))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
		assert.Equal(t, "invalid token provided\n", resp.Body.String())
	}

	// ok result
	{
		lockout.VerifyUnlockAccountTokenAndResetFunc.SetDefaultReturn(true, nil)
		req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{ "token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJpc3MiOiJodHRwczovL3NvdXJjZWdyYXBoLnRlc3Q6MzQ0MyIsInN1YiI6IjEiLCJleHAiOjE2NDk3NzgxNjl9.cm_giwkSviVRXGRCie9iii-ytJD3iAuNdtk9XmBZMrj7HHlH6vfky4ftjudAZ94HBp867cjxkuNc6OJ2uaEJFg" }`))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "", resp.Body.String())
	}
}

func TestHandleAccount_UnlockByAdmin(t *testing.T) {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			AuthProviders: []schema.AuthProviders{
				{
					Builtin: &schema.BuiltinAuthProvider{
						Type: providerType,
					},
				},
			},
		},
	})
	defer conf.Mock(nil)

	db := dbmocks.NewMockDB()
	db.EventLogsFunc.SetDefaultReturn(dbmocks.NewMockEventLogStore())
	db.SecurityEventLogsFunc.SetDefaultReturn(dbmocks.NewMockSecurityEventLogsStore())
	users := dbmocks.NewMockUserStore()
	db.UsersFunc.SetDefaultReturn(users)

	lockout := NewMockLockoutStore()
	logger := logtest.NoOp(t)
	if testing.Verbose() {
		logger = logtest.Scoped(t)
	}
	h := HandleUnlockUserAccount(logger, db, lockout)

	tests := []struct {
		name       string
		username   string
		userExists bool
		userLocked bool
		isAdmin    bool
		status     int
		body       string
	}{
		{
			name:    "unauthorized request if not admin",
			isAdmin: false,
			status:  http.StatusUnauthorized,
			body:    "Only site admins can unlock user accounts\n",
		},
		{
			name:    "bad request if missing username",
			isAdmin: true,
			status:  http.StatusBadRequest,
			body:    "Bad request: missing username\n",
		},
		{
			name:     "not found if user does not exist",
			username: "sguser1",
			isAdmin:  true,
			status:   http.StatusNotFound,
			body:     "Not found: could not find user with username \"sguser1\"\n",
		},
		{
			name:       "bad request if user is not locked",
			username:   "sguser1",
			userExists: true,
			isAdmin:    true,
			status:     http.StatusBadRequest,
			body:       "User with username \"sguser1\" is not locked\n",
		},
		{
			name:       "ok result",
			username:   "sguser1",
			userExists: true,
			userLocked: true,
			isAdmin:    true,
			status:     http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: test.isAdmin}, nil)

			if test.userExists {
				users.GetByUsernameFunc.SetDefaultReturn(&types.User{ID: 1, Username: test.username}, nil)
			} else {
				users.GetByUsernameFunc.SetDefaultReturn(nil, database.MockUserNotFoundErr)
			}

			lockout.IsLockedOutFunc.SetDefaultReturn("", test.userLocked)

			req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(fmt.Sprintf(`{"username": "%s"}`, test.username)))
			require.NoError(t, err)

			resp := httptest.NewRecorder()
			h(resp, req)
			assert.Equal(t, test.status, resp.Code)
			assert.Equal(t, test.body, resp.Body.String())
		})
	}
}

func TestHandleSignUp(t *testing.T) {
	t.Run("signup not allowed by provider", func(t *testing.T) {
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{
					{
						Builtin: &schema.BuiltinAuthProvider{
							Type: providerType,
						},
					},
				},
			},
		})
		defer conf.Mock(nil)

		db := dbmocks.NewMockDB()
		logger := logtest.NoOp(t)
		if testing.Verbose() {
			logger = logtest.Scoped(t)
		}

		events := telemetry.NewEventRecorder(telemetrytest.NewMockEventsStore())
		h := HandleSignUp(logger, db, events)

		req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusNotFound, resp.Code)
		assert.Equal(t, "Signup is not enabled (builtin auth provider allowSignup site configuration option)\n", resp.Body.String())
	})

	t.Run("unsupported request method", func(t *testing.T) {
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{
					{
						Builtin: &schema.BuiltinAuthProvider{
							Type:        providerType,
							AllowSignup: true,
						},
					},
				},
			},
		})
		defer conf.Mock(nil)

		db := dbmocks.NewMockDB()
		logger := logtest.NoOp(t)
		if testing.Verbose() {
			logger = logtest.Scoped(t)
		}

		h := HandleSignUp(logger, db, telemetry.NewEventRecorder(telemetrytest.NewMockEventsStore()))

		req, err := http.NewRequest(http.MethodGet, "/", strings.NewReader(`{}`))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Equal(t, fmt.Sprintf("unsupported method %s\n", http.MethodGet), resp.Body.String())
	})

	t.Run("success", func(t *testing.T) {
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{
					{
						Builtin: &schema.BuiltinAuthProvider{
							Type:        providerType,
							AllowSignup: true,
						},
					},
				},
				ExperimentalFeatures: &schema.ExperimentalFeatures{
					EventLogging: "disabled",
				},
			},
		})
		defer conf.Mock(nil)

		cleanup := session.ResetMockSessionStore(t)
		defer cleanup()

		users := dbmocks.NewMockUserStore()
		users.CreateFunc.SetDefaultHook(func(ctx context.Context, nu database.NewUser) (*types.User, error) {
			if nu.EmailIsVerified == true {
				t.Fatal("expected newUser.EmailIsVerified to be false but got true")
			}
			if nu.EmailVerificationCode == "" {
				t.Fatal("expected newUser.EmailVerficationCode to be non-empty")
			}
			return &types.User{ID: 1, SiteAdmin: false, CreatedAt: time.Now()}, nil
		})

		authz := dbmocks.NewMockAuthzStore()
		authz.GrantPendingPermissionsFunc.SetDefaultReturn(nil)

		eventLogs := dbmocks.NewMockEventLogStore()
		eventLogs.BulkInsertFunc.SetDefaultReturn(nil)

		db := dbmocks.NewMockDB()
		db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
			return f(db)
		})
		db.UsersFunc.SetDefaultReturn(users)
		db.AuthzFunc.SetDefaultReturn(authz)
		db.EventLogsFunc.SetDefaultReturn(eventLogs)

		gss := dbmocks.NewMockGlobalStateStore()
		gss.GetFunc.SetDefaultReturn(database.GlobalState{SiteID: "a"}, nil)
		db.GlobalStateFunc.SetDefaultReturn(gss)

		logger := logtest.NoOp(t)
		if testing.Verbose() {
			logger = logtest.Scoped(t)
		}

		h := HandleSignUp(logger, db, telemetry.NewEventRecorder(telemetrytest.NewMockEventsStore()))

		body := strings.NewReader(`{
			"email": "test@test.com",
			"username": "test-user",
			"password": "somerandomhardtoguesspassword123456789"
		}`)
		req, err := http.NewRequest(http.MethodPost, "/", body)
		require.NoError(t, err)
		req.Header.Set("User-Agent", "test")

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "", resp.Body.String())

		mockrequire.CalledOnce(t, authz.GrantPendingPermissionsFunc)
		mockrequire.CalledOnce(t, users.CreateFunc)
	})
}

func TestHandleSiteInit(t *testing.T) {
	t.Run("unsupported request method", func(t *testing.T) {
		db := dbmocks.NewMockDB()
		logger := logtest.NoOp(t)
		if testing.Verbose() {
			logger = logtest.Scoped(t)
		}

		h := HandleSiteInit(logger, db, telemetry.NewEventRecorder(telemetrytest.NewMockEventsStore()))

		req, err := http.NewRequest(http.MethodGet, "/", strings.NewReader(`{}`))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Equal(t, fmt.Sprintf("unsupported method %s\n", http.MethodGet), resp.Body.String())
	})

	t.Run("success", func(t *testing.T) {
		cleanup := session.ResetMockSessionStore(t)
		defer cleanup()

		users := dbmocks.NewMockUserStore()
		users.CreateFunc.SetDefaultHook(func(ctx context.Context, nu database.NewUser) (*types.User, error) {
			if nu.EmailIsVerified == false {
				t.Fatal("expected newUser.EmailIsVerified to be true but got false")
			}
			if nu.EmailVerificationCode != "" {
				t.Fatalf("expected newUser.EmailVerficationCode to be empty, got %s", nu.EmailVerificationCode)
			}
			return &types.User{ID: 1, SiteAdmin: true, CreatedAt: time.Now()}, nil
		})

		authz := dbmocks.NewMockAuthzStore()
		authz.GrantPendingPermissionsFunc.SetDefaultReturn(nil)

		eventLogs := dbmocks.NewMockEventLogStore()
		eventLogs.BulkInsertFunc.SetDefaultReturn(nil)

		db := dbmocks.NewMockDB()
		db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
			return f(db)
		})
		db.UsersFunc.SetDefaultReturn(users)
		db.AuthzFunc.SetDefaultReturn(authz)
		db.EventLogsFunc.SetDefaultReturn(eventLogs)

		logger := logtest.NoOp(t)
		if testing.Verbose() {
			logger = logtest.Scoped(t)
		}

		h := HandleSiteInit(logger, db, telemetry.NewEventRecorder(telemetrytest.NewMockEventsStore()))

		body := strings.NewReader(`{
			"email": "test@test.com",
			"username": "test-user",
			"password": "somerandomhardtoguesspassword123456789"
		}`)
		req, err := http.NewRequest(http.MethodPost, "/", body)
		require.NoError(t, err)
		req.Header.Set("User-Agent", "test")

		resp := httptest.NewRecorder()
		h(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, "", resp.Body.String())

		mockrequire.CalledOnce(t, authz.GrantPendingPermissionsFunc)
		mockrequire.CalledOnce(t, users.CreateFunc)
		mockrequire.CalledOnce(t, eventLogs.BulkInsertFunc)
	})
}
