// Package internal implements the gitserver service.
package internal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/perforce"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/urlredactor"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/vcssyncer"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/limiter"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// traceLogs is controlled via the env SRC_GITSERVER_TRACE. If true we trace
// logs to stderr
var traceLogs bool

func init() {
	traceLogs, _ = strconv.ParseBool(env.Get("SRC_GITSERVER_TRACE", "false", "Toggles trace logging to stderr"))
}

type Backender func(common.GitDir, api.RepoName) git.GitBackend

type ServerOpts struct {
	// Logger should be used for all logging and logger creation.
	Logger log.Logger

	// FS is the file system to use for the gitserver. It allows to find repos by
	// name on disk and map a dir on disk back to a repo name.
	FS gitserverfs.FS

	// GetBackendFunc is a function which returns the git backend for a
	// repository.
	GetBackendFunc Backender

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
	GetVCSSyncer func(context.Context, api.RepoName) (vcssyncer.VCSSyncer, error)

	// Hostname is how we identify this instance of gitserver. Generally it is the
	// actual hostname but can also be overridden by the HOSTNAME environment variable.
	Hostname string

	// DB provides access to datastores.
	DB database.DB

	// Locker is used to lock repositories while fetching to prevent concurrent work.
	Locker RepositoryLocker

	// RPSLimiter limits the remote code host git operations done per second
	// per gitserver instance
	RPSLimiter *ratelimit.InstrumentedLimiter

	// RecordingCommandFactory is a factory that creates recordable commands by wrapping os/exec.Commands.
	// The factory creates recordable commands with a set predicate, which is used to determine whether a
	// particular command should be recorded or not.
	RecordingCommandFactory *wrexec.RecordingCommandFactory

	// Perforce is a plugin-like service attached to Server for all things Perforce.
	Perforce *perforce.Service
}

func NewServer(opt *ServerOpts) *Server {
	ctx, cancel := context.WithCancelCause(context.Background())

	// GitMaxConcurrentClones controls the maximum number of clones that
	// can happen at once on a single gitserver.
	// Used to prevent throttle limits from a code host. Defaults to 5.
	//
	// The new repo-updater scheduler enforces the rate limit across all gitserver,
	// so ideally this logic could be removed here; however, ensureRevision can also
	// cause an update to happen and it is called on every exec command.
	// Max concurrent clones also means repo updates.
	maxConcurrentClones := conf.GitMaxConcurrentClones()
	cloneLimiter := limiter.NewMutable(maxConcurrentClones)

	conf.Watch(func() {
		limit := conf.GitMaxConcurrentClones()
		cloneLimiter.SetLimit(limit)
	})

	return &Server{
		logger:                  opt.Logger,
		getBackendFunc:          opt.GetBackendFunc,
		getRemoteURLFunc:        opt.GetRemoteURLFunc,
		getVCSSyncer:            opt.GetVCSSyncer,
		hostname:                opt.Hostname,
		db:                      opt.DB,
		locker:                  opt.Locker,
		rpsLimiter:              opt.RPSLimiter,
		recordingCommandFactory: opt.RecordingCommandFactory,
		perforce:                opt.Perforce,
		fs:                      opt.FS,

		repoUpdateLocks: make(map[api.RepoName]*locks),
		cloneLimiter:    cloneLimiter,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Server is a gitserver server.
type Server struct {
	// logger should be used for all logging and logger creation.
	logger log.Logger

	// fs is the file system to use for the gitserver. It allows to find repos by
	// name on disk and map a dir on disk back to a repo name.
	fs gitserverfs.FS

	// getBackendFunc is a function which returns the git backend for a
	// repository.
	getBackendFunc Backender

	// getRemoteURLFunc is a function which returns the remote URL for a
	// repository. This is used when cloning or fetching a repository. In
	// production this will speak to the database to look up the clone URL. In
	// tests this is usually set to clone a local repository or intentionally
	// error.
	getRemoteURLFunc func(context.Context, api.RepoName) (string, error)

	// getVCSSyncer is a function which returns the VCS syncer for a repository.
	// This is used when cloning or fetching a repository. In production this will
	// speak to the database to determine the code host type. In tests this is
	// usually set to return a GitRepoSyncer.
	getVCSSyncer func(context.Context, api.RepoName) (vcssyncer.VCSSyncer, error)

	// hostname is how we identify this instance of gitserver. Generally it is the
	// actual hostname but can also be overridden by the HOSTNAME environment variable.
	hostname string

	// db provides access to datastores.
	db database.DB

	// locker is used to lock repositories while fetching to prevent concurrent work.
	locker RepositoryLocker

	// ctx is the context we use for all background jobs. It is done when the
	// server is stopped. Do not directly call this, rather call
	// Server.context()
	ctx      context.Context
	cancel   context.CancelCauseFunc // used to shutdown background jobs
	cancelMu sync.Mutex              // protects canceled
	canceled bool
	wg       sync.WaitGroup // tracks running background jobs

	// cloneLimiter limits the number of concurrent
	// clones. Use s.acquireCloneLimiter() and instead of using it directly.
	cloneLimiter *limiter.MutableLimiter

	// rpsLimiter limits the remote code host git operations done per second
	// per gitserver instance
	rpsLimiter *ratelimit.InstrumentedLimiter

	repoUpdateLocksMu sync.Mutex // protects the map below and also updates to locks.once
	repoUpdateLocks   map[api.RepoName]*locks

	// recordingCommandFactory is a factory that creates recordable commands by wrapping os/exec.Commands.
	// The factory creates recordable commands with a set predicate, which is used to determine whether a
	// particular command should be recorded or not.
	recordingCommandFactory *wrexec.RecordingCommandFactory

	// perforce is a plugin-like service attached to Server for all things perforce.
	perforce *perforce.Service
}

type locks struct {
	once *sync.Once  // consolidates multiple waiting updates
	mu   *sync.Mutex // prevents updates running in parallel
}

// Stop cancels the running background jobs and returns when done.
func (s *Server) Stop() {
	// idempotent so we can just always set and cancel
	// Provide a little bit of context of where this context cancellation
	// is coming from.
	s.cancel(errors.New("gitserver is shutting down"))
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
	remoteURL, err := s.getRemoteURLFunc(ctx, name)
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

func (s *Server) IsRepoCloneable(ctx context.Context, repo api.RepoName) (protocol.IsRepoCloneableResponse, error) {
	// We use an internal actor here as the repo may be private. It is safe since all
	// we return is a bool indicating whether the repo is cloneable or not. Perhaps
	// the only things that could leak here is whether a private repo exists although
	// the endpoint is only available internally so it's low risk.
	ctx = actor.WithInternalActor(ctx)
	syncer, err := s.getVCSSyncer(ctx, repo)
	if err != nil {
		return protocol.IsRepoCloneableResponse{}, errors.Wrap(err, "GetVCSSyncer")
	}

	cloned, err := s.fs.RepoCloned(repo)
	if err != nil {
		return protocol.IsRepoCloneableResponse{}, errors.Wrap(err, "determine if repo is cloned")
	}

	resp := protocol.IsRepoCloneableResponse{
		Cloned: cloned,
	}
	err = syncer.IsCloneable(ctx, repo)
	if err != nil {
		resp.Reason = err.Error()
	}
	resp.Cloneable = err == nil

	return resp, nil
}

// RepoUpdate triggers an update for the given repo in the background, if it hasn't
// been updated recently.
// If the repo is not cloned, a blocking clone will be triggered instead.
// This function will not return until the update is complete.
// Canceling the context will not cancel the update, but it will let the caller
// escape the function early.
func (s *Server) RepoUpdate(ctx context.Context, req *protocol.RepoUpdateRequest) protocol.RepoUpdateResponse {
	logger := s.logger.Scoped("handleRepoUpdate")

	var resp protocol.RepoUpdateResponse
	dir := s.fs.RepoDir(req.Repo)

	cloned, err := s.fs.RepoCloned(req.Repo)
	if err != nil {
		resp.Error = errors.Wrap(err, "determining cloned status").Error()
		return resp
	}

	if !cloned {
		cloneErr := s.cloneRepo(ctx, req.Repo)
		if cloneErr != nil {
			if !errors.Is(cloneErr, ErrCloneInProgress) {
				logger.Warn("error cloning repo", log.String("repo", string(req.Repo)), log.Error(cloneErr))
			}
			resp.Error = cloneErr.Error()
		} else {
			// attempts to acquire these values are not contingent on the success of
			// the update.
			var statusErr error
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
				// We don't forward a statusErr to the caller.
			}
		}
		return resp
	}

	updateErr := s.doRepoUpdate(ctx, req.Repo, "")

	// attempts to acquire these values are not contingent on the success of
	// the update.
	var statusErr error
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
		s.perforce.EnqueueChangelistMappingJob(perforce.NewChangelistMappingJob(req.Repo, dir))
	}

	return resp
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

func (s *Server) LogIfCorrupt(ctx context.Context, repo api.RepoName, err error) {
	var corruptErr common.ErrRepoCorrupted
	if errors.As(err, &corruptErr) {
		repoCorruptedCounter.Inc()
		if err := s.db.GitserverRepos().LogCorruption(ctx, repo, corruptErr.Reason, s.hostname); err != nil {
			s.logger.Warn("failed to log repo corruption", log.String("repo", string(repo)), log.Error(err))
		}
	}
}

var ErrCloneInProgress = errors.New("clone in progress")

// cloneRepo performs a clone operation for the given repository.
// Canceling the context will not cancel the clone if blocking, but it will let
// the caller escape the function early.
// Canceling the context may result in no clone being scheduled.
func (s *Server) cloneRepo(ctx context.Context, repo api.RepoName) (err error) {
	if isAlwaysCloningTest(repo) {
		return nil
	}

	// PERF: Before doing the network request to check if isCloneable, lets
	// ensure we are not already cloning.
	if _, cloneInProgress := s.locker.Status(repo); cloneInProgress {
		return ErrCloneInProgress
	}

	// We may be attempting to clone a private repo so we need an internal actor.
	ctx = actor.WithInternalActor(ctx)

	syncer, remoteURL, err := func() (_ vcssyncer.VCSSyncer, _ *vcs.URL, err error) {
		syncer, err := s.getVCSSyncer(ctx, repo)
		if err != nil {
			return nil, errors.Wrap(err, "get VCS syncer")
		}

		if err = s.rpsLimiter.Wait(ctx); err != nil {
			return nil, err
		}

		remoteURL, err := s.getRemoteURL(ctx, repo)
		if err != nil {
			return nil, err
		}
		if err := syncer.IsCloneable(ctx, repo); err != nil {
			redactedErr := urlredactor.New(remoteURL).Redact(err.Error())
			return nil, errors.Errorf("error cloning repo: repo %s not cloneable: %s", repo, redactedErr)
		}

		return syncer, nil
	}()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	// Mark this repo as currently being cloned. We have to check again if someone else isn't already
	// cloning since we released the lock. We released the lock since isCloneable is a potentially
	// slow operation.
	lock, ok := s.locker.TryAcquire(repo, "starting clone")
	if !ok {
		// Someone else beat us to it
		return ErrCloneInProgress
	}

	dir := s.fs.RepoDir(repo)

	// Use serverCtx here since we want to let the clone proceed, even if
	// the requestor has cancelled the outer context.
	serverCtx, cancel := s.serverContext()
	defer cancel()

		// Use caller context, if the caller is not interested anymore before we
		// start cloning, we can skip the clone altogether.
		_, cancel, err := s.acquireCloneLimiter(ctx)
		if err != nil {
			lock.Release()
			return "", err
		}
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)

			err = errors.Wrapf(s.doClone(serverCtx, repo, dir, syncer, lock, opts), "failed to clone %s", repo)
		}()

		select {
		case <-done:
			return "", err
		case <-ctx.Done():
			// If the caller is not interested anymore, we finish the clone anyways,
			// but let the caller live on.
			return "", ctx.Err()
		}
	}
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		err = errors.Wrapf(s.doClone(serverCtx, repo, dir, syncer, lock), "failed to clone %s", repo)

		s.setLastErrorNonFatal(serverCtx, repo, err)
	}()

	select {
	case <-done:
		return err
	case <-ctx.Done():
		// If the caller is not interested anymore, we finish the clone anyways,
		// but let the caller live on.
		return ctx.Err()
	}
}

func (s *Server) doClone(
	ctx context.Context,
	repo api.RepoName,
	dir common.GitDir,
	syncer vcssyncer.VCSSyncer,
	lock RepositoryLock,
) (err error) {
	logger := s.logger.Scoped("doClone").With(log.String("repo", string(repo)))

	defer lock.Release()
	defer func() {
		if err != nil {
			repoCloneFailedCounter.Inc()
		}
	}()
	if err := s.rpsLimiter.Wait(ctx); err != nil {
		return err
	}

	dstPath := string(dir)

	// We clone to a temporary directory first, so avoid wasting resources
	// if the directory already exists.
	if _, err := os.Stat(dstPath); err == nil {
		return &os.PathError{
			Op:   "cloneRepo",
			Path: dstPath,
			Err:  os.ErrExist,
		}
	}

	// We clone to a temporary location first to avoid having incomplete
	// clones in the repo tree. This also avoids leaving behind corrupt clones
	// if the clone is interrupted.
	tmpDir, err := s.fs.TempDir("clone-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, ".git")

	logger.Info("cloning repo", log.String("tmp", tmpDir), log.String("dst", dstPath))

	progressReader, progressWriter := io.Pipe()
	// We also capture the entire output in memory for the call to SetLastOutput
	// further down.
	// TODO: This might require a lot of memory depending on the amount of logs
	// produced, the ideal solution would be that readCloneProgress stores it in
	// chunks.
	output := &linebasedBufferedWriter{}
	eg := readCloneProgress(logger, lock, io.TeeReader(progressReader, output), repo)

	cloneTimeout := conf.GitLongCommandTimeout()
	cloneCtx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()

	cloneErr := syncer.Clone(cloneCtx, repo, dir, tmpPath, progressWriter)
	progressWriter.Close()

	if err := eg.Wait(); err != nil {
		s.logger.Error("reading clone progress", log.Error(err))
	}

	// best-effort update the output of the clone
	if err := s.db.GitserverRepos().SetLastOutput(context.Background(), repo, output.String()); err != nil {
		s.logger.Error("Setting last output in DB", log.Error(err))
	}

	if cloneErr != nil {
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			return errors.Newf("failed to clone repo within deadline of %s", cloneTimeout)
		}
		// TODO: Should we really return the entire output here in an error?
		// It could be a super big error string.
		return errors.Wrapf(cloneErr, "clone failed. Output: %s", output.String())
	}

	if err := postRepoFetchActions(ctx, logger, s.fs, s.db, s.getBackendFunc(common.GitDir(tmpPath), repo), s.hostname, repo, common.GitDir(tmpPath), syncer); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), os.ModePerm); err != nil {
		return err
	}
	if err := fileutil.RenameAndSync(tmpPath, dstPath); err != nil {
		return err
	}

	logger.Info("repo cloned")
	repoClonedCounter.Inc()

	s.perforce.EnqueueChangelistMappingJob(perforce.NewChangelistMappingJob(repo, dir))

	return nil
}

// linebasedBufferedWriter is an io.Writer that writes to a buffer.
// '\r' resets the write offset to the index after last '\n' in the buffer,
// or the beginning of the buffer if a '\n' has not been written yet.
//
// This exists to remove intermediate progress reports from "git clone
// --progress".
type linebasedBufferedWriter struct {
	// writeOffset is the offset in buf where the next write should begin.
	writeOffset int

	// afterLastNewline is the index after the last '\n' in buf
	// or 0 if there is no '\n' in buf.
	afterLastNewline int

	buf []byte
}

func (w *linebasedBufferedWriter) Write(p []byte) (n int, err error) {
	l := len(p)
	for {
		if len(p) == 0 {
			// If p ends in a '\r' we still want to include that in the buffer until it is overwritten.
			break
		}
		idx := bytes.IndexAny(p, "\r\n")
		if idx == -1 {
			w.buf = append(w.buf[:w.writeOffset], p...)
			w.writeOffset = len(w.buf)
			break
		}
		w.buf = append(w.buf[:w.writeOffset], p[:idx+1]...)
		switch p[idx] {
		case '\n':
			w.writeOffset = len(w.buf)
			w.afterLastNewline = len(w.buf)
			p = p[idx+1:]
		case '\r':
			// Record that our next write should overwrite the data after the most recent newline.
			// Don't slice it off immediately here, because we want to be able to return that output
			// until it is overwritten.
			w.writeOffset = w.afterLastNewline
			p = p[idx+1:]
		default:
			panic(fmt.Sprintf("unexpected char %q", p[idx]))
		}
	}
	return l, nil
}

// String returns the contents of the buffer as a string.
func (w *linebasedBufferedWriter) String() string {
	return string(w.buf)
}

// Bytes returns the contents of the buffer.
func (w *linebasedBufferedWriter) Bytes() []byte {
	return w.buf
}

func postRepoFetchActions(
	ctx context.Context,
	logger log.Logger,
	fs gitserverfs.FS,
	db database.DB,
	backend git.GitBackend,
	shardID string,
	repo api.RepoName,
	dir common.GitDir,
	syncer vcssyncer.VCSSyncer,
) (errs error) {
	// Note: We use a multi error in this function to try to make as many of the
	// post repo fetch actions succeed.

	if err := git.RemoveBadRefs(ctx, dir); err != nil {
		errs = errors.Append(errs, errors.Wrapf(err, "failed to remove bad refs for repo %q", repo))
	}

	if err := git.SetRepositoryType(ctx, backend.Config(), syncer.Type()); err != nil {
		errs = errors.Append(errs, errors.Wrapf(err, "failed to set repository type for repo %q", repo))
	}

	if err := git.SetGitAttributes(dir); err != nil {
		errs = errors.Append(errs, errors.Wrap(err, "setting git attributes"))
	}

	if err := gitSetAutoGC(ctx, backend.Config()); err != nil {
		errs = errors.Append(errs, errors.Wrap(err, "setting git gc mode"))
	}

	// Update the last-changed stamp on disk.
	if err := setLastChanged(logger, dir); err != nil {
		errs = errors.Append(errs, errors.Wrap(err, "failed to update last changed time"))
	}

	// Successfully updated, best-effort updating of db fetch state based on
	// disk state.
	if err := setLastFetched(ctx, db, shardID, dir, repo); err != nil {
		errs = errors.Append(errs, errors.Wrap(err, "failed setting last fetch in DB"))
	}

	// Successfully updated, best-effort calculation of the repo size.
	repoSizeBytes, err := fs.DirSize(dir.Path())
	if err != nil {
		errs = errors.Append(errs, errors.Wrap(err, "failed to calculate repo size"))
	} else if err := db.GitserverRepos().SetRepoSize(ctx, repo, repoSizeBytes, shardID); err != nil {
		errs = errors.Append(errs, errors.Wrap(err, "failed to set repo size"))
	}

	return errs
}

// readCloneProgress scans the reader and saves the most recent line of output
// as the lock status, and optionally writes to a log file if siteConfig.cloneProgressLog
// is enabled.
func readCloneProgress(logger log.Logger, lock RepositoryLock, pr io.Reader, repo api.RepoName) *errgroup.Group {
	var logFile *os.File

	if conf.Get().CloneProgressLog {
		var err error
		logFile, err = os.CreateTemp("", "")
		if err != nil {
			logger.Warn("failed to create temporary clone log file", log.Error(err), log.String("repo", string(repo)))
		} else {
			logger.Info("logging clone output", log.String("file", logFile.Name()), log.String("repo", string(repo)))
			defer logFile.Close()
		}
	}

	scan := bufio.NewScanner(pr)
	scan.Split(scanCRLF)

	var eg errgroup.Group
	eg.Go(func() error {
		for scan.Scan() {
			progress := scan.Text()
			lock.SetStatus(progress)

			if logFile != nil {
				// Failing to write here is non-fatal and we don't want to spam our logs if there
				// are issues
				_, _ = fmt.Fprintln(logFile, progress)
			}
		}
		if err := scan.Err(); err != nil {
			return err
		}

		return nil
	})

	return &eg
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
	repoClonedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repo_cloned",
		Help: "number of successful git clones run",
	})
	repoCloneFailedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repo_cloned_failed",
		Help: "number of failed git clones",
	})
	repoCorruptedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repo_corrupted",
		Help: "number of corruption events",
	})
)

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

			// Note: We do not pass a ctx down here, because we don't want the update
			// to stall when the request is cancelled, and subsequently fail the
			// background update for potential other callers that wait for the
			// same sync group.
			err = s.doBackgroundRepoUpdate(repo, revspec)
			// Use a background context for reporting, the caller might have given
			// up at this point, but we still want to make the updates.
			serverCtx, cancel := s.serverContext()
			defer cancel()
			if err != nil {
				// We don't want to spam our logs when the rate limiter has been set to block all
				// updates
				if !errors.Is(err, ratelimit.ErrBlockAll) {
					s.logger.Error("performing background repo update", log.Error(err), log.String("repo", string(repo)))
				}

				// The repo update might have failed due to the repo being corrupt
				s.LogIfCorrupt(serverCtx, repo, err)
			}
		})
	}()

	select {
	case <-done:
		return errors.Wrapf(err, "repo %s", repo)
	// In case the caller is no longer interested in the result, let them live on.
	case <-ctx.Done():
		return ctx.Err()
	}
}

var doBackgroundRepoUpdateMock func(api.RepoName) error

func (s *Server) doBackgroundRepoUpdate(repo api.RepoName, revspec string) error {
	logger := s.logger.Scoped("backgroundRepoUpdate").With(log.String("repo", string(repo)))

	if doBackgroundRepoUpdateMock != nil {
		return doBackgroundRepoUpdateMock(repo)
	}

	// We use a server context here, because we don't want the caller to abort a fetch
	// mid-way just because they're not interested in the result anymore. Gitserver
	// is always interested in finishing fetches where possible.
	serverCtx, cancel := s.serverContext()
	defer cancel()

	// ensure the background update doesn't hang forever
	fetchTimeout := conf.GitLongCommandTimeout()
	ctx, cancelTimeout := context.WithTimeout(serverCtx, fetchTimeout)
	defer cancelTimeout()

	// This background process should use our internal actor
	ctx = actor.WithInternalActor(ctx)

	err := func(ctx context.Context) error {
		ctx, cancelLimiter, err := s.acquireCloneLimiter(ctx)
		if err != nil {
			return err
		}
		defer cancelLimiter()

		if err = s.rpsLimiter.Wait(ctx); err != nil {
			return err
		}

		dir := s.fs.RepoDir(repo)

		remoteURL, err := s.getRemoteURL(ctx, repo)
		if err != nil {
			return errors.Wrap(err, "failed to determine Git remote URL")
		}

		syncer, err := s.getVCSSyncer(ctx, repo)
		if err != nil {
			return errors.Wrap(err, "get VCS syncer")
		}

		// drop temporary pack files after a fetch. this function won't
		// return until this fetch has completed or definitely-failed,
		// either way they can't still be in use. we don't care exactly
		// when the cleanup happens, just that it does.
		// TODO: Should be done in janitor.
		defer git.CleanTmpPackFiles(s.logger, dir)

		output, err := syncer.Fetch(ctx, repo, dir, revspec)
		// TODO: Move the redaction also into the VCSSyncer layer here, to be in line
		// with what clone does.
		redactedOutput := urlredactor.New(remoteURL).Redact(string(output))
		// best-effort update the output of the fetch
		if err := s.db.GitserverRepos().SetLastOutput(serverCtx, repo, redactedOutput); err != nil {
			s.logger.Warn("Setting last output in DB", log.Error(err))
		}

		if err != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
			if output != nil {
				return errors.Wrapf(err, "failed to fetch repo %q with output %q", repo, redactedOutput)
			} else {
				return errors.Wrapf(err, "failed to fetch repo %q", repo)
			}
		}

		return postRepoFetchActions(ctx, logger, s.fs, s.db, s.getBackendFunc(dir, repo), s.hostname, repo, dir, syncer)
	}(ctx)

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Newf("failed to update repo within deadline of %s", fetchTimeout)
	}

	return err
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

	hash, err := git.ComputeRefHash(dir)
	if err != nil {
		return errors.Wrapf(err, "computeRefHash failed for %s", dir)
	}

	var stamp time.Time
	if _, err := os.Stat(hashFile); os.IsNotExist(err) {
		// This is the first time we are calculating the hash. Give a more
		// approriate timestamp for sg_refhash than the current time.
		stamp = git.LatestCommitTimestamp(logger, dir)
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

func (s *Server) SearchWithObservability(ctx context.Context, tr trace.Trace, args *protocol.SearchRequest, onMatch func(*protocol.CommitMatch) error) (limitHit bool, err error) {
	return searchWithObservability(ctx, s.logger, s.fs.RepoDir(args.Repo), tr, args, onMatch)
}
