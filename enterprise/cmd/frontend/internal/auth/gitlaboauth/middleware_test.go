package gitlaboauth

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

// TestMiddleware exercises the Middleware with requests that simulate the OAuth 2 login flow on
// GitLab. This tests the logic between the client-issued HTTP requests and the responses from the
// various endpoints, but does NOT cover the logic that is contained within `golang.org/x/oauth2`
// and `github.com/dghubble/gologin` which ensures the correctness of the `/callback` handler.
func TestMiddleware(t *testing.T) {
	cleanup := session.ResetMockSessionStore(t)
	defer cleanup()

	const mockUserID = 123

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("got through"))
		if err != nil {
			t.Fatal(err)
		}
	})
	authedHandler := http.NewServeMux()
	authedHandler.Handle("/.api/", Middleware.API(h))
	authedHandler.Handle("/", Middleware.App(h))

	mockGitLabCom := newMockProvider(t, "gitlab-com-client", "gitlab-com-secret", "https://gitlab.com/")
	mockPrivateGitLab := newMockProvider(t, "gitlab-private-instsance-client", "github-private-instance-secret", "https://mycompany.com/")
	providers.MockProviders = []providers.Provider{mockGitLabCom.Provider}
	defer func() { providers.MockProviders = nil }()

	doRequest := func(method, urlStr, body string, cookies []*http.Cookie, authed bool) *http.Response {
		req := httptest.NewRequest(method, urlStr, bytes.NewBufferString(body))
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		req.Header.Set("User-Agent", "Mozilla")
		if authed {
			req = req.WithContext(actor.WithActor(context.Background(), &actor.Actor{UID: mockUserID}))
		}
		respRecorder := httptest.NewRecorder()
		authedHandler.ServeHTTP(respRecorder, req)
		return respRecorder.Result()
	}
	t.Run("unauthenticated homepage visit -> gitlab oauth flow", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/", "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		if got, want := resp.Header.Get("Location"), "/.auth/gitlab/login?"; !strings.Contains(got, want) {
			t.Errorf("got redirect URL %v, want contains %v", got, want)
		}
		redirectURL, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := redirectURL.Query().Get("redirect"), "/"; got != want {
			t.Errorf("got return-to URL %v, want %v", got, want)
		}
	})
	t.Run("unauthenticated subpage visit -> gitlab oauth flow", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/page", "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		if got, want := resp.Header.Get("Location"), "/.auth/gitlab/login?"; !strings.Contains(got, want) {
			t.Errorf("got redirect URL %v, want contains %v", got, want)
		}
		redirectURL, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := redirectURL.Query().Get("redirect"), "/page"; got != want {
			t.Errorf("got return-to URL %v, want %v", got, want)
		}
	})

	// Add 2 GitLab auth providers
	providers.MockProviders = []providers.Provider{mockPrivateGitLab.Provider, mockGitLabCom.Provider}

	t.Run("unauthenticated API request -> pass through", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.api/foo", "", nil, false)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(body), "got through"; got != want {
			t.Errorf("got response body %v, want %v", got, want)
		}
	})
	t.Run("login -> gitlab auth flow (gitlab.com)", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.auth/gitlab/login?pc="+mockGitLabCom.Provider.ConfigID().ID, "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		redirect := resp.Header.Get("Location")
		if got, want := redirect, "https://gitlab.com/oauth/authorize?"; !strings.HasPrefix(got, want) {
			t.Errorf("got redirect URL %v, want contains %v", got, want)
		}
		uredirect, err := url.Parse(redirect)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := uredirect.Query().Get("client_id"), mockGitLabCom.Provider.CachedInfo().ClientID; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := uredirect.Query().Get("scope"), "api read_user"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := uredirect.Query().Get("response_type"), "code"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		state, err := oauth.DecodeState(uredirect.Query().Get("state"))
		if err != nil {
			t.Fatalf("could not decode state: %v", err)
		}
		if got, want := state.ProviderID, mockGitLabCom.Provider.ConfigID().ID; got != want {
			t.Fatalf("got state provider ID %v, want %v", got, want)
		}
		if got, want := state.Redirect, ""; got != want {
			t.Fatalf("got state redirect %v, want %v", got, want)
		}
	})
	t.Run("login -> gitlab auth flow (GitLab private instance)", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.auth/gitlab/login?pc="+mockPrivateGitLab.Provider.ConfigID().ID, "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		redirect := resp.Header.Get("Location")
		if got, want := redirect, "https://mycompany.com/oauth/authorize?"; !strings.HasPrefix(got, want) {
			t.Errorf("got redirect URL %v, want contains %v", got, want)
		}
		uredirect, err := url.Parse(redirect)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := uredirect.Query().Get("client_id"), mockPrivateGitLab.Provider.CachedInfo().ClientID; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := uredirect.Query().Get("scope"), "api read_user"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := uredirect.Query().Get("response_type"), "code"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		state, err := oauth.DecodeState(uredirect.Query().Get("state"))
		if err != nil {
			t.Fatalf("could not decode state: %v", err)
		}
		if got, want := state.ProviderID, mockPrivateGitLab.Provider.ConfigID().ID; got != want {
			t.Fatalf("got state provider ID %v, want %v", got, want)
		}
		if got, want := state.Redirect, ""; got != want {
			t.Fatalf("got state redirect %v, want %v", got, want)
		}
	})
	t.Run("login -> gitlab auth flow with redirect param", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.auth/gitlab/login?pc="+mockGitLabCom.Provider.ConfigID().ID+"&redirect=%2Fpage", "", nil, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		redirect := resp.Header.Get("Location")
		if got, want := redirect, "https://gitlab.com/oauth/authorize?"; !strings.HasPrefix(got, want) {
			t.Errorf("got redirect URL %v, want contains %v", got, want)
		}
		uredirect, err := url.Parse(redirect)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := uredirect.Query().Get("client_id"), mockGitLabCom.Provider.CachedInfo().ClientID; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := uredirect.Query().Get("scope"), "api read_user"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := uredirect.Query().Get("response_type"), "code"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		state, err := oauth.DecodeState(uredirect.Query().Get("state"))
		if err != nil {
			t.Fatalf("could not decode state: %v", err)
		}
		if got, want := state.ProviderID, mockGitLabCom.Provider.ConfigID().ID; got != want {
			t.Fatalf("got state provider ID %v, want %v", got, want)
		}
		if got, want := state.Redirect, "/page"; got != want {
			t.Fatalf("got state redirect %v, want %v", got, want)
		}
	})
	t.Run("GitLab OAuth callback with valid state param", func(t *testing.T) {
		encodedState, err := oauth.LoginState{
			Redirect:   "/return-to-url",
			ProviderID: mockGitLabCom.Provider.ConfigID().ID,
			CSRF:       "csrf-code",
		}.Encode()
		if err != nil {
			t.Fatal(err)
		}
		callbackCookies := []*http.Cookie{oauth.NewCookie(getStateConfig(), encodedState)}
		resp := doRequest("GET", "http://example.com/.auth/gitlab/callback?code=the-oauth-code&state="+encodedState, "", callbackCookies, false)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		if got, want := mockGitLabCom.lastCallbackRequestURL, "http://example.com/callback?code=the-oauth-code&state="+encodedState; got == nil || got.String() != want {
			t.Errorf("got last gitlab.com callback request url %v, want %v", got, want)
		}
		mockGitLabCom.lastCallbackRequestURL = nil
	})
	t.Run("GitLab OAuth callback with state with unknown provider", func(t *testing.T) {
		encodedState, err := oauth.LoginState{
			Redirect:   "/return-to-url",
			ProviderID: "unknown",
			CSRF:       "csrf-code",
		}.Encode()
		if err != nil {
			t.Fatal(err)
		}
		callbackCookies := []*http.Cookie{oauth.NewCookie(getStateConfig(), encodedState)}
		resp := doRequest("GET", "http://example.com/.auth/gitlab/callback?code=the-oauth-code&state="+encodedState, "", callbackCookies, false)
		if want := http.StatusBadRequest; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		if mockGitLabCom.lastCallbackRequestURL != nil {
			t.Errorf("got last github.com callback request url was non-nil: %v", mockGitLabCom.lastCallbackRequestURL)
		}
		mockGitLabCom.lastCallbackRequestURL = nil
	})
	t.Run("authenticated app request", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/", "", nil, true)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(body), "got through"; got != want {
			t.Errorf("got response body %v, want %v", got, want)
		}
	})
	t.Run("authenticated API request", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.api/foo", "", nil, true)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(body), "got through"; got != want {
			t.Errorf("got response body %v, want %v", got, want)
		}
	})
}

type MockProvider struct {
	*oauth.Provider
	lastCallbackRequestURL *url.URL
}

func newMockProvider(t *testing.T, clientID, clientSecret, baseURL string) *MockProvider {
	var (
		mp       MockProvider
		problems []string
	)
	cfg := schema.AuthProviders{Gitlab: &schema.GitLabAuthProvider{
		Url:          baseURL,
		ClientSecret: clientSecret,
		ClientID:     clientID,
		Type:         extsvc.TypeGitLab,
	}}
	mp.Provider, problems = parseProvider("https://sourcegraph.mine.com/.auth/gitlab/callback", cfg.Gitlab, cfg)
	if len(problems) > 0 {
		t.Fatalf("Expected 0 problems, but got %d: %+v", len(problems), problems)
	}
	if mp.Provider == nil {
		t.Fatalf("Expected provider")
	}
	mp.Provider.Callback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, "GET"; got != want {
			t.Errorf("In OAuth callback handler got %q request, wanted %q", got, want)
		}
		w.WriteHeader(http.StatusFound)
		mp.lastCallbackRequestURL = r.URL
	})
	return &mp
}
