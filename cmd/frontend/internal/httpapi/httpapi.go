package httpapi

import (
	"context"
	"fmt"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/graph-gophers/graphql-go"
	sglog "github.com/sourcegraph/log"
	"golang.org/x/oauth2/clientcredentials"
	"google.golang.org/grpc"

	// Use the current model config (models.json) that is built with the binary.
	// TODO: Load this from memory, so that we can apply any server-side config settings,
	// e.g. what the user specified from site config, or updates we pulled from Cody Gateway.

	zoektProto "github.com/sourcegraph/zoekt/cmd/zoekt-sourcegraph-indexserver/protos/sourcegraph/zoekt/configuration/v1"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/clientconfig"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/handlerutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/httpapi/releasecache"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/httpapi/webhookhandlers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/routevar"
	frontendsearch "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/ssc"
	frontendregistry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/webhooks"
	"github.com/sourcegraph/sourcegraph/internal/api"
	confProto "github.com/sourcegraph/sourcegraph/internal/api/internalapi/v1"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/sams"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/searchcontexts"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/updatecheck"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Handlers struct {
	// Repo sync
	GitHubSyncWebhook          webhooks.Registerer
	GitLabSyncWebhook          webhooks.Registerer
	BitbucketServerSyncWebhook webhooks.Registerer
	BitbucketCloudSyncWebhook  webhooks.Registerer

	// Permissions
	PermissionsGitHubWebhook webhooks.Registerer

	// Batch changes
	BatchesGitHubWebhook            webhooks.Registerer
	BatchesGitLabWebhook            webhooks.RegistererHandler
	BatchesBitbucketServerWebhook   webhooks.RegistererHandler
	BatchesBitbucketCloudWebhook    webhooks.RegistererHandler
	BatchesAzureDevOpsWebhook       webhooks.Registerer
	BatchesChangesFileGetHandler    http.Handler
	BatchesChangesFileExistsHandler http.Handler
	BatchesChangesFileUploadHandler http.Handler

	// SCIM
	SCIMHandler http.Handler

	// Code intel
	NewCodeIntelUploadHandler enterprise.NewCodeIntelUploadHandler

	// Compute
	NewComputeStreamHandler enterprise.NewComputeStreamHandler

	// Code Insights
	CodeInsightsDataExportHandler http.Handler

	// Search jobs
	SearchJobsDataExportHandler http.Handler
	SearchJobsLogsHandler       http.Handler

	// Dotcom license check
	NewDotcomLicenseCheckHandler enterprise.NewDotcomLicenseCheckHandler

	// Completions stream
	NewChatCompletionsStreamHandler enterprise.NewChatCompletionsStreamHandler
	NewCodeCompletionsHandler       enterprise.NewCodeCompletionsHandler
}

// NewHandler returns a new API handler.
//
// 🚨 SECURITY: The caller MUST wrap the returned handler in middleware that checks authentication
// and sets the actor in the request context.
func NewHandler(
	db database.DB,
	schema *graphql.Schema,
	rateLimiter graphqlbackend.LimitWatcher,
	handlers *Handlers,
) (http.Handler, error) {
	logger := sglog.Scoped("Handler")

	m := mux.NewRouter().PathPrefix("/.api/").Subrouter()
	m.StrictSlash(true)
	m.Use(trace.Route)

	jsonHandler := JsonMiddleware(&ErrorHandler{
		Logger: logger,
		// Only display error message to admins when in debug mode, since it
		// may contain sensitive info (like API keys in net/http error
		// messages).
		WriteErrBody: env.InsecureDev,
	})

	m.PathPrefix("/registry").Methods("GET").Handler(jsonHandler(frontendregistry.HandleRegistry))
	m.PathPrefix("/scim/v2").Methods("GET", "POST", "PUT", "PATCH", "DELETE").Handler(handlers.SCIMHandler)
	m.Path("/graphql").Methods("POST").Handler(jsonHandler(serveGraphQL(logger, schema, rateLimiter, false)))

	m.Path("/opencodegraph").Methods("POST").Handler(jsonHandler(serveOpenCodeGraph(logger)))

	// Webhooks
	//
	// First: register handlers
	wh := webhooks.Router{
		Logger: logger.Scoped("webhooks.Router"),
		DB:     db,
	}
	webhookhandlers.Init(&wh)
	handlers.BatchesGitHubWebhook.Register(&wh)
	handlers.BatchesGitLabWebhook.Register(&wh)
	handlers.BitbucketServerSyncWebhook.Register(&wh)
	handlers.BitbucketCloudSyncWebhook.Register(&wh)
	handlers.BatchesBitbucketServerWebhook.Register(&wh)
	handlers.BatchesBitbucketCloudWebhook.Register(&wh)
	handlers.GitHubSyncWebhook.Register(&wh)
	handlers.GitLabSyncWebhook.Register(&wh)
	handlers.PermissionsGitHubWebhook.Register(&wh)
	handlers.BatchesAzureDevOpsWebhook.Register(&wh)
	// Second: register handler on main router
	// 🚨 SECURITY: This handler implements its own secret-based auth
	webhookMiddleware := webhooks.NewLogMiddleware(db.WebhookLogs(keyring.Default().WebhookLogKey))
	webhookHandler := webhooks.NewHandler(logger, db, &wh)
	m.Path("/webhooks/{webhook_uuid}").Methods("POST").Handler(webhookMiddleware.Logger(webhookHandler))

	// Old, soon to be deprecated, webhook handlers
	gitHubWebhook := webhooks.GitHubWebhook{Router: &wh}
	m.Path("/github-webhooks").Methods("POST").Handler(webhookMiddleware.Logger(&gitHubWebhook))
	m.Path("/gitlab-webhooks").Methods("POST").Handler(webhookMiddleware.Logger(handlers.BatchesGitLabWebhook))
	m.Path("/bitbucket-server-webhooks").Methods("POST").Handler(webhookMiddleware.Logger(handlers.BatchesBitbucketServerWebhook))
	m.Path("/bitbucket-cloud-webhooks").Methods("POST").Handler(webhookMiddleware.Logger(handlers.BatchesBitbucketCloudWebhook))

	// Other routes
	m.Path("/files/batch-changes/{spec}/{file}").Methods("GET").Handler(handlers.BatchesChangesFileGetHandler)
	m.Path("/files/batch-changes/{spec}/{file}").Methods("HEAD").Handler(handlers.BatchesChangesFileExistsHandler)
	m.Path("/files/batch-changes/{spec}").Methods("POST").Handler(handlers.BatchesChangesFileUploadHandler)
	m.Path("/lsif/upload").Methods("POST").Handler(lsifDeprecationHandler)
	m.Path("/scip/upload").Methods("POST").Handler(handlers.NewCodeIntelUploadHandler(true))
	m.Path("/scip/upload").Methods("HEAD").Handler(noopHandler)
	m.Path("/compute/stream").Methods("GET", "POST").Handler(handlers.NewComputeStreamHandler())
	m.Path("/blame/" + routevar.Repo + routevar.RepoRevSuffix + "/stream/{Path:.*}").Methods("GET").Handler(handleStreamBlame(logger, db, gitserver.NewClient("http.blamestream")))
	// Set up the src-cli version cache handler (this will effectively be a
	// no-op anywhere other than dot-com).
	m.Path("/src-cli/versions/{rest:.*}").Methods("GET", "POST").Handler(releasecache.NewHandler(logger))
	// Return the minimum src-cli version that's compatible with this instance
	m.Path("/src-cli/{rest:.*}").Methods("GET").Handler(newSrcCliVersionHandler(logger))
	m.Path("/insights/export/{id}").Methods("GET").Handler(handlers.CodeInsightsDataExportHandler)
	m.Path("/search/stream").Methods("GET").Handler(frontendsearch.StreamHandler(db))
	m.Path("/search/export/{id}.jsonl").Methods("GET").Handler(handlers.SearchJobsDataExportHandler)
	m.Path("/search/export/{id}.log").Methods("GET").Handler(handlers.SearchJobsLogsHandler)

	m.Path("/completions/stream").Methods("POST").Handler(handlers.NewChatCompletionsStreamHandler())
	m.Path("/completions/code").Methods("POST").Handler(handlers.NewCodeCompletionsHandler())

	// HTTP endpoints related to Cody client configuration.
	clientConfigHandlers := clientconfig.NewHandlers(db, logger)
	m.Path("/client-config").Methods("GET").HandlerFunc(clientConfigHandlers.GetClientConfigHandler)

	if dotcom.SourcegraphDotComMode() {
		m.Path("/license/check").Methods("POST").Name("dotcom.license.check").Handler(handlers.NewDotcomLicenseCheckHandler())

		updatecheckHandler, err := updatecheck.ForwardHandler()
		if err != nil {
			return nil, errors.Errorf("create updatecheck handler: %v", err)
		}
		m.Path("/updates").
			Methods(http.MethodGet, http.MethodPost).
			Name("updatecheck").
			Handler(updatecheckHandler)

		// Register additional endpoints specific to DOTCOM.
		dotcomConf := conf.Get().Dotcom
		if dotcomConf == nil {
			logger.Error("dotcom configuration is missing, refusing to register /ssc/ APIs")
		} else {
			samsClient := sams.NewClient(
				dotcomConf.SamsServer,
				clientcredentials.Config{
					ClientID:     dotcomConf.SamsClientID,
					ClientSecret: dotcomConf.SamsClientSecret,
					TokenURL:     fmt.Sprintf("%s/oauth/token", dotcomConf.SamsServer),
					Scopes:       []string{"openid", "profile", "email"},
				},
			)

			samsAuthenticator := sams.Authenticator{
				Logger:     logger.Scoped("sams.Authenticator"),
				SAMSClient: samsClient,
			}

			// API endpoint for the SSC backend to trigger cody's rate limit refresh for a user.
			// TODO(sourcegraph#59625): Remove this as part of adding SAMSActor source.
			m.Path("/ssc/users/{samsAccountID}/cody/limits/refresh").Methods("POST").Handler(
				samsAuthenticator.RequireScopes(
					[]sams.Scope{sams.ScopeDotcom},
					newSSCRefreshCodyRateLimitHandler(logger, db),
				),
			)

			// API endpoint for proxying an arbitrary API request to the SSC backend.
			//
			// SECURITY: We are relying on the caller of this function to register the
			// necessary authentication middleware. (e.g. injecting the Sourcegraph actor
			// based on the session cookie or Sg user access token in the request's header.)
			//
			// This middleware handler then exchanges the authenticated Sourcegraph user's
			// credentials for their SAMS external identy's access token, and proxies the
			// HTTP call to the SSC backend.
			//
			// This means that for any cookie-based authentication method, we need to have
			// CSRF protection. (However, that appears to be the case, see `newExternalHTTPHandler`
			// and its use of `CookieMiddlewareWithCSRFSafety`.)
			samsOAuthConfig, err := ssc.GetSAMSOAuthContext()
			if err != nil {
				// This situation is pretty bad, as it means no Cody Pro-related functionality
				// can work properly. So while the site can continue to load as expected,
				// we will supply a zero-value OAuth config that will only serve 503s.
				//
				// This makes the failure a lot more obvious than not registering the routes
				// at all, and trying to figure out why we are seeing 404s or 405s.
				logger.Error("error loading SAMS config, unable to register SSC API proxy", sglog.Error(err))
			}
			sscBackendProxy := ssc.APIProxyHandler{
				CodyProConfig:    conf.Get().Dotcom.CodyProConfig,
				DB:               db,
				Logger:           logger.Scoped("sscProxy"),
				URLPrefix:        "/.api/ssc/proxy",
				SAMSOAuthContext: samsOAuthConfig,
			}
			m.PathPrefix("/ssc/proxy/").Handler(&sscBackendProxy)
		}
	}

	// repo contains routes that are NOT specific to a revision. In these routes, the URL may not contain a revspec after the repo (that is, no "github.com/foo/bar@myrevspec").
	// repo contains routes that are NOT specific to a revision. In these routes, the URL may not contain a revspec after the repo (that is, no "github.com/foo/bar@myrevspec").
	repoPath := `/repos/` + routevar.Repo
	// Additional paths added will be treated as a repo. To add a new path that should not be treated as a repo
	// add above repo paths.
	repo := m.PathPrefix(repoPath + "/" + routevar.RepoPathDelim + "/").Subrouter()
	repo.Path("/shield").Methods("GET").Handler(jsonHandler(serveRepoShield()))
	repo.Path("/refresh").Methods("POST").Handler(jsonHandler(serveRepoRefresh(db)))

	m.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("API no route: %s %s from %s", r.Method, r.URL, r.Referer())
		http.Error(w, "no route", http.StatusNotFound)
	})

	return m, nil
}

const (
	gitInfoRefs   = "internal.git.info-refs"
	gitUploadPack = "internal.git.upload-pack"
)

// RegisterInternalServices registers REST and gRPC handlers for Sourcegraph's internal API on the
// provided mux.Router and gRPC server.
//
// 🚨 SECURITY: This handler should not be served on a publicly exposed port. 🚨
// This handler is not guaranteed to provide the same authorization checks as
// public API handlers.
func RegisterInternalServices(
	m *mux.Router,
	s *grpc.Server,

	db database.DB,
	schema *graphql.Schema,
	newCodeIntelUploadHandler enterprise.NewCodeIntelUploadHandler,
	rankingService enterprise.RankingService,
	newComputeStreamHandler enterprise.NewComputeStreamHandler,
	rateLimitWatcher graphqlbackend.LimitWatcher,
) {
	logger := sglog.Scoped("InternalHandler")
	m.StrictSlash(true)

	handler := JsonMiddleware(&ErrorHandler{
		Logger: logger,
		// Internal endpoints can expose sensitive errors
		WriteErrBody: true,
	})

	gsClient := gitserver.NewClient("http.internalapi")

	// zoekt-indexserver endpoints
	indexer := &searchIndexerServer{
		db:              db,
		logger:          logger.Scoped("searchIndexerServer"),
		gitserverClient: gsClient.Scoped("zoektindexerserver"),
		ListIndexable:   backend.NewRepos(logger, db, gsClient.Scoped("zoektindexerserver")).ListIndexable,
		repoStore:       db.Repos(),
		SearchContextsRepoRevs: func(ctx context.Context, repoIDs []api.RepoID) (map[api.RepoID][]string, error) {
			return searchcontexts.RepoRevs(ctx, db, repoIDs)
		},
		indexers:               search.Indexers(),
		Ranking:                rankingService,
		MinLastChangedDisabled: os.Getenv("SRC_SEARCH_INDEXER_EFFICIENT_POLLING_DISABLED") != "",
	}

	gitService := &gitServiceHandler{Gitserver: gsClient.Scoped("gitservice")}
	m.Path("/git/{RepoName:.*}/info/refs").Methods("GET").Name(gitInfoRefs).Handler(trace.Route(handler(gitService.serveInfoRefs())))
	m.Path("/git/{RepoName:.*}/git-upload-pack").Methods("GET", "POST").Name(gitUploadPack).Handler(trace.Route(handler(gitService.serveGitUploadPack())))

	m.Path("/lsif/upload").Methods("POST").Handler(trace.Route(newCodeIntelUploadHandler(false)))
	m.Path("/scip/upload").Methods("POST").Handler(trace.Route(newCodeIntelUploadHandler(false)))
	m.Path("/scip/upload").Methods("HEAD").Handler(trace.Route(noopHandler))
	m.Path("/search/stream").Methods("GET").Handler(trace.Route(frontendsearch.StreamHandler(db)))
	m.Path("/compute/stream").Methods("GET", "POST").Handler(trace.Route(newComputeStreamHandler()))
	m.Path("/graphql").Methods("POST").Handler(trace.Route(handler(serveGraphQL(logger, schema, rateLimitWatcher, true))))
	m.Path("/ping").Methods("GET").Name("ping").HandlerFunc(handlePing)

	zoektProto.RegisterZoektConfigurationServiceServer(s, &searchIndexerGRPCServer{server: indexer})
	confProto.RegisterConfigServiceServer(s, &configServer{})

	m.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("API no route: %s %s from %s", r.Method, r.URL, r.Referer())
		http.Error(w, "no route", http.StatusNotFound)
	})
}

var schemaDecoder = schema.NewDecoder()

func init() {
	schemaDecoder.IgnoreUnknownKeys(true)

	// Register a converter for unix timestamp strings -> time.Time values
	// (needed for Appdash PageLoadEvent type).
	schemaDecoder.RegisterConverter(time.Time{}, func(s string) reflect.Value {
		ms, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return reflect.ValueOf(err)
		}
		return reflect.ValueOf(time.Unix(0, ms*int64(time.Millisecond)))
	})
}

type ErrorHandler struct {
	// Logger is required
	Logger sglog.Logger

	WriteErrBody bool
}

func (h *ErrorHandler) Handle(w http.ResponseWriter, r *http.Request, status int, err error) {
	logger := trace.Logger(r.Context(), h.Logger)

	trace.SetRequestErrorCause(r.Context(), err)

	// Handle custom errors
	var e *handlerutil.URLMovedError
	if errors.As(err, &e) {
		err := handlerutil.RedirectToNewRepoName(w, r, e.NewRepo)
		if err != nil {
			logger.Error("error redirecting to new URI",
				sglog.Error(err),
				sglog.String("new_url", string(e.NewRepo)))
		}
		return
	}

	// Never cache error responses.
	w.Header().Set("cache-control", "no-cache, max-age=0")

	errBody := err.Error()

	var displayErrBody string
	if h.WriteErrBody {
		displayErrBody = errBody
	}
	http.Error(w, displayErrBody, status)

	// No need to log, as SetRequestErrorCause is consumed and logged.
}

func JsonMiddleware(errorHandler *ErrorHandler) func(func(http.ResponseWriter, *http.Request) error) http.Handler {
	return func(h func(http.ResponseWriter, *http.Request) error) http.Handler {
		return handlerutil.HandlerWithErrorReturn{
			Handler: func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("Content-Type", "application/json")
				return h(w, r)
			},
			Error: errorHandler.Handle,
		}
	}
}

var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

var lsifDeprecationHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte("Sourcegraph v4.5+ no longer accepts LSIF uploads. The Sourcegraph CLI v4.4.2+ will translate LSIF to SCIP prior to uploading. Please check the version of the CLI utility used to upload this artifact."))
})
