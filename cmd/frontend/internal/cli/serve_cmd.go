package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/inconshreveable/log15"
	"github.com/keegancsmith/tmpfriend"
	"github.com/throttled/throttled/v2/store/redigostore"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/updatecheck"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/bg"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cli/loghandlers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/siteid"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/vfsutil"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/logging"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/profiler"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/sysreq"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/tracer"
	"github.com/sourcegraph/sourcegraph/internal/version"
)

var (
	traceFields    = env.Get("SRC_LOG_TRACE", "HTTP", "space separated list of trace logs to show. Options: all, HTTP, build, github")
	traceThreshold = env.Get("SRC_LOG_TRACE_THRESHOLD", "", "show traces that take longer than this")

	printLogo, _ = strconv.ParseBool(env.Get("LOGO", "false", "print Sourcegraph logo upon startup"))

	httpAddr         = env.Get("SRC_HTTP_ADDR", ":3080", "HTTP listen address for app and HTTP API")
	httpAddrInternal = envvar.HTTPAddrInternal

	nginxAddr = env.Get("SRC_NGINX_HTTP_ADDR", "", "HTTP listen address for nginx reverse proxy to SRC_HTTP_ADDR. Has preference over SRC_HTTP_ADDR for ExternalURL.")

	// dev browser browser extension ID. You can find this by going to chrome://extensions
	devExtension = "chrome-extension://bmfbcejdknlknpncfpeloejonjoledha"
	// production browser extension ID. This is found by viewing our extension in the chrome store.
	prodExtension = "chrome-extension://dgjhfomjieaadpoljlnidmbgkdffpack"
)

func init() {
	// If CACHE_DIR is specified, use that
	cacheDir := env.Get("CACHE_DIR", "/tmp", "directory to store cached archives.")
	vfsutil.ArchiveCacheDir = filepath.Join(cacheDir, "frontend-archive-cache")
}

// defaultExternalURL returns the default external URL of the application.
func defaultExternalURL(nginxAddr, httpAddr string) *url.URL {
	addr := nginxAddr
	if addr == "" {
		addr = httpAddr
	}

	var hostPort string
	if strings.HasPrefix(addr, ":") {
		// Prepend localhost if HTTP listen addr is just a port.
		hostPort = "127.0.0.1" + addr
	} else {
		hostPort = addr
	}

	return &url.URL{Scheme: "http", Host: hostPort}
}

// InitDB initializes and returns the global database connection and sets the
// version of the frontend in our versions table.
func InitDB() (*sql.DB, error) {
	if err := dbconn.SetupGlobalConnection(""); err != nil {
		return nil, fmt.Errorf("failed to connect to frontend database: %s", err)
	}

	ctx := context.Background()
	migrate := true

	for {
		// We need this loop so that we handle the missing versions table,
		// which would be added by running the migrations. Once we detect that
		// it's missing, we run the migrations and try to update the version again.

		err := backend.UpdateServiceVersion(ctx, "frontend", version.Version())
		if err != nil && !dbutil.IsPostgresError(err, "42P01") {
			return nil, err
		}

		if !migrate {
			return dbconn.Global, nil
		}

		if err := dbconn.MigrateDB(dbconn.Global, dbconn.Frontend); err != nil {
			return nil, err
		}

		migrate = false
	}
}

// Main is the main entrypoint for the frontend server program.
func Main(enterpriseSetupHook func(db dbutil.DB, outOfBandMigrationRunner *oobmigration.Runner) enterprise.Services) error {
	ctx := context.Background()

	log.SetFlags(0)
	log.SetPrefix("")

	if err := profiler.Init(); err != nil {
		log.Fatalf("failed to initialize profiling: %v", err)
	}

	db, err := InitDB()
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	ui.InitRouter(db)

	// override site config first
	if err := overrideSiteConfig(ctx); err != nil {
		log.Fatalf("failed to apply site config overrides: %v", err)
	}
	globals.ConfigurationServerFrontendOnly = conf.InitConfigurationServerFrontendOnly(&configurationSource{})
	conf.MustValidateDefaults()

	// now we can init the keyring, as it depends on site config
	if err := keyring.Init(ctx); err != nil {
		log.Fatalf("failed to initialize encryption keyring: %v", err)
	}

	if err := overrideGlobalSettings(ctx, db); err != nil {
		log.Fatalf("failed to override global settings: %v", err)
	}

	// now the keyring is configured it's safe to override the rest of the config
	// and that config can access the keyring
	if err := overrideExtSvcConfig(ctx, db); err != nil {
		log.Fatalf("failed to override external service config: %v", err)
	}

	// Filter trace logs
	d, _ := time.ParseDuration(traceThreshold)
	logging.Init(logging.Filter(loghandlers.Trace(strings.Fields(traceFields), d)))
	tracer.Init()
	trace.Init(true)

	// Create an out-of-band migration runner onto which each enterprise init function
	// can register migration routines to run in the background while they have work
	// remaining.
	outOfBandMigrationRunner := oobmigration.NewRunnerWithDB(db, time.Second*30)

	// Run enterprise setup hook
	enterprise := enterpriseSetupHook(db, outOfBandMigrationRunner)

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "help", "-h", "--help":
			log.Printf("Version: %s", version.Version())
			log.Print()

			env.PrintHelp()

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

	printConfigValidation()

	cleanup := tmpfriend.SetupOrNOOP()
	defer cleanup()

	// Don't proceed if system requirements are missing, to avoid
	// presenting users with a half-working experience.
	if err := checkSysReqs(context.Background(), os.Stderr); err != nil {
		return err
	}

	go debugserver.Start()

	siteid.Init()

	globals.WatchExternalURL(defaultExternalURL(nginxAddr, httpAddr))
	globals.WatchPermissionsUserMapping()

	goroutine.Go(func() { bg.CheckRedisCacheEvictionPolicy() })
	goroutine.Go(func() { bg.DeleteOldCacheDataInRedis() })
	goroutine.Go(func() { bg.DeleteOldEventLogsInPostgres(context.Background()) })
	go updatecheck.Start(db)

	// Parse GraphQL schema and set up resolvers that depend on dbconn.Global
	// being initialized
	if dbconn.Global == nil {
		return errors.New("dbconn.Global is nil when trying to parse GraphQL schema")
	}

	schema, err := graphqlbackend.NewSchema(db, enterprise.BatchChangesResolver, enterprise.CodeIntelResolver, enterprise.InsightsResolver, enterprise.AuthzResolver, enterprise.CodeMonitorsResolver, enterprise.LicenseResolver)
	if err != nil {
		return err
	}

	ratelimitStore, err := redigostore.New(redispool.Cache, "gql:rl:", 0)
	if err != nil {
		return err
	}
	rateLimitWatcher := graphqlbackend.NewRateLimiteWatcher(ratelimitStore)

	server, err := makeExternalAPI(db, schema, enterprise, rateLimitWatcher)
	if err != nil {
		return err
	}

	internalAPI, err := makeInternalAPI(schema, db, enterprise, rateLimitWatcher)
	if err != nil {
		return err
	}

	routines := []goroutine.BackgroundRoutine{
		server,
		outOfBandMigrationRunner,
	}
	if internalAPI != nil {
		routines = append(routines, internalAPI)
	}

	if printLogo {
		fmt.Println(" ")
		fmt.Println(logoColor)
		fmt.Println(" ")
	}
	fmt.Printf("✱ Sourcegraph is ready at: %s\n", globals.ExternalURL())

	goroutine.MonitorBackgroundRoutines(context.Background(), routines...)
	return nil
}

func makeExternalAPI(db dbutil.DB, schema *graphql.Schema, enterprise enterprise.Services, rateLimiter *graphqlbackend.RateLimitWatcher) (goroutine.BackgroundRoutine, error) {
	// Create the external HTTP handler.
	externalHandler, err := newExternalHTTPHandler(db, schema, enterprise.GitHubWebhook, enterprise.GitLabWebhook, enterprise.BitbucketServerWebhook, enterprise.NewCodeIntelUploadHandler, enterprise.NewExecutorProxyHandler, rateLimiter)
	if err != nil {
		return nil, err
	}

	listener, err := httpserver.NewListener(httpAddr)
	if err != nil {
		return nil, err
	}

	server := httpserver.New(listener, &http.Server{
		Handler:      externalHandler,
		ReadTimeout:  75 * time.Second,
		WriteTimeout: 10 * time.Minute,
	})

	log15.Debug("HTTP running", "on", httpAddr)
	return server, nil
}

func makeInternalAPI(schema *graphql.Schema, db dbutil.DB, enterprise enterprise.Services, rateLimiter *graphqlbackend.RateLimitWatcher) (goroutine.BackgroundRoutine, error) {
	if httpAddrInternal == "" {
		return nil, nil
	}

	listener, err := httpserver.NewListener(httpAddrInternal)
	if err != nil {
		return nil, err
	}

	// The internal HTTP handler does not include the auth handlers.
	internalHandler := newInternalHTTPHandler(schema, db, enterprise.NewCodeIntelUploadHandler, rateLimiter)

	server := httpserver.New(listener, &http.Server{
		Handler:     internalHandler,
		ReadTimeout: 75 * time.Second,
		// Higher since for internal RPCs which can have large responses
		// (eg git archive). Should match the timeout used for git archive
		// in gitserver.
		WriteTimeout: time.Hour,
	})

	log15.Debug("HTTP (internal) running", "on", httpAddrInternal)
	return server, nil
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, o := range allowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}
