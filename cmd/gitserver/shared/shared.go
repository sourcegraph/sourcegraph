// gitserver is the gitserver server.
package shared

import (
	"container/list"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/sourcegraph/log"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/accesslog"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/perforce"
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
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/crates"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gomodproxy"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/npm"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/pypi"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/rubygems"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/goroutine/recorder"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/instrumentation"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func Main(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, config *Config) error {
	logger := observationCtx.Logger

	// Load and validate configuration.
	if err := config.Validate(); err != nil {
		return errors.Wrap(err, "failed to validate configuration")
	}

	// Prepare the file system.
	{
		// Ensure the ReposDir exists.
		if err := os.MkdirAll(config.ReposDir, os.ModePerm); err != nil {
			return errors.Wrap(err, "creating SRC_REPOS_DIR")
		}
		// Ensure the Perforce Dir exists.
		p4Home := filepath.Join(config.ReposDir, server.P4HomeName)
		if err := os.MkdirAll(p4Home, os.ModePerm); err != nil {
			return errors.Wrapf(err, "ensuring p4Home exists: %q", p4Home)
		}
		// Ensure the tmp dir exists, is cleaned up, and TMP_DIR is set properly.
		tmpDir, err := setupAndClearTmp(logger, config.ReposDir)
		if err != nil {
			return errors.Wrap(err, "failed to setup temporary directory")
		}
		// Additionally, set TMP_DIR so other temporary files we may accidentally
		// create are on the faster RepoDir mount.
		if err := os.Setenv("TMP_DIR", tmpDir); err != nil {
			return errors.Wrap(err, "setting TMP_DIR")
		}

		// Delete the old reposStats file, which was used on gitserver prior to
		// 2023-08-14.
		if err := os.Remove(filepath.Join(config.ReposDir, "repos-stats.json")); err != nil && !os.IsNotExist(err) {
			logger.Error("failed to remove old reposStats file", log.Error(err))
		}
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

	authz.DefaultSubRepoPermsChecker, err = subrepoperms.NewSubRepoPermsClient(db.SubRepoPerms())
	if err != nil {
		return errors.Wrap(err, "failed to create sub-repo client")
	}

	// Setup our server megastruct.
	recordingCommandFactory := wrexec.NewRecordingCommandFactory(nil, 0)
	cloneQueue := server.NewCloneQueue(observationCtx, list.New())
	locker := server.NewRepositoryLocker()
	gitserver := server.Server{
		Logger:         logger,
		ObservationCtx: observationCtx,
		ReposDir:       config.ReposDir,
		GetRemoteURLFunc: func(ctx context.Context, repo api.RepoName) (string, error) {
			return getRemoteURLFunc(ctx, logger, db, repo)
		},
		GetVCSSyncer: func(ctx context.Context, repo api.RepoName) (server.VCSSyncer, error) {
			return getVCSSyncer(ctx, &newVCSSyncerOpts{
				externalServiceStore:    db.ExternalServices(),
				repoStore:               db.Repos(),
				depsSvc:                 dependencies.NewService(observationCtx, db),
				repo:                    repo,
				reposDir:                config.ReposDir,
				coursierCacheDir:        config.CoursierCacheDir,
				recordingCommandFactory: recordingCommandFactory,
			})
		},
		Hostname:                config.ExternalAddress,
		DB:                      db,
		CloneQueue:              cloneQueue,
		GlobalBatchLogSemaphore: semaphore.NewWeighted(int64(config.BatchLogGlobalConcurrencyLimit)),
		Perforce:                perforce.NewService(ctx, observationCtx, logger, db, list.New()),
		RecordingCommandFactory: recordingCommandFactory,
		Locker:                  locker,
		RPSLimiter: ratelimit.NewInstrumentedLimiter(
			ratelimit.GitRPSLimiterBucketName,
			ratelimit.NewGlobalRateLimiter(ratelimit.GitRPSLimiterBucketName),
		),
	}

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

	gitserver.RegisterMetrics(observationCtx, db)

	// Create Handler now since it also initializes state
	// TODO: Why do we set server state as a side effect of creating our handler?
	handler := gitserver.Handler()
	handler = actor.HTTPMiddleware(logger, handler)
	handler = requestclient.InternalHTTPMiddleware(handler)
	handler = trace.HTTPMiddleware(logger, handler, conf.DefaultClient())
	handler = instrumentation.HTTPMiddleware("", handler)
	handler = internalgrpc.MultiplexHandlers(makeGRPCServer(logger, &gitserver), handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	routines := []goroutine.BackgroundRoutine{
		httpserver.NewFromAddr(config.ListenAddress, &http.Server{
			Handler: handler,
		}),
		gitserver.NewClonePipeline(logger, cloneQueue),
		server.NewRepoStateSyncer(
			ctx,
			logger,
			db,
			locker,
			gitserver.Hostname,
			config.ReposDir,
			config.SyncRepoStateInterval,
			config.SyncRepoStateBatchSize,
			config.SyncRepoStateUpdatePerSecond,
		),
	}

	if runtime.GOOS == "windows" {
		// See https://github.com/sourcegraph/sourcegraph/issues/54317 for details.
		logger.Warn("Janitor is disabled on windows")
	} else {
		routines = append(
			routines,
			server.NewJanitor(
				ctx,
				server.JanitorConfig{
					ShardID:            gitserver.Hostname,
					JanitorInterval:    config.JanitorInterval,
					ReposDir:           config.ReposDir,
					DesiredPercentFree: config.JanitorReposDesiredPercentFree,
				},
				db,
				recordingCommandFactory,
				gitserver.CloneRepo,
				logger,
			),
		)
	}

	// Register recorder in all routines that support it.
	recorderCache := recorder.GetCache()
	rec := recorder.New(observationCtx.Logger, env.MyName, recorderCache)
	for _, r := range routines {
		if recordable, ok := r.(recorder.Recordable); ok {
			// Set the hostname to the shardID so we record the routines per
			// gitserver instance.
			recordable.SetJobName(fmt.Sprintf("gitserver %s", gitserver.Hostname))
			recordable.RegisterRecorder(rec)
			rec.Register(recordable)
		}
	}
	rec.RegistrationDone()

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
func makeGRPCServer(logger log.Logger, s *server.Server) *grpc.Server {
	configurationWatcher := conf.DefaultClient()

	var additionalServerOptions []grpc.ServerOption

	for method, scopedLogger := range map[string]log.Logger{
		proto.GitserverService_Exec_FullMethodName:      logger.Scoped("exec.accesslog", "exec endpoint access log"),
		proto.GitserverService_Archive_FullMethodName:   logger.Scoped("archive.accesslog", "archive endpoint access log"),
		proto.GitserverService_P4Exec_FullMethodName:    logger.Scoped("p4exec.accesslog", "p4-exec endpoint access log"),
		proto.GitserverService_GetObject_FullMethodName: logger.Scoped("get-object.accesslog", "get-object endpoint access log"),
	} {
		streamInterceptor := accesslog.StreamServerInterceptor(scopedLogger, configurationWatcher)
		unaryInterceptor := accesslog.UnaryServerInterceptor(scopedLogger, configurationWatcher)

		additionalServerOptions = append(additionalServerOptions,
			grpc.ChainStreamInterceptor(methodSpecificStreamInterceptor(method, streamInterceptor)),
			grpc.ChainUnaryInterceptor(methodSpecificUnaryInterceptor(method, unaryInterceptor)),
		)
	}

	grpcServer := defaults.NewServer(logger, additionalServerOptions...)
	proto.RegisterGitserverServiceServer(grpcServer, &server.GRPCServer{
		Server: s,
	})

	return grpcServer
}

func configureFusionClient(conn schema.PerforceConnection) server.FusionConfig {
	// Set up default settings first
	fc := server.FusionConfig{
		Enabled:             false,
		Client:              conn.P4Client,
		LookAhead:           2000,
		NetworkThreads:      12,
		NetworkThreadsFetch: 12,
		PrintBatch:          10,
		Refresh:             100,
		Retries:             10,
		MaxChanges:          -1,
		IncludeBinaries:     false,
		FsyncEnable:         false,
	}

	if conn.FusionClient == nil {
		return fc
	}

	// Required
	fc.Enabled = conn.FusionClient.Enabled
	fc.LookAhead = conn.FusionClient.LookAhead

	// Optional
	if conn.FusionClient.NetworkThreads > 0 {
		fc.NetworkThreads = conn.FusionClient.NetworkThreads
	}
	if conn.FusionClient.NetworkThreadsFetch > 0 {
		fc.NetworkThreadsFetch = conn.FusionClient.NetworkThreadsFetch
	}
	if conn.FusionClient.PrintBatch > 0 {
		fc.PrintBatch = conn.FusionClient.PrintBatch
	}
	if conn.FusionClient.Refresh > 0 {
		fc.Refresh = conn.FusionClient.Refresh
	}
	if conn.FusionClient.Retries > 0 {
		fc.Retries = conn.FusionClient.Retries
	}
	if conn.FusionClient.MaxChanges > 0 {
		fc.MaxChanges = conn.FusionClient.MaxChanges
	}
	fc.IncludeBinaries = conn.FusionClient.IncludeBinaries
	fc.FsyncEnable = conn.FusionClient.FsyncEnable

	return fc
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
func getRemoteURLFunc(
	ctx context.Context,
	logger log.Logger,
	db database.DB,
	repo api.RepoName,
) (string, error) {
	r, err := db.Repos().GetByName(ctx, repo)
	if err != nil {
		rs, err2 := db.Repos().List(actor.WithInternalActor(context.Background()), database.ReposListOptions{IncludeDeleted: true})
		if err2 != nil {
			logger.Error("failed to list all repos", log.Error(err2))
		} else {
			logger.Error("failed to get repos, here's the table", log.Error(err))
			out, _ := json.Marshal(rs)
			fmt.Printf("repos: %#+v\n", string(out))
		}
		return "", err
	}

	for _, info := range r.Sources {
		// build the clone url using the external service config instead of using
		// the source CloneURL field
		svc, err := db.ExternalServices().GetByID(ctx, info.ExternalServiceID())
		if err != nil {
			return "", err
		}

		if svc.CloudDefault && r.Private {
			// We won't be able to use this remote URL, so we should skip it. This can happen
			// if a repo moves from being public to private while belonging to both a cloud
			// default external service and another external service with a token that has
			// access to the private repo.
			// TODO: This should not be possible anymore, can we remove this check?
			continue
		}

		return repos.EncryptableCloneURL(ctx, logger.Scoped("repos.CloneURL", ""), db, svc.Kind, svc.Config, r)
	}
	return "", errors.Errorf("no sources for %q", repo)
}

type newVCSSyncerOpts struct {
	externalServiceStore    database.ExternalServiceStore
	repoStore               database.RepoStore
	depsSvc                 *dependencies.Service
	repo                    api.RepoName
	reposDir                string
	coursierCacheDir        string
	recordingCommandFactory *wrexec.RecordingCommandFactory
}

func getVCSSyncer(ctx context.Context, opts *newVCSSyncerOpts) (server.VCSSyncer, error) {
	// We need an internal actor in case we are trying to access a private repo. We
	// only need access in order to find out the type of code host we're using, so
	// it's safe.
	r, err := opts.repoStore.GetByName(actor.WithInternalActor(ctx), opts.repo)
	if err != nil {
		return nil, errors.Wrap(err, "get repository")
	}

	extractOptions := func(connection any) (string, error) {
		for _, info := range r.Sources {
			extSvc, err := opts.externalServiceStore.GetByID(ctx, info.ExternalServiceID())
			if err != nil {
				return "", errors.Wrap(err, "get external service")
			}
			rawConfig, err := extSvc.Config.Decrypt(ctx)
			if err != nil {
				return "", err
			}
			normalized, err := jsonc.Parse(rawConfig)
			if err != nil {
				return "", errors.Wrap(err, "normalize JSON")
			}
			if err = jsoniter.Unmarshal(normalized, connection); err != nil {
				return "", errors.Wrap(err, "unmarshal JSON")
			}
			return extSvc.URN(), nil
		}
		return "", errors.Errorf("unexpected empty Sources map in %v", r)
	}

	switch r.ExternalRepo.ServiceType {
	case extsvc.TypePerforce:
		var c schema.PerforceConnection
		if _, err := extractOptions(&c); err != nil {
			return nil, err
		}

		p4Home := filepath.Join(opts.reposDir, server.P4HomeName)
		// Ensure the directory exists
		if err := os.MkdirAll(p4Home, os.ModePerm); err != nil {
			return nil, errors.Wrapf(err, "ensuring p4Home exists: %q", p4Home)
		}

		return &server.PerforceDepotSyncer{
			MaxChanges:   int(c.MaxChanges),
			Client:       c.P4Client,
			FusionConfig: configureFusionClient(c),
			P4Home:       p4Home,
		}, nil
	case extsvc.TypeJVMPackages:
		var c schema.JVMPackagesConnection
		if _, err := extractOptions(&c); err != nil {
			return nil, err
		}
		return server.NewJVMPackagesSyncer(&c, opts.depsSvc, opts.coursierCacheDir), nil
	case extsvc.TypeNpmPackages:
		var c schema.NpmPackagesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli, err := npm.NewHTTPClient(urn, c.Registry, c.Credentials, httpcli.ExternalClientFactory)
		if err != nil {
			return nil, err
		}
		return server.NewNpmPackagesSyncer(c, opts.depsSvc, cli), nil
	case extsvc.TypeGoModules:
		var c schema.GoModulesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli := gomodproxy.NewClient(urn, c.Urls, httpcli.ExternalClientFactory)
		return server.NewGoModulesSyncer(&c, opts.depsSvc, cli), nil
	case extsvc.TypePythonPackages:
		var c schema.PythonPackagesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli, err := pypi.NewClient(urn, c.Urls, httpcli.ExternalClientFactory)
		if err != nil {
			return nil, err
		}
		return server.NewPythonPackagesSyncer(&c, opts.depsSvc, cli, opts.reposDir), nil
	case extsvc.TypeRustPackages:
		var c schema.RustPackagesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli, err := crates.NewClient(urn, httpcli.ExternalClientFactory)
		if err != nil {
			return nil, err
		}
		return server.NewRustPackagesSyncer(&c, opts.depsSvc, cli), nil
	case extsvc.TypeRubyPackages:
		var c schema.RubyPackagesConnection
		urn, err := extractOptions(&c)
		if err != nil {
			return nil, err
		}
		cli, err := rubygems.NewClient(urn, c.Repository, httpcli.ExternalClientFactory)
		if err != nil {
			return nil, err
		}
		return server.NewRubyPackagesSyncer(&c, opts.depsSvc, cli), nil
	}
	return server.NewGitRepoSyncer(opts.recordingCommandFactory), nil
}

// methodSpecificStreamInterceptor returns a gRPC stream server interceptor that only calls the next interceptor if the method matches.
//
// The returned interceptor will call next if the invoked gRPC method matches the method parameter. Otherwise, it will call handler directly.
func methodSpecificStreamInterceptor(method string, next grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if method != info.FullMethod {
			return handler(srv, ss)
		}

		return next(srv, ss, info, handler)
	}
}

// methodSpecificUnaryInterceptor returns a gRPC unary server interceptor that only calls the next interceptor if the method matches.
//
// The returned interceptor will call next if the invoked gRPC method matches the method parameter. Otherwise, it will call handler directly.
func methodSpecificUnaryInterceptor(method string, next grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if method != info.FullMethod {
			return handler(ctx, req)
		}

		return next(ctx, req, info, handler)
	}
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

	return func(ctx context.Context, cmd *exec.Cmd) bool {
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

// setupAndClearTmp sets up the tempdir for reposDir as well as clearing it
// out. It returns the temporary directory location.
func setupAndClearTmp(logger log.Logger, reposDir string) (string, error) {
	logger = logger.Scoped("setupAndClearTmp", "sets up the the tempdir for ReposDir as well as clearing it out")

	// Additionally, we create directories with the prefix .tmp-old which are
	// asynchronously removed. We do not remove in place since it may be a
	// slow operation to block on. Our tmp dir will be ${s.ReposDir}/.tmp
	dir := filepath.Join(reposDir, server.TempDirName) // .tmp
	oldPrefix := server.TempDirName + "-old"
	if _, err := os.Stat(dir); err == nil {
		// Rename the current tmp file, so we can asynchronously remove it. Use
		// a consistent pattern so if we get interrupted, we can clean it
		// another time.
		oldTmp, err := os.MkdirTemp(reposDir, oldPrefix)
		if err != nil {
			return "", err
		}
		// oldTmp dir exists, so we need to use a child of oldTmp as the
		// rename target.
		if err := os.Rename(dir, filepath.Join(oldTmp, server.TempDirName)); err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}

	// Asynchronously remove old temporary directories.
	// TODO: Why async?
	files, err := os.ReadDir(reposDir)
	if err != nil {
		logger.Error("failed to do tmp cleanup", log.Error(err))
	} else {
		for _, f := range files {
			// Remove older .tmp directories as well as our older tmp-
			// directories we would place into ReposDir. In September 2018 we
			// can remove support for removing tmp- directories.
			if !strings.HasPrefix(f.Name(), oldPrefix) && !strings.HasPrefix(f.Name(), "tmp-") {
				continue
			}
			go func(path string) {
				if err := os.RemoveAll(path); err != nil {
					logger.Error("failed to remove old temporary directory", log.String("path", path), log.Error(err))
				}
			}(filepath.Join(reposDir, f.Name()))
		}
	}

	return dir, nil
}
