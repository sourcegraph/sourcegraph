package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/oauthutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	// The metric generated here will be named as "src_gitlab_requests_total".
	requestCounter = metrics.NewRequestMeter("gitlab", "Total number of requests sent to the GitLab API.")
)

// TokenType is the type of an access token.
type TokenType string

const (
	TokenTypePAT   TokenType = "pat"   // "pat" represents personal access token.
	TokenTypeOAuth TokenType = "oauth" // "oauth" represents OAuth token.
)

// ClientProvider creates GitLab API clients. Each client has separate authentication creds and a
// separate cache, but they share an underlying HTTP client and rate limiter. Callers who want a simple
// unauthenticated API client should use `NewClientProvider(baseURL, transport).GetClient()`.
type ClientProvider struct {
	// The URN of the external service that the client is derived from.
	urn string

	// baseURL is the base URL of GitLab; e.g., https://gitlab.com or https://gitlab.example.com
	baseURL *url.URL

	// httpClient is the underlying the HTTP client to use.
	httpClient httpcli.Doer

	gitlabClients   map[string]*Client
	gitlabClientsMu sync.Mutex
}

type CommonOp struct {
	// NoCache, if true, will bypass any caching done in this package
	NoCache bool
}

func NewClientProvider(urn string, baseURL *url.URL, cf *httpcli.Factory) (*ClientProvider, error) {
	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	httpClient, err := cf.Doer(httpcli.CachedTransportOpt)
	if err != nil {
		return nil, err
	}

	httpClient = requestCounter.Doer(httpClient, func(u *url.URL) string {
		// The 3rd component of the Path (/api/v4/XYZ) mostly maps to the type of API
		// request we are making.
		var category string
		if parts := strings.SplitN(u.Path, "/", 3); len(parts) >= 4 {
			category = parts[3]
		}
		return category
	})

	return &ClientProvider{
		urn:           urn,
		baseURL:       baseURL.ResolveReference(&url.URL{Path: path.Join(baseURL.Path, "api/v4") + "/"}),
		httpClient:    httpClient,
		gitlabClients: make(map[string]*Client),
	}, nil
}

// GetAuthenticatorClient returns a client authenticated by the given
// authenticator.
func (p *ClientProvider) GetAuthenticatorClient(a auth.Authenticator) *Client {
	return p.getClient(a)
}

// GetPATClient returns a client authenticated by the personal access token.
func (p *ClientProvider) GetPATClient(personalAccessToken, sudo string) *Client {
	if personalAccessToken == "" {
		return p.getClient(nil)
	}
	return p.getClient(&SudoableToken{Token: personalAccessToken, Sudo: sudo})
}

// GetOAuthClient returns a client authenticated by the OAuth token.
func (p *ClientProvider) GetOAuthClient(oauthToken string) *Client {
	if oauthToken == "" {
		return p.getClient(nil)
	}
	return p.getClient(&auth.OAuthBearerToken{Token: oauthToken})
}

// GetClient returns an unauthenticated client.
func (p *ClientProvider) GetClient() *Client {
	return p.getClient(nil)
}

func (p *ClientProvider) getClient(a auth.Authenticator) *Client {
	p.gitlabClientsMu.Lock()
	defer p.gitlabClientsMu.Unlock()

	key := "<nil>"
	if a != nil {
		key = a.Hash()
	}
	if c, ok := p.gitlabClients[key]; ok {
		return c
	}

	c := p.NewClient(a)
	p.gitlabClients[key] = c
	return c
}

// Client is a GitLab API client. Clients are associated with a particular user
// identity, which is defined by the Auth implementation. In addition to the
// generic types provided by the auth package, Client also supports
// SudoableToken: if this is used and its Sudo field is non-empty, then the user
// identity will be the user ID specified by Sudo (rather than the user that
// owns the token).
//
// The Client's cache is keyed by Auth.Hash(). It is NOT keyed by the actual
// user ID that is defined by the authentication method. So if an OAuth token
// and personal access token belong to the same user and there are two
// corresponding Client instances, those Client instances will NOT share the
// same cache. However, two Client instances sharing the exact same values for
// those fields WILL share a cache.
type Client struct {
	// The URN of the external service that the client is derived from.
	urn string
	log log.Logger

	baseURL             *url.URL
	httpClient          httpcli.Doer
	projCache           *rcache.Cache
	Auth                auth.Authenticator
	externalRateLimiter *ratelimit.Monitor
	internalRateLimiter *ratelimit.InstrumentedLimiter
	waitForRateLimit    bool
	maxRateLimitRetries int
}

// NewClient creates a new GitLab API client with an optional personal access token to authenticate requests.
//
// The URL must point to the base URL of the GitLab instance. This is https://gitlab.com for GitLab.com and
// http[s]://[gitlab-hostname] for self-hosted GitLab instances.
//
// See the docstring of Client for the meaning of the parameters.
func (p *ClientProvider) NewClient(a auth.Authenticator) *Client {
	// Cache for GitLab project metadata.
	var cacheTTL time.Duration
	if isGitLabDotComURL(p.baseURL) && a == nil {
		cacheTTL = 10 * time.Minute // cache for longer when unauthenticated
	} else {
		cacheTTL = 30 * time.Second
	}
	key := "gl_proj:"
	var tokenHash string
	if a != nil {
		tokenHash = a.Hash()
		key += tokenHash
	}
	projCache := rcache.NewWithTTL(key, int(cacheTTL/time.Second))

	rl := ratelimit.NewInstrumentedLimiter(p.urn, ratelimit.NewGlobalRateLimiter(log.Scoped("GitLabClient"), p.urn))
	rlm := ratelimit.DefaultMonitorRegistry.GetOrSet(p.baseURL.String(), tokenHash, "rest", &ratelimit.Monitor{})

	return &Client{
		urn:                 p.urn,
		log:                 log.Scoped("gitlabAPIClient"),
		baseURL:             p.baseURL,
		httpClient:          p.httpClient,
		projCache:           projCache,
		Auth:                a,
		internalRateLimiter: rl,
		externalRateLimiter: rlm,
		waitForRateLimit:    true,
		maxRateLimitRetries: 2,
	}
}

func isGitLabDotComURL(baseURL *url.URL) bool {
	hostname := strings.ToLower(baseURL.Hostname())
	return hostname == "gitlab.com" || hostname == "www.gitlab.com"
}

func (c *Client) Urn() string {
	return c.urn
}

// do is the default method for making API requests and will prepare the correct
// base path.
func (c *Client) do(ctx context.Context, req *http.Request, result any) (responseHeader http.Header, responseCode int, err error) {
	if c.internalRateLimiter != nil {
		err = c.internalRateLimiter.Wait(ctx)
		if err != nil {
			return nil, 0, errors.Wrap(err, "rate limit")
		}
	}

	if c.waitForRateLimit {
		// We don't care whether this happens or not as it is a preventative measure.
		_ = c.externalRateLimiter.WaitForRateLimit(ctx, 1)
	}

	var reqBody []byte
	if req.Body != nil {
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, 0, err
		}
	}
	req.Body = io.NopCloser(bytes.NewReader(reqBody))
	req.URL = c.baseURL.ResolveReference(req.URL)
	respHeader, respCode, err := c.doWithBaseURL(ctx, req, result)

	// GitLab responds with a 429 Too Many Requests if rate limits are exceeded
	numRetries := 0
	for c.waitForRateLimit && numRetries < c.maxRateLimitRetries && respCode == http.StatusTooManyRequests {
		// We always retry since we got a StatusTooManyRequests. This is safe
		// since we bound retries by maxRateLimitRetries.
		_ = c.externalRateLimiter.WaitForRateLimit(ctx, 1)

		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		respHeader, respCode, err = c.doWithBaseURL(ctx, req, result)
		numRetries += 1
	}

	return respHeader, respCode, err
}

// doWithBaseURL doesn't amend the request URL. When an OAuth Bearer token is
// used for authentication, it will try to refresh the token and retry the same
// request when the token has expired.
func (c *Client) doWithBaseURL(ctx context.Context, req *http.Request, result any) (responseHeader http.Header, responseCode int, err error) {
	var resp *http.Response

	tr, ctx := trace.New(ctx, "GitLab",
		attribute.Stringer("url", req.URL))
	defer func() {
		if resp != nil {
			tr.SetAttributes(attribute.String("status", resp.Status))
		}
		tr.EndWithErr(&err)
	}()
	req = req.WithContext(ctx)

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	// Prevent the CachedTransportOpt from caching client side, but still use ETags
	// to cache server-side
	req.Header.Set("Cache-Control", "max-age=0")

	resp, err = oauthutil.DoRequest(ctx, log.Scoped("gitlab client"), c.httpClient, req, c.Auth)
	if resp != nil {
		c.externalRateLimiter.Update(resp.Header)
	}
	if err != nil {
		c.log.Debug("GitLab API error", log.String("method", req.Method), log.String("url", req.URL.String()), log.Error(err))
		return nil, 0, errors.Wrap(err, "request failed")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, errors.Wrap(err, "read response body")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		err := NewHTTPError(resp.StatusCode, body)
		return nil, resp.StatusCode, errors.Wrap(err, fmt.Sprintf("unexpected response from GitLab API (%s)", req.URL))
	}

	return resp.Header, resp.StatusCode, json.Unmarshal(body, result)
}

// ExternalRateLimiter exposes the rate limit monitor.
func (c *Client) ExternalRateLimiter() *ratelimit.Monitor {
	return c.externalRateLimiter
}

func (c *Client) WithAuthenticator(a auth.Authenticator) *Client {
	tokenHash := a.Hash()

	cc := *c
	cc.internalRateLimiter = ratelimit.NewInstrumentedLimiter(c.urn, ratelimit.NewGlobalRateLimiter(log.Scoped("GitLabClient"), c.urn))
	cc.externalRateLimiter = ratelimit.DefaultMonitorRegistry.GetOrSet(cc.baseURL.String(), tokenHash, "rest", &ratelimit.Monitor{})
	cc.Auth = a

	return &cc
}

func (c *Client) ValidateToken(ctx context.Context) error {
	req, err := http.NewRequest(http.MethodGet, "user", nil)
	if err != nil {
		return err
	}
	v := struct{}{}
	_, _, err = c.do(ctx, req, &v)
	return err
}

func (c *Client) GetAuthenticatedUserOAuthScopes(ctx context.Context) ([]string, error) {
	// The oauth token info path is non standard so we need to build it manually
	// without the default `/api/v4` prefix
	u, _ := url.Parse(c.baseURL.String())
	u.Path = "oauth/token/info"

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	v := struct {
		Scopes []string `json:"scopes,omitempty"`
	}{}

	_, _, err = c.doWithBaseURL(ctx, req, &v)
	if err != nil {
		return nil, errors.Wrap(err, "getting oauth scopes")
	}
	return v.Scopes, nil
}

type HTTPError struct {
	code int
	body []byte
}

func NewHTTPError(code int, body []byte) HTTPError {
	return HTTPError{
		code: code,
		body: body,
	}
}

func (err HTTPError) Code() int {
	return err.code
}

func (err HTTPError) Message() string {
	var errBody struct {
		Message string `json:"message"`
	}
	// Swallow error, decoding the body as
	_ = json.Unmarshal(err.body, &errBody)
	return errBody.Message
}

func (err HTTPError) Error() string {
	return fmt.Sprintf("HTTP error status %d", err.code)
}

func (err HTTPError) Unauthorized() bool {
	return err.code == http.StatusUnauthorized
}

func (err HTTPError) Forbidden() bool {
	return err.code == http.StatusForbidden
}

func (err HTTPError) IsTemporary() bool {
	return err.code == http.StatusTooManyRequests
}

// HTTPErrorCode returns err's HTTP status code, if it is an HTTP error from
// this package. Otherwise it returns 0.
func HTTPErrorCode(err error) int {
	var e HTTPError
	if errors.As(err, &e) {
		return e.Code()
	}

	return 0
}

// IsNotFound reports whether err is a GitLab API error of type NOT_FOUND, the equivalent cached
// response error, or HTTP 404.
func IsNotFound(err error) bool {
	return errors.HasType(err, &ProjectNotFoundError{}) ||
		errors.Is(err, ErrMergeRequestNotFound) ||
		HTTPErrorCode(err) == http.StatusNotFound
}

// ErrMergeRequestNotFound is when the requested GitLab merge request is not found.
var ErrMergeRequestNotFound = errors.New("GitLab merge request not found")

// ErrProjectNotFound is when the requested GitLab project is not found.
var ErrProjectNotFound = &ProjectNotFoundError{}

// ProjectNotFoundError is when the requested GitHub repository is not found.
type ProjectNotFoundError struct {
	Name string
}

func (e ProjectNotFoundError) Error() string {
	if e.Name == "" {
		return "GitLab project not found"
	}
	return fmt.Sprintf("GitLab project %q not found", e.Name)
}

func (e ProjectNotFoundError) NotFound() bool { return true }

var MockGetOAuthContext func() *oauthutil.OAuthContext

// GetOAuthContext matches the corresponding auth provider using the given
// baseURL and returns the oauthutil.OAuthContext of it.
func GetOAuthContext(baseURL string) *oauthutil.OAuthContext {
	if MockGetOAuthContext != nil {
		return MockGetOAuthContext()
	}

	for _, authProvider := range conf.SiteConfig().AuthProviders {
		if authProvider.Gitlab != nil {
			p := authProvider.Gitlab
			glURL := strings.TrimSuffix(p.Url, "/")
			if !strings.HasPrefix(baseURL, glURL) {
				continue
			}

			return &oauthutil.OAuthContext{
				ClientID:     p.ClientID,
				ClientSecret: p.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  glURL + "/oauth/authorize",
					TokenURL: glURL + "/oauth/token",
				},
				Scopes: RequestedOAuthScopes(p.ApiScope),
			}
		}
	}
	return nil
}

// ProjectArchivedError is returned when a request cannot be performed due to the
// GitLab project being archived.
type ProjectArchivedError struct{ Name string }

func (ProjectArchivedError) Archived() bool { return true }

func (e ProjectArchivedError) Error() string {
	if e.Name == "" {
		return "GitLab project is archived"
	}
	return fmt.Sprintf("GitLab project %q is archived", e.Name)
}

func (ProjectArchivedError) NonRetryable() bool { return true }
