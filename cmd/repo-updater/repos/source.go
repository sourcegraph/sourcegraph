package repos

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	"github.com/sourcegraph/sourcegraph/pkg/extsvc/github"
	"github.com/sourcegraph/sourcegraph/pkg/jsonc"
	"github.com/sourcegraph/sourcegraph/schema"
)

// Sourcerer converts each code host connection configured via external services
// in the frontend API to a Source that yields Repos. Each invocation of ListSources
// may yield different Sources depending on what the user configured at a given point
// in time.
type Sourcerer struct {
	api InternalAPI
}

// NewSourcerer returns a Sourcerer of the given Frontend API.
func NewSourcerer(api InternalAPI) *Sourcerer {
	return &Sourcerer{api: api}
}

// ListSources lists all configured repository yielding Sources of the given kinds,
// based on the code host connections configured via external services in the frontend API.
func (s Sourcerer) ListSources(ctx context.Context, kinds ...string) ([]Source, error) {
	svcs, err := s.api.ExternalServicesList(ctx, api.ExternalServicesListRequest{Kinds: kinds})
	if err != nil {
		return nil, err
	}

	srcs := make([]Source, 0, len(svcs)+1)
	errs := new(multierror.Error)
	for _, svc := range svcs {
		if src, err := NewSource(svc); err != nil {
			errs = multierror.Append(errs, err)
		} else {
			srcs = append(srcs, src)
		}
	}

	if !includesGitHubDotComSource(srcs) {
		// add a GitHub.com source by default, to support navigating to URL paths like
		// /github.com/foo/bar to auto-add that repository.
		src, err := NewGithubSource(&schema.GitHubConnection{
			RepositoryQuery:             []string{"none"}, // don't try to list all repositories during syncs
			Url:                         "https://github.com",
			InitialRepositoryEnablement: true,
		})
		srcs, errs = append(srcs, src), multierror.Append(errs, err)
	}

	return srcs, errs.ErrorOrNil()
}

// NewSource returns a repository yielding Source from the given api.ExternalService configuration.
func NewSource(svc *api.ExternalService) (Source, error) {
	var conn interface{}

	switch svc.Kind {
	case "AWSCODECOMMIT":
		conn = &schema.AWSCodeCommitConnection{}
	case "BITBUCKETSERVER":
		conn = &schema.BitbucketServerConnection{}
	case "GITHUB":
		conn = &schema.GitHubConnection{}
	case "GITLAB":
		conn = &schema.GitLabConnection{}
	case "GITOLITE":
		conn = &schema.GitoliteConnection{}
	case "PHABRICATOR":
		conn = &schema.PhabricatorConnection{}
	case "OTHER":
		conn = &schema.OtherExternalServiceConnection{}
	default:
		return nil, fmt.Errorf("external service id=%d has unknown kind %q", svc.ID, svc.Kind)
	}

	if err := jsonc.Unmarshal(svc.Config, conn); err != nil {
		return nil, fmt.Errorf("external service id=%d config error: %s", svc.ID, err)
	}

	switch svc.Kind {
	case "GITHUB":
		return NewGithubSource(conn.(*schema.GitHubConnection))
	default:
		panic(fmt.Sprintf("source not implemented for external service kind %q", svc.Kind))
	}
}

func includesGitHubDotComSource(srcs []Source) bool {
	for _, src := range srcs {
		if gs, ok := src.(*GithubSource); !ok {
			continue
		} else if u, err := url.Parse(gs.conn.config.Url); err != nil {
			continue
		} else if strings.HasSuffix(u.Hostname(), "github.com") {
			return true
		}
	}
	return false
}

// A Source yields repositories to be stored and analysed by Sourcegraph.
// Successive calls to its ListRepos method may yield different results.
type Source interface {
	ListRepos(context.Context) ([]*Repo, error)
}

// A GithubSource yields repositories from a single Github connection configured
// in Sourcegraph via the external services configuration.
type GithubSource struct {
	conn *githubConnection
}

// NewGithubSource returns a new GithubSource with the given configs.
func NewGithubSource(c *schema.GitHubConnection) (*GithubSource, error) {
	conn, err := newGitHubConnection(c)
	if err != nil {
		return nil, err
	}
	return &GithubSource{conn: conn}, nil
}

// ListRepos returns all Github repositories accessible to all connections configured
// in Sourcegraph via the external services configuration.
func (s GithubSource) ListRepos(ctx context.Context) ([]*Repo, error) {
	var repos []*Repo
	for repo := range s.conn.listAllRepositories(ctx) {
		repos = append(repos, githubRepoToRepo(repo, s.conn))
	}
	return repos, nil
}

func githubRepoToRepo(ghrepo *github.Repository, conn *githubConnection) *Repo {
	return &Repo{
		Name:         string(githubRepositoryToRepoPath(conn, ghrepo)),
		CloneURL:     conn.authenticatedRemoteURL(ghrepo),
		ExternalRepo: *github.ExternalRepoSpec(ghrepo, *conn.baseURL),
		Description:  ghrepo.Description,
		Fork:         ghrepo.IsFork,
		Archived:     ghrepo.IsArchived,
	}
}
