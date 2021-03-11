package graphqlbackend

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"

	"github.com/cockroachdb/errors"
	"github.com/graph-gophers/graphql-go"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	gitserverproto "github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	repoupdaterproto "github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

const maxUserPublicRepos = 100

func (r *schemaResolver) SetUserPublicRepos(ctx context.Context, args struct {
	UserID   graphql.ID
	RepoURIs []string
}) (*EmptyResponse, error) {
	if !envvar.SourcegraphDotComMode() {
		return nil, errors.Errorf("SetUserPublicRepos is not supported on instances without SOURCEGRAPHDOTCOM_MODE=true")
	}
	if len(args.RepoURIs) > maxUserPublicRepos {
		return nil, errors.Errorf("Too many repository URLs, please specify %v or fewer", maxUserPublicRepos)
	}
	var (
		repoStore = database.Repos(r.db)
		uprs      = database.UserPublicRepos(r.db)
		repos     = make([]database.UserPublicRepo, len(args.RepoURIs))
		eg        errgroup.Group
	)
	userID, err := UnmarshalUserID(args.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshaling User ID")
	}
	for i, repoURI := range args.RepoURIs {
		i, repoURI := i, repoURI
		eg.Go(func() error {
			repo, err := getRepo(ctx, repoStore, repoURI)
			if err != nil {
				return errors.Wrapf(err, "getting ID for repo %s", repoURI)
			}
			repos[i] = database.UserPublicRepo{
				UserID:  userID,
				RepoURI: repo.URI,
				RepoID:  repo.ID,
			}
			return nil
		})
	}
	err = eg.Wait()
	if err != nil {
		return nil, err
	}
	err = uprs.SetUserRepos(ctx, userID, repos)
	if err != nil {
		return nil, errors.Wrap(err, "Updating list of public repos")
	}
	return &EmptyResponse{}, nil
}

// getRepo attempts to find a repo in the database by URI, returning the ID if it's found. If it's not found
// it will use RepoLookup on repo-updater to fetch the repo info from a code host, store it in the repos table,
// enqueue a clone for that repo, and return the repo ID
func getRepo(ctx context.Context, repoStore *database.RepoStore, repoURI string) (repo *types.Repo, err error) {
	u, err := url.Parse(repoURI)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to parse repository URL "+repoURI)
	}

	var repoName = gitserverproto.NormalizeRepo(api.RepoName(u.Host + u.Path))

	if !strings.HasPrefix(string(repoName), "github.com") && !strings.HasPrefix(string(repoName), "gitlab.com") {
		return nil, errors.Errorf("Unable to add non-GitHub.com or GitLab.com repository: " + repoURI)
	}

	// if the repo exists we always want to enqueue an update, so the user can search an up to date version of the repo
	defer func() {
		if err != nil {
			return
		}
		_, err = repoupdater.DefaultClient.EnqueueRepoUpdate(ctx, repoName)
	}()

	// the repo may already exist, try looking it up by name (host + path)
	//
	// note: if the user provides the URL without a scheme (eg just 'github.com/foo/bar')
	// the host is '', but the path contains the host instead, so this works both ways 😅
	repo, err = repoStore.GetByName(ctx, repoName)
	if err != nil && !database.IsRepoNotFoundErr(err) {
		return nil, errors.Wrap(err, "Error checking if repo exists already")
	} else if repo != nil {
		// repo already exists, nice.
		return repo, nil
	}

	// the repo doesn't exist yet, let's look it up and enqueue a clone, and store the ID
	res, err := repoupdater.DefaultClient.RepoLookup(
		ctx,
		repoupdaterproto.RepoLookupArgs{Repo: repoName},
	)
	if err != nil {
		return nil, errors.Wrap(err, "looking up repo on remote host")
	}
	if res.Repo == nil {
		return nil, fmt.Errorf("unable to find repo %s", repoURI)
	}

	repoName = res.Repo.Name

	// it was able to find a repo, but annoyingly doesn't return the repo ID.
	// getting the repo ID out of RepoLookup is non-trivial, so instead we'll
	// just look the repo up by name. janky.
	repo, err = repoStore.GetByName(ctx, repoName)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't find repo after fetching from code host")
	}
	return repo, err
}
