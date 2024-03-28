package background

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/own"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func handleAnalytics(ctx context.Context, lgr log.Logger, repoId api.RepoID, db database.DB) error {
	// 🚨 SECURITY: we use the internal actor because the background indexer is not associated with any user,
	// and needs to see all repos and files.
	internalCtx := actor.WithInternalActor(ctx)
	indexer := newAnalyticsIndexer(gitserver.NewClient("own.analyticsindexer"), db, lgr)
	err := indexer.indexRepo(internalCtx, repoId, authz.DefaultSubRepoPermsChecker)
	if err != nil {
		lgr.Error("own analytics indexing failure", log.String("msg", err.Error()))
	}
	return err
}

type analyticsIndexer struct {
	client gitserver.Client
	db     database.DB
	logger log.Logger
}

func newAnalyticsIndexer(client gitserver.Client, db database.DB, lgr log.Logger) *analyticsIndexer {
	return &analyticsIndexer{client: client, db: db, logger: lgr}
}

var ownAnalyticsFilesCounter = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "src",
	Name:      "own_analytics_files_indexed_total",
})

func (r *analyticsIndexer) indexRepo(ctx context.Context, repoId api.RepoID, checker authz.SubRepoPermissionChecker) error {
	// If the repo has sub-repo perms enabled, skip indexing
	isSubRepoPermsRepo, err := authz.SubRepoEnabledForRepoID(ctx, checker, repoId)
	if err != nil {
		return errcode.MakeNonRetryable(err)
	} else if isSubRepoPermsRepo {
		r.logger.Debug("skipping own contributor signal due to the repo having subrepo perms enabled", log.Int32("repoID", int32(repoId)))
		return nil
	}

	repoStore := r.db.Repos()
	repo, err := repoStore.Get(ctx, repoId)
	if err != nil {
		return errors.Wrap(err, "repoStore.Get")
	}
	// Try to compute ownership stats
	commitID, err := r.client.ResolveRevision(ctx, repo.Name, "HEAD", gitserver.ResolveRevisionOptions{NoEnsureRevision: true})
	if err != nil {
		return errcode.MakeNonRetryable(errors.Wrapf(err, "cannot resolve HEAD"))
	}
	files, err := r.client.ReadDir(ctx, repo.Name, commitID, "", true)
	if err != nil {
		return errors.Wrap(err, "ls-tree")
	}
	isOwnedViaCodeowners := r.codeowners(ctx, repo, commitID)
	isOwnedViaAssignedOwnership := r.assignedOwners(ctx, repo, commitID)
	var totalCount int
	var ownCounts database.PathAggregateCounts
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		totalCount++
		countCodeowners := isOwnedViaCodeowners(f.Name())
		countAssignedOwnership := isOwnedViaAssignedOwnership(f.Name())
		if countCodeowners {
			ownCounts.CodeownedFileCount++
		}
		if countAssignedOwnership {
			ownCounts.AssignedOwnershipFileCount++
		}
		if countCodeowners || countAssignedOwnership {
			ownCounts.TotalOwnedFileCount++
		}
	}
	timestamp := time.Now()
	totalFileCountUpdate := rootPathIterator[int]{value: totalCount}
	rowCount, err := r.db.RepoPaths().UpdateFileCounts(ctx, repo.ID, totalFileCountUpdate, timestamp)
	if err != nil {
		return errors.Wrap(err, "UpdateFileCounts")
	}
	if rowCount == 0 {
		return errors.New("expected total file count updates")
	}
	codeownedCountUpdate := rootPathIterator[database.PathAggregateCounts]{value: ownCounts}
	rowCount, err = r.db.OwnershipStats().UpdateAggregateCounts(ctx, repo.ID, codeownedCountUpdate, timestamp)
	if err != nil {
		return errors.Wrap(err, "UpdateAggregateCounts")
	}
	if rowCount == 0 {
		return errors.New("expected CODEOWNERS-owned file count update")
	}
	ownAnalyticsFilesCounter.Add(float64(totalCount))
	return nil
}

// codeowners pulls a path matcher for repo HEAD.
// If result function is nil, then no CODEOWNERS file was found.
func (r *analyticsIndexer) codeowners(ctx context.Context, repo *types.Repo, commitID api.CommitID) func(string) bool {
	ownService := own.NewService(r.client, r.db)
	ruleset, err := ownService.RulesetForRepo(ctx, repo.Name, repo.ID, commitID)
	if ruleset == nil || err != nil {
		// TODO(#53155): Return error in case there is an issue,
		// but return noRuleset and no error if CODEOWNERS is not found.
		return noOwners
	}
	return func(path string) bool {
		rule := ruleset.Match(path)
		owners := rule.GetOwner()
		return len(owners) > 0
	}
}

func (r *analyticsIndexer) assignedOwners(ctx context.Context, repo *types.Repo, commitID api.CommitID) func(string) bool {
	ownService := own.NewService(r.client, r.db)
	assignedOwners, err := own.NewService(r.client, r.db).AssignedOwnership(ctx, repo.ID, commitID)
	if err != nil {
		// TODO(#53155): Return error in case there is an issue,
		// but return noRuleset and no error if CODEOWNERS is not found.
		return noOwners
	}
	assignedTeams, err := ownService.AssignedTeams(ctx, repo.ID, commitID)
	if err != nil {
		// TODO(#53155): Return error in case there is an issue,
		// but return noRuleset and no error if CODEOWNERS is not found.
		return noOwners
	}
	return func(path string) bool {
		return len(assignedOwners.Match(path)) > 0 || len(assignedTeams.Match(path)) > 0
	}
}

// For proto it is safe to return nil from a function,
// since the implementation handles a nil reference gracefully.
// Just need to use getters instead of field access.
func noOwners(string) bool {
	return false
}

type rootPathIterator[T any] struct {
	value T
}

func (i rootPathIterator[T]) Iterate(f func(path string, value T) error) error {
	return f("", i.value)
}
