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

	"github.com/Masterminds/semver"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/visitor"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// V4Client is a GitHub GraphQL API client.
type V4Client struct {
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

	// externalRateLimiter is the API rate limit monitor.
	externalRateLimiter *ratelimit.Monitor

	// internalRateLimiter is our self imposed rate limiter.
	internalRateLimiter *ratelimit.InstrumentedLimiter

	// waitForRateLimit determines whether or not the client will wait and retry a request if external rate limits are encountered
	waitForRateLimit bool

	// maxRateLimitRetries determines how many times we retry requests due to rate limits
	maxRateLimitRetries int
}

// NewV4Client creates a new GitHub GraphQL API client with an optional default
// authenticator.
//
// apiURL must point to the base URL of the GitHub API. See the docstring for
// V4Client.apiURL.
func NewV4Client(urn string, apiURL *url.URL, a auth.Authenticator, cf *httpcli.Factory) (*V4Client, error) {
	apiURL = canonicalizedURL(apiURL)

	if cf == nil {
		// We don't need a cached doer here like we initialize for the v3 client,
		// because GraphQL is using POST only and we don't cache that, also GitHub
		// doesn't have a way to not count GraphQL requests that are cacheable
		// to the rate limits.
		cf = httpcli.NewExternalClientFactory()
	}

	cf = cf.WithOpts(
		// Use a 30s timeout to avoid running into EOF errors, because GitHub
		// closes idle connections after 60s
		httpcli.NewIdleConnTimeoutOpt(30 * time.Second),
	)

	cli, err := cf.Doer()
	if err != nil {
		return nil, err
	}

	if gitHubDisable {
		cli = disabledClient{}
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

	return newV4Client(urn, apiURL, a, cli), nil
}

func newV4Client(urn string, apiURL *url.URL, auth auth.Authenticator, cli httpcli.Doer) *V4Client {
	var tokenHash string
	if auth != nil {
		tokenHash = auth.Hash()
	}

	rl := ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("GitHubClient"), urn))
	rlm := ratelimit.DefaultMonitorRegistry.GetOrSet(apiURL.String(), tokenHash, "graphql", &ratelimit.Monitor{HeaderPrefix: "X-"})

	return &V4Client{
		log:                 log.Scoped("github.v4"),
		urn:                 urn,
		apiURL:              apiURL,
		githubDotCom:        URLIsGitHubDotCom(apiURL),
		auth:                auth,
		httpClient:          cli,
		internalRateLimiter: rl,
		externalRateLimiter: rlm,
		waitForRateLimit:    true,
		maxRateLimitRetries: 2,
	}
}

// WithAuthenticator returns a new V4Client that uses the same configuration as
// the current V4Client, except authenticated as the GitHub user with the given
// authenticator instance (most likely a token).
func (c *V4Client) WithAuthenticator(a auth.Authenticator) *V4Client {
	return newV4Client(c.urn, c.apiURL, a, c.httpClient)
}

// ExternalRateLimiter exposes the rate limit monitor.
func (c *V4Client) ExternalRateLimiter() *ratelimit.Monitor {
	return c.externalRateLimiter
}

func (c *V4Client) requestGraphQL(ctx context.Context, query string, vars map[string]any, result any) (err error) {
	reqBody, err := json.Marshal(struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}{
		Query:     query,
		Variables: vars,
	})
	if err != nil {
		return err
	}

	// GitHub.com GraphQL endpoint is api.github.com/graphql. GitHub Enterprise is /api/graphql (the
	// REST endpoint is /api/v3, necessitating the "..").
	graphqlEndpoint := "/graphql"
	if !c.githubDotCom {
		graphqlEndpoint = "../graphql"
	}
	req, err := http.NewRequest("POST", graphqlEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	urlCopy := *req.URL

	// Enable Checks API
	// https://developer.github.com/v4/previews/#checks
	req.Header.Add("Accept", "application/vnd.github.antiope-preview+json")
	var respBody struct {
		Data   json.RawMessage `json:"data"`
		Errors graphqlErrors   `json:"errors"`
	}

	cost, err := estimateGraphQLCost(query)
	if err != nil {
		return errors.Wrap(err, "estimating graphql cost")
	}

	if err := c.internalRateLimiter.WaitN(ctx, cost); err != nil {
		return errors.Wrap(err, "rate limit")
	}

	if c.waitForRateLimit {
		_ = c.externalRateLimiter.WaitForRateLimit(ctx, cost)
	}

	_, err = doRequest(ctx, c.log, c.apiURL, c.auth, c.externalRateLimiter, c.httpClient, req, &respBody)

	apiError := &APIError{}
	numRetries := 0

	for c.waitForRateLimit && err != nil && numRetries < c.maxRateLimitRetries &&
		errors.As(err, &apiError) && apiError.Code == http.StatusForbidden {
		// Reset Body/URL to the originals, to ignore changes a previous
		// `doRequest` might have made.
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		// Create a copy of the URL, because this loop might execute
		// multiple times.
		reqURLCopy := urlCopy
		req.URL = &reqURLCopy

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
		if c.externalRateLimiter.WaitForRateLimit(ctx, cost) {
			_, err = doRequest(ctx, c.log, c.apiURL, c.auth, c.externalRateLimiter, c.httpClient, req, &respBody)
			numRetries++
		} else {
			break
		}
	}

	// If the GraphQL response has errors, still attempt to unmarshal the data portion, as some
	// requests may expect errors but have useful responses (e.g., querying a list of repositories,
	// some of which you expect to 404).
	if len(respBody.Errors) > 0 {
		err = respBody.Errors
	}
	if result != nil && respBody.Data != nil {
		if err0 := unmarshal(respBody.Data, result); err0 != nil && err == nil {
			return err0
		}
	}
	return err
}

// estimateGraphQLCost estimates the cost of the query as described here:
// https://developer.github.com/v4/guides/resource-limitations/#calculating-a-rate-limit-score-before-running-the-call
func estimateGraphQLCost(query string) (int, error) {
	doc, err := parser.Parse(parser.ParseParams{
		Source: query,
	})
	if err != nil {
		return 0, errors.Wrap(err, "parsing query")
	}

	var totalCost int
	for _, def := range doc.Definitions {
		cost := calcDefinitionCost(def)
		totalCost += cost
	}

	// As per the calculation spec, cost should be divided by 100
	totalCost /= 100
	if totalCost < 1 {
		return 1, nil
	}
	return totalCost, nil
}

type limitDepth struct {
	// The 'first' or 'last' limit
	limit int
	// The depth at which it was added
	depth int
}

func calcDefinitionCost(def ast.Node) int {
	var cost int
	limitStack := make([]limitDepth, 0)

	v := &visitor.VisitorOptions{
		Enter: func(p visitor.VisitFuncParams) (string, any) {
			switch node := p.Node.(type) {
			case *ast.IntValue:
				// We're looking for a 'first' or 'last' param indicating a limit
				parent, ok := p.Parent.(*ast.Argument)
				if !ok {
					return visitor.ActionNoChange, nil
				}
				if parent.Name == nil {
					return visitor.ActionNoChange, nil
				}
				if parent.Name.Value != "first" && parent.Name.Value != "last" {
					return visitor.ActionNoChange, nil
				}

				// Prune anything above our current depth as we may have started walking
				// back down the tree
				currentDepth := len(p.Ancestors)
				limitStack = filterInPlace(limitStack, currentDepth)

				limit, err := strconv.Atoi(node.Value)
				if err != nil {
					return "", errors.Wrap(err, "parsing limit")
				}
				limitStack = append(limitStack, limitDepth{limit: limit, depth: currentDepth})
				// The first item in the tree is always worth 1
				if len(limitStack) == 1 {
					cost++
					return visitor.ActionNoChange, nil
				}
				// The cost of the current item is calculated using the limits of
				// its children
				children := limitStack[:len(limitStack)-1]
				product := 1
				// Multiply them all together
				for _, n := range children {
					product = n.limit * product
				}
				cost += product
			}
			return visitor.ActionNoChange, nil
		},
	}

	_ = visitor.Visit(def, v, nil)

	return cost
}

func filterInPlace(limitStack []limitDepth, depth int) []limitDepth {
	n := 0
	for _, x := range limitStack {
		if depth > x.depth {
			limitStack[n] = x
			n++
		}
	}
	limitStack = limitStack[:n]
	return limitStack
}

type graphqlError struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Path      []any  `json:"path"`
	Locations []struct {
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"locations,omitempty"`
}

// graphqlErrors describes the errors in a GraphQL response. It contains at least 1 element when returned by
// requestGraphQL. See https://graphql.github.io/graphql-spec/June2018/#sec-Errors.
type graphqlErrors []graphqlError

const graphqlErrTypeNotFound = "NOT_FOUND"

func (e graphqlErrors) Error() string {
	return fmt.Sprintf("error in GraphQL response: %s", e[0].Message)
}

// unmarshal wraps json.Unmarshal, but includes extra context in the case of
// json.UnmarshalTypeError
func unmarshal(data []byte, v any) error {
	err := json.Unmarshal(data, v)
	var e *json.UnmarshalTypeError
	if errors.As(err, &e) && e.Offset >= 0 {
		a := e.Offset - 100
		b := e.Offset + 100
		if a < 0 {
			a = 0
		}
		if b > int64(len(data)) {
			b = int64(len(data))
		}
		if e.Offset >= int64(len(data)) {
			return errors.Wrapf(err, "graphql: cannot unmarshal at offset %d: before %q", e.Offset, string(data[a:e.Offset]))
		}
		return errors.Wrapf(err, "graphql: cannot unmarshal at offset %d: before %q; after %q", e.Offset, string(data[a:e.Offset]), string(data[e.Offset:b]))
	}
	return err
}

// determineGitHubVersion returns a *semver.Version for the targetted GitHub instance by this client. When an
// error occurs, we print a warning to the logs but don't fail and return the allMatchingSemver.
func (c *V4Client) determineGitHubVersion(ctx context.Context) *semver.Version {
	urlStr := normalizeURL(c.apiURL.String())
	globalVersionCache.mu.Lock()
	defer globalVersionCache.mu.Unlock()

	if globalVersionCache.lastReset.IsZero() || time.Now().After(globalVersionCache.lastReset.Add(versionCacheResetTime)) {
		// Clear cache and set last expiry to now.
		globalVersionCache.lastReset = time.Now()
		globalVersionCache.versions = make(map[string]*semver.Version)
	}
	if version, ok := globalVersionCache.versions[urlStr]; ok {
		return version
	}
	version := c.fetchGitHubVersion(ctx)
	globalVersionCache.versions[urlStr] = version
	return version
}

// fetchGitHubVersion will attempt to identify the GitHub Enterprise Server's version.  If the
// method is called by a client configured to use github.com, it will return allMatchingSemver.
//
// Additionally if it fails to parse the version. or the API request fails with an error, it
// defaults to returning allMatchingSemver as well.
func (c *V4Client) fetchGitHubVersion(ctx context.Context) (version *semver.Version) {
	if c.githubDotCom {
		return allMatchingSemver
	}

	// Initiate a v3Client since this requires a V3 API request.
	logger := c.log.Scoped("fetchGitHubVersion")
	v3Client := newV3Client(logger, c.urn, c.apiURL, c.auth, "rest", c.httpClient)
	v, err := v3Client.GetVersion(ctx)
	if err != nil {
		c.log.Warn("Failed to fetch GitHub enterprise version",
			log.String("method", "fetchGitHubVersion"),
			log.String("apiURL", c.apiURL.String()),
			log.Error(err),
		)
		return allMatchingSemver
	}

	version, err = semver.NewVersion(v)
	if err != nil {
		return allMatchingSemver
	}

	return version
}

func (c *V4Client) GetAuthenticatedUser(ctx context.Context) (*Actor, error) {
	var result struct {
		Viewer Actor `json:"viewer"`
	}
	err := c.requestGraphQL(ctx, `query GetAuthenticatedUser {
    viewer {
        login
        avatarUrl
        url
    }
}`, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result.Viewer, nil
}

// A Cursor is a pagination cursor returned by the API in fields like endCursor.
type Cursor string

// SearchReposParams are the inputs to the SearchRepos method.
type SearchReposParams struct {
	// Query is the GitHub search query. See https://docs.github.com/en/github/searching-for-information-on-github/searching-on-github/searching-for-repositories
	Query string
	// After is the cursor to paginate from.
	After Cursor
	// First is the page size. Default to 100 if left zero.
	First int
}

// SearchReposResults is the result type of SearchRepos.
type SearchReposResults struct {
	// The repos that matched the Query in SearchReposParams.
	Repos []Repository
	// The total result count of the Query in SearchReposParams.
	// Since GitHub's search API limits result sets to 1000, we can
	// use this to determine if we need to refine the search query to
	// not miss results.
	TotalCount int
	// The cursor pointing to the next page of results.
	EndCursor Cursor
}

// SearchRepos searches for repositories matching the given search query (https://github.com/search/advanced), using
// the given pagination parameters provided by the caller.
func (c *V4Client) SearchRepos(ctx context.Context, p SearchReposParams) (SearchReposResults, error) {
	if p.First == 0 {
		p.First = 100
	}

	vars := map[string]any{
		"query": p.Query,
		"type":  "REPOSITORY",
		"first": p.First,
	}

	if p.After != "" {
		vars["after"] = p.After
	}

	query := c.buildSearchReposQuery(ctx)

	var resp struct {
		Search struct {
			RepositoryCount int
			PageInfo        struct {
				HasNextPage bool
				EndCursor   Cursor
			}
			Nodes []Repository
		}
	}

	err := c.requestGraphQL(ctx, query, vars, &resp)
	if err != nil {
		return SearchReposResults{}, err
	}

	results := SearchReposResults{
		Repos:      resp.Search.Nodes,
		TotalCount: resp.Search.RepositoryCount,
	}

	if resp.Search.PageInfo.HasNextPage {
		results.EndCursor = resp.Search.PageInfo.EndCursor
	}

	return results, nil
}

func (c *V4Client) buildSearchReposQuery(ctx context.Context) string {
	var b strings.Builder
	b.WriteString(c.repositoryFieldsGraphQLFragment(ctx))
	b.WriteString(`
query($query: String!, $type: SearchType!, $after: String, $first: Int!) {
	search(query: $query, type: $type, after: $after, first: $first) {
		repositoryCount
		pageInfo { hasNextPage,  endCursor }
		nodes { ... on Repository { ...RepositoryFields } }
	}
}`)
	return b.String()
}

// GetReposByNameWithOwner fetches the specified repositories (namesWithOwners)
// from the GitHub GraphQL API and returns a slice of repositories.
// If a repository is not found, it will return an error.
//
// The maximum number of repositories to be fetched is 30. If more
// namesWithOwners are given, the method returns an error. 30 is not a official
// limit of the API, but based on the observation that the GitHub GraphQL does
// not return results when more than 37 aliases are specified in a query. 30 is
// the conservative step back from 37.
//
// This method does not cache.
func (c *V4Client) GetReposByNameWithOwner(ctx context.Context, namesWithOwners ...string) ([]*Repository, error) {
	if len(namesWithOwners) > 30 {
		return nil, ErrBatchTooLarge
	}

	query, err := c.buildGetReposBatchQuery(ctx, namesWithOwners)
	if err != nil {
		return nil, err
	}

	var result map[string]*Repository
	err = c.requestGraphQL(ctx, query, map[string]any{}, &result)
	if err != nil {
		var e graphqlErrors
		if errors.As(err, &e) {
			for _, err2 := range e {
				if err2.Type == graphqlErrTypeNotFound {
					c.log.Warn("GitHub repository not found", graphQLErrorField(err2))
					continue
				}
				return nil, err
			}
			// The lack of an error return here is intentional. Do not use this
			// as a basis for implementing other functions that need normal
			// error handling!
		} else {
			return nil, err
		}
	}

	repos := make([]*Repository, 0, len(result))
	for _, r := range result {
		if r != nil {
			repos = append(repos, r)
		}
	}
	return repos, nil
}

func (c *V4Client) buildGetReposBatchQuery(ctx context.Context, namesWithOwners []string) (string, error) {
	var b strings.Builder
	b.WriteString(c.repositoryFieldsGraphQLFragment(ctx))
	b.WriteString("query {\n")

	for i, pair := range namesWithOwners {
		owner, name, err := SplitRepositoryNameWithOwner(pair)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "repo%d: repository(owner: %q, name: %q) { ", i, owner, name)
		b.WriteString("... on Repository { ...RepositoryFields parent { nameWithOwner, isFork } } }\n")
	}

	b.WriteString("}")

	return b.String(), nil
}

// repositoryFieldsGraphQLFragment returns a GraphQL fragment that contains the fields needed to populate the
// Repository struct.
func (c *V4Client) repositoryFieldsGraphQLFragment(ctx context.Context) string {
	if c.githubDotCom {
		return `
fragment RepositoryFields on Repository {
	id
	databaseId
	nameWithOwner
	description
	url
	isPrivate
	isFork
	isArchived
	isLocked
	isDisabled
	viewerPermission
	stargazerCount
	forkCount
	diskUsage
	repositoryTopics(first:100) {
		nodes {
			topic {
				name
			}
		}
	}
}
	`
	}
	conditionalGHEFields := []string{}
	version := c.determineGitHubVersion(ctx)

	if ghe300PlusOrDotComSemver.Check(version) {
		conditionalGHEFields = append(conditionalGHEFields, "stargazerCount")
	}

	if ghe330PlusOrDotComSemver.Check(version) {
		conditionalGHEFields = append(conditionalGHEFields, "visibility")
	}

	// Some fields are not yet available on GitHub Enterprise yet
	// or are available but too new to expect our customers to have updated:
	// - viewerPermission
	return fmt.Sprintf(`
fragment RepositoryFields on Repository {
	id
	databaseId
	nameWithOwner
	description
	url
	isPrivate
	isFork
	isArchived
	isLocked
	isDisabled
	forkCount
	diskUsage
	repositoryTopics(first:100) {
		nodes {
			topic {
				name
			}
		}
	}
	%s
}
	`, strings.Join(conditionalGHEFields, "\n	"))
}

type RecentCommittersParams struct {
	// Repository name
	Name string
	// Repository owner
	Owner string
	// After is the cursor to paginate from.
	After Cursor
	// First is the page size. Default to 100 if left zero.
	First int
}

type RecentCommittersResults struct {
	Nodes []struct {
		Authors struct {
			Nodes []struct {
				Date  string
				Email string
				Name  string
				User  struct {
					Login string
				}
				AvatarURL string
			}
		}
	}
	PageInfo struct {
		HasNextPage bool
		EndCursor   Cursor
	}
}

// Lists recent committers for a repository.
func (c *V4Client) RecentCommitters(ctx context.Context, params *RecentCommittersParams) (*RecentCommittersResults, error) {
	if params.First == 0 {
		params.First = 100
	}

	query := `
	  query($name: String!, $owner: String!, $after: String, $first: Int!) {
		repository(name: $name, owner: $owner) {
		  defaultBranchRef {
			target {
			  ... on Commit {
				history(after: $after, first: $first) {
				  pageInfo { hasNextPage, endCursor }
				  nodes {
					authors(first: 50) {
					  nodes {
						email
						name
						user {
							login
						}
						avatarUrl
						date
					  }
					}
				  }
				}
			  }
			}
		  }
		}
	  }
	`

	vars := map[string]any{
		"name":  params.Name,
		"owner": params.Owner,
		"first": params.First,
	}
	if params.After != "" {
		vars["after"] = params.After
	}

	var result struct {
		Repository struct {
			DefaultBranchRef struct {
				Target struct {
					History RecentCommittersResults
				}
			}
		}
	}
	err := c.requestGraphQL(ctx, query, vars, &result)
	if err != nil {
		var e graphqlErrors
		if errors.As(err, &e) {
			for _, err2 := range e {
				if err2.Type == graphqlErrTypeNotFound {
					c.log.Warn("RecentCommitters: GitHub repository not found")
					continue
				}
				return nil, err
			}
		}
		return nil, err
	}
	return &result.Repository.DefaultBranchRef.Target.History, nil
}

type Release struct {
	TagName      string
	IsDraft      bool
	IsPrerelease bool
}

type ReleasesResult struct {
	Nodes    []Release
	PageInfo struct {
		HasNextPage bool
		EndCursor   Cursor
	}
}

type ReleasesParams struct {
	// Repository name
	Name string
	// Repository owner
	Owner string
	// After is the cursor to paginate from.
	After Cursor
	// First is the page size. Default to 100 if left zero.
	First int
}

// Releases returns the releases for the given repository, ordered from newest
// to oldest. This excludes pre-release and draft releases.
func (c *V4Client) Releases(ctx context.Context, params *ReleasesParams) (*ReleasesResult, error) {
	const query = `
		query($owner: String!, $name: String!, $first: Int!, $after: String, $order: ReleaseOrder!) {
			repository(owner: $owner, name: $name) {
				releases(first: $first, after: $after, orderBy: $order) {
					nodes {
						tagName
						isDraft
						isPrerelease
					}
					pageInfo {
						hasNextPage
						endCursor
					}
				}
			}
		}
	`

	if params.First == 0 {
		params.First = 100
	}

	vars := map[string]any{
		"name":  params.Name,
		"owner": params.Owner,
		"first": params.First,
		"order": map[string]any{
			"field":     "CREATED_AT",
			"direction": "DESC",
		},
	}
	if params.After != "" {
		vars["after"] = params.After
	}

	var result struct {
		Repository struct {
			Releases ReleasesResult
		}
	}
	err := c.requestGraphQL(ctx, query, vars, &result)
	if err != nil {
		var e graphqlErrors
		if errors.As(err, &e) {
			for _, err2 := range e {
				if err2.Type == graphqlErrTypeNotFound {
					c.log.Warn("GitHub repository not found", graphQLErrorField(err2))
					continue
				}
				return nil, err
			}
		}
		return nil, err
	}

	return &result.Repository.Releases, nil
}

func graphQLErrorField(err graphqlError) log.Field {
	return log.Object("err",
		log.String("message", err.Message),
		log.String("type", err.Type),
		log.String("path", fmt.Sprintf("%+v", err.Path)),
		log.String("locations", fmt.Sprintf("%+v", err.Locations)))
}
