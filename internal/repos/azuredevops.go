package repos

import (
	"context"
	"fmt"
	"path"

	"github.com/goware/urlx"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A AzureDevOpsSource yields repositories from a single Azure DevOps connection configured
// in Sourcegraph via the external services configuration.
type AzureDevOpsSource struct {
	svc       *types.ExternalService
	cli       azuredevops.Client
	serviceID string
	config    schema.AzureDevOpsConnection
	logger    log.Logger
	excluder  repoExcluder
}

// NewAzureDevOpsSource returns a new AzureDevOpsSource from the given external service.
func NewAzureDevOpsSource(ctx context.Context, logger log.Logger, svc *types.ExternalService, cf *httpcli.Factory) (*AzureDevOpsSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "external service id=%d config", svc.ID)
	}
	var c schema.AzureDevOpsConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Wrapf(err, "external service id=%d config error", svc.ID)
	}

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	cli, err := azuredevops.NewClient(svc.URN(), c.Url, &auth.BasicAuth{Username: c.Username, Password: c.Token}, cf)
	if err != nil {
		return nil, err
	}

	var ex repoExcluder
	for _, r := range c.Exclude {
		ex.AddRule().
			Exact(r.Name).
			Pattern(r.Pattern)
	}
	if err := ex.RuleErrors(); err != nil {
		return nil, err
	}

	return &AzureDevOpsSource{
		svc:       svc,
		cli:       cli,
		serviceID: extsvc.NormalizeBaseURL(cli.GetURL()).String(),
		config:    c,
		logger:    logger,
		excluder:  ex,
	}, nil
}

// CheckConnection at this point assumes availability and relies on errors returned
// from the subsequent calls. This is going to be expanded as part of issue #44683
// to actually only return true if the source can serve requests.
func (s *AzureDevOpsSource) CheckConnection(ctx context.Context) error {
	if s.cli.IsAzureDevOpsServices() {
		_, err := s.cli.GetAuthorizedProfile(ctx)
		return err
	}
	// If this isn't Azure DevOps Services, i.e. not https://dev.azure.com, return
	// ok but log a warning because it is not supported.
	s.logger.Warn("connection check for Azure DevOps Server is not supported, skipping.")
	return nil
}

// ListRepos returns all Azure DevOps repositories configured with this AzureDevOpsSource's config.
func (s *AzureDevOpsSource) ListRepos(ctx context.Context, results chan SourceResult) {
	for _, project := range s.config.Projects {
		s.processReposFromProjectOrOrg(ctx, project, results)
	}

	for _, org := range s.config.Orgs {
		s.processReposFromProjectOrOrg(ctx, org, results)
	}
}

func (s *AzureDevOpsSource) processReposFromProjectOrOrg(ctx context.Context, name string, results chan SourceResult) {
	repos, err := s.cli.ListRepositoriesByProjectOrOrg(ctx, azuredevops.ListRepositoriesByProjectOrOrgArgs{
		ProjectOrOrgName: name,
	})
	if err != nil {
		results <- SourceResult{Source: s, Err: err}
		return
	}

	for _, repo := range repos {
		org, err := repo.GetOrganization()
		if err != nil {
			results <- SourceResult{Source: s, Err: err}
			continue
		}
		fullName := fmt.Sprintf("%s/%s/%s", org, repo.Project.Name, repo.Name)
		if s.excluder.ShouldExclude(fullName) {
			continue
		}
		repo, err := s.makeRepo(repo)
		if err != nil {
			results <- SourceResult{Source: s, Err: err}
			return
		}
		results <- SourceResult{Source: s, Repo: repo}
	}
}

// ExternalServices returns a singleton slice containing the external service.
func (s *AzureDevOpsSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}

// WithAuthenticator returns a copy of the original Source configured to use the
// given authenticator, provided that authenticator type is supported by the
// code host.
func (s *AzureDevOpsSource) WithAuthenticator(a auth.Authenticator) (Source, error) {
	sc := *s
	cli, err := sc.cli.WithAuthenticator(a)
	if err != nil {
		return nil, err
	}
	sc.cli = cli

	return &sc, nil
}

func (s *AzureDevOpsSource) makeRepo(p azuredevops.Repository) (*types.Repo, error) {
	urn := s.svc.URN()
	org, err := p.GetOrganization()
	if err != nil {
		return nil, err
	}
	fullURL, err := urlx.Parse(fmt.Sprintf("%s%s/%s/%s", s.cli.GetURL().String(), org, p.Project.Name, p.Name))
	if err != nil {
		return nil, err
	}

	cloneURL := p.RemoteURL
	if s.config.GitURLType == "ssh" {
		cloneURL = p.SSHURL
	}

	name := path.Join(fullURL.Host, fullURL.Path)
	return &types.Repo{
		Name: api.RepoName(name),
		URI:  name,
		Fork: p.IsFork,
		ExternalRepo: api.ExternalRepoSpec{
			ID:          p.ID,
			ServiceType: extsvc.TypeAzureDevOps,
			ServiceID:   s.serviceID,
		},
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: cloneURL,
			},
		},
		Metadata: p,
		Private:  p.Project.Visibility == "private",
	}, nil
}
