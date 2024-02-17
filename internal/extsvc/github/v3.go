package github

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

	"github.com/google/go-github/v55/github"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// V3Client is a caching GitHub API client for GitHub's REST API v3.
//
// All instances use a map of rcache.Cache instances for caching (see the `repoCache` field). These
// separate instances have consistent naming prefixes so that different instances will share the
// same Redis cache entries (provided they were computed with the same API URL and access
// token). The cache keys are agnostic of the http.RoundTripper transport.
type V3Client struct {
	log log.Logger

	// The URN of the external service that the client is derived from.
	urn string

	// apiURL is the base URL of a GitHub API. It must point to the base URL of the GitHub API. This
	// is https://api.github.com for GitHub.com and http[s]://[github-enterprise-hostname]/api for
	// GitHub Enterprise.
	apiURL *url.URL

	// githubDotCom is true if this client connects to github.com.
	githubDotCom bool

	// auth is used to authenticate requests. May be empty, in which case the
	// default behavior is to make unauthenticated requests.
	// 🚨 SECURITY: Should not be changed after client creation to prevent
	// unauthorized access to the repository cache. Use `WithAuthenticator` to
	// create a new client with a different authenticator instead.
	auth auth.Authenticator

	// httpClient is the HTTP client used to make requests to the GitHub API.
	httpClient httpcli.Doer

	// externalRateLimiter is the external API rate limit monitor.
	externalRateLimiter *ratelimit.Monitor

	// internalRateLimiter is our self-imposed rate limiter
	internalRateLimiter *ratelimit.InstrumentedLimiter

	// waitForRateLimit determines whether or not the client will wait and retry a request if external rate limits are encountered
	waitForRateLimit bool

	// maxRateLimitRetries determines how many times we retry requests due to rate limits
	maxRateLimitRetries int

	// resource specifies which API this client is intended for.
	// One of 'rest' or 'search'.
	resource string
}

// NewV3Client creates a new GitHub API client with an optional default
// authenticator.
//
// apiURL must point to the base URL of the GitHub API. See the docstring for
// V3Client.apiURL.
func NewV3Client(logger log.Logger, urn string, apiURL *url.URL, a auth.Authenticator, cf *httpcli.Factory) (*V3Client, error) {
	cli, err := clientForFactory(cf)
	if err != nil {
		return nil, err
	}

	return newV3Client(logger, urn, apiURL, a, "rest", cli), nil
}

// NewV3SearchClient creates a new GitHub API client intended for use with the
// search API with an optional default authenticator.
//
// apiURL must point to the base URL of the GitHub API. See the docstring for
// V3Client.apiURL.
func NewV3SearchClient(logger log.Logger, urn string, apiURL *url.URL, a auth.Authenticator, cf *httpcli.Factory) (*V3Client, error) {
	cli, err := clientForFactory(cf)
	if err != nil {
		return nil, err
	}

	return newV3Client(logger, urn, apiURL, a, "search", cli), nil
}

func clientForFactory(cf *httpcli.Factory) (httpcli.Doer, error) {
	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}
	cf = cf.WithOpts(
		httpcli.CachedTransportOpt,
		// Use a 30s timeout to avoid running into EOF errors, because GitHub
		// closes idle connections after 60s
		httpcli.NewIdleConnTimeoutOpt(30*time.Second),
	)

	cli, err := cf.Doer()
	if err != nil {
		return nil, err
	}

	if gitHubDisable {
		cli = disabledClient{}
	}
	if cli == nil {
		cli = httpcli.ExternalDoer
	}

	cli = requestCounter.Doer(cli, func(u *url.URL) string {
		// The first component of the Path mostly maps to the type of API
		// request we are making. See `curl https://api.github.com` for the
		// exact mapping
		var category string
		if parts := strings.SplitN(u.Path, "/", 3); len(parts) > 1 {
			category = parts[1]
		}
		return category
	})

	return cli, nil
}

func newV3Client(logger log.Logger, urn string, apiURL *url.URL, a auth.Authenticator, resource string, cli httpcli.Doer) *V3Client {
	apiURL = canonicalizedURL(apiURL)

	var tokenHash string
	if a != nil {
		tokenHash = a.Hash()
	}

	rl := ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("GitHubClient"), urn))
	rlm := ratelimit.DefaultMonitorRegistry.GetOrSet(apiURL.String(), tokenHash, resource, &ratelimit.Monitor{HeaderPrefix: "X-"})

	return &V3Client{
		log: logger.Scoped("github.v3").
			With(
				log.String("urn", urn),
				log.String("resource", resource),
			),
		urn:                 urn,
		apiURL:              apiURL,
		githubDotCom:        URLIsGitHubDotCom(apiURL),
		auth:                a,
		httpClient:          cli,
		internalRateLimiter: rl,
		externalRateLimiter: rlm,
		resource:            resource,
		waitForRateLimit:    true,
		maxRateLimitRetries: 2,
	}
}

// WithAuthenticator returns a new V3Client that uses the same configuration as
// the current V3Client, except authenticated as the GitHub user with the given
// authenticator instance (most likely a token).
func (c *V3Client) WithAuthenticator(a auth.Authenticator) *V3Client {
	return newV3Client(c.log, c.urn, c.apiURL, a, c.resource, c.httpClient)
}

// SetWaitForRateLimit sets whether the client should respond to external rate
// limits by waiting and retrying a request.
func (c *V3Client) SetWaitForRateLimit(wait bool) {
	c.waitForRateLimit = wait
}

// ExternalRateLimiter exposes the rate limit monitor.
func (c *V3Client) ExternalRateLimiter() *ratelimit.Monitor {
	return c.externalRateLimiter
}

func (c *V3Client) get(ctx context.Context, requestURI string, result any) (*httpResponseState, error) {
	req, err := http.NewRequest("GET", requestURI, nil)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, req, result)
}

func (c *V3Client) post(ctx context.Context, requestURI string, payload, result any) (*httpResponseState, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling payload")
	}

	req, err := http.NewRequest("POST", requestURI, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return c.request(ctx, req, result)
}

func (c *V3Client) patch(ctx context.Context, requestURI string, payload, result any) (*httpResponseState, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling payload")
	}

	req, err := http.NewRequest("PATCH", requestURI, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return c.request(ctx, req, result)
}

func (c *V3Client) delete(ctx context.Context, requestURI string) (*httpResponseState, error) {
	req, err := http.NewRequest("DELETE", requestURI, bytes.NewReader(make([]byte, 0)))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return c.request(ctx, req, struct{}{})
}

func (c *V3Client) request(ctx context.Context, req *http.Request, result any) (*httpResponseState, error) {
	// Include node_id (GraphQL ID) in response. See
	// https://developer.github.com/changes/2017-12-19-graphql-node-id/.
	//
	// Enable the repository topics API. See
	// https://developer.github.com/v3/repos/#list-all-topics-for-a-repository
	req.Header.Add("Accept", "application/vnd.github.jean-grey-preview+json,application/vnd.github.mercy-preview+json")

	// Enable the GitHub App API. See
	// https://developer.github.com/v3/apps/installations/#list-repositories
	req.Header.Add("Accept", "application/vnd.github.machine-man-preview+json")

	if conf.ExperimentalFeatures().EnableGithubInternalRepoVisibility {
		// Include "visibility" in the REST API response for getting a repository. See
		// https://docs.github.com/en/enterprise-server@2.22/rest/reference/repos#get-a-repository
		req.Header.Add("Accept", "application/vnd.github.nebula-preview+json")
	}

	err := c.internalRateLimiter.Wait(ctx)
	if err != nil {
		// We don't want to return a misleading rate limit exceeded error if the error is coming
		// from the context.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		c.log.Warn("internal rate limiter error", log.Error(err))
		return nil, errInternalRateLimitExceeded
	}

	if c.waitForRateLimit {
		c.externalRateLimiter.WaitForRateLimit(ctx, 1) // We don't care whether we waited or not, this is a preventative measure.
	}

	// Store request Body and URL because we might call `doRequest` twice and
	// can't guarantee that `doRequest` doesn't modify them. (In fact: it does
	// modify them!)
	// So when we retry, we can reset to the original state.
	var reqBody []byte
	var reqURL *url.URL
	if req.Body != nil {
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}
	if req.URL != nil {
		u := *req.URL
		reqURL = &u
	}

	req.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	var resp *httpResponseState
	resp, err = doRequest(ctx, c.log, c.apiURL, c.auth, c.externalRateLimiter, c.httpClient, req, result)

	apiError := &APIError{}
	numRetries := 0
	// We retry only if waitForRateLimit is set, and until:
	// 1. We've exceeded the number of retries
	// 2. The error returned is not a rate limit error
	// 3. We succeed
	for c.waitForRateLimit && err != nil && numRetries < c.maxRateLimitRetries &&
		errors.As(err, &apiError) && apiError.Code == http.StatusForbidden {
		// Because GitHub responds with http.StatusForbidden when a rate limit is hit, we cannot
		// say with absolute certainty that a rate limit was hit. It might have been an honest
		// http.StatusForbidden. So we use the externalRateLimiter's WaitForRateLimit function
		// to calculate the amount of time we need to wait before retrying the request.
		// If that calculated time is zero or in the past, we have to assume that the
		// rate limiting information we have is old and no longer relevant.
		//
		// There is an extremely unlikely edge case where we will falsely not retry a request.
		// If a request is rejected because we have no more rate limit tokens, but the token reset
		// time is just around the corner (like 1 second from now), and for some reason the time
		// between reading the headers and doing this "should we retry" check is greater than
		// that time, the rate limit information we will have will look like old information and
		// we won't retry the request.
		if c.externalRateLimiter.WaitForRateLimit(ctx, 1) {
			// Reset Body/URL to ignore changes that the first `doRequest`
			// might have made.
			req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			// Create a copy of the URL, because this loop might execute
			// multiple times.
			reqURLCopy := *reqURL
			req.URL = &reqURLCopy

			resp, err = doRequest(ctx, c.log, c.apiURL, c.auth, c.externalRateLimiter, c.httpClient, req, result)
			numRetries++
		} else {
			// We did not wait because of rate limiting, so we break the loop
			break
		}
	}

	return resp, err
}

// APIError is an error type returned by Client when the GitHub API responds with
// an error.
type APIError struct {
	URL              string
	Code             int
	Message          string
	DocumentationURL string `json:"documentation_url"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("request to %s returned status %d: %s", e.URL, e.Code, e.Message)
}

func (e *APIError) Unauthorized() bool {
	return e.Code == http.StatusUnauthorized
}

func (e *APIError) AccountSuspended() bool {
	return e.Code == http.StatusForbidden && strings.Contains(e.Message, "account was suspended")
}

func (e *APIError) UnavailableForLegalReasons() bool {
	return e.Code == http.StatusUnavailableForLegalReasons
}

func (e *APIError) Temporary() bool { return IsRateLimitExceeded(e) }

// HTTPErrorCode returns err's HTTP status code, if it is an HTTP error from
// this package. Otherwise it returns 0.
func HTTPErrorCode(err error) int {
	var e *APIError
	if errors.As(err, &e) {
		return e.Code
	}

	return 0
}

func (c *V3Client) GetVersion(ctx context.Context) (string, error) {
	if c.githubDotCom {
		return "unknown", nil
	}

	var empty any

	respState, err := c.get(ctx, "/", &empty)
	if err != nil {
		return "", err
	}
	v := respState.headers.Get("X-GitHub-Enterprise-Version")
	return v, nil
}

func (c *V3Client) GetAuthenticatedUser(ctx context.Context) (*User, error) {
	var u User
	_, err := c.get(ctx, "/user", &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

var MockGetAuthenticatedUserEmails func(ctx context.Context) ([]*UserEmail, error)

// GetAuthenticatedUserEmails returns the first 100 emails associated with the currently
// authenticated user.
func (c *V3Client) GetAuthenticatedUserEmails(ctx context.Context) ([]*UserEmail, error) {
	if MockGetAuthenticatedUserEmails != nil {
		return MockGetAuthenticatedUserEmails(ctx)
	}

	var emails []*UserEmail
	_, err := c.get(ctx, "/user/emails?per_page=100", &emails)
	if err != nil {
		return nil, err
	}
	return emails, nil
}

var MockGetAuthenticatedUserOrgs struct {
	FnMock    func(ctx context.Context) ([]*Org, bool, int, error)
	PagesMock map[int][]*Org
}

// GetAuthenticatedUserOrgs returns given page of 100 organizations associated with the currently
// authenticated user.
func (c *V3Client) GetAuthenticatedUserOrgs(ctx context.Context, page int) (
	orgs []*Org,
	hasNextPage bool,
	rateLimitCost int,
	err error,
) {
	// checking whether the function is mocked
	if MockGetAuthenticatedUserOrgs.FnMock != nil || MockGetAuthenticatedUserOrgs.PagesMock != nil {
		if MockGetAuthenticatedUserOrgs.FnMock != nil {
			return MockGetAuthenticatedUserOrgs.FnMock(ctx)
		}

		orgsPage, ok := MockGetAuthenticatedUserOrgs.PagesMock[page]
		if !ok {
			err = errors.New("cannot find orgs page mock")
			return
		}
		return orgsPage, page < len(MockGetAuthenticatedUserOrgs.PagesMock), 1, err
	}

	respState, err := c.get(ctx, fmt.Sprintf("/user/orgs?per_page=100&page=%d", page), &orgs)
	if err != nil {
		return
	}
	return orgs, respState.hasNextPage(), 1, err
}

// OrgDetailsAndMembership is a results container for the results from the API calls made
// in GetAuthenticatedUserOrgsDetailsAndMembership
type OrgDetailsAndMembership struct {
	*OrgDetails

	*OrgMembership
}

// GetAuthenticatedUserOrgsDetailsAndMembership returns the organizations associated with the currently
// authenticated user as well as additional information about each org by making API
// requests for each org (see `OrgDetails` and `OrgMembership` docs for more details).
func (c *V3Client) GetAuthenticatedUserOrgsDetailsAndMembership(ctx context.Context, page int) (
	orgs []OrgDetailsAndMembership,
	hasNextPage bool,
	rateLimitCost int,
	err error,
) {
	orgNames, hasNextPage, cost, err := c.GetAuthenticatedUserOrgs(ctx, page)
	if err != nil {
		return
	}
	orgs = make([]OrgDetailsAndMembership, len(orgNames))
	for i, org := range orgNames {
		if _, err = c.get(ctx, "/orgs/"+org.Login, &orgs[i].OrgDetails); err != nil {
			return nil, false, cost + 2*i, err
		}
		if _, err = c.get(ctx, "/user/memberships/orgs/"+org.Login, &orgs[i].OrgMembership); err != nil {
			return nil, false, cost + 2*i, err
		}
	}
	return orgs,
		hasNextPage,
		cost + 2*len(orgs), // 2 requests per org
		nil
}

type restTeam struct {
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
	URL  string `json:"url,omitempty"`

	ReposCount   int  `json:"repos_count,omitempty"`
	Organization *Org `json:"organization,omitempty"`
}

func (t *restTeam) convert() *Team {
	return &Team{
		Name:         t.Name,
		Slug:         t.Slug,
		URL:          t.URL,
		ReposCount:   t.ReposCount,
		Organization: t.Organization,
	}
}

var MockGetAuthenticatedUserTeams func(ctx context.Context, page int) ([]*Team, bool, int, error)

// GetAuthenticatedUserTeams lists GitHub teams affiliated with the client token.
//
// The page is the page of results to return, and is 1-indexed (so the first call should
// be for page 1).
func (c *V3Client) GetAuthenticatedUserTeams(ctx context.Context, page int) (
	teams []*Team,
	hasNextPage bool,
	rateLimitCost int,
	err error,
) {
	if MockGetAuthenticatedUserTeams != nil {
		return MockGetAuthenticatedUserTeams(ctx, 1)
	}

	var restTeams []*restTeam
	respState, err := c.get(ctx, fmt.Sprintf("/user/teams?per_page=100&page=%d", page), &restTeams)
	if err != nil {
		return
	}

	teams = make([]*Team, len(restTeams))
	for i, t := range restTeams {
		teams[i] = t.convert()
	}

	return teams, respState.hasNextPage(), 1, err
}

var MockGetAuthenticatedOAuthScopes func(ctx context.Context) ([]string, error)

// GetAuthenticatedOAuthScopes gets the list of OAuth scopes granted to the token in use.
func (c *V3Client) GetAuthenticatedOAuthScopes(ctx context.Context) ([]string, error) {
	if MockGetAuthenticatedOAuthScopes != nil {
		return MockGetAuthenticatedOAuthScopes(ctx)
	}
	// We only care about headers
	var dest struct{}
	respState, err := c.get(ctx, "/", &dest)
	if err != nil {
		return nil, err
	}
	scope := respState.headers.Get("x-oauth-scopes")
	if scope == "" {
		return []string{}, nil
	}
	return strings.Split(scope, ", "), nil
}

// ListRepositoryCollaborators lists GitHub users that has access to the repository.
//
// The page is the page of results to return, and is 1-indexed (so the first call should
// be for page 1). If no affiliations are provided, all users with access to the repository
// are listed.
func (c *V3Client) ListRepositoryCollaborators(ctx context.Context, owner, repo string, page int, affiliation CollaboratorAffiliation) (users []*Collaborator, hasNextPage bool, _ error) {
	path := fmt.Sprintf("/repos/%s/%s/collaborators?page=%d&per_page=100", owner, repo, page)
	if len(affiliation) > 0 {
		path = fmt.Sprintf("%s&affiliation=%s", path, affiliation)
	}
	respState, err := c.get(ctx, path, &users)
	if err != nil {
		return nil, false, err
	}
	return users, respState.hasNextPage(), nil
}

// ListRepositoryTeams lists GitHub teams that has access to the repository.
//
// The page is the page of results to return, and is 1-indexed (so the first call should
// be for page 1).
func (c *V3Client) ListRepositoryTeams(ctx context.Context, owner, repo string, page int) (teams []*Team, hasNextPage bool, _ error) {
	path := fmt.Sprintf("/repos/%s/%s/teams?page=%d&per_page=100", owner, repo, page)
	var restTeams []*restTeam
	respState, err := c.get(ctx, path, &restTeams)
	if err != nil {
		return nil, false, err
	}
	teams = make([]*Team, len(restTeams))
	for i, t := range restTeams {
		teams[i] = t.convert()
	}
	return teams, respState.hasNextPage(), nil
}

// GetRepository gets a repository from GitHub by owner and repository name.
func (c *V3Client) GetRepository(ctx context.Context, owner, name string) (*Repository, error) {
	return c.getRepositoryFromAPI(ctx, owner, name)
}

// GetOrganization gets an org from GitHub by its login.
func (c *V3Client) GetOrganization(ctx context.Context, login string) (org *OrgDetails, err error) {
	_, err = c.get(ctx, "/orgs/"+login, &org)
	if err != nil && strings.Contains(err.Error(), "404") {
		err = &OrgNotFoundError{}
	}
	return
}

// ListOrganizations lists all orgs from GitHub. This is intended to be used for GitHub enterprise
// server instances only. Callers should be careful not to use this for github.com or GitHub
// enterprise cloud.
//
// The argument "since" is the ID of the org and the API call will only return orgs with ID greater
// than this value. To list all orgs in a GitHub instance, invoke this initially with:
//
// orgs, nextSince, err := ListOrganizations(ctx, 0)
//
// And the next call with:
//
// orgs, nextSince, err := ListOrganizations(ctx, nextSince)
//
// Repeat this in a for-loop until nextSince is a non-positive integer.
//
// 🚀🚀🚀
//
// This API supports conditional requests and the underlying httpcache transport can leverage this
// to use the cache to return responses.
func (c *V3Client) ListOrganizations(ctx context.Context, since int) (orgs []*Org, nextSince int, err error) {
	path := fmt.Sprintf("/organizations?since=%d&per_page=100", since)

	_, err = c.get(ctx, path, &orgs)
	if err != nil {
		return nil, -1, err
	}

	getNextSince := func() int {
		total := len(orgs)
		if total == 0 {
			return -1
		}

		return orgs[total-1].ID
	}

	return orgs, getNextSince(), nil
}

// ListOrganizationMembers retrieves collaborators in the given organization.
//
// The page is the page of results to return, and is 1-indexed (so the first call should
// be for page 1).
func (c *V3Client) ListOrganizationMembers(ctx context.Context, owner string, page int, adminsOnly bool) (users []*Collaborator, hasNextPage bool, _ error) {
	path := fmt.Sprintf("/orgs/%s/members?page=%d&per_page=100", owner, page)
	if adminsOnly {
		path += "&role=admin"
	}
	respState, err := c.get(ctx, path, &users)
	if err != nil {
		return nil, false, err
	}
	return users, respState.hasNextPage(), nil
}

// ListTeamMembers retrieves collaborators in the given team.
//
// The team should be the team slug, not team name.
// The page is the page of results to return, and is 1-indexed (so the first call should
// be for page 1).
func (c *V3Client) ListTeamMembers(ctx context.Context, owner, team string, page int) (users []*Collaborator, hasNextPage bool, _ error) {
	path := fmt.Sprintf("/orgs/%s/teams/%s/members?page=%d&per_page=100", owner, team, page)
	respState, err := c.get(ctx, path, &users)
	if err != nil {
		return nil, false, err
	}
	return users, respState.hasNextPage(), nil
}

// getPublicRepositories returns a page of public repositories that were created
// after the repository identified by sinceRepoID.
// An empty sinceRepoID returns the first page of results.
// This is only intended to be called for GitHub Enterprise, so no rate limit information is returned.
// https://developer.github.com/v3/repos/#list-all-public-repositories
func (c *V3Client) getPublicRepositories(ctx context.Context, sinceRepoID int64) ([]*Repository, bool, error) {
	path := "repositories"
	if sinceRepoID > 0 {
		path += "?per_page=100&since=" + strconv.FormatInt(sinceRepoID, 10)
	}
	return c.listRepositories(ctx, path)
}

func (c *V3Client) ListPublicRepositories(ctx context.Context, sinceRepoID int64) ([]*Repository, bool, error) {
	return c.getPublicRepositories(ctx, sinceRepoID)
}

// ListAffiliatedRepositories lists GitHub repositories affiliated with the client token.
//
// page is the page of results to return, and is 1-indexed (so the first call should be
// for page 1).
// visibility and affiliations are filters for which repositories should be returned.
func (c *V3Client) ListAffiliatedRepositories(ctx context.Context, visibility Visibility, page int, perPage int, affiliations ...RepositoryAffiliation) (
	repos []*Repository,
	hasNextPage bool,
	rateLimitCost int,
	err error,
) {
	path := fmt.Sprintf("user/repos?sort=created&visibility=%s&page=%d&per_page=%d", visibility, page, perPage)
	if len(affiliations) > 0 {
		affilationsStrings := make([]string, 0, len(affiliations))
		for _, affiliation := range affiliations {
			affilationsStrings = append(affilationsStrings, string(affiliation))
		}
		path = fmt.Sprintf("%s&affiliation=%s", path, strings.Join(affilationsStrings, ","))
	}
	repos, hasNextPage, err = c.listRepositories(ctx, path)

	return repos, hasNextPage, 1, err
}

// ListOrgRepositories lists GitHub repositories from the specified organization.
// org is the name of the organization. page is the page of results to return.
// Pages are 1-indexed (so the first call should be for page 1).
func (c *V3Client) ListOrgRepositories(ctx context.Context, org string, page int, repoType string) (repos []*Repository, hasNextPage bool, rateLimitCost int, err error) {
	path := fmt.Sprintf("orgs/%s/repos?sort=created&page=%d&per_page=100&type=%s", org, page, repoType)
	repos, hasNextPage, err = c.listRepositories(ctx, path)
	return repos, hasNextPage, 1, err
}

// ListTeamRepositories lists GitHub repositories from the specified team.
// org is the name of the team's organization. team is the team slug (not name).
// page is the page of results to return. Pages are 1-indexed (so the first call should be for page 1).
func (c *V3Client) ListTeamRepositories(ctx context.Context, org, team string, page int) (repos []*Repository, hasNextPage bool, rateLimitCost int, err error) {
	path := fmt.Sprintf("orgs/%s/teams/%s/repos?page=%d&per_page=100", org, team, page)
	repos, hasNextPage, err = c.listRepositories(ctx, path)
	return repos, hasNextPage, 1, err
}

// ListUserRepositories lists GitHub repositories from the specified user.
// Pages are 1-indexed (so the first call should be for page 1)
func (c *V3Client) ListUserRepositories(ctx context.Context, user string, page int) (repos []*Repository, hasNextPage bool, rateLimitCost int, err error) {
	path := fmt.Sprintf("users/%s/repos?sort=created&type=owner&page=%d&per_page=100", user, page)
	repos, hasNextPage, err = c.listRepositories(ctx, path)
	return repos, hasNextPage, 1, err
}

func (c *V3Client) ListRepositoriesForSearch(ctx context.Context, searchString string, page int) (RepositoryListPage, error) {
	urlValues := url.Values{
		"q":        []string{searchString},
		"page":     []string{strconv.Itoa(page)},
		"per_page": []string{"100"},
	}
	path := "search/repositories?" + urlValues.Encode()
	var response restSearchResponse
	if _, err := c.get(ctx, path, &response); err != nil {
		return RepositoryListPage{}, err
	}
	if response.IncompleteResults {
		return RepositoryListPage{}, ErrIncompleteResults
	}
	repos := make([]*Repository, 0, len(response.Items))
	for _, restRepo := range response.Items {
		repos = append(repos, convertRestRepo(restRepo))
	}

	return RepositoryListPage{
		TotalCount:  response.TotalCount,
		Repos:       repos,
		HasNextPage: page*100 < response.TotalCount,
	}, nil
}

// ListTopicsOnRepository lists topics on the given repository.
func (c *V3Client) ListTopicsOnRepository(ctx context.Context, ownerAndName string) ([]string, error) {
	owner, name, err := SplitRepositoryNameWithOwner(ownerAndName)
	if err != nil {
		return nil, err
	}

	var result restTopicsResponse
	if _, err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/topics", owner, name), &result); err != nil {
		if HTTPErrorCode(err) == http.StatusNotFound {
			return nil, ErrRepoNotFound
		}
		return nil, err
	}
	return result.Names, nil
}

// ListInstallationRepositories lists repositories on which the authenticated
// GitHub App has been installed.
//
// API docs: https://docs.github.com/en/rest/reference/apps#list-repositories-accessible-to-the-app-installation
func (c *V3Client) ListInstallationRepositories(ctx context.Context, page int) (
	repos []*Repository,
	hasNextPage bool,
	rateLimitCost int,
	err error,
) {
	type response struct {
		Repositories []restRepository `json:"repositories"`
	}
	var resp response
	path := fmt.Sprintf("installation/repositories?page=%d&per_page=100", page)
	respState, err := c.get(ctx, path, &resp)
	if err != nil {
		return nil, false, 1, err
	}
	repos = make([]*Repository, 0, len(resp.Repositories))
	for _, restRepo := range resp.Repositories {
		repos = append(repos, convertRestRepo(restRepo))
	}
	return repos, respState.hasNextPage(), 1, nil
}

// listRepositories is a generic method that unmarshalls the given JSON HTTP
// endpoint into a []restRepository. It will return an error if it fails.
//
// This is used to extract repositories from the GitHub API endpoints:
// - /users/:user/repos
// - /orgs/:org/repos
// - /user/repos
func (c *V3Client) listRepositories(ctx context.Context, requestURI string) ([]*Repository, bool, error) {
	var restRepos []restRepository
	respState, err := c.get(ctx, requestURI, &restRepos)
	if err != nil {
		return nil, false, err
	}
	repos := make([]*Repository, 0, len(restRepos))
	for _, restRepo := range restRepos {
		// Sometimes GitHub API returns null JSON objects and JSON decoder unmarshalls
		// them as a zero-valued `restRepository` objects.
		//
		// See https://github.com/sourcegraph/customer/issues/1688 for details.
		if restRepo.ID == "" {
			c.log.Warn("GitHub returned a repository without an ID", log.String("restRepository", fmt.Sprintf("%#v", restRepo)))
			continue
		}
		repos = append(repos, convertRestRepo(restRepo))
	}
	return repos, respState.hasNextPage(), nil
}

func (c *V3Client) GetRepo(ctx context.Context, owner, repo string) (*Repository, error) {
	var restRepo restRepository
	if _, err := c.get(ctx, "repos/"+owner+"/"+repo, &restRepo); err != nil {
		return nil, err
	}

	return convertRestRepo(restRepo), nil
}

// Fork forks the given repository. If org is given, then the repository will
// be forked into that organisation, otherwise the repository is forked into
// the authenticated user's account.
func (c *V3Client) Fork(ctx context.Context, owner, repo string, org *string, forkName string) (*Repository, error) {
	// GitHub's fork endpoint will happily accept either a new or existing fork,
	// and returns a valid repository either way. As such, we don't need to check
	// if there's already an extant fork.

	payload := struct {
		Org  *string `json:"organization,omitempty"`
		Name string  `json:"name"`
	}{Org: org, Name: forkName}

	var restRepo restRepository
	if _, err := c.post(ctx, "repos/"+owner+"/"+repo+"/forks", payload, &restRepo); err != nil {
		return nil, err
	}

	return convertRestRepo(restRepo), nil
}

// DeleteBranch deletes the given branch from the given repository.
func (c *V3Client) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	if _, err := c.delete(ctx, "repos/"+owner+"/"+repo+"/git/refs/heads/"+branch); err != nil {
		return err
	}
	return nil
}

// GetRef gets the contents of a single commit reference in a repository. The ref should
// be supplied in a fully qualified format, such as `refs/heads/branch` or
// `refs/tags/tag`.
func (c *V3Client) GetRef(ctx context.Context, owner, repo, ref string) (*restCommitRef, error) {
	var commit restCommitRef
	if _, err := c.get(ctx, "repos/"+owner+"/"+repo+"/commits/"+ref, &commit); err != nil {
		return nil, err
	}
	return &commit, nil
}

// CreateCommit creates a commit in the given repository based on a tree object.
func (c *V3Client) CreateCommit(ctx context.Context, owner, repo, message, tree string, parents []string, author, committer *restAuthorCommiter) (*RestCommit, error) {
	payload := struct {
		Message   string              `json:"message"`
		Tree      string              `json:"tree"`
		Parents   []string            `json:"parents"`
		Author    *restAuthorCommiter `json:"author,omitempty"`
		Committer *restAuthorCommiter `json:"committer,omitempty"`
	}{Message: message, Tree: tree, Parents: parents, Author: author, Committer: committer}

	var commit RestCommit
	if _, err := c.post(ctx, "repos/"+owner+"/"+repo+"/git/commits", payload, &commit); err != nil {
		return nil, err
	}
	return &commit, nil
}

// UpdateRef updates the ref of a branch to point to the given commit. The ref should be
// supplied in a fully qualified format, such as `refs/heads/branch` or `refs/tags/tag`.
func (c *V3Client) UpdateRef(ctx context.Context, owner, repo, ref, commit string) (*restUpdatedRef, error) {
	var updatedRef restUpdatedRef
	if _, err := c.patch(ctx, "repos/"+owner+"/"+repo+"/git/"+ref, struct {
		SHA   string `json:"sha"`
		Force bool   `json:"force"`
	}{SHA: commit, Force: true}, &updatedRef); err != nil {
		return nil, err
	}
	return &updatedRef, nil
}

// GetAppInstallation gets information of a GitHub App installation.
//
// API docs: https://docs.github.com/en/rest/reference/apps#get-an-installation-for-the-authenticated-app
func (c *V3Client) GetAppInstallation(ctx context.Context, installationID int64) (*github.Installation, error) {
	var ins github.Installation
	if _, err := c.get(ctx, fmt.Sprintf("app/installations/%d", installationID), &ins); err != nil {
		return nil, err
	}
	return &ins, nil
}

// GetAppInstallations fetches a list of GitHub App instalaltions for the
// authenticated GitHub App.
//
// API docs: https://docs.github.com/en/rest/apps/apps#list-installations-for-the-authenticated-app
func (c *V3Client) GetAppInstallations(ctx context.Context, page int) (ins []*github.Installation, hasNextPage bool, _ error) {
	respState, err := c.get(ctx, fmt.Sprintf("app/installations?page=%d", page), &ins)
	if err != nil {
		return nil, false, err
	}
	return ins, respState.hasNextPage(), nil
}

// CreateAppInstallationAccessToken creates an access token for the installation.
//
// API docs: https://docs.github.com/en/rest/reference/apps#create-an-installation-access-token-for-an-app
func (c *V3Client) CreateAppInstallationAccessToken(ctx context.Context, installationID int64) (*github.InstallationToken, error) {
	var token github.InstallationToken
	if _, err := c.post(ctx, fmt.Sprintf("app/installations/%d/access_tokens", installationID), nil, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// GetUserInstallations returns a list of GitHub App installations the user has access to
//
// API docs: https://docs.github.com/en/rest/reference/apps#list-app-installations-accessible-to-the-user-access-token
func (c *V3Client) GetUserInstallations(ctx context.Context) ([]github.Installation, error) {
	var resultStruct struct {
		Installations []github.Installation `json:"installations,omitempty"`
	}
	if _, err := c.get(ctx, "user/installations", &resultStruct); err != nil {
		return nil, err
	}

	return resultStruct.Installations, nil
}

type WebhookPayload struct {
	Name   string   `json:"name"`
	ID     int      `json:"id,omitempty"`
	Config Config   `json:"config"`
	Events []string `json:"events"`
	Active bool     `json:"active"`
	URL    string   `json:"url"`
}

type Config struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Secret      string `json:"secret"`
	InsecureSSL string `json:"insecure_ssl"`
	Token       string `json:"token"`
	Digest      string `json:"digest,omitempty"`
}

// CreateSyncWebhook returns the id of the newly created webhook, or 0 if there
// was an error
//
// Cloud API docs: https://docs.github.com/en/enterprise-cloud@latest/rest/webhooks/repos#create-a-repository-webhook
// Server API docs: https://docs.github.com/en/enterprise-server@3.3/rest/webhooks/repos#create-a-repository-webhook
func (c *V3Client) CreateSyncWebhook(ctx context.Context, repoName, targetHost, secret string) (int, error) {
	hooksUrl, err := webhookURLBuilder(repoName)
	if err != nil {
		return 0, err
	}

	payload := WebhookPayload{
		Name:   "web",
		Active: true,
		Config: Config{
			URL:         fmt.Sprintf("https://%s/github-webhooks", targetHost),
			ContentType: "json",
			Secret:      secret,
			InsecureSSL: "0",
		},
		Events: []string{
			"push",
		},
	}

	var result WebhookPayload
	resp, err := c.post(ctx, hooksUrl, payload, &result)
	if err != nil {
		return 0, err
	}

	if resp.statusCode != http.StatusCreated {
		return 0, errors.Newf("expected status code 201, got %d", resp.statusCode)
	}

	return result.ID, nil
}

// ListSyncWebhooks returns an array of WebhookPayloads
//
// Cloud API docs: https://docs.github.com/en/enterprise-cloud@latest/rest/webhooks/repos#list-repository-webhooks
// Server API docs: https://docs.github.com/en/enterprise-server@3.3/rest/webhooks/repos#list-repository-webhooks
func (c *V3Client) ListSyncWebhooks(ctx context.Context, repoName string) ([]WebhookPayload, error) {
	hooksUrl, err := webhookURLBuilder(repoName)
	if err != nil {
		return nil, err
	}

	var results []WebhookPayload
	resp, err := c.get(ctx, hooksUrl, &results)
	if err != nil {
		return nil, err
	}

	if resp.statusCode != http.StatusOK {
		return nil, errors.Newf("expected status code 200, got %d", resp.statusCode)
	}

	return results, nil
}

// FindSyncWebhook looks for any webhook with the targetURL ending in
// /github-webhooks
func (c *V3Client) FindSyncWebhook(ctx context.Context, repoName string) (*WebhookPayload, error) {
	payloads, err := c.ListSyncWebhooks(ctx, repoName)
	if err != nil {
		return nil, err
	}

	for _, payload := range payloads {
		if strings.Contains(payload.Config.URL, "github-webhooks") {
			return &payload, nil
		}
	}

	return nil, errors.New("unable to find webhook")
}

// DeleteSyncWebhook returns a boolean answer as to whether the target repo was
// deleted or not
//
// Cloud API docs: https://docs.github.com/en/enterprise-cloud@latest/rest/webhooks/repos#delete-a-repository-webhook
// Server API docs: https://docs.github.com/en/enterprise-server@3.3/rest/webhooks/repos#delete-a-repository-webhook
func (c *V3Client) DeleteSyncWebhook(ctx context.Context, repoName string, hookID int) (bool, error) {
	hookUrl, err := webhookURLBuilderWithID(repoName, hookID)
	if err != nil {
		return false, err
	}

	resp, err := c.delete(ctx, hookUrl)
	if err != nil && err != io.EOF {
		return false, err
	}

	if resp.statusCode != http.StatusNoContent {
		return false, errors.Newf("expected status code 204, got %d", resp.statusCode)
	}

	return true, nil
}

// webhookURLBuilder builds the URL to interface with the GitHub Webhooks API
func webhookURLBuilder(repoName string) (string, error) {
	repoName = fmt.Sprintf("//%s", repoName)
	u, err := url.Parse(repoName)
	if err != nil {
		return "", errors.Newf("error parsing URL:", err)
	}

	if u.Host == "github.com" {
		return fmt.Sprintf("https://api.github.com/repos%s/hooks", u.Path), nil
	}
	return fmt.Sprintf("https://%s/api/v3/repos%s/hooks", u.Host, u.Path), nil
}

// webhookURLBuilderWithID builds the URL to interface with the GitHub Webhooks
// API but with a hook ID
func webhookURLBuilderWithID(repoName string, hookID int) (string, error) {
	repoName = fmt.Sprintf("//%s", repoName)
	u, err := url.Parse(repoName)
	if err != nil {
		return "", errors.Newf("error parsing URL:", err)
	}

	if u.Host == "github.com" {
		return fmt.Sprintf("https://api.github.com/repos%s/hooks/%d", u.Path, hookID), nil
	}
	return fmt.Sprintf("https://%s/api/v3/repos%s/hooks/%d", u.Host, u.Path, hookID), nil
}

// responseHasNextPage checks if the Link header of the response contains a
// URL tagged with rel="next".
// If this header is not present, it also means there is only one page.
func (r *httpResponseState) hasNextPage() bool {
	return strings.Contains(r.headers.Get("Link"), "rel=\"next\"")
}
