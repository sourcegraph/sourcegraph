//nolint:bodyclose // Body is closed in Client.Do, but the response is still returned to provide access to the headers
package azuredevops

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/goware/urlx"
	"github.com/sourcegraph/log"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/oauthutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	AzureDevOpsAPIURL       = "https://dev.azure.com/"
	ClientAssertionType     = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	apiVersion              = "7.0"
	continuationTokenHeader = "x-ms-continuationtoken"
)

// Client used to access an AzureDevOps code host via the REST API.
type Client interface {
	WithAuthenticator(a auth.Authenticator) (Client, error)
	Authenticator() auth.Authenticator
	GetURL() *url.URL
	IsAzureDevOpsServices() bool
	AbandonPullRequest(ctx context.Context, args PullRequestCommonArgs) (PullRequest, error)
	CreatePullRequest(ctx context.Context, args OrgProjectRepoArgs, input CreatePullRequestInput) (PullRequest, error)
	GetPullRequest(ctx context.Context, args PullRequestCommonArgs) (PullRequest, error)
	GetPullRequestStatuses(ctx context.Context, args PullRequestCommonArgs) ([]PullRequestBuildStatus, error)
	UpdatePullRequest(ctx context.Context, args PullRequestCommonArgs, input PullRequestUpdateInput) (PullRequest, error)
	CreatePullRequestCommentThread(ctx context.Context, args PullRequestCommonArgs, input PullRequestCommentInput) (PullRequestCommentResponse, error)
	CompletePullRequest(ctx context.Context, args PullRequestCommonArgs, input PullRequestCompleteInput) (PullRequest, error)
	GetRepo(ctx context.Context, args OrgProjectRepoArgs) (Repository, error)
	ListRepositoriesByProjectOrOrg(ctx context.Context, args ListRepositoriesByProjectOrOrgArgs) ([]Repository, error)
	ForkRepository(ctx context.Context, org string, input ForkRepositoryInput) (Repository, error)
	GetRepositoryBranch(ctx context.Context, args OrgProjectRepoArgs, branchName string) (Ref, error)
	GetProject(ctx context.Context, org, project string) (Project, error)
	GetAuthorizedProfile(ctx context.Context) (Profile, error)
	ListAuthorizedUserOrganizations(ctx context.Context, profile Profile) ([]Org, error)
	SetWaitForRateLimit(wait bool)
}

type client struct {
	// HTTP Client used to communicate with the API.
	httpClient httpcli.Doer

	// URL is the base URL of AzureDevOps.
	URL *url.URL

	urn string

	internalRateLimiter *ratelimit.InstrumentedLimiter
	externalRateLimiter *ratelimit.Monitor
	auth                auth.Authenticator
	waitForRateLimit    bool
	maxRateLimitRetries int
}

// NewClient returns an authenticated AzureDevOps API client with
// the provided configuration. If cf is nil, an external doer will be used.
func NewClient(urn string, url string, auth auth.Authenticator, cf *httpcli.Factory) (Client, error) {
	u, err := urlx.Parse(url)
	if err != nil {
		return nil, err
	}

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	httpClient, err := cf.Doer(httpcli.CachedTransportOpt)
	if err != nil {
		return nil, err
	}

	return newClient(urn, u, auth, httpClient), nil
}

func newClient(urn string, url *url.URL, auth auth.Authenticator, cli httpcli.Doer) Client {
	return &client{
		httpClient:          cli,
		URL:                 url,
		internalRateLimiter: ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("AzureDevOpsClient"), urn)),
		externalRateLimiter: ratelimit.DefaultMonitorRegistry.GetOrSet(url.String(), auth.Hash(), "rest", &ratelimit.Monitor{HeaderPrefix: "X-"}),
		auth:                auth,
		urn:                 urn,
		waitForRateLimit:    true,
		maxRateLimitRetries: 2,
	}
}

// do performs the specified request, returning any errors and a continuationToken used for pagination (if the API supports it).
//
//nolint:unparam // http.Response is never used, but it makes sense API wise.
func (c *client) do(ctx context.Context, req *http.Request, urlOverride string, result any) (continuationToken string, err error) {
	u := c.URL
	if urlOverride != "" {
		u, err = url.Parse(urlOverride)
		if err != nil {
			return "", err
		}
	}

	queryParams := req.URL.Query()
	queryParams.Set("api-version", apiVersion)
	req.URL.RawQuery = queryParams.Encode()
	req.URL = u.ResolveReference(req.URL)

	var reqBody []byte
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return "", err
		}
	}
	req.Body = io.NopCloser(bytes.NewReader(reqBody))

	// Add authentication headers for authenticated requests.
	if err := c.auth.Authenticate(req); err != nil {
		return "", err
	}

	if err := c.internalRateLimiter.Wait(ctx); err != nil {
		return "", err
	}

	if c.waitForRateLimit {
		_ = c.externalRateLimiter.WaitForRateLimit(ctx, 1)
	}

	logger := log.Scoped("azuredevops.Client")
	resp, err := oauthutil.DoRequest(ctx, logger, c.httpClient, req, c.auth)
	if err != nil {
		return "", err
	}

	c.externalRateLimiter.Update(resp.Header)

	numRetries := 0
	for c.waitForRateLimit && resp.StatusCode == http.StatusTooManyRequests &&
		numRetries < c.maxRateLimitRetries {
		// We always retry since we got a StatusTooManyRequests. This is safe
		// since we bound retries by maxRateLimitRetries.
		_ = c.externalRateLimiter.WaitForRateLimit(ctx, 1)

		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		resp, err = oauthutil.DoRequest(ctx, logger, c.httpClient, req, c.auth)
		numRetries++
	}

	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", &HTTPError{
			URL:        req.URL,
			StatusCode: resp.StatusCode,
			Body:       bs,
		}
	}

	return resp.Header.Get(continuationTokenHeader), json.Unmarshal(bs, result)
}

// WithAuthenticator returns a new Client that uses the same configuration,
// HTTPClient, and RateLimiter as the current Client, except authenticated with
// the given authenticator instance.
//
// Note that using an unsupported Authenticator implementation may result in
// unexpected behaviour, or (more likely) errors. At present, only BasicAuth and
// BasicAuthWithSSH are supported.
func (c *client) WithAuthenticator(a auth.Authenticator) (Client, error) {
	switch a.(type) {
	case *auth.BasicAuth, *auth.BasicAuthWithSSH:
		break
	default:
		return nil, errors.Errorf("authenticator type unsupported for Azure DevOps clients: %s", a)
	}

	return newClient(c.urn, c.URL, a, c.httpClient), nil
}

func (c *client) SetWaitForRateLimit(wait bool) {
	c.waitForRateLimit = wait
}

func (c *client) Authenticator() auth.Authenticator {
	return c.auth
}

func (c *client) GetURL() *url.URL {
	return c.URL
}

// IsAzureDevOpsServices returns true if the client is configured to Azure DevOps
// Services (https://dev.azure.com)
func (c *client) IsAzureDevOpsServices() bool {
	return c.URL.String() == AzureDevOpsAPIURL
}

func GetOAuthContext(refreshToken string) (*oauthutil.OAuthContext, error) {
	for _, authProvider := range conf.SiteConfig().AuthProviders {
		if authProvider.AzureDevOps != nil {
			authURL, err := url.JoinPath(VisualStudioAppURL, "oauth2/authorize")
			if err != nil {
				return nil, err
			}
			tokenURL, err := url.JoinPath(VisualStudioAppURL, "oauth2/token")
			if err != nil {
				return nil, err
			}

			redirectURL, err := GetRedirectURL(nil)
			if err != nil {
				return nil, err
			}

			p := authProvider.AzureDevOps
			return &oauthutil.OAuthContext{
				ClientID:     p.ClientID,
				ClientSecret: p.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  authURL,
					TokenURL: tokenURL,
				},
				// The API expects some custom values in the POST body to refresh the token. See:
				// https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/authentication/oauth?view=azure-devops#4-use-the-access-token
				//
				// DEBUGGING NOTE: The token refresher (internal/oauthutil/token.go:newTokenRequest)
				// adds some default key-value pairs to the body, some of which are eventually
				// overridden by the values in this map. But some extra arg remain in the body that
				// is sent in the request. This works for now, but if refreshing a token ever stops
				// working for Azure Dev Ops this is a good place to start looking by writing a
				// custom implementation that only sends the key-value pairs that the API expects.
				CustomQueryParams: map[string]string{
					"client_assertion_type": ClientAssertionType,
					"client_assertion":      url.QueryEscape(p.ClientSecret),
					"grant_type":            "refresh_token",
					"assertion":             url.QueryEscape(refreshToken),
					"redirect_uri":          redirectURL.String(),
				},
			}, nil
		}
	}

	return nil, errors.New("No authprovider configured for AzureDevOps, check site configuration.")
}

// GetRedirectURL returns the redirect URL for azuredevops OAuth provider. It takes an optional
// SiteConfigQuerier to query the ExternalURL from the site config. If nil, it directly reads the
// site config using the conf.SiteConfig method.
func GetRedirectURL(cfg conftypes.SiteConfigQuerier) (*url.URL, error) {
	var externalURL string
	if cfg != nil {
		externalURL = cfg.SiteConfig().ExternalURL
	} else {
		externalURL = conf.SiteConfig().ExternalURL
	}

	parsedURL, err := url.Parse(externalURL)
	if err != nil {
		return nil, errors.New("Could not parse `externalURL`, which is needed to determine the OAuth callback URL.")
	}

	parsedURL.Path = "/.auth/azuredevops/callback"
	return parsedURL, nil
}

func (e *HTTPError) Unauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

func (e *HTTPError) NotFound() bool {
	return e.StatusCode == http.StatusNotFound
}
