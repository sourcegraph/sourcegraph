package repos

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A GitLabSource yields repositories from a single GitLab connection configured
// in Sourcegraph via the external services configuration.
type GitLabSource struct {
	svc                       *types.ExternalService
	config                    *schema.GitLabConnection
	excluder                  repoExcluder
	baseURL                   *url.URL // URL with path /api/v4 (no trailing slash)
	nameTransformations       reposource.NameTransformations
	provider                  *gitlab.ClientProvider
	client                    *gitlab.Client
	logger                    log.Logger
	markInternalReposAsPublic bool
}

var (
	_ Source                     = &GitLabSource{}
	_ UserSource                 = &GitLabSource{}
	_ AffiliatedRepositorySource = &GitLabSource{}
	_ VersionSource              = &GitLabSource{}
)

// NewGitLabSource returns a new GitLabSource from the given external service.
func NewGitLabSource(ctx context.Context, logger log.Logger, svc *types.ExternalService, cf *httpcli.Factory) (*GitLabSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.GitLabConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	return newGitLabSource(logger, svc, &c, cf)
}

var gitlabRemainingGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "src_gitlab_rate_limit_remaining",
	Help: "Number of calls to GitLab's API remaining before hitting the rate limit.",
}, []string{"resource", "name"})

var gitlabRatelimitWaitCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_gitlab_rate_limit_wait_duration_seconds",
	Help: "The amount of time spent waiting on the rate limit",
}, []string{"resource", "name"})

func newGitLabSource(logger log.Logger, svc *types.ExternalService, c *schema.GitLabConnection, cf *httpcli.Factory) (*GitLabSource, error) {
	baseURL, err := url.Parse(c.Url)
	if err != nil {
		return nil, err
	}
	baseURL = extsvc.NormalizeBaseURL(baseURL)

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	if c.Certificate != "" {
		cf = cf.WithOpts(httpcli.NewCertPoolOpt(c.Certificate))
	}

	var ex repoExcluder
	for _, r := range c.Exclude {
		rule := ex.AddRule().
			Exact(r.Name).
			Pattern(r.Pattern)

		if r.Id != 0 {
			rule.Exact(strconv.Itoa(r.Id))
		}

		if r.EmptyRepos {
			rule.Generic(func(repo any) bool {
				if project, ok := repo.(gitlab.Project); ok {
					return project.EmptyRepo
				}
				return false
			})
		}
	}
	if err := ex.RuleErrors(); err != nil {
		return nil, err
	}

	// Validate and cache user-defined name transformations.
	nts, err := reposource.CompileGitLabNameTransformations(c.NameTransformations)
	if err != nil {
		return nil, err
	}

	provider, err := gitlab.NewClientProvider(svc.URN(), baseURL, cf)
	if err != nil {
		return nil, err
	}

	var client *gitlab.Client
	switch gitlab.TokenType(c.TokenType) {
	case gitlab.TokenTypeOAuth:
		client = provider.GetOAuthClient(c.Token)
	default:
		client = provider.GetPATClient(c.Token, "")
	}

	if !envvar.SourcegraphDotComMode() || svc.CloudDefault {
		client.ExternalRateLimiter().SetCollector(&ratelimit.MetricsCollector{
			Remaining: func(n float64) {
				gitlabRemainingGauge.WithLabelValues("rest", svc.DisplayName).Set(n)
			},
			WaitDuration: func(n time.Duration) {
				gitlabRatelimitWaitCounter.WithLabelValues("rest", svc.DisplayName).Add(n.Seconds())
			},
		})
	}

	return &GitLabSource{
		svc:                       svc,
		config:                    c,
		excluder:                  ex,
		baseURL:                   baseURL,
		nameTransformations:       nts,
		provider:                  provider,
		client:                    client,
		logger:                    logger,
		markInternalReposAsPublic: c.MarkInternalReposAsPublic,
	}, nil
}

func (s GitLabSource) WithAuthenticator(a auth.Authenticator) (Source, error) {
	switch a.(type) {
	case *auth.OAuthBearerToken,
		*auth.OAuthBearerTokenWithSSH:
		break

	default:
		return nil, newUnsupportedAuthenticatorError("GitLabSource", a)
	}

	sc := s
	sc.client = sc.client.WithAuthenticator(a)

	return &sc, nil
}

func (s GitLabSource) Version(ctx context.Context) (string, error) {
	return s.client.GetVersion(ctx)
}

func (s GitLabSource) ValidateAuthenticator(ctx context.Context) error {
	return s.client.ValidateToken(ctx)
}

func (s GitLabSource) CheckConnection(ctx context.Context) error {
	_, err := s.client.GetUser(ctx, "")
	if err != nil {
		return errors.Wrap(err, "connection check failed. could not fetch authenticated user")
	}
	return nil
}

// ListRepos returns all GitLab repositories accessible to all connections configured
// in Sourcegraph via the external services configuration.
func (s GitLabSource) ListRepos(ctx context.Context, results chan SourceResult) {
	s.listAllProjects(ctx, results)
}

// GetRepo returns the GitLab repository with the given pathWithNamespace.
func (s GitLabSource) GetRepo(ctx context.Context, pathWithNamespace string) (*types.Repo, error) {
	proj, err := s.client.GetProject(ctx, gitlab.GetProjectOp{
		PathWithNamespace: pathWithNamespace,
		CommonOp:          gitlab.CommonOp{NoCache: true},
	})
	if err != nil {
		return nil, err
	}

	return s.makeRepo(proj), nil
}

// ExternalServices returns a singleton slice containing the external service.
func (s GitLabSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}

func (s GitLabSource) makeRepo(proj *gitlab.Project) *types.Repo {
	urn := s.svc.URN()

	private := proj.Visibility == gitlab.Private || proj.Visibility == gitlab.Internal
	if proj.Visibility == gitlab.Internal && s.markInternalReposAsPublic {
		private = false
	}

	return &types.Repo{
		Name: reposource.GitLabRepoName(
			s.config.RepositoryPathPattern,
			s.baseURL.Hostname(),
			proj.PathWithNamespace,
			s.nameTransformations,
		),
		URI: string(reposource.GitLabRepoName(
			"",
			s.baseURL.Hostname(),
			proj.PathWithNamespace,
			s.nameTransformations,
		)),
		ExternalRepo: gitlab.ExternalRepoSpec(proj, *s.baseURL),
		Description:  proj.Description,
		Fork:         proj.ForkedFromProject != nil,
		Archived:     proj.Archived,
		Stars:        proj.StarCount,
		Private:      private,
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: s.remoteURL(proj),
			},
		},
		Metadata: proj,
	}
}

// remoteURL returns the GitLab project's Git remote URL
//
// note: this used to contain credentials but that is no longer the case
// if you need to get an authenticated clone url use repos.CloneURL
func (s *GitLabSource) remoteURL(proj *gitlab.Project) string {
	if s.config.GitURLType == "ssh" {
		return proj.SSHURLToRepo // SSH authentication must be provided out-of-band
	}
	return proj.HTTPURLToRepo
}

func (s *GitLabSource) excludes(p *gitlab.Project) bool {
	return s.excluder.ShouldExclude(p.PathWithNamespace) ||
		s.excluder.ShouldExclude(strconv.Itoa(p.ID)) ||
		s.excluder.ShouldExclude(*p)
}

func (s *GitLabSource) listAllProjects(ctx context.Context, results chan SourceResult) {
	type batch struct {
		projs []*gitlab.Project
		err   error
	}

	ch := make(chan batch)

	var wg sync.WaitGroup

	projch := make(chan *schema.GitLabProject)
	for i := 0; i < 5; i++ { // 5 concurrent requests
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range projch {
				if err := ctx.Err(); err != nil {
					ch <- batch{err: err}
					return
				}

				proj, err := s.client.GetProject(ctx, gitlab.GetProjectOp{
					ID:                p.Id,
					PathWithNamespace: p.Name,
					CommonOp:          gitlab.CommonOp{NoCache: true},
				})

				if err != nil {
					// TODO(tsenart): When implementing dry-run, reconsider alternatives to return
					// 404 errors on external service config validation.
					if gitlab.IsNotFound(err) {
						s.logger.Warn("skipping missing gitlab.projects entry:", log.String("name", p.Name), log.Int("id", p.Id), log.Error(err))
						continue
					}
					ch <- batch{err: errors.Wrapf(err, "gitlab.projects: id: %d, name: %q", p.Id, p.Name)}
				} else {
					ch <- batch{projs: []*gitlab.Project{proj}}
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(projch)
		// Admins normally add to end of lists, so end of list most likely has
		// new repos => stream them first.
		for i := len(s.config.Projects) - 1; i >= 0; i-- {
			select {
			case projch <- s.config.Projects[i]:
			case <-ctx.Done():
				return
			}
		}
	}()

	for _, projectQuery := range s.config.ProjectQuery {
		if projectQuery == "none" {
			continue
		}

		const perPage = 100
		wg.Add(1)
		go func(projectQuery string) {
			defer wg.Done()

			urlStr, err := projectQueryToURL(projectQuery, perPage) // first page URL
			if err != nil {
				ch <- batch{err: errors.Wrapf(err, "invalid GitLab projectQuery=%q", projectQuery)}
				return
			}

			for {
				if err := ctx.Err(); err != nil {
					ch <- batch{err: err}
					return
				}
				projects, nextPageURL, err := s.client.ListProjects(ctx, urlStr)
				if err != nil {
					ch <- batch{err: errors.Wrapf(err, "error listing GitLab projects: url=%q", urlStr)}
					return
				}
				ch <- batch{projs: projects}
				if nextPageURL == nil {
					return
				}
				urlStr = *nextPageURL
			}
		}(projectQuery)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	seen := make(map[int]bool)
	for b := range ch {
		if b.err != nil {
			results <- SourceResult{Source: s, Err: b.err}
			continue
		}

		for _, proj := range b.projs {
			if !seen[proj.ID] && !s.excludes(proj) {
				results <- SourceResult{Source: s, Repo: s.makeRepo(proj)}
				seen[proj.ID] = true
			}
		}
	}
}

var schemeOrHostNotEmptyErr = errors.New("scheme and host should be empty")

func projectQueryToURL(projectQuery string, perPage int) (string, error) {
	// If all we have is the URL query, prepend "projects"
	if strings.HasPrefix(projectQuery, "?") {
		projectQuery = "projects" + projectQuery
	} else if projectQuery == "" {
		projectQuery = "projects"
	}

	u, err := url.Parse(projectQuery)
	if err != nil {
		return "", err
	}
	if u.Scheme != "" || u.Host != "" {
		return "", schemeOrHostNotEmptyErr
	}
	q := u.Query()
	q.Set("per_page", strconv.Itoa(perPage))
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (s *GitLabSource) AffiliatedRepositories(ctx context.Context) ([]types.CodeHostRepository, error) {
	queryURL, err := projectQueryToURL("projects?membership=true&archived=no", 40) // first page URL
	if err != nil {
		return nil, err
	}
	var (
		projects    []*gitlab.Project
		nextPageURL = &queryURL
	)

	out := []types.CodeHostRepository{}
	for nextPageURL != nil {
		projects, nextPageURL, err = s.client.ListProjects(ctx, *nextPageURL)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			out = append(out, types.CodeHostRepository{
				Name:       p.PathWithNamespace,
				Private:    p.Visibility == "private",
				CodeHostID: s.svc.ID,
			})
		}
	}
	return out, nil
}
