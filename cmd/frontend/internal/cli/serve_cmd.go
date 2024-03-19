package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"net/http"
	"os"
	"time"

	"github.com/go-logr/stdr"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/keegancsmith/tmpfriend"
	sglog "github.com/sourcegraph/log"
	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/redigostore"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/bg"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/httpapi"
	oce "github.com/sourcegraph/sourcegraph/cmd/frontend/oneclickexport"
	"github.com/sourcegraph/sourcegraph/internal/adminanalytics"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth/userpasswd"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/database"
	connections "github.com/sourcegraph/sourcegraph/internal/database/connections/live"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/store"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/highlight"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/internal/sysreq"
	"github.com/sourcegraph/sourcegraph/internal/updatecheck"
	"github.com/sourcegraph/sourcegraph/internal/users"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/internal/version/upgradestore"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	printLogo = env.MustGetBool("LOGO", false, "print Sourcegraph logo upon startup")

	httpAddr = env.Get("SRC_HTTP_ADDR", func() string {
		if env.InsecureDev {
			return "127.0.0.1:3080"
		}
		return ":3080"
	}(), "HTTP listen address for app and HTTP API")
	httpAddrInternal = envvar.HTTPAddrInternal

	// dev browser extension ID. You can find this by going to chrome://extensions
	devExtension = "chrome-extension://bmfbcejdknlknpncfpeloejonjoledha"
	// production browser extension ID. This is found by viewing our extension in the chrome store.
	prodExtension = "chrome-extension://dgjhfomjieaadpoljlnidmbgkdffpack"
)

// InitDB initializes and returns the global database connection and sets the
// version of the frontend in our versions table.
func InitDB(logger sglog.Logger) (*sql.DB, error) {
	sqlDB, err := connections.EnsureNewFrontendDB(observation.ContextWithLogger(logger, &observation.TestContext), "", "frontend")
	if err != nil {
		return nil, errors.Errorf("failed to connect to frontend database: %s", err)
	}

	if err := upgradestore.New(database.NewDB(logger, sqlDB)).UpdateServiceVersion(context.Background(), version.Version()); err != nil {
		return nil, err
	}

	return sqlDB, nil
}

type LazyDebugserverEndpoint struct {
	GlobalRateLimiterState       http.Handler
	ListAuthzProvidersEndpoint   http.Handler
	GitserverReposStatusEndpoint http.Handler
}

type SetupFunc func(database.DB, conftypes.UnifiedWatchable) enterprise.Services

// Main is the main entrypoint for the frontend server program.
func Main(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, debugserverEndpoints *LazyDebugserverEndpoint, enterpriseSetupHook SetupFunc, enterpriseMigratorHook store.RegisterMigratorsUsingConfAndStoreFactoryFunc) error {
	logger := observationCtx.Logger

	if err := tryAutoUpgrade(ctx, observationCtx, ready, enterpriseMigratorHook); err != nil {
		return errors.Wrap(err, "frontend.tryAutoUpgrade")
	}

	sqlDB, err := InitDB(logger)
	if err != nil {
		return err
	}
	db := database.NewDB(logger, sqlDB)

	// Used by opentelemetry logging
	stdr.SetVerbosity(10)

	if os.Getenv("SRC_DISABLE_OOBMIGRATION_VALIDATION") != "" {
		logger.Warn("Skipping out-of-band migrations check")
	} else {
		outOfBandMigrationRunner := oobmigration.NewRunnerWithDB(observationCtx, db, oobmigration.RefreshInterval)

		if err := outOfBandMigrationRunner.SynchronizeMetadata(ctx); err != nil {
			return errors.Wrap(err, "failed to synchronize out of band migration metadata")
		}

		if err := oobmigration.ValidateOutOfBandMigrationRunner(ctx, db, outOfBandMigrationRunner); err != nil {
			return errors.Wrap(err, "failed to validate out of band migrations")
		}
	}

	userpasswd.Init()
	highlight.Init()

	// override site config first
	if err := overrideSiteConfig(ctx, logger, db); err != nil {
		return errors.Wrap(err, "failed to apply site config overrides")
	}
	globals.ConfigurationServerFrontendOnly = conf.InitConfigurationServerFrontendOnly(newConfigurationSource(logger, db))
	conf.MustValidateDefaults()

	// now we can init the keyring, as it depends on site config
	if err := keyring.Init(ctx); err != nil {
		return errors.Wrap(err, "failed to initialize encryption keyring")
	}

	if err := overrideGlobalSettings(ctx, logger, db); err != nil {
		return errors.Wrap(err, "failed to override global settings")
	}

	// now the keyring is configured it's safe to override the rest of the config
	// and that config can access the keyring
	if err := overrideExtSvcConfig(ctx, logger, db); err != nil {
		return errors.Wrap(err, "failed to override external service config")
	}

	// Run enterprise setup hook
	enterpriseServices := enterpriseSetupHook(db, conf.DefaultClient())

	ui.InitRouter(db)

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "help", "-h", "--help":
			log.Printf("Version: %s", version.Version())
			log.Print()

			log.Print(env.HelpString())

			log.Print()
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			for _, st := range sysreq.Check(ctx, skippedSysReqs()) {
				log.Printf("%s:", st.Name)
				if st.OK() {
					log.Print("\tOK")
					continue
				}
				if st.Skipped {
					log.Print("\tSkipped")
					continue
				}
				if st.Problem != "" {
					log.Print("\t" + st.Problem)
				}
				if st.Err != nil {
					log.Printf("\tError: %s", st.Err)
				}
				if st.Fix != "" {
					log.Printf("\tPossible fix: %s", st.Fix)
				}
			}

			return nil
		}
	}

	printConfigValidation(logger)

	cleanup := tmpfriend.SetupOrNOOP()
	defer cleanup()

	// Don't proceed if system requirements are missing, to avoid
	// presenting users with a half-working experience.
	if err := checkSysReqs(context.Background(), os.Stderr); err != nil {
		return err
	}

	globals.WatchBranding()
	globals.WatchExternalURL()

	goroutine.Go(func() { bg.CheckRedisCacheEvictionPolicy() })
	goroutine.Go(func() { bg.DeleteOldCacheDataInRedis() })
	goroutine.Go(func() { bg.DeleteOldEventLogsInPostgres(context.Background(), logger, db) })
	goroutine.Go(func() { bg.DeleteOldSecurityEventLogsInPostgres(context.Background(), logger, db) })
	goroutine.Go(func() { bg.UpdatePermissions(ctx, logger, db) })
	goroutine.Go(func() { updatecheck.Start(logger, db) })
	goroutine.Go(func() { adminanalytics.StartAnalyticsCacheRefresh(context.Background(), db) })
	goroutine.Go(func() { users.StartUpdateAggregatedUsersStatisticsTable(context.Background(), db) })

	schema, err := graphqlbackend.NewSchema(
		db,
		gitserver.NewClient("graphql.schemaresolver"),
		[]graphqlbackend.OptionalResolver{enterpriseServices.OptionalResolver},
	)
	if err != nil {
		return err
	}

	rateLimitWatcher, err := makeRateLimitWatcher()
	if err != nil {
		return err
	}

	server, err := makeExternalAPI(db, logger, schema, enterpriseServices, rateLimitWatcher)
	if err != nil {
		return err
	}

	internalAPI, err := makeInternalAPI(db, logger, schema, enterpriseServices, rateLimitWatcher)
	if err != nil {
		return err
	}

	routines := []goroutine.BackgroundRoutine{server}
	if internalAPI != nil {
		routines = append(routines, internalAPI)
	}

	oce.GlobalExporter = oce.NewDataExporter(db, logger)

	debugserverEndpoints.GlobalRateLimiterState = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := ratelimit.GetGlobalLimiterState(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read rate limiter state: %q", err.Error()), http.StatusInternalServerError)
			return
		}
		resp, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal rate limiter state: %q", err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})
	debugserverEndpoints.GitserverReposStatusEndpoint = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo := r.FormValue("repo")
		if repo == "" {
			http.Error(w, "missing 'repo' param", http.StatusBadRequest)
			return
		}

		status, err := db.GitserverRepos().GetByName(r.Context(), api.RepoName(repo))
		if err != nil {
			http.Error(w, fmt.Sprintf("fetching repository status: %q", err), http.StatusInternalServerError)
			return
		}

		resp, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal status: %q", err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})
	debugserverEndpoints.ListAuthzProvidersEndpoint = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type providerInfo struct {
			ServiceType        string `json:"service_type"`
			ServiceID          string `json:"service_id"`
			ExternalServiceURL string `json:"external_service_url"`
		}

		_, providers := authz.GetProviders()
		infos := make([]providerInfo, len(providers))
		for i, p := range providers {
			_, id := extsvc.DecodeURN(p.URN())

			// Note that the ID marshalling below replicates code found in `graphqlbackend`.
			// We cannot import that package's code into this one (see /dev/check/go-dbconn-import.sh).
			infos[i] = providerInfo{
				ServiceType:        p.ServiceType(),
				ServiceID:          p.ServiceID(),
				ExternalServiceURL: fmt.Sprintf("%s/site-admin/external-services/%s", globals.ExternalURL(), relay.MarshalID("ExternalService", id)),
			}
		}

		resp, err := json.MarshalIndent(infos, "", "  ")
		if err != nil {
			http.Error(w, "failed to marshal infos: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})

	if printLogo {
		// This is not a log entry and is usually disabled
		println(fmt.Sprintf("\n\n%s\n\n", logoColor))
	}
	logger.Info(fmt.Sprintf("✱ Sourcegraph is ready at: %s", globals.ExternalURL()))
	ready()

	goroutine.MonitorBackgroundRoutines(context.Background(), routines...)
	return nil
}

func makeExternalAPI(db database.DB, logger sglog.Logger, schema *graphql.Schema, enterprise enterprise.Services, rateLimiter graphqlbackend.LimitWatcher) (goroutine.BackgroundRoutine, error) {
	listener, err := httpserver.NewListener(httpAddr)
	if err != nil {
		return nil, err
	}

	// Create the external HTTP handler.
	externalHandler, err := newExternalHTTPHandler(
		db,
		schema,
		rateLimiter,
		&httpapi.Handlers{
			GitHubSyncWebhook:               enterprise.ReposGithubWebhook,
			GitLabSyncWebhook:               enterprise.ReposGitLabWebhook,
			BitbucketServerSyncWebhook:      enterprise.ReposBitbucketServerWebhook,
			BitbucketCloudSyncWebhook:       enterprise.ReposBitbucketCloudWebhook,
			PermissionsGitHubWebhook:        enterprise.PermissionsGitHubWebhook,
			BatchesGitHubWebhook:            enterprise.BatchesGitHubWebhook,
			BatchesGitLabWebhook:            enterprise.BatchesGitLabWebhook,
			BatchesBitbucketServerWebhook:   enterprise.BatchesBitbucketServerWebhook,
			BatchesBitbucketCloudWebhook:    enterprise.BatchesBitbucketCloudWebhook,
			BatchesAzureDevOpsWebhook:       enterprise.BatchesAzureDevOpsWebhook,
			BatchesChangesFileGetHandler:    enterprise.BatchesChangesFileGetHandler,
			BatchesChangesFileExistsHandler: enterprise.BatchesChangesFileExistsHandler,
			BatchesChangesFileUploadHandler: enterprise.BatchesChangesFileUploadHandler,
			SCIMHandler:                     enterprise.SCIMHandler,
			NewCodeIntelUploadHandler:       enterprise.NewCodeIntelUploadHandler,
			NewComputeStreamHandler:         enterprise.NewComputeStreamHandler,
			CodeInsightsDataExportHandler:   enterprise.CodeInsightsDataExportHandler,
			SearchJobsDataExportHandler:     enterprise.SearchJobsDataExportHandler,
			SearchJobsLogsHandler:           enterprise.SearchJobsLogsHandler,
			NewDotcomLicenseCheckHandler:    enterprise.NewDotcomLicenseCheckHandler,
			NewChatCompletionsStreamHandler: enterprise.NewChatCompletionsStreamHandler,
			NewCodeCompletionsHandler:       enterprise.NewCodeCompletionsHandler,
		},
		enterprise.NewExecutorProxyHandler,
	)
	if err != nil {
		return nil, errors.Errorf("create external HTTP handler: %v", err)
	}
	httpServer := &http.Server{
		Handler:      externalHandler,
		ReadTimeout:  75 * time.Second,
		WriteTimeout: 10 * time.Minute,
	}

	server := httpserver.New(listener, httpServer, makeServerOptions()...)
	logger.Debug("HTTP running", sglog.String("on", httpAddr))
	return server, nil
}

func makeInternalAPI(
	db database.DB,
	logger sglog.Logger,
	schema *graphql.Schema,
	enterprise enterprise.Services,
	rateLimiter graphqlbackend.LimitWatcher,
) (goroutine.BackgroundRoutine, error) {
	if httpAddrInternal == "" {
		return nil, nil
	}

	listener, err := httpserver.NewListener(httpAddrInternal)
	if err != nil {
		return nil, err
	}

	grpcServer := defaults.NewServer(logger)

	// The internal HTTP handler does not include the auth handlers.
	internalHandler := newInternalHTTPHandler(
		schema,
		db,
		grpcServer,
		enterprise.NewCodeIntelUploadHandler,
		enterprise.RankingService,
		enterprise.NewComputeStreamHandler,
		rateLimiter,
	)
	internalHandler = internalgrpc.MultiplexHandlers(grpcServer, internalHandler)

	httpServer := &http.Server{
		Handler:     internalHandler,
		ReadTimeout: 75 * time.Second,
		// Higher since for internal RPCs which can have large responses
		// (eg git archive). Should match the timeout used for git archive
		// in gitserver.
		WriteTimeout: time.Hour,
	}

	server := httpserver.New(listener, httpServer, makeServerOptions()...)
	logger.Debug("HTTP (internal) running", sglog.String("on", httpAddrInternal))
	return server, nil
}

func makeServerOptions() (options []httpserver.ServerOptions) {
	if deploy.IsDeployTypeKubernetes(deploy.Type()) {
		// On kubernetes, we want to wait an additional 5 seconds after we receive a
		// shutdown request to give some additional time for the endpoint changes
		// to propagate to services talking to this server like the LB or ingress
		// controller. We only do this in frontend and not on all services, because
		// frontend is the only publicly exposed service where we don't control
		// retries on connection failures (see httpcli.InternalClient).
		options = append(options, httpserver.WithPreShutdownPause(time.Second*5))
	}

	return options
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, o := range allowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

func makeRateLimitWatcher() (*graphqlbackend.BasicLimitWatcher, error) {
	var store throttled.GCRAStoreCtx
	var err error
	pool := redispool.Cache.Pool()
	store, err = redigostore.NewCtx(pool, "gql:rl:", 0)
	if err != nil {
		return nil, err
	}

	return graphqlbackend.NewBasicLimitWatcher(sglog.Scoped("BasicLimitWatcher"), store), nil
}

// GetInternalAddr returns the address of the internal HTTP API server.
func GetInternalAddr() string {
	return httpAddrInternal
}
