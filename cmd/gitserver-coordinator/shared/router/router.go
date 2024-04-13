package router

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type RepositoryRouter interface {
	Route(ctx context.Context, repo *proto.GitserverRepository) (*proto.GitserverRepository, string, error)
	RouteLegacy(ctx context.Context, repoName api.RepoName) (*proto.GitserverRepository, string, error)
}

func NewLegacyRouter(db database.DB) RepositoryRouter {
	return &legacyRouter{db: db}
}

type legacyRouter struct {
	db database.DB
}

func (r *legacyRouter) Route(ctx context.Context, repo *proto.GitserverRepository) (_ *proto.GitserverRepository, _ string, err error) {
	tr, ctx := trace.New(ctx, "Route")
	defer tr.EndWithErr(&err)

	// Routing decisions should not need to run against repo permissions.
	ctx = actor.WithInternalActor(ctx)

	addrs := gitserver.NewGitserverAddresses(conf.Get())

	if repo.GetUid() == "" {
		return nil, "", errors.Newf("no uid given for repo")
	}

	id, err := strconv.Atoi(repo.GetUid())
	if err != nil {
		return nil, "", errors.Wrap(err, "invalid uid")
	}

	dbrs, err := r.db.Repos().ListMinimalRepos(ctx, database.ReposListOptions{
		IDs: []api.RepoID{api.RepoID(id)},
	})
	if err != nil {
		return nil, "", err
	}

	if len(dbrs) == 0 {
		return nil, "", errors.Newf("repo %d not found", id)
	}

	dbr := dbrs[0]

	return &proto.GitserverRepository{
		Uid:  strconv.Itoa(int(dbr.ID)),
		Name: string(dbr.Name),
		Path: repoDirFromName(dbr.Name),
	}, addrs.AddrForRepo(ctx, dbr.Name), nil
}

func (r *legacyRouter) RouteLegacy(ctx context.Context, repoName api.RepoName) (_ *proto.GitserverRepository, _ string, err error) {
	tr, ctx := trace.New(ctx, "RouteLegacy")
	defer tr.EndWithErr(&err)

	// Routing decisions should not need to run against repo permissions.
	ctx = actor.WithInternalActor(ctx)

	addrs := gitserver.NewGitserverAddresses(conf.Get())

	gr, err := r.db.GitserverRepos().GetByName(ctx, repoName)
	if err != nil {
		return nil, "", err
	}

	return &proto.GitserverRepository{
		Uid:  strconv.Itoa(int(gr.RepoID)),
		Name: string(repoName),
		Path: repoDirFromName(repoName),
	}, addrs.AddrForRepo(ctx, repoName), nil
}

func repoDirFromName(name api.RepoName) string {
	// We need to use api.UndeletedRepoName(repo) for the name, as this is a name
	// transformation done on the database side that gitserver cannot know about.
	name = api.UndeletedRepoName(name)

	p := string(protocol.NormalizeRepo(name))
	return filepath.Join("/", filepath.FromSlash(p), ".git")
}
