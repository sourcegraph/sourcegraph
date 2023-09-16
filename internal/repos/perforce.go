package repos

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/perforce"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A PerforceSource yields depots from a single Perforce connection configured
// in Sourcegraph via the external services configuration.
type PerforceSource struct {
	gitserverClient gitserver.Client
	svc             *types.ExternalService
	config          *schema.PerforceConnection
}

// NewPerforceSource returns a new PerforceSource from the given external
// service.
func NewPerforceSource(ctx context.Context, svc *types.ExternalService) (*PerforceSource, error) {
	return NewPerforceSourceWithGitserverClient(ctx, gitserver.NewClient(), svc)
}

func NewPerforceSourceWithGitserverClient(ctx context.Context, gitserverClient gitserver.Client, svc *types.ExternalService) (*PerforceSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.PerforceConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	return newPerforceSource(gitserverClient, svc, &c)
}

func newPerforceSource(gitserverClient gitserver.Client, svc *types.ExternalService, c *schema.PerforceConnection) (*PerforceSource, error) {
	return &PerforceSource{
		svc:             svc,
		config:          c,
		gitserverClient: gitserverClient,
	}, nil
}

// CheckConnection at this point assumes availability and relies on errors returned
// from the subsequent calls. This is going to be expanded as part of issue #44683
// to actually only return true if the source can serve requests.
func (s PerforceSource) CheckConnection(ctx context.Context) error {
	rc, _, err := s.gitserverClient.P4Exec(ctx, s.config.P4Port, s.config.P4User, s.config.P4Passwd, "login", "-s")
	defer rc.Close()
	return err
}

// ListRepos returns all Perforce depots accessible to all connections
// configured in Sourcegraph via the external services configuration.
func (s PerforceSource) ListRepos(ctx context.Context, results chan SourceResult) {
	for _, depot := range s.config.Depots {
		// Tiny optimization: exit early if context has been canceled.
		if err := ctx.Err(); err != nil {
			results <- SourceResult{Source: s, Err: err}
			return
		}

		u := url.URL{
			Scheme: "perforce",
			Host:   s.config.P4Port,
			Path:   depot,
			User:   url.UserPassword(s.config.P4User, s.config.P4Passwd),
		}
		p4Url, err := vcs.ParseURL(u.String())
		if err != nil {
			results <- SourceResult{Source: s, Err: err}
			continue
		}

		syncer := server.PerforceDepotSyncer{}
		// We don't need to provide repo name and use "" instead because p4 commands are
		// not recorded in the following `syncer.IsCloneable` call.
		// First, check if the depot is clonable, this also checks that our credentials
		// are valid, and returns a proper error.
		if err := syncer.IsCloneable(ctx, "", p4Url); err == nil {
			// "Depot" is actually not the correct word in our config, it can be any
			// path in perforce, for example //eng/batches/. We grab the name of the root
			// depot to fetch some additional metadata and do a type == local check below.
			depotForPath := strings.SplitN(strings.Trim(depot, "/"), "/", 2)[0]
			// TODO: It looks like perforce actually creates the depot when querying it
			// with -o??
			rc, _, err := s.gitserverClient.P4Exec(ctx, s.config.P4Port, s.config.P4User, s.config.P4Passwd, "-Mj", "-z", "tag", "depot", "-o", depotForPath)
			if err != nil {
				results <- SourceResult{Source: s, Err: err}
				continue
			}
			defer rc.Close()
			var pd perforce.Depot
			if err := json.NewDecoder(rc).Decode(&pd); err != nil {
				results <- SourceResult{Source: s, Err: err}
				continue
			}

			if pd.Type != perforce.Local {
				results <- SourceResult{
					Source: s,
					Err:    errors.Newf("depot %q is of type %s, but only local depots are supported", depot, pd.Type),
				}
				continue
			}

			results <- SourceResult{Source: s, Repo: s.makeRepo(depot, &pd)}
		} else {
			results <- SourceResult{Source: s, Err: err}
		}
	}
}

// composePerforceCloneURL composes a clone URL for a Perforce depot based on
// given information. e.g.
// perforce://ssl:111.222.333.444:1666//Sourcegraph/
func composePerforceCloneURL(host, depot string) string {
	cloneURL := url.URL{
		Scheme: "perforce",
		Host:   host,
		Path:   depot,
	}
	return cloneURL.String()
}

func (s PerforceSource) makeRepo(depot string, pd *perforce.Depot) *types.Repo {
	if !strings.HasSuffix(depot, "/") {
		depot += "/"
	}
	name := strings.Trim(depot, "/")
	urn := s.svc.URN()

	cloneURL := composePerforceCloneURL(s.config.P4Port, depot)

	// Best effort parsing.
	createdAt, _ := time.Parse(pd.Date, "2006/01/02 03:04:05")

	return &types.Repo{
		Name: reposource.PerforceRepoName(
			s.config.RepositoryPathPattern,
			name,
		),
		URI: string(reposource.PerforceRepoName(
			"",
			name,
		)),
		Description: strings.Trim(strings.TrimSpace(pd.Description), "\n"),
		CreatedAt:   createdAt,
		ExternalRepo: api.ExternalRepoSpec{
			ID:          depot,
			ServiceType: extsvc.TypePerforce,
			ServiceID:   s.config.P4Port,
		},
		Private: true,
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: cloneURL,
			},
		},
		// TODO: The depot field will be wrong here, causing an incorrect cloneURL
		// to be created.
		Metadata: pd,
	}
}

// ExternalServices returns a singleton slice containing the external service.
func (s PerforceSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}
