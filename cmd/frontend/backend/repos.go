package backend

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/env"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/inventory"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbcache"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
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
//
// NOTE: The underlying cache is reused from Repos global variable to actually
// make cache be useful. This is mostly a workaround for now until we come up a
// more idiomatic solution.
func NewRepos(logger log.Logger, db database.DB, client gitserver.Client) ReposService {
	repoStore := db.Repos()
	logger = logger.Scoped("repos")
	return &repos{
		logger:          logger,
		db:              db,
		gitserverClient: client,
		store:           repoStore,
		cache:           dbcache.NewIndexableReposLister(logger, repoStore),
	}
}

type repos struct {
	logger          log.Logger
	db              database.DB
	gitserverClient gitserver.Client
	cf              httpcli.Doer
	store           database.RepoStore
	cache           *dbcache.IndexableReposLister
}

func (s *repos) Get(ctx context.Context, repo api.RepoID) (_ *types.Repo, err error) {
	if Mocks.Repos.Get != nil {
		return Mocks.Repos.Get(ctx, repo)
	}

	ctx, done := startTrace(ctx, "Get", repo, &err)
	defer done()

	return s.store.Get(ctx, repo)
}

// GetByName retrieves the repository with the given name. It will lazy sync a repo
// not yet present in the database under certain conditions. See repos.Syncer.SyncRepo.
func (s *repos) GetByName(ctx context.Context, name api.RepoName) (_ *types.Repo, err error) {
	if Mocks.Repos.GetByName != nil {
		return Mocks.Repos.GetByName(ctx, name)
	}

	ctx, done := startTrace(ctx, "GetByName", name, &err)
	defer done()

	repo, err := s.store.GetByName(ctx, name)
	if err == nil {
		return repo, nil
	}

	if !errcode.IsNotFound(err) {
		return nil, err
	}

	if errcode.IsNotFound(err) && !dotcom.LazilySyncsIndefinitelyManyRepositories() {
		// The repo doesn't exist and we should not lazy clone it.
		return nil, err
	}

	newName, err := s.addRepoToSourcegraphDotCom(ctx, name)
	if err != nil {
		return nil, err
	}

	return s.store.GetByName(ctx, newName)

}

// addRepoToSourcegraphDotCom adds the repository with the given name to the database by calling
// repo-updater when in sourcegraph.com mode. It's possible that the repo has
// been renamed on the code host in which case a different name may be returned.
// name is assumed to not exist as a repo in the database.
func (s *repos) addRepoToSourcegraphDotCom(ctx context.Context, name api.RepoName) (addedName api.RepoName, err error) {
	ctx, done := startTrace(ctx, "Add", name, &err)
	defer done()

	// Avoid hitting repo-updater (and incurring a hit against our GitHub/etc. API rate
	// limit) for repositories that don't exist or private repositories that people attempt to
	// access.
	codehost := extsvc.CodeHostOf(name, extsvc.PublicCodeHosts...)
	if codehost == nil {
		return "", &database.RepoNotFoundErr{Name: name}
	}

	// Verify repo exists and is cloneable publicly before continuing to put load
	// on repo-updater.
	// For package hosts, we have no good metric to figure this out at the moment.
	if !codehost.IsPackageHost() {
		if err := s.isGitRepoPubliclyCloneable(ctx, name); err != nil {
			return "", err
		}
	}

	// Looking up the repo in repo-updater makes it sync that repo to the
	// database on sourcegraph.com if that repo is from github.com or gitlab.com
	lookupResult, err := repoupdater.DefaultClient.RepoLookup(ctx, protocol.RepoLookupArgs{Repo: name})
	if lookupResult != nil && lookupResult.Repo != nil {
		return lookupResult.Repo.Name, err
	}
	return "", err
}

var metricIsRepoCloneable = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_frontend_repo_add_is_cloneable",
	Help: "temporary metric to measure if this codepath is valuable on sourcegraph.com",
}, []string{"status"})

// isGitRepoPubliclyCloneable checks if a git repo with the given name would be
// cloneable without auth - ie. if sourcegraph.com could clone it with a cloud_default
// external service. This is explicitly without any auth, so we don't consume
// any API rate limit, since many users visit private or bogus repos.
// We deduce the unauthenticated clone URL from the repo name by simply adding .git
// to it.
// Name is verified by the caller to be for either of our public cloud default
// hosts.
func (s *repos) isGitRepoPubliclyCloneable(ctx context.Context, name api.RepoName) error {
	// This is on the request path, don't block for too long if upstream is struggling.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	status := "unknown"
	defer func() {
		metricIsRepoCloneable.WithLabelValues(status).Inc()
	}()

	// Speak git smart protocol to check if repo exists without cloning.
	remoteURL, err := vcs.ParseURL("https://" + string(name) + ".git/info/refs?service=git-upload-pack")
	if err != nil {
		// No idea how to construct a remote URL for this repo, bail.
		return &database.RepoNotFoundErr{Name: api.RepoName(name)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL.String(), nil)
	if err != nil {
		return errors.Wrap(err, "failed to construct request to check if repository exists")
	}

	cf := httpcli.ExternalDoer
	if s.cf != nil {
		cf = s.cf
	}

	resp, err := cf.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to check if repository exists")
	}

	// No interest in the response body.
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if ctx.Err() != nil {
			status = "timeout"
		} else {
			status = "fail"
		}
		// Not cloneable without auth.
		return &database.RepoNotFoundErr{Name: api.RepoName(name)}
	}

	status = "success"

	return nil
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

	if dotcom.LazilySyncsIndefinitelyManyRepositories() {
		return s.cache.List(ctx)
	}

	return s.store.ListMinimalRepos(ctx, database.ReposListOptions{
		OnlyCloned: true,
	})
}

var getInventoryTimeout, _ = strconv.Atoi(env.Get("GET_INVENTORY_TIMEOUT", "5", "Time in minutes before cancelling getInventory requests. Raise this if your repositories are large and need a long time to process."))

func (s *repos) GetInventory(ctx context.Context, repo api.RepoName, commitID api.CommitID, forceEnhancedLanguageDetection bool) (res *inventory.Inventory, err error) {
	if Mocks.Repos.GetInventory != nil {
		return Mocks.Repos.GetInventory(ctx, repo, commitID)
	}

	ctx, done := startTrace(ctx, "GetInventory", map[string]any{"repo": repo, "commitID": commitID}, &err)
	defer done()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(getInventoryTimeout)*time.Minute)
	defer cancel()

	invCtx, err := InventoryContext(s.logger, repo, s.gitserverClient, commitID, forceEnhancedLanguageDetection)
	if err != nil {
		return nil, err
	}

	root, err := s.gitserverClient.Stat(ctx, repo, commitID, "")
	if err != nil {
		return nil, err
	}

	// In computing the inventory, sub-tree inventories are cached based on the OID of the Git
	// tree. Compared to per-blob caching, this creates many fewer cache entries, which means fewer
	// stores, fewer lookups, and less cache storage overhead. Compared to per-commit caching, this
	// yields a higher cache hit rate because most trees are unchanged across commits.
	inv, err := invCtx.Entries(ctx, root)
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
