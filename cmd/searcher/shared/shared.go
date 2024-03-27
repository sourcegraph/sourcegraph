// Package shared is the shared main entrypoint for searcher, a simple service which exposes an API
// to text search a repo at a specific commit. See the searcher package for more information.
package shared

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/keegancsmith/tmpfriend"
	"github.com/sourcegraph/log"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	sharedsearch "github.com/sourcegraph/sourcegraph/internal/search"
	proto "github.com/sourcegraph/sourcegraph/internal/searcher/v1"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	cacheDirName = env.ChooseFallbackVariableName("SEARCHER_CACHE_DIR", "CACHE_DIR")

	cacheDir    = env.Get(cacheDirName, "/tmp", "directory to store cached archives.")
	cacheSizeMB = env.Get("SEARCHER_CACHE_SIZE_MB", "100000", "maximum size of the on disk cache in megabytes")

	// Same environment variable name (and default value) used by symbols.
	backgroundTimeout = env.MustGetDuration("PROCESSING_TIMEOUT", 2*time.Hour, "maximum time to spend processing a repository")

	maxTotalPathsLengthRaw = env.Get("MAX_TOTAL_PATHS_LENGTH", "100000", "maximum sum of lengths of all paths in a single call to git archive")

	disableHybridSearch = env.MustGetBool("DISABLE_HYBRID_SEARCH", false, "if true, unindexed search will not consult indexed search to speed up searches")
)

const port = "3181"

func shutdownOnSignal(ctx context.Context, server *http.Server) error {
	// Listen for shutdown signals. When we receive one attempt to clean up,
	// but do an insta-shutdown if we receive more than one signal.
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)

	// Once we receive one of the signals from above, continues with the shutdown
	// process.
	select {
	case <-c:
	case <-ctx.Done(): // still call shutdown below
	}

	go func() {
		// If a second signal is received, exit immediately.
		<-c
		os.Exit(1)
	}()

	// Wait for at most for the configured shutdown timeout.
	ctx, cancel := context.WithTimeout(ctx, goroutine.GracefulShutdownTimeout)
	defer cancel()
	// Stop accepting requests.
	return server.Shutdown(ctx)
}

// setupTmpDir sets up a temporary directory on the same volume as the
// cacheDir.
//
// Structural search relies on temporary files created from zoekt responses.
// Additionally we shell out to programs that may or may not need a temporary
// directory.
//
// search.Store will also take into account the files in tmp when deciding on
// evicting items due to disk pressure. It won't delete those files unless
// they are zip files. In the case of comby the files are temporary so them
// being deleted while read by comby is fine since it will maintain an open
// FD.
func setupTmpDir() error {
	tmpRoot := filepath.Join(cacheDir, ".searcher.tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return err
	}
	if !tmpfriend.IsTmpFriendDir(tmpRoot) {
		_, err := tmpfriend.RootTempDir(tmpRoot)
		return err
	}
	return nil
}

func Start(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc) error {
	logger := observationCtx.Logger

	// Ready as soon as the database connection has been established.
	ready()

	var cacheSizeBytes int64
	if i, err := strconv.ParseInt(cacheSizeMB, 10, 64); err != nil {
		return errors.Wrapf(err, "invalid int %q for SEARCHER_CACHE_SIZE_MB", cacheSizeMB)
	} else {
		cacheSizeBytes = i * 1000 * 1000
	}

	maxTotalPathsLength, err := strconv.Atoi(maxTotalPathsLengthRaw)
	if err != nil {
		return errors.Wrapf(err, "invalid int %q for MAX_TOTAL_PATHS_LENGTH", maxTotalPathsLengthRaw)
	}

	if err := setupTmpDir(); err != nil {
		return errors.Wrap(err, "failed to setup TMPDIR")
	}

	// Explicitly don't scope Store logger under the parent logger
	storeObservationCtx := observation.NewContext(log.Scoped("Store"))

	git := gitserver.NewClient("searcher")

	sService := &search.Service{
		Store: &search.Store{
			GitserverClient: git,
			FetchTar: func(ctx context.Context, repo api.RepoName, commit api.CommitID) (io.ReadCloser, error) {
				// We pass in a nil sub-repo permissions checker and an internal actor here since
				// searcher needs access to all data in the archive.
				ctx = actor.WithInternalActor(ctx)
				return git.ArchiveReader(ctx, repo, gitserver.ArchiveOptions{
					Treeish: string(commit),
					Format:  gitserver.ArchiveFormatTar,
				})
			},
			FetchTarPaths: func(ctx context.Context, repo api.RepoName, commit api.CommitID, paths []string) (io.ReadCloser, error) {
				// We pass in a nil sub-repo permissions checker and an internal actor here since
				// searcher needs access to all data in the archive.
				ctx = actor.WithInternalActor(ctx)
				return git.ArchiveReader(ctx, repo, gitserver.ArchiveOptions{
					Treeish: string(commit),
					Format:  gitserver.ArchiveFormatTar,
					Paths:   paths,
				})
			},
			FilterTar:         search.NewFilter,
			Path:              filepath.Join(cacheDir, "searcher-archives"),
			MaxCacheSizeBytes: cacheSizeBytes,
			BackgroundTimeout: backgroundTimeout,
			Log:               storeObservationCtx.Logger,
			ObservationCtx:    storeObservationCtx,
		},

		Indexed: sharedsearch.Indexed(),

		GitDiffSymbols: func(ctx context.Context, repo api.RepoName, commitA, commitB api.CommitID) ([]byte, error) {
			// As this is an internal service call, we need an internal actor.
			ctx = actor.WithInternalActor(ctx)
			return git.DiffSymbols(ctx, repo, commitA, commitB)
		},
		MaxTotalPathsLength: maxTotalPathsLength,

		Log: logger,

		DisableHybridSearch: disableHybridSearch,
	}
	sService.Store.Start()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	grpcServer := defaults.NewServer(logger)
	proto.RegisterSearcherServiceServer(grpcServer, &search.Server{
		Service: sService,
	})

	addr := getAddr()
	server := &http.Server{
		ReadTimeout:  75 * time.Second,
		WriteTimeout: 10 * time.Minute,
		Addr:         addr,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
		Handler: internalgrpc.MultiplexHandlers(grpcServer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For cluster liveness and readiness probes
			if r.URL.Path == "/healthz" {
				w.WriteHeader(200)
				_, _ = w.Write([]byte("ok"))
				return
			}
			http.NotFoundHandler().ServeHTTP(w, r)
		})),
	}

	g, ctx := errgroup.WithContext(ctx)

	// Listen
	g.Go(func() error {
		logger.Info("listening", log.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	// Shutdown
	g.Go(func() error {
		return shutdownOnSignal(ctx, server)
	})

	return g.Wait()
}

func getAddr() string {
	host := ""
	if env.InsecureDev {
		host = "127.0.0.1"
	}

	return net.JoinHostPort(host, port)
}
