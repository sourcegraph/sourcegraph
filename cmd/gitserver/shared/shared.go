// gitserver is the gitserver server.
package shared

import (
	"container/list"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/vcs"

	"github.com/sourcegraph/log"
	"google.golang.org/grpc"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal"
	server "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/accesslog"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/cloneurl"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git/gitcli"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/perforce"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/vcssyncer"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/subrepoperms"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	connections "github.com/sourcegraph/sourcegraph/internal/database/connections/live"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/env"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/goroutine/recorder"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/instrumentation"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
	"github.com/sourcegraph/sourcegraph/internal/requestinteraction"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type LazyDebugserverEndpoint struct {
	lockerStatusEndpoint http.HandlerFunc
}

func Main(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, debugserverEndpoints *LazyDebugserverEndpoint, config *Config) error {
	logger := observationCtx.Logger

	// Load and validate configuration.
	if err := config.Validate(); err != nil {
		return errors.Wrap(err, "failed to validate configuration")
	}

	// Prepare the file system.
	fs := gitserverfs.New(observationCtx, config.ReposDir)
	if err := fs.Initialize(); err != nil {
		return err
	}

	// Create a database connection.
	sqlDB, err := getDB(observationCtx)
	if err != nil {
		return errors.Wrap(err, "initializing database stores")
	}
	db := database.NewDB(observationCtx.Logger, sqlDB)

	// Initialize the keyring.
	err = keyring.Init(ctx)
	if err != nil {
		return errors.Wrap(err, "initializing keyring")
	}

	authz.DefaultSubRepoPermsChecker = subrepoperms.NewSubRepoPermsClient(db.SubRepoPerms())

	// Setup our server megastruct.
	recordingCommandFactory := wrexec.NewRecordingCommandFactory(nil, 0)
	locker := server.NewRepositoryLocker()
	hostname := config.ExternalAddress
	backendSource := func(dir common.GitDir, repoName api.RepoName) git.GitBackend {
		return git.NewObservableBackend(gitcli.NewBackend(logger, recordingCommandFactory, dir, repoName))
	}
	gitserver := server.NewServer(&server.ServerOpts{
		Logger:           logger,
		GitBackendSource: backendSource,
		GetRemoteURLFunc: func(ctx context.Context, repo api.RepoName) (string, error) {
			return getRemoteURLFunc(ctx, db, repo)
		},
		GetVCSSyncer: func(ctx context.Context, repo api.RepoName) (vcssyncer.VCSSyncer, error) {
			return vcssyncer.NewVCSSyncer(ctx, &vcssyncer.NewVCSSyncerOpts{
				ExternalServiceStore:    db.ExternalServices(),
				RepoStore:               db.Repos(),
				DepsSvc:                 dependencies.NewService(observationCtx, db),
				Repo:                    repo,
				CoursierCacheDir:        config.CoursierCacheDir,
				RecordingCommandFactory: recordingCommandFactory,
				Logger:                  logger,
				FS:                      fs,
				GetRemoteURLSource: func(ctx context.Context, repo api.RepoName) (vcssyncer.RemoteURLSource, error) {
					return vcssyncer.RemoteURLSourceFunc(func(ctx context.Context) (*vcs.URL, error) {
						rawURL, err := getRemoteURLFunc(ctx, db, repo)
						if err != nil {
							return nil, errors.Wrapf(err, "getting remote URL for %q", repo)

						}

						u, err := vcs.ParseURL(rawURL)
						if err != nil {
							// TODO@ggilmore: Note that we can't redact the URL here because we can't
							// parse it to know where the sensitive information is.
							return nil, errors.Wrapf(err, "parsing remote URL %q", rawURL)
						}

						return u, nil

					}), nil
				},
			})
		},
		FS:                      fs,
		Hostname:                hostname,
		DB:                      db,
		Perforce:                perforce.NewService(ctx, observationCtx, logger, db, list.New()),
		RecordingCommandFactory: recordingCommandFactory,
		Locker:                  locker,
		RPSLimiter: ratelimit.NewInstrumentedLimiter(
			ratelimit.GitRPSLimiterBucketName,
			ratelimit.NewGlobalRateLimiter(logger, ratelimit.GitRPSLimiterBucketName),
		),
	})

	// Make sure we watch for config updates that affect the recordingCommandFactory.
	go conf.Watch(func() {
		// We update the factory with a predicate func. Each subsequent recordable command will use this predicate
		// to determine whether a command should be recorded or not.
		recordingConf := conf.Get().SiteConfig().GitRecorder
		if recordingConf == nil {
			recordingCommandFactory.Disable()
			return
		}
		recordingCommandFactory.Update(recordCommandsOnRepos(recordingConf.Repos, recordingConf.IgnoredGitCommands), recordingConf.Size)
	})

	internal.RegisterEchoMetric(logger.Scoped("echoMetricReporter"))

	handler := internal.NewHTTPHandler(logger, fs, backendSource)
	handler = actor.HTTPMiddleware(logger, handler)
	handler = requestclient.InternalHTTPMiddleware(handler)
	handler = requestinteraction.HTTPMiddleware(handler)
	handler = trace.HTTPMiddleware(logger, handler)
	handler = instrumentation.HTTPMiddleware("", handler)
	handler = internalgrpc.MultiplexHandlers(makeGRPCServer(logger, gitserver, config), handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	routines := []goroutine.BackgroundRoutine{
		httpserver.NewFromAddr(config.ListenAddress, &http.Server{
			Handler: handler,
		}),
		server.NewRepoStateSyncer(
			ctx,
			logger,
			db,
			locker,
			hostname,
			fs,
			config.SyncRepoStateInterval,
			config.SyncRepoStateBatchSize,
			config.SyncRepoStateUpdatePerSecond,
		),
		server.NewJanitor(
			ctx,
			server.JanitorConfig{
				ShardID:                        hostname,
				JanitorInterval:                config.JanitorInterval,
				DesiredPercentFree:             config.JanitorReposDesiredPercentFree,
				DisableDeleteReposOnWrongShard: config.JanitorDisableDeleteReposOnWrongShard,
			},
			db,
			fs,
			backendSource,
			recordingCommandFactory,
			logger,
		),
	}

	// Register recorder in all routines that support it.
	recorderCache := recorder.GetCache()
	rec := recorder.New(observationCtx.Logger, env.MyName, recorderCache)
	for _, r := range routines {
		if recordable, ok := r.(recorder.Recordable); ok {
			// Set the hostname to the shardID so we record the routines per
			// gitserver instance.
			recordable.SetJobName(fmt.Sprintf("gitserver %s", hostname))
			recordable.RegisterRecorder(rec)
			rec.Register(recordable)
		}
	}
	rec.RegistrationDone()

	debugserverEndpoints.lockerStatusEndpoint = func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(locker.AllStatuses()); err != nil {
			logger.Error("failed to encode locker statuses", log.Error(err))
		}
	}

	logger.Info("git-server: listening", log.String("addr", config.ListenAddress))

	// We're ready!
	ready()

	// Launch all routines!
	goroutine.MonitorBackgroundRoutines(ctx, routines...)

	// The most important thing this does is kill all our clones. If we just
	// shutdown they will be orphaned and continue running.
	gitserver.Stop()

	return nil
}

// makeGRPCServer creates a new *grpc.Server for the gitserver endpoints and registers
// it with methods on the given server.
func makeGRPCServer(logger log.Logger, s *server.Server, c *Config) *grpc.Server {
	configurationWatcher := conf.DefaultClient()
	scopedLogger := logger.Scoped("gitserver.accesslog")

	grpcServer := defaults.NewServer(
		logger,
		grpc.ChainStreamInterceptor(accesslog.StreamServerInterceptor(scopedLogger, configurationWatcher)),
		grpc.ChainUnaryInterceptor(accesslog.UnaryServerInterceptor(scopedLogger, configurationWatcher)),
	)
	proto.RegisterGitserverServiceServer(grpcServer, server.NewGRPCServer(s, &server.GRPCServerConfig{
		ExhaustiveRequestLoggingEnabled: c.ExhaustiveRequestLoggingEnabled,
	}))
	proto.RegisterGitserverRepositoryServiceServer(grpcServer, server.NewRepositoryServiceServer(s, &server.GRPCRepositoryServiceConfig{
		ExhaustiveRequestLoggingEnabled: c.ExhaustiveRequestLoggingEnabled,
	}))

	return grpcServer
}

// getDB initializes a connection to the database and returns a dbutil.DB
func getDB(observationCtx *observation.Context) (*sql.DB, error) {
	// Gitserver is an internal actor. We rely on the frontend to do authz checks for
	// user requests.
	//
	// This call to SetProviders is here so that calls to GetProviders don't block.
	authz.SetProviders(true, []authz.Provider{})

	dsn := conf.GetServiceConnectionValueAndRestartOnChange(func(serviceConnections conftypes.ServiceConnections) string {
		return serviceConnections.PostgresDSN
	})
	return connections.EnsureNewFrontendDB(observationCtx, dsn, "gitserver")
}

// getRemoteURLFunc returns a remote URL for the given repo, if any external service
// connections reference it. The first external service mentioned in repo.Sources
// will be used to construct the URL and get credentials.
// Since r.Sources is a map, a random referencing service will be used, so this
// function is not idempotent.
// This allows us to try different tokens, to maximize the chances of a repo eventually
// cloning successfully.
func getRemoteURLFunc(
	ctx context.Context,
	db database.DB,
	repo api.RepoName,
) (string, error) {
	r, err := db.Repos().GetByName(ctx, repo)
	if err != nil {
		return "", err
	}

	for _, info := range r.Sources {
		// build the clone url using the external service config instead of using
		// the source CloneURL field
		svc, err := db.ExternalServices().GetByID(ctx, info.ExternalServiceID())
		if err != nil {
			return "", err
		}

		return cloneurl.ForEncryptableConfig(ctx, db, svc.Kind, svc.Config, r)
	}
	return "", errors.Errorf("no sources for %q", repo)
}

var defaultIgnoredGitCommands = []string{
	"show",
	"rev-parse",
	"log",
	"diff",
	"ls-tree",
}

// recordCommandsOnRepos returns a ShouldRecordFunc which determines whether the given command should be recorded
// for a particular repository.
func recordCommandsOnRepos(repos []string, ignoredGitCommands []string) wrexec.ShouldRecordFunc {
	// empty repos, means we should never record since there is nothing to match on
	if len(repos) == 0 {
		return func(ctx context.Context, c *exec.Cmd) bool {
			return false
		}
	}

	if len(ignoredGitCommands) == 0 {
		ignoredGitCommands = append(ignoredGitCommands, defaultIgnoredGitCommands...)
	}

	// we won't record any git commands with these commands since they are considered to be not destructive
	ignoredGitCommandsMap := collections.NewSet(ignoredGitCommands...)

	return func(_ context.Context, cmd *exec.Cmd) bool {
		base := filepath.Base(cmd.Path)
		if base != "git" {
			return false
		}

		repoMatch := false
		// If repos contains a single "*" element, it means to record commands
		// for all repositories.
		if len(repos) == 1 && repos[0] == "*" {
			repoMatch = true
		} else {
			for _, repo := range repos {
				// We need to check the suffix, because we can have some common parts in
				// different repo names. E.g. "sourcegraph/sourcegraph" and
				// "sourcegraph/sourcegraph-code-ownership" will both be allowed even if only the
				// first name is included in the config.
				if strings.HasSuffix(cmd.Dir, repo+"/.git") {
					repoMatch = true
					break
				}
			}
		}

		// If the repo doesn't match, no use in checking if it is a command we should record.
		if !repoMatch {
			return false
		}
		// we have to scan the Args, since it isn't guaranteed that the Arg at index 1 is the git command:
		// git -c "protocol.version=2" remote show
		for _, arg := range cmd.Args {
			if ok := ignoredGitCommandsMap.Has(arg); ok {
				return false
			}
		}
		return true
	}
}
