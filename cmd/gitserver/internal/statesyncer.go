package internal

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	repoSyncStateCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repo_sync_state_counter",
		Help: "Incremented each time we check the state of repo",
	}, []string{"type"})
	repoStateUpsertCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repo_sync_state_upsert_counter",
		Help: "Incremented each time we upsert repo state in the database",
	}, []string{"success"})
)

// NewRepoStateSyncer returns a periodic goroutine that syncs state on disk to the
// database for all repos. We perform a full sync if the known gitserver addresses
// has changed since the last run. Otherwise, we only sync repos that have not yet
// been assigned a shard.
func NewRepoStateSyncer(
	ctx context.Context,
	logger log.Logger,
	db database.DB,
	locker RepositoryLocker,
	shardID string,
	fs gitserverfs.FS,
	interval time.Duration,
	batchSize int,
	perSecond int,
) goroutine.BackgroundRoutine {
	var previousAddrs string
	var previousPinned string
	fullSync := true

	return goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(ctx),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			gitServerAddrs := gitserver.NewGitserverAddresses(conf.Get())
			addrs := gitServerAddrs.Addresses
			// We turn addrs into a string here for easy comparison and storage of previous
			// addresses since we'd need to take a copy of the slice anyway.
			currentAddrs := strings.Join(addrs, ",")
			// If the addresses changed, we need to do a full sync.
			fullSync = fullSync || currentAddrs != previousAddrs
			previousAddrs = currentAddrs

			// We turn PinnedServers into a string here for easy comparison and storage
			// of previous pins.
			pinnedServerPairs := make([]string, 0, len(gitServerAddrs.PinnedServers))
			for k, v := range gitServerAddrs.PinnedServers {
				pinnedServerPairs = append(pinnedServerPairs, fmt.Sprintf("%s=%s", k, v))
			}
			sort.Strings(pinnedServerPairs)
			currentPinned := strings.Join(pinnedServerPairs, ",")
			// If the pinned repos changed, we need to do a full sync.
			fullSync = fullSync || currentPinned != previousPinned
			previousPinned = currentPinned

			if err := syncRepoState(ctx, logger, db, locker, shardID, fs, gitServerAddrs, batchSize, perSecond, fullSync); err != nil {
				// after a failed full sync, we should attempt it again in the next
				// invocation.
				fullSync = true
				return errors.Wrap(err, "syncing repo state")
			}

			// Last full sync was a success, so next time we can be more optimistic.
			fullSync = false

			return nil
		}),
		goroutine.WithName("gitserver.repo-state-syncer"),
		goroutine.WithDescription("syncs repo state on disk with the gitserver_repos table"),
		goroutine.WithInterval(interval),
	)
}

func syncRepoState(
	ctx context.Context,
	logger log.Logger,
	db database.DB,
	locker RepositoryLocker,
	shardID string,
	fs gitserverfs.FS,
	gitServerAddrs gitserver.GitserverAddresses,
	batchSize int,
	perSecond int,
	fullSync bool,
) error {
	logger.Debug("starting syncRepoState", log.Bool("fullSync", fullSync))
	addrs := gitServerAddrs.Addresses

	// When fullSync is true we'll scan all repos in the database and ensure we set
	// their clone state and assign any that belong to this shard with the correct
	// shard_id.
	//
	// When fullSync is false, we assume that we only need to check repos that have
	// not yet had their shard_id allocated.

	// Sanity check our host exists in addrs before starting any work
	var found bool
	for _, a := range addrs {
		if hostnameMatch(shardID, a) {
			found = true
			break
		}
	}
	if !found {
		return errors.Errorf("gitserver hostname, %q, not found in list", shardID)
	}

	// The rate limit should be enforced across all instances
	perSecond = perSecond / len(addrs)
	if perSecond < 0 {
		perSecond = 1
	}
	limiter := ratelimit.NewInstrumentedLimiter("SyncRepoState", rate.NewLimiter(rate.Limit(perSecond), perSecond))

	// The rate limiter doesn't allow writes that are larger than the burst size
	// which we've set to perSecond.
	if batchSize > perSecond {
		batchSize = perSecond
	}

	batch := make([]*types.GitserverRepo, 0)

	writeBatch := func() {
		if len(batch) == 0 {
			return
		}
		// We always clear the batch
		defer func() {
			batch = batch[0:0]
		}()
		err := limiter.WaitN(ctx, len(batch))
		if err != nil {
			logger.Error("Waiting for rate limiter", log.Error(err))
			return
		}

		if err := db.GitserverRepos().Update(ctx, batch...); err != nil {
			repoStateUpsertCounter.WithLabelValues("false").Add(float64(len(batch)))
			logger.Error("Updating GitserverRepos", log.Error(err))
			return
		}
		repoStateUpsertCounter.WithLabelValues("true").Add(float64(len(batch)))
	}

	// Make sure we fetch at least a good chunk of records, assuming that most
	// would not need an update anyways. Don't fetch too many though to keep the
	// DB load at a reasonable level and constrain memory usage.
	iteratePageSize := batchSize * 2
	if iteratePageSize < 500 {
		iteratePageSize = 500
	}

	options := database.IterateRepoGitserverStatusOptions{
		// We also want to include deleted repos as they may still be cloned on disk
		IncludeDeleted:   true,
		BatchSize:        iteratePageSize,
		OnlyWithoutShard: !fullSync,
	}
	for {
		repos, nextRepo, err := db.GitserverRepos().IterateRepoGitserverStatus(ctx, options)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			repoSyncStateCounter.WithLabelValues("check").Inc()

			// Ensure we're only dealing with repos we are responsible for.
			addr := gitServerAddrs.AddrForRepo(ctx, repo.Name)
			if !hostnameMatch(shardID, addr) {
				repoSyncStateCounter.WithLabelValues("other_shard").Inc()
				continue
			}
			repoSyncStateCounter.WithLabelValues("this_shard").Inc()

			cloned, err := fs.RepoCloned(repo.Name)
			if err != nil {
				// Failed to determine cloned state, we have to skip this record for now.
				continue
			}
			_, cloning := locker.Status(repo.Name)

			var shouldUpdate bool
			if repo.ShardID != shardID {
				repo.ShardID = shardID
				shouldUpdate = true
			}
			cloneStatus := cloneStatus(cloned, cloning)
			if repo.CloneStatus != cloneStatus {
				repo.CloneStatus = cloneStatus
				// Since the repo has been recloned or is being cloned
				// we can reset the corruption
				repo.CorruptedAt = time.Time{}
				shouldUpdate = true
			}

			if !shouldUpdate {
				continue
			}

			batch = append(batch, repo.GitserverRepo)

			if len(batch) >= batchSize {
				writeBatch()
			}
		}

		if nextRepo == 0 {
			break
		}

		options.NextCursor = nextRepo
	}

	// Attempt final write
	writeBatch()

	return nil
}
