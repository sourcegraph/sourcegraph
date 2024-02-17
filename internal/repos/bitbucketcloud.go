package repos

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A BitbucketCloudSource yields repositories from a single BitbucketCloud connection configured
// in Sourcegraph via the external services configuration.
type BitbucketCloudSource struct {
	svc      *types.ExternalService
	config   *schema.BitbucketCloudConnection
	excluder repoExcluder
	client   bitbucketcloud.Client
	logger   log.Logger
}

var _ UserSource = &BitbucketCloudSource{}

// NewBitbucketCloudSource returns a new BitbucketCloudSource from the given external service.
func NewBitbucketCloudSource(ctx context.Context, logger log.Logger, svc *types.ExternalService, cf *httpcli.Factory) (*BitbucketCloudSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.BitbucketCloudConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	return newBitbucketCloudSource(logger, svc, &c, cf)
}

func newBitbucketCloudSource(logger log.Logger, svc *types.ExternalService, c *schema.BitbucketCloudConnection, cf *httpcli.Factory) (*BitbucketCloudSource, error) {
	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	var ex repoExcluder
	for _, r := range c.Exclude {
		ex.AddRule().
			Exact(r.Name).
			Exact(r.Uuid).
			Pattern(r.Pattern)
	}
	if err := ex.RuleErrors(); err != nil {
		return nil, err
	}

	client, err := bitbucketcloud.NewClient(svc.URN(), c, cf)
	if err != nil {
		return nil, err
	}

	return &BitbucketCloudSource{
		svc:      svc,
		config:   c,
		excluder: ex,
		client:   client,
		logger:   logger,
	}, nil
}

func (s BitbucketCloudSource) CheckConnection(ctx context.Context) error {
	_, _, err := s.client.Repos(ctx, nil, "", nil)
	if err != nil {
		return errors.Wrap(err, "connection check failed. could not fetch authenticated user")
	}
	return nil
}

// ListRepos returns all Bitbucket Cloud repositories accessible to all connections configured
// in Sourcegraph via the external services configuration.
func (s BitbucketCloudSource) ListRepos(ctx context.Context, results chan SourceResult) {
	s.listAllRepos(ctx, results)
}

// ExternalServices returns a singleton slice containing the external service.
func (s BitbucketCloudSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}

func (s BitbucketCloudSource) makeRepo(r *bitbucketcloud.Repo) *types.Repo {
	host, err := url.Parse(s.config.Url)
	if err != nil {
		// This should never happen
		panic(errors.Errorf("malformed Bitbucket Cloud config, invalid URL: %q, error: %s", s.config.Url, err))
	}
	host = extsvc.NormalizeBaseURL(host)

	urn := s.svc.URN()
	return &types.Repo{
		Name: reposource.BitbucketCloudRepoName(
			s.config.RepositoryPathPattern,
			host.Hostname(),
			r.FullName,
		),
		URI: string(reposource.BitbucketCloudRepoName(
			"",
			host.Hostname(),
			r.FullName,
		)),
		ExternalRepo: api.ExternalRepoSpec{
			ID:          r.UUID,
			ServiceType: extsvc.TypeBitbucketCloud,
			ServiceID:   host.String(),
		},
		Description: r.Description,
		Fork:        r.Parent != nil,
		Private:     r.IsPrivate,
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: s.remoteURL(r),
			},
		},
		Metadata: r,
	}
}

// remoteURL returns the repository's Git remote URL
//
// note: this used to contain credentials but that is no longer the case
// if you need to get an authenticated clone url use repos.CloneURL
func (s *BitbucketCloudSource) remoteURL(repo *bitbucketcloud.Repo) string {
	if s.config.GitURLType == "ssh" {
		return fmt.Sprintf("git@%s:%s.git", s.config.Url, repo.FullName)
	}

	fallbackURL := (&url.URL{
		Scheme: "https",
		Host:   s.config.Url,
		Path:   "/" + repo.FullName,
	}).String()

	httpsURL, err := repo.Links.Clone.HTTPS()
	if err != nil {
		s.logger.Warn("Error adding authentication to Bitbucket Cloud repository Git remote URL.", log.String("url", fmt.Sprintf("%v", repo.Links.Clone)), log.Error(err))
		return fallbackURL
	}
	return httpsURL
}

func (s *BitbucketCloudSource) excludes(r *bitbucketcloud.Repo) bool {
	return s.excluder.ShouldExclude(r.FullName) || s.excluder.ShouldExclude(r.UUID)
}

func (s *BitbucketCloudSource) listAllRepos(ctx context.Context, results chan SourceResult) {
	type batch struct {
		repos []*bitbucketcloud.Repo
		err   error
	}

	ch := make(chan batch)

	var wg sync.WaitGroup

	// List all repositories of teams selected that the account has access to
	wg.Add(1)
	go func() {
		defer wg.Done()

		for _, t := range s.config.Teams {
			page := &bitbucketcloud.PageToken{Pagelen: 100}
			var err error
			var repos []*bitbucketcloud.Repo
			for page.HasMore() || page.Page == 0 {
				if repos, page, err = s.client.Repos(ctx, page, t, nil); err != nil {
					ch <- batch{err: errors.Wrapf(err, "bitbucketcloud.teams: item=%q, page=%+v", t, page)}
					break
				}

				ch <- batch{repos: repos}
			}
		}
	}()

	go func() {
		wg.Wait()
		close(ch)
	}()

	seen := make(map[string]bool)
	for r := range ch {
		if r.err != nil {
			results <- SourceResult{Source: s, Err: r.err}
			continue
		}

		for _, repo := range r.repos {
			// Discard non-Git repositories
			if repo.SCM != "git" {
				continue
			}

			if !seen[repo.UUID] && !s.excludes(repo) {
				results <- SourceResult{Source: s, Repo: s.makeRepo(repo)}
				seen[repo.UUID] = true
			}
		}
	}
}

// WithAuthenticator returns a copy of the original Source configured to use
// the given authenticator, provided that authenticator type is supported by
// the code host.
func (s *BitbucketCloudSource) WithAuthenticator(a auth.Authenticator) (Source, error) {
	switch a.(type) {
	case
		*auth.BasicAuth,
		*auth.BasicAuthWithSSH:
		break

	default:
		return nil, newUnsupportedAuthenticatorError("BitbucketCloudSource", a)
	}

	sc := *s
	sc.client = sc.client.WithAuthenticator(a)

	return &sc, nil
}

// ValidateAuthenticator validates the currently set authenticator is usable.
// Returns an error, when validating the Authenticator yielded an error.
func (s *BitbucketCloudSource) ValidateAuthenticator(ctx context.Context) error {
	_, _, err := s.client.Repos(ctx, nil, "", nil)
	return err
}
