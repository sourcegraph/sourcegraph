package janitor

import (
	"context"
	"math"
	"time"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/janitor/stats"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
)

// New creates a new janitor. This is a temporary routine that lives in gitserver
// that somewhat mimics the behavior of the old janitor by simply iterating over
// all the repositories and optimizing them. If we build coordinator, the per-repo
// optimization will be invoked by the coordinator via gRPC instead, and it will
// manage the queue of repos to optimize.
func New(logger log.Logger, concurrency int, fs gitserverfs.FS, getBackendFunc func(dir common.GitDir, repoName api.RepoName) git.GitBackend) *janitor {
	if concurrency < 1 {
		panic("concurrency must be at least 1")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &janitor{
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger.Scoped("janitor"),
		fs:             fs,
		getBackendFunc: getBackendFunc,
		concurrency:    concurrency,
		stopped:        make(chan struct{}),
	}
}

var _ goroutine.BackgroundRoutine = (*janitor)(nil)

type janitor struct {
	logger         log.Logger
	fs             gitserverfs.FS
	ctx            context.Context
	cancel         context.CancelFunc
	getBackendFunc func(dir common.GitDir, repoName api.RepoName) git.GitBackend
	concurrency    int
	stopped        chan struct{}
}

func (j *janitor) Start() {
	for {
		start := time.Now()
		p := pool.New().WithMaxGoroutines(j.concurrency)
		err := j.fs.ForEachRepo(func(name api.RepoName, dir common.GitDir) (done bool) {
			p.Go(func() {
				backend := j.getBackendFunc(dir, name)

				logger := j.logger.With(log.String("repo", string(name)))

				logger.Info("optimizing repository")

				ctx, cancel := context.WithTimeout(j.ctx, 2*time.Hour)
				defer cancel()

				// We always attempt to bring the repo into a good initial state.
				err := repairRepo(ctx, logger, backend, dir)
				if err != nil {
					logger.Error("failed to repair repository", log.String("repo", string(name)), log.Error(err))
					return
				}

				s, err := stats.RepositoryInfoForRepository(ctx, dir)
				if err != nil {
					logger.Error("failed to get repository stats, won't attempt to optimize repository", log.String("repo", string(name)), log.Error(err))
					return
				}

				now := time.Now()

				shouldRepack, repackCfg := shouldRepackObjects(logger, s, now)
				if shouldRepack {
					logger.Info("will repack objects")
					if err := RepackObjects(ctx, backend, dir, repackCfg); err != nil {
						logger.Error("failed to repack objects", log.String("repo", string(name)), log.Error(err))
						return
					}
				}

				shouldPrune, expiry := shouldPruneObjects(s, now)
				if shouldPrune {
					logger.Info("will prune objects")
					if err := backend.PruneObjects(ctx, expiry); err != nil {
						logger.Error("failed to prune objects", log.String("repo", string(name)), log.Error(err))
						return
					}
				}

				if shouldRepackReferences(s) {
					logger.Info("will repack references")
					if err := backend.PackRefs(ctx); err != nil {
						logger.Error("failed to pack refs", log.String("repo", string(name)), log.Error(err))
						return
					}
				}

				shouldWrite, replaceChain := shouldWriteCommitGraph(s, now)
				if shouldWrite {
					logger.Info("will write commit graph")
					err := backend.WriteCommitGraph(ctx, replaceChain)
					if err != nil {
						logger.Error("failed to write commit graph", log.String("repo", string(name)), log.Error(err))
						return
					}
				}

				// TODO: If we did any of the above, we might want to recalculate the repo size
				// and store the new size in the DB.

				return
			})

			return false
		})
		p.Wait()
		if err != nil {
			j.logger.Error("failed to iterate over repositories", log.Error(err))
		}

		j.logger.Info("Janitor run finished", log.Duration("duration", time.Since(start)))
		time.Sleep(1 * time.Minute)
	}
}

func (j *janitor) Stop() {
	close(j.stopped)
	j.cancel()
}

func shouldWriteCommitGraph(s stats.RepositoryInfo, now time.Time) (bool, bool) {
	// If the repository doesn't have any references at all then there is no point in writing
	// commit-graphs given that it would only contain reachable objects, of which there are
	// none.
	if s.References.LooseReferencesCount == 0 && s.References.PackedReferencesSize == 0 {
		return false, false
	}

	// When we have pruned objects in the repository then it may happen that the commit-graph
	// still refers to commits that have now been deleted. While this wouldn't typically cause
	// any issues during runtime, it may cause errors when explicitly asking for any commit that
	// does exist in the commit-graph, only. Furthermore, it causes git-fsck(1) to report that
	// the commit-graph is inconsistent.
	//
	// To fix this case we will replace the complete commit-chain when we have pruned objects
	// from the repository.
	if shouldPrune, _ := shouldPruneObjects(s, now); shouldPrune {
		return true, true
	}

	if commitGraphNeedsRewrite(s.CommitGraph) {
		return true, true
	}

	// When we repacked the repository then chances are high that we have accumulated quite some
	// objects since the last time we wrote a commit-graph.
	if needsRepacking, repackCfg := shouldRepackObjects(log.NoOp(), s, now); needsRepacking {
		// Same as with pruning: if we are repacking the repository and write cruft
		// packs with an expiry date then we may end up pruning objects. We thus
		// need to replace the commit-graph chain in that case.
		replaceChain := repackCfg.Strategy == RepackObjectsStrategyFullWithCruft && !repackCfg.CruftExpireBefore.IsZero()
		return true, replaceChain
	}

	return false, false
}

// commitGraphNeedsRewrite determines whether the commit-graph needs to be rewritten. This can be
// the case when it is either a monolithic commit-graph or when it is missing some extensions that
// only get written on a full rewrite.
func commitGraphNeedsRewrite(commitGraphInfo stats.CommitGraphInfo) bool {
	if commitGraphInfo.CommitGraphChainLength == 0 {
		// The repository does not have a commit-graph chain. This either indicates we ain't
		// got no commit-graph at all, or that it's monolithic. In both cases we want to
		// replace the commit-graph chain.
		return true
	} else if !commitGraphInfo.HasBloomFilters {
		// If the commit-graph-chain exists, we want to rewrite it in case we see that it
		// ain't got bloom filters enabled. This is because Git will refuse to write any
		// bloom filters as long as any of the commit-graph slices is missing this info.
		return true
	} else if !commitGraphInfo.HasGenerationData {
		// The same is true for generation data.
		return true
	}

	return false
}

const (
	// LooseObjectLimit is the limit of loose objects we accept both when doing incremental
	// repacks and when pruning objects.
	LooseObjectLimit = 1024
)

// ShouldPruneObjects determines whether the repository has stale objects that should be pruned.
// Object pools are never pruned to not lose data in them, but otherwise we prune when we've found
// enough stale objects that might in fact get pruned.
func shouldPruneObjects(s stats.RepositoryInfo, now time.Time) (bool, time.Time) {
	// When we have a number of loose objects that is older than two weeks then they have
	// surpassed the grace period and may thus be pruned.
	if s.LooseObjects.StaleCount <= LooseObjectLimit {
		return false, time.Time{}
	}

	return true, now.Add(stats.StaleObjectsGracePeriod)
}

const (
	// FullRepackCooldownPeriod is the cooldown period that needs to pass since the last full
	// repack before we consider doing another full repack.
	FullRepackCooldownPeriod = 5 * 24 * time.Hour
)

// ShouldRepackObjects checks whether the repository's objects need to be repacked. This uses a
// set of heuristics that scales with the size of the object database: the larger the repository,
// the less frequent does it get a full repack.
func shouldRepackObjects(logger log.Logger, s stats.RepositoryInfo, now time.Time) (bool, RepackObjectsConfig) {
	// If there are neither packfiles nor loose objects in this repository then there is no need
	// to repack anything.
	if s.Packfiles.Count == 0 && s.LooseObjects.Count == 0 {
		return false, RepackObjectsConfig{}
	}

	nonCruftPackfilesCount := s.Packfiles.Count - s.Packfiles.CruftCount
	timeSinceLastFullRepack := time.Since(s.Packfiles.LastFullRepack)

	fullRepackCfg := RepackObjectsConfig{
		// We want to be able to expire unreachable objects. We thus use cruft
		// packs with an expiry date.
		Strategy:    RepackObjectsStrategyFullWithCruft,
		WriteBitmap: true,
		// We rewrite all packfiles into a single one and thus change the layout
		// that was indexed by the multi-pack-index. We thus need to update it, as
		// well.
		WriteMultiPackIndex: true,
		CruftExpireBefore:   now.Add(stats.StaleObjectsGracePeriod),
	}

	geometricRepackCfg := RepackObjectsConfig{
		Strategy:    RepackObjectsStrategyGeometric,
		WriteBitmap: true,
		// We're rewriting packfiles that may be part of the multi-pack-index, so we
		// do want to update it to reflect the new layout.
		WriteMultiPackIndex: true,
	}

	// Incremental repacks only pack unreachable objects into a new pack. As we only
	// perform this kind of repack in the case where the overall repository structure
	// looks good to us we try to do use the least amount of resources to update them.
	// We thus neither update the multi-pack-index nor do we update bitmaps.
	incrementalRepackCfg := RepackObjectsConfig{
		Strategy:            RepackObjectsStrategyIncrementalWithUnreachable,
		WriteBitmap:         false,
		WriteMultiPackIndex: false,
	}

	// It is mandatory for us that we perform regular full repacks in repositories so
	// that we can evict objects which are unreachable into a separate cruft pack. So in
	// the case where we have more than one non-cruft packfiles and the time since our
	// last full repack is longer than the grace period we'll perform a full repack.
	//
	// This heuristic is simple on purpose: customers care about when objects will be
	// declared as unreachable and when the pruning grace period starts as it impacts
	// usage quotas. So with this simple policy we can tell customers that we evict and
	// expire unreachable objects on a regular schedule.
	//
	// On the other hand, for object pools, we also need to perform regular full
	// repacks. The reason is different though, as we don't ever delete objects from
	// pool repositories anyway.
	//
	// Geometric repacking does not take delta islands into account as it does not
	// perform a graph walk. We need proper delta islands though so that packfiles can
	// be efficiently served across forks of a repository.
	//
	// Once a full repack has been performed, the deltas will be carried forward even
	// across geometric repacks. That being said, the quality of our delta islands will
	// regress over time as new objects are pulled into the pool repository.
	//
	// So we perform regular full repacks in the repository to ensure that the delta
	// islands will be "freshened" again. If geometric repacks ever learn to take delta
	// islands into account we can get rid of this condition and only do geometric
	// repacks.
	if nonCruftPackfilesCount > 1 && timeSinceLastFullRepack > FullRepackCooldownPeriod {
		// TODO: Here we should also consider if the repo changed at all since the last
		// repack.
		logger.Info("performing a full repack of objects, last full repack is long ago", log.Int("non_cruft_count", int(nonCruftPackfilesCount)))
		return true, fullRepackCfg
	}

	// In case both packfiles and loose objects are in a good state, but we don't yet
	// have a multi-pack-index we perform an incremental repack to generate one. We need
	// to have multi-pack-indices for the next heuristic, so it's bad if it was missing.
	if !s.Packfiles.MultiPackIndex.Exists {
		logger.Info("performing a geometric repack of objects, no MIDX exists")
		return true, geometricRepackCfg
	}

	// Last but not least, we also need to take into account whether new packfiles have
	// been written into the repository since our last geometric repack. This is
	// necessary so that we can enforce the geometric sequence of packfiles and to make
	// sure that the multi-pack-index tracks those new packfiles.
	//
	// To calculate this we use the number of packfiles tracked by the multi-pack index:
	// the difference between the total number of packfiles and the number of packfiles
	// tracked by the index is the amount of packfiles written since the last geometric
	// repack. As we only update the MIDX during housekeeping this metric should in
	// theory be accurate.
	//
	// Theoretically, we could perform a geometric repack whenever there is at least one
	// untracked packfile as git-repack(1) would exit early in case it finds that the
	// geometric sequence is kept. But there are multiple reasons why we want to avoid
	// this:
	//
	// - We would end up spawning git-repack(1) on almost every single repository
	//   optimization, but ideally we want to be lazy and do only as much work as is
	//   really required.
	//
	// - While we wouldn't need to repack objects in case the geometric sequence is kept
	//   anyway, we'd still need to update the multi-pack-index. This action scales with
	//   the number of overall objects in the repository.
	//
	// Instead, we use a strategy that heuristically determines whether the repository
	// has too many untracked packfiles and scale the number with the combined size of
	// all packfiles. The intent is to perform geometric repacks less often the larger
	// the repository, also because larger repositories tend to be more active, too.
	//
	// The formula we use is:
	//
	//	log(total_packfile_size) / log(1.8)
	//
	// Which gives us the following allowed number of untracked packfiles:
	//
	// -----------------------------------------------------
	// | total packfile size | allowed untracked packfiles |
	// -----------------------------------------------------
	// | none or <10MB       |  2                          |
	// | 10MB                |  3                          |
	// | 100MB               |  7                          |
	// | 500MB               | 10                          |
	// | 1GB                 | 11                          |
	// | 5GB                 | 14                          |
	// | 10GB                | 15                          |
	// | 100GB               | 19                          |
	// -----------------------------------------------------
	allowedLowerLimit := 2.0
	allowedUpperLimit := math.Log(float64(s.Packfiles.Size/1024/1024)) / math.Log(1.8)
	actualLimit := math.Max(allowedLowerLimit, allowedUpperLimit)

	untrackedPackfiles := s.Packfiles.Count - s.Packfiles.MultiPackIndex.PackfileCount

	if untrackedPackfiles > uint64(actualLimit) {
		logger.Info(
			"performing a geometric repack of objects, the amount of packfiles exceeds the threshold",
			log.Int("untracked_packfiles", int(untrackedPackfiles)),
			log.Int("allowed_limit", int(actualLimit)),
			log.Float64("packfiles_size", float64(s.Packfiles.Size/1024/1024)),
		)
		return true, geometricRepackCfg
	}

	// If there are loose objects then we want to roll them up into a new packfile.
	// Loose objects naturally accumulate during day-to-day operations, e.g. when
	// executing RPCs part of the OperationsService which write objects into the repo
	// directly.
	//
	// As we have already verified that the packfile structure looks okay-ish to us, we
	// don't need to perform a geometric repack here as that could be expensive: we
	// might end up soaking up packfiles because the geometric sequence is not intact,
	// but more importantly we would end up writing the multi-pack-index and potentially
	// a bitmap. Writing these data structures introduces overhead that scales with the
	// number of objects in the repository.
	//
	// So instead, we only do an incremental repack of all loose objects, regardless of
	// their reachability. This is the cheapest we can do: we don't need to compute
	// whether objects are reachable and we don't need to update any data structures
	// that scale with the repository size.
	if s.LooseObjects.Count > LooseObjectLimit {
		logger.Info("performing an incremental repack of objects, too many loose objects", log.Int("looose_object_count", int(s.LooseObjects.Count)))
		return true, incrementalRepackCfg
	}

	return false, RepackObjectsConfig{}
}

// RepackObjectsStrategy defines how objects shall be repacked.
type RepackObjectsStrategy string

const (
	// RepackObjectsStrategyIncrementalWithUnreachable performs an incremental repack by writing
	// all loose objects into a new packfile, regardless of their reachability. The loose
	// objects will be deleted.
	RepackObjectsStrategyIncrementalWithUnreachable = RepackObjectsStrategy("incremental_with_unreachable")
	// RepackObjectsStrategyFullWithCruft performs a full repack by writing all reachable
	// objects into a new packfile. Unreachable objects will be written into a separate cruft
	// packfile.
	RepackObjectsStrategyFullWithCruft = RepackObjectsStrategy("full_with_cruft")
	// RepackObjectsStrategyGeometric performs an geometric repack. This strategy will repack
	// packfiles so that the resulting pack structure forms a geometric sequence in the number
	// of objects. Loose objects will get soaked up as part of the repack regardless of their
	// reachability.
	RepackObjectsStrategyGeometric = RepackObjectsStrategy("geometric")
)

// RepackObjectsConfig is configuration for RepackObjects.
type RepackObjectsConfig struct {
	// Strategy determines the strategy with which to repack objects.
	Strategy RepackObjectsStrategy
	// WriteBitmap determines whether reachability bitmaps should be written or not. There is no
	// reason to set this to `false`, except for legacy compatibility reasons with existing RPC
	// behaviour
	WriteBitmap bool
	// WriteMultiPackIndex determines whether a multi-pack index should be written or not.
	WriteMultiPackIndex bool
	// CruftExpireBefore determines the cutoff date before which unreachable cruft objects shall
	// be expired and thus deleted.
	CruftExpireBefore time.Time
}

// shouldRepackReferences determines whether the repository's references need to be repacked based
// on heuristics. The more references there are, the more loose referencos may exist until they are
// packed again.
func shouldRepackReferences(s stats.RepositoryInfo) bool {
	// If there aren't any loose refs then there is nothing we need to do.
	if s.References.LooseReferencesCount == 0 {
		return false
	}

	// Packing loose references into the packed-refs file scales with the number of references
	// we're about to write. We thus decide whether we repack refs by weighing the current size
	// of the packed-refs file against the number of loose references. This is done such that we
	// do not repack too often on repositories with a huge number of references, where we can
	// expect a lot of churn in the number of references.
	//
	// As a heuristic, we repack if the number of loose references in the repository exceeds
	// `log(packed_refs_size_in_bytes/100)/log(1.15)`, which scales as following (number of refs
	// is estimated with 100 bytes per reference):
	//
	// - 1kB ~ 10 packed refs: 16 refs
	// - 10kB ~ 100 packed refs: 33 refs
	// - 100kB ~ 1k packed refs: 49 refs
	// - 1MB ~ 10k packed refs: 66 refs
	// - 10MB ~ 100k packed refs: 82 refs
	// - 100MB ~ 1m packed refs: 99 refs
	//
	// We thus allow roughly 16 additional loose refs per factor of ten of packed refs.
	//
	// This heuristic may likely need tweaking in the future, but should serve as a good first
	// iteration.
	if uint64(math.Max(16, math.Log(float64(s.References.PackedReferencesSize)/100)/math.Log(1.15))) > s.References.LooseReferencesCount {
		return false
	}

	return true
}
