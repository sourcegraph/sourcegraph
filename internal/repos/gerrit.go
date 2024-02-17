package repos

import (
	"context"
	"net/url"
	"path"
	"sort"

	"github.com/goware/urlx"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gerrit"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A GerritSource yields repositories from a single Gerrit connection configured
// in Sourcegraph via the external services configuration.
type GerritSource struct {
	svc             *types.ExternalService
	cli             gerrit.Client
	serviceID       string
	perPage         int
	private         bool
	allowedProjects map[string]struct{}
	// disallowedProjects is a set of project names that will never be added
	// by this source. This takes precedence over allowedProjects.
	disallowedProjects map[string]struct{}
}

// NewGerritSource returns a new GerritSource from the given external service.
func NewGerritSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*GerritSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.GerritConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Wrapf(err, "external service id=%d config error", svc.ID)
	}

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	u, err := url.Parse(c.Url)
	if err != nil {
		return nil, err
	}

	cli, err := gerrit.NewClient(svc.URN(), u, &gerrit.AccountCredentials{
		Username: c.Username,
		Password: c.Password,
	}, cf)
	if err != nil {
		return nil, err
	}

	allowedProjects := make(map[string]struct{})
	for _, project := range c.Projects {
		allowedProjects[project] = struct{}{}
	}

	disallowedProjects := make(map[string]struct{})
	for _, project := range c.Exclude {
		disallowedProjects[project.Name] = struct{}{}
	}

	return &GerritSource{
		svc:                svc,
		cli:                cli,
		allowedProjects:    allowedProjects,
		disallowedProjects: disallowedProjects,
		serviceID:          extsvc.NormalizeBaseURL(cli.GetURL()).String(),
		perPage:            100,
		private:            c.Authorization != nil,
	}, nil
}

// CheckConnection at this point assumes availability and relies on errors returned
// from the subsequent calls. This is going to be expanded as part of issue #44683
// to actually only return true if the source can serve requests.
func (s *GerritSource) CheckConnection(ctx context.Context) error {
	return nil
}

// ListRepos returns all Gerrit repositories configured with this GerritSource's config.
func (s *GerritSource) ListRepos(ctx context.Context, results chan SourceResult) {
	args := gerrit.ListProjectsArgs{
		Cursor:           &gerrit.Pagination{PerPage: s.perPage, Page: 1},
		OnlyCodeProjects: true,
	}

	for {
		page, nextPage, err := s.cli.ListProjects(ctx, args)
		if err != nil {
			results <- SourceResult{Source: s, Err: err}
			return
		}

		// Unfortunately, because Gerrit API responds with a map, we have to sort it to maintain proper ordering
		pageKeySlice := make([]string, 0, len(page))

		for p := range page {
			pageKeySlice = append(pageKeySlice, p)
		}

		sort.Strings(pageKeySlice)

		for _, p := range pageKeySlice {
			if _, ok := s.disallowedProjects[p]; ok {
				continue
			}

			// Only check if the project is allowed if we have a list of allowed projects
			if len(s.allowedProjects) != 0 {
				if _, ok := s.allowedProjects[p]; !ok {
					continue
				}
			}

			repo, err := s.makeRepo(p, page[p])
			if err != nil {
				results <- SourceResult{Source: s, Err: err}
				return
			}
			results <- SourceResult{Source: s, Repo: repo}
		}

		if !nextPage {
			break
		}

		args.Cursor.Page++
	}
}

// ExternalServices returns a singleton slice containing the external service.
func (s *GerritSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}

func (s *GerritSource) makeRepo(projectName string, p *gerrit.Project) (*types.Repo, error) {
	urn := s.svc.URN()

	fullURL, err := urlx.Parse(s.cli.GetURL().JoinPath(projectName).String())
	if err != nil {
		return nil, err
	}

	name := path.Join(fullURL.Host, fullURL.Path)
	return &types.Repo{
		Name:        api.RepoName(name),
		URI:         name,
		Description: p.Description,
		Fork:        p.Parent != "",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          p.ID,
			ServiceType: extsvc.TypeGerrit,
			ServiceID:   s.serviceID,
		},
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: fullURL.String(),
			},
		},
		Metadata: p,
		Private:  s.private,
	}, nil
}

// WithAuthenticator returns a copy of the original Source configured to use the
// given authenticator, provided that authenticator type is supported by the
// code host.
func (s *GerritSource) WithAuthenticator(a auth.Authenticator) (Source, error) {
	sc := *s
	cli, err := sc.cli.WithAuthenticator(a)
	if err != nil {
		return nil, err
	}
	sc.cli = cli

	return &sc, nil
}
