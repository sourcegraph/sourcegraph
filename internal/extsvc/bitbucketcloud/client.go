package bitbucketcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/oauthutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// The metric generated here will be named as "src_bitbucket_cloud_requests_total".
var requestCounter = metrics.NewRequestMeter("bitbucket_cloud", "Total number of requests sent to the Bitbucket Cloud API.")

type Client interface {
	Authenticator() auth.Authenticator
	WithAuthenticator(a auth.Authenticator) Client

	Ping(ctx context.Context) error

	CreatePullRequest(ctx context.Context, repo *Repo, input PullRequestInput) (*PullRequest, error)
	DeclinePullRequest(ctx context.Context, repo *Repo, id int64) (*PullRequest, error)
	GetPullRequest(ctx context.Context, repo *Repo, id int64) (*PullRequest, error)
	GetPullRequestStatuses(repo *Repo, id int64) (*PaginatedResultSet, error)
	UpdatePullRequest(ctx context.Context, repo *Repo, id int64, input PullRequestInput) (*PullRequest, error)
	CreatePullRequestComment(ctx context.Context, repo *Repo, id int64, input CommentInput) (*Comment, error)
	MergePullRequest(ctx context.Context, repo *Repo, id int64, opts MergePullRequestOpts) (*PullRequest, error)

	Repo(ctx context.Context, namespace, slug string) (*Repo, error)
	Repos(ctx context.Context, pageToken *PageToken, accountName string, opts *ReposOptions) ([]*Repo, *PageToken, error)
	ForkRepository(ctx context.Context, upstream *Repo, input ForkInput) (*Repo, error)

	ListExplicitUserPermsForRepo(ctx context.Context, pageToken *PageToken, owner, slug string, opts *RequestOptions) ([]*Account, *PageToken, error)

	CurrentUser(ctx context.Context) (*User, error)
	CurrentUserEmails(ctx context.Context, pageToken *PageToken) ([]*UserEmail, *PageToken, error)
	AllCurrentUserEmails(ctx context.Context) ([]*UserEmail, error)
}

type RequestOptions struct {
	FetchAll bool
}

// client access a Bitbucket Cloud via the REST API 2.0.
type client struct {
	// HTTP Client used to communicate with the API
	httpClient httpcli.Doer

	// URL is the base URL of Bitbucket Cloud.
	URL *url.URL

	// Auth is the authentication method used when accessing the server. Only
	// auth.BasicAuth is currently supported.
	Auth auth.Authenticator

	// RateLimit is the self-imposed rate limiter (since Bitbucket does not have a concept
	// of rate limiting in HTTP response headers).
	rateLimit *ratelimit.InstrumentedLimiter
}

// NewClient creates a new Bitbucket Cloud API client from the given external
// service configuration. If a nil cf is provided, an external doer will be used.
func NewClient(urn string, config *schema.BitbucketCloudConnection, cf *httpcli.Factory) (Client, error) {
	return newClient(urn, config, cf)
}

func newClient(urn string, config *schema.BitbucketCloudConnection, cf *httpcli.Factory) (*client, error) {
	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	httpClient, err := cf.Doer(httpcli.CachedTransportOpt)
	if err != nil {
		return nil, err
	}

	httpClient = requestCounter.Doer(httpClient, func(u *url.URL) string {
		// The second component of the Path mostly maps to the type of API
		// request we are making.
		var category string
		if parts := strings.SplitN(u.Path, "/", 4); len(parts) > 2 {
			category = parts[2]
		}
		return category
	})

	apiURL, err := UrlFromConfig(config)
	if err != nil {
		return nil, err
	}

	var auther auth.Authenticator
	if config.AccessToken != "" {
		auther = &auth.OAuthBearerToken{Token: config.AccessToken}
	} else {
		auther = &auth.BasicAuth{
			Username: config.Username,
			Password: config.AppPassword,
		}
	}

	return &client{
		httpClient: httpClient,
		URL:        extsvc.NormalizeBaseURL(apiURL),
		Auth:       auther,
		// Default limits are defined in extsvc.GetLimitFromConfig
		rateLimit: ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("BitbucketCloudClient"), urn)),
	}, nil
}

func (c *client) Authenticator() auth.Authenticator {
	return c.Auth
}

// WithAuthenticator returns a new Client that uses the same configuration,
// HTTPClient, and RateLimiter as the current Client, except authenticated with
// the given authenticator instance.
//
// Note that using an unsupported Authenticator implementation may result in
// unexpected behaviour, or (more likely) errors. At present, only BasicAuth is
// supported.
func (c *client) WithAuthenticator(a auth.Authenticator) Client {
	return &client{
		httpClient: c.httpClient,
		URL:        c.URL,
		Auth:       a,
		rateLimit:  c.rateLimit,
	}
}

// Ping makes a request to the API root, thereby validating that the current
// authenticator is valid.
func (c *client) Ping(ctx context.Context) error {
	// This relies on an implementation detail: Bitbucket Cloud doesn't have an
	// API endpoint at /2.0/, but does the authentication check before returning
	// the 404, so we can distinguish based on the response code.
	//
	// The reason we do this is because there literally isn't an API call
	// available that doesn't require a specific scope.
	req, err := http.NewRequest("GET", "/2.0/", nil)
	if err != nil {
		return errors.Wrap(err, "creating request")
	}

	_, err = c.do(ctx, req, nil)
	if err != nil && !errcode.IsNotFound(err) {
		return err
	}
	return nil
}

func fetchAll[T any](ctx context.Context, c *client, results []T, next *PageToken, err error) ([]T, error) {
	var page []T
	var nextURL *url.URL
	for err == nil && next.HasMore() {
		nextURL, err = url.Parse(next.Next)
		if err != nil {
			return nil, err
		}
		next, err = c.page(ctx, nextURL.Path, nil, next, &page)
		results = append(results, page...)
	}

	return results, err
}

func (c *client) page(ctx context.Context, path string, qry url.Values, token *PageToken, results any) (*PageToken, error) {
	if qry == nil {
		qry = make(url.Values)
	}

	for k, vs := range token.Values() {
		qry[k] = append(qry[k], vs...)
	}

	u := url.URL{Path: path, RawQuery: qry.Encode()}
	return c.reqPage(ctx, u.String(), results)
}

// reqPage directly requests resources from given URL assuming all attributes have been
// included in the URL parameter. This is particular useful since the Bitbucket Cloud
// API 2.0 pagination renders the full link of next page in the response.
// See more at https://developer.atlassian.com/bitbucket/api/2/reference/meta/pagination
// However, for the very first request, use method page instead.
func (c *client) reqPage(ctx context.Context, url string, results any) (*PageToken, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var next PageToken
	_, err = c.do(ctx, req, &struct {
		*PageToken
		Values any `json:"values"`
	}{
		PageToken: &next,
		Values:    results,
	})

	if err != nil {
		return nil, err
	}

	return &next, nil
}

func (c *client) do(ctx context.Context, req *http.Request, result any) (code int, err error) {
	tr, ctx := trace.New(ctx, "BitbucketCloud.do")
	defer tr.EndWithErr(&err)
	req = req.WithContext(ctx)

	req.URL = c.URL.ResolveReference(req.URL)

	// If the request doesn't expect a body, then including a content-type can
	// actually cause errors on the Bitbucket side. So we need to pick apart the
	// request just a touch to figure out if we should add the header.
	var reqBody []byte
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return code, err
		}
	}
	req.Body = io.NopCloser(bytes.NewReader(reqBody))

	if err = c.rateLimit.Wait(ctx); err != nil {
		return code, err
	}

	// Because we have no external rate limiting data for Bitbucket Cloud, we do an exponential
	// back-off and retry for requests where we recieve a 429 Too Many Requests.
	// If we still don't succeed after waiting a total of 5 min, we give up.
	var resp *http.Response
	sleepTime := 10 * time.Second
	for {
		resp, err = oauthutil.DoRequest(ctx, nil, c.httpClient, req, c.Auth)
		if resp != nil {
			code = resp.StatusCode
		}
		if err != nil {
			return code, err
		}

		if code != http.StatusTooManyRequests {
			break
		}

		timeutil.SleepWithContext(ctx, sleepTime)
		sleepTime = sleepTime * 2
		if sleepTime.Seconds() > 160 {
			break
		}
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return code, err
	}

	if code < http.StatusOK || code >= http.StatusBadRequest {
		return code, errors.WithStack(&httpError{
			URL:        req.URL,
			StatusCode: code,
			Body:       string(bs),
		})
	}

	if result != nil {
		return code, json.Unmarshal(bs, result)
	}

	return code, nil
}

type PageToken struct {
	Size    int    `json:"size"`
	Page    int    `json:"page"`
	Pagelen int    `json:"pagelen"`
	Next    string `json:"next"`
}

func (t *PageToken) HasMore() bool {
	if t == nil {
		return false
	}
	return len(t.Next) > 0
}

func (t *PageToken) Values() url.Values {
	v := url.Values{}
	if t == nil {
		return v
	}
	if t.Next != "" {
		nextURL, err := url.Parse(t.Next)
		if err == nil {
			v = nextURL.Query()
		}
	}
	if t.Pagelen != 0 {
		v.Set("pagelen", strconv.Itoa(t.Pagelen))
	}
	return v
}

type httpError struct {
	StatusCode int
	URL        *url.URL
	Body       string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("Bitbucket Cloud API HTTP error: code=%d url=%q body=%q", e.StatusCode, e.URL, e.Body)
}

func (e *httpError) Unauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

func (e *httpError) NotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

func UrlFromConfig(config *schema.BitbucketCloudConnection) (*url.URL, error) {
	if config.ApiURL == "" {
		return url.Parse("https://api.bitbucket.org")
	}
	return url.Parse(config.ApiURL)
}
