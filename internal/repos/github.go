package repos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	ghauth "github.com/sourcegraph/sourcegraph/internal/extsvc/github/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A GitHubSource yields repositories from a single GitHub connection configured
// in Sourcegraph via the external services configuration.
type GitHubSource struct {
	svc          *types.ExternalService
	config       *schema.GitHubConnection
	excluder     repoExcluder
	githubDotCom bool
	baseURL      *url.URL
	v3Client     *github.V3Client
	v4Client     *github.V4Client
	// searchClient is for using the GitHub search API, which has an independent
	// rate limit much lower than non-search API requests.
	searchClient *github.V3Client

	// originalHostname is the hostname of config.Url (differs from client APIURL, whose host is api.github.com
	// for an originalHostname of github.com).
	originalHostname string

	logger log.Logger

	markInternalReposAsPublic bool
}

var (
	_ Source                     = &GitHubSource{}
	_ UserSource                 = &GitHubSource{}
	_ AffiliatedRepositorySource = &GitHubSource{}
	_ VersionSource              = &GitHubSource{}
)

// NewGitHubSource returns a new GitHubSource from the given external service.
func NewGitHubSource(ctx context.Context, logger log.Logger, db database.DB, svc *types.ExternalService, cf *httpcli.Factory) (*GitHubSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.GitHubConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	return newGitHubSource(ctx, logger, db, svc, &c, cf)
}

var githubRemainingGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	// _v2 since we have an older metric defined in github-proxy
	Name: "src_github_rate_limit_remaining_v2",
	Help: "Number of calls to GitHub's API remaining before hitting the rate limit.",
}, []string{"resource", "name"})

var githubRatelimitWaitCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_github_rate_limit_wait_duration_seconds",
	Help: "The amount of time spent waiting on the rate limit",
}, []string{"resource", "name"})

func newGitHubSource(
	ctx context.Context,
	logger log.Logger,
	db database.DB,
	svc *types.ExternalService,
	c *schema.GitHubConnection,
	cf *httpcli.Factory,
) (*GitHubSource, error) {
	baseURL, err := url.Parse(c.Url)
	if err != nil {
		return nil, err
	}
	baseURL = extsvc.NormalizeBaseURL(baseURL)
	originalHostname := baseURL.Hostname()

	apiURL, githubDotCom := github.APIRoot(baseURL)

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	if c.Certificate != "" {
		cf = cf.WithOpts(httpcli.NewCertPoolOpt(c.Certificate))
	}

	var ex repoExcluder
	excludeArchived := func(repo any) bool {
		if githubRepo, ok := repo.(github.Repository); ok {
			return githubRepo.IsArchived
		}
		return false
	}
	excludeFork := func(repo any) bool {
		if githubRepo, ok := repo.(github.Repository); ok {
			return githubRepo.IsFork
		}
		return false
	}

	for _, r := range c.Exclude {
		rule := ex.AddRule().
			Exact(r.Name).
			Exact(r.Id).
			Pattern(r.Pattern)

		if r.Size != "" {
			fn, err := buildSizeConstraintsExcludeFn(r.Size)
			if err != nil {
				return nil, err
			}
			rule.Generic(fn)
		}
		if r.Stars != "" {
			fn, err := buildStarsConstraintsExcludeFn(r.Stars)
			if err != nil {
				return nil, err
			}
			rule.Generic(fn)
		}

		if r.Archived {
			rule.Generic(excludeArchived)
		}
		if r.Forks {
			rule.Generic(excludeFork)
		}
	}
	if err := ex.RuleErrors(); err != nil {
		return nil, err
	}

	auther, err := ghauth.FromConnection(ctx, c, db.GitHubApps(), keyring.Default().GitHubAppKey)
	if err != nil {
		return nil, err
	}
	urn := svc.URN()

	v4Client, err := github.NewV4Client(urn, apiURL, auther, cf)
	if err != nil {
		return nil, err
	}

	v3ClientLogger := log.Scoped("source")
	v3Client, err := github.NewV3Client(v3ClientLogger, urn, apiURL, auther, cf)
	if err != nil {
		return nil, err
	}
	searchClientLogger := log.Scoped("search")
	searchClient, err := github.NewV3SearchClient(searchClientLogger, urn, apiURL, auther, cf)
	if err != nil {
		return nil, err
	}

	for resource, monitor := range map[string]*ratelimit.Monitor{
		"rest":    v3Client.ExternalRateLimiter(),
		"graphql": v4Client.ExternalRateLimiter(),
		"search":  searchClient.ExternalRateLimiter(),
	} {
		// Copy the resource or funcs below will use the last one seen while iterating
		// the map
		resource := resource
		// Copy displayName so that the funcs below don't capture the svc pointer
		displayName := svc.DisplayName
		monitor.SetCollector(&ratelimit.MetricsCollector{
			Remaining: func(n float64) {
				githubRemainingGauge.WithLabelValues(resource, displayName).Set(n)
			},
			WaitDuration: func(n time.Duration) {
				githubRatelimitWaitCounter.WithLabelValues(resource, displayName).Add(n.Seconds())
			},
		})
	}

	return &GitHubSource{
		svc:                       svc,
		config:                    c,
		excluder:                  ex,
		baseURL:                   baseURL,
		githubDotCom:              githubDotCom,
		v3Client:                  v3Client,
		v4Client:                  v4Client,
		searchClient:              searchClient,
		originalHostname:          originalHostname,
		markInternalReposAsPublic: (c.Authorization != nil) && c.Authorization.MarkInternalReposAsPublic,
		logger: logger.With(
			log.Object("GitHubSource",
				log.Bool("githubDotCom", githubDotCom),
				log.String("originalHostname", originalHostname),
			),
		),
	}, nil
}

func (s *GitHubSource) WithAuthenticator(a auth.Authenticator) (Source, error) {
	sc := *s
	sc.v3Client = sc.v3Client.WithAuthenticator(a)
	sc.v4Client = sc.v4Client.WithAuthenticator(a)
	sc.searchClient = sc.searchClient.WithAuthenticator(a)

	return &sc, nil
}

type githubResult struct {
	err  error
	repo *github.Repository
}

func (s *GitHubSource) ValidateAuthenticator(ctx context.Context) error {
	var err error
	_, err = s.v3Client.GetAuthenticatedOAuthScopes(ctx)
	return err
}

func (s *GitHubSource) Version(ctx context.Context) (string, error) {
	return s.v3Client.GetVersion(ctx)
}

func (s *GitHubSource) CheckConnection(ctx context.Context) (err error) {
	if s.config.GitHubAppDetails == nil {
		_, err = s.v3Client.GetAuthenticatedUser(ctx)
	} else {
		_, _, _, err = s.v3Client.ListInstallationRepositories(ctx, 1)
	}
	if err != nil {
		return errors.Wrap(err, "connection check failed")
	}
	return nil
}

// ListRepos returns all Github repositories accessible to all connections configured
// in Sourcegraph via the external services configuration.
func (s *GitHubSource) ListRepos(ctx context.Context, results chan SourceResult) {
	unfiltered := make(chan *githubResult)
	go func() {
		s.listAllRepositories(ctx, unfiltered)
		close(unfiltered)
	}()

	seen := make(map[int64]bool)
	for res := range unfiltered {
		if res.err != nil {
			results <- SourceResult{Source: s, Err: res.err}
			continue
		}

		s.logger.Debug("unfiltered", log.String("repo", res.repo.NameWithOwner))
		if !seen[res.repo.DatabaseID] && !s.excludes(res.repo) {
			results <- SourceResult{Source: s, Repo: s.makeRepo(res.repo)}
			s.logger.Debug("sent to result", log.String("repo", res.repo.NameWithOwner))
			seen[res.repo.DatabaseID] = true
		}
	}
}

// SearchRepositories returns the Github repositories matching the repositoryQuery and excluded repositories criteria.
func (s *GitHubSource) SearchRepositories(ctx context.Context, query string, first int, excludedRepos []string, results chan SourceResult) {
	// default to fetching affiliated repositories
	if query == "" {
		s.fetchReposAffiliated(ctx, first, excludedRepos, results)
	} else {
		s.searchReposSinglePage(ctx, query, first, excludedRepos, results)
	}
}

func (s *GitHubSource) searchReposSinglePage(ctx context.Context, query string, first int, excludedRepos []string, results chan SourceResult) {
	unfiltered := make(chan *githubResult)
	var queryWithExcludeBuilder strings.Builder
	queryWithExcludeBuilder.WriteString(query)
	for _, repo := range excludedRepos {
		fmt.Fprintf(&queryWithExcludeBuilder, " -repo:%s", repo)
	}

	queryWithExclude := queryWithExcludeBuilder.String()
	repoQuery := repositoryQuery{Query: queryWithExclude, First: first, Searcher: s.v4Client, Logger: s.logger}

	go func() {
		repoQuery.DoSingleRequest(ctx, unfiltered)
		close(unfiltered)
	}()

	s.logger.Debug("fetch github repos by search query", log.String("query", query), log.Int("excluded repos count", len(excludedRepos)))
	for res := range unfiltered {
		if res.err != nil {
			results <- SourceResult{Source: s, Err: res.err}
			continue
		}

		results <- SourceResult{Source: s, Repo: s.makeRepo(res.repo)}
		s.logger.Debug("sent to result", log.String("repo", res.repo.NameWithOwner))
	}
}

func (s *GitHubSource) fetchReposAffiliated(ctx context.Context, first int, excludedRepos []string, results chan SourceResult) {
	unfiltered := make(chan *githubResult)

	// request larger page of results to account for exclusion taking effect afterwards
	bufferedFirst := first + len(excludedRepos)
	go func() {
		s.listAffiliatedPage(ctx, bufferedFirst, unfiltered)
		close(unfiltered)
	}()

	set := make(map[string]struct{})
	for _, r := range excludedRepos {
		set[r] = struct{}{}
	}
	excluded := func(name string) bool { _, ok := set[name]; return ok }

	s.logger.Debug("fetch github repos by affiliation", log.Int("excluded repos count", len(excludedRepos)))
	for res := range unfiltered {
		if first < 1 {
			continue // drain the remaining githubResults from unfiltered
		}
		if res.err != nil {
			results <- SourceResult{Source: s, Err: res.err}
			continue
		}
		s.logger.Debug("unfiltered", log.String("repo", res.repo.NameWithOwner))
		if !excluded(res.repo.NameWithOwner) {
			results <- SourceResult{Source: s, Repo: s.makeRepo(res.repo)}
			s.logger.Debug("sent to result", log.String("repo", res.repo.NameWithOwner))
			first--
		}
	}
}

// ExternalServices returns a singleton slice containing the external service.
func (s *GitHubSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}

// ListNamespaces returns all Github organizations accessible to the given source defined
// via the external service configuration.
func (s *GitHubSource) ListNamespaces(ctx context.Context, results chan SourceNamespaceResult) {
	var err error

	orgs := make([]*github.Org, 0)
	hasNextPage := true
	for page := 1; hasNextPage; page++ {
		if err = ctx.Err(); err != nil {
			results <- SourceNamespaceResult{Err: err}
			return
		}
		var pageOrgs []*github.Org
		pageOrgs, hasNextPage, _, err = s.v3Client.GetAuthenticatedUserOrgs(ctx, page)
		if err != nil {
			results <- SourceNamespaceResult{Source: s, Err: err}
			continue
		}
		orgs = append(orgs, pageOrgs...)
	}
	for _, org := range orgs {
		results <- SourceNamespaceResult{Source: s, Namespace: &types.ExternalServiceNamespace{ID: org.ID, Name: org.Login, ExternalID: org.NodeID}}
	}
}

// GetRepo returns the GitHub repository with the given name and owner
// ("org/repo-name")
func (s *GitHubSource) GetRepo(ctx context.Context, nameWithOwner string) (*types.Repo, error) {
	r, err := s.getRepository(ctx, nameWithOwner)
	if err != nil {
		return nil, err
	}
	return s.makeRepo(r), nil
}

func sanitizeToUTF8(s string) string {
	return strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "")
}

func (s *GitHubSource) makeRepo(r *github.Repository) *types.Repo {
	urn := s.svc.URN()
	metadata := *r
	// This field flip flops depending on which token was used to retrieve the repo
	// so we don't want to store it.
	metadata.ViewerPermission = ""
	metadata.Description = sanitizeToUTF8(metadata.Description)

	if github.Visibility(strings.ToLower(string(r.Visibility))) == github.VisibilityInternal && s.markInternalReposAsPublic {
		r.IsPrivate = false
	}

	return &types.Repo{
		Name: reposource.GitHubRepoName(
			s.config.RepositoryPathPattern,
			s.originalHostname,
			r.NameWithOwner,
		),
		URI: string(reposource.GitHubRepoName(
			"",
			s.originalHostname,
			r.NameWithOwner,
		)),
		ExternalRepo: github.ExternalRepoSpec(r, s.baseURL),
		Description:  sanitizeToUTF8(r.Description),
		Fork:         r.IsFork,
		Archived:     r.IsArchived,
		Stars:        r.StargazerCount,
		Private:      r.IsPrivate,
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: s.remoteURL(r),
			},
		},
		Metadata: &metadata,
	}
}

// remoteURL returns the repository's Git remote URL
//
// note: this used to contain credentials but that is no longer the case
// if you need to get an authenticated clone url use repos.CloneURL
func (s *GitHubSource) remoteURL(repo *github.Repository) string {
	if s.config.GitURLType == "ssh" {
		assembledURL := fmt.Sprintf("git@%s:%s.git", s.originalHostname, repo.NameWithOwner)
		return assembledURL
	}

	return repo.URL
}

func (s *GitHubSource) excludes(r *github.Repository) bool {
	if r.IsLocked || r.IsDisabled {
		return true
	}

	if s.excluder.ShouldExclude(r.NameWithOwner) ||
		s.excluder.ShouldExclude(r.ID) ||
		s.excluder.ShouldExclude(*r) {
		return true
	}

	return false
}

// repositoryPager is a function that returns repositories on a given `page`.
// It also returns:
// - `hasNext` bool: if there is a next page
// - `cost` int: rate limit cost used to determine recommended wait before next call
// - `err` error: if something goes wrong
type repositoryPager func(page int) (repos []*github.Repository, hasNext bool, cost int, err error)

// paginate returns all the repositories from the given repositoryPager.
// It repeatedly calls `pager` with incrementing page count until it
// returns false for hasNext.
func paginate(ctx context.Context, results chan *githubResult, pager repositoryPager) {
	hasNext := true
	for page := 1; hasNext; page++ {
		if err := ctx.Err(); err != nil {
			results <- &githubResult{err: err}
			return
		}

		var pageRepos []*github.Repository
		var err error
		pageRepos, hasNext, _, err = pager(page)
		if err != nil {
			results <- &githubResult{err: err}
			return
		}

		for _, r := range pageRepos {
			if err := ctx.Err(); err != nil {
				results <- &githubResult{err: err}
				return
			}

			results <- &githubResult{repo: r}
		}
	}
}

// listOrg handles the `org` config option.
// It returns all the repositories belonging to the given organization
// by hitting the /orgs/:org/repos endpoint.
//
// It returns an error if the request fails on the first page.
func (s *GitHubSource) listOrg(ctx context.Context, org string, results chan *githubResult) {
	dedupC := make(chan *githubResult)

	// Currently, the Github API doesn't return internal repos
	// when calling it with the "all" type.
	// We need to call it twice, once with the "all" type and
	// once with the "internal" type.
	// However, since we don't have any guarantee that this behavior
	// will always remain the same and that Github will never fix this issue,
	// we need to deduplicate the results before sending them to the results channel.

	getReposByType := func(tp string) error {
		var oerr error

		paginate(ctx, dedupC, func(page int) (repos []*github.Repository, hasNext bool, cost int, err error) {
			defer func() {
				if page == 1 {
					var e *github.APIError
					if errors.As(err, &e) && e.Code == 404 {
						oerr = errors.Errorf("organisation %q (specified in configuration) not found", org)
						err = nil
					}
				}

				remaining, reset, retry, _ := s.v3Client.ExternalRateLimiter().Get()
				s.logger.Debug(
					"github sync: ListOrgRepositories",
					log.Int("repos", len(repos)),
					log.Int("rateLimitCost", cost),
					log.Int("rateLimitRemaining", remaining),
					log.Duration("rateLimitReset", reset),
					log.Duration("retryAfter", retry),
					log.String("type", tp),
				)
			}()

			return s.v3Client.ListOrgRepositories(ctx, org, page, tp)
		})

		return oerr
	}

	go func() {
		defer close(dedupC)

		err := getReposByType("all")
		// Handle 404 from org repos endpoint by trying user repos endpoint
		if err != nil && ctx.Err() == nil {
			if s.listUser(ctx, org, dedupC) != nil {
				dedupC <- &githubResult{err: err}
			}
			return
		}

		if err := ctx.Err(); err != nil {
			dedupC <- &githubResult{err: err}
			return
		}

		// if the first call succeeded,
		// call the same endpoint with the "internal" type
		if err = getReposByType("internal"); err != nil {
			dedupC <- &githubResult{err: err}
		}
	}()

	seen := make(map[string]bool)

	for res := range dedupC {
		if res.err == nil {
			if seen[res.repo.ID] {
				continue
			}

			seen[res.repo.ID] = true
		}

		results <- res
	}
}

// listUser returns all the repositories belonging to the given user
// by hitting the /users/:user/repos endpoint.
//
// It returns an error if the request fails on the first page.
func (s *GitHubSource) listUser(ctx context.Context, user string, results chan *githubResult) (fail error) {
	paginate(ctx, results, func(page int) (repos []*github.Repository, hasNext bool, cost int, err error) {
		defer func() {
			if err != nil && page == 1 {
				fail, err = err, nil
			}

			remaining, reset, retry, _ := s.v3Client.ExternalRateLimiter().Get()
			s.logger.Debug(
				"github sync: ListUserRepositories",
				log.Int("repos", len(repos)),
				log.Int("rateLimitCost", cost),
				log.Int("rateLimitRemaining", remaining),
				log.Duration("rateLimitReset", reset),
				log.Duration("retryAfter", retry),
			)
		}()
		return s.v3Client.ListUserRepositories(ctx, user, page)
	})
	return
}

// listAppInstallation returns all the repositories belonging to the authenticated GitHub App installation
// by hitting the /installation/repositories endpoint.
//
// It returns an error if the request fails on the first page.
func (s *GitHubSource) listAppInstallation(ctx context.Context, results chan *githubResult) (fail error) {
	paginate(ctx, results, func(page int) (repos []*github.Repository, hasNext bool, cost int, err error) {
		defer func() {
			if err != nil && page == 1 {
				fail, err = err, nil
			}

			remaining, reset, retry, _ := s.v3Client.ExternalRateLimiter().Get()
			s.logger.Debug(
				"github sync: ListInstallationRepositories",
				log.Int("repos", len(repos)),
				log.Int("rateLimitCost", cost),
				log.Int("rateLimitRemaining", remaining),
				log.Duration("rateLimitReset", reset),
				log.Duration("retryAfter", retry),
			)
		}()
		return s.v3Client.ListInstallationRepositories(ctx, page)
	})
	return
}

// listRepos returns the valid repositories from the given list of repository names.
// This is done by hitting the /repos/:owner/:name endpoint for each of the given
// repository names.
func (s *GitHubSource) listRepos(ctx context.Context, repos []string, results chan *githubResult) {
	if err := s.fetchAllRepositoriesInBatches(ctx, results); err == nil {
		return
	} else {
		if err := ctx.Err(); err != nil {
			return
		}
		// The way we fetch repositories in batches through the GraphQL API -
		// using aliases to query multiple repositories in one query - is
		// currently "undefined behaviour". Very rarely but unreproducibly it
		// resulted in EOF errors while testing. And since we rely on fetching
		// to work, we fall back to the (slower) sequential fetching in case we
		// run into an GraphQL API error
		s.logger.Warn("github sync: fetching in batches failed, falling back to sequential fetch", log.Error(err))
	}

	// Admins normally add to end of lists, so end of list most likely has new
	// repos => stream them first.
	for i := len(repos) - 1; i >= 0; i-- {
		nameWithOwner := repos[i]
		if err := ctx.Err(); err != nil {
			results <- &githubResult{err: errors.Wrapf(err, "context error for repository: namewithOwner=%s", nameWithOwner)}
			return
		}

		owner, name, err := github.SplitRepositoryNameWithOwner(nameWithOwner)
		if err != nil {
			results <- &githubResult{err: errors.Newf("Invalid GitHub repository: nameWithOwner=%s", nameWithOwner)}
			return
		}
		var repo *github.Repository
		repo, err = s.v3Client.GetRepository(ctx, owner, name)
		if err != nil {
			// TODO(tsenart): When implementing dry-run, reconsider alternatives to return
			// 404 errors on external service config validation.
			if github.IsNotFound(err) {
				s.logger.Warn("skipping missing github.repos entry:", log.String("name", nameWithOwner), log.Error(err))
			} else {
				results <- &githubResult{err: errors.Wrapf(err, "Error getting GitHub repository: nameWithOwner=%s", nameWithOwner)}
			}
			continue
		}
		s.logger.Debug("github sync: GetRepository", log.String("repo", repo.NameWithOwner))

		results <- &githubResult{repo: repo}
	}
}

// listPublic handles the `public` keyword of the `repositoryQuery` config option.
// It returns the public repositories listed on the /repositories endpoint.
func (s *GitHubSource) listPublic(ctx context.Context, results chan *githubResult) {
	if s.githubDotCom {
		results <- &githubResult{err: errors.New(`unsupported configuration "public" for "repositoryQuery" for github.com`)}
		return
	}

	// The regular Github API endpoint for listing public repos doesn't return whether the repo is archived, so we have to list
	// all of the public archived repos first so we know if a repo is archived or not.
	// TODO: Remove querying for archived repos first when https://github.com/orgs/community/discussions/12554 gets resolved
	archivedReposChan := make(chan *githubResult)
	archivedRepos := make(map[string]struct{})
	archivedReposCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		s.listPublicArchivedRepos(archivedReposCtx, archivedReposChan)
		close(archivedReposChan)
	}()

	for res := range archivedReposChan {
		if res.err != nil {
			results <- &githubResult{err: errors.Wrap(res.err, "failed to list public archived Github repositories")}
			return
		}
		archivedRepos[res.repo.ID] = struct{}{}
	}

	var sinceRepoID int64
	for {
		if err := ctx.Err(); err != nil {
			results <- &githubResult{err: err}
			return
		}

		repos, hasNextPage, err := s.v3Client.ListPublicRepositories(ctx, sinceRepoID)
		if err != nil {
			apiError := &github.APIError{}
			// If the error is a http.StatusNotFound, we have paginated past the last page
			if errors.As(err, &apiError) && apiError.Code == http.StatusNotFound {
				return
			}
			results <- &githubResult{err: errors.Wrapf(err, "failed to list public repositories: sinceRepoID=%d", sinceRepoID)}
			return
		}
		s.logger.Debug("github sync public", log.Int("repos", len(repos)), log.Error(err))
		for _, r := range repos {
			_, isArchived := archivedRepos[r.ID]
			r.IsArchived = isArchived
			if err := ctx.Err(); err != nil {
				results <- &githubResult{err: err}
				return
			}

			results <- &githubResult{repo: r}
			if sinceRepoID < r.DatabaseID {
				sinceRepoID = r.DatabaseID
			}
		}
		if !hasNextPage {
			return
		}
	}
}

// listPublicArchivedRepos returns all of the public archived repositories listed on the /search/repositories endpoint.
// NOTE: There is a limitation on the search API that this uses, if there are more than 1000 public archived repos that
// were created in the same time (to the second), this list will miss any repos that lie outside of the first 1000.
func (s *GitHubSource) listPublicArchivedRepos(ctx context.Context, results chan *githubResult) {
	s.listSearch(ctx, "archived:true is:public", results)
}

// listAffiliated handles the `affiliated` keyword of the `repositoryQuery` config option.
// It returns the repositories affiliated with the client token by hitting the /user/repos
// endpoint.
//
// Affiliation is present if the user: (1) owns the repo, (2) is a part of an org that
// the repo belongs to, or (3) is a collaborator.
func (s *GitHubSource) listAffiliated(ctx context.Context, results chan *githubResult) {
	paginate(ctx, results, func(page int) (repos []*github.Repository, hasNext bool, cost int, err error) {
		defer func() {
			remaining, reset, retry, _ := s.v3Client.ExternalRateLimiter().Get()
			s.logger.Debug(
				"github sync: ListAffiliated",
				log.Int("repos", len(repos)),
				log.Int("rateLimitCost", cost),
				log.Int("rateLimitRemaining", remaining),
				log.Duration("rateLimitReset", reset),
				log.Duration("retryAfter", retry),
			)
		}()
		return s.v3Client.ListAffiliatedRepositories(ctx, github.VisibilityAll, page, 100)
	})
}

func (s *GitHubSource) listAffiliatedPage(ctx context.Context, first int, results chan *githubResult) {
	repos, _, _, err := s.v3Client.ListAffiliatedRepositories(ctx, github.VisibilityAll, 0, first)
	if err != nil {
		results <- &githubResult{err: err}
		return
	}

	for _, r := range repos {
		if err := ctx.Err(); err != nil {
			results <- &githubResult{err: err}
			return
		}

		results <- &githubResult{repo: r}
	}
}

// listSearch handles the `repositoryQuery` config option when a keyword is not present.
// It returns the repositories matching a GitHub's advanced repository search query
// via the GraphQL API.
func (s *GitHubSource) listSearch(ctx context.Context, q string, results chan *githubResult) {
	newRepositoryQuery(q, s.v4Client, s.logger).DoWithRefinedWindow(ctx, results)
}

// GitHub was founded on February 2008, so this minimum date covers all repos
// created on it.
var minCreated = time.Date(2007, time.June, 1, 0, 0, 0, 0, time.UTC)

type dateRange struct{ From, To time.Time }

var createdRegexp = regexp.MustCompile(`created:([^\s]+)`) // Matches the term "created:" followed by all non-white-space text

// stripDateRange strips the `created:` filter from the given string (modifying it in place)
// and returns a pointer to the resulting dateRange object.
// If no dateRange could be parsed from the string, nil is returned and the string is left unchanged.
func stripDateRange(s *string) *dateRange {
	matches := createdRegexp.FindStringSubmatch(*s)
	if len(matches) < 2 {
		return nil
	}
	dateStr := matches[1]

	parseDate := func(dateStr string, untilEndOfDay bool) (time.Time, error) {
		if strings.Contains(dateStr, "T") {
			if strings.Contains(dateStr, "+") || strings.Contains(dateStr, "Z") {
				return time.Parse(time.RFC3339, dateStr)
			}
			return time.Parse("2006-01-02T15:04:05", dateStr)
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return t, err
		}
		// If we need to match until the end of the day, the time should be 23:59:59
		// This only applies if no time was specified
		if untilEndOfDay {
			t = t.Add(24 * time.Hour).Add(-1 * time.Second)
		}
		return t, err
	}

	var fromDateStr, toDateStr string
	var fromTimeAdd, toTimeAdd time.Duration // Time to add to the respective dates in case of exclusive bounds
	var toEndOfDay bool                      // Whether or not the "To" date should include the entire day (for inclusive bounds checks)
	switch {
	case strings.HasPrefix(dateStr, ">="):
		fromDateStr = dateStr[2:]
	case strings.HasPrefix(dateStr, ">"):
		fromDateStr = dateStr[1:]
		fromTimeAdd = 1 * time.Second
	case strings.HasPrefix(dateStr, "<="):
		toDateStr = dateStr[2:]
		toEndOfDay = true
	case strings.HasPrefix(dateStr, "<"):
		toDateStr = dateStr[1:]
		toTimeAdd = -1 * time.Second
	default:
		rangeParts := strings.Split(dateStr, "..")
		if len(rangeParts) != 2 {
			return nil
		}
		fromDateStr = rangeParts[0]
		toDateStr = rangeParts[1]
		if toDateStr != "*" {
			toEndOfDay = true
		}
	}

	var err error
	dr := &dateRange{}
	if fromDateStr != "" && fromDateStr != "*" {
		dr.From, err = parseDate(fromDateStr, false)
		if err != nil {
			return nil
		}
		dr.From = dr.From.Add(fromTimeAdd)
	}
	if toDateStr != "" && toDateStr != "*" {
		dr.To, err = parseDate(toDateStr, toEndOfDay)
		if err != nil {
			return nil
		}
		dr.To = dr.To.Add(toTimeAdd)
	}

	*s = strings.ReplaceAll(*s, matches[0], "")
	return dr
}

func (r dateRange) String() string {
	const dateFormat = "2006-01-02T15:04:05-07:00"

	return fmt.Sprintf("%s..%s",
		r.From.Format(dateFormat),
		r.To.Format(dateFormat),
	)
}

func (r dateRange) Size() time.Duration { return r.To.Sub(r.From) }

type searchReposCount struct {
	known bool
	count int
}

type repositoryQuery struct {
	Query     string
	Created   *dateRange
	Cursor    github.Cursor
	First     int
	Limit     int
	Searcher  *github.V4Client
	Logger    log.Logger
	RepoCount searchReposCount
}

func newRepositoryQuery(query string, searcher *github.V4Client, logger log.Logger) *repositoryQuery {
	// First we need to parse the query to see if it is querying within a date range,
	// and if so, strip that date range from the query.
	dr := stripDateRange(&query)
	if dr == nil {
		dr = &dateRange{}
	}
	if dr.From.IsZero() {
		dr.From = minCreated
	}
	if dr.To.IsZero() {
		dr.To = time.Now()
	}
	return &repositoryQuery{
		Query:    query,
		Searcher: searcher,
		Logger:   logger,
		Created:  dr,
	}
}

// DoWithRefinedWindow attempts to retrieve all matching repositories by refining the window of acceptable Created dates
// to smaller windows and re-running the search (down to a minimum window size)
// and exiting once all repositories are returned.
func (q *repositoryQuery) DoWithRefinedWindow(ctx context.Context, results chan *githubResult) {
	if q.First == 0 {
		q.First = 100
	}
	if q.Limit == 0 {
		// GitHub's search API returns a maximum of 1000 results
		q.Limit = 1000
	}
	if q.Created == nil {
		q.Created = &dateRange{
			From: minCreated,
			To:   time.Now(),
		}
	}

	if err := q.doRecursively(ctx, results); err != nil {
		select {
		case <-ctx.Done():
		case results <- &githubResult{err: errors.Wrapf(err, "failed to search GitHub repositories with %q", q)}:
		}
	}
}

// DoSingleRequest accepts the first n results and does not refine the search window on Created date.
// Missing some repositories which match the criteria is acceptable.
func (q *repositoryQuery) DoSingleRequest(ctx context.Context, results chan *githubResult) {
	if q.First == 0 {
		q.First = 100
	}

	if err := ctx.Err(); err != nil {
		results <- &githubResult{err: err}
	}
	res, err := q.Searcher.SearchRepos(ctx, github.SearchReposParams{
		Query: q.String(),
		First: q.First,
		After: q.Cursor,
	})
	if err != nil {
		select {
		case <-ctx.Done():
		case results <- &githubResult{err: errors.Wrapf(err, "failed to search GitHub repositories with %q", q)}:
		}
	}

	for i := range res.Repos {
	out:
		select {
		case <-ctx.Done():
			break out
		case results <- &githubResult{repo: &res.Repos[i]}:
		}
	}
}

func (q *repositoryQuery) split(ctx context.Context, results chan *githubResult) error {
	middle := q.Created.From.Add(q.Created.To.Sub(q.Created.From) / 2)
	q1, q2 := *q, *q
	q1.RepoCount.known = false
	q1.Created = &dateRange{
		From: q.Created.From,
		To:   middle.Add(-1 * time.Second),
	}
	q2.Created = &dateRange{
		From: middle,
		To:   q.Created.To,
	}
	if err := q1.doRecursively(ctx, results); err != nil {
		return err
	}
	// We now know the repoCount of q2 by subtracting the repoCount of q1 from the original q
	q2.RepoCount = searchReposCount{
		known: true,
		count: q.RepoCount.count - q1.RepoCount.count,
	}
	return q2.doRecursively(ctx, results)
}

// doRecursively performs a query with the following procedure:
// 1. Perform the query.
// 2. If the number of search results returned is greater than the query limit, split the query in half by filtering by repo creation date, and perform those two queries. Do so recursively.
// 3. If the number of search results returned is less than or equal to the query limit, iterate over the results and return them to the channel.
func (q *repositoryQuery) doRecursively(ctx context.Context, results chan *githubResult) error {
	// If we know that the number of repos in this query is greater than the limit, we can immediately split the query
	// Also, GitHub createdAt time stamps are only accurate to 1 second. So if the time difference is no longer
	// greater than 2 seconds, we should stop refining as it cannot get more precise.
	if q.RepoCount.known && q.RepoCount.count > q.Limit && q.Created.To.Sub(q.Created.From) >= 2*time.Second {
		return q.split(ctx, results)
	}

	// Otherwise we need to confirm the number of repositories first
	res, err := q.Searcher.SearchRepos(ctx, github.SearchReposParams{
		Query: q.String(),
		First: q.First,
		After: q.Cursor,
	})
	if err != nil {
		return nil
	}

	q.RepoCount = searchReposCount{
		known: true,
		count: res.TotalCount,
	}

	// Now that we know the repo count, we can perform a check again and split if necessary
	if q.RepoCount.count > q.Limit && q.Created.To.Sub(q.Created.From) >= 2*time.Second {
		return q.split(ctx, results)
	}

	const maxTries = 3
	numTries := 0
	seen := make(map[int64]struct{}, res.TotalCount)
	// If the number of repos is lower than the limit, we perform the actual search
	// and iterate over the results
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		for i := range res.Repos {
			select {
			case <-ctx.Done():
				return nil
			default:
				if _, ok := seen[res.Repos[i].DatabaseID]; !ok {
					results <- &githubResult{repo: &res.Repos[i]}
					seen[res.Repos[i].DatabaseID] = struct{}{}
					if len(seen) >= res.TotalCount {
						break
					}
				}
			}
		}

		// Only break if we've seen a number of repositories equal to the expected count
		// res.EndCursor will loop by itself
		if len(seen) >= res.TotalCount || len(seen) >= q.Limit {
			break
		}

		// Set a hard cap on the number of retries
		if res.EndCursor == "" {
			numTries += 1
			if numTries >= maxTries {
				break
			}
		}

		res, err = q.Searcher.SearchRepos(ctx, github.SearchReposParams{
			Query: q.String(),
			First: q.First,
			After: res.EndCursor,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Refine does one pass at refining the query to match <= 1000 repos in order
// to avoid hitting the GitHub search API 1000 results limit, which would cause
// use to miss matches.
func (s *repositoryQuery) Refine() bool {
	if s.Created.Size() < 2*time.Second {
		// Can't refine further than 1 second
		return false
	}

	// We found too many results, move the slice:
	// From -> To  ----> From -> To - (To-From)/2
	s.Created.To = s.Created.To.Add(-(s.Created.Size() / 2))
	return true
}

func (s repositoryQuery) String() string {
	q := s.Query
	if s.Created != nil {
		q += " created:" + s.Created.String()
	}
	return q
}

// regOrg is a regular expression that matches the pattern `org:<org-name>`
// `<org-name>` follows the GitHub username convention:
// - only single hyphens and alphanumeric characters allowed.
// - cannot begin/end with hyphen.
// - up to 38 characters.
var regOrg = lazyregexp.New(`^org:([a-zA-Z0-9](?:-?[a-zA-Z0-9]){0,38})$`)

// matchOrg extracts the org name from the pattern `org:<org-name>` if it exists.
func matchOrg(q string) string {
	match := regOrg.FindStringSubmatch(q)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

// listRepositoryQuery handles the `repositoryQuery` config option.
// The supported keywords to select repositories are:
// - `public`: public repositories (from endpoint: /repositories)
// - `affiliated`: repositories affiliated with client token (from endpoint: /user/repos)
// - `none`: disables `repositoryQuery`
// Inputs other than these three keywords will be queried using
// GitHub advanced repository search (endpoint: /search/repositories)
func (s *GitHubSource) listRepositoryQuery(ctx context.Context, query string, results chan *githubResult) {
	switch query {
	case "public":
		s.listPublic(ctx, results)
		return
	case "affiliated":
		s.listAffiliated(ctx, results)
		return
	case "none":
		// nothing
		return
	}

	// Special-casing for `org:<org-name>`
	// to directly use GitHub's org repo
	// list API instead of the limited
	// search API.
	//
	// If the org repo list API fails, we
	// try the user repo list API.
	if org := matchOrg(query); org != "" {
		s.listOrg(ctx, org, results)
		return
	}

	// Run the query as a GitHub advanced repository search
	// (https://github.com/search/advanced).
	s.listSearch(ctx, query, results)
}

// listAllRepositories returns the repositories from the given `orgs`, `repos`,
// `repositoryQuery`, and GitHubAppDetails config options, excluding the ones specified by `exclude`.
func (s *GitHubSource) listAllRepositories(ctx context.Context, results chan *githubResult) {
	s.listRepos(ctx, s.config.Repos, results)

	// Admins normally add to end of lists, so end of list most likely has new
	// repos => stream them first.
	for i := len(s.config.RepositoryQuery) - 1; i >= 0; i-- {
		s.listRepositoryQuery(ctx, s.config.RepositoryQuery[i], results)
	}

	for i := len(s.config.Orgs) - 1; i >= 0; i-- {
		s.listOrg(ctx, s.config.Orgs[i], results)
	}

	if s.config.GitHubAppDetails != nil && s.config.GitHubAppDetails.CloneAllRepositories {
		s.listAppInstallation(ctx, results)
	}
}

func (s *GitHubSource) getRepository(ctx context.Context, nameWithOwner string) (*github.Repository, error) {
	owner, name, err := github.SplitRepositoryNameWithOwner(nameWithOwner)
	if err != nil {
		return nil, errors.Wrapf(err, "Invalid GitHub repository: nameWithOwner="+nameWithOwner)
	}

	repo, err := s.v3Client.GetRepository(ctx, owner, name)
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// fetchAllRepositoriesInBatches fetches the repositories configured in
// config.Repos in batches and adds them to the supplied set
func (s *GitHubSource) fetchAllRepositoriesInBatches(ctx context.Context, results chan *githubResult) error {
	const batchSize = 30

	// Admins normally add to end of lists, so end of list most likely has new
	// repos => stream them first.
	s.logger.Debug("fetching list of repos", log.Int("len", len(s.config.Repos)))
	for end := len(s.config.Repos); end > 0; end -= batchSize {
		if err := ctx.Err(); err != nil {
			return err
		}

		start := end - batchSize
		if start < 0 {
			start = 0
		}
		batch := s.config.Repos[start:end]

		repos, err := s.v4Client.GetReposByNameWithOwner(ctx, batch...)
		if err != nil {
			return errors.Wrap(err, "GetReposByNameWithOwner failed")
		}

		s.logger.Debug("github sync: GetReposByNameWithOwner", log.Strings("repos", batch))
		for _, r := range repos {
			if err := ctx.Err(); err != nil {
				if r != nil {
					err = errors.Wrapf(err, "context error for repository: %s", r.NameWithOwner)
				}

				results <- &githubResult{err: err}
				return err
			}

			results <- &githubResult{repo: r}
			s.logger.Debug("sent repo to result", log.String("repo", fmt.Sprintf("%+v", r)))
		}
	}

	return nil
}

func (s *GitHubSource) AffiliatedRepositories(ctx context.Context) ([]types.CodeHostRepository, error) {
	var (
		repos []*github.Repository
		page  = 1
		cost  int
		err   error
	)
	defer func() {
		remaining, reset, retry, _ := s.v3Client.ExternalRateLimiter().Get()
		s.logger.Debug(
			"github sync: ListAffiliated",
			log.Int("repos", len(repos)),
			log.Int("rateLimitCost", cost),
			log.Int("rateLimitRemaining", remaining),
			log.Duration("rateLimitReset", reset),
			log.Duration("retryAfter", retry),
		)
	}()
	out := make([]types.CodeHostRepository, 0)
	hasNextPage := true
	for hasNextPage {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var repos []*github.Repository
		repos, hasNextPage, _, err = s.v3Client.ListAffiliatedRepositories(ctx, github.VisibilityAll, page, 100)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			out = append(out, types.CodeHostRepository{
				Name:       repo.NameWithOwner,
				Private:    repo.IsPrivate,
				CodeHostID: s.svc.ID,
			})
		}
		page++
	}
	return out, nil
}
