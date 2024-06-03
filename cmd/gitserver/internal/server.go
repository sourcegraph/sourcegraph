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
	"github.com/sourcegraph/sourcegraph/internal/types"
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

type ServerOpts struct {
	// Logger should be used for all logging and logger creation.
	Logger log.Logger

	// FS is the file system to use for the gitserver. It allows to find repos by
	// name on disk and map a dir on disk back to a repo name.
	FS gitserverfs.FS

	// GitBackendSource is a function which returns the git backend for a
	// repository.
	GitBackendSource git.GitBackendSource

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
		gitBackendSource:        opt.GitBackendSource,
		getRemoteURLFunc:        opt.GetRemoteURLFunc,
		getVCSSyncer:            opt.GetVCSSyncer,
		hostname:                opt.Hostname,
		db:                      opt.DB,
		locker:                  opt.Locker,
		rpsLimiter:              opt.RPSLimiter,
		recordingCommandFactory: opt.RecordingCommandFactory,
		fs:                      opt.FS,

		cloneLimiter: cloneLimiter,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Server is a gitserver server.
type Server struct {
	// logger should be used for all logging and logger creation.
	logger log.Logger

	// fs is the file system to use for the gitserver. It allows to find repos by
	// name on disk and map a dir on disk back to a repo name.
	fs gitserverfs.FS

	// gitBackendSource is a function which returns the git backend for a
	// repository.
	gitBackendSource git.GitBackendSource

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
	// clones.
	cloneLimiter *limiter.MutableLimiter

	// rpsLimiter limits the remote code host git operations done per second
	// per gitserver instance
	rpsLimiter *ratelimit.InstrumentedLimiter

	// recordingCommandFactory is a factory that creates recordable commands by wrapping os/exec.Commands.
	// The factory creates recordable commands with a set predicate, which is used to determine whether a
	// particular command should be recorded or not.
	recordingCommandFactory *wrexec.RecordingCommandFactory
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

// FetchRepository triggers an update for the given repo.
// If the repo is not cloned, a blocking clone will be triggered instead.
// This function will not return until the update is complete.
// Canceling the context will not cancel the update, but it will let the caller
// escape the function early.
func (s *Server) FetchRepository(ctx context.Context, repoName api.RepoName) (lastFetched, lastChanged time.Time, err error) {
	err = s.repoUpdateOrClone(ctx, repoName)
	if err != nil {
		return lastFetched, lastChanged, err
	}

	dir := s.fs.RepoDir(repoName)

	lastFetched, err = repoLastFetched(dir)
	if err != nil {
		return lastFetched, lastChanged, errors.Wrap(err, "failed to get last fetched time")
	}

	lastChanged, err = repoLastChanged(dir)
	if err != nil {
		return lastFetched, lastChanged, errors.Wrap(err, "failed to get last changed time")
	}

	return lastFetched, lastChanged, nil
}

func (s *Server) repoUpdateOrClone(ctx context.Context, repoName api.RepoName) error {
	logger := s.logger.Scoped("repoUpdateOrClone")

	lock, ok := s.locker.TryAcquire(repoName, "starting fetch")
	if !ok {
		return ErrFetchInProgress
	}

	// Use caller context, if the caller is not interested anymore before we
	// start cloning, we can skip the clone altogether.
	pendingClones.Inc()
	_, cancelCloneLimiter, err := s.cloneLimiter.Acquire(ctx)
	pendingClones.Dec()
	if err != nil {
		lock.Release()
		return err
	}

	errCh := make(chan error, 1)

	// We spawn a background job to do the update. This is to keep going when the
	// caller has already cancelled the context, or when the connection was interrupted.
	go func() {
		errCh <- func() (err error) {
			defer lock.Release()
			defer cancelCloneLimiter()

			// We use server context here to ensure that we can cancel the background
			// job when the server is shutting down, and to make sure that the job
			// keeps running beyond the lifetime of the gRPC request that triggers this.
			ctx, cancel := s.serverContext()
			defer cancel()
			// We may be attempting to clone a private repo so we need an internal actor.
			ctx = actor.WithInternalActor(ctx)

			defer func() {
				var errString string
				if err != nil {
					errString = err.Error()
				}

				if err := s.db.GitserverRepos().SetLastError(ctx, repoName, errString, s.hostname); err != nil {
					s.logger.Error("Setting last error in DB", log.Error(err))
				}
			}()

			cloned, err := s.fs.RepoCloned(repoName)
			if err != nil {
				return errors.Wrap(err, "determining cloned status")
			}

			if !cloned {
				if err := s.cloneRepo(ctx, repoName, lock); err != nil {
					repoCloneFailedCounter.Inc()
					logger.Error("error cloning repo", log.String("repo", string(repoName)), log.Error(err))
					return errors.Wrapf(err, "failed to clone %s", repoName)
				}
				repoClonedCounter.Inc()
				logger.Info("cloned repo", log.String("repo", string(repoName)))
			} else {
				if err := s.doRepoUpdate(ctx, repoName, lock); err != nil {
					// The repo update might have failed due to the repo being corrupt
					s.LogIfCorrupt(ctx, repoName, err)

					logger.Error("error updating repo", log.Error(err), log.String("repo", string(repoName)))
					return errors.Wrapf(err, "failed to fetch %s", repoName)
				}
			}

			return nil
		}()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
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

var ErrFetchInProgress = errors.New("fetch for this repo already in progress")

// cloneRepo performs a clone operation for the given repository.
func (s *Server) cloneRepo(ctx context.Context, repo api.RepoName, lock RepositoryLock) (err error) {
	if isAlwaysCloningTest(repo) {
		return nil
	}

	logger := s.logger.Scoped("cloneRepo").With(log.String("repo", string(repo)))

	syncer, err := func() (_ vcssyncer.VCSSyncer, err error) {
		syncer, err := s.getVCSSyncer(ctx, repo)
		if err != nil {
			return nil, errors.Wrap(err, "get VCS syncer")
		}

		if err = s.rpsLimiter.Wait(ctx); err != nil {
			return nil, err
		}

		if err := syncer.IsCloneable(ctx, repo); err != nil {
			return nil, errors.Wrapf(err, "error cloning repo: repo %s not cloneable", repo)
		}

		return syncer, nil
	}()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	dir := s.fs.RepoDir(repo)

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

	if err := s.db.GitserverRepos().SetCloneStatus(ctx, repo, types.CloneStatusCloning, s.hostname); err != nil {
		s.logger.Error("Setting clone status in DB", log.Error(err))
	}
	defer func() {
		cloned, err := s.fs.RepoCloned(repo)
		if err != nil {
			s.logger.Error("failed to check if repo is cloned", log.Error(err))
		} else if err := s.db.GitserverRepos().SetCloneStatus(
			// Use a background context to ensure we still update the DB even if we time out
			context.Background(),
			repo,
			cloneStatus(cloned, false),
			s.hostname,
		); err != nil {
			s.logger.Error("Setting clone status in DB", log.Error(err))
		}
	}()

	logger.Info("cloning repo", log.String("tmp", tmpDir), log.String("dst", dstPath))

	progressReader, progressWriter := io.Pipe()
	// We also capture the entire output in memory for the call to SetLastOutput
	// further down.
	// TODO: This might require a lot of memory depending on the amount of logs
	// produced, the ideal solution would be that readCloneProgress stores it in
	// chunks.
	output := &linebasedBufferedWriter{}
	eg := readFetchProgress(logger, lock, io.TeeReader(progressReader, output), repo)

	cloneTimeout := conf.GitLongCommandTimeout()
	cloneCtx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()

	// Now we start the actual clone. This works by cloning to a temporary directory
	// first, and then moving the contents into the right place in the FS once the
	// clone is done. That prevents the repo from being accessed before it's ready
	// for use.

	// First, create a tmpPath directory to clone into.
	if err := os.MkdirAll(tmpPath, os.ModePerm); err != nil {
		return errors.Wrapf(err, "clone failed to create tmp dir")
	}

	// Next, initialize a bare repo in that tmp path.
	if err := git.MakeBareRepo(ctx, tmpPath); err != nil {
		return err
	}

	fetchErr := syncer.Fetch(cloneCtx, repo, common.GitDir(tmpPath), progressWriter)
	progressWriter.Close()

	if err := eg.Wait(); err != nil {
		s.logger.Error("reading clone progress", log.Error(err))
	}

	// best-effort update the output of the clone
	if err := s.db.GitserverRepos().SetLastOutput(ctx, repo, output.String()); err != nil {
		s.logger.Error("Setting last output in DB", log.Error(err))
	}

	if fetchErr != nil {
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			return errors.Newf("failed to clone repo within deadline of %s", cloneTimeout)
		}
		// TODO: Should we really return the entire output here in an error?
		// It could be a super big error string.
		return errors.Wrapf(fetchErr, "clone failed. Output: %s", output.String())
	}

	// Set a separate timeout for post repo fetch actions, otherwise git commands
	// that are run as part of that will have the default timeout of 1 minute,
	// and we want this to succeed rather than be super fast.
	ctx, cancel = context.WithTimeout(ctx, conf.GitLongCommandTimeout())
	defer cancel()
	if err := postRepoFetchActions(ctx, logger, s.fs, s.db, s.gitBackendSource(common.GitDir(tmpPath), repo), s.hostname, repo, common.GitDir(tmpPath), syncer); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), os.ModePerm); err != nil {
		return err
	}
	if err := fileutil.RenameAndSync(tmpPath, dstPath); err != nil {
		return err
	}

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

// readFetchProgress scans the reader and saves the most recent line of output
// as the lock status, and optionally writes to a log file if siteConfig.cloneProgressLog
// is enabled.
func readFetchProgress(logger log.Logger, lock RepositoryLock, pr io.Reader, repo api.RepoName) *errgroup.Group {
	var logFile *os.File

	if conf.Get().CloneProgressLog {
		var err error
		logFile, err = os.CreateTemp("", "")
		if err != nil {
			logger.Warn("failed to create temporary fetch log file", log.Error(err), log.String("repo", string(repo)))
		} else {
			logger.Info("logging fetch output", log.String("file", logFile.Name()), log.String("repo", string(repo)))
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

var doBackgroundRepoUpdateMock func(api.RepoName) error

func (s *Server) doRepoUpdate(ctx context.Context, repo api.RepoName, lock RepositoryLock) (err error) {
	logger := s.logger.Scoped("repoUpdate").With(log.String("repo", string(repo)))

	if doBackgroundRepoUpdateMock != nil {
		return doBackgroundRepoUpdateMock(repo)
	}

	fetchTimeout := conf.GitLongCommandTimeout()

	err = func(ctx context.Context) error {
		if err = s.rpsLimiter.Wait(ctx); err != nil {
			return err
		}

		dir := s.fs.RepoDir(repo)

		syncer, err := s.getVCSSyncer(ctx, repo)
		if err != nil {
			return errors.Wrap(err, "get VCS syncer")
		}

		// ensure the background update doesn't hang forever
		fetchCtx, cancelTimeout := context.WithTimeout(ctx, fetchTimeout)
		defer cancelTimeout()

		progressReader, progressWriter := io.Pipe()
		// We also capture the entire output in memory for the call to SetLastOutput
		// further down.
		// TODO: This might require a lot of memory depending on the amount of logs
		// produced, the ideal solution would be that readCloneProgress stores it in
		// chunks.
		output := &linebasedBufferedWriter{}
		eg := readFetchProgress(logger, lock, io.TeeReader(progressReader, output), repo)

		fetchErr := syncer.Fetch(fetchCtx, repo, dir, progressWriter)
		progressWriter.Close()

		if err := eg.Wait(); err != nil {
			s.logger.Error("reading fetch progress", log.Error(err))
		}

		// best-effort store the output of the fetch
		if err := s.db.GitserverRepos().SetLastOutput(ctx, repo, output.String()); err != nil {
			s.logger.Error("Setting last output in DB", log.Error(err))
		}

		if fetchErr != nil {
			if err := fetchCtx.Err(); err != nil {
				return err
			}
			// TODO: Should we really return the entire output here in an error?
			// It could be a super big error string.
			return errors.Wrapf(err, "failed to fetch repo %q with output %q", repo, output.String())
		}

		// Set a separate timeout for post repo fetch actions, otherwise git commands
		// that are run as part of that will have the default timeout of 1 minute,
		// and we want this to succeed rather than be super fast.
		ctx, cancel := context.WithTimeout(ctx, conf.GitLongCommandTimeout())
		defer cancel()
		return postRepoFetchActions(ctx, logger, s.fs, s.db, s.gitBackendSource(dir, repo), s.hostname, repo, dir, syncer)
	}(ctx)

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Newf("failed to update repo within deadline of %s", fetchTimeout)
	}

	return err
}
