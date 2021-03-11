package openidconnect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

// providerJSON is the JSON structure the OIDC provider returns at its discovery endpoing
type providerJSON struct {
	Issuer      string `json:"issuer"`
	AuthURL     string `json:"authorization_endpoint"`
	TokenURL    string `json:"token_endpoint"`
	JWKSURL     string `json:"jwks_uri"`
	UserInfoURL string `json:"userinfo_endpoint"`
}

var (
	testOIDCUser = "bob-test-user"
	testClientID = "aaaaaaaaaaaaaa"
)

// new OIDCIDServer returns a new running mock OIDC ID Provider service. It is the caller's
// responsibility to call Close().
func newOIDCIDServer(t *testing.T, code string, oidcProvider *schema.OpenIDConnectAuthProvider) (server *httptest.Server, emailPtr *string) {
	idBearerToken := "test_id_token_f4bdefbd77f"
	s := http.NewServeMux()

	s.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(providerJSON{
			Issuer:      oidcProvider.Issuer,
			AuthURL:     oidcProvider.Issuer + "/oauth2/v1/authorize",
			TokenURL:    oidcProvider.Issuer + "/oauth2/v1/token",
			UserInfoURL: oidcProvider.Issuer + "/oauth2/v1/userinfo",
		})
	})
	s.HandleFunc("/oauth2/v1/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		b, _ := ioutil.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(b))

		if values.Get("code") != code {
			t.Errorf("got code %q, want %q", values.Get("code"), code)
		}
		if got, want := values.Get("grant_type"), "authorization_code"; got != want {
			t.Errorf("got grant_type %v, want %v", got, want)
		}
		redirectURI, _ := url.QueryUnescape(values.Get("redirect_uri"))
		if want := "http://example.com/.auth/callback"; redirectURI != want {
			t.Errorf("got redirect_uri %v, want %v", redirectURI, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"access_token": "aaaaa",
			"token_type": "Bearer",
			"expires_in": 3600,
			"scope": "openid",
			"id_token": %q
		}`, idBearerToken)))
	})
	email := "bob@example.com"
	s.HandleFunc("/oauth2/v1/userinfo", func(w http.ResponseWriter, r *http.Request) {
		authzHeader := r.Header.Get("Authorization")
		authzParts := strings.Split(authzHeader, " ")
		if len(authzParts) != 2 {
			t.Fatalf("Expected 2 parts to authz header, instead got %d: %q", len(authzParts), authzHeader)
		}
		if authzParts[0] != "Bearer" {
			t.Fatalf("No bearer token found in authz header %q", authzHeader)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"sub": %q,
			"profile": "This is a profile",
			"email": "`+email+`",
			"email_verified": true,
			"picture": "https://example.com/picture.png"
		}`, testOIDCUser)))
	})

	srv := httptest.NewServer(s)

	auth.MockGetAndSaveUser = func(ctx context.Context, op auth.GetAndSaveUserOp) (userID int32, safeErrMsg string, err error) {
		if op.ExternalAccount.ServiceType == "openidconnect" && op.ExternalAccount.ServiceID == oidcProvider.Issuer && op.ExternalAccount.ClientID == testClientID && op.ExternalAccount.AccountID == testOIDCUser {
			return 123, "", nil
		}
		return 0, "safeErr", fmt.Errorf("account %v not found in mock", op.ExternalAccount)
	}

	return srv, &email
}

func TestMiddleware(t *testing.T) {
	cleanup := session.ResetMockSessionStore(t)
	defer cleanup()

	defer licensing.TestingSkipFeatureChecks()()

	tempdir, err := ioutil.TempDir("", "sourcegraph-oidc-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)

	mockGetProviderValue = &provider{
		config: schema.OpenIDConnectAuthProvider{
			ClientID:           testClientID,
			ClientSecret:       "aaaaaaaaaaaaaaaaaaaaaaaaa",
			RequireEmailDomain: "example.com",
		},
	}
	defer func() { mockGetProviderValue = nil }()
	providers.MockProviders = []providers.Provider{mockGetProviderValue}
	defer func() { providers.MockProviders = nil }()

	oidcIDServer, emailPtr := newOIDCIDServer(t, "THECODE", &mockGetProviderValue.config)
	defer oidcIDServer.Close()
	defer func() { auth.MockGetAndSaveUser = nil }()
	mockGetProviderValue.config.Issuer = oidcIDServer.URL

	database.Mocks.Users.GetByID = func(ctx context.Context, id int32) (*types.User, error) {
		return &types.User{ID: id, CreatedAt: time.Now()}, nil
	}
	defer func() { database.Mocks = database.MockStores{} }()

	if err := mockGetProviderValue.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	validState := (&authnState{CSRFToken: "THE_CSRF_TOKEN", Redirect: "/redirect", ProviderID: mockGetProviderValue.ConfigID().ID}).Encode()
	mockVerifyIDToken = func(rawIDToken string) *oidc.IDToken {
		if rawIDToken != "test_id_token_f4bdefbd77f" {
			t.Fatalf("unexpected raw ID token: %s", rawIDToken)
		}
		return &oidc.IDToken{
			Issuer:  oidcIDServer.URL,
			Subject: testOIDCUser,
			Expiry:  time.Now().Add(time.Hour),
			Nonce:   validState, // we re-use the state param as the nonce
		}
	}

	const mockUserID = 123

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	authedHandler := http.NewServeMux()
	authedHandler.Handle("/.api/", Middleware.API(h))
	authedHandler.Handle("/", Middleware.App(h))

	doRequest := func(method, urlStr, body string, cookies []*http.Cookie, authed bool) *http.Response {
		req := httptest.NewRequest(method, urlStr, bytes.NewBufferString(body))
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		if authed {
			req = req.WithContext(actor.WithActor(context.Background(), &actor.Actor{UID: mockUserID}))
		}
		respRecorder := httptest.NewRecorder()
		authedHandler.ServeHTTP(respRecorder, req)
		return respRecorder.Result()
	}

	state := func(t *testing.T, urlStr string) (state authnState) {
		u, _ := url.Parse(urlStr)
		if err := state.Decode(u.Query().Get("nonce")); err != nil {
			t.Fatal(err)
		}
		return state
	}
	t.Run("unauthenticated homepage visit -> oidc auth flow", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/", "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		if got, want := resp.Header.Get("Location"), "/oauth2/v1/authorize?"; !strings.Contains(got, want) {
			t.Errorf("got redirect URL %v, want contains %v", got, want)
		}
		if state, want := state(t, resp.Header.Get("Location")), "/"; state.Redirect != want {
			t.Errorf("got redirect destination %q, want %q", state.Redirect, want)
		}
	})
	t.Run("unauthenticated subpage visit -> oidc auth flow", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/page", "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		if got, want := resp.Header.Get("Location"), "/oauth2/v1/authorize?"; !strings.Contains(got, want) {
			t.Errorf("got redirect URL %v, want contains %v", got, want)
		}
		if state, want := state(t, resp.Header.Get("Location")), "/page"; state.Redirect != want {
			t.Errorf("got redirect destination %q, want %q", state.Redirect, want)
		}
	})
	t.Run("unauthenticated non-existent page visit -> oidc auth flow", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/nonexistent", "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		if got, want := resp.Header.Get("Location"), "/oauth2/v1/authorize?"; !strings.Contains(got, want) {
			t.Errorf("got redirect URL %v, want contains %v", got, want)
		}
		if state, want := state(t, resp.Header.Get("Location")), "/nonexistent"; state.Redirect != want {
			t.Errorf("got redirect destination %q, want %q", state.Redirect, want)
		}
	})
	t.Run("unauthenticated API request -> pass through", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.api/foo", "", nil, false)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
	})
	t.Run("login -> oidc auth flow", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.auth/openidconnect/login?p="+mockGetProviderValue.ConfigID().ID, "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		locHeader := resp.Header.Get("Location")
		if !strings.HasPrefix(locHeader, mockGetProviderValue.config.Issuer+"/") {
			t.Error("did not redirect to OIDC Provider")
		}
		idpLoginURL, err := url.Parse(locHeader)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := idpLoginURL.Query().Get("client_id"), mockGetProviderValue.config.ClientID; got != want {
			t.Errorf("got client id %q, want %q", got, want)
		}
		if got, want := idpLoginURL.Query().Get("redirect_uri"), "http://example.com/.auth/callback"; got != want {
			t.Errorf("got redirect_uri %v, want %v", got, want)
		}
		if got, want := idpLoginURL.Query().Get("response_type"), "code"; got != want {
			t.Errorf("got response_type %v, want %v", got, want)
		}
		if got, want := idpLoginURL.Query().Get("scope"), "openid profile email"; got != want {
			t.Errorf("got scope %v, want %v", got, want)
		}
	})
	t.Run("OIDC callback without CSRF token -> error", func(t *testing.T) {
		invalidState := (&authnState{CSRFToken: "bad", ProviderID: mockGetProviderValue.ConfigID().ID}).Encode()
		resp := doRequest("GET", "http://example.com/.auth/callback?code=THECODE&state="+url.PathEscape(invalidState), "", nil, false)
		if want := http.StatusBadRequest; resp.StatusCode != want {
			t.Errorf("got status code %v, want %v", resp.StatusCode, want)
		}
	})
	t.Run("OIDC callback with CSRF token -> set auth cookies", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.auth/callback?code=THECODE&state="+url.PathEscape(validState), "", []*http.Cookie{{Name: stateCookieName, Value: validState}}, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got status code %v, want %v", resp.StatusCode, want)
		}
		if got, want := resp.Header.Get("Location"), "/redirect"; got != want {
			t.Errorf("got redirect URL %v, want %v", got, want)
		}
	})
	*emailPtr = "bob@invalid.com" // doesn't match requiredEmailDomain
	t.Run("OIDC callback with bad email domain -> error", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.auth/callback?code=THECODE&state="+url.PathEscape(validState), "", []*http.Cookie{{Name: stateCookieName, Value: validState}}, false)
		if want := http.StatusUnauthorized; resp.StatusCode != want {
			t.Errorf("got status code %v, want %v", resp.StatusCode, want)
		}
	})
	t.Run("authenticated app request", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/", "", nil, true)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
	})
	t.Run("authenticated API request", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.api/foo", "", nil, true)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
	})
}

func TestMiddleware_NoOpenRedirect(t *testing.T) {
	cleanup := session.ResetMockSessionStore(t)
	defer cleanup()

	defer licensing.TestingSkipFeatureChecks()()

	tempdir, err := ioutil.TempDir("", "sourcegraph-oidc-test-no-open-redirect")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)

	mockGetProviderValue = &provider{
		config: schema.OpenIDConnectAuthProvider{
			ClientID:     testClientID,
			ClientSecret: "aaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	defer func() { mockGetProviderValue = nil }()
	providers.MockProviders = []providers.Provider{mockGetProviderValue}
	defer func() { providers.MockProviders = nil }()

	oidcIDServer, _ := newOIDCIDServer(t, "THECODE", &mockGetProviderValue.config)
	defer oidcIDServer.Close()
	defer func() { auth.MockGetAndSaveUser = nil }()
	mockGetProviderValue.config.Issuer = oidcIDServer.URL

	if err := mockGetProviderValue.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	state := (&authnState{CSRFToken: "THE_CSRF_TOKEN", Redirect: "http://evil.com", ProviderID: mockGetProviderValue.ConfigID().ID}).Encode()
	mockVerifyIDToken = func(rawIDToken string) *oidc.IDToken {
		if rawIDToken != "test_id_token_f4bdefbd77f" {
			t.Fatalf("unexpected raw ID token: %s", rawIDToken)
		}
		return &oidc.IDToken{
			Issuer:  oidcIDServer.URL,
			Subject: testOIDCUser,
			Expiry:  time.Now().Add(time.Hour),
			Nonce:   state, // we re-use the state param as the nonce
		}
	}

	database.Mocks.Users.GetByID = func(ctx context.Context, id int32) (*types.User, error) {
		return &types.User{ID: id, CreatedAt: time.Now()}, nil
	}
	defer func() { database.Mocks = database.MockStores{} }()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	authedHandler := Middleware.App(h)

	doRequest := func(method, urlStr, body string, cookies []*http.Cookie) *http.Response {
		req := httptest.NewRequest(method, urlStr, bytes.NewBufferString(body))
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		respRecorder := httptest.NewRecorder()
		authedHandler.ServeHTTP(respRecorder, req)
		return respRecorder.Result()
	}

	t.Run("OIDC callback with CSRF token -> set auth cookies", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.auth/callback?code=THECODE&state="+url.PathEscape(state), "", []*http.Cookie{{Name: stateCookieName, Value: state}})
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got status code %v, want %v", resp.StatusCode, want)
		}
		if got, want := resp.Header.Get("Location"), "/"; got != want {
			t.Errorf("got redirect URL %v, want %v", got, want)
		} // Redirect to "/", NOT "http://evil.com"
	})
}
