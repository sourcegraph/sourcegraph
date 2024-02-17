//nolint:bodyclose // Body is closed in Client.Do, but the response is still returned to provide access to the headers
package bitbucketserver

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/RoaringBitmap/roaring"
	"github.com/gomodule/oauth1/oauth"
	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/segmentio/fasthash/fnv1"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// The metric generated here will be named as "src_bitbucket_requests_total".
var requestCounter = metrics.NewRequestMeter("bitbucket", "Total number of requests sent to the Bitbucket API.")

// Client access a Bitbucket Server via the REST API.
type Client struct {
	// URL is the base URL of Bitbucket Server.
	URL *url.URL

	// Auth is the authentication method used when accessing the server.
	// Supported types are:
	// * auth.OAuthBearerToken for a personal access token; see also
	//   https://bitbucket.example.com/plugins/servlet/access-tokens/manage
	// * auth.BasicAuth for a username and password combination. Typically
	//   these are only used when the server doesn't support personal access
	//   tokens (such as Bitbucket Server 5.4 and older).
	// * SudoableClient for an OAuth 1 client used to authenticate requests.
	//   This is generally set using SetOAuth.
	Auth auth.Authenticator

	// HTTP Client used to communicate with the API
	httpClient httpcli.Doer

	// RateLimit is the self-imposed rate limiter (since Bitbucket does not have a
	// concept of rate limiting in HTTP response headers). Default limits are defined
	// in extsvc.GetLimitFromConfig
	rateLimit *ratelimit.InstrumentedLimiter
}

// NewClient returns an authenticated Bitbucket Server API client with
// the provided configuration.
func NewClient(urn string, config *schema.BitbucketServerConnection, cf *httpcli.Factory) (*Client, error) {
	client, err := newClient(urn, config, cf)
	if err != nil {
		return nil, err
	}

	if config.Authorization == nil {
		if config.Token != "" {
			client.Auth = &auth.OAuthBearerToken{Token: config.Token}
		} else {
			client.Auth = &auth.BasicAuth{
				Username: config.Username,
				Password: config.Password,
			}
		}
	} else {
		err := client.SetOAuth(
			config.Authorization.Oauth.ConsumerKey,
			config.Authorization.Oauth.SigningKey,
		)
		if err != nil {
			return nil, errors.Wrap(err, "authorization.oauth.signingKey")
		}
	}

	return client, nil
}

func newClient(urn string, config *schema.BitbucketServerConnection, cf *httpcli.Factory) (*Client, error) {
	u, err := url.Parse(config.Url)
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
	httpClient = requestCounter.Doer(httpClient, categorize)

	return &Client{
		httpClient: httpClient,
		URL:        u,
		// Default limits are defined in extsvc.GetLimitFromConfig
		rateLimit: ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("BitbucketServerClient"), urn)),
	}, nil
}

// WithAuthenticator returns a new Client that uses the same configuration,
// HTTPClient, and RateLimiter as the current Client, except authenticated user
// with the given authenticator instance.
func (c *Client) WithAuthenticator(a auth.Authenticator) *Client {
	return &Client{
		httpClient: c.httpClient,
		URL:        c.URL,
		rateLimit:  c.rateLimit,
		Auth:       a,
	}
}

// SetOAuth enables OAuth authentication in a Client, using the given consumer
// key to identify with the Bitbucket Server API and the request signing RSA key
// to authenticate requests. It parses the given Base64 encoded PEM encoded private key,
// returning an error in case of failure.
//
// When using OAuth authentication, it's possible to impersonate any Bitbucket
// Server API user by passing a ?user_id=$username query parameter. This requires
// the Application Link in the Bitbucket Server API to be configured with 2 legged
// OAuth and for it to allow user impersonation.
func (c *Client) SetOAuth(consumerKey, signingKey string) error {
	pemKey, err := base64.StdEncoding.DecodeString(signingKey)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(pemKey)
	if block == nil {
		return errors.New("failed to parse PEM block containing the signing key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}

	c.Auth = &SudoableOAuthClient{
		Client: auth.OAuthClient{
			Client: &oauth.Client{
				Credentials:     oauth.Credentials{Token: consumerKey},
				PrivateKey:      key,
				SignatureMethod: oauth.RSASHA1,
			},
		},
	}

	return nil
}

// Sudo returns a copy of the Client authenticated as the Bitbucket Server user with
// the given username. This only works when using OAuth authentication and if the
// Application Link in Bitbucket Server is configured to allow user impersonation,
// returning an error otherwise.
func (c *Client) Sudo(username string) (*Client, error) {
	a, ok := c.Auth.(*SudoableOAuthClient)
	if !ok || a == nil {
		return nil, errors.New("bitbucketserver.Client: OAuth not configured")
	}

	authCopy := *a
	authCopy.Username = username

	sudo := *c
	sudo.Auth = &authCopy
	return &sudo, nil
}

// Username returns the username that will be used when communicating with
// Bitbucket Server, if the authentication method includes a username.
func (c *Client) Username() (string, error) {
	switch a := c.Auth.(type) {
	case *SudoableOAuthClient:
		return a.Username, nil
	case *auth.BasicAuth:
		return a.Username, nil
	default:
		return "", errors.New("bitbucketserver.Client: authentication method does not include a username")
	}
}

// UserFilters is a list of UserFilter that is ANDed together.
type UserFilters []UserFilter

// EncodeTo encodes the UserFilter to the given url.Values.
func (fs UserFilters) EncodeTo(qry url.Values) {
	var perm int
	for _, f := range fs {
		if f.Permission != (PermissionFilter{}) {
			perm++
			f.Permission.index = perm
		}
		f.EncodeTo(qry)
	}
}

// UserFilter defines a sum type of filters to be used when listing users.
type UserFilter struct {
	// Filter filters the returned users to those whose username,
	// name or email address contain this value.
	// The API doesn't support exact matches.
	Filter string
	// Group filters the returned users to those who are in the give group.
	Group string
	// Permission filters the returned users to those having the given
	// permissions.
	Permission PermissionFilter
}

// EncodeTo encodes the UserFilter to the given url.Values.
func (f UserFilter) EncodeTo(qry url.Values) {
	if f.Filter != "" {
		qry.Set("filter", f.Filter)
	}

	if f.Group != "" {
		qry.Set("group", f.Group)
	}

	if f.Permission != (PermissionFilter{}) {
		f.Permission.EncodeTo(qry)
	}
}

// A PermissionFilter is a filter used to list users that have specific
// permissions.
type PermissionFilter struct {
	Root           Perm
	ProjectID      string
	ProjectKey     string
	RepositoryID   string
	RepositorySlug string

	index int
}

// EncodeTo encodes the PermissionFilter to the given url.Values.
func (p PermissionFilter) EncodeTo(qry url.Values) {
	q := "permission"

	if p.index != 0 {
		q += "." + strconv.Itoa(p.index)
	}

	qry.Set(q, string(p.Root))

	if p.ProjectID != "" {
		qry.Set(q+".projectId", p.ProjectID)
	}

	if p.ProjectKey != "" {
		qry.Set(q+".projectKey", p.ProjectKey)
	}

	if p.RepositoryID != "" {
		qry.Set(q+".repositoryId", p.RepositoryID)
	}

	if p.RepositorySlug != "" {
		qry.Set(q+".repositorySlug", p.RepositorySlug)
	}
}

// ErrUserFiltersLimit is returned by Client.Users when the UserFiltersLimit is exceeded.
var ErrUserFiltersLimit = errors.Errorf("maximum of %d user filters exceeded", userFiltersLimit)

// userFiltersLimit defines the maximum number of UserFilters that can
// be passed to a single Client.Users call.
const userFiltersLimit = 50

// Users retrieves a page of users, optionally run through provided filters.
func (c *Client) Users(ctx context.Context, pageToken *PageToken, fs ...UserFilter) ([]*User, *PageToken, error) {
	if len(fs) > userFiltersLimit {
		return nil, nil, ErrUserFiltersLimit
	}

	qry := make(url.Values)
	UserFilters(fs).EncodeTo(qry)

	var users []*User
	next, err := c.page(ctx, "rest/api/1.0/users", qry, pageToken, &users)
	return users, next, err
}

// UserPermissions retrieves the global permissions assigned to the user with the given
// username. Used to validate that the client is authenticated as an admin.
func (c *Client) UserPermissions(ctx context.Context, username string) (perms []Perm, _ error) {
	qry := url.Values{"filter": {username}}

	type permission struct {
		User       *User `json:"user"`
		Permission Perm  `json:"permission"`
	}

	var ps []permission
	_, err := c.send(ctx, "GET", "rest/api/1.0/admin/permissions/users", qry, nil, &struct {
		Values []permission `json:"values"`
	}{
		Values: ps,
	})
	if err != nil {
		return nil, err
	}

	for _, p := range ps {
		if p.User.Name == username {
			perms = append(perms, p.Permission)
		}
	}

	return perms, nil
}

// CreateUser creates the given User returning an error in case of failure.
func (c *Client) CreateUser(ctx context.Context, u *User) error {
	qry := url.Values{
		"name":              {u.Name},
		"password":          {u.Password},
		"displayName":       {u.DisplayName},
		"emailAddress":      {u.EmailAddress},
		"addToDefaultGroup": {"true"},
	}

	_, err := c.send(ctx, "POST", "rest/api/1.0/admin/users", qry, nil, nil)
	return err
}

// LoadUser loads the given User returning an error in case of failure.
func (c *Client) LoadUser(ctx context.Context, u *User) error {
	_, err := c.send(ctx, "GET", "rest/api/1.0/users/"+u.Slug, nil, nil, u)
	return err
}

// LoadGroup loads the given Group returning an error in case of failure.
func (c *Client) LoadGroup(ctx context.Context, g *Group) error {
	qry := url.Values{"filter": {g.Name}}
	var groups struct {
		Values []*Group `json:"values"`
	}

	_, err := c.send(ctx, "GET", "rest/api/1.0/admin/groups", qry, nil, &groups)
	if err != nil {
		return err
	}

	if len(groups.Values) != 1 {
		return errors.New("group not found")
	}

	*g = *groups.Values[0]

	return nil
}

// CreateGroup creates the given Group returning an error in case of failure.
func (c *Client) CreateGroup(ctx context.Context, g *Group) error {
	qry := url.Values{"name": {g.Name}}
	_, err := c.send(ctx, "POST", "rest/api/1.0/admin/groups", qry, g, g)
	return err
}

// CreateGroupMembership creates the given Group's membership returning an error in case of failure.
func (c *Client) CreateGroupMembership(ctx context.Context, g *Group) error {
	type membership struct {
		Group string   `json:"group"`
		Users []string `json:"users"`
	}
	m := &membership{Group: g.Name, Users: g.Users}
	_, err := c.send(ctx, "POST", "rest/api/1.0/admin/groups/add-users", nil, m, nil)
	return err
}

// CreateUserRepoPermission creates the given permission returning an error in case of failure.
func (c *Client) CreateUserRepoPermission(ctx context.Context, p *UserRepoPermission) error {
	path := "rest/api/1.0/projects/" + p.Repo.Project.Key + "/repos/" + p.Repo.Slug + "/permissions/users"
	return c.createPermission(ctx, path, p.User.Name, p.Perm)
}

// CreateUserProjectPermission creates the given permission returning an error in case of failure.
func (c *Client) CreateUserProjectPermission(ctx context.Context, p *UserProjectPermission) error {
	path := "rest/api/1.0/projects/" + p.Project.Key + "/permissions/users"
	return c.createPermission(ctx, path, p.User.Name, p.Perm)
}

// CreateGroupProjectPermission creates the given permission returning an error in case of failure.
func (c *Client) CreateGroupProjectPermission(ctx context.Context, p *GroupProjectPermission) error {
	path := "rest/api/1.0/projects/" + p.Project.Key + "/permissions/groups"
	return c.createPermission(ctx, path, p.Group.Name, p.Perm)
}

// CreateGroupRepoPermission creates the given permission returning an error in case of failure.
func (c *Client) CreateGroupRepoPermission(ctx context.Context, p *GroupRepoPermission) error {
	path := "rest/api/1.0/projects/" + p.Repo.Project.Key + "/repos/" + p.Repo.Slug + "/permissions/groups"
	return c.createPermission(ctx, path, p.Group.Name, p.Perm)
}

func (c *Client) createPermission(ctx context.Context, path, name string, p Perm) error {
	qry := url.Values{
		"name":       {name},
		"permission": {string(p)},
	}
	_, err := c.send(ctx, "PUT", path, qry, nil, nil)
	return err
}

// CreateRepo creates the given Repo returning an error in case of failure.
func (c *Client) CreateRepo(ctx context.Context, r *Repo) error {
	path := "rest/api/1.0/projects/" + r.Project.Key + "/repos"
	_, err := c.send(ctx, "POST", path, nil, r, &struct {
		Values []*Repo `json:"values"`
	}{
		Values: []*Repo{r},
	})
	return err
}

// LoadProject loads the given Project returning an error in case of failure.
func (c *Client) LoadProject(ctx context.Context, p *Project) error {
	_, err := c.send(ctx, "GET", "rest/api/1.0/projects/"+p.Key, nil, nil, p)
	return err
}

// CreateProject creates the given Project returning an error in case of failure.
func (c *Client) CreateProject(ctx context.Context, p *Project) error {
	_, err := c.send(ctx, "POST", "rest/api/1.0/projects", nil, p, p)
	return err
}

// ErrPullRequestNotFound is returned by LoadPullRequest when the pull request has
// been deleted on upstream, or never existed. It will NOT be thrown, if it can't
// be determined whether the pull request exists, because the credential used
// cannot view the repository.
var ErrPullRequestNotFound = errors.New("pull request not found")

// LoadPullRequest loads the given PullRequest returning an error in case of failure.
func (c *Client) LoadPullRequest(ctx context.Context, pr *PullRequest) error {
	if pr.ToRef.Repository.Slug == "" {
		return errors.New("repository slug empty")
	}
	if pr.ToRef.Repository.Project.Key == "" {
		return errors.New("project key empty")
	}

	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%d",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
		pr.ID,
	)
	_, err := c.send(ctx, "GET", path, nil, nil, pr)
	if err != nil {
		var e *httpError
		if errors.As(err, &e) && e.NoSuchPullRequestException() {
			return ErrPullRequestNotFound
		}

		return err
	}
	return nil
}

type UpdatePullRequestInput struct {
	PullRequestID string `json:"-"`
	Version       int    `json:"version"`

	Title       string     `json:"title"`
	Description string     `json:"description"`
	ToRef       Ref        `json:"toRef"`
	Reviewers   []Reviewer `json:"reviewers"`
}

func (c *Client) UpdatePullRequest(ctx context.Context, in *UpdatePullRequestInput) (*PullRequest, error) {
	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%s",
		in.ToRef.Repository.Project.Key,
		in.ToRef.Repository.Slug,
		in.PullRequestID,
	)

	pr := &PullRequest{}
	_, err := c.send(ctx, "PUT", path, nil, in, pr)
	return pr, err
}

// ErrAlreadyExists is returned by Client.CreatePullRequest when a Pull Request
// for the given FromRef and ToRef already exists.
type ErrAlreadyExists struct {
	Existing *PullRequest
}

func (e ErrAlreadyExists) Error() string {
	return "A pull request with the given to and from refs already exists"
}

// CreatePullRequest creates the given PullRequest returning an error in case of failure.
func (c *Client) CreatePullRequest(ctx context.Context, pr *PullRequest) error {
	for _, namedRef := range [...]struct {
		name string
		ref  Ref
	}{
		{"ToRef", pr.ToRef},
		{"FromRef", pr.FromRef},
	} {
		if namedRef.ref.ID == "" {
			return errors.Errorf("%s id empty", namedRef.name)
		}
		if namedRef.ref.Repository.Slug == "" {
			return errors.Errorf("%s repository slug empty", namedRef.name)
		}
		if namedRef.ref.Repository.Project.Key == "" {
			return errors.Errorf("%s project key empty", namedRef.name)
		}
	}

	// Minimal version of Reviewer, to reduce payload size sent.
	type reviewer struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}

	type requestBody struct {
		Title       string     `json:"title"`
		Description string     `json:"description"`
		State       string     `json:"state"`
		Open        bool       `json:"open"`
		Closed      bool       `json:"closed"`
		FromRef     Ref        `json:"fromRef"`
		ToRef       Ref        `json:"toRef"`
		Locked      bool       `json:"locked"`
		Reviewers   []reviewer `json:"reviewers"`
	}

	defaultReviewers, err := c.FetchDefaultReviewers(ctx, pr)
	if err != nil {
		log15.Error("Failed to fetch default reviewers", "err", err)
		// TODO: Once validated this works alright, we want to properly throw
		// an error here. For now, we log an error and continue.
		// return errors.Wrap(err, "fetching default reviewers")
	}

	reviewers := make([]reviewer, 0, len(defaultReviewers))
	for _, r := range defaultReviewers {
		reviewers = append(reviewers, reviewer{User: struct {
			Name string `json:"name"`
		}{Name: r}})
	}

	// Bitbucket Server doesn't support GFM taskitems. But since we might add
	// those to a PR description for certain batch changes, we have to
	// "downgrade" here and for now, removing taskitems is enough.
	description := strings.ReplaceAll(pr.Description, "- [ ] ", "- ")

	payload := requestBody{
		Title:       pr.Title,
		Description: description,
		State:       "OPEN",
		Open:        true,
		Closed:      false,
		FromRef:     pr.FromRef,
		ToRef:       pr.ToRef,
		Locked:      false,
		Reviewers:   reviewers,
	}

	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
	)

	resp, err := c.send(ctx, "POST", path, nil, payload, pr)

	if err != nil {
		var code int
		if resp != nil {
			code = resp.StatusCode
		}
		if IsDuplicatePullRequest(err) {
			pr, extractErr := ExtractExistingPullRequest(err)
			if extractErr != nil {
				log15.Error("Extracting existing PR", "err", extractErr)
			}
			return &ErrAlreadyExists{
				Existing: pr,
			}
		}
		return errcode.MaybeMakeNonRetryable(code, err)
	}
	return nil
}

// FetchDefaultReviewers loads the suggested default reviewers for the given PR.
func (c *Client) FetchDefaultReviewers(ctx context.Context, pr *PullRequest) ([]string, error) {
	// Validate input.
	for _, namedRef := range [...]struct {
		name string
		ref  Ref
	}{
		{"ToRef", pr.ToRef},
		{"FromRef", pr.FromRef},
	} {
		if namedRef.ref.ID == "" {
			return nil, errors.Errorf("%s id empty", namedRef.name)
		}
		if namedRef.ref.Repository.ID == 0 {
			return nil, errors.Errorf("%s repository id empty", namedRef.name)
		}
		if namedRef.ref.Repository.Slug == "" {
			return nil, errors.Errorf("%s repository slug empty", namedRef.name)
		}
		if namedRef.ref.Repository.Project.Key == "" {
			return nil, errors.Errorf("%s project key empty", namedRef.name)
		}
	}

	path := fmt.Sprintf(
		"rest/default-reviewers/1.0/projects/%s/repos/%s/reviewers",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
	)
	queryParams := url.Values{
		"sourceRepoId": []string{strconv.Itoa(pr.FromRef.Repository.ID)},
		"targetRepoId": []string{strconv.Itoa(pr.ToRef.Repository.ID)},
		"sourceRefId":  []string{pr.FromRef.ID},
		"targetRefId":  []string{pr.ToRef.ID},
	}

	var resp []User
	_, err := c.send(ctx, "GET", path, queryParams, nil, &resp)
	if err != nil {
		return nil, err
	}

	reviewerNames := make([]string, 0, len(resp))
	for _, r := range resp {
		reviewerNames = append(reviewerNames, r.Name)
	}
	return reviewerNames, nil
}

// DeclinePullRequest declines and closes the given PullRequest, returning an error in case of failure.
func (c *Client) DeclinePullRequest(ctx context.Context, pr *PullRequest) error {
	if pr.ToRef.Repository.Slug == "" {
		return errors.New("repository slug empty")
	}

	if pr.ToRef.Repository.Project.Key == "" {
		return errors.New("project key empty")
	}

	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/decline",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
		pr.ID,
	)

	qry := url.Values{"version": {strconv.Itoa(pr.Version)}}

	_, err := c.send(ctx, "POST", path, qry, nil, pr)
	return err
}

// ReopenPullRequest reopens a previously declined & closed PullRequest,
// returning an error in case of failure.
func (c *Client) ReopenPullRequest(ctx context.Context, pr *PullRequest) error {
	if pr.ToRef.Repository.Slug == "" {
		return errors.New("repository slug empty")
	}

	if pr.ToRef.Repository.Project.Key == "" {
		return errors.New("project key empty")
	}

	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/reopen",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
		pr.ID,
	)

	qry := url.Values{"version": {strconv.Itoa(pr.Version)}}

	_, err := c.send(ctx, "POST", path, qry, nil, pr)
	return err
}

type DeleteBranchInput struct {
	// Don't actually delete the ref name, just do a dry run
	DryRun bool `json:"dryRun,omitempty"`
	// Commit ID that the provided ref name is expected to point to. Should the ref point
	// to a different commit ID, a 400 response will be returned with appropriate error
	// details.
	EndPoint *string `json:"endPoint,omitempty"`
	// Name of the ref to be deleted
	Name string `json:"name,omitempty"`
}

// DeleteBranch deletes a branch on the given repo.
func (c *Client) DeleteBranch(ctx context.Context, projectKey, repoSlug string, input DeleteBranchInput) error {
	path := fmt.Sprintf(
		"rest/branch-utils/latest/projects/%s/repos/%s/branches",
		projectKey,
		repoSlug,
	)

	resp, err := c.send(ctx, "DELETE", path, nil, input, nil)
	if resp != nil && resp.StatusCode != http.StatusNoContent {
		return errors.Newf("unexpected status code: %d", resp.StatusCode)
	}

	return err
}

// LoadPullRequestActivities loads the given PullRequest's timeline of activities,
// returning an error in case of failure.
func (c *Client) LoadPullRequestActivities(ctx context.Context, pr *PullRequest) (err error) {
	if pr.ToRef.Repository.Slug == "" {
		return errors.New("repository slug empty")
	}

	if pr.ToRef.Repository.Project.Key == "" {
		return errors.New("project key empty")
	}

	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/activities",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
		pr.ID,
	)

	t := &PageToken{Limit: 1000}

	var activities []*Activity
	for t.HasMore() {
		var page []*Activity
		if t, err = c.page(ctx, path, nil, t, &page); err != nil {
			return err
		}
		activities = append(activities, page...)
	}

	pr.Activities = activities
	return nil
}

func (c *Client) LoadPullRequestCommits(ctx context.Context, pr *PullRequest) (err error) {
	if pr.ToRef.Repository.Slug == "" {
		return errors.New("repository slug empty")
	}

	if pr.ToRef.Repository.Project.Key == "" {
		return errors.New("project key empty")
	}

	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/commits",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
		pr.ID,
	)

	t := &PageToken{Limit: 1000}

	var commits []*Commit
	for t.HasMore() {
		var page []*Commit
		if t, err = c.page(ctx, path, nil, t, &page); err != nil {
			return err
		}
		commits = append(commits, page...)
	}

	pr.Commits = commits
	return nil
}

func (c *Client) LoadPullRequestBuildStatuses(ctx context.Context, pr *PullRequest) (err error) {
	if len(pr.Commits) == 0 {
		return nil
	}

	var latestCommit Commit
	for _, c := range pr.Commits {
		if latestCommit.CommitterTimestamp < c.CommitterTimestamp {
			latestCommit = *c
		}
	}

	path := fmt.Sprintf("rest/build-status/1.0/commits/%s", latestCommit.ID)

	t := &PageToken{Limit: 1000}

	var statuses []*CommitStatus
	for t.HasMore() {
		var page []*BuildStatus
		if t, err = c.page(ctx, path, nil, t, &page); err != nil {
			return err
		}
		for i := range page {
			status := &CommitStatus{
				Commit: latestCommit.ID,
				Status: *page[i],
			}
			statuses = append(statuses, status)
		}
	}

	pr.CommitStatus = statuses
	return nil
}

// ProjectRepos returns all repos of a project with a given projectKey
func (c *Client) ProjectRepos(ctx context.Context, projectKey string) (repos []*Repo, err error) {
	if projectKey == "" {
		return nil, errors.New("project key empty")
	}

	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos", projectKey)

	pageToken := &PageToken{Limit: 1000}

	for pageToken.HasMore() {
		var page []*Repo
		if pageToken, err = c.page(ctx, path, nil, pageToken, &page); err != nil {
			return nil, err
		}
		repos = append(repos, page...)
	}

	return repos, nil
}

func (c *Client) Repo(ctx context.Context, projectKey, repoSlug string) (*Repo, error) {
	u := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s", projectKey, repoSlug)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	var resp Repo
	_, err = c.do(ctx, req, &resp)
	return &resp, err
}

func (c *Client) Repos(ctx context.Context, pageToken *PageToken, searchQueries ...string) ([]*Repo, *PageToken, error) {
	qry, err := parseQueryStrings(searchQueries...)
	if err != nil {
		return nil, pageToken, err
	}

	var repos []*Repo
	next, err := c.page(ctx, "rest/api/1.0/repos", qry, pageToken, &repos)
	return repos, next, err
}

func (c *Client) LabeledRepos(ctx context.Context, pageToken *PageToken, label string) ([]*Repo, *PageToken, error) {
	u := fmt.Sprintf("rest/api/1.0/labels/%s/labeled", label)
	qry := url.Values{
		"REPOSITORY": []string{""},
	}

	var repos []*Repo
	next, err := c.page(ctx, u, qry, pageToken, &repos)
	return repos, next, err
}

// RepoIDs fetches a list of repository IDs that the user token has permission for.
// Permission: ["admin", "read", "write"]
func (c *Client) RepoIDs(ctx context.Context, permission string) ([]uint32, error) {
	u := fmt.Sprintf("rest/sourcegraph-admin/1.0/permissions/repositories?permission=%s", permission)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	var resp []byte
	_, err = c.do(ctx, req, &resp)
	if err != nil {
		return nil, err
	}

	bitmap := roaring.New()
	if err := bitmap.UnmarshalBinary(resp); err != nil {
		return nil, err
	}
	return bitmap.ToArray(), nil
}

func (c *Client) RecentRepos(ctx context.Context, pageToken *PageToken) ([]*Repo, *PageToken, error) {
	var repos []*Repo
	next, err := c.page(ctx, "rest/api/1.0/profile/recent/repos", nil, pageToken, &repos)
	return repos, next, err
}

type CreateForkInput struct {
	Name          *string                 `json:"name,omitempty"`
	DefaultBranch *string                 `json:"defaultBranch,omitempty"`
	Project       *CreateForkInputProject `json:"project,omitempty"`
}

type CreateForkInputProject struct {
	Key string `json:"key"`
}

func (c *Client) Fork(ctx context.Context, projectKey, repoSlug string, input CreateForkInput) (*Repo, error) {
	u := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s", projectKey, repoSlug)

	var resp Repo
	_, err := c.send(ctx, "POST", u, nil, input, &resp)
	return &resp, err
}

func (c *Client) page(ctx context.Context, path string, qry url.Values, token *PageToken, results any) (*PageToken, error) {
	if qry == nil {
		qry = make(url.Values)
	}

	for k, vs := range token.Values() {
		qry[k] = append(qry[k], vs...)
	}

	u := url.URL{Path: path, RawQuery: qry.Encode()}
	req, err := http.NewRequest("GET", u.String(), nil)
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

func (c *Client) send(ctx context.Context, method, path string, qry url.Values, payload, result any) (*http.Response, error) {
	if qry == nil {
		qry = make(url.Values)
	}

	var body io.ReadWriter
	if payload != nil {
		body = new(bytes.Buffer)
		if err := json.NewEncoder(body).Encode(payload); err != nil {
			return nil, err
		}
	}

	u := url.URL{Path: path, RawQuery: qry.Encode()}
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}

	return c.do(ctx, req, result)
}

func (c *Client) do(ctx context.Context, req *http.Request, result any) (_ *http.Response, err error) {
	tr, ctx := trace.New(ctx, "BitbucketServer.do")
	defer tr.EndWithErr(&err)
	req = req.WithContext(ctx)

	req.URL.Path, err = url.JoinPath(c.URL.Path, req.URL.Path) // First join paths so that base path is kept
	if err != nil {
		return nil, err
	}
	req.URL = c.URL.ResolveReference(req.URL)

	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}

	if err := c.Auth.Authenticate(req); err != nil {
		return nil, err
	}

	if err := c.rateLimit.Wait(ctx); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return resp, err
	}

	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return resp, errors.WithStack(&httpError{
			URL:        req.URL,
			StatusCode: resp.StatusCode,
			Body:       bs,
		})
	}

	// handle binary response
	if s, ok := result.(*[]byte); ok {
		*s = bs
	} else if result != nil {
		return resp, errors.Wrap(json.Unmarshal(bs, result), "failed to unmarshal response to JSON")
	}

	return resp, nil
}

func parseQueryStrings(qs ...string) (url.Values, error) {
	vals := make(url.Values)
	for _, q := range qs {
		query, err := url.ParseQuery(strings.TrimPrefix(q, "?"))
		if err != nil {
			return nil, err
		}
		for k, vs := range query {
			vals[k] = append(vals[k], vs...)
		}
	}
	return vals, nil
}

// categorize returns a category for an API URL. Used by metrics.
func categorize(u *url.URL) string {
	// API to URL mapping looks like this:
	//
	// 	Repo -> rest/api/1.0/profile/recent/repos%s
	// 	Repos -> rest/api/1.0/projects/%s/repos/%s
	// 	RecentRepos -> rest/api/1.0/repos%s
	//
	// We guess the category based on the fourth path component ("profile", "projects", "repos%s").
	var category string
	if parts := strings.SplitN(u.Path, "/", 3); len(parts) >= 4 {
		category = parts[3]
	}
	switch {
	case category == "profile":
		return "Repo"
	case category == "projects":
		return "Repos"
	case strings.HasPrefix(category, "repos"):
		return "RecentRepos"
	default:
		// don't return category directly as that could introduce too much dimensionality
		return "unknown"
	}
}

type PageToken struct {
	Size          int  `json:"size"`
	Limit         int  `json:"limit"`
	IsLastPage    bool `json:"isLastPage"`
	Start         int  `json:"start"`
	NextPageStart int  `json:"nextPageStart"`
}

func (t *PageToken) HasMore() bool {
	if t == nil {
		return true
	}
	return !t.IsLastPage
}

func (t *PageToken) Query() string {
	if t == nil {
		return ""
	}
	v := t.Values()
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}

func (t *PageToken) Values() url.Values {
	v := url.Values{}
	if t == nil {
		return v
	}
	if t.NextPageStart != 0 {
		v.Set("start", strconv.Itoa(t.NextPageStart))
	}
	if t.Limit != 0 {
		v.Set("limit", strconv.Itoa(t.Limit))
	}
	return v
}

// Perm represents a Bitbucket Server permission.
type Perm string

// Permission constants.
const (
	PermSysAdmin      Perm = "SYS_ADMIN"
	PermAdmin         Perm = "ADMIN"
	PermLicensedUser  Perm = "LICENSED_USER"
	PermProjectCreate Perm = "PROJECT_CREATE"

	PermProjectAdmin Perm = "PROJECT_ADMIN"
	PermProjectWrite Perm = "PROJECT_WRITE"
	PermProjectView  Perm = "PROJECT_VIEW"
	PermProjectRead  Perm = "PROJECT_READ"

	PermRepoAdmin Perm = "REPO_ADMIN"
	PermRepoRead  Perm = "REPO_READ"
	PermRepoWrite Perm = "REPO_WRITE"
)

// User account in a Bitbucket Server instance.
type User struct {
	Name         string `json:"name,omitempty"`
	Password     string `json:"-"`
	EmailAddress string `json:"emailAddress,omitempty"`
	ID           int    `json:"id,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	Active       bool   `json:"active,omitempty"`
	Slug         string `json:"slug,omitempty"`
	Type         string `json:"type,omitempty"`
}

// Group of users in a Bitbucket Server instance.
type Group struct {
	Name  string   `json:"name,omitempty"`
	Users []string `json:"users,omitempty"`
}

// A UserRepoPermission of a User to perform certain actions
// on a Repo.
type UserRepoPermission struct {
	User *User
	Perm Perm
	Repo *Repo
}

// A GroupRepoPermission of a Group to perform certain actions
// on a Repo.
type GroupRepoPermission struct {
	Group *Group
	Perm  Perm
	Repo  *Repo
}

// A UserProjectPermission of a User to perform certain actions
// on a Project.
type UserProjectPermission struct {
	User    *User
	Perm    Perm
	Project *Project
}

// A GroupProjectPermission of a Group to perform certain actions
// on a Project.
type GroupProjectPermission struct {
	Group   *Group
	Perm    Perm
	Project *Project
}

type Link struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

type RepoLinks struct {
	Clone []Link `json:"clone"`
	Self  []struct {
		Href string `json:"href"`
	} `json:"self"`
}

type Repo struct {
	Slug          string    `json:"slug"`
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	SCMID         string    `json:"scmId"`
	State         string    `json:"state"`
	StatusMessage string    `json:"statusMessage"`
	Forkable      bool      `json:"forkable"`
	Origin        *Repo     `json:"origin"`
	Project       *Project  `json:"project"`
	Public        bool      `json:"public"`
	Links         RepoLinks `json:"links"`
}

// IsPersonalRepository tells if the repository is a personal one.
func (r *Repo) IsPersonalRepository() bool {
	return r.Project.Type == "PERSONAL"
}

type Project struct {
	Key    string `json:"key"`
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
	Type   string `json:"type"`
	Links  struct {
		Self []struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"links"`
}

type ProjectKey struct {
	Key string `json:"key"`
}

type RefRepository struct {
	ID      int        `json:"id"`
	Slug    string     `json:"slug"`
	Project ProjectKey `json:"project"`
}

type Ref struct {
	ID         string        `json:"id"`
	Repository RefRepository `json:"repository"`
}

type PullRequest struct {
	ID           int               `json:"id"`
	Version      int               `json:"version"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	State        string            `json:"state"`
	Open         bool              `json:"open"`
	Closed       bool              `json:"closed"`
	CreatedDate  int               `json:"createdDate"`
	UpdatedDate  int               `json:"updatedDate"`
	FromRef      Ref               `json:"fromRef"`
	ToRef        Ref               `json:"toRef"`
	Locked       bool              `json:"locked"`
	Author       PullRequestAuthor `json:"author"`
	Reviewers    []Reviewer        `json:"reviewers"`
	Participants []Participant     `json:"participants"`
	Links        struct {
		Self []struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"links"`

	Activities   []*Activity     `json:"activities,omitempty"`
	Commits      []*Commit       `json:"commits,omitempty"`
	CommitStatus []*CommitStatus `json:"commit_status,omitempty"`

	// Deprecated, use CommitStatus instead. BuildStatus was not tied to individual commits
	BuildStatuses []*BuildStatus `json:"buildstatuses,omitempty"`
}

// PullRequestAuthor is the author of a pull request.
type PullRequestAuthor struct {
	User     *User  `json:"user"`
	Role     string `json:"role"`
	Approved bool   `json:"approved"`
	Status   string `json:"status"`
}

// Reviewer is a user that left feedback on a pull request.
type Reviewer struct {
	User               *User  `json:"user"`
	LastReviewedCommit string `json:"lastReviewedCommit"`
	Role               string `json:"role"`
	Approved           bool   `json:"approved"`
	Status             string `json:"status"`
}

// Participant is a user that was involved in a pull request.
type Participant struct {
	User     *User  `json:"user"`
	Role     string `json:"role"`
	Approved bool   `json:"approved"`
	Status   string `json:"status"`
}

// Activity is a union type of all supported pull request activity items.
type Activity struct {
	ID          int            `json:"id"`
	CreatedDate int            `json:"createdDate"`
	User        User           `json:"user"`
	Action      ActivityAction `json:"action"`

	// Comment activity fields.
	CommentAction string         `json:"commentAction,omitempty"`
	Comment       *Comment       `json:"comment,omitempty"`
	CommentAnchor *CommentAnchor `json:"commentAnchor,omitempty"`

	// Reviewers change fields.
	AddedReviewers   []User `json:"addedReviewers,omitempty"`
	RemovedReviewers []User `json:"removedReviewers,omitempty"`

	// Merged event fields.
	Commit *Commit `json:"commit,omitempty"`
}

// Key is a unique key identifying this activity in the context of its pull request.
func (a *Activity) Key() string { return strconv.Itoa(a.ID) }

// BuildStatus represents the build status of a commit
type BuildStatus struct {
	State       string `json:"state,omitempty"`
	Key         string `json:"key,omitempty"`
	Name        string `json:"name,omitempty"`
	Url         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
	DateAdded   int64  `json:"dateAdded,omitempty"`
}

// Commit status is the build status for a specific commit
type CommitStatus struct {
	Commit string      `json:"commit,omitempty"`
	Status BuildStatus `json:"status,omitempty"`
}

func (s *CommitStatus) Key() string {
	key := fmt.Sprintf("%s:%s:%s:%s", s.Commit, s.Status.Key, s.Status.Name, s.Status.Url)
	return strconv.FormatInt(int64(fnv1.HashString64(key)), 16)
}

// ActivityAction defines the action taken in an Activity.
type ActivityAction string

// Known ActivityActions
const (
	ApprovedActivityAction   ActivityAction = "APPROVED"
	UnapprovedActivityAction ActivityAction = "UNAPPROVED"
	DeclinedActivityAction   ActivityAction = "DECLINED"
	ReviewedActivityAction   ActivityAction = "REVIEWED"
	OpenedActivityAction     ActivityAction = "OPENED"
	ReopenedActivityAction   ActivityAction = "REOPENED"
	RescopedActivityAction   ActivityAction = "RESCOPED"
	UpdatedActivityAction    ActivityAction = "UPDATED"
	CommentedActivityAction  ActivityAction = "COMMENTED"
	MergedActivityAction     ActivityAction = "MERGED"
)

// A Comment in a PullRequest.
type Comment struct {
	ID                  int                 `json:"id"`
	Version             int                 `json:"version"`
	Text                string              `json:"text"`
	Author              User                `json:"author"`
	CreatedDate         int                 `json:"createdDate"`
	UpdatedDate         int                 `json:"updatedDate"`
	Comments            []Comment           `json:"comments"` // Replies to the comment
	Tasks               []Task              `json:"tasks"`
	PermittedOperations PermittedOperations `json:"permittedOperations"`
}

// A CommentAnchor captures the location of a code comment in a PullRequest.
type CommentAnchor struct {
	FromHash string `json:"fromHash"`
	ToHash   string `json:"toHash"`
	Line     int    `json:"line"`
	LineType string `json:"lineType"`
	FileType string `json:"fileType"`
	Path     string `json:"path"`
	DiffType string `json:"diffType"`
	Orphaned bool   `json:"orphaned"`
}

// A Task in a PullRequest.
type Task struct {
	ID                  int                 `json:"id"`
	Author              User                `json:"author"`
	Text                string              `json:"text"`
	State               string              `json:"state"`
	CreatedDate         int                 `json:"createdDate"`
	PermittedOperations PermittedOperations `json:"permittedOperations"`
}

// PermittedOperations of a Comment or Task.
type PermittedOperations struct {
	Editable       bool `json:"editable,omitempty"`
	Deletable      bool `json:"deletable,omitempty"`
	Transitionable bool `json:"transitionable,omitempty"`
}

// A Commit in a Repository.
type Commit struct {
	ID                 string   `json:"id,omitempty"`
	DisplayID          string   `json:"displayId,omitempty"`
	Author             *User    `json:"user,omitempty"`
	AuthorTimestamp    int64    `json:"authorTimestamp,omitempty"`
	Committer          *User    `json:"committer,omitempty"`
	CommitterTimestamp int64    `json:"committerTimestamp,omitempty"`
	Message            string   `json:"message,omitempty"`
	Parents            []Commit `json:"parents,omitempty"`
}

// IsNotFound reports whether err is a Bitbucket Server API not found error.
func IsNotFound(err error) bool {
	return errcode.IsNotFound(err)
}

// IsUnauthorized reports whether err is a Bitbucket Server API 401 error.
func IsUnauthorized(err error) bool {
	return errcode.IsUnauthorized(err)
}

// IsNoSuchLabel reports whether err is a Bitbucket Server API "No Such Label"
// error.
func IsNoSuchLabel(err error) bool {
	var e *httpError
	return errors.As(err, &e) && e.NoSuchLabelException()
}

// IsDuplicatePullRequest reports whether err is a Bitbucket Server API
// "Duplicate Pull Request" error.
func IsDuplicatePullRequest(err error) bool {
	var e *httpError
	return errors.As(err, &e) && e.DuplicatePullRequest()
}

func IsPullRequestOutOfDate(err error) bool {
	var e *httpError
	return errors.As(err, &e) && e.PullRequestOutOfDateException()
}

func IsMergePreconditionFailedException(err error) bool {
	var e *httpError
	return errors.As(err, &e) && e.MergePreconditionFailedException()
}

// ExtractExistingPullRequest will attempt to extract the existing PR returned with an error.
func ExtractExistingPullRequest(err error) (*PullRequest, error) {
	var e *httpError
	if errors.As(err, &e) {
		return e.ExtractExistingPullRequest()
	}

	return nil, errors.Errorf("error does not contain existing PR")
}

// ExtractPullRequest will attempt to extract the PR returned with an error.
func ExtractPullRequest(err error) (*PullRequest, error) {
	var e *httpError
	if errors.As(err, &e) {
		return e.ExtractPullRequest()
	}

	return nil, errors.Errorf("error does not contain existing PR")
}

type httpError struct {
	StatusCode int
	URL        *url.URL
	Body       []byte
}

func (e *httpError) Error() string {
	return fmt.Sprintf("Bitbucket API HTTP error: code=%d url=%q body=%q", e.StatusCode, e.URL, e.Body)
}

func (e *httpError) Unauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

func (e *httpError) NotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

func (e *httpError) DuplicatePullRequest() bool {
	return strings.Contains(string(e.Body), bitbucketDuplicatePRException)
}

func (e *httpError) NoSuchPullRequestException() bool {
	return strings.Contains(string(e.Body), bitbucketNoSuchPullRequestException)
}

func (e *httpError) NoSuchLabelException() bool {
	return strings.Contains(string(e.Body), bitbucketNoSuchLabelException)
}

func (e *httpError) MergePreconditionFailedException() bool {
	return strings.Contains(string(e.Body), bitbucketPullRequestMergeVetoedException)
}

func (e *httpError) PullRequestOutOfDateException() bool {
	return strings.Contains(string(e.Body), bitbucketPullRequestOutOfDateException)
}

const (
	bitbucketDuplicatePRException            = "com.atlassian.bitbucket.pull.DuplicatePullRequestException"
	bitbucketNoSuchLabelException            = "com.atlassian.bitbucket.label.NoSuchLabelException"
	bitbucketNoSuchPullRequestException      = "com.atlassian.bitbucket.pull.NoSuchPullRequestException"
	bitbucketPullRequestOutOfDateException   = "com.atlassian.bitbucket.pull.PullRequestOutOfDateException"
	bitbucketPullRequestMergeVetoedException = "com.atlassian.bitbucket.pull.PullRequestMergeVetoedException"
)

// ExtractExistingPullRequest will try to extract a PullRequest from the
// ExistingPullRequest field of the first Error in the response body.
func (e *httpError) ExtractExistingPullRequest() (*PullRequest, error) {
	var dest struct {
		Errors []struct {
			ExceptionName       string
			ExistingPullRequest PullRequest
		}
	}

	err := json.Unmarshal(e.Body, &dest)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling error")
	}

	for _, e := range dest.Errors {
		if e.ExceptionName == bitbucketDuplicatePRException {
			return &e.ExistingPullRequest, nil
		}
	}

	return nil, errors.New("existing PR not found")
}

// ExtractPullRequest will try to extract a PullRequest from the
// PullRequest field of the first Error in the response body.
func (e *httpError) ExtractPullRequest() (*PullRequest, error) {
	var dest struct {
		Errors []struct {
			ExceptionName string
			// This is different from ExistingPullRequest
			PullRequest PullRequest
		}
	}

	err := json.Unmarshal(e.Body, &dest)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling error")
	}

	if len(dest.Errors) == 0 {
		return nil, errors.New("existing PR not found")
	}

	return &dest.Errors[0].PullRequest, nil
}

// AuthenticatedUsername returns the username associated with the credentials
// used by the client.
// Since BitbucketServer doesn't offer an endpoint in their API to query the
// currently-authenticated user, we send a request to list a single user on the
// instance and then inspect the response headers in which BitbucketServer sets
// the username in X-Ausername.
// If no username is found in the response headers, an error is returned.
func (c *Client) AuthenticatedUsername(ctx context.Context) (username string, err error) {
	resp, err := c.send(ctx, "GET", "rest/api/1.0/users", url.Values{"limit": []string{"1"}}, nil, nil)
	if err != nil {
		return "", err
	}

	username = resp.Header.Get("X-Ausername")
	if username == "" {
		return "", errors.New("no username in X-Ausername header")
	}

	return username, nil
}

func (c *Client) CreatePullRequestComment(ctx context.Context, pr *PullRequest, body string) error {
	if pr.ToRef.Repository.Slug == "" {
		return errors.New("repository slug empty")
	}

	if pr.ToRef.Repository.Project.Key == "" {
		return errors.New("project key empty")
	}

	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
		pr.ID,
	)

	qry := url.Values{"version": {strconv.Itoa(pr.Version)}}

	payload := map[string]any{
		"text": body,
	}

	var resp *Comment
	_, err := c.send(ctx, "POST", path, qry, &payload, &resp)
	return err
}

func (c *Client) MergePullRequest(ctx context.Context, pr *PullRequest) error {
	if pr.ToRef.Repository.Slug == "" {
		return errors.New("repository slug empty")
	}

	if pr.ToRef.Repository.Project.Key == "" {
		return errors.New("project key empty")
	}

	path := fmt.Sprintf(
		"rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/merge",
		pr.ToRef.Repository.Project.Key,
		pr.ToRef.Repository.Slug,
		pr.ID,
	)

	qry := url.Values{"version": {strconv.Itoa(pr.Version)}}

	_, err := c.send(ctx, "POST", path, qry, nil, pr)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetVersion(ctx context.Context) (string, error) {
	var v struct {
		Version     string
		BuildNumber string
		BuildDate   string
		DisplayName string
	}

	_, err := c.send(ctx, "GET", "/rest/api/1.0/application-properties", nil, nil, &v)
	return v.Version, err
}
