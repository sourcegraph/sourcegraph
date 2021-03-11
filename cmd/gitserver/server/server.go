// Package server implements the gitserver service.
package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
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

	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/honey"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/mutablelimiter"
	"github.com/sourcegraph/sourcegraph/internal/repotrackutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
)

// tempDirName is the name used for the temporary directory under ReposDir.
const tempDirName = ".tmp"

// traceLogs is controlled via the env SRC_GITSERVER_TRACE. If true we trace
// logs to stderr
var traceLogs bool

var lastCheckAt = make(map[api.RepoName]time.Time)
var lastCheckMutex sync.Mutex

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

// runCommandMock is set by tests. When non-nil it is run instead of
// runCommand
var runCommandMock func(context.Context, *exec.Cmd) (int, error)

// runCommand runs the command and returns the exit status. All clients of this function should set the context
// in cmd themselves, but we have to pass the context separately here for the sake of tracing.
func runCommand(ctx context.Context, cmd *exec.Cmd) (exitCode int, err error) {
	if runCommandMock != nil {
		return runCommandMock(ctx, cmd)
	}
	span, _ := ot.StartSpanFromContext(ctx, "runCommand")
	span.SetTag("path", cmd.Path)
	span.SetTag("args", cmd.Args)
	span.SetTag("dir", cmd.Dir)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
			span.SetTag("exitCode", exitCode)
		}
		span.Finish()
	}()

	err = cmd.Run()
	exitStatus := -10810
	if cmd.ProcessState != nil { // is nil if process failed to start
		exitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	}
	return exitStatus, err
}

// Server is a gitserver server.
type Server struct {
	// ReposDir is the path to the base directory for gitserver storage.
	ReposDir string

	// DesiredPercentFree is the desired percentage of disk space to keep free.
	DesiredPercentFree int

	// DiskSizer tells how much disk is free and how large the disk is.
	DiskSizer DiskSizer

	// GetRemoteURLFunc is a function which returns the remote URL for a
	// repository. This is used when cloning or fetching a repository. In
	// production this will speak to the database to look up the clone URL. In
	// tests this is usually set to clone a local repository or intentionally
	// error.
	//
	// Note: internal uses should call getRemoteURL which will handle
	// GetRemoteURLFunc being nil.
	GetRemoteURLFunc func(context.Context, api.RepoName) (string, error)

	// GetVCSSyncer is a function which returns the VCS syncer for a repository.
	// This is used when cloning or fetching a repository. In production this will
	// speak to the database to determine the code host type. In tests this is
	// usually set to return a GitRepoSyncer.
	GetVCSSyncer func(context.Context, api.RepoName) (VCSSyncer, error)

	// Hostname is how we identify this instance of gitserver. Generally it is the
	// actual hostname but can also be overridden by the HOSTNAME environment variable.
	Hostname string

	// shared db handle
	DB dbutil.DB

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

	locker *RepositoryLocker

	// cloneLimiter and cloneableLimiter limits the number of concurrent
	// clones and ls-remotes respectively. Use s.acquireCloneLimiter() and
	// s.acquireClonableLimiter() instead of using these directly.
	cloneLimiter     *mutablelimiter.Limiter
	cloneableLimiter *mutablelimiter.Limiter

	repoUpdateLocksMu sync.Mutex // protects the map below and also updates to locks.once
	repoUpdateLocks   map[api.RepoName]*locks
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
		return longGitCommandTimeout

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

// This is a timeout for long git commands like clone or remote update.
// that may take a while for large repos. These types of commands should
// be run in the background.
var longGitCommandTimeout = time.Hour

// Handler returns the http.Handler that should be used to serve requests.
func (s *Server) Handler() http.Handler {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.locker = &RepositoryLocker{}
	s.repoUpdateLocks = make(map[api.RepoName]*locks)

	// GitMaxConcurrentClones controls the maximum number of clones that
	// can happen at once on a single gitserver.
	// Used to prevent throttle limits from a code host. Defaults to 5.
	//
	// The new repo-updater scheduler enforces the rate limit across all gitserver,
	// so ideally this logic could be removed here; however, ensureRevision can also
	// cause an update to happen and it is called on every exec command.
	maxConcurrentClones := conf.Get().GitMaxConcurrentClones
	if maxConcurrentClones == 0 {
		maxConcurrentClones = 5
	}
	s.cloneLimiter = mutablelimiter.New(maxConcurrentClones)
	s.cloneableLimiter = mutablelimiter.New(maxConcurrentClones)
	conf.Watch(func() {
		limit := conf.Get().GitMaxConcurrentClones
		if limit == 0 {
			limit = 5
		}
		s.cloneLimiter.SetLimit(limit)
		s.cloneableLimiter.SetLimit(limit)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/archive", s.handleArchive)
	mux.HandleFunc("/exec", s.handleExec)
	mux.HandleFunc("/p4-exec", s.handleP4Exec)
	mux.HandleFunc("/list", s.handleList)
	mux.HandleFunc("/list-gitolite", s.handleListGitolite)
	mux.HandleFunc("/is-repo-cloneable", s.handleIsRepoCloneable)
	mux.HandleFunc("/is-repo-cloned", s.handleIsRepoCloned)
	mux.HandleFunc("/repos", s.handleRepoInfo)
	mux.HandleFunc("/repos-stats", s.handleReposStats)
	mux.HandleFunc("/repo-clone-progress", s.handleRepoCloneProgress)
	mux.HandleFunc("/delete", s.handleRepoDelete)
	mux.HandleFunc("/repo-update", s.handleRepoUpdate)
	mux.HandleFunc("/getGitolitePhabricatorMetadata", s.handleGetGitolitePhabricatorMetadata)
	mux.HandleFunc("/create-commit-from-patch", s.handleCreateCommitFromPatch)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle("/git/", http.StripPrefix("/git", &gitServiceHandler{
		Dir: func(d string) string { return string(s.dir(api.RepoName(d))) },
	}))

	return mux
}

// Janitor does clean up tasks over s.ReposDir and is expected to run in a
// background goroutine.
func (s *Server) Janitor(interval time.Duration) {
	for {
		addrs := conf.Get().ServiceConnections.GitServers
		s.cleanupRepos(addrs)
		time.Sleep(interval)
	}
}

// SyncRepoState syncs state on disk to the database for all repos and is expected to
// run in a background goroutine.
func (s *Server) SyncRepoState(interval time.Duration, batchSize, perSecond int) {
	for {
		addrs := conf.Get().ServiceConnections.GitServers
		if err := s.syncRepoState(addrs, batchSize, perSecond); err != nil {
			log15.Error("Syncing repo state", "error ", err)
		}
		time.Sleep(interval)
	}
}

// hostnameMatch checks whether the hostname matches the given address.
// If we don't find an exact match, we look at the initial prefix.
func (s *Server) hostnameMatch(addr string) bool {
	if !strings.HasPrefix(addr, s.Hostname) {
		return false
	}
	if addr == s.Hostname {
		return true
	}
	// We know that s.Hostname is shorter than addr so we can safely check the next
	// char
	next := addr[len(s.Hostname)]
	return next == '.' || next == ':'
}

var (
	repoSyncStateCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repo_sync_state_counter",
		Help: "Incremented each time we check the state of repo",
	}, []string{"type"})
	repoSyncStatePercentComplete = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_repo_sync_state_percent_complete",
		Help: "Percent complete for the current sync run, from 0 to 100",
	})
	repoStateUpsertCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repo_sync_state_upsert_counter",
		Help: "Incremented each time we upsert repo state in the database",
	}, []string{"success"})
)

func (s *Server) syncRepoState(addrs []string, batchSize, perSecond int) error {
	// Sanity check our host exists in addrs before starting any work
	var found bool
	for _, a := range addrs {
		if s.hostnameMatch(a) {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("gitserver hostname, %q, not found in list", s.Hostname)
	}

	ctx := s.ctx
	store := database.GitserverRepos(s.DB)

	// The rate limit should be enforced across all instances
	perSecond = perSecond / len(addrs)
	if perSecond < 0 {
		perSecond = 1
	}
	limiter := rate.NewLimiter(rate.Limit(perSecond), perSecond)

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
			log15.Error("Waiting for rate limiter", "error", err)
			return
		}

		if err := store.Upsert(ctx, batch...); err != nil {
			repoStateUpsertCounter.WithLabelValues("false").Add(float64(len(batch)))
			log15.Error("Upserting GitserverRepos", "error", err)
			return
		}
		repoStateUpsertCounter.WithLabelValues("true").Add(float64(len(batch)))
	}

	totalRepos, err := database.Repos(s.DB).Count(ctx, database.ReposListOptions{})
	if err != nil {
		return errors.Wrap(err, "counting repos")
	}

	var count int
	err = store.IterateRepoGitserverStatus(ctx, func(repo types.RepoGitserverStatus) error {
		count++
		repoSyncStatePercentComplete.Set((float64(count) / float64(totalRepos)) * 100)

		repoSyncStateCounter.WithLabelValues("check").Inc()
		// Ensure we're only dealing with repos we are responsible for
		if addr := gitserver.AddrForRepo(repo.Name, addrs); !s.hostnameMatch(addr) {
			repoSyncStateCounter.WithLabelValues("other_shard").Inc()
			return nil
		}
		repoSyncStateCounter.WithLabelValues("this_shard").Inc()

		dir := s.dir(repo.Name)
		cloned := repoCloned(dir)
		_, cloning := s.locker.Status(dir)

		var shouldUpdate bool
		if repo.GitserverRepo == nil {
			repo.GitserverRepo = &types.GitserverRepo{
				RepoID: repo.ID,
			}
			shouldUpdate = true
		}
		if repo.ShardID != s.Hostname {
			repo.ShardID = s.Hostname
			shouldUpdate = true
		}
		cloneStatus := cloneStatus(cloned, cloning)
		if repo.CloneStatus != cloneStatus {
			repo.CloneStatus = cloneStatus
			shouldUpdate = true
		}

		if !shouldUpdate {
			return nil
		}

		batch = append(batch, repo.GitserverRepo)

		if len(batch) >= batchSize {
			writeBatch()
		}

		return nil
	})

	// Attempt final write
	writeBatch()

	return err
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

func (s *Server) getRemoteURL(ctx context.Context, name api.RepoName) (*url.URL, error) {
	if s.GetRemoteURLFunc == nil {
		return nil, errors.New("gitserver GetRemoteURLFunc is unset")
	}

	remoteURL, err := s.GetRemoteURLFunc(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "GetRemoteURLFunc")
	}

	return vcs.ParseURL(remoteURL)
}

// acquireCloneLimiter() acquires a cancellable context associated with the
// clone limiter.
func (s *Server) acquireCloneLimiter(ctx context.Context) (context.Context, context.CancelFunc, error) {
	cloneQueue.Inc()
	defer cloneQueue.Dec()
	return s.cloneLimiter.Acquire(ctx)
}

// queryCloneLimiter reports the capacity and length of the clone limiter's queue
func (s *Server) queryCloneLimiter() (cap, len int) {
	return s.cloneLimiter.GetLimit()
}

func (s *Server) acquireCloneableLimiter(ctx context.Context) (context.Context, context.CancelFunc, error) {
	lsRemoteQueue.Inc()
	defer lsRemoteQueue.Dec()
	return s.cloneableLimiter.Acquire(ctx)
}

// tempDir is a wrapper around ioutil.TempDir, but using the server's
// temporary directory filepath.Join(s.ReposDir, tempDirName).
//
// This directory is cleaned up by gitserver and will be ignored by repository
// listing operations.
func (s *Server) tempDir(prefix string) (name string, err error) {
	dir := filepath.Join(s.ReposDir, tempDirName)

	// Create tmpdir directory if doesn't exist yet.
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}

	return ioutil.TempDir(dir, prefix)
}

func (s *Server) ignorePath(path string) bool {
	// We ignore any path which starts with .tmp in ReposDir
	if filepath.Dir(path) != s.ReposDir {
		return false
	}
	return strings.HasPrefix(filepath.Base(path), tempDirName)
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

	var syncer VCSSyncer
	remoteURL, err := s.getRemoteURL(r.Context(), req.Repo)
	if err != nil {
		// We use this endpoint to verify if a repo exists without consuming
		// API rate limit, since many users visit private or bogus repos,
		// so we deduce the unauthenticated clone URL from the repo name.
		remoteURL, _ = vcs.ParseURL("https://" + string(req.Repo) + ".git")

		// At this point we are assuming it's a git repo
		syncer = &GitRepoSyncer{}
	} else {
		syncer, err = s.GetVCSSyncer(r.Context(), req.Repo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var resp protocol.IsRepoCloneableResponse
	if err := syncer.IsCloneable(r.Context(), remoteURL); err == nil {
		resp = protocol.IsRepoCloneableResponse{Cloneable: true}
	} else {
		resp = protocol.IsRepoCloneableResponse{
			Cloneable: false,
			Reason:    err.Error(),
		}
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleIsRepoCloned(w http.ResponseWriter, r *http.Request) {
	var req protocol.IsRepoClonedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if repoCloned(s.dir(req.Repo)) {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
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
	var resp protocol.RepoUpdateResponse
	req.Repo = protocol.NormalizeRepo(req.Repo)
	dir := s.dir(req.Repo)

	// despite the existence of a context on the request, we don't want to
	// cancel the git commands partway through if the request terminates.
	ctx, cancel1 := s.serverContext()
	defer cancel1()
	ctx, cancel2 := context.WithTimeout(ctx, longGitCommandTimeout)
	defer cancel2()
	resp.QueueCap, resp.QueueLen = s.queryCloneLimiter()
	if !repoCloned(dir) && !s.skipCloneForTests {
		// optimistically, we assume that our cloning attempt might
		// succeed.
		resp.CloneInProgress = true
		_, err := s.cloneRepo(ctx, req.Repo, &cloneOptions{Block: true})
		if err != nil {
			log15.Warn("error cloning repo", "repo", req.Repo, "err", err)
			resp.Error = err.Error()
		}
	} else {
		resp.Cloned = true
		var statusErr, updateErr error

		if debounce(req.Repo, req.Since) {
			updateErr = s.doRepoUpdate(ctx, req.Repo)
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
			log15.Error("failed to get status of repo", "repo", req.Repo, "error", statusErr)
			// report this error in-band, but still produce a valid response with the
			// other information.
			resp.Error = statusErr.Error()
		}
		// If an error occurred during update, report it but don't actually make
		// it into an http error; we want the client to get the information cleanly.
		// An update error "wins" over a status error.
		if updateErr != nil {
			resp.Error = updateErr.Error()
		}
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	var (
		q       = r.URL.Query()
		treeish = q.Get("treeish")
		repo    = q.Get("repo")
		format  = q.Get("format")
		paths   = q["path"]
	)

	if err := checkSpecArgSafety(treeish); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log15.Error("gitserver.archive.CheckSpecArgSafety", "error", err)
		return
	}

	if repo == "" || format == "" {
		w.WriteHeader(http.StatusBadRequest)
		log15.Error("gitserver.archive", "error", "empty repo or format")
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

	if format == "zip" {
		// Compression level of 0 (no compression) seems to perform the
		// best overall on fast network links, but this has not been tuned
		// thoroughly.
		req.Args = append(req.Args, "-0")
	}

	req.Args = append(req.Args, treeish, "--")
	req.Args = append(req.Args, paths...)

	s.exec(w, r, req)
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	var req protocol.ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.exec(w, r, &req)
}

func (s *Server) exec(w http.ResponseWriter, r *http.Request, req *protocol.ExecRequest) {
	// Flush writes more aggressively than standard net/http so that clients
	// with a context deadline see as much partial response body as possible.
	if fw := newFlushingResponseWriter(w); fw != nil {
		w = fw
		defer fw.Close()
	}

	ctx, cancel := context.WithTimeout(r.Context(), shortGitCommandTimeout(req.Args))
	defer cancel()

	start := time.Now()
	var cmdStart time.Time // set once we have ensured commit
	exitStatus := -10810   // sentinel value to indicate not set
	var stdoutN, stderrN int64
	var status string
	var execErr error
	var ensureRevisionStatus string

	req.Repo = protocol.NormalizeRepo(req.Repo)

	// Instrumentation
	{
		repo := repotrackutil.GetTrackedRepo(req.Repo)
		cmd := ""
		if len(req.Args) > 0 {
			cmd = req.Args[0]
		}
		args := strings.Join(req.Args, " ")

		var tr *trace.Trace
		tr, ctx = trace.New(ctx, "exec."+cmd, string(req.Repo))
		tr.LogFields(
			otlog.Object("args", args),
			otlog.String("ensure_revision", req.EnsureRevision),
		)

		execRunning.WithLabelValues(cmd, repo).Inc()
		defer func() {
			tr.LogFields(
				otlog.String("status", status),
				otlog.Int64("stdout", stdoutN),
				otlog.Int64("stderr", stderrN),
				otlog.String("ensure_revision_status", ensureRevisionStatus),
			)
			tr.SetError(execErr)
			tr.Finish()

			duration := time.Since(start)
			execRunning.WithLabelValues(cmd, repo).Dec()
			execDuration.WithLabelValues(cmd, repo, status).Observe(duration.Seconds())

			var cmdDuration time.Duration
			var fetchDuration time.Duration
			if !cmdStart.IsZero() {
				cmdDuration = time.Since(cmdStart)
				fetchDuration = cmdStart.Sub(start)
			}

			isSlow := cmdDuration > shortGitCommandSlow(req.Args)
			isSlowFetch := fetchDuration > 10*time.Second
			if honey.Enabled() || traceLogs || isSlow || isSlowFetch {
				ev := honey.Event("gitserver-exec")
				ev.SampleRate = honeySampleRate(cmd)
				ev.AddField("repo", req.Repo)
				ev.AddField("cmd", cmd)
				ev.AddField("args", args)
				ev.AddField("actor", r.Header.Get("X-Sourcegraph-Actor"))
				ev.AddField("ensure_revision", req.EnsureRevision)
				ev.AddField("ensure_revision_status", ensureRevisionStatus)
				ev.AddField("client", r.UserAgent())
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
					ev.AddField("fetch_duration_ms", fetchDuration.Milliseconds())
				}
				if span := opentracing.SpanFromContext(ctx); span != nil {
					spanURL := trace.SpanURL(span)
					// URLs starting with # don't have a trace. eg
					// "#tracer-not-enabled"
					if !strings.HasPrefix(spanURL, "#") {
						ev.AddField("trace", spanURL)
					}
				}

				if honey.Enabled() {
					_ = ev.Send()
				}
				if traceLogs {
					log15.Debug("TRACE gitserver exec", mapToLog15Ctx(ev.Fields())...)
				}
				if isSlow {
					log15.Warn("Long exec request", mapToLog15Ctx(ev.Fields())...)
				}
				if isSlowFetch {
					log15.Warn("Slow fetch/clone for exec request", mapToLog15Ctx(ev.Fields())...)
				}
			}
		}()
	}

	dir := s.dir(req.Repo)
	if !repoCloned(dir) {
		cloneProgress, cloneInProgress := s.locker.Status(dir)
		if cloneInProgress {
			status = "clone-in-progress"
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(&protocol.NotFoundPayload{
				CloneInProgress: true,
				CloneProgress:   cloneProgress,
			})
			return
		}

		cloneProgress, err := s.cloneRepo(ctx, req.Repo, nil)
		if err != nil {
			log15.Debug("error cloning repo", "repo", req.Repo, "err", err)
			status = "repo-not-found"
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(&protocol.NotFoundPayload{CloneInProgress: false})
			return
		}
		status = "clone-in-progress"
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(&protocol.NotFoundPayload{
			CloneInProgress: true,
			CloneProgress:   cloneProgress,
		})
		return
	}

	didUpdate := s.ensureRevision(ctx, req.Repo, req.EnsureRevision, dir)
	if didUpdate {
		ensureRevisionStatus = "fetched"
	} else {
		ensureRevisionStatus = "noop"
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-cache")

	w.Header().Set("Trailer", "X-Exec-Error")
	w.Header().Add("Trailer", "X-Exec-Exit-Status")
	w.Header().Add("Trailer", "X-Exec-Stderr")
	w.WriteHeader(http.StatusOK)

	// Special-case `git rev-parse HEAD` requests. These are invoked by search queries for every repo in scope.
	// For searches over large repo sets (> 1k), this leads to too many child process execs, which can lead
	// to a persistent failure mode where every exec takes > 10s, which is disastrous for gitserver performance.
	if len(req.Args) == 2 && req.Args[0] == "rev-parse" && req.Args[1] == "HEAD" {
		if resolved, err := quickRevParseHead(dir); err == nil && isAbsoluteRevision(resolved) {
			_, _ = w.Write([]byte(resolved))
			w.Header().Set("X-Exec-Error", "")
			w.Header().Set("X-Exec-Exit-Status", "0")
			w.Header().Set("X-Exec-Stderr", "")
			return
		}
	}
	// Special-case `git symbolic-ref HEAD` requests. These are invoked by resolvers determining the default branch of a repo.
	// For searches over large repo sets (> 1k), this leads to too many child process execs, which can lead
	// to a persistent failure mode where every exec takes > 10s, which is disastrous for gitserver performance.
	if len(req.Args) == 2 && req.Args[0] == "symbolic-ref" && req.Args[1] == "HEAD" {
		if resolved, err := quickSymbolicRefHead(dir); err == nil {
			_, _ = w.Write([]byte(resolved))
			w.Header().Set("X-Exec-Error", "")
			w.Header().Set("X-Exec-Exit-Status", "0")
			w.Header().Set("X-Exec-Stderr", "")
			return
		}
	}

	var stderrBuf bytes.Buffer
	stdoutW := &writeCounter{w: w}
	stderrW := &writeCounter{w: &limitWriter{W: &stderrBuf, N: 1024}}

	cmdStart = time.Now()
	cmd := exec.CommandContext(ctx, "git", req.Args...)
	dir.Set(cmd)
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	exitStatus, execErr = runCommand(ctx, cmd)

	status = strconv.Itoa(exitStatus)
	stdoutN = stdoutW.n
	stderrN = stderrW.n

	stderr := stderrBuf.String()
	checkMaybeCorruptRepo(req.Repo, dir, stderr)

	// write trailer
	w.Header().Set("X-Exec-Error", errorString(execErr))
	w.Header().Set("X-Exec-Exit-Status", status)
	w.Header().Set("X-Exec-Stderr", stderr)
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
	allowlist := []string{"protects", "groups", "users", "group"}
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

	// Make sure credentials are valid before heavier operation
	err := p4pingWithLogin(r.Context(), req.P4Port, req.P4User, req.P4Passwd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.p4exec(w, r, &req)
}

func (s *Server) p4exec(w http.ResponseWriter, r *http.Request, req *protocol.P4ExecRequest) {
	// Flush writes more aggressively than standard net/http so that clients
	// with a context deadline see as much partial response body as possible.
	if fw := newFlushingResponseWriter(w); fw != nil {
		w = fw
		defer fw.Close()
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()

	start := time.Now()
	var cmdStart time.Time // set once we have ensured commit
	exitStatus := -10810   // sentinel value to indicate not set
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

		var tr *trace.Trace
		tr, ctx = trace.New(ctx, "p4exec."+cmd, req.P4Port)
		tr.LogFields(
			otlog.Object("args", args),
		)

		execRunning.WithLabelValues(cmd, req.P4Port).Inc()
		defer func() {
			tr.LogFields(
				otlog.String("status", status),
				otlog.Int64("stdout", stdoutN),
				otlog.Int64("stderr", stderrN),
			)
			tr.SetError(execErr)
			tr.Finish()

			duration := time.Since(start)
			execRunning.WithLabelValues(cmd, req.P4Port).Dec()
			execDuration.WithLabelValues(cmd, req.P4Port, status).Observe(duration.Seconds())

			var cmdDuration time.Duration
			if !cmdStart.IsZero() {
				cmdDuration = time.Since(cmdStart)
			}

			isSlow := cmdDuration > 30*time.Second
			if honey.Enabled() || traceLogs || isSlow {
				ev := honey.Event("gitserver-p4exec")
				ev.SampleRate = honeySampleRate(cmd)
				ev.AddField("p4port", req.P4Port)
				ev.AddField("cmd", cmd)
				ev.AddField("args", args)
				ev.AddField("actor", r.Header.Get("X-Sourcegraph-Actor"))
				ev.AddField("client", r.UserAgent())
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
				if span := opentracing.SpanFromContext(ctx); span != nil {
					spanURL := trace.SpanURL(span)
					// URLs starting with # don't have a trace. eg
					// "#tracer-not-enabled"
					if !strings.HasPrefix(spanURL, "#") {
						ev.AddField("trace", spanURL)
					}
				}

				if honey.Enabled() {
					_ = ev.Send()
				}
				if traceLogs {
					log15.Debug("TRACE gitserver p4exec", mapToLog15Ctx(ev.Fields())...)
				}
				if isSlow {
					log15.Warn("Long p4exec request", mapToLog15Ctx(ev.Fields())...)
				}
			}
		}()
	}

	w.Header().Set("Trailer", "X-Exec-Error")
	w.Header().Add("Trailer", "X-Exec-Exit-Status")
	w.Header().Add("Trailer", "X-Exec-Stderr")
	w.WriteHeader(http.StatusOK)

	var stderrBuf bytes.Buffer
	stdoutW := &writeCounter{w: w}
	stderrW := &writeCounter{w: &limitWriter{W: &stderrBuf, N: 1024}}

	cmdStart = time.Now()
	cmd := exec.CommandContext(ctx, "p4", req.Args...)
	cmd.Env = append(os.Environ(),
		"P4PORT="+req.P4Port,
		"P4USER="+req.P4User,
	)
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	exitStatus, execErr = runCommand(ctx, cmd)

	status = strconv.Itoa(exitStatus)
	stdoutN = stdoutW.n
	stderrN = stderrW.n

	stderr := stderrBuf.String()

	// write trailer
	w.Header().Set("X-Exec-Error", errorString(execErr))
	w.Header().Set("X-Exec-Exit-Status", status)
	w.Header().Set("X-Exec-Stderr", stderr)
}

// setGitAttributes writes our global gitattributes to
// gitDir/info/attributes. This will override .gitattributes inside of
// repositories. It is used to unset attributes such as export-ignore.
func setGitAttributes(dir GitDir) error {
	infoDir := dir.Path("info")
	if err := os.Mkdir(infoDir, os.ModePerm); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "failed to set git attributes")
	}

	_, err := updateFileIfDifferent(
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
var testRepoCorrupter func(ctx context.Context, tmpDir GitDir)

// cloneOptions specify optional behaviour for the cloneRepo function.
type cloneOptions struct {
	// Block will wait for the clone to finish before returning. If the clone
	// fails, the error will be returned. The passed in context is
	// respected. When not blocking the clone is done with a server background
	// context.
	Block bool

	// Overwrite will overwrite the existing clone.
	Overwrite bool
}

// cloneRepo performs a clone operation for the given repository. It is
// non-blocking by default.
func (s *Server) cloneRepo(ctx context.Context, repo api.RepoName, opts *cloneOptions) (string, error) {
	if strings.ToLower(string(repo)) == "github.com/sourcegraphtest/alwayscloningtest" {
		return "This will never finish cloning", nil
	}

	dir := s.dir(repo)

	// PERF: Before doing the network request to check if isCloneable, lets
	// ensure we are not already cloning.
	if progress, cloneInProgress := s.locker.Status(dir); cloneInProgress {
		return progress, nil
	}

	syncer, err := s.GetVCSSyncer(ctx, repo)
	if err != nil {
		return "", errors.Wrap(err, "get VCS syncer")
	}

	remoteURL, err := s.getRemoteURL(ctx, repo)
	if err != nil {
		return "", err
	}

	redactor := newURLRedactor(remoteURL)

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
	if err := syncer.IsCloneable(ctx, remoteURL); err != nil {
		return "", fmt.Errorf("error cloning repo: repo %s not cloneable: %s", repo, redactor.redact(err.Error()))
	}

	// Mark this repo as currently being cloned. We have to check again if someone else isn't already
	// cloning since we released the lock. We released the lock since isCloneable is a potentially
	// slow operation.
	lock, ok := s.locker.TryAcquire(dir, "starting clone")
	if !ok {
		// Someone else beat us to it
		status, _ := s.locker.Status(dir)
		return status, nil
	}

	if s.skipCloneForTests {
		lock.Release()
		return "", nil
	}

	// We clone to a temporary location first to avoid having incomplete
	// clones in the repo tree. This also avoids leaving behind corrupt clones
	// if the clone is interrupted.
	doClone := func(ctx context.Context) error {
		defer lock.Release()

		ctx, cancel1, err := s.acquireCloneLimiter(ctx)
		if err != nil {
			return err
		}
		defer cancel1()

		ctx, cancel2 := context.WithTimeout(ctx, longGitCommandTimeout)
		defer cancel2()

		dstPath := string(dir)
		overwrite := opts != nil && opts.Overwrite
		if !overwrite {
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

		tmpPath, err := s.tempDir("clone-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpPath)
		tmpPath = filepath.Join(tmpPath, ".git")
		tmp := GitDir(tmpPath)

		// It may already be cloned
		if !repoCloned(dir) {
			s.setCloneStatusNonFatal(ctx, repo, types.CloneStatusCloning)
		}
		defer func() {
			// Use a different context to ensure we still update the DB even if we time out
			s.setCloneStatusNonFatal(context.Background(), repo, cloneStatus(repoCloned(dir), false))
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
		log15.Info("cloning repo", "repo", repo, "tmp", tmpPath, "dst", dstPath)

		pr, pw := io.Pipe()
		defer pw.Close()
		go readCloneProgress(redactor, lock, pr)

		if output, err := runWithRemoteOpts(ctx, cmd, pw); err != nil {
			return errors.Wrapf(err, "clone failed. Output: %s", string(output))
		}

		if testRepoCorrupter != nil {
			testRepoCorrupter(ctx, tmp)
		}

		removeBadRefs(ctx, tmp)

		if err := setHEAD(ctx, tmp, syncer, repo, remoteURL); err != nil {
			log15.Error("Failed to ensure HEAD exists", "repo", repo, "error", err)
			return errors.Wrap(err, "failed to ensure HEAD exists")
		}

		if err := setRepositoryType(tmp, syncer.Type()); err != nil {
			return errors.Wrap(err, `git config set "sourcegraph.type"`)
		}

		// Update the last-changed stamp.
		if err := setLastChanged(tmp); err != nil {
			return errors.Wrapf(err, "failed to update last changed time")
		}

		// Set gitattributes
		if err := setGitAttributes(tmp); err != nil {
			return err
		}

		if overwrite {
			// remove the current repo by putting it into our temporary directory
			err := renameAndSync(dstPath, filepath.Join(filepath.Dir(tmpPath), "old"))
			if err != nil && !os.IsNotExist(err) {
				return errors.Wrapf(err, "failed to remove old clone")
			}
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), os.ModePerm); err != nil {
			return err
		}
		if err := renameAndSync(tmpPath, dstPath); err != nil {
			return err
		}

		log15.Info("repo cloned", "repo", repo)
		repoClonedCounter.Inc()

		return nil
	}

	if opts != nil && opts.Block {
		// We are blocking, so use the passed in context.
		if err := doClone(ctx); err != nil {
			return "", errors.Wrapf(err, "failed to clone %s", repo)
		}
		return "", nil
	}

	go func() {
		// Create a new context because this is in a background goroutine.
		ctx, cancel := s.serverContext()
		defer cancel()
		if err := doClone(ctx); err != nil {
			log15.Error("failed to clone repo", "repo", repo, "error", err)
		}
	}()

	return "", nil
}

// readCloneProgress scans the reader and saves the most recent line of output
// as the lock status.
func readCloneProgress(redactor *urlRedactor, lock *RepositoryLock, pr io.Reader) {
	scan := bufio.NewScanner(pr)
	scan.Split(scanCRLF)
	for scan.Scan() {
		progress := scan.Text()

		// 🚨 SECURITY: The output could include the clone url with may contain a sensitive token.
		// Redact the full url and any found HTTP credentials to be safe.
		//
		// e.g.
		// $ git clone http://token@github.com/foo/bar
		// Cloning into 'nick'...
		// fatal: repository 'http://token@github.com/foo/bar/' not found
		redactedProgress := redactor.redact(progress)

		lock.SetStatus(redactedProgress)
	}
	if err := scan.Err(); err != nil {
		log15.Error("error reporting progress", "error", err)
	}
}

// urlRedactor redacts all sensitive strings from a message.
type urlRedactor struct {
	// sensitive are sensitive strings to be redacted.
	// The strings should not be empty.
	sensitive []string
}

// newURLRedactor returns a new urlRedactor that redacts
// credentials found in rawurl, and the rawurl itself.
func newURLRedactor(parsedURL *url.URL) *urlRedactor {
	var sensitive []string
	pw, _ := parsedURL.User.Password()
	u := parsedURL.User.Username()
	if pw != "" && u != "" {
		// Only block password if we have both as we can
		// assume that the username isn't sensitive in this case
		sensitive = append(sensitive, pw)
	} else {
		if pw != "" {
			sensitive = append(sensitive, pw)
		}
		if u != "" {
			sensitive = append(sensitive, u)
		}
	}
	sensitive = append(sensitive, parsedURL.String())
	return &urlRedactor{sensitive: sensitive}
}

// redact returns a redacted version of message.
// Sensitive strings are replaced with "<redacted>".
func (r *urlRedactor) redact(message string) string {
	for _, s := range r.sensitive {
		message = strings.Replace(message, s, "<redacted>", -1)
	}
	return message
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

// testGitRepoExists is a test fixture that overrides the return value for
// GitRepoSyncer.IsCloneable when it is set.
var testGitRepoExists func(ctx context.Context, remoteURL *url.URL) error

var (
	execRunning = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_gitserver_exec_running",
		Help: "number of gitserver.Command running concurrently.",
	}, []string{"cmd", "repo"})
	execDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_gitserver_exec_duration_seconds",
		Help:    "gitserver.Command latencies in seconds.",
		Buckets: trace.UserLatencyBuckets,
	}, []string{"cmd", "repo", "status"})
	cloneQueue = promauto.NewGauge(prometheus.GaugeOpts{
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
// to isntead be dynamic, since "rev-parse" is 12 times more likely than the
// next most common command.
func honeySampleRate(cmd string) uint {
	switch cmd {
	case "rev-parse":
		// 1 in 128. In practice 12 times more likely than our next most
		// common command.
		return 128
	default:
		// 1 in 16
		return 16
	}
}

var headBranchPattern = lazyregexp.New(`HEAD branch: (.+?)\n`)

func (s *Server) doRepoUpdate(ctx context.Context, repo api.RepoName) error {
	span, ctx := ot.StartSpanFromContext(ctx, "Server.doRepoUpdate")
	span.SetTag("repo", repo)
	defer span.Finish()

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

	// doRepoUpdate2 can block longer than our context deadline. done will
	// close when its done. We can return when either done is closed or our
	// deadline has passed.
	done := make(chan struct{})
	err := errors.New("another operation is already in progress")
	go func() {
		defer close(done)
		once.Do(func() {
			mu.Lock() // Prevent multiple updates in parallel. It works fine, but it wastes resources.
			defer mu.Unlock()

			s.repoUpdateLocksMu.Lock()
			l.once = new(sync.Once) // Make new requests wait for next update.
			s.repoUpdateLocksMu.Unlock()

			err = s.doRepoUpdate2(repo)
		})
	}()

	select {
	case <-done:
		return errors.Wrapf(err, "repo %s:", repo)
	case <-ctx.Done():
		span.LogFields(otlog.String("event", "context canceled"))
		return ctx.Err()
	}
}

var (
	badRefsOnce sync.Once
	badRefs     []string
)

// removeBadRefs removes bad refs and tags from the git repo at dir. This
// should be run after a clone or fetch. If your repository contains a ref or
// tag called HEAD (case insensitive), most commands will output a warning
// from git:
//
//  warning: refname 'HEAD' is ambiguous.
//
// Instead we just remove this ref.
func removeBadRefs(ctx context.Context, dir GitDir) {
	// older versions of git do not remove tags case insensitively, so we
	// generate every possible case of HEAD (2^4 = 16)
	badRefsOnce.Do(func() {
		for bits := uint8(0); bits < (1 << 4); bits++ {
			s := []byte("HEAD")
			for i, c := range s {
				// lowercase if the i'th bit of bits is 1
				if bits&(1<<i) != 0 {
					s[i] = c - 'A' + 'a'
				}
			}
			badRefs = append(badRefs, string(s))
		}
	})

	args := append([]string{"branch", "-D"}, badRefs...)
	cmd := exec.CommandContext(ctx, "git", args...)
	dir.Set(cmd)
	_ = cmd.Run()

	args = append([]string{"tag", "-d"}, badRefs...)
	cmd = exec.CommandContext(ctx, "git", args...)
	dir.Set(cmd)
	_ = cmd.Run()
}

// ensureHEAD verifies that there is a HEAD file within the repo, and that it
// is of non-zero length. If either condition is met, we configure a
// best-effort default.
func ensureHEAD(dir GitDir) {
	head, err := os.Stat(dir.Path("HEAD"))
	if os.IsNotExist(err) || head.Size() == 0 {
		ioutil.WriteFile(dir.Path("HEAD"), []byte("ref: refs/heads/master"), 0600)
	}
}

// setHEAD configures git repo defaults (such as what HEAD is) which are
// needed for git commands to work.
func setHEAD(ctx context.Context, dir GitDir, syncer VCSSyncer, repo api.RepoName, remoteURL *url.URL) error {
	// Verify that there is a HEAD file within the repo, and that it is of
	// non-zero length.
	ensureHEAD(dir)

	// Fallback to git's default branch name if git remote show fails.
	headBranch := "master"

	// try to fetch HEAD from origin
	cmd, err := syncer.RemoteShowCommand(ctx, remoteURL)
	if err != nil {
		return errors.Wrap(err, "get remote show command")
	}
	cmd.Dir = string(dir)
	output, err := runWithRemoteOpts(ctx, cmd, nil)
	if err != nil {
		log15.Error("Failed to fetch remote info", "repo", repo, "error", err, "output", string(output))
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
	cmd.Dir = string(dir)
	if err := cmd.Run(); err != nil {
		// branch does not exist, pick first branch
		cmd := exec.CommandContext(ctx, "git", "branch")
		cmd.Dir = string(dir)
		list, err := cmd.Output()
		if err != nil {
			log15.Error("Failed to list branches", "repo", repo, "error", err, "output", string(output))
			return errors.Wrap(err, "failed to list branches")
		}
		lines := strings.Split(string(list), "\n")
		branch := strings.TrimPrefix(strings.TrimPrefix(lines[0], "* "), "  ")
		if branch != "" {
			headBranch = branch
		}
	}

	// set HEAD
	cmd = exec.CommandContext(ctx, "git", "symbolic-ref", "HEAD", "refs/heads/"+headBranch)
	cmd.Dir = string(dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		log15.Error("Failed to set HEAD", "repo", repo, "error", err, "output", string(output))
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
func setLastChanged(dir GitDir) error {
	hashFile := dir.Path("sg_refhash")

	hash, err := computeRefHash(dir)
	if err != nil {
		return errors.Wrapf(err, "computeRefHash failed for %s", dir)
	}

	var stamp time.Time
	if _, err := os.Stat(hashFile); os.IsNotExist(err) {
		// This is the first time we are calculating the hash. Give a more
		// approriate timestamp for sg_refhash than the current time.
		stamp, err = computeLatestCommitTimestamp(dir)
		if err != nil {
			return errors.Wrapf(err, "computeLatestCommitTimestamp failed for %s", dir)
		}
	}

	_, err = updateFileIfDifferent(hashFile, hash)
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
// future, time.Now is returned.
func computeLatestCommitTimestamp(dir GitDir) (time.Time, error) {
	now := time.Now() // return current time if we don't find a more accurate time
	cmd := exec.Command("git", "rev-list", "--all", "--timestamp", "-n", "1")
	dir.Set(cmd)
	output, err := cmd.Output()
	// If we don't have a more specific stamp, we'll return the current time,
	// and possibly an error.
	if err != nil {
		return now, err
	}

	words := bytes.Split(output, []byte(" "))
	// An empty rev-list output, without an error, is okay.
	if len(words) < 2 {
		return now, nil
	}

	// We should have a timestamp and a commit hash; format is
	// 1521316105 ff03fac223b7f16627b301e03bf604e7808989be
	epoch, err := strconv.ParseInt(string(words[0]), 10, 64)
	if err != nil {
		return now, errors.Wrap(err, "invalid timestamp in rev-list output")
	}
	stamp := time.Unix(epoch, 0)
	if stamp.After(now) {
		return now, nil
	}
	return stamp, nil
}

// computeRefHash returns a hash of the refs for dir. The hash should only
// change if the set of refs and the commits they point to change.
func computeRefHash(dir GitDir) ([]byte, error) {
	// Do not use CommandContext since this is a fast operation we do not want
	// to interrupt.
	cmd := exec.Command("git", "show-ref")
	dir.Set(cmd)
	output, err := cmd.Output()
	if err != nil {
		// Ignore the failure for an empty repository: show-ref fails with
		// empty output and an exit code of 1
		if e, ok := err.(*exec.ExitError); !ok || len(output) != 0 || len(e.Stderr) != 0 || e.Sys().(syscall.WaitStatus).ExitStatus() != 1 {
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

func (s *Server) doRepoUpdate2(repo api.RepoName) error {
	// background context.
	ctx, cancel1 := s.serverContext()
	defer cancel1()

	ctx, cancel2, err := s.acquireCloneLimiter(ctx)
	if err != nil {
		return err
	}
	defer cancel2()

	repo = protocol.NormalizeRepo(repo)
	dir := s.dir(repo)

	remoteURL, err := s.getRemoteURL(ctx, repo)
	if err != nil {
		return errors.Wrap(err, "failed to determine Git remote URL")
	}

	syncer, err := s.GetVCSSyncer(ctx, repo)
	if err != nil {
		return errors.Wrap(err, "get VCS syncer")
	}

	cmd, configRemoteOpts, err := syncer.FetchCommand(ctx, remoteURL)
	if err != nil {
		return errors.Wrap(err, "get fetch command")
	}

	dir.Set(cmd)

	// drop temporary pack files after a fetch. this function won't
	// return until this fetch has completed or definitely-failed,
	// either way they can't still be in use. we don't care exactly
	// when the cleanup happens, just that it does.
	defer s.cleanTmpFiles(dir)

	if output, err := runWith(ctx, cmd, configRemoteOpts, nil); err != nil {
		log15.Error("Failed to update", "repo", repo, "error", err, "output", string(output))
		return errors.Wrap(err, "failed to update")
	}

	removeBadRefs(ctx, dir)

	if err := setHEAD(ctx, dir, syncer, repo, remoteURL); err != nil {
		log15.Error("Failed to ensure HEAD exists", "repo", repo, "error", err)
		return errors.Wrap(err, "failed to ensure HEAD exists")
	}

	if err := setRepositoryType(dir, syncer.Type()); err != nil {
		return errors.Wrap(err, `git config set "sourcegraph.type"`)
	}

	// Update the last-changed stamp.
	if err := setLastChanged(dir); err != nil {
		log15.Warn("Failed to update last changed time", "repo", repo, "error", err)
	}

	return nil
}

func (s *Server) ensureRevision(ctx context.Context, repo api.RepoName, rev string, repoDir GitDir) (didUpdate bool) {
	if rev == "" || rev == "HEAD" {
		return false
	}
	// rev-parse on an OID does not check if the commit actually exists, so it
	// is always works. So we append ^0 to force the check
	if isAbsoluteRevision(rev) {
		rev = rev + "^0"
	}
	cmd := exec.Command("git", "rev-parse", rev, "--")
	cmd.Dir = string(repoDir)
	if err := cmd.Run(); err == nil {
		return false
	}
	// Revision not found, update before returning.
	_ = s.doRepoUpdate(ctx, repo)
	return true
}

const headFileRefPrefix = "ref: "

// quickSymbolicRefHead best-effort mimics the execution of `git symbolic-ref HEAD`, but doesn't exec a child process.
// It just reads the .git/HEAD file from the bare git repository directory.
func quickSymbolicRefHead(dir GitDir) (string, error) {
	// See if HEAD contains a commit hash and fail if so.
	head, err := ioutil.ReadFile(dir.Path("HEAD"))
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
func quickRevParseHead(dir GitDir) (string, error) {
	// See if HEAD contains a commit hash and return it if so.
	head, err := ioutil.ReadFile(dir.Path("HEAD"))
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
		return "", fmt.Errorf("invalid ref format: %s", headRef)
	}
	headRefFile := dir.Path(filepath.FromSlash(string(headRef)))
	if refs, err := ioutil.ReadFile(headRefFile); err == nil {
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
