package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestAccessTokenAuthMiddleware(t *testing.T) {
	newHandler := func(db database.DB) http.Handler {
		return AccessTokenAuthMiddleware(
			db,
			logtest.NoOp(t),
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actor := sgactor.FromContext(r.Context())
				if actor.IsAuthenticated() {
					_, _ = fmt.Fprintf(w, "user %v", actor.UID)
				} else {
					_, _ = fmt.Fprint(w, "no user")
				}
			}))
	}

	checkHTTPResponse := func(t *testing.T, db database.DB, req *http.Request, wantStatusCode int, wantBody string) {
		rr := httptest.NewRecorder()
		newHandler(db).ServeHTTP(rr, req)
		if rr.Code != wantStatusCode {
			t.Errorf("got response status %d, want %d", rr.Code, wantStatusCode)
		}
		if got := rr.Body.String(); got != wantBody {
			t.Errorf("got response body %q, want %q", got, wantBody)
		}
	}

	db := dbmocks.NewMockDB()
	db.UserExternalAccountsFunc.SetDefaultReturn(dbmocks.NewMockUserExternalAccountsStore())
	t.Run("no header", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		checkHTTPResponse(t, db, req, http.StatusOK, "no user")
	})

	// Test that the absence of an Authorization header doesn't unset the actor provided by a prior
	// auth middleware.
	t.Run("no header, actor present", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req = req.WithContext(sgactor.WithActor(context.Background(), &sgactor.Actor{UID: 123}))
		checkHTTPResponse(t, db, req, http.StatusOK, "user 123")
	})

	for _, unrecognizedHeaderValue := range []string{"x", "x y", "Basic abcd"} {
		t.Run("unrecognized header "+unrecognizedHeaderValue, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", unrecognizedHeaderValue)
			checkHTTPResponse(t, db, req, http.StatusOK, "no user")
		})
	}

	for _, invalidHeaderValue := range []string{"token-sudo abc", `token-sudo token=""`, "token "} {
		t.Run("invalid header "+invalidHeaderValue, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", invalidHeaderValue)
			checkHTTPResponse(t, db, req, http.StatusUnauthorized, "Invalid Authorization header.\n")
		})
	}

	t.Run("license check bypasses handler in dotcom mode", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		req, _ := http.NewRequest("GET", "/.api/license/check", nil)
		req.Header.Set("Authorization", "Bearer sometoken")
		checkHTTPResponse(t, db, req, http.StatusOK, "no user")
	})

	t.Run("valid header with invalid token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "token badbad")

		accessTokens := dbmocks.NewMockAccessTokenStore()
		accessTokens.LookupFunc.SetDefaultReturn(0, database.InvalidTokenError{})
		db.AccessTokensFunc.SetDefaultReturn(accessTokens)

		securityEventLogs := dbmocks.NewMockSecurityEventLogsStore()
		securityEventLogs.LogSecurityEventFunc.SetDefaultHook(func(ctx context.Context, eventName database.SecurityEventName, url string, userID uint32, anonymousUserID string, source string, arguments any) error {
			if want := database.SecurityEventAccessTokenInvalid; eventName != want {
				t.Errorf("got %q, want %q", eventName, want)
			}
			return nil
		})
		db.SecurityEventLogsFunc.SetDefaultReturn(securityEventLogs)

		checkHTTPResponse(t, db, req, http.StatusUnauthorized, "Invalid access token.\n")
		mockrequire.Called(t, accessTokens.LookupFunc)
		mockrequire.Called(t, securityEventLogs.LogSecurityEventFunc)
	})

	for _, headerValue := range []string{"token abcdef", `token token="abcdef"`} {
		t.Run("valid non-sudo token: "+headerValue, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", headerValue)

			accessTokens := dbmocks.NewMockAccessTokenStore()
			accessTokens.LookupFunc.SetDefaultHook(func(_ context.Context, tokenHexEncoded string, opts database.TokenLookupOpts) (subjectUserID int32, err error) {
				if want := "abcdef"; tokenHexEncoded != want {
					t.Errorf("got %q, want %q", tokenHexEncoded, want)
				}
				if want := authz.ScopeUserAll; opts.RequiredScope != want {
					t.Errorf("got %q, want %q", opts.RequiredScope, want)
				}
				return 123, nil
			})
			db.AccessTokensFunc.SetDefaultReturn(accessTokens)

			checkHTTPResponse(t, db, req, http.StatusOK, "user 123")
			mockrequire.Called(t, accessTokens.LookupFunc)
		})
	}

	// Test that an access token overwrites the actor set by a prior auth middleware.
	t.Run("actor present, valid non-sudo token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "token abcdef")
		req = req.WithContext(sgactor.WithActor(context.Background(), &sgactor.Actor{UID: 456}))

		accessTokens := dbmocks.NewMockAccessTokenStore()
		accessTokens.LookupFunc.SetDefaultHook(func(_ context.Context, tokenHexEncoded string, opts database.TokenLookupOpts) (subjectUserID int32, err error) {
			if want := "abcdef"; tokenHexEncoded != want {
				t.Errorf("got %q, want %q", tokenHexEncoded, want)
			}
			if want := authz.ScopeUserAll; opts.RequiredScope != want {
				t.Errorf("got %q, want %q", opts.RequiredScope, want)
			}
			return 123, nil
		})
		db.AccessTokensFunc.SetDefaultReturn(accessTokens)

		checkHTTPResponse(t, db, req, http.StatusOK, "user 123")
		mockrequire.Called(t, accessTokens.LookupFunc)
	})

	// Test that an access token overwrites the actor set by a prior auth middleware.
	const (
		sourceQueryParam = "query-param"
		sourceBasicAuth  = "basic-auth"
	)
	for _, source := range []string{sourceQueryParam, sourceBasicAuth} {
		t.Run("actor present, valid non-sudo token in "+source, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			if source == sourceQueryParam {
				q := url.Values{}
				q.Add("token", "abcdef")
				req.URL.RawQuery = q.Encode()
			} else {
				req.SetBasicAuth("abcdef", "")
			}
			req = req.WithContext(sgactor.WithActor(context.Background(), &sgactor.Actor{UID: 456}))

			accessTokens := dbmocks.NewMockAccessTokenStore()
			accessTokens.LookupFunc.SetDefaultHook(func(_ context.Context, tokenHexEncoded string, opts database.TokenLookupOpts) (subjectUserID int32, err error) {
				if want := "abcdef"; tokenHexEncoded != want {
					t.Errorf("got %q, want %q", tokenHexEncoded, want)
				}
				if want := authz.ScopeUserAll; opts.RequiredScope != want {
					t.Errorf("got %q, want %q", opts.RequiredScope, want)
				}
				return 123, nil
			})
			db.AccessTokensFunc.SetDefaultReturn(accessTokens)

			checkHTTPResponse(t, db, req, http.StatusOK, "user 123")
			mockrequire.Called(t, accessTokens.LookupFunc)
		})
	}

	t.Run("valid sudo token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", `token-sudo token="abcdef",user="alice"`)

		accessTokens := dbmocks.NewMockAccessTokenStore()
		accessTokens.LookupFunc.SetDefaultHook(func(_ context.Context, tokenHexEncoded string, opts database.TokenLookupOpts) (subjectUserID int32, err error) {
			if want := "abcdef"; tokenHexEncoded != want {
				t.Errorf("got %q, want %q", tokenHexEncoded, want)
			}
			if want := authz.ScopeSiteAdminSudo; opts.RequiredScope != want {
				t.Errorf("got %q, want %q", opts.RequiredScope, want)
			}
			return 123, nil
		})

		users := dbmocks.NewMockUserStore()
		users.GetByIDFunc.SetDefaultHook(func(ctx context.Context, userID int32) (*types.User, error) {
			if want := int32(123); userID != want {
				t.Errorf("got %d, want %d", userID, want)
			}
			return &types.User{ID: userID, SiteAdmin: true}, nil
		})
		users.GetByUsernameFunc.SetDefaultHook(func(ctx context.Context, username string) (*types.User, error) {
			if want := "alice"; username != want {
				t.Errorf("got %q, want %q", username, want)
			}
			return &types.User{ID: 456, SiteAdmin: true}, nil
		})

		securityEventLogs := dbmocks.NewMockSecurityEventLogsStore()
		securityEventLogs.LogSecurityEventFunc.SetDefaultHook(func(ctx context.Context, eventName database.SecurityEventName, url string, userID uint32, anonymousUserID string, source string, arguments any) error {
			if want := database.SecurityEventAccessTokenImpersonated; eventName != want {
				t.Errorf("got %q, want %q", eventName, want)
			}
			return nil
		})

		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)
		db.SecurityEventLogsFunc.SetDefaultReturn(securityEventLogs)

		checkHTTPResponse(t, db, req, http.StatusOK, "user 456")
		mockrequire.Called(t, accessTokens.LookupFunc)
		mockrequire.Called(t, users.GetByIDFunc)
		mockrequire.Called(t, users.GetByUsernameFunc)
		mockrequire.Called(t, securityEventLogs.LogSecurityEventFunc)
	})

	t.Run("valid sudo token as a Sourcegraph operator", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", `token-sudo token="abcdef",user="alice"`)

		accessTokens := dbmocks.NewMockAccessTokenStore()
		accessTokens.LookupFunc.SetDefaultHook(func(_ context.Context, tokenHexEncoded string, opts database.TokenLookupOpts) (subjectUserID int32, err error) {
			if want := "abcdef"; tokenHexEncoded != want {
				t.Errorf("got %q, want %q", tokenHexEncoded, want)
			}
			if want := authz.ScopeSiteAdminSudo; opts.RequiredScope != want {
				t.Errorf("got %q, want %q", opts.RequiredScope, want)
			}
			return 123, nil
		})

		users := dbmocks.NewMockUserStore()
		users.GetByIDFunc.SetDefaultHook(func(ctx context.Context, userID int32) (*types.User, error) {
			if want := int32(123); userID != want {
				t.Errorf("got %d, want %d", userID, want)
			}
			return &types.User{ID: userID, SiteAdmin: true}, nil
		})
		users.GetByUsernameFunc.SetDefaultHook(func(ctx context.Context, username string) (*types.User, error) {
			if want := "alice"; username != want {
				t.Errorf("got %q, want %q", username, want)
			}
			return &types.User{ID: 456, SiteAdmin: true}, nil
		})

		userExternalAccountsStore := dbmocks.NewMockUserExternalAccountsStore()
		userExternalAccountsStore.CountFunc.SetDefaultReturn(1, nil)

		securityEventLogsStore := dbmocks.NewMockSecurityEventLogsStore()
		securityEventLogsStore.LogSecurityEventFunc.SetDefaultHook(func(ctx context.Context, eventName database.SecurityEventName, url string, userID uint32, anonymousUserID string, source string, arguments any) error {
			require.True(t, sgactor.FromContext(ctx).SourcegraphOperator, "the actor should be a Sourcegraph operator")
			return nil
		})

		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)
		db.UserExternalAccountsFunc.SetDefaultReturn(userExternalAccountsStore)
		db.SecurityEventLogsFunc.SetDefaultReturn(securityEventLogsStore)

		checkHTTPResponse(t, db, req, http.StatusOK, "user 456")
		mockrequire.Called(t, accessTokens.LookupFunc)
		mockrequire.Called(t, users.GetByIDFunc)
		mockrequire.Called(t, users.GetByUsernameFunc)
		mockrequire.Called(t, securityEventLogsStore.LogSecurityEventFunc)
	})

	// Test that if a sudo token's subject user is not a site admin (which means they were demoted
	// from site admin AFTER the token was created), then the sudo token is invalid.
	t.Run("valid sudo token, subject is not site admin", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", `token-sudo token="abcdef",user="alice"`)

		accessTokens := dbmocks.NewMockAccessTokenStore()
		accessTokens.LookupFunc.SetDefaultHook(func(_ context.Context, tokenHexEncoded string, opts database.TokenLookupOpts) (subjectUserID int32, err error) {
			if want := "abcdef"; tokenHexEncoded != want {
				t.Errorf("got %q, want %q", tokenHexEncoded, want)
			}
			if want := authz.ScopeSiteAdminSudo; opts.RequiredScope != want {
				t.Errorf("got %q, want %q", opts.RequiredScope, want)
			}
			return 123, nil
		})

		users := dbmocks.NewMockUserStore()
		users.GetByIDFunc.SetDefaultHook(func(ctx context.Context, userID int32) (*types.User, error) {
			if want := int32(123); userID != want {
				t.Errorf("got %d, want %d", userID, want)
			}
			return &types.User{ID: userID, SiteAdmin: false}, nil
		})

		securityEventLogsStore := dbmocks.NewMockSecurityEventLogsStore()
		securityEventLogsStore.LogSecurityEventFunc.SetDefaultHook(func(ctx context.Context, eventName database.SecurityEventName, url string, userID uint32, anonymousUserID string, source string, arguments any) error {
			if want := database.SecurityEventAccessTokenSubjectNotSiteAdmin; eventName != want {
				t.Errorf("got %q, want %q", eventName, want)
			}
			return nil
		})

		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)
		db.SecurityEventLogsFunc.SetDefaultReturn(securityEventLogsStore)

		checkHTTPResponse(t, db, req, http.StatusForbidden, "The subject user of a sudo access token must be a site admin.\n")
		mockrequire.Called(t, accessTokens.LookupFunc)
		mockrequire.Called(t, users.GetByIDFunc)
		mockrequire.Called(t, securityEventLogsStore.LogSecurityEventFunc)
	})

	t.Run("valid sudo token, invalid sudo user", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", `token-sudo token="abcdef",user="doesntexist"`)

		accessTokens := dbmocks.NewMockAccessTokenStore()
		accessTokens.LookupFunc.SetDefaultHook(func(_ context.Context, tokenHexEncoded string, opts database.TokenLookupOpts) (subjectUserID int32, err error) {
			if want := "abcdef"; tokenHexEncoded != want {
				t.Errorf("got %q, want %q", tokenHexEncoded, want)
			}
			if want := authz.ScopeSiteAdminSudo; opts.RequiredScope != want {
				t.Errorf("got %q, want %q", opts.RequiredScope, want)
			}
			return 123, nil
		})

		users := dbmocks.NewMockUserStore()
		users.GetByIDFunc.SetDefaultHook(func(ctx context.Context, userID int32) (*types.User, error) {
			if want := int32(123); userID != want {
				t.Errorf("got %d, want %d", userID, want)
			}
			return &types.User{ID: userID, SiteAdmin: true}, nil
		})
		users.GetByUsernameFunc.SetDefaultHook(func(ctx context.Context, username string) (*types.User, error) {
			if want := "doesntexist"; username != want {
				t.Errorf("got %q, want %q", username, want)
			}
			return nil, &errcode.Mock{IsNotFound: true}
		})

		db.AccessTokensFunc.SetDefaultReturn(accessTokens)
		db.UsersFunc.SetDefaultReturn(users)

		checkHTTPResponse(t, db, req, http.StatusForbidden, "Unable to sudo to nonexistent user.\n")
		mockrequire.Called(t, accessTokens.LookupFunc)
		mockrequire.Called(t, users.GetByIDFunc)
		mockrequire.Called(t, users.GetByUsernameFunc)
	})
}
