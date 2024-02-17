package sourcegraphoperator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc"
	mockrequire "github.com/derision-test/go-mockgen/testutil/require"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/openidconnect"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	internalauth "github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/cloud"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	testOIDCUser = "testOIDCUser"
	testClientID = "testClientID"
	testIDToken  = "testIDToken"
)

// new OIDCIDServer returns a new running mock OIDC ID provider service. It is
// the caller's responsibility to call Close().
func newOIDCIDServer(t *testing.T, code string, providerConfig *cloud.SchemaAuthProviderSourcegraphOperator) (server *httptest.Server, emailPtr *string) {
	s := http.NewServeMux()

	s.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(
			map[string]string{
				"issuer":                 providerConfig.Issuer,
				"authorization_endpoint": providerConfig.Issuer + "/oauth2/v1/authorize",
				"token_endpoint":         providerConfig.Issuer + "/oauth2/v1/token",
				"userinfo_endpoint":      providerConfig.Issuer + "/oauth2/v1/userinfo",
			},
		)
		require.NoError(t, err)
	})
	s.HandleFunc("/oauth2/v1/token", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, code, values.Get("code"))
		require.Equal(t, "authorization_code", values.Get("grant_type"))

		redirectURI, err := url.QueryUnescape(values.Get("redirect_uri"))
		require.NoError(t, err)
		require.Equal(t, "http://example.com/.auth/sourcegraph-operator/callback", redirectURI)

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(
			map[string]any{
				"access_token": "testAccessToken",
				"token_type":   "Bearer",
				"expires_in":   3600,
				"scope":        "openid",
				"id_token":     testIDToken,
			},
		)
		require.NoError(t, err)
	})
	email := "alice@sourcegraph.com"
	s.HandleFunc("/oauth2/v1/userinfo", func(w http.ResponseWriter, r *http.Request) {
		authzHeader := r.Header.Get("Authorization")
		authzParts := strings.Split(authzHeader, " ")
		require.Len(t, authzParts, 2)
		require.Equal(t, "Bearer", authzParts[0])

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(
			map[string]any{
				"sub":            testOIDCUser,
				"profile":        "This is a profile",
				"email":          email,
				"email_verified": true,
				"picture":        "http://example.com/picture.png",
			},
		)
		require.NoError(t, err)
	})

	auth.MockGetAndSaveUser = func(ctx context.Context, op auth.GetAndSaveUserOp) (newUserCreated bool, userID int32, safeErrMsg string, err error) {
		if op.ExternalAccount.ServiceType == internalauth.SourcegraphOperatorProviderType &&
			op.ExternalAccount.ServiceID == providerConfig.Issuer &&
			op.ExternalAccount.ClientID == testClientID &&
			op.ExternalAccount.AccountID == testOIDCUser {
			return false, 123, "", nil
		}
		return false, 0, "safeErr", errors.Errorf("account %q not found in mock", op.ExternalAccount)
	}
	t.Cleanup(func() {
		auth.MockGetAndSaveUser = nil
	})
	return httptest.NewServer(s), &email
}

type doRequestFunc func(method, urlStr, body string, authed bool, state string) *http.Response

type mockDetails struct {
	usersStore            *dbmocks.MockUserStore
	externalAccountsStore *dbmocks.MockUserExternalAccountsStore
	doRequest             doRequestFunc
}

func newMockDBAndRequester(t *testing.T) mockDetails {
	t.Helper()
	usersStore := dbmocks.NewMockUserStore()
	userExternalAccountsStore := dbmocks.NewMockUserExternalAccountsStore()
	userExternalAccountsStore.ListFunc.SetDefaultReturn(
		[]*extsvc.Account{
			{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: internalauth.SourcegraphOperatorProviderType,
				},
			},
		},
		nil,
	)
	usersStore.SetIsSiteAdminFunc.SetDefaultReturn(nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(usersStore)
	db.UserExternalAccountsFunc.SetDefaultReturn(userExternalAccountsStore)
	db.SecurityEventLogsFunc.SetDefaultReturn(dbmocks.NewMockSecurityEventLogsStore())

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	authedHandler := http.NewServeMux()
	authedHandler.Handle("/.api/", Middleware(db).API(h))
	authedHandler.Handle("/", Middleware(db).App(h))

	doRequest := func(method, urlStr, body string, authed bool, state string) *http.Response {
		req := httptest.NewRequest(method, urlStr, bytes.NewBufferString(body))
		if authed {
			req = req.WithContext(actor.WithActor(context.Background(), &actor.Actor{UID: 1}))
		}
		resp := httptest.NewRecorder()
		if state != "" {
			require.NoError(t, session.SetData(resp, req, "oidcState", state))
		}
		authedHandler.ServeHTTP(resp, req)
		return resp.Result()
	}

	return mockDetails{
		usersStore:            usersStore,
		externalAccountsStore: userExternalAccountsStore,
		doRequest:             doRequest,
	}
}

func TestMiddleware(t *testing.T) {
	cleanup := session.ResetMockSessionStore(t)
	defer cleanup()

	const testCode = "testCode"
	providerConfig := cloud.SchemaAuthProviderSourcegraphOperator{
		ClientID:          testClientID,
		ClientSecret:      "testClientSecret",
		LifecycleDuration: 60,
	}
	oidcIDServer, emailPtr := newOIDCIDServer(t, testCode, &providerConfig)
	defer oidcIDServer.Close()
	providerConfig.Issuer = oidcIDServer.URL

	cli, err := httpcli.NewFactory(nil).Doer()
	require.NoError(t, err)

	mockProvider := NewProvider(providerConfig, cli).(*provider)
	providers.MockProviders = []providers.Provider{mockProvider}
	defer func() { providers.MockProviders = nil }()

	t.Run("refresh", func(t *testing.T) {
		err := mockProvider.Refresh(context.Background())
		require.NoError(t, err)
	})

	t.Run("unauthenticated API request should pass through", func(t *testing.T) {
		mocks := newMockDBAndRequester(t)

		resp := mocks.doRequest(http.MethodGet, "http://example.com/.api/foo", "", false, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("login triggers auth flow", func(t *testing.T) {
		mocks := newMockDBAndRequester(t)

		urlStr := fmt.Sprintf("http://example.com%s/login?pc=%s", authPrefix, mockProvider.ConfigID().ID)
		resp := mocks.doRequest(http.MethodGet, urlStr, "", false, "")
		assert.Equal(t, http.StatusFound, resp.StatusCode)

		location := resp.Header.Get("Location")
		wantPrefix := mockProvider.config.Issuer + "/"
		assert.True(t, strings.HasPrefix(location, wantPrefix), "%q does not have prefix %q", location, wantPrefix)

		loginURL, err := url.Parse(location)
		require.NoError(t, err)
		assert.Equal(t, mockProvider.config.ClientID, loginURL.Query().Get("client_id"))
		assert.Equal(t, "http://example.com/.auth/sourcegraph-operator/callback", loginURL.Query().Get("redirect_uri"))
		assert.Equal(t, "code", loginURL.Query().Get("response_type"))
		assert.Equal(t, "openid profile email", loginURL.Query().Get("scope"))
	})

	t.Run("callback with bad CSRF should fail", func(t *testing.T) {
		mocks := newMockDBAndRequester(t)

		badState := &openidconnect.AuthnState{
			CSRFToken:  "bad",
			Redirect:   "/redirect",
			ProviderID: mockProvider.ConfigID().ID,
		}
		urlStr := fmt.Sprintf("http://example.com/.auth/sourcegraph-operator/callback?code=%s&state=%s", testCode, badState.Encode())
		resp := mocks.doRequest(http.MethodGet, urlStr, "", false, badState.Encode())
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("callback with good CSRF should set auth cookie", func(t *testing.T) {
		mocks := newMockDBAndRequester(t)

		state := &openidconnect.AuthnState{
			CSRFToken:  "good",
			Redirect:   "/redirect",
			ProviderID: mockProvider.ConfigID().ID,
		}
		openidconnect.MockVerifyIDToken = func(rawIDToken string) *oidc.IDToken {
			require.Equal(t, testIDToken, rawIDToken)
			return &oidc.IDToken{
				Issuer:  oidcIDServer.URL,
				Subject: testOIDCUser,
				Expiry:  time.Now().Add(time.Hour),
				Nonce:   state.Encode(),
			}
		}
		defer func() { openidconnect.MockVerifyIDToken = nil }()

		mocks.usersStore.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.User, error) {
			return &types.User{
				ID:        id,
				CreatedAt: time.Now(),
			}, nil
		})
		mocks.usersStore.CreateWithExternalAccountFunc.SetDefaultHook(func(_ context.Context, user database.NewUser, _ *extsvc.Account) (*types.User, error) {
			assert.True(t, strings.HasPrefix(user.Username, usernamePrefix), "%q does not have prefix %q", user.Username, usernamePrefix)
			return &types.User{ID: 1}, nil
		})

		urlStr := fmt.Sprintf("http://example.com/.auth/sourcegraph-operator/callback?code=%s&state=%s", testCode, state.Encode())
		resp := mocks.doRequest(http.MethodGet, urlStr, "", false, state.Encode())
		assert.Equal(t, http.StatusFound, resp.StatusCode)
		wantRedirect := fmt.Sprintf(`%s?signin=OpenIDConnect`, state.Redirect)
		assert.Equal(t, wantRedirect, resp.Header.Get("Location"))
		mockrequire.CalledOnce(t, mocks.usersStore.SetIsSiteAdminFunc)
	})

	t.Run("callback with bad email domain should fail", func(t *testing.T) {
		mocks := newMockDBAndRequester(t)

		oldEmail := *emailPtr
		*emailPtr = "alice@example.com" // Doesn't match requiredEmailDomain
		defer func() { *emailPtr = oldEmail }()

		state := &openidconnect.AuthnState{
			CSRFToken:  "good",
			Redirect:   "/redirect",
			ProviderID: mockProvider.ConfigID().ID,
		}
		openidconnect.MockVerifyIDToken = func(rawIDToken string) *oidc.IDToken {
			require.Equal(t, testIDToken, rawIDToken)
			return &oidc.IDToken{
				Issuer:  oidcIDServer.URL,
				Subject: testOIDCUser,
				Expiry:  time.Now().Add(time.Hour),
				Nonce:   state.Encode(),
			}
		}
		defer func() { openidconnect.MockVerifyIDToken = nil }()

		urlStr := fmt.Sprintf("http://example.com/.auth/sourcegraph-operator/callback?code=%s&state=%s", testCode, state.Encode())
		resp := mocks.doRequest(http.MethodGet, urlStr, "", false, state.Encode())
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("no open redirection", func(t *testing.T) {
		mocks := newMockDBAndRequester(t)

		state := &openidconnect.AuthnState{
			CSRFToken:  "good",
			Redirect:   "https://evil.com",
			ProviderID: mockProvider.ConfigID().ID,
		}
		openidconnect.MockVerifyIDToken = func(rawIDToken string) *oidc.IDToken {
			require.Equal(t, testIDToken, rawIDToken)
			return &oidc.IDToken{
				Issuer:  oidcIDServer.URL,
				Subject: testOIDCUser,
				Expiry:  time.Now().Add(time.Hour),
				Nonce:   state.Encode(),
			}
		}
		defer func() { openidconnect.MockVerifyIDToken = nil }()

		mocks.usersStore.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.User, error) {
			return &types.User{
				ID:        id,
				CreatedAt: time.Now(),
			}, nil
		})

		urlStr := fmt.Sprintf("http://example.com/.auth/sourcegraph-operator/callback?code=%s&state=%s", testCode, state.Encode())
		resp := mocks.doRequest(http.MethodGet, urlStr, "", false, state.Encode())
		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/", resp.Header.Get("Location"))
		mockrequire.CalledOnce(t, mocks.usersStore.SetIsSiteAdminFunc)
	})

	t.Run("lifetime expired", func(t *testing.T) {
		mocks := newMockDBAndRequester(t)

		mocks.usersStore.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.User, error) {
			return &types.User{
				ID:        id,
				CreatedAt: time.Now().Add(-61 * time.Minute),
			}, nil
		})
		mocks.usersStore.HardDeleteFunc.SetDefaultHook(func(ctx context.Context, _ int32) error {
			require.True(t, actor.FromContext(ctx).SourcegraphOperator, "the actor should be a Sourcegraph operator")
			return nil
		})

		state := &openidconnect.AuthnState{
			CSRFToken:  "good",
			Redirect:   "https://evil.com",
			ProviderID: mockProvider.ConfigID().ID,
		}
		openidconnect.MockVerifyIDToken = func(rawIDToken string) *oidc.IDToken {
			require.Equal(t, testIDToken, rawIDToken)
			return &oidc.IDToken{
				Issuer:  oidcIDServer.URL,
				Subject: testOIDCUser,
				Expiry:  time.Now().Add(time.Hour),
				Nonce:   state.Encode(),
			}
		}
		defer func() { openidconnect.MockVerifyIDToken = nil }()

		urlStr := fmt.Sprintf("http://example.com/.auth/sourcegraph-operator/callback?code=%s&state=%s", testCode, state.Encode())
		resp := mocks.doRequest(http.MethodGet, urlStr, "", false, state.Encode())
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "The retrieved user account lifecycle has already expired")
		mockrequire.Called(t, mocks.usersStore.HardDeleteFunc)
	})
}
