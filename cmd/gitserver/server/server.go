// Package server implements the gitserver service.
package server

import (
	"bufio"
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/accesslog"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/perforce"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/urlredactor"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/adapters"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/search"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/honey"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/limiter"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	streamhttp "github.com/sourcegraph/sourcegraph/internal/search/streaming/http"
	"github.com/sourcegraph/sourcegraph/internal/syncx"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// TempDirName is the name used for the temporary directory under ReposDir.
const TempDirName = ".tmp"

// P4HomeName is the name used for the directory that git p4 will use as $HOME
// and where it will store cache data.
const P4HomeName = ".p4home"

// traceLogs is controlled via the env SRC_GITSERVER_TRACE. If true we trace
// logs to stderr
var traceLogs bool

var (
	lastCheckAt    = make(map[api.RepoName]time.Time)
	lastCheckMutex sync.Mutex
)

// debounce() provides some filtering to prevent spammy requests for the same
// repository. If the last fetch of the repository was within the given
// duration, returns false, otherwise returns true and updates the last
// fetch stamp.
func debounce(name api.RepoName, since time.Duration) bool {
	lastCheckMutex.Lock()
	defer lastCheckMutex.Unlock()
	if t, ok := lastCheckAt[name]; ok && time.Now().Before(t.Add(since)) {
		return false
	}
	lastCheckAt[name] = time.Now()
	return true
}

func init() {
	traceLogs, _ = strconv.ParseBool(env.Get("SRC_GITSERVER_TRACE", "false", "Toggles trace logging to stderr"))
}

// cloneJob abstracts away a repo and necessary metadata to clone it. In the future it may be
// possible to simplify this, but to do that, doClone will need to do a lot less than it does at the
// moment.
type cloneJob struct {
	repo   api.RepoName
	dir    common.GitDir
	syncer VCSSyncer

	// TODO: cloneJobConsumer should acquire a new lock. We are trying to keep the changes simple
	// for the time being. When we start using the new approach of using long lived goroutines for
	// cloning we will refactor doClone to acquire a new lock.
	lock RepositoryLock

	remoteURL *vcs.URL
	options   CloneOptions
}

// cloneTask is a thin wrapper around a cloneJob to associate the doneFunc with each job.
type cloneTask struct {
	*cloneJob
	done func() time.Duration
}

// NewCloneQueue initializes a new cloneQueue.
func NewCloneQueue(obctx *observation.Context, jobs *list.List) *common.Queue[*cloneJob] {
	return common.NewQueue[*cloneJob](obctx, "clone-queue", jobs)
}

// Server is a gitserver server.
type Server struct {
	// Logger should be used for all logging and logger creation.
	Logger log.Logger

	// ObservationCtx is used to initialize an operations struct
	// with the appropriate metrics register etc.
	ObservationCtx *observation.Context

	// ReposDir is the path to the base directory for gitserver storage.
	ReposDir string

	// GetRemoteURLFunc is a function which returns the remote URL for a
	// repository. This is used when cloning or fetching a repository. In
	// production this will speak to the database to look up the clone URL. In
	// tests this is usually set to clone a local repository or intentionally
	// error.
	GetRemoteURLFunc func(context.Context, api.RepoName) (string, error)

	// GetVCSSyncer is a function which returns the VCS syncer for a repository.
	// This is used when cloning or fetching a repository. In production this will
	// speak to the database to determine the code host type. In tests this is
	// usually set to return a GitRepoSyncer.
	GetVCSSyncer func(context.Context, api.RepoName) (VCSSyncer, error)

	// Hostname is how we identify this instance of gitserver. Generally it is the
	// actual hostname but can also be overridden by the HOSTNAME environment variable.
	Hostname string

	// DB provides access to datastores.
	DB database.DB

	// CloneQueue is a threadsafe queue used by DoBackgroundClones to process incoming clone
	// requests asynchronously.
	CloneQueue *common.Queue[*cloneJob]

	// Locker is used to lock repositories while fetching to prevent concurrent work.
	Locker RepositoryLocker

	// skipCloneForTests is set by tests to avoid clones.
	skipCloneForTests bool

	// ctx is the context we use for all background jobs. It is done when the
	// server is stopped. Do not directly call this, rather call
	// Server.context()
	ctx      context.Context
	cancel   context.CancelFunc // used to shutdown background jobs
	cancelMu sync.Mutex         // protects canceled
	canceled bool
	wg       sync.WaitGroup // tracks running background jobs

	// cloneLimiter and cloneableLimiter limits the number of concurrent
	// clones and ls-remotes respectively. Use s.acquireCloneLimiter() and
	// s.acquireCloneableLimiter() instead of using these directly.
	cloneLimiter     *limiter.MutableLimiter
	cloneableLimiter *limiter.MutableLimiter

	// RPSLimiter limits the remote code host git operations done per second
	// per gitserver instance
	RPSLimiter *ratelimit.InstrumentedLimiter

	repoUpdateLocksMu sync.Mutex // protects the map below and also updates to locks.once
	repoUpdateLocks   map[api.RepoName]*locks

	// GlobalBatchLogSemaphore is a semaphore shared between all requests to ensure that a
	// maximum number of Git subprocesses are active for all /batch-log requests combined.
	GlobalBatchLogSemaphore *semaphore.Weighted

	// operations provide uniform observability via internal/observation. This value is
	// set by RegisterMetrics when compiled as part of the gitserver binary. The server
	// method ensureOperations should be used in all references to avoid a nil pointer
	// dereferences.
	operations *operations

	// RecordingCommandFactory is a factory that creates recordable commands by wrapping os/exec.Commands.
	// The factory creates recordable commands with a set predicate, which is used to determine whether a
	// particular command should be recorded or not.
	RecordingCommandFactory *wrexec.RecordingCommandFactory

	// Perforce is a plugin-like service attached to Server for all things Perforce.
	Perforce *perforce.Service
}

type locks struct {
	once *sync.Once  // consolidates multiple waiting updates
	mu   *sync.Mutex // prevents updates running in parallel
}

// shortGitCommandTimeout returns the timeout for git commands that should not
// take a long time. Some commands such as "git archive" are allowed more time
// than "git rev-parse", so this will return an appropriate timeout given the
// command.
func shortGitCommandTimeout(args []string) time.Duration {
	if len(args) < 1 {
		return time.Minute
	}
	switch args[0] {
	case "archive":
		// This is a long time, but this never blocks a user request for this
		// long. Even repos that are not that large can take a long time, for
		// example a search over all repos in an organization may have several
		// large repos. All of those repos will be competing for IO => we need
		// a larger timeout.
		return conf.GitLongCommandTimeout()

	case "ls-remote":
		return 30 * time.Second

	default:
		return time.Minute
	}
}

// shortGitCommandSlow returns the threshold for regarding an git command as
// slow. Some commands such as "git archive" are inherently slower than "git
// rev-parse", so this will return an appropriate threshold given the command.
func shortGitCommandSlow(args []string) time.Duration {
	if len(args) < 1 {
		return time.Second
	}
	switch args[0] {
	case "archive":
		return 1 * time.Minute

	case "blame", "ls-tree", "log", "show":
		return 5 * time.Second

	default:
		return 2500 * time.Millisecond
	}
}

// 🚨 SECURITY: headerXRequestedWithMiddleware will ensure that the X-Requested-With
// header contains the correct value. See "What does X-Requested-With do, anyway?" in
// https://github.com/sourcegraph/sourcegraph/pull/27931.
func headerXRequestedWithMiddleware(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := log.Scoped("gitserver", "headerXRequestedWithMiddleware")

		// Do not apply the middleware to /ping and /git endpoints.
		//
		// 1. /ping is used by health check services who most likely don't set this header
		// at all.
		//
		// 2. /git may be used to run "git fetch" from another gitserver instance over
		// HTTP and the fetchCommand does not set this header yet.
		if strings.HasPrefix(r.URL.Path, "/ping") || strings.HasPrefix(r.URL.Path, "/git") {
			next.ServeHTTP(w, r)
			return
		}

		if value := r.Header.Get("X-Requested-With"); value != "Sourcegraph" {
			l.Error("header X-Requested-With is not set or is invalid", log.String("path", r.URL.Path))
			http.Error(w, "header X-Requested-With is not set or is invalid", http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// Handler returns the http.Handler that should be used to serve requests.
func (s *Server) Handler() http.Handler {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.repoUpdateLocks = make(map[api.RepoName]*locks)

	// GitMaxConcurrentClones controls the maximum number of clones that
	// can happen at once on a single gitserver.
	// Used to prevent throttle limits from a code host. Defaults to 5.
	//
	// The new repo-updater scheduler enforces the rate limit across all gitserver,
	// so ideally this logic could be removed here; however, ensureRevision can also
	// cause an update to happen and it is called on every exec command.
	// Max concurrent clones also means repo updates.
	maxConcurrentClones := conf.GitMaxConcurrentClones()
	s.cloneLimiter = limiter.NewMutable(maxConcurrentClones)
	s.cloneableLimiter = limiter.NewMutable(maxConcurrentClones)

	// TODO: Remove side-effects from this Handler method.
	conf.Watch(func() {
		limit := conf.GitMaxConcurrentClones()
		s.cloneLimiter.SetLimit(limit)
		s.cloneableLimiter.SetLimit(limit)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/archive", trace.WithRouteName("archive", accesslog.HTTPMiddleware(
		s.Logger.Scoped("archive.accesslog", "archive endpoint access log"),
		conf.DefaultClient(),
		s.handleArchive,
	)))
	mux.HandleFunc("/exec", trace.WithRouteName("exec", accesslog.HTTPMiddleware(
		s.Logger.Scoped("exec.accesslog", "exec endpoint access log"),
		conf.DefaultClient(),
		s.handleExec,
	)))
	mux.HandleFunc("/search", trace.WithRouteName("search", s.handleSearch))
	mux.HandleFunc("/batch-log", trace.WithRouteName("batch-log", s.handleBatchLog))
	mux.HandleFunc("/p4-exec", trace.WithRouteName("p4-exec", accesslog.HTTPMiddleware(
		s.Logger.Scoped("p4-exec.accesslog", "p4-exec endpoint access log"),
		conf.DefaultClient(),
		s.handleP4Exec,
	)))
	mux.HandleFunc("/list-gitolite", trace.WithRouteName("list-gitolite", s.handleListGitolite))
	mux.HandleFunc("/is-repo-cloneable", trace.WithRouteName("is-repo-cloneable", s.handleIsRepoCloneable))
	// TODO: Remove this endpoint after 5.2, it is deprecated.
	mux.HandleFunc("/repos-stats", trace.WithRouteName("repos-stats", s.handleReposStats))
	mux.HandleFunc("/repo-clone-progress", trace.WithRouteName("repo-clone-progress", s.handleRepoCloneProgress))
	mux.HandleFunc("/delete", trace.WithRouteName("delete", s.handleRepoDelete))
	mux.HandleFunc("/repo-update", trace.WithRouteName("repo-update", s.handleRepoUpdate))
	mux.HandleFunc("/repo-clone", trace.WithRouteName("repo-clone", s.handleRepoClone))
	mux.HandleFunc("/create-commit-from-patch-binary", trace.WithRouteName("create-commit-from-patch-binary", s.handleCreateCommitFromPatchBinary))
	mux.HandleFunc("/disk-info", trace.WithRouteName("disk-info", s.handleDiskInfo))
	mux.HandleFunc("/ping", trace.WithRouteName("ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// This endpoint allows us to expose gitserver itself as a "git service"
	// (ETOOMANYGITS!) that allows other services to run commands like "git fetch"
	// directly against a gitserver replica and treat it as a git remote.
	//
	// Example use case for this is a repo migration from one replica to another during
	// scaling events and the new destination gitserver replica can directly clone from
	// the gitserver replica which hosts the repository currently.
	mux.HandleFunc("/git/", trace.WithRouteName("git", accesslog.HTTPMiddleware(
		s.Logger.Scoped("git.accesslog", "git endpoint access log"),
		conf.DefaultClient(),
		func(rw http.ResponseWriter, r *http.Request) {
			http.StripPrefix("/git", s.gitServiceHandler()).ServeHTTP(rw, r)
		},
	)))

	// Migration to hexagonal architecture starting here:
	gitAdapter := &adapters.Git{
		ReposDir:                s.ReposDir,
		RecordingCommandFactory: s.RecordingCommandFactory,
	}
	getObjectService := gitdomain.GetObjectService{
		RevParse:      gitAdapter.RevParse,
		GetObjectType: gitAdapter.GetObjectType,
	}
	getObjectFunc := gitdomain.GetObjectFunc(func(ctx context.Context, repo api.RepoName, objectName string) (_ *gitdomain.GitObject, err error) {
		// Tracing is server concern, so add it here. Once generics lands we should be
		// able to create some simple wrappers
		tr, ctx := trace.New(ctx, "GetObject",
			attribute.String("objectName", objectName))
		defer tr.EndWithErr(&err)

		return getObjectService.GetObject(ctx, repo, objectName)
	})

	mux.HandleFunc("/commands/get-object", trace.WithRouteName("commands/get-object",
		accesslog.HTTPMiddleware(
			s.Logger.Scoped("commands/get-object.accesslog", "commands/get-object endpoint access log"),
			conf.DefaultClient(),
			handleGetObject(s.Logger.Scoped("commands/get-object", "handles get object"), getObjectFunc),
		)))

	// 🚨 SECURITY: This must be wrapped in headerXRequestedWithMiddleware.
	return headerXRequestedWithMiddleware(mux)
}

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
	reposDir string,
	interval time.Duration,
	batchSize int,
	perSecond int,
) goroutine.BackgroundRoutine {
	var previousAddrs string
	var previousPinned string

	return goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(ctx),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			gitServerAddrs := gitserver.NewGitserverAddresses(conf.Get())
			addrs := gitServerAddrs.Addresses
			// We turn addrs into a string here for easy comparison and storage of previous
			// addresses since we'd need to take a copy of the slice anyway.
			currentAddrs := strings.Join(addrs, ",")
			fullSync := currentAddrs != previousAddrs
			previousAddrs = currentAddrs

			// We turn PinnedServers into a string here for easy comparison and storage
			// of previous pins.
			pinnedServerPairs := make([]string, 0, len(gitServerAddrs.PinnedServers))
			for k, v := range gitServerAddrs.PinnedServers {
				pinnedServerPairs = append(pinnedServerPairs, fmt.Sprintf("%s=%s", k, v))
			}
			sort.Strings(pinnedServerPairs)
			currentPinned := strings.Join(pinnedServerPairs, ",")
			fullSync = fullSync || currentPinned != previousPinned
			previousPinned = currentPinned

			if err := syncRepoState(ctx, logger, db, locker, shardID, reposDir, gitServerAddrs, batchSize, perSecond, fullSync); err != nil {
				return errors.Wrap(err, "syncing repo state")
			}

			return nil
		}),
		goroutine.WithName("gitserver.repo-state-syncer"),
		goroutine.WithDescription("syncs repo state on disk with the gitserver_repos table"),
		goroutine.WithInterval(interval),
	)
}

func addrForRepo(ctx context.Context, repoName api.RepoName, gitServerAddrs gitserver.GitserverAddresses) string {
	return gitServerAddrs.AddrForRepo(ctx, filepath.Base(os.Args[0]), repoName)
}

// NewClonePipeline creates a new pipeline that clones repos asynchronously. It
// creates a producer-consumer pipeline that handles clone requests asychronously.
func (s *Server) NewClonePipeline(logger log.Logger, cloneQueue *common.Queue[*cloneJob]) goroutine.BackgroundRoutine {
	return &clonePipelineRoutine{
		tasks:  make(chan *cloneTask),
		logger: logger,
		s:      s,
		queue:  cloneQueue,
	}
}

type clonePipelineRoutine struct {
	logger log.Logger

	tasks chan *cloneTask
	// TODO: Get rid of this dependency.
	s      *Server
	queue  *common.Queue[*cloneJob]
	cancel context.CancelFunc
}

func (p *clonePipelineRoutine) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	// Start a go routine for each the producer and the consumer.
	go p.cloneJobConsumer(ctx, p.tasks)
	go p.cloneJobProducer(ctx, p.tasks)
}

func (p *clonePipelineRoutine) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *clonePipelineRoutine) cloneJobProducer(ctx context.Context, tasks chan<- *cloneTask) {
	defer close(tasks)

	for {
		// Acquire the cond mutex lock and wait for a signal if the queue is empty.
		p.queue.Mutex.Lock()
		if p.queue.Empty() {
			// TODO: This should only wait if ctx is not canceled.
			p.queue.Cond.Wait()
		}

		// The queue is not empty and we have a job to process! But don't forget to unlock the cond
		// mutex here as we don't need to hold the lock beyond this point for now.
		p.queue.Mutex.Unlock()

		// Keep popping from the queue until the queue is empty again, in which case we start all
		// over again from the top.
		for {
			job, doneFunc := p.queue.Pop()
			if job == nil {
				break
			}

			select {
			case tasks <- &cloneTask{
				cloneJob: *job,
				done:     doneFunc,
			}:
			case <-ctx.Done():
				p.logger.Error("cloneJobProducer", log.Error(ctx.Err()))
				return
			}
		}
	}
}

func (p *clonePipelineRoutine) cloneJobConsumer(ctx context.Context, tasks <-chan *cloneTask) {
	logger := p.s.Logger.Scoped("cloneJobConsumer", "process clone jobs")

	for task := range tasks {
		logger := logger.With(log.String("job.repo", string(task.repo)))

		select {
		case <-ctx.Done():
			logger.Error("context done", log.Error(ctx.Err()))
			return
		default:
		}

		ctx, cancel, err := p.s.acquireCloneLimiter(ctx)
		if err != nil {
			logger.Error("acquireCloneLimiter", log.Error(err))
			continue
		}

		go func(task *cloneTask) {
			defer cancel()

			err := p.s.doClone(ctx, task.repo, task.dir, task.syncer, task.lock, task.remoteURL, task.options)
			if err != nil {
				logger.Error("failed to clone repo", log.Error(err))
			}
			// Use a different context in case we failed because the original context failed.
			p.s.setLastErrorNonFatal(p.s.ctx, task.repo, err)
			_ = task.done()
		}(task)
	}
}

var (
	repoSyncStateCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repo_sync_state_counter",
		Help: "Incremented each time we check the state of repo",
	}, []string{"type"})
	repoStateUpsertCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repo_sync_state_upsert_counter",
		Help: "Incremented each time we upsert repo state in the database",
	}, []string{"success"})
	wrongShardReposTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_gitserver_repo_wrong_shard",
		Help: "The number of repos that are on disk on the wrong shard",
	})
	wrongShardReposSizeTotalBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_gitserver_repo_wrong_shard_bytes",
		Help: "Size (in bytes) of repos that are on disk on the wrong shard",
	})
	wrongShardReposDeletedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repo_wrong_shard_deleted",
		Help: "The number of repos on the wrong shard that we deleted",
	})
)

func syncRepoState(
	ctx context.Context,
	logger log.Logger,
	db database.DB,
	locker RepositoryLocker,
	shardID string,
	reposDir string,
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

			// We may have a deleted repo, we need to extract the original name both to
			// ensure that the shard check is correct and also so that we can find the
			// directory.
			repo.Name = api.UndeletedRepoName(repo.Name)

			// Ensure we're only dealing with repos we are responsible for.
			addr := addrForRepo(ctx, repo.Name, gitServerAddrs)
			if !hostnameMatch(shardID, addr) {
				repoSyncStateCounter.WithLabelValues("other_shard").Inc()
				continue
			}
			repoSyncStateCounter.WithLabelValues("this_shard").Inc()

			dir := repoDirFromName(reposDir, repo.Name)
			cloned := repoCloned(dir)
			_, cloning := locker.Status(dir)

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

// repoCloned checks if dir or `${dir}/.git` is a valid GIT_DIR.
var repoCloned = func(dir common.GitDir) bool {
	_, err := os.Stat(dir.Path("HEAD"))
	return !os.IsNotExist(err)
}

// Stop cancels the running background jobs and returns when done.
func (s *Server) Stop() {
	// idempotent so we can just always set and cancel
	s.cancel()
	s.cancelMu.Lock()
	s.canceled = true
	s.cancelMu.Unlock()
	s.wg.Wait()
}

// serverContext returns a child context tied to the lifecycle of server.
func (s *Server) serverContext() (context.Context, context.CancelFunc) {
	// if we are already canceled don't increment our WaitGroup. This is to
	// prevent a loop somewhere preventing us from ever finishing the
	// WaitGroup, even though all calls fails instantly due to the canceled
	// context.
	s.cancelMu.Lock()
	if s.canceled {
		s.cancelMu.Unlock()
		return s.ctx, func() {}
	}
	s.wg.Add(1)
	s.cancelMu.Unlock()

	ctx, cancel := context.WithCancel(s.ctx)

	// we need to track if we have called cancel, since we are only allowed to
	// call wg.Done() once, but CancelFuncs can be called any number of times.
	var canceled int32
	return ctx, func() {
		ok := atomic.CompareAndSwapInt32(&canceled, 0, 1)
		if ok {
			cancel()
			s.wg.Done()
		}
	}
}

func (s *Server) getRemoteURL(ctx context.Context, name api.RepoName) (*vcs.URL, error) {
	remoteURL, err := s.GetRemoteURLFunc(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "GetRemoteURLFunc")
	}

	return vcs.ParseURL(remoteURL)
}

// acquireCloneLimiter() acquires a cancellable context associated with the
// clone limiter.
func (s *Server) acquireCloneLimiter(ctx context.Context) (context.Context, context.CancelFunc, error) {
	pendingClones.Inc()
	defer pendingClones.Dec()
	return s.cloneLimiter.Acquire(ctx)
}

func (s *Server) acquireCloneableLimiter(ctx context.Context) (context.Context, context.CancelFunc, error) {
	lsRemoteQueue.Inc()
	defer lsRemoteQueue.Dec()
	return s.cloneableLimiter.Acquire(ctx)
}

// tempDir is a wrapper around os.MkdirTemp, but using the given reposDir
// temporary directory filepath.Join(s.ReposDir, tempDirName).
//
// This directory is cleaned up by gitserver and will be ignored by repository
// listing operations.
func tempDir(reposDir, prefix string) (name string, err error) {
	// TODO: At runtime, this directory always exists. We only need to ensure
	// the directory exists here because tests use this function without creating
	// the directory first. Ideally, we can remove this later.
	tmp := filepath.Join(reposDir, TempDirName)
	if err := os.MkdirAll(tmp, os.ModePerm); err != nil {
		return "", err
	}
	return os.MkdirTemp(tmp, prefix)
}

func ignorePath(reposDir string, path string) bool {
	// We ignore any path which starts with .tmp or .p4home in ReposDir
	if filepath.Dir(path) != reposDir {
		return false
	}
	base := filepath.Base(path)
	return strings.HasPrefix(base, TempDirName) || strings.HasPrefix(base, P4HomeName)
}

func (s *Server) handleIsRepoCloneable(w http.ResponseWriter, r *http.Request) {
	var req protocol.IsRepoCloneableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Repo == "" {
		http.Error(w, "no Repo given", http.StatusBadRequest)
		return
	}
	resp, err := s.isRepoCloneable(r.Context(), req.Repo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) isRepoCloneable(ctx context.Context, repo api.RepoName) (protocol.IsRepoCloneableResponse, error) {
	var syncer VCSSyncer
	// We use an internal actor here as the repo may be private. It is safe since all
	// we return is a bool indicating whether the repo is cloneable or not. Perhaps
	// the only things that could leak here is whether a private repo exists although
	// the endpoint is only available internally so it's low risk.
	remoteURL, err := s.getRemoteURL(actor.WithInternalActor(ctx), repo)
	if err != nil {
		// We use this endpoint to verify if a repo exists without consuming
		// API rate limit, since many users visit private or bogus repos,
		// so we deduce the unauthenticated clone URL from the repo name.
		remoteURL, _ = vcs.ParseURL("https://" + string(repo) + ".git")

		// At this point we are assuming it's a git repo
		syncer = NewGitRepoSyncer(s.RecordingCommandFactory)
	} else {
		syncer, err = s.GetVCSSyncer(ctx, repo)
		if err != nil {
			return protocol.IsRepoCloneableResponse{}, err
		}
	}

	resp := protocol.IsRepoCloneableResponse{
		Cloned: repoCloned(repoDirFromName(s.ReposDir, repo)),
	}
	if err := syncer.IsCloneable(ctx, repo, remoteURL); err == nil {
		resp.Cloneable = true
	} else {
		resp.Reason = err.Error()
	}

	return resp, nil
}

// handleRepoUpdate is a synchronous (waits for update to complete or
// time out) method so it can yield errors. Updates are not
// unconditional; we debounce them based on the provided
// interval, to avoid spam.
func (s *Server) handleRepoUpdate(w http.ResponseWriter, r *http.Request) {
	var req protocol.RepoUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := s.repoUpdate(&req)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) repoUpdate(req *protocol.RepoUpdateRequest) protocol.RepoUpdateResponse {
	logger := s.Logger.Scoped("handleRepoUpdate", "synchronous http handler for repo updates")
	var resp protocol.RepoUpdateResponse
	req.Repo = protocol.NormalizeRepo(req.Repo)
	dir := repoDirFromName(s.ReposDir, req.Repo)

	// despite the existence of a context on the request, we don't want to
	// cancel the git commands partway through if the request terminates.
	ctx, cancel1 := s.serverContext()
	defer cancel1()
	ctx, cancel2 := context.WithTimeout(ctx, conf.GitLongCommandTimeout())
	defer cancel2()
	if !repoCloned(dir) && !s.skipCloneForTests {
		_, err := s.CloneRepo(ctx, req.Repo, CloneOptions{Block: true})
		if err != nil {
			logger.Warn("error cloning repo", log.String("repo", string(req.Repo)), log.Error(err))
			resp.Error = err.Error()
		}
	} else {
		var statusErr, updateErr error

		if debounce(req.Repo, req.Since) {
			updateErr = s.doRepoUpdate(ctx, req.Repo, "")
		}

		// attempts to acquire these values are not contingent on the success of
		// the update.
		lastFetched, err := repoLastFetched(dir)
		if err != nil {
			statusErr = err
		} else {
			resp.LastFetched = &lastFetched
		}
		lastChanged, err := repoLastChanged(dir)
		if err != nil {
			statusErr = err
		} else {
			resp.LastChanged = &lastChanged
		}
		if statusErr != nil {
			logger.Error("failed to get status of repo", log.String("repo", string(req.Repo)), log.Error(statusErr))
			// report this error in-band, but still produce a valid response with the
			// other information.
			resp.Error = statusErr.Error()
		}
		// If an error occurred during update, report it but don't actually make
		// it into an http error; we want the client to get the information cleanly.
		// An update error "wins" over a status error.
		if updateErr != nil {
			resp.Error = updateErr.Error()
		} else {
			s.Perforce.EnqueueChangelistMappingJob(perforce.NewChangelistMappingJob(req.Repo, dir))
		}
	}

	return resp
}

// handleRepoClone is an asynchronous (does not wait for update to complete or
// time out) call to clone a repository.
// Asynchronous errors will have to be checked in the gitserver_repos table under last_error.
func (s *Server) handleRepoClone(w http.ResponseWriter, r *http.Request) {
	logger := s.Logger.Scoped("handleRepoClone", "asynchronous http handler for repo clones")
	var req protocol.RepoCloneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var resp protocol.RepoCloneResponse
	req.Repo = protocol.NormalizeRepo(req.Repo)

	_, err := s.CloneRepo(context.Background(), req.Repo, CloneOptions{Block: false})
	if err != nil {
		logger.Warn("error cloning repo", log.String("repo", string(req.Repo)), log.Error(err))
		resp.Error = err.Error()
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	var (
		logger    = s.Logger.Scoped("handleArchive", "http handler for repo archive")
		q         = r.URL.Query()
		treeish   = q.Get("treeish")
		repo      = q.Get("repo")
		format    = q.Get("format")
		pathspecs = q["path"]
	)

	// Log which which actor is accessing the repo.
	accesslog.Record(r.Context(), repo,
		log.String("treeish", treeish),
		log.String("format", format),
		log.Strings("path", pathspecs),
	)

	if err := checkSpecArgSafety(treeish); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error("gitserver.archive.CheckSpecArgSafety", log.Error(err))
		return
	}

	if repo == "" || format == "" {
		w.WriteHeader(http.StatusBadRequest)
		logger.Error("gitserver.archive", log.String("error", "empty repo or format"))
		return
	}

	req := &protocol.ExecRequest{
		Repo: api.RepoName(repo),
		Args: []string{
			"archive",

			// Suppresses fatal error when the repo contains paths matching **/.git/** and instead
			// includes those files (to allow archiving invalid such repos). This is unexpected
			// behavior; the --worktree-attributes flag should merely let us specify a gitattributes
			// file that contains `**/.git/** export-ignore`, but it actually makes everything work as
			// desired. Tested by the "repo with .git dir" test case.
			"--worktree-attributes",

			"--format=" + format,
		},
	}

	if format == string(gitserver.ArchiveFormatZip) {
		// Compression level of 0 (no compression) seems to perform the
		// best overall on fast network links, but this has not been tuned
		// thoroughly.
		req.Args = append(req.Args, "-0")
	}

	req.Args = append(req.Args, treeish, "--")
	req.Args = append(req.Args, pathspecs...)

	s.execHTTP(w, r, req)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	logger := s.Logger.Scoped("handleSearch", "http handler for search")
	tr, ctx := trace.New(r.Context(), "handleSearch")
	defer tr.End()

	// Decode the request
	protocol.RegisterGob()
	var args protocol.SearchRequest
	if err := gob.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	eventWriter, err := streamhttp.NewWriter(w)
	if err != nil {
		tr.SetError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var matchesBufMux sync.Mutex
	matchesBuf := streamhttp.NewJSONArrayBuf(8*1024, func(data []byte) error {
		tr.AddEvent("flushing data", attribute.Int("data.len", len(data)))
		return eventWriter.EventBytes("matches", data)
	})

	// Start a goroutine that periodically flushes the buffer
	var flusherWg conc.WaitGroup
	flusherCtx, flusherCancel := context.WithCancel(context.Background())
	defer flusherCancel()
	flusherWg.Go(func() {
		flushTicker := time.NewTicker(50 * time.Millisecond)
		defer flushTicker.Stop()

		for {
			select {
			case <-flushTicker.C:
				matchesBufMux.Lock()
				matchesBuf.Flush()
				matchesBufMux.Unlock()
			case <-flusherCtx.Done():
				return
			}
		}
	})

	// Create a callback that appends the match to the buffer
	var haveFlushed atomic.Bool
	onMatch := func(match *protocol.CommitMatch) error {
		matchesBufMux.Lock()
		defer matchesBufMux.Unlock()

		err := matchesBuf.Append(match)
		if err != nil {
			return err
		}

		// If we haven't sent any results yet, flush immediately
		if !haveFlushed.Load() {
			haveFlushed.Store(true)
			return matchesBuf.Flush()
		}

		return nil
	}

	// Run the search
	limitHit, searchErr := s.searchWithObservability(ctx, tr, &args, onMatch)
	if writeErr := eventWriter.Event("done", protocol.NewSearchEventDone(limitHit, searchErr)); writeErr != nil {
		if !errors.Is(writeErr, syscall.EPIPE) {
			logger.Error("failed to send done event", log.Error(writeErr))
		}
	}

	// Clean up the flusher goroutine, then do one final flush
	flusherCancel()
	flusherWg.Wait()
	matchesBuf.Flush()
}

func (s *Server) searchWithObservability(ctx context.Context, tr trace.Trace, args *protocol.SearchRequest, onMatch func(*protocol.CommitMatch) error) (limitHit bool, err error) {
	searchStart := time.Now()

	searchRunning.Inc()
	defer searchRunning.Dec()

	tr.SetAttributes(
		args.Repo.Attr(),
		attribute.Bool("include_diff", args.IncludeDiff),
		attribute.String("query", args.Query.String()),
		attribute.Int("limit", args.Limit),
		attribute.Bool("include_modified_files", args.IncludeModifiedFiles),
	)
	defer func() {
		tr.AddEvent("done", attribute.Bool("limit_hit", limitHit))
		tr.SetError(err)
		searchDuration.
			WithLabelValues(strconv.FormatBool(err != nil)).
			Observe(time.Since(searchStart).Seconds())

		if honey.Enabled() || traceLogs {
			act := actor.FromContext(ctx)
			ev := honey.NewEvent("gitserver-search")
			ev.SetSampleRate(honeySampleRate("", act))
			ev.AddField("repo", args.Repo)
			ev.AddField("revisions", args.Revisions)
			ev.AddField("include_diff", args.IncludeDiff)
			ev.AddField("include_modified_files", args.IncludeModifiedFiles)
			ev.AddField("actor", act.UIDString())
			ev.AddField("query", args.Query.String())
			ev.AddField("limit", args.Limit)
			ev.AddField("duration_ms", time.Since(searchStart).Milliseconds())
			if err != nil {
				ev.AddField("error", err.Error())
			}
			if traceID := trace.ID(ctx); traceID != "" {
				ev.AddField("traceID", traceID)
				ev.AddField("trace", trace.URL(traceID, conf.DefaultClient()))
			}
			if honey.Enabled() {
				_ = ev.Send()
			}
			if traceLogs {
				s.Logger.Debug("TRACE gitserver search", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
			}
		}
	}()

	observeLatency := syncx.OnceFunc(func() {
		searchLatency.Observe(time.Since(searchStart).Seconds())
	})

	onMatchWithLatency := func(cm *protocol.CommitMatch) error {
		observeLatency()
		return onMatch(cm)
	}

	return s.search(ctx, args, onMatchWithLatency)
}

// search handles the core logic of the search. It is passed a matchesBuf so it doesn't need to
// concern itself with event types, and all instrumentation is handled in the calling function.
func (s *Server) search(ctx context.Context, args *protocol.SearchRequest, onMatch func(*protocol.CommitMatch) error) (limitHit bool, err error) {
	args.Repo = protocol.NormalizeRepo(args.Repo)
	if args.Limit == 0 {
		args.Limit = math.MaxInt32
	}

	// We used to have an `ensureRevision`/`CloneRepo` calls here that were
	// obsolete, because a search for an unknown revision of the repo (of an
	// uncloned repo) won't make it to gitserver and fail with an ErrNoResolvedRepos
	// and a related search alert before calling the gitserver.
	//
	// However, to protect for a weird case of getting an uncloned repo here (e.g.
	// via a direct API call), we leave a `repoCloned` check and return an error if
	// the repo is not cloned.
	dir := repoDirFromName(s.ReposDir, args.Repo)
	if !repoCloned(dir) {
		s.Logger.Debug("attempted to search for a not cloned repo")
		return false, &gitdomain.RepoNotExistError{
			Repo: args.Repo,
		}
	}

	mt, err := search.ToMatchTree(args.Query)
	if err != nil {
		return false, err
	}

	// Ensure that we populate ModifiedFiles when we have a DiffModifiesFile filter.
	// --name-status is not zero cost, so we don't do it on every search.
	hasDiffModifiesFile := false
	search.Visit(mt, func(mt search.MatchTree) {
		switch mt.(type) {
		case *search.DiffModifiesFile:
			hasDiffModifiesFile = true
		}
	})

	// Create a callback that detects whether we've hit a limit
	// and stops sending when we have.
	var sentCount atomic.Int64
	var hitLimit atomic.Bool
	limitedOnMatch := func(match *protocol.CommitMatch) {
		// Avoid sending if we've already hit the limit
		if int(sentCount.Load()) >= args.Limit {
			hitLimit.Store(true)
			return
		}

		sentCount.Add(int64(matchCount(match)))
		onMatch(match)
	}

	searcher := &search.CommitSearcher{
		Logger:               s.Logger,
		RepoName:             args.Repo,
		RepoDir:              dir.Path(),
		Revisions:            args.Revisions,
		Query:                mt,
		IncludeDiff:          args.IncludeDiff,
		IncludeModifiedFiles: args.IncludeModifiedFiles || hasDiffModifiesFile,
	}

	return hitLimit.Load(), searcher.Search(ctx, limitedOnMatch)
}

// matchCount returns either:
// 1) the number of diff matches if there are any
// 2) the number of messsage matches if there are any
// 3) one, to represent matching the commit, but nothing inside it
func matchCount(cm *protocol.CommitMatch) int {
	if len(cm.Diff.MatchedRanges) > 0 {
		return len(cm.Diff.MatchedRanges)
	}
	if len(cm.Message.MatchedRanges) > 0 {
		return len(cm.Message.MatchedRanges)
	}
	return 1
}

func (s *Server) performGitLogCommand(ctx context.Context, repoCommit api.RepoCommit, format string) (output string, isRepoCloned bool, err error) {
	ctx, _, endObservation := s.operations.batchLogSingle.With(ctx, &err, observation.Args{
		Attrs: append(
			[]attribute.KeyValue{
				attribute.String("format", format),
			},
			repoCommit.Attrs()...,
		),
	})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Bool("isRepoCloned", isRepoCloned),
		}})
	}()

	dir := repoDirFromName(s.ReposDir, repoCommit.Repo)
	if !repoCloned(dir) {
		return "", false, nil
	}

	var buf bytes.Buffer

	commitId := string(repoCommit.CommitID)
	// make sure CommitID is not an arg
	if commitId[0] == '-' {
		return "", true, errors.New("commit ID starting with - is not allowed")
	}

	cmd := s.RecordingCommandFactory.Command(ctx, s.Logger, string(repoCommit.Repo), "git", "log", "-n", "1", "--name-only", format, commitId)
	dir.Set(cmd.Unwrap())
	cmd.Unwrap().Stdout = &buf

	if _, err := runCommand(ctx, cmd); err != nil {
		return "", true, err
	}

	return buf.String(), true, nil
}

func (s *Server) batchGitLogInstrumentedHandler(ctx context.Context, req protocol.BatchLogRequest) (resp protocol.BatchLogResponse, err error) {
	ctx, _, endObservation := s.operations.batchLog.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.String("results", fmt.Sprintf("%+v", resp.Results)),
		}})
	}()

	// Perform requests in each repository in the input batch. We perform these commands
	// concurrently, but only allow for so many commands to be in-flight at a time so that
	// we don't overwhelm a shard with either a large request or too many concurrent batch
	// requests.

	g, ctx := errgroup.WithContext(ctx)
	results := make([]protocol.BatchLogResult, len(req.RepoCommits))

	if s.GlobalBatchLogSemaphore == nil {
		return protocol.BatchLogResponse{}, errors.New("s.GlobalBatchLogSemaphore not initialized")
	}

	for i, repoCommit := range req.RepoCommits {
		// Avoid capture of loop variables
		i, repoCommit := i, repoCommit

		start := time.Now()
		if err := s.GlobalBatchLogSemaphore.Acquire(ctx, 1); err != nil {
			return resp, err
		}
		s.operations.batchLogSemaphoreWait.Observe(time.Since(start).Seconds())

		g.Go(func() error {
			defer s.GlobalBatchLogSemaphore.Release(1)

			output, isRepoCloned, gitLogErr := s.performGitLogCommand(ctx, repoCommit, req.Format)
			if gitLogErr == nil && !isRepoCloned {
				gitLogErr = errors.Newf("repo not found")
			}
			var errMessage string
			if gitLogErr != nil {
				errMessage = gitLogErr.Error()
			}

			// Concurrently write results to shared slice. This slice is already properly
			// sized, and each goroutine writes to a unique index exactly once. There should
			// be no data race conditions possible here.

			results[i] = protocol.BatchLogResult{
				RepoCommit:    repoCommit,
				CommandOutput: output,
				CommandError:  errMessage,
			}
			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return
	}
	return protocol.BatchLogResponse{Results: results}, nil
}

func (s *Server) handleBatchLog(w http.ResponseWriter, r *http.Request) {
	// 🚨 SECURITY: Only allow POST requests.
	if strings.ToUpper(r.Method) != http.MethodPost {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	s.operations = s.ensureOperations()

	// Read request body
	var req protocol.BatchLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request parameters
	if len(req.RepoCommits) == 0 {
		// Early exit: implicitly writes 200 OK
		_ = json.NewEncoder(w).Encode(protocol.BatchLogResponse{Results: []protocol.BatchLogResult{}})
		return
	}
	if !strings.HasPrefix(req.Format, "--format=") {
		http.Error(w, "format parameter expected to be of the form `--format=<git log format>`", http.StatusUnprocessableEntity)
		return
	}

	// Handle unexpected error conditions
	resp, err := s.batchGitLogInstrumentedHandler(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write payload to client: implicitly writes 200 OK
	_ = json.NewEncoder(w).Encode(resp)
}

// ensureOperations returns the non-nil operations value supplied to this server
// via RegisterMetrics (when constructed as part of the gitserver binary), or
// constructs and memoizes a no-op operations value (for use in tests).
func (s *Server) ensureOperations() *operations {
	if s.operations == nil {
		s.operations = newOperations(s.ObservationCtx)
	}

	return s.operations
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	// 🚨 SECURITY: Only allow POST requests.
	// See https://github.com/sourcegraph/security-issues/issues/213.
	if strings.ToUpper(r.Method) != http.MethodPost {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Log which actor is accessing the repo.
	args := req.Args
	cmd := ""
	if len(req.Args) > 0 {
		cmd = req.Args[0]
		args = args[1:]
	}
	accesslog.Record(r.Context(), string(req.Repo),
		log.String("cmd", cmd),
		log.Strings("args", args),
	)

	s.execHTTP(w, r, &req)
}

var blockedCommandExecutedCounter = promauto.NewCounter(prometheus.CounterOpts{
	Name: "src_gitserver_exec_blocked_command_received",
	Help: "Incremented each time a command not in the allowlist for gitserver is executed",
})

var ErrInvalidCommand = errors.New("invalid command")

type NotFoundError struct {
	Payload *protocol.NotFoundPayload
}

func (e *NotFoundError) Error() string { return "not found" }

type execStatus struct {
	ExitStatus int
	Stderr     string
	Err        error
}

// exec runs a git command. After the first write to w, it must not return an error.
// TODO(@camdencheek): once gRPC is the only consumer of this, do everything with errors
// because gRPC can handle trailing errors on a stream.
func (s *Server) exec(ctx context.Context, logger log.Logger, req *protocol.ExecRequest, userAgent string, w io.Writer) (execStatus, error) {
	// 🚨 SECURITY: Ensure that only commands in the allowed list are executed.
	// See https://github.com/sourcegraph/security-issues/issues/213.

	repoPath := string(protocol.NormalizeRepo(req.Repo))
	repoDir := filepath.Join(s.ReposDir, filepath.FromSlash(repoPath))

	if !gitdomain.IsAllowedGitCmd(logger, req.Args, repoDir) {
		blockedCommandExecutedCounter.Inc()
		return execStatus{}, ErrInvalidCommand
	}

	if !req.NoTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, shortGitCommandTimeout(req.Args))
		defer cancel()
	}

	start := time.Now()
	var cmdStart time.Time // set once we have ensured commit
	exitStatus := unsetExitStatus
	var stdoutN, stderrN int64
	var status string
	var execErr error
	ensureRevisionStatus := "noop"

	req.Repo = protocol.NormalizeRepo(req.Repo)
	repoName := req.Repo

	// Instrumentation
	{
		cmd := ""
		if len(req.Args) > 0 {
			cmd = req.Args[0]
		}
		args := strings.Join(req.Args, " ")

		var tr trace.Trace
		tr, ctx = trace.New(ctx, "exec."+cmd, repoName.Attr())
		tr.SetAttributes(
			attribute.String("args", args),
			attribute.String("ensure_revision", req.EnsureRevision),
		)
		logger = logger.WithTrace(trace.Context(ctx))

		execRunning.WithLabelValues(cmd).Inc()
		defer func() {
			tr.AddEvent(
				"done",
				attribute.String("status", status),
				attribute.Int64("stdout", stdoutN),
				attribute.Int64("stderr", stderrN),
				attribute.String("ensure_revision_status", ensureRevisionStatus),
			)
			tr.SetError(execErr)
			tr.End()

			duration := time.Since(start)
			execRunning.WithLabelValues(cmd).Dec()
			execDuration.WithLabelValues(cmd, status).Observe(duration.Seconds())

			var cmdDuration time.Duration
			var fetchDuration time.Duration
			if !cmdStart.IsZero() {
				cmdDuration = time.Since(cmdStart)
				fetchDuration = cmdStart.Sub(start)
			}

			isSlow := cmdDuration > shortGitCommandSlow(req.Args)
			isSlowFetch := fetchDuration > 10*time.Second
			if honey.Enabled() || traceLogs || isSlow || isSlowFetch {
				act := actor.FromContext(ctx)
				ev := honey.NewEvent("gitserver-exec")
				ev.SetSampleRate(honeySampleRate(cmd, act))
				ev.AddField("repo", repoName)
				ev.AddField("cmd", cmd)
				ev.AddField("args", args)
				ev.AddField("actor", act.UIDString())
				ev.AddField("ensure_revision", req.EnsureRevision)
				ev.AddField("ensure_revision_status", ensureRevisionStatus)
				ev.AddField("client", userAgent)
				ev.AddField("duration_ms", duration.Milliseconds())
				ev.AddField("stdin_size", len(req.Stdin))
				ev.AddField("stdout_size", stdoutN)
				ev.AddField("stderr_size", stderrN)
				ev.AddField("exit_status", exitStatus)
				ev.AddField("status", status)
				if execErr != nil {
					ev.AddField("error", execErr.Error())
				}
				if !cmdStart.IsZero() {
					ev.AddField("cmd_duration_ms", cmdDuration.Milliseconds())
					ev.AddField("fetch_duration_ms", fetchDuration.Milliseconds())
				}

				if traceID := trace.ID(ctx); traceID != "" {
					ev.AddField("traceID", traceID)
					ev.AddField("trace", trace.URL(traceID, conf.DefaultClient()))
				}

				if honey.Enabled() {
					_ = ev.Send()
				}

				if traceLogs {
					logger.Debug("TRACE gitserver exec", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
				}
				if isSlow {
					logger.Warn("Long exec request", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
				}
				if isSlowFetch {
					logger.Warn("Slow fetch/clone for exec request", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
				}
			}
		}()
	}

	if notFoundPayload, cloned := s.maybeStartClone(ctx, logger, repoName); !cloned {
		if notFoundPayload.CloneInProgress {
			status = "clone-in-progress"
		} else {
			status = "repo-not-found"
		}

		return execStatus{}, &NotFoundError{notFoundPayload}
	}

	dir := repoDirFromName(s.ReposDir, repoName)
	if s.ensureRevision(ctx, repoName, req.EnsureRevision, dir) {
		ensureRevisionStatus = "fetched"
	}

	// Special-case `git rev-parse HEAD` requests. These are invoked by search queries for every repo in scope.
	// For searches over large repo sets (> 1k), this leads to too many child process execs, which can lead
	// to a persistent failure mode where every exec takes > 10s, which is disastrous for gitserver performance.
	if len(req.Args) == 2 && req.Args[0] == "rev-parse" && req.Args[1] == "HEAD" {
		if resolved, err := quickRevParseHead(dir); err == nil && isAbsoluteRevision(resolved) {
			_, _ = w.Write([]byte(resolved))
			return execStatus{}, nil
		}
	}

	// Special-case `git symbolic-ref HEAD` requests. These are invoked by resolvers determining the default branch of a repo.
	// For searches over large repo sets (> 1k), this leads to too many child process execs, which can lead
	// to a persistent failure mode where every exec takes > 10s, which is disastrous for gitserver performance.
	if len(req.Args) == 2 && req.Args[0] == "symbolic-ref" && req.Args[1] == "HEAD" {
		if resolved, err := quickSymbolicRefHead(dir); err == nil {
			_, _ = w.Write([]byte(resolved))
			return execStatus{}, nil
		}
	}

	var stderrBuf bytes.Buffer
	stdoutW := &writeCounter{w: w}
	stderrW := &writeCounter{w: &limitWriter{W: &stderrBuf, N: 1024}}

	cmdStart = time.Now()
	cmd := s.RecordingCommandFactory.Command(ctx, s.Logger, string(repoName), "git", req.Args...)
	dir.Set(cmd.Unwrap())
	cmd.Unwrap().Stdout = stdoutW
	cmd.Unwrap().Stderr = stderrW
	cmd.Unwrap().Stdin = bytes.NewReader(req.Stdin)

	exitStatus, execErr = runCommand(ctx, cmd)

	status = strconv.Itoa(exitStatus)
	stdoutN = stdoutW.n
	stderrN = stderrW.n

	stderr := stderrBuf.String()
	s.logIfCorrupt(ctx, repoName, dir, stderr)

	return execStatus{
		Err:        execErr,
		Stderr:     stderr,
		ExitStatus: exitStatus,
	}, nil
}

// execHTTP translates the results of an exec into the expected HTTP statuses and payloads
func (s *Server) execHTTP(w http.ResponseWriter, r *http.Request, req *protocol.ExecRequest) {
	logger := s.Logger.Scoped("exec", "").With(log.Strings("req.Args", req.Args))

	// Flush writes more aggressively than standard net/http so that clients
	// with a context deadline see as much partial response body as possible.
	if fw := newFlushingResponseWriter(logger, w); fw != nil {
		w = fw
		defer fw.Close()
	}

	ctx := r.Context()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-cache")

	w.Header().Set("Trailer", "X-Exec-Error")
	w.Header().Add("Trailer", "X-Exec-Exit-Status")
	w.Header().Add("Trailer", "X-Exec-Stderr")

	execStatus, err := s.exec(ctx, logger, req, r.UserAgent(), w)
	w.Header().Set("X-Exec-Error", errorString(execStatus.Err))
	w.Header().Set("X-Exec-Exit-Status", strconv.Itoa(execStatus.ExitStatus))
	w.Header().Set("X-Exec-Stderr", execStatus.Stderr)
	if err != nil {
		if v := (&NotFoundError{}); errors.As(err, &v) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(v.Payload)

		} else if errors.Is(err, ErrInvalidCommand) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid command"))

		} else {
			// If it's not a well-known error, send the error text
			// and a generic error code.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
		}
	}
}

func (s *Server) handleP4Exec(w http.ResponseWriter, r *http.Request) {
	var req protocol.P4ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Args) < 1 {
		http.Error(w, "args must be greater than or equal to 1", http.StatusBadRequest)
		return
	}

	// Make sure the subcommand is explicitly allowed
	allowlist := []string{"protects", "groups", "users", "group", "changes"}
	allowed := false
	for _, arg := range allowlist {
		if req.Args[0] == arg {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, fmt.Sprintf("subcommand %q is not allowed", req.Args[0]), http.StatusBadRequest)
		return
	}

	// Log which actor is accessing p4-exec.
	//
	// p4-exec is currently only used for fetching user based permissions information
	// so, we don't have a repo name.
	accesslog.Record(r.Context(), "<no-repo>",
		log.String("p4user", req.P4User),
		log.String("p4port", req.P4Port),
		log.Strings("args", req.Args),
	)

	// Make sure credentials are valid before heavier operation
	err := p4testWithTrust(r.Context(), req.P4Port, req.P4User, req.P4Passwd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.p4execHTTP(w, r, &req)
}

func (s *Server) p4execHTTP(w http.ResponseWriter, r *http.Request, req *protocol.P4ExecRequest) {
	logger := s.Logger.Scoped("p4exec", "")

	// Flush writes more aggressively than standard net/http so that clients
	// with a context deadline see as much partial response body as possible.
	if fw := newFlushingResponseWriter(logger, w); fw != nil {
		w = fw
		defer fw.Close()
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()

	w.Header().Set("Trailer", "X-Exec-Error")
	w.Header().Add("Trailer", "X-Exec-Exit-Status")
	w.Header().Add("Trailer", "X-Exec-Stderr")
	w.WriteHeader(http.StatusOK)

	execStatus := s.p4Exec(ctx, logger, req, r.UserAgent(), w)
	w.Header().Set("X-Exec-Error", errorString(execStatus.Err))
	w.Header().Set("X-Exec-Exit-Status", strconv.Itoa(execStatus.ExitStatus))
	w.Header().Set("X-Exec-Stderr", execStatus.Stderr)

}

func (s *Server) p4Exec(ctx context.Context, logger log.Logger, req *protocol.P4ExecRequest, userAgent string, w io.Writer) execStatus {

	start := time.Now()
	var cmdStart time.Time // set once we have ensured commit
	exitStatus := unsetExitStatus
	var stdoutN, stderrN int64
	var status string
	var execErr error

	// Instrumentation
	{
		cmd := ""
		if len(req.Args) > 0 {
			cmd = req.Args[0]
		}
		args := strings.Join(req.Args, " ")

		var tr trace.Trace
		tr, ctx = trace.New(ctx, "p4exec."+cmd, attribute.String("port", req.P4Port))
		tr.SetAttributes(attribute.String("args", args))
		logger = logger.WithTrace(trace.Context(ctx))

		execRunning.WithLabelValues(cmd).Inc()
		defer func() {
			tr.AddEvent("done",
				attribute.String("status", status),
				attribute.Int64("stdout", stdoutN),
				attribute.Int64("stderr", stderrN),
			)
			tr.SetError(execErr)
			tr.End()

			duration := time.Since(start)
			execRunning.WithLabelValues(cmd).Dec()
			execDuration.WithLabelValues(cmd, status).Observe(duration.Seconds())

			var cmdDuration time.Duration
			if !cmdStart.IsZero() {
				cmdDuration = time.Since(cmdStart)
			}

			isSlow := cmdDuration > 30*time.Second
			if honey.Enabled() || traceLogs || isSlow {
				act := actor.FromContext(ctx)
				ev := honey.NewEvent("gitserver-p4exec")
				ev.SetSampleRate(honeySampleRate(cmd, act))
				ev.AddField("p4port", req.P4Port)
				ev.AddField("cmd", cmd)
				ev.AddField("args", args)
				ev.AddField("actor", act.UIDString())
				ev.AddField("client", userAgent)
				ev.AddField("duration_ms", duration.Milliseconds())
				ev.AddField("stdout_size", stdoutN)
				ev.AddField("stderr_size", stderrN)
				ev.AddField("exit_status", exitStatus)
				ev.AddField("status", status)
				if execErr != nil {
					ev.AddField("error", execErr.Error())
				}
				if !cmdStart.IsZero() {
					ev.AddField("cmd_duration_ms", cmdDuration.Milliseconds())
				}

				if traceID := trace.ID(ctx); traceID != "" {
					ev.AddField("traceID", traceID)
					ev.AddField("trace", trace.URL(traceID, conf.DefaultClient()))
				}

				_ = ev.Send()

				if traceLogs {
					logger.Debug("TRACE gitserver p4exec", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
				}
				if isSlow {
					logger.Warn("Long p4exec request", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
				}
			}
		}()
	}

	var stderrBuf bytes.Buffer
	stdoutW := &writeCounter{w: w}
	stderrW := &writeCounter{w: &limitWriter{W: &stderrBuf, N: 1024}}

	cmdStart = time.Now()
	cmd := exec.CommandContext(ctx, "p4", req.Args...)
	cmd.Env = append(os.Environ(),
		"P4PORT="+req.P4Port,
		"P4USER="+req.P4User,
		"P4PASSWD="+req.P4Passwd,
	)
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	exitStatus, execErr = runCommand(ctx, s.RecordingCommandFactory.Wrap(ctx, s.Logger, cmd))

	status = strconv.Itoa(exitStatus)
	stdoutN = stdoutW.n
	stderrN = stderrW.n

	stderr := stderrBuf.String()

	return execStatus{
		ExitStatus: exitStatus,
		Stderr:     stderr,
		Err:        execErr,
	}
}

func setLastFetched(ctx context.Context, db database.DB, shardID string, dir common.GitDir, name api.RepoName) error {
	lastFetched, err := repoLastFetched(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to get last fetched for %s", name)
	}

	lastChanged, err := repoLastChanged(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to get last changed for %s", name)
	}

	return db.GitserverRepos().SetLastFetched(ctx, name, database.GitserverFetchData{
		LastFetched: lastFetched,
		LastChanged: lastChanged,
		ShardID:     shardID,
	})
}

// setLastErrorNonFatal will set the last_error column for the repo in the gitserver table.
func (s *Server) setLastErrorNonFatal(ctx context.Context, name api.RepoName, err error) {
	var errString string
	if err != nil {
		errString = err.Error()
	}

	if err := s.DB.GitserverRepos().SetLastError(ctx, name, errString, s.Hostname); err != nil {
		s.Logger.Warn("Setting last error in DB", log.Error(err))
	}
}

func (s *Server) logIfCorrupt(ctx context.Context, repo api.RepoName, dir common.GitDir, stderr string) {
	if checkMaybeCorruptRepo(s.Logger, s.RecordingCommandFactory, repo, s.ReposDir, dir, stderr) {
		reason := stderr
		if err := s.DB.GitserverRepos().LogCorruption(ctx, repo, reason, s.Hostname); err != nil {
			s.Logger.Warn("failed to log repo corruption", log.String("repo", string(repo)), log.Error(err))
		}
	}
}

// setGitAttributes writes our global gitattributes to
// gitDir/info/attributes. This will override .gitattributes inside of
// repositories. It is used to unset attributes such as export-ignore.
func setGitAttributes(dir common.GitDir) error {
	infoDir := dir.Path("info")
	if err := os.Mkdir(infoDir, os.ModePerm); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "failed to set git attributes")
	}

	_, err := fileutil.UpdateFileIfDifferent(
		filepath.Join(infoDir, "attributes"),
		[]byte(`# Managed by Sourcegraph gitserver.

# We want every file to be present in git archive.
* -export-ignore
`))
	if err != nil {
		return errors.Wrap(err, "failed to set git attributes")
	}
	return nil
}

// testRepoCorrupter is used by tests to disrupt a cloned repository (e.g. deleting
// HEAD, zeroing it out, etc.)
var testRepoCorrupter func(ctx context.Context, tmpDir common.GitDir)

// cloneOptions specify optional behaviour for the cloneRepo function.
type CloneOptions struct {
	// Block will wait for the clone to finish before returning. If the clone
	// fails, the error will be returned. The passed in context is
	// respected. When not blocking the clone is done with a server background
	// context.
	Block bool

	// Overwrite will overwrite the existing clone.
	Overwrite bool
}

// CloneRepo performs a clone operation for the given repository. It is
// non-blocking by default.
func (s *Server) CloneRepo(ctx context.Context, repo api.RepoName, opts CloneOptions) (cloneProgress string, err error) {
	if isAlwaysCloningTest(repo) {
		return "This will never finish cloning", nil
	}

	dir := repoDirFromName(s.ReposDir, repo)

	// PERF: Before doing the network request to check if isCloneable, lets
	// ensure we are not already cloning.
	if progress, cloneInProgress := s.Locker.Status(dir); cloneInProgress {
		return progress, nil
	}

	// We always want to store whether there was an error cloning the repo, but only
	// after we checked if a clone is already in progress, otherwise we would race with
	// the actual running clone for the DB state of last_error.
	defer func() {
		// Use a different context in case we failed because the original context failed.
		s.setLastErrorNonFatal(s.ctx, repo, err)
	}()

	syncer, err := s.GetVCSSyncer(ctx, repo)
	if err != nil {
		return "", errors.Wrap(err, "get VCS syncer")
	}

	// We may be attempting to clone a private repo so we need an internal actor.
	remoteURL, err := s.getRemoteURL(actor.WithInternalActor(ctx), repo)
	if err != nil {
		return "", err
	}

	// isCloneable causes a network request, so we limit the number that can
	// run at one time. We use a separate semaphore to cloning since these
	// checks being blocked by a few slow clones will lead to poor feedback to
	// users. We can defer since the rest of the function does not block this
	// goroutine.
	ctx, cancel, err := s.acquireCloneableLimiter(ctx)
	if err != nil {
		return "", err // err will be a context error
	}
	defer cancel()

	if err = s.RPSLimiter.Wait(ctx); err != nil {
		return "", err
	}

	if err := syncer.IsCloneable(ctx, repo, remoteURL); err != nil {
		redactedErr := urlredactor.New(remoteURL).Redact(err.Error())
		return "", errors.Errorf("error cloning repo: repo %s not cloneable: %s", repo, redactedErr)
	}

	// Mark this repo as currently being cloned. We have to check again if someone else isn't already
	// cloning since we released the lock. We released the lock since isCloneable is a potentially
	// slow operation.
	lock, ok := s.Locker.TryAcquire(dir, "starting clone")
	if !ok {
		// Someone else beat us to it
		status, _ := s.Locker.Status(dir)
		return status, nil
	}

	if s.skipCloneForTests {
		lock.Release()
		return "", nil
	}

	if opts.Block {
		ctx, cancel, err := s.acquireCloneLimiter(ctx)
		if err != nil {
			return "", err
		}
		defer cancel()

		// We are blocking, so use the passed in context.
		err = s.doClone(ctx, repo, dir, syncer, lock, remoteURL, opts)
		err = errors.Wrapf(err, "failed to clone %s", repo)
		return "", err
	}

	// We push the cloneJob to a queue and let the producer-consumer pipeline take over from this
	// point. See definitions of cloneJobProducer and cloneJobConsumer to understand how these jobs
	// are processed.
	s.CloneQueue.Push(&cloneJob{
		repo:      repo,
		dir:       dir,
		syncer:    syncer,
		lock:      lock,
		remoteURL: remoteURL,
		options:   opts,
	})

	return "", nil
}

func (s *Server) doClone(
	ctx context.Context,
	repo api.RepoName,
	dir common.GitDir,
	syncer VCSSyncer,
	lock RepositoryLock,
	remoteURL *vcs.URL,
	opts CloneOptions,
) (err error) {
	logger := s.Logger.Scoped("doClone", "").With(log.String("repo", string(repo)))

	defer lock.Release()
	defer func() {
		if err != nil {
			repoCloneFailedCounter.Inc()
		}
	}()
	if err := s.RPSLimiter.Wait(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, conf.GitLongCommandTimeout())
	defer cancel()

	dstPath := string(dir)
	if !opts.Overwrite {
		// We clone to a temporary directory first, so avoid wasting resources
		// if the directory already exists.
		if _, err := os.Stat(dstPath); err == nil {
			return &os.PathError{
				Op:   "cloneRepo",
				Path: dstPath,
				Err:  os.ErrExist,
			}
		}
	}

	// We clone to a temporary location first to avoid having incomplete
	// clones in the repo tree. This also avoids leaving behind corrupt clones
	// if the clone is interrupted.
	tmpPath, err := tempDir(s.ReposDir, "clone-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpPath)
	tmpPath = filepath.Join(tmpPath, ".git")
	tmp := common.GitDir(tmpPath)

	// It may already be cloned
	if !repoCloned(dir) {
		if err := s.DB.GitserverRepos().SetCloneStatus(ctx, repo, types.CloneStatusCloning, s.Hostname); err != nil {
			s.Logger.Warn("Setting clone status in DB", log.Error(err))
		}
	}
	defer func() {
		// Use a background context to ensure we still update the DB even if we time out
		if err := s.DB.GitserverRepos().SetCloneStatus(context.Background(), repo, cloneStatus(repoCloned(dir), false), s.Hostname); err != nil {
			s.Logger.Warn("Setting clone status in DB", log.Error(err))
		}
	}()

	cmd, err := syncer.CloneCommand(ctx, remoteURL, tmpPath)
	if err != nil {
		return errors.Wrap(err, "get clone command")
	}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}

	// see issue #7322: skip LFS content in repositories with Git LFS configured
	cmd.Env = append(cmd.Env, "GIT_LFS_SKIP_SMUDGE=1")
	logger.Info("cloning repo", log.String("tmp", tmpPath), log.String("dst", dstPath))

	pr, pw := io.Pipe()
	defer pw.Close()

	redactor := urlredactor.New(remoteURL)

	go readCloneProgress(s.DB, logger, redactor, lock, pr, repo)

	output, err := runRemoteGitCommand(ctx, s.RecordingCommandFactory.WrapWithRepoName(ctx, s.Logger, repo, cmd).WithRedactorFunc(redactor.Redact), true, pw)
	redactedOutput := redactor.Redact(string(output))
	// best-effort update the output of the clone
	if err := s.DB.GitserverRepos().SetLastOutput(context.Background(), repo, redactedOutput); err != nil {
		s.Logger.Warn("Setting last output in DB", log.Error(err))
	}

	if err != nil {
		return errors.Wrapf(err, "clone failed. Output: %s", redactedOutput)
	}

	if testRepoCorrupter != nil {
		testRepoCorrupter(ctx, tmp)
	}

	if err := postRepoFetchActions(ctx, logger, s.DB, s.Hostname, s.RecordingCommandFactory, s.ReposDir, repo, tmp, remoteURL, syncer); err != nil {
		return err
	}

	if opts.Overwrite {
		// remove the current repo by putting it into our temporary directory
		err := fileutil.RenameAndSync(dstPath, filepath.Join(filepath.Dir(tmpPath), "old"))
		if err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to remove old clone")
		}
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), os.ModePerm); err != nil {
		return err
	}
	if err := fileutil.RenameAndSync(tmpPath, dstPath); err != nil {
		return err
	}

	logger.Info("repo cloned")
	repoClonedCounter.Inc()

	s.Perforce.EnqueueChangelistMappingJob(perforce.NewChangelistMappingJob(repo, dir))

	return nil
}

func postRepoFetchActions(
	ctx context.Context,
	logger log.Logger,
	db database.DB,
	shardID string,
	rcf *wrexec.RecordingCommandFactory,
	reposDir string,
	repo api.RepoName,
	dir common.GitDir,
	remoteURL *vcs.URL,
	syncer VCSSyncer,
) error {
	if err := removeBadRefs(ctx, dir); err != nil {
		logger.Warn("failed to remove bad refs", log.String("repo", string(repo)), log.Error(err))
	}

	if err := setHEAD(ctx, logger, rcf, repo, dir, syncer, remoteURL); err != nil {
		return errors.Wrapf(err, "failed to ensure HEAD exists for repo %q", repo)
	}

	if err := setRepositoryType(rcf, reposDir, dir, syncer.Type()); err != nil {
		return errors.Wrapf(err, "failed to set repository type for repo %q", repo)
	}

	if err := setGitAttributes(dir); err != nil {
		return errors.Wrap(err, "setting git attributes")
	}

	if err := gitSetAutoGC(rcf, reposDir, dir); err != nil {
		return errors.Wrap(err, "setting git gc mode")
	}

	// Update the last-changed stamp on disk.
	if err := setLastChanged(logger, dir); err != nil {
		return errors.Wrap(err, "failed to update last changed time")
	}

	// Successfully updated, best-effort updating of db fetch state based on
	// disk state.
	if err := setLastFetched(ctx, db, shardID, dir, repo); err != nil {
		logger.Warn("failed setting last fetch in DB", log.Error(err))
	}

	// Successfully updated, best-effort calculation of the repo size.
	repoSizeBytes := dirSize(dir.Path("."))
	if err := db.GitserverRepos().SetRepoSize(ctx, repo, repoSizeBytes, shardID); err != nil {
		logger.Warn("failed to set repo size", log.Error(err))
	}

	return nil
}

// readCloneProgress scans the reader and saves the most recent line of output
// as the lock status.
func readCloneProgress(db database.DB, logger log.Logger, redactor *urlredactor.URLRedactor, lock RepositoryLock, pr io.Reader, repo api.RepoName) {
	// Use a background context to ensure we still update the DB even if we
	// time out. IE we intentionally don't take an input ctx.
	ctx := featureflag.WithFlags(context.Background(), db.FeatureFlags())

	var logFile *os.File
	var err error

	if conf.Get().CloneProgressLog {
		logFile, err = os.CreateTemp("", "")
		if err != nil {
			logger.Warn("failed to create temporary clone log file", log.Error(err), log.String("repo", string(repo)))
		} else {
			logger.Info("logging clone output", log.String("file", logFile.Name()), log.String("repo", string(repo)))
			defer logFile.Close()
		}
	}

	dbWritesLimiter := rate.NewLimiter(rate.Limit(1.0), 1)
	scan := bufio.NewScanner(pr)
	scan.Split(scanCRLF)
	store := db.GitserverRepos()
	for scan.Scan() {
		progress := scan.Text()
		// 🚨 SECURITY: The output could include the clone url with may contain a sensitive token.
		// Redact the full url and any found HTTP credentials to be safe.
		//
		// e.g.
		// $ git clone http://token@github.com/foo/bar
		// Cloning into 'nick'...
		// fatal: repository 'http://token@github.com/foo/bar/' not found
		redactedProgress := redactor.Redact(progress)

		lock.SetStatus(redactedProgress)

		if logFile != nil {
			// Failing to write here is non-fatal and we don't want to spam our logs if there
			// are issues
			_, _ = fmt.Fprintln(logFile, progress)
		}
		// Only write to the database persisted status if line indicates progress
		// which is recognized by presence of a '%'. We filter these writes not to waste
		// rate-limit tokens on log lines that would not be relevant to the user.
		if featureflag.FromContext(ctx).GetBoolOr("clone-progress-logging", false) &&
			strings.Contains(redactedProgress, "%") &&
			dbWritesLimiter.Allow() {
			if err := store.SetCloningProgress(ctx, repo, redactedProgress); err != nil {
				logger.Error("error updating cloning progress in the db", log.Error(err))
			}
		}
	}
	if err := scan.Err(); err != nil {
		logger.Error("error reporting progress", log.Error(err))
	}
}

// scanCRLF is similar to bufio.ScanLines except it splits on both '\r' and '\n'
// and it does not return tokens that contain only whitespace.
func scanCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	trim := func(data []byte) []byte {
		data = bytes.TrimSpace(data)
		if len(data) == 0 {
			// Don't pass back a token that is all whitespace.
			return nil
		}
		return data
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, trim(data[:i]), nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), trim(data), nil
	}
	// Request more data.
	return 0, nil, nil
}

var (
	execRunning = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_gitserver_exec_running",
		Help: "number of gitserver.GitCommand running concurrently.",
	}, []string{"cmd"})
	execDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_gitserver_exec_duration_seconds",
		Help:    "gitserver.GitCommand latencies in seconds.",
		Buckets: trace.UserLatencyBuckets,
	}, []string{"cmd", "status"})

	searchRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_gitserver_search_running",
		Help: "number of gitserver.Search running concurrently.",
	})
	searchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_gitserver_search_duration_seconds",
		Help:    "gitserver.Search duration in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30},
	}, []string{"error"})
	searchLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "src_gitserver_search_latency_seconds",
		Help:    "gitserver.Search latency (time until first result is sent) in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30},
	})

	pendingClones = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_gitserver_clone_queue",
		Help: "number of repos waiting to be cloned.",
	})
	lsRemoteQueue = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_gitserver_lsremote_queue",
		Help: "number of repos waiting to check existence on remote code host (git ls-remote).",
	})
	repoClonedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repo_cloned",
		Help: "number of successful git clones run",
	})
	repoCloneFailedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repo_cloned_failed",
		Help: "number of failed git clones",
	})
)

// Send 1 in 16 events to honeycomb. This is hardcoded since we only use this
// for Sourcegraph.com.
//
// 2020-05-29 1 in 4. We are currently at the top tier for honeycomb (before
// enterprise) and using double our quota. This gives us room to grow. If you
// find we keep bumping this / missing data we care about we can look into
// more dynamic ways to sample in our application code.
//
// 2020-07-20 1 in 16. Again hitting very high usage. Likely due to recent
// scaling up of the indexed search cluster. Will require more investigation,
// but we should probably segment user request path traffic vs internal batch
// traffic.
//
// 2020-11-02 Dynamically sample. Again hitting very high usage. Same root
// cause as before, scaling out indexed search cluster. We update our sampling
// to instead be dynamic, since "rev-parse" is 12 times more likely than the
// next most common command.
//
// 2021-08-20 over two hours we did 128 * 128 * 1e6 rev-parse requests
// internally. So we update our sampling to heavily downsample internal
// rev-parse, while upping our sampling for non-internal.
// https://ui.honeycomb.io/sourcegraph/datasets/gitserver-exec/result/67e4bLvUddg
func honeySampleRate(cmd string, actor *actor.Actor) uint {
	// HACK(keegan) 2022-11-02 IsInternal on sourcegraph.com is always
	// returning false. For now I am also marking it internal if UID is not
	// set to work around us hammering honeycomb.
	internal := actor.IsInternal() || actor.UID == 0
	switch {
	case cmd == "rev-parse" && internal:
		return 1 << 14 // 16384

	case internal:
		// we care more about user requests, so downsample internal more.
		return 16

	default:
		return 8
	}
}

var headBranchPattern = lazyregexp.New(`HEAD branch: (.+?)\n`)

func (s *Server) doRepoUpdate(ctx context.Context, repo api.RepoName, revspec string) (err error) {
	tr, ctx := trace.New(ctx, "doRepoUpdate", repo.Attr())
	defer tr.EndWithErr(&err)

	s.repoUpdateLocksMu.Lock()
	l, ok := s.repoUpdateLocks[repo]
	if !ok {
		l = &locks{
			once: new(sync.Once),
			mu:   new(sync.Mutex),
		}
		s.repoUpdateLocks[repo] = l
	}
	once := l.once
	mu := l.mu
	s.repoUpdateLocksMu.Unlock()

	// doBackgroundRepoUpdate can block longer than our context deadline. done will
	// close when its done. We can return when either done is closed or our
	// deadline has passed.
	done := make(chan struct{})
	err = errors.New("another operation is already in progress")
	go func() {
		defer close(done)
		once.Do(func() {
			mu.Lock() // Prevent multiple updates in parallel. It works fine, but it wastes resources.
			defer mu.Unlock()

			s.repoUpdateLocksMu.Lock()
			l.once = new(sync.Once) // Make new requests wait for next update.
			s.repoUpdateLocksMu.Unlock()

			err = s.doBackgroundRepoUpdate(repo, revspec)
			if err != nil {
				// We don't want to spam our logs when the rate limiter has been set to block all
				// updates
				if !errors.Is(err, ratelimit.ErrBlockAll) {
					s.Logger.Error("performing background repo update", log.Error(err), log.String("repo", string(repo)), log.String("revspec", revspec))
				}

				// The repo update might have failed due to the repo being corrupt
				var gitErr *common.GitCommandError
				if errors.As(err, &gitErr) {
					s.logIfCorrupt(ctx, repo, repoDirFromName(s.ReposDir, repo), gitErr.Output)
				}
			}
			s.setLastErrorNonFatal(s.ctx, repo, err)
		})
	}()

	select {
	case <-done:
		return errors.Wrapf(err, "repo %s:", repo)
	case <-ctx.Done():
		return ctx.Err()
	}
}

var doBackgroundRepoUpdateMock func(api.RepoName) error

func (s *Server) doBackgroundRepoUpdate(repo api.RepoName, revspec string) error {
	logger := s.Logger.Scoped("backgroundRepoUpdate", "").With(log.String("repo", string(repo)))

	if doBackgroundRepoUpdateMock != nil {
		return doBackgroundRepoUpdateMock(repo)
	}
	// background context.
	ctx, cancel1 := s.serverContext()
	defer cancel1()

	// ensure the background update doesn't hang forever
	ctx, cancel2 := context.WithTimeout(ctx, conf.GitLongCommandTimeout())
	defer cancel2()

	// This background process should use our internal actor
	ctx = actor.WithInternalActor(ctx)

	ctx, cancel2, err := s.acquireCloneLimiter(ctx)
	if err != nil {
		return err
	}
	defer cancel2()

	if err = s.RPSLimiter.Wait(ctx); err != nil {
		return err
	}

	repo = protocol.NormalizeRepo(repo)
	dir := repoDirFromName(s.ReposDir, repo)

	remoteURL, err := s.getRemoteURL(ctx, repo)
	if err != nil {
		return errors.Wrapf(err, "failed to determine remote URL")
	}

	syncer, err := s.GetVCSSyncer(ctx, repo)
	if err != nil {
		return errors.Wrapf(err, "failed to get VCS syncer")
	}

	// drop temporary pack files after a fetch. this function won't
	// return until this fetch has completed or definitely-failed,
	// either way they can't still be in use. we don't care exactly
	// when the cleanup happens, just that it does.
	defer cleanTmpFiles(s.Logger, dir)

	output, err := syncer.Fetch(ctx, remoteURL, repo, dir, revspec)
	redactedOutput := urlredactor.New(remoteURL).Redact(string(output))
	// best-effort update the output of the fetch
	if err := s.DB.GitserverRepos().SetLastOutput(context.Background(), repo, redactedOutput); err != nil {
		s.Logger.Warn("Setting last output in DB", log.Error(err))
	}

	if err != nil {
		if output != nil {
			return errors.Wrapf(err, "failed to fetch repo %q with output %q", repo, redactedOutput)
		} else {
			return errors.Wrapf(err, "failed to fetch repo %q", repo)
		}
	}

	return postRepoFetchActions(ctx, logger, s.DB, s.Hostname, s.RecordingCommandFactory, s.ReposDir, repo, dir, remoteURL, syncer)
}

// older versions of git do not remove tags case insensitively, so we generate
// every possible case of HEAD (2^4 = 16)
var badRefs = syncx.OnceValue(func() []string {
	refs := make([]string, 0, 1<<4)
	for bits := uint8(0); bits < (1 << 4); bits++ {
		s := []byte("HEAD")
		for i, c := range s {
			// lowercase if the i'th bit of bits is 1
			if bits&(1<<i) != 0 {
				s[i] = c - 'A' + 'a'
			}
		}
		refs = append(refs, string(s))
	}
	return refs
})

// removeBadRefs removes bad refs and tags from the git repo at dir. This
// should be run after a clone or fetch. If your repository contains a ref or
// tag called HEAD (case insensitive), most commands will output a warning
// from git:
//
//	warning: refname 'HEAD' is ambiguous.
//
// Instead we just remove this ref.
func removeBadRefs(ctx context.Context, dir common.GitDir) (errs error) {
	args := append([]string{"branch", "-D"}, badRefs()...)
	cmd := exec.CommandContext(ctx, "git", args...)
	dir.Set(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// We expect to get a 1 exit code here, because ideally none of the bad refs
		// exist, this is fine. All other exit codes or errors are not.
		if ex, ok := err.(*exec.ExitError); !ok || ex.ExitCode() != 1 {
			errs = errors.Append(errs, errors.Wrap(err, string(out)))
		}
	}

	args = append([]string{"tag", "-d"}, badRefs()...)
	cmd = exec.CommandContext(ctx, "git", args...)
	dir.Set(cmd)
	out, err = cmd.CombinedOutput()
	if err != nil {
		// We expect to get a 1 exit code here, because ideally none of the bad refs
		// exist, this is fine. All other exit codes or errors are not.
		if ex, ok := err.(*exec.ExitError); !ok || ex.ExitCode() != 1 {
			errs = errors.Append(errs, errors.Wrap(err, string(out)))
		}
	}

	return errs
}

// ensureHEAD verifies that there is a HEAD file within the repo, and that it
// is of non-zero length. If either condition is met, we configure a
// best-effort default.
func ensureHEAD(dir common.GitDir) error {
	head, err := os.Stat(dir.Path("HEAD"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) || head.Size() == 0 {
		return os.WriteFile(dir.Path("HEAD"), []byte("ref: refs/heads/master"), 0o600)
	}
	return nil
}

// setHEAD configures git repo defaults (such as what HEAD is) which are
// needed for git commands to work.
func setHEAD(ctx context.Context, logger log.Logger, rcf *wrexec.RecordingCommandFactory, repoName api.RepoName, dir common.GitDir, syncer VCSSyncer, remoteURL *vcs.URL) error {
	// Verify that there is a HEAD file within the repo, and that it is of
	// non-zero length.
	if err := ensureHEAD(dir); err != nil {
		logger.Error("failed to ensure HEAD exists", log.Error(err), log.String("repo", string(repoName)))
	}

	// Fallback to git's default branch name if git remote show fails.
	headBranch := "master"

	// try to fetch HEAD from origin
	cmd, err := syncer.RemoteShowCommand(ctx, remoteURL)
	if err != nil {
		return errors.Wrap(err, "get remote show command")
	}
	dir.Set(cmd)
	r := urlredactor.New(remoteURL)
	output, err := runRemoteGitCommand(ctx, rcf.WrapWithRepoName(ctx, logger, repoName, cmd).WithRedactorFunc(r.Redact), true, nil)
	if err != nil {
		logger.Error("Failed to fetch remote info", log.Error(err), log.String("output", string(output)))
		return errors.Wrap(err, "failed to fetch remote info")
	}

	submatches := headBranchPattern.FindSubmatch(output)
	if len(submatches) == 2 {
		submatch := string(submatches[1])
		if submatch != "(unknown)" {
			headBranch = submatch
		}
	}

	// check if branch pointed to by HEAD exists
	cmd = exec.CommandContext(ctx, "git", "rev-parse", headBranch, "--")
	dir.Set(cmd)
	if err := cmd.Run(); err != nil {
		// branch does not exist, pick first branch
		cmd := exec.CommandContext(ctx, "git", "branch")
		dir.Set(cmd)
		output, err := cmd.Output()
		if err != nil {
			logger.Error("Failed to list branches", log.Error(err), log.String("output", string(output)))
			return errors.Wrap(err, "failed to list branches")
		}
		lines := strings.Split(string(output), "\n")
		branch := strings.TrimPrefix(strings.TrimPrefix(lines[0], "* "), "  ")
		if branch != "" {
			headBranch = branch
		}
	}

	// set HEAD
	cmd = exec.CommandContext(ctx, "git", "symbolic-ref", "HEAD", "refs/heads/"+headBranch)
	dir.Set(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Error("Failed to set HEAD", log.Error(err), log.String("output", string(output)))
		return errors.Wrap(err, "Failed to set HEAD")
	}

	return nil
}

// setLastChanged discerns an approximate last-changed timestamp for a
// repository. This can be approximate; it's used to determine how often we
// should run `git fetch`, but is not relied on strongly. The basic plan
// is as follows: If a repository has never had a timestamp before, we
// guess that the right stamp is *probably* the timestamp of the most
// chronologically-recent commit. If there are no commits, we just use the
// current time because that's probably usually a temporary state.
//
// If a timestamp already exists, we want to update it if and only if
// the set of references (as determined by `git show-ref`) has changed.
//
// To accomplish this, we assert that the file `sg_refhash` in the git
// directory should, if it exists, contain a hash of the output of
// `git show-ref`, and have a timestamp of "the last time this changed",
// except that if we're creating that file for the first time, we set
// it to the timestamp of the top commit. We then compute the hash of
// the show-ref output, and store it in the file if and only if it's
// different from the current contents.
//
// If show-ref fails, we use rev-list to determine whether that's just
// an empty repository (not an error) or some kind of actual error
// that is possibly causing our data to be incorrect, which should
// be reported.
func setLastChanged(logger log.Logger, dir common.GitDir) error {
	hashFile := dir.Path("sg_refhash")

	hash, err := computeRefHash(dir)
	if err != nil {
		return errors.Wrapf(err, "computeRefHash failed for %s", dir)
	}

	var stamp time.Time
	if _, err := os.Stat(hashFile); os.IsNotExist(err) {
		// This is the first time we are calculating the hash. Give a more
		// approriate timestamp for sg_refhash than the current time.
		stamp = computeLatestCommitTimestamp(logger, dir)
	}

	_, err = fileutil.UpdateFileIfDifferent(hashFile, hash)
	if err != nil {
		return errors.Wrapf(err, "failed to update %s", hashFile)
	}

	// If stamp is non-zero we have a more approriate mtime.
	if !stamp.IsZero() {
		err = os.Chtimes(hashFile, stamp, stamp)
		if err != nil {
			return errors.Wrapf(err, "failed to set mtime to the lastest commit timestamp for %s", dir)
		}
	}

	return nil
}

// computeLatestCommitTimestamp returns the timestamp of the most recent
// commit if any. If there are no commits or the latest commit is in the
// future, or there is any error, time.Now is returned.
func computeLatestCommitTimestamp(logger log.Logger, dir common.GitDir) time.Time {
	logger = logger.Scoped("computeLatestCommitTimestamp", "compute the timestamp of the most recent commit").
		With(log.String("repo", string(dir)))

	now := time.Now() // return current time if we don't find a more accurate time
	cmd := exec.Command("git", "rev-list", "--all", "--timestamp", "-n", "1")
	dir.Set(cmd)
	output, err := cmd.Output()
	// If we don't have a more specific stamp, we'll return the current time,
	// and possibly an error.
	if err != nil {
		logger.Warn("failed to execute, defaulting to time.Now", log.Error(err))
		return now
	}

	words := bytes.Split(output, []byte(" "))
	// An empty rev-list output, without an error, is okay.
	if len(words) < 2 {
		return now
	}

	// We should have a timestamp and a commit hash; format is
	// 1521316105 ff03fac223b7f16627b301e03bf604e7808989be
	epoch, err := strconv.ParseInt(string(words[0]), 10, 64)
	if err != nil {
		logger.Warn("ignoring corrupted timestamp, defaulting to time.Now", log.String("timestamp", string(words[0])))
		return now
	}
	stamp := time.Unix(epoch, 0)
	if stamp.After(now) {
		return now
	}
	return stamp
}

// computeRefHash returns a hash of the refs for dir. The hash should only
// change if the set of refs and the commits they point to change.
func computeRefHash(dir common.GitDir) ([]byte, error) {
	// Do not use CommandContext since this is a fast operation we do not want
	// to interrupt.
	cmd := exec.Command("git", "show-ref")
	dir.Set(cmd)
	output, err := cmd.Output()
	if err != nil {
		// Ignore the failure for an empty repository: show-ref fails with
		// empty output and an exit code of 1
		var e *exec.ExitError
		if !errors.As(err, &e) || len(output) != 0 || len(e.Stderr) != 0 || e.Sys().(syscall.WaitStatus).ExitStatus() != 1 {
			return nil, err
		}
	}

	lines := bytes.Split(output, []byte("\n"))
	sort.Slice(lines, func(i, j int) bool {
		return bytes.Compare(lines[i], lines[j]) < 0
	})
	hasher := sha256.New()
	for _, b := range lines {
		_, _ = hasher.Write(b)
		_, _ = hasher.Write([]byte("\n"))
	}
	hash := make([]byte, hex.EncodedLen(hasher.Size()))
	hex.Encode(hash, hasher.Sum(nil))
	return hash, nil
}

func (s *Server) ensureRevision(ctx context.Context, repo api.RepoName, rev string, repoDir common.GitDir) (didUpdate bool) {
	if rev == "" || rev == "HEAD" {
		return false
	}
	if conf.Get().DisableAutoGitUpdates {
		// ensureRevision may kick off a git fetch operation which we don't want if we've
		// configured DisableAutoGitUpdates.
		return false
	}

	// rev-parse on an OID does not check if the commit actually exists, so it always
	// works. So we append ^0 to force the check
	if isAbsoluteRevision(rev) {
		rev = rev + "^0"
	}
	cmd := exec.Command("git", "rev-parse", rev, "--")
	repoDir.Set(cmd)
	// TODO: Check here that it's actually been a rev-parse error, and not something else.
	if err := cmd.Run(); err == nil {
		return false
	}
	// Revision not found, update before returning.
	err := s.doRepoUpdate(ctx, repo, rev)
	if err != nil {
		s.Logger.Warn("failed to perform background repo update", log.Error(err), log.String("repo", string(repo)), log.String("rev", rev))
		// TODO: Shouldn't we return false here?
	}
	return true
}

const headFileRefPrefix = "ref: "

// quickSymbolicRefHead best-effort mimics the execution of `git symbolic-ref HEAD`, but doesn't exec a child process.
// It just reads the .git/HEAD file from the bare git repository directory.
func quickSymbolicRefHead(dir common.GitDir) (string, error) {
	// See if HEAD contains a commit hash and fail if so.
	head, err := os.ReadFile(dir.Path("HEAD"))
	if err != nil {
		return "", err
	}
	head = bytes.TrimSpace(head)
	if isAbsoluteRevision(string(head)) {
		return "", errors.New("ref HEAD is not a symbolic ref")
	}

	// HEAD doesn't contain a commit hash. It contains something like "ref: refs/heads/master".
	if !bytes.HasPrefix(head, []byte(headFileRefPrefix)) {
		return "", errors.New("unrecognized HEAD file format")
	}
	headRef := bytes.TrimPrefix(head, []byte(headFileRefPrefix))
	return string(headRef), nil
}

// quickRevParseHead best-effort mimics the execution of `git rev-parse HEAD`, but doesn't exec a child process.
// It just reads the relevant files from the bare git repository directory.
func quickRevParseHead(dir common.GitDir) (string, error) {
	// See if HEAD contains a commit hash and return it if so.
	head, err := os.ReadFile(dir.Path("HEAD"))
	if err != nil {
		return "", err
	}
	head = bytes.TrimSpace(head)
	if h := string(head); isAbsoluteRevision(h) {
		return h, nil
	}

	// HEAD doesn't contain a commit hash. It contains something like "ref: refs/heads/master".
	if !bytes.HasPrefix(head, []byte(headFileRefPrefix)) {
		return "", errors.New("unrecognized HEAD file format")
	}
	// Look for the file in refs/heads. If it exists, it contains the commit hash.
	headRef := bytes.TrimPrefix(head, []byte(headFileRefPrefix))
	if bytes.HasPrefix(headRef, []byte("../")) || bytes.Contains(headRef, []byte("/../")) || bytes.HasSuffix(headRef, []byte("/..")) {
		// 🚨 SECURITY: prevent leakage of file contents outside repo dir
		return "", errors.Errorf("invalid ref format: %s", headRef)
	}
	headRefFile := dir.Path(filepath.FromSlash(string(headRef)))
	if refs, err := os.ReadFile(headRefFile); err == nil {
		return string(bytes.TrimSpace(refs)), nil
	}

	// File didn't exist in refs/heads. Look for it in packed-refs.
	f, err := os.Open(dir.Path("packed-refs"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 2 {
			continue
		}
		commit, ref := fields[0], fields[1]
		if bytes.Equal(ref, headRef) {
			return string(commit), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	// Didn't find the refs/heads/$HEAD_BRANCH in packed_refs
	return "", errors.New("could not compute `git rev-parse HEAD` in-process, try running `git` process")
}

// errorString returns the error string. If err is nil it returns the empty
// string.
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// IsAbsoluteRevision checks if the revision is a git OID SHA string.
//
// Note: This doesn't mean the SHA exists in a repository, nor does it mean it
// isn't a ref. Git allows 40-char hexadecimal strings to be references.
//
// copied from internal/vcs/git to avoid cyclic import
func isAbsoluteRevision(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if !(('0' <= r && r <= '9') ||
			('a' <= r && r <= 'f') ||
			('A' <= r && r <= 'F')) {
			return false
		}
	}
	return true
}
