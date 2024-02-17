package repos

import (
	"context"
	"path"
	"strconv"

	"github.com/goware/urlx"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/pagure"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A PagureSource yields repositories from a single Pagure connection configured
// in Sourcegraph via the external services configuration.
type PagureSource struct {
	svc       *types.ExternalService
	cli       *pagure.Client
	serviceID string
	perPage   int
}

// NewPagureSource returns a new PagureSource from the given external service.
func NewPagureSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*PagureSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.PagureConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Wrapf(err, "external service id=%d config error", svc.ID)
	}

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	httpCli, err := cf.Doer()
	if err != nil {
		return nil, err
	}

	cli, err := pagure.NewClient(svc.URN(), &c, httpCli)
	if err != nil {
		return nil, err
	}

	return &PagureSource{
		svc:       svc,
		cli:       cli,
		serviceID: extsvc.NormalizeBaseURL(cli.URL).String(),
		perPage:   100,
	}, nil
}

// CheckConnection at this point assumes availability and relies on errors returned
// from the subsequent calls. This is going to be expanded as part of issue #44683
// to actually only return true if the source can serve requests.
func (s *PagureSource) CheckConnection(ctx context.Context) error {
	return nil
}

// ListRepos returns all Pagure repositories configured with this PagureSource's config.
func (s *PagureSource) ListRepos(ctx context.Context, results chan SourceResult) {
	args := pagure.ListProjectsArgs{
		Cursor:    &pagure.Pagination{PerPage: s.perPage, Page: 1},
		Tags:      s.cli.Config.Tags,
		Pattern:   s.cli.Config.Pattern,
		Namespace: s.cli.Config.Namespace,
		Fork:      s.cli.Config.Forks,
	}

	it := s.cli.ListProjects(ctx, args)

	for it.Next() {
		repo, err := s.makeRepo(it.Current())
		if err != nil {
			results <- SourceResult{Source: s, Err: err}
			return
		}
		results <- SourceResult{Source: s, Repo: repo}
	}

	if err := it.Err(); err != nil {
		results <- SourceResult{Source: s, Err: err}
	}
}

// ExternalServices returns a singleton slice containing the external service.
func (s *PagureSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}

func (s *PagureSource) makeRepo(p *pagure.Project) (*types.Repo, error) {
	urn := s.svc.URN()

	fullURL, err := urlx.Parse(p.FullURL)
	if err != nil {
		return nil, err
	}

	name := path.Join(fullURL.Host, fullURL.Path)

	return &types.Repo{
		Name:        api.RepoName(name),
		URI:         name,
		Description: p.Description,
		Fork:        p.Parent != nil,
		ExternalRepo: api.ExternalRepoSpec{
			ID:          strconv.FormatInt(int64(p.ID), 10),
			ServiceType: extsvc.TypePagure,
			ServiceID:   s.serviceID,
		},
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: p.FullURL,
			},
		},
		Metadata: p,
	}, nil
}
