package backend

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/inventory"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type ReposService interface {
	Get(ctx context.Context, repo api.RepoID) (*types.Repo, error)
	GetByName(ctx context.Context, name api.RepoName) (*types.Repo, error)
	List(ctx context.Context, opt database.ReposListOptions) ([]*types.Repo, error)
	ListIndexable(ctx context.Context) ([]types.MinimalRepo, error)
	GetInventory(ctx context.Context, repoName api.RepoName, commitID api.CommitID, forceEnhancedLanguageDetection bool) (*inventory.Inventory, error)
	RecloneRepository(ctx context.Context, repoID api.RepoID) error
	ResolveRev(ctx context.Context, repo api.RepoName, rev string) (api.CommitID, error)
}

// NewRepos uses the provided `database.DB` to initialize a new RepoService.
func NewRepos(logger log.Logger, db database.DB, client gitserver.Client) ReposService {
	repoStore := db.Repos()
	logger = logger.Scoped("repos")
	return &repos{
		logger:          logger,
		db:              db,
		gitserverClient: client,
		store:           repoStore,
	}
}

type repos struct {
	logger          log.Logger
	db              database.DB
	gitserverClient gitserver.Client
	store           database.RepoStore
}

func (s *repos) Get(ctx context.Context, repo api.RepoID) (_ *types.Repo, err error) {
	if Mocks.Repos.Get != nil {
		return Mocks.Repos.Get(ctx, repo)
	}

	ctx, done := startTrace(ctx, "Get", repo, &err)
	defer done()

	return s.store.Get(ctx, repo)
}

// GetByName retrieves the repository with the given name.
func (s *repos) GetByName(ctx context.Context, name api.RepoName) (_ *types.Repo, err error) {
	if Mocks.Repos.GetByName != nil {
		return Mocks.Repos.GetByName(ctx, name)
	}

	ctx, done := startTrace(ctx, "GetByName", name, &err)
	defer done()

	return s.store.GetByName(ctx, name)
}

func (s *repos) List(ctx context.Context, opt database.ReposListOptions) (repos []*types.Repo, err error) {
	if Mocks.Repos.List != nil {
		return Mocks.Repos.List(ctx, opt)
	}

	ctx, done := startTrace(ctx, "List", opt, &err)
	defer func() {
		if err == nil {
			trace.FromContext(ctx).SetAttributes(
				attribute.Int("result.len", len(repos)),
			)
		}
		done()
	}()

	return s.store.List(ctx, opt)
}

// ListIndexable calls database.ListMinimalRepos, with tracing. It lists ALL
// indexable repos. In addition, it only lists cloned repositories.
//
// The intended call site for this is the logic which assigns repositories to
// zoekt shards.
func (s *repos) ListIndexable(ctx context.Context) (repos []types.MinimalRepo, err error) {
	ctx, done := startTrace(ctx, "ListIndexable", nil, &err)
	defer func() {
		if err == nil {
			trace.FromContext(ctx).SetAttributes(
				attribute.Int("result.len", len(repos)),
			)
		}
		done()
	}()

	return s.store.ListMinimalRepos(ctx, database.ReposListOptions{
		OnlyCloned: true,
	})
}

var getInventoryTimeout, _ = strconv.Atoi(env.Get("GET_INVENTORY_TIMEOUT", "5", "Time in minutes before cancelling getInventory requests. Raise this if your repositories are large and need a long time to process."))

func (s *repos) GetInventory(ctx context.Context, repo api.RepoName, commitID api.CommitID, forceEnhancedLanguageDetection bool) (res *inventory.Inventory, err error) {
	if Mocks.Repos.GetInventory != nil {
		return Mocks.Repos.GetInventory(ctx, repo, commitID)
	}

	if commitID == "" {
		return nil, errors.Errorf("Could not find a commit in the repository. Please add a file first.")
	}

	ctx, done := startTrace(ctx, "GetInventory", map[string]any{"repo": repo, "commitID": commitID}, &err)
	defer done()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(getInventoryTimeout)*time.Minute)
	defer cancel()

	invCtx, err := InventoryContext(s.logger, repo, s.gitserverClient, commitID, forceEnhancedLanguageDetection)
	if err != nil {
		return nil, err
	}

	// In computing the inventory, sub-tree inventories are cached based on the OID of the Git
	// tree. Compared to per-blob caching, this creates many fewer cache entries, which means fewer
	// stores, fewer lookups, and less cache storage overhead. Compared to per-commit caching, this
	// yields a higher cache hit rate because most trees are unchanged across commits.
	inv, err := invCtx.All(ctx)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *repos) RecloneRepository(ctx context.Context, repoID api.RepoID) (err error) {
	if Mocks.Repos.RecloneRepository != nil {
		return Mocks.Repos.RecloneRepository(ctx, repoID)
	}

	repo, err := s.Get(ctx, repoID)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error while fetching repo with ID %d", repoID))
	}

	ctx, done := startTrace(ctx, "RecloneRepository", repoID, &err)
	defer done()

	return repoupdater.DefaultClient.RecloneRepository(ctx, repo.Name)
}

// ResolveRev will return the absolute commit for a commit-ish spec in a repo.
// If no rev is specified, HEAD is used.
// Error cases:
// * Repo does not exist: gitdomain.RepoNotExistError
// * Commit does not exist: gitdomain.RevisionNotFoundError
// * Empty repository: gitdomain.RevisionNotFoundError
// * The user does not have permission: errcode.IsNotFound
// * Other unexpected errors.
func (s *repos) ResolveRev(ctx context.Context, repo api.RepoName, rev string) (commitID api.CommitID, err error) {
	if Mocks.Repos.ResolveRev != nil {
		return Mocks.Repos.ResolveRev(ctx, repo, rev)
	}

	ctx, done := startTrace(ctx, "ResolveRev", map[string]any{"repo": repo, "rev": rev}, &err)
	defer done()

	return s.gitserverClient.ResolveRevision(ctx, repo, rev, gitserver.ResolveRevisionOptions{EnsureRevision: true})
}

// ErrRepoSeeOther indicates that the repo does not exist on this server but might exist on an external Sourcegraph
// server.
type ErrRepoSeeOther struct {
	// RedirectURL is the base URL for the repository at an external location.
	RedirectURL string
}

func (e ErrRepoSeeOther) Error() string {
	return fmt.Sprintf("repo not found at this location, but might exist at %s", e.RedirectURL)
}
