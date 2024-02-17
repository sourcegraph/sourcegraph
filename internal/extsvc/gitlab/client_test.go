package gitlab

import (
	"bytes"
	"context"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/regexp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/oauthutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestGetAuthenticatedUserOAuthScopes(t *testing.T) {
	// To update this test's fixtures, use the GitLab token stored in
	// 1Password under gitlab@sourcegraph.com.
	client := createTestClient(t)
	ctx := context.Background()
	have, err := client.GetAuthenticatedUserOAuthScopes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"read_user", "read_api", "api"}
	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatal(diff)
	}
}

func createTestProvider(t *testing.T) *ClientProvider {
	t.Helper()
	fac, cleanup := httptestutil.NewRecorderFactory(t, update(t.Name()), t.Name())
	t.Cleanup(cleanup)
	baseURL, _ := url.Parse("https://gitlab.com/")
	provider, err := NewClientProvider("Test", baseURL, fac)
	require.NoError(t, err)
	return provider
}

func createTestClient(t *testing.T) *Client {
	t.Helper()
	token := os.Getenv("GITLAB_TOKEN")
	c := createTestProvider(t).GetOAuthClient(token)
	c.internalRateLimiter = ratelimit.NewInstrumentedLimiter("gitlab", rate.NewLimiter(100, 10))
	return c
}

var updateRegex = flag.String("update", "", "Update testdata of tests matching the given regex")

func update(name string) bool {
	if updateRegex == nil || *updateRegex == "" {
		return false
	}
	return regexp.MustCompile(*updateRegex).MatchString(name)
}

type mockTransport struct {
	do func(*http.Request) (*http.Response, error)
}

func (c *mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return c.do(r)
}

func TestClient_doWithBaseURL(t *testing.T) {
	baseURL, err := url.Parse("https://gitlab.com/")
	require.NoError(t, err)

	cf := httpcli.WrapTransport(&mockTransport{
		do: func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") == "Bearer bad token" {
				return &http.Response{
					Status:     http.StatusText(http.StatusUnauthorized),
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"invalid_token","error_description":"Token is expired. You can either do re-authorization or token refresh."}`))),
				}, nil
			}

			body := `{"access_token": "refreshed-token", "token_type": "Bearer", "expires_in":3600, "refresh_token":"refresh-now", "scope":"create"}`
			return &http.Response{
				Status:     http.StatusText(http.StatusOK),
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(body))),
			}, nil
		},
	}, http.DefaultTransport)

	ctx := context.Background()

	provider, err := NewClientProvider("Test", baseURL, httpcli.NewFactory(nil, func(c *http.Client) error {
		c.Transport = cf
		return nil
	}))
	require.NoError(t, err)

	client := provider.getClient(&auth.OAuthBearerToken{Token: "bad token", RefreshToken: "refresh token", RefreshFunc: func(ctx context.Context, cli httpcli.Doer, obt *auth.OAuthBearerToken) (string, string, time.Time, error) {
		obt.Token = "refreshed-token"
		obt.RefreshToken = "refresh-now"

		return "refreshed-token", "refresh-now", time.Now().Add(1 * time.Hour), nil
	}})

	req, err := http.NewRequest(http.MethodGet, "url", nil)
	require.NoError(t, err)

	var result map[string]any
	_, _, err = client.doWithBaseURL(ctx, req, &result)
	require.NoError(t, err)
}

func TestRateLimitRetry(t *testing.T) {
	rcache.SetupForTest(t)

	ctx := context.Background()

	tests := map[string]struct {
		useRateLimit     bool
		useRetryAfter    bool
		succeeded        bool
		waitForRateLimit bool
		wantNumRequests  int
	}{
		"retry-after hit": {
			useRetryAfter:    true,
			succeeded:        true,
			waitForRateLimit: true,
			wantNumRequests:  2,
		},
		"rate limit hit": {
			useRateLimit:     true,
			succeeded:        true,
			waitForRateLimit: true,
			wantNumRequests:  2,
		},
		"no rate limit hit": {
			succeeded:        true,
			waitForRateLimit: true,
			wantNumRequests:  1,
		},
		"error if rate limit hit but no waitForRateLimit": {
			useRateLimit:    true,
			wantNumRequests: 1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			numRequests := 0
			succeeded := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				numRequests += 1
				if tt.useRetryAfter {
					w.Header().Add("Retry-After", "1")
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte("Try again later"))

					tt.useRetryAfter = false
					return
				}

				if tt.useRateLimit {
					w.Header().Add("RateLimit-Name", "test")
					w.Header().Add("RateLimit-Limit", "60")
					w.Header().Add("RateLimit-Observed", "67")
					w.Header().Add("RateLimit-Remaining", "0")
					resetTime := time.Now().Add(time.Second)
					w.Header().Add("RateLimit-Reset", strconv.Itoa(int(resetTime.Unix())))
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte("Try again later"))

					tt.useRateLimit = false
					return
				}

				succeeded = true
				w.Write([]byte(`{"some": "response"}`))
			}))
			t.Cleanup(srv.Close)

			srvURL, err := url.Parse(srv.URL)
			require.NoError(t, err)

			provider, err := NewClientProvider("Test", srvURL, httpcli.NewFactory(nil))
			require.NoError(t, err)
			client := provider.getClient(nil)
			client.internalRateLimiter = ratelimit.NewInstrumentedLimiter("gitlab", rate.NewLimiter(100, 10))
			client.waitForRateLimit = tt.waitForRateLimit

			req, err := http.NewRequest(http.MethodGet, "url", nil)
			require.NoError(t, err)
			var result map[string]any

			_, _, err = client.do(ctx, req, &result)
			if tt.succeeded {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			assert.Equal(t, tt.succeeded, succeeded)
			assert.Equal(t, tt.wantNumRequests, numRequests)
		})
	}
}

func TestGetOAuthContext(t *testing.T) {
	conf.Mock(
		&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{
					{
						Github: &schema.GitHubAuthProvider{
							Url: "https://gitlab.com/", // Matching URL but wrong provider
						},
					}, {
						Gitlab: &schema.GitLabAuthProvider{
							Url: "https://gitlab.myexample.com/", // URL doesn't match
						},
					}, {
						Gitlab: &schema.GitLabAuthProvider{
							ClientID:     "my-client-id",
							ClientSecret: "my-client-secret",
							Url:          "https://gitlab.com/", // Good
						},
					},
				},
			},
		},
	)
	defer conf.Mock(nil)

	tests := []struct {
		name    string
		baseURL string
		want    *oauthutil.OAuthContext
	}{
		{
			name:    "match with API URL",
			baseURL: "https://gitlab.com/api/v4/",
			want: &oauthutil.OAuthContext{
				ClientID:     "my-client-id",
				ClientSecret: "my-client-secret",
				Endpoint: oauthutil.Endpoint{
					AuthURL:   "https://gitlab.com/oauth/authorize",
					TokenURL:  "https://gitlab.com/oauth/token",
					AuthStyle: 0,
				},
				Scopes: []string{"read_user", "api"},
			},
		},
		{
			name:    "match with root URL",
			baseURL: "https://gitlab.com/",
			want: &oauthutil.OAuthContext{
				ClientID:     "my-client-id",
				ClientSecret: "my-client-secret",
				Endpoint: oauthutil.Endpoint{
					AuthURL:   "https://gitlab.com/oauth/authorize",
					TokenURL:  "https://gitlab.com/oauth/token",
					AuthStyle: 0,
				},
				Scopes: []string{"read_user", "api"},
			},
		},
		{
			name:    "no match",
			baseURL: "https://gitlab.example.com/api/v4/",
			want:    nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetOAuthContext(test.baseURL)
			assert.Equal(t, test.want, got)
		})
	}
}
