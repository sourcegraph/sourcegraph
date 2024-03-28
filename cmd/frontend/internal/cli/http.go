package cli

import (
	"net/http"
	"strings"

	"github.com/NYTimes/gziphandler"
	gcontext "github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/graph-gophers/graphql-go"
	"google.golang.org/grpc"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/assetsutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/ipallowlist"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/authz"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cli/middleware"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/httpapi"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	internalauth "github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/deviceid"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/instrumentation"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
	"github.com/sourcegraph/sourcegraph/internal/requestinteraction"
	"github.com/sourcegraph/sourcegraph/internal/session"
	tracepkg "github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// newExternalHTTPHandler creates and returns the HTTP handler that serves the app and API pages to
// external clients.
func newExternalHTTPHandler(
	db database.DB,
	schema *graphql.Schema,
	rateLimitWatcher graphqlbackend.LimitWatcher,
	handlers *httpapi.Handlers,
	newExecutorProxyHandler enterprise.NewExecutorProxyHandler,
) (http.Handler, error) {
	logger := log.Scoped("external")

	ipAllowlistMiddleware := ipallowlist.New(logger)

	// Each auth middleware determines on a per-request basis whether it should be enabled (if not, it
	// immediately delegates the request to the next middleware in the chain).
	authMiddlewares := auth.AuthMiddleware()

	// HTTP API handler, the call order of middleware is LIFO.
	apiHandler, err := httpapi.NewHandler(db, schema, rateLimitWatcher, handlers)
	if err != nil {
		return nil, errors.Errorf("create external HTTP API handler: %v", err)
	}
	// 🚨 SECURITY: These all run after the auth handler so the client is authenticated.
	apiHandler = authz.PostAuthMiddleware(logger, db, apiHandler)
	apiHandler = featureflag.Middleware(db.FeatureFlags(), apiHandler)
	apiHandler = actor.AnonymousUIDMiddleware(apiHandler)
	apiHandler = authMiddlewares.API(apiHandler) // 🚨 SECURITY: auth middleware
	// 🚨 SECURITY: The HTTP API should not accept cookies as authentication, except from trusted
	// origins, to avoid CSRF attacks. See session.CookieMiddlewareWithCSRFSafety for details.
	apiHandler = session.CookieMiddlewareWithCSRFSafety(logger, db, apiHandler, corsAllowHeader, isTrustedOrigin) // API accepts cookies with special header
	apiHandler = httpapi.AccessTokenAuthMiddleware(db, logger, apiHandler)                                        // API accepts access tokens
	apiHandler = requestclient.ExternalHTTPMiddleware(apiHandler)
	apiHandler = requestinteraction.HTTPMiddleware(apiHandler)
	apiHandler = gziphandler.GzipHandler(apiHandler)
	if dotcom.SourcegraphDotComMode() {
		apiHandler = deviceid.Middleware(apiHandler)
	}

	// 🚨 SECURITY: This handler implements its own token auth inside enterprise
	executorProxyHandler := newExecutorProxyHandler()

	// App handler (HTML pages), the call order of middleware is LIFO.
	appHandler := app.NewHandler(db, logger)
	// 🚨 SECURITY: These all run after the auth handler so the client is authenticated.
	appHandler = authz.PostAuthMiddleware(logger, db, appHandler)
	appHandler = featureflag.Middleware(db.FeatureFlags(), appHandler)
	appHandler = actor.AnonymousUIDMiddleware(appHandler)
	appHandler = authMiddlewares.App(appHandler) // 🚨 SECURITY: auth middleware
	appHandler = middleware.OpenGraphMetadataMiddleware(db.FeatureFlags(), appHandler)
	appHandler = session.CookieMiddleware(logger, db, appHandler)          // app accepts cookies
	appHandler = httpapi.AccessTokenAuthMiddleware(db, logger, appHandler) // app accepts access tokens
	appHandler = requestclient.ExternalHTTPMiddleware(appHandler)
	appHandler = requestinteraction.HTTPMiddleware(appHandler)
	if dotcom.SourcegraphDotComMode() {
		appHandler = deviceid.Middleware(appHandler)
	}
	// Mount handlers and assets.
	sm := http.NewServeMux()
	sm.Handle("/.api/", secureHeadersMiddleware(apiHandler, crossOriginPolicyAPI))
	sm.Handle("/.executors/", secureHeadersMiddleware(executorProxyHandler, crossOriginPolicyNever))
	sm.Handle("/", secureHeadersMiddleware(appHandler, crossOriginPolicyNever))
	const urlPathPrefix = "/.assets"
	// The asset handler should be wrapped into a middleware that enables cross-origin requests
	// to allow the loading of the Phabricator native extension assets.
	assetHandler := assetsutil.NewAssetHandler(sm)
	sm.Handle(urlPathPrefix+"/", http.StripPrefix(urlPathPrefix, secureHeadersMiddleware(assetHandler, crossOriginPolicyAssets)))

	var h http.Handler = sm

	// Wrap in middleware, first line is last to run.
	//
	// 🚨 SECURITY: Auth middleware that must run before other auth middlewares.
	h = middleware.Trace(h)
	h = middleware.DataLoader(h)
	h = gcontext.ClearHandler(h)
	h = healthCheckMiddleware(h)
	h = middleware.BlackHole(h)
	h = middleware.SourcegraphComGoGetHandler(h)
	h = internalauth.ForbidAllRequestsMiddleware(h)
	h = tracepkg.HTTPMiddleware(logger, h, conf.DefaultClient())
	h = instrumentation.HTTPMiddleware("external", h)
	// 🚨 SECURITY: ip allowlist must be the first middleware to run to avoid doing unnecessary things
	h = ipAllowlistMiddleware.Handle(h)

	return h, nil
}

func healthCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/__version":
			_, _ = w.Write([]byte(version.Version()))
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// newInternalHTTPHandler creates and returns the HTTP handler for the internal API (accessible to
// other internal services).
func newInternalHTTPHandler(
	schema *graphql.Schema,
	db database.DB,
	grpcServer *grpc.Server,
	newCodeIntelUploadHandler enterprise.NewCodeIntelUploadHandler,
	rankingService enterprise.RankingService,
	newComputeStreamHandler enterprise.NewComputeStreamHandler,
	rateLimitWatcher graphqlbackend.LimitWatcher,
) http.Handler {
	internalMux := http.NewServeMux()
	logger := log.Scoped("internal")

	internalRouter := mux.NewRouter().PathPrefix("/.internal/").Subrouter()
	httpapi.RegisterInternalServices(
		internalRouter,
		grpcServer,
		db,
		schema,
		newCodeIntelUploadHandler,
		rankingService,
		newComputeStreamHandler,
		rateLimitWatcher,
	)

	internalMux.Handle("/.internal/", gziphandler.GzipHandler(
		actor.HTTPMiddleware(
			logger,
			featureflag.Middleware(
				db.FeatureFlags(),
				internalRouter,
			),
		),
	))

	h := http.Handler(internalMux)
	h = gcontext.ClearHandler(h)
	h = tracepkg.HTTPMiddleware(logger, h, conf.DefaultClient())
	h = instrumentation.HTTPMiddleware("internal", h)
	return h
}

// corsAllowHeader is the HTTP header that, if present (and assuming secureHeadersMiddleware is
// used), indicates that the incoming HTTP request is either same-origin or is from an allowed
// origin. See
// https://www.owasp.org/index.php/Cross-Site_Request_Forgery_(CSRF)_Prevention_Cheat_Sheet#Protecting_REST_Services:_Use_of_Custom_Request_Headers
// for more information on this technique.
const corsAllowHeader = "X-Requested-With"

// crossOriginPolicy describes the cross-origin policy the middleware should be enforcing.
type crossOriginPolicy string

const (
	// crossOriginPolicyAPI describes that the middleware should handle cross-origin requests as a
	// public API. That is, cross-origin requests are allowed from any domain but
	// cookie/session-based authentication is only allowed if the origin is in the configured
	// allow-list of origins. Otherwise, only access token authentication is permitted.
	//
	// This is to be used for all /.api routes, such as our GraphQL and search streaming APIs as we
	// want third-party websites (such as e.g. github1s.com, or internal tools for on-prem
	// customers) to be able to leverage our API. Their users will need to provide an access token,
	// or the website would need to be added to Sourcegraph's CORS allow list in order to be granted
	// cookie/session-based authentication (which is dangerous to expose to untrusted domains.)
	crossOriginPolicyAPI crossOriginPolicy = "API"

	// crossOriginPolicyAssets describes that the middleware should handle cross-origin requests to
	// static resources as a public API. That is, cross-origin requests are allowed from any domain.
	//
	// This is to be used for static assets served from the /.assets route. For example, using this
	// route, the Phabricator native extension loads styles via the fetch interface.
	crossOriginPolicyAssets crossOriginPolicy = "assets"

	// crossOriginPolicyNever describes that the middleware should handle cross-origin requests by
	// never allowing them. This makes sense for e.g. routes such as e.g. sign out pages, where
	// cookie based authentication is needed and requests should never come from a domain other than
	// the Sourcegraph instance itself.
	//
	// Important: This only applies to cross-origin requests issued by clients that respect CORS,
	// such as browsers. So for example Code Intelligence /.executors, despite being "an API",
	// should use this policy unless they intend to get cross-origin requests _from browsers_.
	crossOriginPolicyNever crossOriginPolicy = "never"
)

// secureHeadersMiddleware adds and checks for HTTP security-related headers.
//
// 🚨 SECURITY: This handler is served to all clients, even on private servers to clients who have
// not authenticated. It must not reveal any sensitive information.
func secureHeadersMiddleware(next http.Handler, policy crossOriginPolicy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// headers for security
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "DENY")
		// no cache by default
		w.Header().Set("Cache-Control", "no-cache, max-age=0")

		// Write CORS headers and potentially handle the requests if it is a OPTIONS request.
		if handled := handleCORSRequest(w, r, policy); handled {
			return // request was handled, do not invoke next handler
		}

		next.ServeHTTP(w, r)
	})
}

// handleCORSRequest handles checking the Origin header and writing CORS Access-Control-Allow-*
// headers. In some cases, it may handle OPTIONS CORS preflight requests in which case the function
// returns true and the request should be considered fully served.
func handleCORSRequest(w http.ResponseWriter, r *http.Request, policy crossOriginPolicy) (handled bool) {
	// If this route is one which should never allow cross-origin requests, then we should return
	// early. We do not write ANY Access-Control-Allow-* CORS headers, which triggers the browsers
	// default (and strict) behavior of not allowing cross-origin requests.
	//
	// We could instead parse the domain from conf.Get().ExternalURL and use that in the response,
	// to make things more explicit, but it would add more logic here to think about and you would
	// also want to think about whether or not `OPTIONS` requests should be handled and if the other
	// headers (-Credentials, -Methods, -Headers, etc.) should be sent back in such a situation.
	// Instead, it's easier to reason about the code by just saying "we send back nothing in this
	// case, and so the browser enforces no cross-origin requests".
	//
	// This is in compliance with section 7.2 "Resource Sharing Check" of the CORS standard: https://www.w3.org/TR/2020/SPSD-cors-20200602/#resource-sharing-check-0
	// It states:
	//
	// > If the response includes zero or more than one Access-Control-Allow-Origin header values,
	// > return fail and terminate this algorithm.
	//
	// And you may also see the type of error the browser would produce in this instance at e.g.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS/Errors/CORSMissingAllowOrigin
	//
	if policy == crossOriginPolicyNever {
		return false
	}

	// If the crossOriginPolicyAssets is used and the requested asset is not from the extension folder,
	// we do not write ANY Access-Control-Allow-* CORS headers, which triggers the browser's default
	// (and strict) behavior of not allowing cross-origin requests.
	//
	// We allow cross-origin requests for assets in the `./client/web/dist/extension` folder because they
	// are required for the native Phabricator extension.
	if policy == crossOriginPolicyAssets && !strings.HasPrefix(r.URL.Path, "/extension/") {
		return false
	}

	// crossOriginPolicyAPI and crossOriginPolicyAssets - handling of API and static assets routes.
	//
	// Even if the request was not from a trusted origin, we will allow the browser to send it AND
	// include credentials even. Traditionally, this would be a CSRF vulnerability! But because we
	// know for a fact that we will only respect sessions (cookie-based-authentication) iff the
	// request came from a trusted origin, in session.go:CookieMiddlewareWIthCSRFSafety, we know it
	// is safe to do this.
	//
	// This is the ONLY way in which we can enable public access of our Sourcegraph.com API, i.e. to
	// allow random.com to send requests to our GraphQL and search APIs either unauthenticated or
	// using an access token.
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Note: This must mirror the request's `Origin` header exactly as API users rely on this
	// codepath handling for example wildcards `*` and `null` origins properly. For example, if
	// Sourcegraph is behind a corporate VPN an admin may choose to set the CORS origin to "*" (via
	// a proxy, a browser would never send a literal "*") and would expect Sourcegraph to respond
	// appropriately with the request's Origin header. Similarly, some environments issue requests
	// with a `null` Origin header, such as VS Code extensions from within WebViews and Figma
	// extensions. Thus:
	//
	// 	"Origin: *" -> "Access-Control-Allow-Origin: *"
	// 	"Origin: null" -> "Access-Control-Allow-Origin: null"
	// 	"Origin: https://foobar.com" -> "Access-Control-Allow-Origin: https://foobar.com"
	//
	// Again, this is fine because we allow API requests from any origin and instead prevent CSRF
	// attacks via enforcing that we only respect session auth iff the origin is trusted. See the
	// docstring above this one for more info.
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))

	if r.Method == "OPTIONS" {
		// CRITICAL: Only trusted origins are allowed to send the secure X-Requested-With and
		// X-Sourcegraph-Client headers, which indicate to us later (in session.go:CookieMiddlewareWIthCSRFSafety)
		// that the request came from a trusted origin. To understand these secure headers, see
		// "What does X-Requested-With do anyway?" in https://github.com/sourcegraph/sourcegraph/pull/27931
		//
		// Any origin may send us POST, GET, OPTIONS requests with arbitrary content types, auth
		// (session cookies and access tokens), etc. but only trusted origins may send us the secure
		// X-Requested-With header.
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if isTrustedOrigin(r) {
			// X-Sourcegraph-Client is the deprecated form of X-Requested-With, so we treat it the same
			// way. It is NOT respected anymore, but is left as an allowed header so as to not block
			// requests that still do include it as e.g. part of a proxy put in front of Sourcegraph.
			w.Header().Set("Access-Control-Allow-Headers", corsAllowHeader+", X-Sourcegraph-Client, Content-Type, Authorization, X-Sourcegraph-Should-Trace")
		} else {
			// X-Sourcegraph-Should-Trace just indicates if we should record an HTTP request to our
			// tracing system and never has an impact on security, it's fine to always allow that
			// header to be set by browsers.
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Sourcegraph-Should-Trace")
		}
		w.WriteHeader(http.StatusOK)
		return true // we handled the request
	}
	return false
}

// isTrustedOrigin returns whether the HTTP request's Origin is trusted to initiate authenticated
// cross-origin requests.
func isTrustedOrigin(r *http.Request) bool {
	requestOrigin := r.Header.Get("Origin")

	isExtensionRequest := requestOrigin == devExtension || requestOrigin == prodExtension

	var isCORSAllowedRequest bool
	if corsOrigin := conf.Get().CorsOrigin; corsOrigin != "" {
		isCORSAllowedRequest = isAllowedOrigin(requestOrigin, strings.Fields(corsOrigin))
	}

	if externalURL := strings.TrimSuffix(conf.Get().ExternalURL, "/"); externalURL != "" && requestOrigin == externalURL {
		isCORSAllowedRequest = true
	}

	return isExtensionRequest || isCORSAllowedRequest
}
